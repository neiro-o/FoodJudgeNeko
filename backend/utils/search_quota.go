package utils

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"mtv2/backend/database"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Search quota model (replaces the old "1 point per search" billing):
//
//   - Every search that returns results consumes 1 unit from BOTH quotas.
//   - 1-hour quota: a fixed window that starts at the first consumed search
//     and ends 1 hour later; the next search after it ends starts a new one.
//   - Weekly quota: resets every Monday 00:00 SGT.
//   - Limits are derived from last week's score in weekly_scores (see
//     SearchQuotaLimitsForScore) and are evaluated on every request, so they
//     are never stored; only usage counters live in the search_quotas
//     collection (one document per account, _id = accounts._id).
//   - Admin and is_in_group accounts have no quota.
const (
	CtxKeySearchAccountID   = "search_account_id"
	CtxKeySearchUnlimited   = "search_unlimited"
	ctxKeySearchQuotaLimits = "search_quota_limits"
	ctxKeySearchDedupKey    = "search_dedup_key"

	// CodeSearchQuotaExceeded is returned when the 1-hour or weekly search
	// quota is used up.
	CodeSearchQuotaExceeded = 4291

	OneHourQuotaWindow = time.Hour

	searchDedupPrefix = "search_quota_dedup:v1:"
	// searchDedupTTL is how long an identical search by the same account is
	// treated as already paid for.
	searchDedupTTL = 2 * time.Minute
)

// SearchQuotaLimits are the per-window limits of one account.
// FiveHour names and five_hour BSON/JSON keys are retained for compatibility;
// they now represent the 1-hour quota.
type SearchQuotaLimits struct {
	FiveHour int
	Weekly   int
}

// SearchQuotaLimitsForScore maps last week's score to the quota limits.
func SearchQuotaLimitsForScore(score float64) SearchQuotaLimits {
	switch {
	case score >= 75:
		return SearchQuotaLimits{FiveHour: 75, Weekly: 1000}
	case score >= 50:
		return SearchQuotaLimits{FiveHour: 60, Weekly: 800}
	case score >= 40:
		return SearchQuotaLimits{FiveHour: 45, Weekly: 600}
	case score >= 20:
		return SearchQuotaLimits{FiveHour: 30, Weekly: 300}
	default:
		return SearchQuotaLimits{FiveHour: 15, Weekly: 180}
	}
}

// searchQuotaDoc mirrors a document in the search_quotas collection.
// Timestamps are unix seconds so they can be compared inside update pipelines.
type searchQuotaDoc struct {
	ID            primitive.ObjectID `bson:"_id"`
	FiveHourStart int64              `bson:"five_hour_start"` // 0 = no window started
	FiveHourUsed  int                `bson:"five_hour_used"`
	WeekStart     int64              `bson:"week_start"` // Monday 00:00 SGT
	WeekUsed      int                `bson:"week_used"`
	LastOK        bool               `bson:"last_ok"`
	UpdatedAt     time.Time          `bson:"updated_at"`
}

// SearchQuotaWindow describes one quota window in API responses.
type SearchQuotaWindow struct {
	Limit       int     `json:"limit"`
	Used        int     `json:"used"`
	Remaining   int     `json:"remaining"`
	StartedAt   *string `json:"started_at,omitempty"`
	ResetAt     *string `json:"reset_at"`
	ResetAtUnix *int64  `json:"reset_at_unix"`
}

// SearchQuotaStatus is the response of the quota query API.
type SearchQuotaStatus struct {
	Unlimited     bool               `json:"unlimited"`
	LastWeekScore float64            `json:"last_week_score"`
	FiveHour      *SearchQuotaWindow `json:"five_hour,omitempty"`
	Weekly        *SearchQuotaWindow `json:"weekly,omitempty"`
	// NextResetAt is the earliest upcoming reset of either window.
	NextResetAt     *string `json:"next_reset_at"`
	NextResetAtUnix *int64  `json:"next_reset_at_unix"`
	// Blocked is true when searching is currently forbidden; UnlockAt is
	// then the moment all exhausted windows have reset.
	Blocked      bool    `json:"blocked"`
	UnlockAt     *string `json:"unlock_at"`
	UnlockAtUnix *int64  `json:"unlock_at_unix"`
	// Exhausted windows ("five_hour" / "weekly"), only set when blocked.
	ExhaustedWindows []string `json:"exhausted_windows,omitempty"`
}

func formatQuotaTime(t time.Time) (*string, *int64) {
	s := t.In(SGTLocation()).Format(time.RFC3339)
	u := t.Unix()
	return &s, &u
}

func remaining(limit, used int) int {
	if used >= limit {
		return 0
	}
	return limit - used
}

// buildSearchQuotaStatus derives the effective status of an account from its
// stored counters (doc may be nil), applying window expiry/rollover as of now.
func buildSearchQuotaStatus(doc *searchQuotaDoc, limits SearchQuotaLimits, score float64, now time.Time) SearchQuotaStatus {
	var fiveUsed, weekUsed int
	var fiveStart time.Time
	fiveActive := false
	weekStart := WeekStartSGT(now)
	weekReset := weekStart.AddDate(0, 0, 7)

	if doc != nil {
		if doc.FiveHourStart > 0 && now.Unix() < doc.FiveHourStart+int64(OneHourQuotaWindow/time.Second) {
			fiveActive = true
			fiveStart = time.Unix(doc.FiveHourStart, 0)
			fiveUsed = doc.FiveHourUsed
		}
		if doc.WeekStart == weekStart.Unix() {
			weekUsed = doc.WeekUsed
		}
	}

	five := &SearchQuotaWindow{Limit: limits.FiveHour, Used: fiveUsed, Remaining: remaining(limits.FiveHour, fiveUsed)}
	week := &SearchQuotaWindow{Limit: limits.Weekly, Used: weekUsed, Remaining: remaining(limits.Weekly, weekUsed)}
	week.ResetAt, week.ResetAtUnix = formatQuotaTime(weekReset)

	next := weekReset
	var fiveReset time.Time
	if fiveActive {
		fiveReset = fiveStart.Add(OneHourQuotaWindow)
		five.StartedAt, _ = formatQuotaTime(fiveStart)
		five.ResetAt, five.ResetAtUnix = formatQuotaTime(fiveReset)
		if fiveReset.Before(next) {
			next = fiveReset
		}
	}

	status := SearchQuotaStatus{LastWeekScore: score, FiveHour: five, Weekly: week}
	status.NextResetAt, status.NextResetAtUnix = formatQuotaTime(next)

	var unlock time.Time
	if five.Remaining == 0 {
		status.ExhaustedWindows = append(status.ExhaustedWindows, "five_hour")
		if fiveActive {
			unlock = fiveReset
		}
	}
	if week.Remaining == 0 {
		status.ExhaustedWindows = append(status.ExhaustedWindows, "weekly")
		if weekReset.After(unlock) {
			unlock = weekReset
		}
	}
	if len(status.ExhaustedWindows) > 0 {
		status.Blocked = true
		if unlock.IsZero() {
			// A zero limit with no active window never unlocks by waiting
			// for the 1-hour window; fall back to the weekly reset.
			unlock = weekReset
		}
		status.UnlockAt, status.UnlockAtUnix = formatQuotaTime(unlock)
	}
	return status
}

// lastWeekScore returns the account's score in the previous ISO week
// (0 when there is no weekly_scores row).
func lastWeekScore(ctx context.Context, accountID primitive.ObjectID, now time.Time) (float64, error) {
	year, week := now.In(SGTLocation()).AddDate(0, 0, -7).ISOWeek()
	var row struct {
		Score float64 `bson:"score"`
	}
	err := database.WeeklyScores.FindOne(ctx, bson.M{
		"userId": accountID,
		"year":   year,
		"weekId": week,
	}).Decode(&row)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	return row.Score, nil
}

// GetSearchQuotaStatus reads the current quota status without consuming any.
func GetSearchQuotaStatus(ctx context.Context, acct SearchAccount) (SearchQuotaStatus, SearchQuotaLimits, error) {
	if acct.Unlimited {
		return SearchQuotaStatus{Unlimited: true}, SearchQuotaLimits{}, nil
	}
	now := time.Now()
	score, err := lastWeekScore(ctx, acct.ID, now)
	if err != nil {
		return SearchQuotaStatus{}, SearchQuotaLimits{}, fmt.Errorf("load last week score: %w", err)
	}
	limits := SearchQuotaLimitsForScore(score)

	var doc searchQuotaDoc
	var docPtr *searchQuotaDoc
	err = database.SearchQuotas.FindOne(ctx, bson.M{"_id": acct.ID}).Decode(&doc)
	switch {
	case err == nil:
		docPtr = &doc
	case errors.Is(err, mongo.ErrNoDocuments):
	default:
		return SearchQuotaStatus{}, SearchQuotaLimits{}, fmt.Errorf("load search quota: %w", err)
	}
	return buildSearchQuotaStatus(docPtr, limits, score, now), limits, nil
}

// consumeSearchQuota atomically rolls expired windows over and, if both
// quotas have room, consumes one unit from each. It returns whether a unit
// was consumed.
func consumeSearchQuota(ctx context.Context, accountID primitive.ObjectID, limits SearchQuotaLimits, now time.Time) (bool, error) {
	nowUnix := now.Unix()
	weekStart := WeekStartSGT(now).Unix()
	fiveSecs := int64(OneHourQuotaWindow / time.Second)

	// Pipeline stages are separate so each one sees the previous one's output.
	pipeline := mongo.Pipeline{
		// 1. Which windows are still current?
		{{Key: "$set", Value: bson.M{
			"five_hour_active": bson.M{"$and": bson.A{
				bson.M{"$gt": bson.A{bson.M{"$ifNull": bson.A{"$five_hour_start", 0}}, 0}},
				bson.M{"$lt": bson.A{nowUnix, bson.M{"$add": bson.A{bson.M{"$ifNull": bson.A{"$five_hour_start", 0}}, fiveSecs}}}},
			}},
			"week_active": bson.M{"$eq": bson.A{bson.M{"$ifNull": bson.A{"$week_start", 0}}, weekStart}},
		}}},
		// 2. Reset the windows that rolled over.
		{{Key: "$set", Value: bson.M{
			"five_hour_start": bson.M{"$cond": bson.A{"$five_hour_active", "$five_hour_start", 0}},
			"five_hour_used":  bson.M{"$cond": bson.A{"$five_hour_active", bson.M{"$ifNull": bson.A{"$five_hour_used", 0}}, 0}},
			"week_start":      weekStart,
			"week_used":       bson.M{"$cond": bson.A{"$week_active", bson.M{"$ifNull": bson.A{"$week_used", 0}}, 0}},
		}}},
		// 3. Is there room in both?
		{{Key: "$set", Value: bson.M{
			"last_ok": bson.M{"$and": bson.A{
				bson.M{"$lt": bson.A{"$five_hour_used", limits.FiveHour}},
				bson.M{"$lt": bson.A{"$week_used", limits.Weekly}},
			}},
		}}},
		// 4. Consume (starting the 1-hour window on its first use).
		{{Key: "$set", Value: bson.M{
			"five_hour_start": bson.M{"$cond": bson.A{
				bson.M{"$and": bson.A{"$last_ok", bson.M{"$eq": bson.A{"$five_hour_start", 0}}}},
				nowUnix,
				"$five_hour_start",
			}},
			"five_hour_used": bson.M{"$add": bson.A{"$five_hour_used", bson.M{"$cond": bson.A{"$last_ok", 1, 0}}}},
			"week_used":      bson.M{"$add": bson.A{"$week_used", bson.M{"$cond": bson.A{"$last_ok", 1, 0}}}},
			"updated_at":     "$$NOW",
		}}},
		{{Key: "$unset", Value: bson.A{"five_hour_active", "week_active"}}},
	}

	opts := options.FindOneAndUpdate().SetUpsert(true).SetReturnDocument(options.After)
	var doc searchQuotaDoc
	var err error
	// Two concurrent first-ever upserts can collide on _id; the retry then
	// takes the plain update path.
	for attempt := 0; attempt < 2; attempt++ {
		err = database.SearchQuotas.FindOneAndUpdate(ctx, bson.M{"_id": accountID}, pipeline, opts).Decode(&doc)
		if err == nil || !mongo.IsDuplicateKeyError(err) {
			break
		}
	}
	if err != nil {
		return false, err
	}
	return doc.LastOK, nil
}

// QuotaExceededResponse tells the client searching is forbidden until the
// exhausted quota windows reset.
func QuotaExceededResponse(c *gin.Context, status SearchQuotaStatus) {
	message := "查题额度已用完，请稍后再试。"
	if status.UnlockAt != nil && status.UnlockAtUnix != nil {
		label := "1小时额度"
		if len(status.ExhaustedWindows) == 1 && status.ExhaustedWindows[0] == "weekly" {
			label = "周额度"
		} else if len(status.ExhaustedWindows) > 1 {
			label = "1小时额度和周额度"
		}
		message = fmt.Sprintf("%s已用完，禁止查题，将于 %s (SGT) 恢复。",
			label, time.Unix(*status.UnlockAtUnix, 0).In(SGTLocation()).Format("2006-01-02 15:04"))
	}
	c.JSON(http.StatusOK, Response{Code: CodeSearchQuotaExceeded, Message: message, Data: status})
}

// CheckSearchQuota is the pre-search gate used by the search middleware. It
// stashes the billed account on the context for FinalizeSearchBilling, and
// sends QuotaExceededResponse (returning false) when the account is blocked.
// A search identical to one this account already paid for moments ago is let
// through even if the quota has just run out.
func CheckSearchQuota(c *gin.Context, acct SearchAccount) bool {
	c.Set(CtxKeySearchAccountID, acct.ID)
	c.Set(CtxKeySearchUnlimited, acct.Unlimited)
	if acct.Unlimited {
		return true
	}

	dedupKey := searchDedupKey(c, acct.ID)
	c.Set(ctxKeySearchDedupKey, dedupKey)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if database.RedisClient != nil {
		if n, err := database.RedisClient.Exists(ctx, dedupKey).Result(); err == nil && n > 0 {
			return true
		}
	}

	status, limits, err := GetSearchQuotaStatus(ctx, acct)
	if err != nil {
		InternalServerErrorResponse(c, "Failed to check search quota")
		return false
	}
	c.Set(ctxKeySearchQuotaLimits, limits)
	if status.Blocked {
		QuotaExceededResponse(c, status)
		return false
	}
	return true
}

// FinalizeSearchBilling consumes one unit of the 1-hour and weekly quotas
// after a search with results. Empty results and failed searches consume
// nothing, and neither does repeating an identical search shortly after one
// that was already charged. It sends an error response and returns false if
// the quota cannot be consumed.
func FinalizeSearchBilling(c *gin.Context, total int64) bool {
	if total <= 0 {
		return true
	}

	unlimitedVal, exists := c.Get(CtxKeySearchUnlimited)
	if !exists {
		InternalServerErrorResponse(c, "Search account context missing")
		return false
	}
	if unlimited, _ := unlimitedVal.(bool); unlimited {
		return true
	}

	accountIDVal, exists := c.Get(CtxKeySearchAccountID)
	accountID, ok := accountIDVal.(primitive.ObjectID)
	if !exists || !ok {
		InternalServerErrorResponse(c, "Search account context missing")
		return false
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Claim the dedup key; losing the claim means an identical search has
	// already been (or is being) charged, so this one is free.
	dedupKey, _ := c.Get(ctxKeySearchDedupKey)
	dedupKeyStr, _ := dedupKey.(string)
	claimed := false
	if dedupKeyStr != "" && database.RedisClient != nil {
		set, err := database.RedisClient.SetNX(ctx, dedupKeyStr, 1, searchDedupTTL).Result()
		if err == nil {
			if !set {
				return true
			}
			claimed = true
		}
	}
	release := func() {
		if claimed {
			database.RedisClient.Del(ctx, dedupKeyStr)
		}
	}

	limits, ok := func() (SearchQuotaLimits, bool) {
		if v, exists := c.Get(ctxKeySearchQuotaLimits); exists {
			if l, ok := v.(SearchQuotaLimits); ok {
				return l, true
			}
		}
		score, err := lastWeekScore(ctx, accountID, time.Now())
		if err != nil {
			return SearchQuotaLimits{}, false
		}
		return SearchQuotaLimitsForScore(score), true
	}()
	if !ok {
		release()
		InternalServerErrorResponse(c, "Failed to load search quota limits")
		return false
	}

	consumed, err := consumeSearchQuota(ctx, accountID, limits, time.Now())
	if err != nil {
		release()
		InternalServerErrorResponse(c, "Failed to record search quota usage")
		return false
	}
	if !consumed {
		release()
		status, _, err := GetSearchQuotaStatus(ctx, SearchAccount{ID: accountID})
		if err != nil {
			InternalServerErrorResponse(c, "Search quota exceeded")
			return false
		}
		QuotaExceededResponse(c, status)
		return false
	}
	return true
}
