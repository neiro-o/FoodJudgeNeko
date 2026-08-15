package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"mtv2/backend/config"
	"mtv2/backend/database"
	"mtv2/backend/utils"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const (
	vanishedBatchSize       = 500
	vanishedMinApproveCount = 35
	problemShareURL         = "https://zqt.meituan.com/xiaomei/vote/jury/api/r/rediectByScene"
)

var vanishedSGT = time.FixedZone("SGT", 8*60*60)

// VanishedComment is both the API item shape and the frontend CSV column shape.
type VanishedComment struct {
	ProblemID  string `json:"problemId"`
	UserReview string `json:"user_review"`
	Timestamp  string `json:"timestamp"`
	CommentID  string `json:"commentId"`
	Content    string `json:"content"`
	Likes      int64  `json:"likes"`
	CreateTime string `json:"createTime"`
	URL        string `json:"url"`
}

type vanishedCommentCandidate struct {
	ProblemID  string
	CommentID  string
	Content    string
	Likes      int64
	CreateTime string
}

type vanishedESFields struct {
	UserReview string
	Timestamp  int64
}

func normalizeVanishedUserID(raw string) (string, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", false
	}
	for _, char := range raw {
		if char < '0' || char > '9' {
			return "", false
		}
	}
	normalized := strings.TrimLeft(raw, "0")
	if normalized == "" {
		normalized = "0"
	}
	return normalized, true
}

func formatVanishedTimeSGT(value interface{}) string {
	var parsed time.Time
	switch val := value.(type) {
	case time.Time:
		parsed = val
	case primitive.DateTime:
		parsed = val.Time()
	case int:
		parsed = unixVanishedTime(int64(val))
	case int32:
		parsed = unixVanishedTime(int64(val))
	case int64:
		parsed = unixVanishedTime(val)
	case float64:
		parsed = unixVanishedTime(int64(val))
	case json.Number:
		timestamp, err := val.Int64()
		if err != nil {
			return ""
		}
		parsed = unixVanishedTime(timestamp)
	case string:
		timestamp, err := strconv.ParseInt(strings.TrimSpace(val), 10, 64)
		if err != nil {
			return ""
		}
		parsed = unixVanishedTime(timestamp)
	default:
		return ""
	}
	return parsed.In(vanishedSGT).Format("2006-01-02 15:04:05-07:00")
}

func unixVanishedTime(timestamp int64) time.Time {
	if math.Abs(float64(timestamp)) >= 100_000_000_000 {
		return time.UnixMilli(timestamp)
	}
	return time.Unix(timestamp, 0)
}

func vanishedArray(value interface{}) []interface{} {
	switch values := value.(type) {
	case bson.A:
		return []interface{}(values)
	case []interface{}:
		return values
	default:
		return nil
	}
}

func vanishedMap(value interface{}) map[string]interface{} {
	switch item := value.(type) {
	case bson.M:
		return map[string]interface{}(item)
	case map[string]interface{}:
		return item
	default:
		return nil
	}
}

func problemCommentIDs(problem bson.M) map[string]struct{} {
	result := make(map[string]struct{})
	for _, pageValue := range vanishedArray(problem["comment"]) {
		page := vanishedMap(pageValue)
		if page == nil {
			continue
		}
		for _, commentValue := range vanishedArray(page["pageContent"]) {
			comment := vanishedMap(commentValue)
			if comment == nil || comment["commentId"] == nil {
				continue
			}
			result[fmt.Sprint(comment["commentId"])] = struct{}{}
		}
	}
	return result
}

func buildProblemShareURL(userID, taskID interface{}) string {
	return problemShareURL +
		"?jumpScene=mockTaskShare" +
		"&userId=" + url.QueryEscape(vanishedString(userID)) +
		"&channel=mockTaskShare" +
		"&encryptMockTaskNo=" + url.QueryEscape(vanishedString(taskID))
}

func vanishedString(value interface{}) string {
	if value == nil {
		return ""
	}
	return fmt.Sprint(value)
}

func vanishedInt64(value interface{}) int64 {
	if number, ok := value.(json.Number); ok {
		if parsed, err := number.Int64(); err == nil {
			return parsed
		}
		if parsed, err := strconv.ParseFloat(number.String(), 64); err == nil {
			return int64(parsed)
		}
	}
	return toInt64Value(value)
}

func vanishedESFieldsFromSource(source map[string]interface{}) vanishedESFields {
	userReview, _ := source["user_review"].(string)
	if strings.TrimSpace(userReview) == "" {
		appeals := vanishedArray(source["appeals"])
		if len(appeals) > 0 {
			firstAppeal := vanishedMap(appeals[0])
			if content, ok := firstAppeal["content"].(string); ok {
				userReview = content
			}
		}
	}
	return vanishedESFields{
		UserReview: userReview,
		Timestamp:  vanishedInt64(source["timestamp"]),
	}
}

func fetchVanishedESFields(
	ctx context.Context,
	problemIDs []string,
) (map[string]vanishedESFields, error) {
	result := make(map[string]vanishedESFields, len(problemIDs))
	for start := 0; start < len(problemIDs); start += vanishedBatchSize {
		end := start + vanishedBatchSize
		if end > len(problemIDs) {
			end = len(problemIDs)
		}

		query := map[string]interface{}{
			"query": map[string]interface{}{
				"terms": map[string]interface{}{"mongo_id": problemIDs[start:end]},
			},
			"_source": []string{"mongo_id", "user_review", "timestamp", "appeals.content"},
			"size":    end - start,
		}
		queryJSON, err := json.Marshal(query)
		if err != nil {
			return nil, fmt.Errorf("failed to build Elasticsearch query: %w", err)
		}

		response, err := database.ESClient.Search(
			database.ESClient.Search.WithContext(ctx),
			database.ESClient.Search.WithIndex(config.AppConfig.Elasticsearch.IndexName),
			database.ESClient.Search.WithBody(bytes.NewReader(queryJSON)),
		)
		if err != nil {
			return nil, fmt.Errorf("Elasticsearch search failed: %w", err)
		}

		if response.IsError() {
			var errorBody map[string]interface{}
			decodeErr := json.NewDecoder(response.Body).Decode(&errorBody)
			response.Body.Close()
			if decodeErr != nil {
				return nil, fmt.Errorf("Elasticsearch returned %s", response.Status())
			}
			return nil, fmt.Errorf("Elasticsearch error: %v", errorBody["error"])
		}

		decoder := json.NewDecoder(response.Body)
		decoder.UseNumber()
		var body map[string]interface{}
		decodeErr := decoder.Decode(&body)
		response.Body.Close()
		if decodeErr != nil {
			return nil, fmt.Errorf("failed to decode Elasticsearch response: %w", decodeErr)
		}

		hits := vanishedMap(body["hits"])
		for _, hitValue := range vanishedArray(hits["hits"]) {
			hit := vanishedMap(hitValue)
			source := vanishedMap(hit["_source"])
			mongoID, _ := source["mongo_id"].(string)
			if mongoID == "" {
				continue
			}
			result[mongoID] = vanishedESFieldsFromSource(source)
		}
	}
	return result, nil
}

// GetVanishedComments returns approveCount > 35 comments that disappeared
// from an existing problem's comment[].pageContent[] array.
// GET /api/user_detail/vanished/:userId
func GetVanishedComments(c *gin.Context) {
	userID, ok := normalizeVanishedUserID(c.Param("userId"))
	if !ok {
		utils.BadRequestResponse(c, "Invalid userId: must be an integer")
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	commentCursor, err := database.Comments.Find(
		ctx,
		bson.M{"userId": userID, "approveCount": bson.M{"$gt": vanishedMinApproveCount}},
		options.Find().SetProjection(bson.M{
			"_id": 0, "problemId": 1, "commentId": 1,
			"content": 1, "approveCount": 1, "createTime": 1,
		}),
	)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to find high-liked comments")
		return
	}
	defer commentCursor.Close(ctx)

	candidatesByProblem := make(map[primitive.ObjectID][]vanishedCommentCandidate)
	for commentCursor.Next(ctx) {
		var comment bson.M
		if err := commentCursor.Decode(&comment); err != nil {
			continue
		}
		problemID, _ := comment["problemId"].(string)
		objectID, err := primitive.ObjectIDFromHex(problemID)
		if err != nil {
			continue
		}
		commentID := vanishedString(comment["commentId"])
		content, _ := comment["content"].(string)
		candidatesByProblem[objectID] = append(candidatesByProblem[objectID], vanishedCommentCandidate{
			ProblemID:  problemID,
			CommentID:  commentID,
			Content:    content,
			Likes:      toInt64Value(comment["approveCount"]),
			CreateTime: formatVanishedTimeSGT(comment["createTime"]),
		})
	}
	if err := commentCursor.Err(); err != nil {
		utils.InternalServerErrorResponse(c, "Failed to read high-liked comments")
		return
	}

	problemIDs := make([]primitive.ObjectID, 0, len(candidatesByProblem))
	for problemID := range candidatesByProblem {
		problemIDs = append(problemIDs, problemID)
	}

	rows := make([]VanishedComment, 0)
	uniqueVanishedProblems := make(map[string]struct{})
	for start := 0; start < len(problemIDs); start += vanishedBatchSize {
		end := start + vanishedBatchSize
		if end > len(problemIDs) {
			end = len(problemIDs)
		}
		problemCursor, err := database.Problems.Find(
			ctx,
			bson.M{"_id": bson.M{"$in": problemIDs[start:end]}},
			options.Find().SetProjection(bson.M{
				"userId": 1, "taskId": 1, "comment.pageContent.commentId": 1,
			}),
		)
		if err != nil {
			utils.InternalServerErrorResponse(c, "Failed to find related problems")
			return
		}

		for problemCursor.Next(ctx) {
			var problem bson.M
			if err := problemCursor.Decode(&problem); err != nil {
				continue
			}
			problemID, ok := problem["_id"].(primitive.ObjectID)
			if !ok {
				continue
			}
			existingCommentIDs := problemCommentIDs(problem)
			for _, candidate := range candidatesByProblem[problemID] {
				if _, exists := existingCommentIDs[candidate.CommentID]; exists {
					continue
				}
				rows = append(rows, VanishedComment{
					ProblemID:  candidate.ProblemID,
					CommentID:  candidate.CommentID,
					Content:    candidate.Content,
					Likes:      candidate.Likes,
					CreateTime: candidate.CreateTime,
					URL:        buildProblemShareURL(problem["userId"], problem["taskId"]),
				})
				uniqueVanishedProblems[candidate.ProblemID] = struct{}{}
			}
		}
		cursorErr := problemCursor.Err()
		problemCursor.Close(ctx)
		if cursorErr != nil {
			utils.InternalServerErrorResponse(c, "Failed to read related problems")
			return
		}
	}

	if len(rows) > 0 {
		esProblemIDs := make([]string, 0, len(uniqueVanishedProblems))
		for problemID := range uniqueVanishedProblems {
			esProblemIDs = append(esProblemIDs, problemID)
		}
		esFields, err := fetchVanishedESFields(ctx, esProblemIDs)
		if err != nil {
			utils.InternalServerErrorResponse(c, "Failed to find related Elasticsearch documents")
			return
		}
		for index := range rows {
			if fields, exists := esFields[rows[index].ProblemID]; exists {
				rows[index].UserReview = fields.UserReview
				rows[index].Timestamp = formatVanishedTimeSGT(fields.Timestamp)
			}
		}
	}

	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].Likes != rows[j].Likes {
			return rows[i].Likes > rows[j].Likes
		}
		if rows[i].ProblemID != rows[j].ProblemID {
			return rows[i].ProblemID < rows[j].ProblemID
		}
		return rows[i].CommentID < rows[j].CommentID
	})
	utils.SuccessResponse(c, rows)
}
