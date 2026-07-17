package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"mtv2/backend/ai"
	"mtv2/backend/config"
	"mtv2/backend/database"

	"mtv2/backend/utils"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Tuning constants for the AI user-summary feature. Kept together so the
// input budget/cache lifetime is easy to audit and adjust.
const (
	aiSummaryMaxComments          = 300
	aiSummaryCacheTTL             = 7 * 24 * time.Hour
	aiSummaryCommentContentRunes  = 300
	aiSummaryProblemContextRunes  = 200
	aiSummaryProviderTimeout      = 60 * time.Second
	aiSummaryGenerationCtxTimeout = 130 * time.Second
	aiSummaryDBTimeout            = 20 * time.Second

	// With up to aiSummaryMaxComments (300) highest-liked comments now in
	// play, sending every one at full length could blow past a model's
	// context window. Instead of a flat per-comment cap, we track a total
	// rune budget across all sample content/audio/problemContext combined
	// (samples are processed highest-liked first, see the Mongo sort in
	// buildAIUserSummaryInput) and stop adding samples once the budget is
	// spent. This keeps prompt size bounded regardless of comment count,
	// while still preferring the most-liked (most representative) comments
	// and giving each of them their full length when there's headroom.
	aiSummarySampleBudgetRunes = 40000
	// aiSummaryMaxProblemLookups caps how many *distinct* problems we'll
	// query Mongo for best-effort context, independent of the rune budget
	// above, so a user with hundreds of comments across hundreds of
	// different problems can't turn one summary generation into hundreds
	// of extra round trips.
	aiSummaryMaxProblemLookups = 100

	// aiSummaryRawDebugRunes bounds how much of a provider's raw (unparsed
	// or truncated) response we echo back into LastError when generation
	// fails, so the failure is debuggable from the cached doc/API response
	// without risking an unbounded blob if a provider ever returns
	// something huge.
	aiSummaryRawDebugRunes = 2000
)

// generationLocks prevents two concurrent POST requests for the same user
// from calling the LLM (and upserting the cache) at the same time. This is a
// lightweight in-process mutex rather than a persisted job/task record: the
// feature is a manual, low-frequency, cache-first action (see plan), so a
// short-lived in-memory guard is enough to avoid duplicate spend/writes
// without introducing a task queue this codebase doesn't otherwise have.
var generationLocks sync.Map // userID (string) -> *sync.Mutex

func lockForUser(userID string) *sync.Mutex {
	lock, _ := generationLocks.LoadOrStore(userID, &sync.Mutex{})
	return lock.(*sync.Mutex)
}

// aiUserSummaryDoc is the MongoDB representation of a cached AI user summary.
type aiUserSummaryDoc struct {
	UserID        string                  `bson:"userId"`
	Status        string                  `bson:"status"` // "ready" or "failed"
	Result        *ai.UserSummaryResult   `bson:"result,omitempty"`
	Provider      string                  `bson:"provider,omitempty"`
	Model         string                  `bson:"model,omitempty"`
	PromptVersion string                  `bson:"promptVersion,omitempty"`
	InputSnapshot aiUserSummaryInputStats `bson:"inputSnapshot"`
	GeneratedAt   *time.Time              `bson:"generatedAt,omitempty"`
	ExpiresAt     *time.Time              `bson:"expiresAt,omitempty"`
	LastError     string                  `bson:"lastError,omitempty"`
	LastAttemptAt *time.Time              `bson:"lastAttemptAt,omitempty"`
}

type aiUserSummaryInputStats struct {
	TotalLikes              int64 `bson:"totalLikes" json:"totalLikes"`
	LikesRank               int64 `bson:"likesRank" json:"likesRank"`
	TotalCommentCount       int64 `bson:"totalCommentCount" json:"totalCommentCount"`
	SampledCommentCount     int   `bson:"sampledCommentCount" json:"sampledCommentCount"`
	SupportedDisplayCount   int64 `bson:"supportedDisplayCount" json:"supportedDisplayCount"`
	UnsupportedDisplayCount int64 `bson:"unsupportedDisplayCount" json:"unsupportedDisplayCount"`
	UnknownChoiceCount      int64 `bson:"unknownChoiceCount" json:"unknownChoiceCount"`
}

// aiCommentSample is a single redacted, length-bounded comment sent to the
// LLM. "ID" is a sequential sample label (e.g. "c1"), not the real commentId,
// so the model (and any provider-side logging) never sees internal
// identifiers, usernames, avatars, or media URLs.
type aiCommentSample struct {
	ID             string `json:"id"`
	Content        string `json:"content"`
	ApproveCount   int64  `json:"approveCount"`
	Choice         int    `json:"choice"`
	AudioText      string `json:"audioText,omitempty"`
	ProblemContext string `json:"problemContext,omitempty"`
}

// AIUserSummaryResponse is the JSON shape returned to the frontend for both
// GET (read cache) and POST (generate-or-return-cache).
type AIUserSummaryResponse struct {
	Status        string                `json:"status"` // "none" | "ready" | "failed"
	Result        *ai.UserSummaryResult `json:"result,omitempty"`
	Provider      string                `json:"provider,omitempty"`
	Model         string                `json:"model,omitempty"`
	PromptVersion string                `json:"promptVersion,omitempty"`
	GeneratedAt   *int64                `json:"generatedAt,omitempty"`
	ExpiresAt     *int64                `json:"expiresAt,omitempty"`
	Stale         bool                  `json:"stale"`
	LastError     string                `json:"lastError,omitempty"`
}

func docToResponse(doc *aiUserSummaryDoc) AIUserSummaryResponse {
	if doc == nil {
		return AIUserSummaryResponse{Status: "none"}
	}

	resp := AIUserSummaryResponse{
		Status:        doc.Status,
		Result:        doc.Result,
		Provider:      doc.Provider,
		Model:         doc.Model,
		PromptVersion: doc.PromptVersion,
		LastError:     doc.LastError,
	}
	if doc.GeneratedAt != nil {
		ts := doc.GeneratedAt.Unix()
		resp.GeneratedAt = &ts
		resp.Stale = time.Since(*doc.GeneratedAt) > aiSummaryCacheTTL
	}
	if doc.ExpiresAt != nil {
		ts := doc.ExpiresAt.Unix()
		resp.ExpiresAt = &ts
	}
	if resp.Status == "" {
		resp.Status = "none"
	}
	return resp
}

// GetAIUserSummary returns the cached AI summary for a user, if any, without
// triggering generation.
// GET /api/user_detail/ai_summary?userId=xxx
func GetAIUserSummary(c *gin.Context) {
	userID := c.Query("userId")
	if userID == "" {
		utils.BadRequestResponse(c, "Missing userId parameter")
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), aiSummaryDBTimeout)
	defer cancel()

	doc, err := findAIUserSummary(ctx, userID)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to load AI summary")
		return
	}

	utils.SuccessResponse(c, docToResponse(doc))
}

// PostAIUserSummary generates (or returns a still-fresh cached) AI summary
// for a user. Cache hits (generated within the last 7 days) are returned
// immediately without calling the LLM. On a cache miss/stale cache, this
// synchronously calls DeepSeek first, then OpenRouter as a fallback, and
// upserts the result before responding.
// POST /api/user_detail/ai_summary?userId=xxx
func PostAIUserSummary(c *gin.Context) {
	userID := c.Query("userId")
	if userID == "" {
		utils.BadRequestResponse(c, "Missing userId parameter")
		return
	}

	dbCtx, dbCancel := context.WithTimeout(context.Background(), aiSummaryDBTimeout)
	existing, err := findAIUserSummary(dbCtx, userID)
	dbCancel()
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to load AI summary")
		return
	}

	if existing != nil && existing.Status == "ready" && existing.GeneratedAt != nil &&
		time.Since(*existing.GeneratedAt) <= aiSummaryCacheTTL {
		utils.SuccessResponse(c, docToResponse(existing))
		return
	}

	lock := lockForUser(userID)
	if !lock.TryLock() {
		utils.ConflictResponse(c, "AI summary is already being generated for this user, please retry shortly")
		return
	}
	defer lock.Unlock()

	// Re-check the cache after acquiring the lock in case another request
	// finished generating it while we were waiting to acquire the lock.
	dbCtx2, dbCancel2 := context.WithTimeout(context.Background(), aiSummaryDBTimeout)
	existing, err = findAIUserSummary(dbCtx2, userID)
	dbCancel2()
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to load AI summary")
		return
	}
	if existing != nil && existing.Status == "ready" && existing.GeneratedAt != nil &&
		time.Since(*existing.GeneratedAt) <= aiSummaryCacheTTL {
		utils.SuccessResponse(c, docToResponse(existing))
		return
	}

	genCtx, genCancel := context.WithTimeout(context.Background(), aiSummaryGenerationCtxTimeout)
	defer genCancel()

	doc, genErr := generateAIUserSummary(genCtx, userID)
	if genErr != nil {
		utils.InternalServerErrorResponse(c, fmt.Sprintf("Failed to generate AI summary: %v", genErr))
		return
	}

	utils.SuccessResponse(c, docToResponse(doc))
}

func findAIUserSummary(ctx context.Context, userID string) (*aiUserSummaryDoc, error) {
	var doc aiUserSummaryDoc
	err := database.AIUserSummaries.FindOne(ctx, bson.M{"userId": userID}).Decode(&doc)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, err
	}
	return &doc, nil
}

// generateAIUserSummary builds the redacted prompt input from Mongo, calls
// the configured providers (DeepSeek then OpenRouter), validates/sanitizes
// the structured result, and upserts the cache document. On total provider
// failure, a "failed" status is upserted (without clobbering a still-valid
// prior "ready" result) and an error is returned.
func generateAIUserSummary(ctx context.Context, userID string) (*aiUserSummaryDoc, error) {
	stats, samples, allowedIDs, err := buildAIUserSummaryInput(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to build input: %w", err)
	}

	systemPrompt, err := ai.BuildUserProfileSummaryPrompt()
	if err != nil {
		return nil, fmt.Errorf("failed to build system prompt: %w", err)
	}

	userPrompt := renderUserPrompt(stats, samples)
	messages := []ai.Message{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userPrompt},
	}

	providers := []ai.Provider{
		{
			Name:    "deepseek",
			BaseURL: config.AppConfig.AI.DeepSeek.BaseURL,
			APIKey:  config.AppConfig.AI.DeepSeek.Key,
			Model:   config.AppConfig.AI.DeepSeek.Model,
		},
		{
			Name:    "openrouter",
			BaseURL: config.AppConfig.AI.OpenRouter.BaseURL,
			APIKey:  config.AppConfig.AI.OpenRouter.Key,
			Model:   config.AppConfig.AI.OpenRouter.Model,
		},
	}

	var attemptErrors []string
	var lastRawDebug string // raw/partial content from the most recent failed attempt, for debugging
	raw, usedProvider, genErr := ai.GenerateWithFallback(ctx, providers, messages, aiSummaryProviderTimeout, func(provider string, err error) {
		attemptErrors = append(attemptErrors, fmt.Sprintf("%s: %v", provider, err))
		if truncated := extractTruncatedContent(err); truncated != "" {
			lastRawDebug = truncated
		}
	})

	now := time.Now()
	if genErr != nil {
		if len(attemptErrors) > 0 {
			fmt.Printf("AI summary generation for user %s: all providers failed: %s\n", userID, strings.Join(attemptErrors, "; "))
		}
		lastError := genErr.Error()
		if lastRawDebug != "" {
			lastError += "\n[raw provider output]\n" + truncateRunesLocal(lastRawDebug, aiSummaryRawDebugRunes)
		}
		failDoc := aiUserSummaryDoc{
			UserID:        userID,
			Status:        "failed",
			InputSnapshot: stats,
			LastError:     lastError,
			LastAttemptAt: &now,
		}
		upsertAIUserSummary(userID, failDoc)
		return nil, genErr
	}

	result, parseErr := ai.ParseAndSanitizeUserSummaryResult(raw, allowedIDs)
	if parseErr != nil {
		combinedErr := fmt.Errorf("provider %s returned an invalid response: %w", usedProvider.Name, parseErr)
		lastError := combinedErr.Error() + "\n[raw provider output]\n" + truncateRunesLocal(raw, aiSummaryRawDebugRunes)
		failDoc := aiUserSummaryDoc{
			UserID:        userID,
			Status:        "failed",
			InputSnapshot: stats,
			LastError:     lastError,
			LastAttemptAt: &now,
		}
		upsertAIUserSummary(userID, failDoc)
		return nil, combinedErr
	}

	expiresAt := now.Add(aiSummaryCacheTTL)
	readyDoc := aiUserSummaryDoc{
		UserID:        userID,
		Status:        "ready",
		Result:        result,
		Provider:      usedProvider.Name,
		Model:         usedProvider.Model,
		PromptVersion: ai.UserProfileSummaryPromptVersion,
		InputSnapshot: stats,
		GeneratedAt:   &now,
		ExpiresAt:     &expiresAt,
		LastAttemptAt: &now,
	}
	upsertAIUserSummary(userID, readyDoc)
	return &readyDoc, nil
}

// upsertAIUserSummary always uses its own bounded timeout (rather than the
// caller's context) so a cache write for a successful/failed generation
// still completes even if the request's own deadline is close to expiring.
func upsertAIUserSummary(userID string, doc aiUserSummaryDoc) {
	set := bson.M{
		"userId":        userID,
		"status":        doc.Status,
		"inputSnapshot": doc.InputSnapshot,
	}
	if doc.Result != nil {
		set["result"] = doc.Result
	}
	if doc.Provider != "" {
		set["provider"] = doc.Provider
	}
	if doc.Model != "" {
		set["model"] = doc.Model
	}
	if doc.PromptVersion != "" {
		set["promptVersion"] = doc.PromptVersion
	}
	if doc.GeneratedAt != nil {
		set["generatedAt"] = *doc.GeneratedAt
	}
	if doc.ExpiresAt != nil {
		set["expiresAt"] = *doc.ExpiresAt
	}
	if doc.LastAttemptAt != nil {
		set["lastAttemptAt"] = *doc.LastAttemptAt
	}
	// lastError is always set (cleared to "" on success) so a stale error
	// from a previous failed attempt doesn't linger after a later success.
	set["lastError"] = doc.LastError

	dbCtx, cancel := context.WithTimeout(context.Background(), aiSummaryDBTimeout)
	defer cancel()
	_, err := database.AIUserSummaries.UpdateOne(
		dbCtx,
		bson.M{"userId": userID},
		bson.M{"$set": set},
		options.Update().SetUpsert(true),
	)
	if err != nil {
		fmt.Printf("Warning: failed to upsert AI user summary for %s: %v\n", userID, err)
	}
}

// buildAIUserSummaryInput loads user stats and up to aiSummaryMaxComments
// highest-liked comments for userID, redacts them down to LLM-safe fields,
// and returns the stats, the samples, and the set of sample IDs the model is
// allowed to cite as evidence.
func buildAIUserSummaryInput(ctx context.Context, userID string) (aiUserSummaryInputStats, []aiCommentSample, map[string]bool, error) {
	stats := aiUserSummaryInputStats{}

	// Total comment count + choice breakdown across ALL of the user's
	// comments (not just the sampled top comments), so the stats block reflects
	// the full history even though only a sample of the text is sent.
	pipeline := mongo.Pipeline{
		{{Key: "$match", Value: bson.M{"userId": userID}}},
		{{Key: "$group", Value: bson.M{
			"_id":        "$choice",
			"count":      bson.M{"$sum": 1},
			"totalLikes": bson.M{"$sum": "$approveCount"},
		}}},
	}
	cursor, err := database.Comments.Aggregate(ctx, pipeline)
	if err != nil {
		return stats, nil, nil, fmt.Errorf("failed to aggregate comment stats: %w", err)
	}
	defer cursor.Close(ctx)
	for cursor.Next(ctx) {
		var row struct {
			ID         interface{} `bson:"_id"`
			Count      int64       `bson:"count"`
			TotalLikes int64       `bson:"totalLikes"`
		}
		if err := cursor.Decode(&row); err != nil {
			continue
		}
		stats.TotalCommentCount += row.Count
		stats.TotalLikes += row.TotalLikes
		switch toInt64(row.ID) {
		case 1:
			stats.SupportedDisplayCount = row.Count
		case 2:
			stats.UnsupportedDisplayCount = row.Count
		default:
			stats.UnknownChoiceCount += row.Count
		}
	}

	var rankingDoc struct {
		Rank int64 `bson:"rank"`
	}
	if err := database.UserRankings.FindOne(ctx, bson.M{"userId": userID}).Decode(&rankingDoc); err == nil {
		stats.LikesRank = rankingDoc.Rank
	}

	// Fetch the top-N highest-liked comments (the actual text sample sent to
	// the LLM), same ordering as GetUserComments/GetProblemComments plus a
	// commentId tiebreaker for a fully deterministic sample across runs.
	findOptions := options.Find().
		SetSort(bson.D{
			{Key: "approveCount", Value: -1},
			{Key: "createTime", Value: -1},
			{Key: "commentId", Value: 1},
		}).
		SetLimit(aiSummaryMaxComments)

	commentCursor, err := database.Comments.Find(ctx, bson.M{"userId": userID}, findOptions)
	if err != nil {
		return stats, nil, nil, fmt.Errorf("failed to find comments: %w", err)
	}
	defer commentCursor.Close(ctx)

	problemContextCache := map[string]string{}
	samples := make([]aiCommentSample, 0, aiSummaryMaxComments)
	allowedIDs := make(map[string]bool, aiSummaryMaxComments)

	remainingBudget := aiSummarySampleBudgetRunes

	for commentCursor.Next(ctx) {
		// Comments are sorted highest-liked-first (see findOptions above),
		// so once the budget runs dry we stop rather than spend it on the
		// long tail of lower-signal comments.
		if remainingBudget <= 0 {
			break
		}

		var doc bson.M
		if err := commentCursor.Decode(&doc); err != nil {
			continue
		}

		sampleID := fmt.Sprintf("c%d", len(samples)+1)
		sample := aiCommentSample{
			ID:     sampleID,
			Choice: int(toInt64(doc["choice"])),
		}
		if v, ok := doc["content"].(string); ok {
			sample.Content = truncateRunesLocal(strings.TrimSpace(v), aiSummaryCommentContentRunes)
		}
		sample.ApproveCount = toInt64(doc["approveCount"])
		sample.AudioText = truncateRunesLocal(extractAudioText(doc["audios"]), aiSummaryCommentContentRunes)

		if problemID, ok := doc["problemId"].(string); ok && problemID != "" {
			if cached, ok := problemContextCache[problemID]; ok {
				sample.ProblemContext = cached
			} else if len(problemContextCache) < aiSummaryMaxProblemLookups {
				fetched := fetchProblemContextBestEffort(problemID)
				problemContextCache[problemID] = fetched
				sample.ProblemContext = fetched
			}
		}

		if sample.Content == "" && sample.AudioText == "" {
			// Nothing useful to analyze (e.g. image-only comment); skip so
			// we don't waste input budget on an empty sample.
			continue
		}

		samples = append(samples, sample)
		allowedIDs[sampleID] = true
		remainingBudget -= utf8.RuneCountInString(sample.Content) +
			utf8.RuneCountInString(sample.AudioText) +
			utf8.RuneCountInString(sample.ProblemContext)
	}

	stats.SampledCommentCount = len(samples)

	return stats, samples, allowedIDs, nil
}

// fetchProblemContextBestEffort loads a short, redacted piece of case context
// (the user's review text, or the first textual evidence item) for the
// problem a comment belongs to. This is best-effort: any lookup failure or
// missing field simply results in an empty string rather than failing the
// whole summary generation. It uses its own short-lived context because
// problem lookups are opportunistic and must never block or fail the whole
// summary.
func fetchProblemContextBestEffort(problemID string) string {
	objID, err := primitive.ObjectIDFromHex(problemID)
	if err != nil {
		return ""
	}

	dbCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var doc bson.M
	err = database.Problems.FindOne(dbCtx, bson.M{"_id": objID}).Decode(&doc)
	if err != nil {
		return ""
	}

	detail, _ := doc["detail"].(bson.M)
	taskInfo, _ := detail["taskInfo"].(bson.M)
	voteContent, _ := taskInfo["voteContent"].(bson.M)

	if review, ok := voteContent["review"].(bson.M); ok {
		if content, ok := review["reviewContent"].(string); ok && strings.TrimSpace(content) != "" {
			return truncateRunesLocal(strings.TrimSpace(content), aiSummaryProblemContextRunes)
		}
	}

	if evidenceList, ok := taskInfo["evidenceList"].(bson.A); ok {
		for _, item := range evidenceList {
			itemMap, ok := item.(bson.M)
			if !ok {
				continue
			}
			if content, ok := itemMap["txtContent"].(string); ok && strings.TrimSpace(content) != "" {
				return truncateRunesLocal(strings.TrimSpace(content), aiSummaryProblemContextRunes)
			}
		}
	}

	return ""
}

func extractAudioText(v interface{}) string {
	audios := toCommentAudios(v)
	texts := make([]string, 0, len(audios))
	for _, audio := range audios {
		if text, ok := audio["audioText"].(string); ok && strings.TrimSpace(text) != "" {
			texts = append(texts, strings.TrimSpace(text))
		}
	}
	return strings.Join(texts, " / ")
}

// extractTruncatedContent unwraps a (possibly provider-wrapped) error to see
// if it's an ai.TruncatedResponseError, returning the partial content the
// model produced before hitting max_tokens, or "" if the error isn't that
// specific case.
func extractTruncatedContent(err error) string {
	var truncated *ai.TruncatedResponseError
	if errors.As(err, &truncated) {
		return truncated.Content
	}
	return ""
}

func truncateRunesLocal(s string, max int) string {
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max])
}

// renderUserPrompt builds the <user_stats>/<comment_samples> XML-ish wrapper
// the prompt expects, using compact JSON for each block.
func renderUserPrompt(stats aiUserSummaryInputStats, samples []aiCommentSample) string {
	// Sort samples defensively by approveCount desc so the highest-liked
	// comments are seen first by the model regardless of upstream ordering.
	sort.SliceStable(samples, func(i, j int) bool {
		return samples[i].ApproveCount > samples[j].ApproveCount
	})

	statsJSON := mustMarshalCompact(stats)
	samplesJSON := mustMarshalCompact(samples)

	var b strings.Builder
	b.WriteString("<user_stats>")
	b.WriteString(statsJSON)
	b.WriteString("</user_stats>\n<comment_samples>")
	b.WriteString(samplesJSON)
	b.WriteString("</comment_samples>")
	return b.String()
}

func mustMarshalCompact(v interface{}) string {
	b, err := json.Marshal(v)
	if err != nil {
		return "{}"
	}
	return string(b)
}
