package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"time"
	"unicode/utf8"

	"mtv2/backend/config"
	"mtv2/backend/database"
	"mtv2/backend/utils"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
)

type SearchRequest struct {
	Keyword string `form:"keyword" json:"keyword" binding:"required"`
	Limit   int    `form:"limit" json:"limit" binding:"required,min=5,max=30"`
}

type RecentProblemsRequest struct {
	Limit int `form:"limit" json:"limit" binding:"omitempty,min=5,max=20"`
}

// isSingleCharacter checks if the keyword is a single character (handles Unicode properly)
func isSingleCharacter(keyword string) bool {
	return utf8.RuneCountInString(keyword) == 1
}

// escapeWildcard escapes special wildcard characters for Elasticsearch wildcard query
func escapeWildcard(s string) string {
	// Escape special characters: *, ?, \
	re := regexp.MustCompile(`([*?\\])`)
	return re.ReplaceAllString(s, `\$1`)
}

// calculateFuzziness calculates fuzziness based on keyword length
// - 1-4 chars: fuzziness = 0
// - 5-8 chars: fuzziness = 1
// - 9+ chars: fuzziness = 2 (capped at 2, Elasticsearch max is 2)
func calculateFuzziness(keyword string) int {
	length := len(keyword)
	if length <= 4 {
		return 0
	} else if length <= 8 {
		return 1
	}
	return 2 // Cap at 2 - Elasticsearch only allows 0, 1, or 2
}

// calculateMinScore calculates minimum score threshold based on keyword length
// Shorter keywords need higher thresholds to avoid random matches
// Longer keywords are more specific, so lower thresholds are acceptable
func calculateMinScore(keyword string) float64 {
	length := len(keyword)
	if length <= 3 {
		return 20.0 // Very short: strict matching needed
	} else if length <= 6 {
		return 15.0 // Short: high threshold
	} else if length <= 10 {
		return 10.0 // Medium: moderate threshold
	} else if length <= 15 {
		return 5.0 // Long: lower threshold
	}
	return 2.0 // Very long: very low threshold, these are specific queries
}

type SearchResponse struct {
	Total   int64        `json:"total"`
	Results []ESDocument `json:"results"`
}

// ESDocument represents a simplified search result with only required fields
type ESDocument struct {
	ESID          interface{}         `json:"id"`                   // Elasticsearch unique identifier (_id)
	MongoID       string              `json:"mongo_id"`             // MongoDB unique identifier
	UserReview    string              `json:"user_review"`          // user's text review
	Timestamp     int64               `json:"timestamp"`            // user review timestamp in seconds
	Answer        int                 `json:"answer"`               // review result: 1-support user, 2-support merchant
	Hot1Answer    *int                `json:"hot1_answer"`          // choice from comments[0].choice (nullable)
	CommentAnswer *int                `json:"comment_answer"`       // most selected choice from comments (excluding last if even length)
	Ratio1        float64             `json:"ratio_1"`              // ratio of choice 1 (0~100)
	Ratio2        float64             `json:"ratio_2"`              // ratio of choice 2 (0~100)
	Score         float64             `json:"_score"`               // Elasticsearch relevance score
	Highlight     map[string][]string `json:"_highlight,omitempty"` // search highlights
}

// FullESDocument is the full document structure (kept for other functions)
type FullESDocument struct {
	ID          interface{}         `json:"id"`                     // unique identifier for the review record (string or number)
	MongoID     string              `json:"mongo_id"`               // MongoDB unique identifier
	Stars       int                 `json:"stars"`                  // user rating, 1-5, 1 is worst, 5 is best
	UserReview  string              `json:"user_review"`            // user's text review, may be empty for refund problems
	ReviewPics  []string            `json:"review_pics"`            // array of review image URLs
	Timestamp   int64               `json:"timestamp"`              // user review timestamp in seconds
	Others      string              `json:"others"`                 // additional notes, e.g., "消费后评价"
	ProblemType int                 `json:"problem_type"`           // 1-delivery, 2-dine-in, 3-delivery refund, 4-dine-in refund, 5-other
	Answer      int                 `json:"answer"`                 // review result: 1-support user, 2-support merchant
	Ratio1      float64             `json:"ratio_1"`                // ratio of choice 1 (0~100)
	Ratio2      float64             `json:"ratio_2"`                // ratio of choice 2 (0~100)
	Uploader    string              `json:"uploader"`               // ID of user who uploaded this problem
	TaskID      string              `json:"taskId"`                 // associated task ID
	UserID      string              `json:"userId"`                 // original link's userID parameter
	CreatedAt   int64               `json:"created_at"`             // creation timestamp in seconds
	Replies     []Reply             `json:"replies,omitempty"`      // timeline of merchant/user replies
	Appeals     []Appeal            `json:"appeals,omitempty"`      // appeals from user/merchant
	OrderInfo   interface{}         `json:"order_info,omitempty"`   // order info for non-delivery problems (variable dict)
	Orders      []Order             `json:"orders,omitempty"`       // order items
	OrderDetail *OrderDetail        `json:"order_detail,omitempty"` // order detail for delivery scenarios
	Comments    []Comment           `json:"comments,omitempty"`     // evaluation comments
	Score       float64             `json:"_score"`                 // Elasticsearch relevance score
	Highlight   map[string][]string `json:"_highlight,omitempty"`   // search highlights
}

type Reply struct {
	Role      string `json:"role"`      // merchant, user, or others_X
	Timestamp int64  `json:"timestamp"` // reply timestamp in seconds
	Content   string `json:"content"`   // reply content
}

type Appeal struct {
	Role      string   `json:"role"`      // merchant, user, or others_X
	Timestamp int64    `json:"timestamp"` // appeal submission timestamp in seconds
	Content   string   `json:"content"`   // appeal text description
	Pics      []string `json:"pics"`      // array of evidence image URLs
}

type Order struct {
	Name      string   `json:"name"`      // product name
	Count     int      `json:"count"`     // purchase quantity
	Desc      string   `json:"desc"`      // product description
	Selection []string `json:"selection"` // user-selected specifications or flavors
	Pic       string   `json:"pic"`       // product image URL
	Others    string   `json:"others"`    // other additional information or notes
}

type OrderDetail struct {
	OrderStarted  int64  `json:"order_started"`  // order placement timestamp (seconds)
	OrderFinished int64  `json:"order_finished"` // completion timestamp (seconds), 0 for pickup
	DeliverTime   int64  `json:"deliver_time"`   // delivery time in seconds, -1 for unavailable
	TotalTime     int64  `json:"total_time"`     // total time from order to completion in seconds
	DeliverBy     string `json:"deliver_by"`     // delivery party: merchant, meituan, or user
	Note          string `json:"note"`           // user notes
	Utensils      int    `json:"utensils"`       // utensils count: 0 for eco-friendly, -1 for on-demand, other for specified
	Invoice       bool   `json:"invoice"`        // whether invoice was issued
}

type Comment struct {
	UserID    int64  `json:"userid"`    // comment user ID
	Name      string `json:"name"`      // comment user nickname
	Content   string `json:"content"`   // comment content
	Timestamp int64  `json:"timestamp"` // comment timestamp in seconds
	Choice    int    `json:"choice"`    // comment choice: 1 for support user, 2 for support merchant
	Likes     int    `json:"likes"`     // number of likes
}

// extractRandomKeyword checks if "rand" or "random" appears as a whole word (prefix or suffix)
// Returns (cleaned keyword, isRandom)
func extractRandomKeyword(keyword string) (string, bool) {
	// Use word boundary regex to match whole words
	randRegex := regexp.MustCompile(`(?i)^\s*(rand|random)\s+|\s+(rand|random)\s*$`)

	// Check if rand/random appears as whole word
	if randRegex.MatchString(keyword) {
		// Remove rand/random from keyword
		cleaned := randRegex.ReplaceAllString(keyword, " ")
		cleaned = regexp.MustCompile(`\s+`).ReplaceAllString(cleaned, " ")
		cleaned = regexp.MustCompile(`^\s+|\s+$`).ReplaceAllString(cleaned, "")
		return cleaned, true
	}
	return keyword, false
}

// executeESQuery executes an Elasticsearch query and returns parsed results
func executeESQuery(ctx context.Context, query map[string]interface{}, limit int) ([]ESDocument, int64, error) {
	query["size"] = limit

	queryJSON, err := json.Marshal(query)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to build search query: %v", err)
	}

	indexName := config.AppConfig.Elasticsearch.IndexName
	res, err := database.ESClient.Search(
		database.ESClient.Search.WithContext(ctx),
		database.ESClient.Search.WithIndex(indexName),
		database.ESClient.Search.WithBody(bytes.NewReader(queryJSON)),
		database.ESClient.Search.WithTrackTotalHits(true),
	)
	if err != nil {
		return nil, 0, fmt.Errorf("Elasticsearch search failed: %v", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		var e map[string]interface{}
		if err := json.NewDecoder(res.Body).Decode(&e); err != nil {
			return nil, 0, fmt.Errorf("error parsing error response: %v", err)
		}
		return nil, 0, fmt.Errorf("Elasticsearch error: %v", e["error"])
	}

	var result map[string]interface{}
	if err := json.NewDecoder(res.Body).Decode(&result); err != nil {
		return nil, 0, fmt.Errorf("failed to parse search results: %v", err)
	}

	hits, ok := result["hits"].(map[string]interface{})
	if !ok {
		return nil, 0, fmt.Errorf("invalid search response format")
	}

	total, _ := hits["total"].(map[string]interface{})
	totalValue, _ := total["value"].(float64)

	hitsList, ok := hits["hits"].([]interface{})
	if !ok {
		return nil, 0, fmt.Errorf("invalid hits format")
	}

	results := make([]ESDocument, 0, len(hitsList))
	for _, hit := range hitsList {
		hitMap, ok := hit.(map[string]interface{})
		if !ok {
			continue
		}

		source, ok := hitMap["_source"].(map[string]interface{})
		if !ok {
			continue
		}

		score, _ := hitMap["_score"].(float64)

		doc := ESDocument{
			Score: score,
		}

		if esID, ok := hitMap["_id"]; ok {
			doc.ESID = esID
		}

		if highlight, ok := hitMap["highlight"].(map[string]interface{}); ok {
			doc.Highlight = make(map[string][]string)
			for k, v := range highlight {
				if arr, ok := v.([]interface{}); ok {
					strs := make([]string, len(arr))
					for i, item := range arr {
						if str, ok := item.(string); ok {
							strs[i] = str
						}
					}
					doc.Highlight[k] = strs
				}
			}
		}

		if mongoID, ok := source["mongo_id"].(string); ok {
			doc.MongoID = mongoID
		}
		if userReview, ok := source["user_review"].(string); ok {
			doc.UserReview = userReview
		}
		if timestamp, ok := source["timestamp"].(float64); ok {
			doc.Timestamp = int64(timestamp)
		}
		if answer, ok := source["answer"].(float64); ok {
			doc.Answer = int(answer)
		}
		if ratio1, ok := source["ratio_1"].(float64); ok {
			doc.Ratio1 = ratio1
		}
		if ratio2, ok := source["ratio_2"].(float64); ok {
			doc.Ratio2 = ratio2
		}

		if comments, ok := source["comments"].([]interface{}); ok {
			parsedComments := parseComments(comments)
			if len(parsedComments) > 0 {
				hot1Answer := parsedComments[0].Choice
				doc.Hot1Answer = &hot1Answer
			}
			doc.CommentAnswer = calculateCommentAnswer(parsedComments)
		}

		results = append(results, doc)
	}

	return results, int64(totalValue), nil
}

// SearchByRatio1 searches ES documents where ratio_1 == ratio
// orderByRandom: if true, order by random; if false, order by created_at desc
func SearchByRatio1(ctx context.Context, ratio float64, limit int, orderByRandom bool) ([]ESDocument, int64, error) {
	query := map[string]interface{}{
		"query": map[string]interface{}{
			"term": map[string]interface{}{
				"ratio_1": ratio,
			},
		},
	}

	if orderByRandom {
		// Use random_score for random ordering
		query["sort"] = []map[string]interface{}{
			{
				"_script": map[string]interface{}{
					"type":   "number",
					"script": map[string]interface{}{"source": "Math.random()"},
					"order":  "desc",
				},
			},
		}
	} else {
		// Order by created_at desc
		query["sort"] = []map[string]interface{}{
			{
				"created_at": map[string]interface{}{
					"order": "desc",
				},
			},
		}
	}

	return executeESQuery(ctx, query, limit)
}

// Search122 searches ES documents where answer == ans and comments[0].choice == (3 - ans)
// orderByRandom: if true, order by random; if false, order by created_at desc
func Search122(ctx context.Context, ans int, limit int, orderByRandom bool) ([]ESDocument, int64, error) {
	expectedChoice := 3 - ans

	// Build query that checks answer and that there's a comment with the expected choice
	// Since comments is a nested field and we need to check comments[0] specifically,
	// we'll use a nested query to match documents with the expected choice, then filter
	// in application code to ensure it's the first comment (by timestamp)
	// Build query to get documents with answer and comments having expected choice
	query := map[string]interface{}{
		"query": map[string]interface{}{
			"bool": map[string]interface{}{
				"must": []map[string]interface{}{
					{
						"term": map[string]interface{}{
							"answer": ans,
						},
					},
					{
						"nested": map[string]interface{}{
							"path": "comments",
							"query": map[string]interface{}{
								"term": map[string]interface{}{
									"comments.choice": expectedChoice,
								},
							},
						},
					},
				},
			},
		},
		"_source": []string{"mongo_id", "user_review", "timestamp", "answer", "ratio_1", "ratio_2", "comments", "created_at"},
	}

	if orderByRandom {
		query["sort"] = []map[string]interface{}{
			{
				"_script": map[string]interface{}{
					"type":   "number",
					"script": map[string]interface{}{"source": "Math.random()"},
					"order":  "desc",
				},
			},
		}
	} else {
		query["sort"] = []map[string]interface{}{
			{
				"created_at": map[string]interface{}{
					"order": "desc",
				},
			},
		}
	}

	// Fetch more results to account for filtering
	fetchLimit := limit * 5
	query["size"] = fetchLimit

	queryJSON, err := json.Marshal(query)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to build search query: %v", err)
	}

	indexName := config.AppConfig.Elasticsearch.IndexName
	res, err := database.ESClient.Search(
		database.ESClient.Search.WithContext(ctx),
		database.ESClient.Search.WithIndex(indexName),
		database.ESClient.Search.WithBody(bytes.NewReader(queryJSON)),
		database.ESClient.Search.WithTrackTotalHits(true),
	)
	if err != nil {
		return nil, 0, fmt.Errorf("Elasticsearch search failed: %v", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		var e map[string]interface{}
		if err := json.NewDecoder(res.Body).Decode(&e); err != nil {
			return nil, 0, fmt.Errorf("error parsing error response: %v", err)
		}
		return nil, 0, fmt.Errorf("Elasticsearch error: %v", e["error"])
	}

	var result map[string]interface{}
	if err := json.NewDecoder(res.Body).Decode(&result); err != nil {
		return nil, 0, fmt.Errorf("failed to parse search results: %v", err)
	}

	hits, ok := result["hits"].(map[string]interface{})
	if !ok {
		return nil, 0, fmt.Errorf("invalid search response format")
	}

	total, _ := hits["total"].(map[string]interface{})
	totalValue, _ := total["value"].(float64)

	hitsList, ok := hits["hits"].([]interface{})
	if !ok {
		return nil, 0, fmt.Errorf("invalid hits format")
	}

	// Filter results to ensure comments[0].choice == expectedChoice
	filteredResults := make([]ESDocument, 0, limit)
	for _, hit := range hitsList {
		hitMap, ok := hit.(map[string]interface{})
		if !ok {
			continue
		}

		source, ok := hitMap["_source"].(map[string]interface{})
		if !ok {
			continue
		}

		// Check if comments[0].choice == expectedChoice
		comments, ok := source["comments"].([]interface{})
		if !ok || len(comments) == 0 {
			continue
		}

		// Parse comments and find the first one by timestamp
		parsedComments := parseComments(comments)
		if len(parsedComments) == 0 {
			continue
		}

		// Find the comment with the minimum timestamp (first comment)
		firstComment := parsedComments[0]
		for _, comment := range parsedComments {
			if comment.Timestamp < firstComment.Timestamp {
				firstComment = comment
			}
		}

		// Check if first comment (by timestamp) has expected choice
		if firstComment.Choice != expectedChoice {
			continue
		}

		// This document matches, add it to results
		score, _ := hitMap["_score"].(float64)
		doc := ESDocument{
			Score: score,
		}

		if esID, ok := hitMap["_id"]; ok {
			doc.ESID = esID
		}

		if mongoID, ok := source["mongo_id"].(string); ok {
			doc.MongoID = mongoID
		}
		if userReview, ok := source["user_review"].(string); ok {
			doc.UserReview = userReview
		}
		if timestamp, ok := source["timestamp"].(float64); ok {
			doc.Timestamp = int64(timestamp)
		}
		if answer, ok := source["answer"].(float64); ok {
			doc.Answer = int(answer)
		}
		if ratio1, ok := source["ratio_1"].(float64); ok {
			doc.Ratio1 = ratio1
		}
		if ratio2, ok := source["ratio_2"].(float64); ok {
			doc.Ratio2 = ratio2
		}

		// Set hot1_answer and comment_answer
		hot1Answer := firstComment.Choice
		doc.Hot1Answer = &hot1Answer
		doc.CommentAnswer = calculateCommentAnswer(parsedComments)

		filteredResults = append(filteredResults, doc)
		if len(filteredResults) >= limit {
			break
		}
	}

	return filteredResults, int64(totalValue), nil
}

// Search2026WaiTi searches problems with timestamp after 2026-01-01 00:00:00 (UTC+8)
// where over 85% of comment choices are different from the document's answer.
func Search2026WaiTi(ctx context.Context, limit int, orderByRandom bool) ([]ESDocument, int64, error) {
	// 2026-01-01 00:00:00 UTC+8 = 2025-12-31 16:00:00 UTC
	threshold := time.Date(2026, 1, 1, 0, 0, 0, 0, time.FixedZone("CST", 8*3600)).Unix()

	query := map[string]interface{}{
		"query": map[string]interface{}{
			"range": map[string]interface{}{
				"timestamp": map[string]interface{}{
					"gt": threshold,
				},
			},
		},
		"_source": []string{"mongo_id", "user_review", "timestamp", "answer", "ratio_1", "ratio_2", "comments", "created_at"},
	}

	if orderByRandom {
		query["sort"] = []map[string]interface{}{
			{
				"_script": map[string]interface{}{
					"type":   "number",
					"script": map[string]interface{}{"source": "Math.random()"},
					"order":  "desc",
				},
			},
		}
	} else {
		query["sort"] = []map[string]interface{}{
			{
				"timestamp": map[string]interface{}{
					"order": "desc",
				},
			},
		}
	}

	// Fetch extra to account for application-side filtering
	fetchLimit := limit * 10
	query["size"] = fetchLimit

	queryJSON, err := json.Marshal(query)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to build search query: %v", err)
	}

	indexName := config.AppConfig.Elasticsearch.IndexName
	res, err := database.ESClient.Search(
		database.ESClient.Search.WithContext(ctx),
		database.ESClient.Search.WithIndex(indexName),
		database.ESClient.Search.WithBody(bytes.NewReader(queryJSON)),
		database.ESClient.Search.WithTrackTotalHits(true),
	)
	if err != nil {
		return nil, 0, fmt.Errorf("Elasticsearch search failed: %v", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		var e map[string]interface{}
		if err := json.NewDecoder(res.Body).Decode(&e); err != nil {
			return nil, 0, fmt.Errorf("error parsing error response: %v", err)
		}
		return nil, 0, fmt.Errorf("Elasticsearch error: %v", e["error"])
	}

	var result map[string]interface{}
	if err := json.NewDecoder(res.Body).Decode(&result); err != nil {
		return nil, 0, fmt.Errorf("failed to parse search results: %v", err)
	}

	hits, ok := result["hits"].(map[string]interface{})
	if !ok {
		return nil, 0, fmt.Errorf("invalid search response format")
	}

	total, _ := hits["total"].(map[string]interface{})
	totalValue, _ := total["value"].(float64)

	hitsList, ok := hits["hits"].([]interface{})
	if !ok {
		return nil, 0, fmt.Errorf("invalid hits format")
	}

	filteredResults := make([]ESDocument, 0, limit)
	for _, hit := range hitsList {
		if len(filteredResults) >= limit {
			break
		}
		hitMap, ok := hit.(map[string]interface{})
		if !ok {
			continue
		}
		source, ok := hitMap["_source"].(map[string]interface{})
		if !ok {
			continue
		}

		answer := 0
		if v, ok := source["answer"].(float64); ok {
			answer = int(v)
		}
		if answer != 1 && answer != 2 {
			continue
		}

		// Parse comments
		var parsedComments []Comment
		if commentsRaw, ok := source["comments"].([]interface{}); ok {
			for _, c := range commentsRaw {
				if cm, ok := c.(map[string]interface{}); ok {
					comment := Comment{}
					if choice, ok := cm["choice"].(float64); ok {
						comment.Choice = int(choice)
					}
					if ts, ok := cm["timestamp"].(float64); ok {
						comment.Timestamp = int64(ts)
					}
					if name, ok := cm["name"].(string); ok {
						comment.Name = name
					}
					if content, ok := cm["content"].(string); ok {
						comment.Content = content
					}
					if likes, ok := cm["likes"].(float64); ok {
						comment.Likes = int(likes)
					}
					parsedComments = append(parsedComments, comment)
				}
			}
		}

		if len(parsedComments) == 0 {
			continue
		}

		// Count how many comments have choice != answer
		differentCount := 0
		for _, c := range parsedComments {
			if c.Choice != answer {
				differentCount++
			}
		}

		// Over 85% of comments must be different from answer
		ratio := float64(differentCount) / float64(len(parsedComments))
		if ratio <= 0.85 {
			continue
		}

		doc := ESDocument{}
		doc.ESID = hitMap["_id"]
		if v, ok := source["mongo_id"].(string); ok {
			doc.MongoID = v
		}
		if v, ok := source["user_review"].(string); ok {
			doc.UserReview = v
		}
		if v, ok := source["timestamp"].(float64); ok {
			doc.Timestamp = int64(v)
		}
		doc.Answer = answer
		if v, ok := source["ratio_1"].(float64); ok {
			doc.Ratio1 = v
		}
		if v, ok := source["ratio_2"].(float64); ok {
			doc.Ratio2 = v
		}
		doc.Hot1Answer = nil
		if len(parsedComments) > 0 {
			choice := parsedComments[0].Choice
			doc.Hot1Answer = &choice
		}
		doc.CommentAnswer = calculateCommentAnswer(parsedComments)

		filteredResults = append(filteredResults, doc)
	}

	return filteredResults, int64(totalValue), nil
}

func Search(c *gin.Context) {
	var req SearchRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		utils.BadRequestResponse(c, fmt.Sprintf("Invalid request: %v", err))
		return
	}

	// Validate limit: ensure it's between 5 and 30
	if req.Limit < 5 || req.Limit > 30 {
		utils.BadRequestResponse(c, "limit must be between 5 and 30")
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Extract random keyword and check if random ordering is requested
	cleanedKeyword, orderByRandom := extractRandomKeyword(req.Keyword)
	keyword := cleanedKeyword

	// Check for special search patterns
	// (1) Match full text "5149" or "4951": SearchByRatio1, ratio = 51, 49
	if keyword == "5149" {
		results, total, err := SearchByRatio1(ctx, 51.0, req.Limit, orderByRandom)
		if err != nil {
			utils.InternalServerErrorResponse(c, fmt.Sprintf("Search failed: %v", err))
			return
		}
		utils.SuccessResponse(c, SearchResponse{
			Total:   total,
			Results: results,
		})
		return
	}
	if keyword == "4951" {
		results, total, err := SearchByRatio1(ctx, 49.0, req.Limit, orderByRandom)
		if err != nil {
			utils.InternalServerErrorResponse(c, fmt.Sprintf("Search failed: %v", err))
			return
		}
		utils.SuccessResponse(c, SearchResponse{
			Total:   total,
			Results: results,
		})
		return
	}

	// (2) Match regex "^\s*(\d+)(:|-)(\d+)\s*$" and sum equals 100: SearchByRatio1, ratio = int($1)
	ratioRegex := regexp.MustCompile(`^\s*(\d+)(:|-)(\d+)\s*$`)
	if matches := ratioRegex.FindStringSubmatch(keyword); matches != nil {
		num1, err1 := strconv.Atoi(matches[1])
		num2, err2 := strconv.Atoi(matches[3])
		if err1 == nil && err2 == nil && num1+num2 == 100 {
			results, total, err := SearchByRatio1(ctx, float64(num1), req.Limit, orderByRandom)
			if err != nil {
				utils.InternalServerErrorResponse(c, fmt.Sprintf("Search failed: %v", err))
				return
			}
			utils.SuccessResponse(c, SearchResponse{
				Total:   total,
				Results: results,
			})
			return
		}
	}

	// (3) Match full text "122": Search122, ans = 1
	if keyword == "122" {
		results, total, err := Search122(ctx, 1, req.Limit, orderByRandom)
		if err != nil {
			utils.InternalServerErrorResponse(c, fmt.Sprintf("Search failed: %v", err))
			return
		}
		utils.SuccessResponse(c, SearchResponse{
			Total:   total,
			Results: results,
		})
		return
	}

	// (4) Match full text "211": Search122, ans = 2
	if keyword == "211" {
		results, total, err := Search122(ctx, 2, req.Limit, orderByRandom)
		if err != nil {
			utils.InternalServerErrorResponse(c, fmt.Sprintf("Search failed: %v", err))
			return
		}
		utils.SuccessResponse(c, SearchResponse{
			Total:   total,
			Results: results,
		})
		return
	}

	// (5) Match full text "2026歪题": Search2026WaiTi (ignores req.Limit, returns up to 100)
	if keyword == "2026歪题" {
		results, total, err := Search2026WaiTi(ctx, 100, orderByRandom)
		if err != nil {
			utils.InternalServerErrorResponse(c, fmt.Sprintf("Search failed: %v", err))
			return
		}
		utils.SuccessResponse(c, SearchResponse{
			Total:   total,
			Results: results,
		})
		return
	}

	// (6) If none of the search match, normally search the keyword as current does
	// Use the cleaned keyword (without rand/random) for the actual search
	var query map[string]interface{}

	// For single character keywords, use wildcard query on user_review field
	if isSingleCharacter(keyword) {
		// Escape special wildcard characters
		escapedKeyword := escapeWildcard(keyword)

		query = map[string]interface{}{
			"query": map[string]interface{}{
				"wildcard": map[string]interface{}{
					"user_review": map[string]interface{}{
						"value":            fmt.Sprintf("*%s*", escapedKeyword),
						"case_insensitive": true,
					},
				},
			},
			"size": req.Limit,
			"highlight": map[string]interface{}{
				"fields": map[string]interface{}{
					"user_review": map[string]interface{}{
						"require_field_match": false,
						"highlight_query": map[string]interface{}{
							"wildcard": map[string]interface{}{
								"user_review": map[string]interface{}{
									"value":            fmt.Sprintf("*%s*", escapedKeyword),
									"case_insensitive": true,
								},
							},
						},
					},
				},
				"pre_tags":  []string{"<mark>"},
				"post_tags": []string{"</mark>"},
			},
		}

		// Add sorting: random if requested, otherwise by relevance (default)
		if orderByRandom {
			query["sort"] = []map[string]interface{}{
				{
					"_script": map[string]interface{}{
						"type":   "number",
						"script": map[string]interface{}{"source": "Math.random()"},
						"order":  "desc",
					},
				},
			}
		}
	} else {
		// For multi-character keywords, use the normal query builder
		// Calculate dynamic fuzziness and min_score based on keyword length
		fuzziness := calculateFuzziness(keyword)
		minScore := calculateMinScore(keyword)

		// Build Elasticsearch query using the centralized query builder
		boolQuery := utils.BuildBoolQuery(keyword, fuzziness)

		query = map[string]interface{}{
			"query": map[string]interface{}{
				"bool": boolQuery,
			},
			"min_score": minScore, // Dynamic score threshold based on keyword length
			"size":      req.Limit,
			"highlight": map[string]interface{}{
				"fields": map[string]interface{}{
					"user_review": map[string]interface{}{
						"require_field_match": false, // Allow highlighting even if field doesn't match query
						"highlight_query": map[string]interface{}{
							"match_phrase": map[string]interface{}{
								"user_review": map[string]interface{}{
									"query": keyword,
								},
							},
						},
					},
					"others": map[string]interface{}{
						"require_field_match": false,
						"highlight_query": map[string]interface{}{
							"match_phrase": map[string]interface{}{
								"others": map[string]interface{}{
									"query": keyword,
								},
							},
						},
					},
					"replies.content": map[string]interface{}{
						"require_field_match": false,
						"highlight_query": map[string]interface{}{
							"match_phrase": map[string]interface{}{
								"replies.content": map[string]interface{}{
									"query": keyword,
								},
							},
						},
					},
					"appeals.content": map[string]interface{}{
						"require_field_match": false,
						"highlight_query": map[string]interface{}{
							"match_phrase": map[string]interface{}{
								"appeals.content": map[string]interface{}{
									"query": keyword,
								},
							},
						},
					},
				},
				"pre_tags":  []string{"<mark>"},
				"post_tags": []string{"</mark>"},
			},
		}
	}

	// Add sorting: random if requested, otherwise by relevance (default)
	if orderByRandom {
		query["sort"] = []map[string]interface{}{
			{
				"_script": map[string]interface{}{
					"type":   "number",
					"script": map[string]interface{}{"source": "Math.random()"},
					"order":  "desc",
				},
			},
		}
	}

	queryJSON, err := json.Marshal(query)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to build search query")
		return
	}

	// Execute search
	indexName := config.AppConfig.Elasticsearch.IndexName
	res, err := database.ESClient.Search(
		database.ESClient.Search.WithContext(ctx),
		database.ESClient.Search.WithIndex(indexName),
		database.ESClient.Search.WithBody(bytes.NewReader(queryJSON)),
		database.ESClient.Search.WithTrackTotalHits(true),
	)
	if err != nil {
		utils.InternalServerErrorResponse(c, fmt.Sprintf("Elasticsearch search failed: %v", err))
		return
	}
	defer res.Body.Close()

	if res.IsError() {
		var e map[string]interface{}
		if err := json.NewDecoder(res.Body).Decode(&e); err != nil {
			utils.InternalServerErrorResponse(c, fmt.Sprintf("Error parsing error response: %v", err))
		} else {
			utils.InternalServerErrorResponse(c, fmt.Sprintf("Elasticsearch error: %v", e["error"]))
		}
		return
	}

	// Parse response
	var result map[string]interface{}
	if err := json.NewDecoder(res.Body).Decode(&result); err != nil {
		utils.InternalServerErrorResponse(c, "Failed to parse search results")
		return
	}

	// Extract hits
	hits, ok := result["hits"].(map[string]interface{})
	if !ok {
		utils.InternalServerErrorResponse(c, "Invalid search response format")
		return
	}

	total, _ := hits["total"].(map[string]interface{})
	totalValue, _ := total["value"].(float64)

	hitsList, ok := hits["hits"].([]interface{})
	if !ok {
		utils.InternalServerErrorResponse(c, "Invalid hits format")
		return
	}

	// Convert to response format with only required fields
	results := make([]ESDocument, 0, len(hitsList))
	for _, hit := range hitsList {
		hitMap, ok := hit.(map[string]interface{})
		if !ok {
			continue
		}

		source, ok := hitMap["_source"].(map[string]interface{})
		if !ok {
			continue
		}

		score, _ := hitMap["_score"].(float64)

		doc := ESDocument{
			Score: score,
		}

		// Extract ES unique id from _id field
		if esID, ok := hitMap["_id"]; ok {
			doc.ESID = esID
		}

		// Extract highlight if present
		if highlight, ok := hitMap["highlight"].(map[string]interface{}); ok {
			doc.Highlight = make(map[string][]string)
			for k, v := range highlight {
				if arr, ok := v.([]interface{}); ok {
					strs := make([]string, len(arr))
					for i, item := range arr {
						if str, ok := item.(string); ok {
							strs[i] = str
						}
					}
					doc.Highlight[k] = strs
				}
			}
		}

		// Extract required fields from source
		if mongoID, ok := source["mongo_id"].(string); ok {
			doc.MongoID = mongoID
		}
		if userReview, ok := source["user_review"].(string); ok {
			doc.UserReview = userReview
		}
		if timestamp, ok := source["timestamp"].(float64); ok {
			doc.Timestamp = int64(timestamp)
		}
		if answer, ok := source["answer"].(float64); ok {
			doc.Answer = int(answer)
		}
		if ratio1, ok := source["ratio_1"].(float64); ok {
			doc.Ratio1 = ratio1
		}
		if ratio2, ok := source["ratio_2"].(float64); ok {
			doc.Ratio2 = ratio2
		}

		// Parse comments to calculate hot1_answer and comment_answer
		if comments, ok := source["comments"].([]interface{}); ok {
			parsedComments := parseComments(comments)
			if len(parsedComments) > 0 {
				// hot1_answer: from comments[0].choice
				hot1Answer := parsedComments[0].Choice
				doc.Hot1Answer = &hot1Answer
			}
			// comment_answer: most selected choice, excluding last if even length
			doc.CommentAnswer = calculateCommentAnswer(parsedComments)
		}

		results = append(results, doc)
	}

	utils.SuccessResponse(c, SearchResponse{
		Total:   int64(totalValue),
		Results: results,
	})
}

func parseReplies(replies []interface{}) []Reply {
	result := make([]Reply, 0, len(replies))
	for _, r := range replies {
		if replyMap, ok := r.(map[string]interface{}); ok {
			reply := Reply{}
			if role, ok := replyMap["role"].(string); ok {
				reply.Role = role
			}
			if timestamp, ok := replyMap["timestamp"].(float64); ok {
				reply.Timestamp = int64(timestamp)
			}
			if content, ok := replyMap["content"].(string); ok {
				reply.Content = content
			}
			result = append(result, reply)
		}
	}
	return result
}

func parseAppeals(appeals []interface{}) []Appeal {
	result := make([]Appeal, 0, len(appeals))
	for _, a := range appeals {
		if appealMap, ok := a.(map[string]interface{}); ok {
			appeal := Appeal{}
			if role, ok := appealMap["role"].(string); ok {
				appeal.Role = role
			}
			if timestamp, ok := appealMap["timestamp"].(float64); ok {
				appeal.Timestamp = int64(timestamp)
			}
			if content, ok := appealMap["content"].(string); ok {
				appeal.Content = content
			}
			if pics, ok := appealMap["pics"].([]interface{}); ok {
				appeal.Pics = make([]string, len(pics))
				for i, pic := range pics {
					if str, ok := pic.(string); ok {
						appeal.Pics[i] = str
					}
				}
			}
			result = append(result, appeal)
		}
	}
	return result
}

func parseOrders(orders []interface{}) []Order {
	result := make([]Order, 0, len(orders))
	for _, o := range orders {
		if orderMap, ok := o.(map[string]interface{}); ok {
			order := Order{}
			if name, ok := orderMap["name"].(string); ok {
				order.Name = name
			}
			if count, ok := orderMap["count"].(float64); ok {
				order.Count = int(count)
			}
			if desc, ok := orderMap["desc"].(string); ok {
				order.Desc = desc
			}
			if selection, ok := orderMap["selection"].([]interface{}); ok {
				order.Selection = make([]string, len(selection))
				for i, sel := range selection {
					if str, ok := sel.(string); ok {
						order.Selection[i] = str
					}
				}
			}
			if pic, ok := orderMap["pic"].(string); ok {
				order.Pic = pic
			}
			if others, ok := orderMap["others"].(string); ok {
				order.Others = others
			}
			result = append(result, order)
		}
	}
	return result
}

func parseOrderDetail(detail map[string]interface{}) *OrderDetail {
	od := &OrderDetail{}
	if orderStarted, ok := detail["order_started"].(float64); ok {
		od.OrderStarted = int64(orderStarted)
	}
	if orderFinished, ok := detail["order_finished"].(float64); ok {
		od.OrderFinished = int64(orderFinished)
	}
	if deliverTime, ok := detail["deliver_time"].(float64); ok {
		od.DeliverTime = int64(deliverTime)
	}
	if totalTime, ok := detail["total_time"].(float64); ok {
		od.TotalTime = int64(totalTime)
	}
	if deliverBy, ok := detail["deliver_by"].(string); ok {
		od.DeliverBy = deliverBy
	}
	if note, ok := detail["note"].(string); ok {
		od.Note = note
	}
	if utensils, ok := detail["utensils"].(float64); ok {
		od.Utensils = int(utensils)
	}
	if invoice, ok := detail["invoice"].(bool); ok {
		od.Invoice = invoice
	}
	return od
}

func parseComments(comments []interface{}) []Comment {
	result := make([]Comment, 0, len(comments))
	for _, c := range comments {
		if commentMap, ok := c.(map[string]interface{}); ok {
			comment := Comment{}
			if userID, ok := commentMap["userid"].(float64); ok {
				comment.UserID = int64(userID)
			}
			if name, ok := commentMap["name"].(string); ok {
				comment.Name = name
			}
			if content, ok := commentMap["content"].(string); ok {
				comment.Content = content
			}
			if timestamp, ok := commentMap["timestamp"].(float64); ok {
				comment.Timestamp = int64(timestamp)
			}
			if choice, ok := commentMap["choice"].(float64); ok {
				comment.Choice = int(choice)
			}
			if likes, ok := commentMap["likes"].(float64); ok {
				comment.Likes = int(likes)
			}
			result = append(result, comment)
		}
	}
	return result
}

// userIdEncrypt encrypts a userId string by:
// 1. Reverse the userId number
// 2. Set it to base-9
// 3. Reverse it again
// 4. Return it to base 10
func userIdEncrypt(userId string) string {
	if userId == "" {
		return userId
	}

	// Step 1: Convert userId string to number and reverse it
	num, err := strconv.ParseInt(userId, 10, 64)
	if err != nil {
		// If userId is not a valid number, return as is
		return userId
	}

	// Reverse the number
	reversedNum := reverseNumber(num)

	// Step 2: Convert to base-9
	base9Str := strconv.FormatInt(reversedNum, 9)

	// Step 3: Reverse the base-9 string
	reversedBase9 := reverseString(base9Str)

	// Step 4: Convert back to base-10
	encryptedNum, err := strconv.ParseInt(reversedBase9, 9, 64)
	if err != nil {
		// If conversion fails, return original
		return userId
	}

	// Return as string
	return strconv.FormatInt(encryptedNum, 10)
}

// reverseNumber reverses the digits of a number
// Example: 12345 -> 54321
func reverseNumber(num int64) int64 {
	if num == 0 {
		return 0
	}

	reversed := int64(0)
	for num > 0 {
		reversed = reversed*10 + num%10
		num /= 10
	}
	return reversed
}

// reverseString reverses a string
func reverseString(s string) string {
	runes := []rune(s)
	for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
		runes[i], runes[j] = runes[j], runes[i]
	}
	return string(runes)
}

// calculateCommentAnswer calculates the most selected choice from comments
// Excludes the last comment if the comments length is even
func calculateCommentAnswer(comments []Comment) *int {
	if len(comments) == 0 {
		return nil
	}

	// Determine the range to consider (exclude last if even length)
	endIndex := len(comments)
	if len(comments)%2 == 0 {
		endIndex = len(comments) - 1
	}

	if endIndex == 0 {
		return nil
	}

	// Count choices (1 or 2)
	count1 := 0
	count2 := 0
	for i := 0; i < endIndex; i++ {
		if comments[i].Choice == 1 {
			count1++
		} else if comments[i].Choice == 2 {
			count2++
		}
	}

	// Return the most selected choice
	if count1 > count2 {
		result := 1
		return &result
	} else if count2 > count1 {
		result := 2
		return &result
	}
	// If equal, return nil (no clear winner)
	return nil
}

// parseDocument parses a single document from Elasticsearch source
// Returns FullESDocument for use in SearchByESID and SearchByMongoID
func parseDocument(source map[string]interface{}, score float64, highlight map[string]interface{}) FullESDocument {
	doc := FullESDocument{
		Score: score,
	}

	// Extract highlight if present
	if highlight != nil {
		doc.Highlight = make(map[string][]string)
		for k, v := range highlight {
			if arr, ok := v.([]interface{}); ok {
				strs := make([]string, len(arr))
				for i, item := range arr {
					if str, ok := item.(string); ok {
						strs[i] = str
					}
				}
				doc.Highlight[k] = strs
			}
		}
	}

	// Map fields from source
	if id, ok := source["id"]; ok {
		doc.ID = id
	}
	if mongoID, ok := source["mongo_id"].(string); ok {
		doc.MongoID = mongoID
	}
	if stars, ok := source["stars"].(float64); ok {
		doc.Stars = int(stars)
	}
	if userReview, ok := source["user_review"].(string); ok {
		doc.UserReview = userReview
	}
	if reviewPics, ok := source["review_pics"].([]interface{}); ok {
		doc.ReviewPics = make([]string, len(reviewPics))
		for i, pic := range reviewPics {
			if str, ok := pic.(string); ok {
				doc.ReviewPics[i] = str
			}
		}
	}
	if timestamp, ok := source["timestamp"].(float64); ok {
		doc.Timestamp = int64(timestamp)
	}
	if others, ok := source["others"].(string); ok {
		doc.Others = others
	}
	if problemType, ok := source["problem_type"].(float64); ok {
		doc.ProblemType = int(problemType)
	}
	if answer, ok := source["answer"].(float64); ok {
		doc.Answer = int(answer)
	}
	if ratio1, ok := source["ratio_1"].(float64); ok {
		doc.Ratio1 = ratio1
	}
	if ratio2, ok := source["ratio_2"].(float64); ok {
		doc.Ratio2 = ratio2
	}
	if uploader, ok := source["uploader"].(string); ok {
		doc.Uploader = uploader
	}
	if taskID, ok := source["taskId"].(string); ok {
		doc.TaskID = taskID
	}
	if userID, ok := source["userId"].(string); ok {
		doc.UserID = userID
	}
	if createdAt, ok := source["created_at"].(float64); ok {
		doc.CreatedAt = int64(createdAt)
	}

	// Parse nested structures
	if replies, ok := source["replies"].([]interface{}); ok {
		doc.Replies = parseReplies(replies)
	}
	if appeals, ok := source["appeals"].([]interface{}); ok {
		doc.Appeals = parseAppeals(appeals)
	}
	if orderInfo, ok := source["order_info"]; ok {
		doc.OrderInfo = orderInfo
	}
	if orders, ok := source["orders"].([]interface{}); ok {
		doc.Orders = parseOrders(orders)
	}
	if orderDetail, ok := source["order_detail"].(map[string]interface{}); ok {
		doc.OrderDetail = parseOrderDetail(orderDetail)
	}
	if comments, ok := source["comments"].([]interface{}); ok {
		doc.Comments = parseComments(comments)
	}

	return doc
}

// SearchByESID searches for a document by Elasticsearch document ID
func SearchByESID(c *gin.Context) {
	esID := c.Param("id")
	if esID == "" {
		utils.BadRequestResponse(c, "ESID parameter is required")
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Build query to search by _id field
	query := map[string]interface{}{
		"query": map[string]interface{}{
			"term": map[string]interface{}{
				"_id": esID,
			},
		},
		"size": 1,
	}

	queryJSON, err := json.Marshal(query)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to build search query")
		return
	}

	indexName := config.AppConfig.Elasticsearch.IndexName
	res, err := database.ESClient.Search(
		database.ESClient.Search.WithContext(ctx),
		database.ESClient.Search.WithIndex(indexName),
		database.ESClient.Search.WithBody(bytes.NewReader(queryJSON)),
	)
	if err != nil {
		utils.InternalServerErrorResponse(c, fmt.Sprintf("Elasticsearch search failed: %v", err))
		return
	}
	defer res.Body.Close()

	if res.IsError() {
		var e map[string]interface{}
		if err := json.NewDecoder(res.Body).Decode(&e); err != nil {
			utils.InternalServerErrorResponse(c, fmt.Sprintf("Error parsing error response: %v", err))
		} else {
			utils.InternalServerErrorResponse(c, fmt.Sprintf("Elasticsearch error: %v", e["error"]))
		}
		return
	}

	// Parse response
	var result map[string]interface{}
	if err := json.NewDecoder(res.Body).Decode(&result); err != nil {
		utils.InternalServerErrorResponse(c, "Failed to parse search results")
		return
	}

	// Extract hits
	hits, ok := result["hits"].(map[string]interface{})
	if !ok {
		utils.InternalServerErrorResponse(c, "Invalid search response format")
		return
	}

	hitsList, ok := hits["hits"].([]interface{})
	if !ok {
		utils.InternalServerErrorResponse(c, "Invalid hits format")
		return
	}

	if len(hitsList) == 0 {
		utils.ErrorResponse(c, 404, "Document not found")
		return
	}

	// Get first hit
	hitMap, ok := hitsList[0].(map[string]interface{})
	if !ok {
		utils.InternalServerErrorResponse(c, "Invalid hit format")
		return
	}

	source, ok := hitMap["_source"].(map[string]interface{})
	if !ok {
		utils.InternalServerErrorResponse(c, "Invalid document format")
		return
	}

	score := 0.0
	if s, ok := hitMap["_score"].(float64); ok {
		score = s
	}

	var highlight map[string]interface{}
	if h, ok := hitMap["highlight"].(map[string]interface{}); ok {
		highlight = h
	}

	doc := parseDocument(source, score, highlight)

	// Encrypt userId before returning
	if doc.UserID != "" {
		doc.UserID = userIdEncrypt(doc.UserID)
	}

	utils.SuccessResponse(c, doc)
}

// isMaliciousUser checks if a user ID is in the malicious collection
func isMaliciousUser(userID string) bool {
	if userID == "" {
		return false
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var result bson.M
	err := database.Malicious.FindOne(ctx, bson.M{"userId": userID}).Decode(&result)
	return err == nil
}

// filterMaliciousComments removes comments from malicious users
func filterMaliciousComments(comments []Comment) []Comment {
	if len(comments) == 0 {
		return comments
	}

	filtered := make([]Comment, 0, len(comments))
	for _, comment := range comments {
		// Convert userid (int64) to string for lookup
		userIDStr := strconv.FormatInt(comment.UserID, 10)
		if !isMaliciousUser(userIDStr) {
			filtered = append(filtered, comment)
		}
	}
	return filtered
}

// SearchByMongoID searches for a document by MongoDB ID
func SearchByMongoID(c *gin.Context) {
	mongoID := c.Param("id")
	if mongoID == "" {
		utils.BadRequestResponse(c, "MongoID parameter is required")
		return
	}

	// Get blockMaliciousComment parameter (default: 1)
	blockMaliciousComment := c.DefaultQuery("blockMaliciousComment", "1")
	shouldBlock := blockMaliciousComment != "0"

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Build query to search by mongo_id
	query := map[string]interface{}{
		"query": map[string]interface{}{
			"term": map[string]interface{}{
				"mongo_id": mongoID,
			},
		},
		"size": 1,
	}

	queryJSON, err := json.Marshal(query)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to build search query")
		return
	}

	indexName := config.AppConfig.Elasticsearch.IndexName
	res, err := database.ESClient.Search(
		database.ESClient.Search.WithContext(ctx),
		database.ESClient.Search.WithIndex(indexName),
		database.ESClient.Search.WithBody(bytes.NewReader(queryJSON)),
	)
	if err != nil {
		utils.InternalServerErrorResponse(c, fmt.Sprintf("Elasticsearch search failed: %v", err))
		return
	}
	defer res.Body.Close()

	if res.IsError() {
		var e map[string]interface{}
		if err := json.NewDecoder(res.Body).Decode(&e); err != nil {
			utils.InternalServerErrorResponse(c, fmt.Sprintf("Error parsing error response: %v", err))
		} else {
			utils.InternalServerErrorResponse(c, fmt.Sprintf("Elasticsearch error: %v", e["error"]))
		}
		return
	}

	// Parse response
	var result map[string]interface{}
	if err := json.NewDecoder(res.Body).Decode(&result); err != nil {
		utils.InternalServerErrorResponse(c, "Failed to parse search results")
		return
	}

	// Extract hits
	hits, ok := result["hits"].(map[string]interface{})
	if !ok {
		utils.InternalServerErrorResponse(c, "Invalid search response format")
		return
	}

	hitsList, ok := hits["hits"].([]interface{})
	if !ok {
		utils.InternalServerErrorResponse(c, "Invalid hits format")
		return
	}

	if len(hitsList) == 0 {
		utils.ErrorResponse(c, 404, "Document not found")
		return
	}

	// Get first hit
	hitMap, ok := hitsList[0].(map[string]interface{})
	if !ok {
		utils.InternalServerErrorResponse(c, "Invalid hit format")
		return
	}

	source, ok := hitMap["_source"].(map[string]interface{})
	if !ok {
		utils.InternalServerErrorResponse(c, "Invalid document format")
		return
	}

	score := 0.0
	if s, ok := hitMap["_score"].(float64); ok {
		score = s
	}

	var highlight map[string]interface{}
	if h, ok := hitMap["highlight"].(map[string]interface{}); ok {
		highlight = h
	}

	doc := parseDocument(source, score, highlight)

	// Filter malicious comments if blockMaliciousComment is not 0
	if shouldBlock && len(doc.Comments) > 0 {
		doc.Comments = filterMaliciousComments(doc.Comments)
	}

	// Encrypt userId before returning
	if doc.UserID != "" {
		doc.UserID = userIdEncrypt(doc.UserID)
	}

	utils.SuccessResponse(c, doc)
}

// GetRecentProblems returns the newest problems sorted by created_at
func GetRecentProblems(c *gin.Context) {
	var req RecentProblemsRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		utils.BadRequestResponse(c, fmt.Sprintf("Invalid request: %v", err))
		return
	}

	// Default limit to 15 if not provided
	limit := req.Limit
	if limit == 0 {
		limit = 15
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Build query to get all documents sorted by created_at descending
	query := map[string]interface{}{
		"query": map[string]interface{}{
			"match_all": map[string]interface{}{},
		},
		"sort": []map[string]interface{}{
			{
				"created_at": map[string]interface{}{
					"order": "desc",
				},
			},
		},
		"size": limit,
	}

	queryJSON, err := json.Marshal(query)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to build search query")
		return
	}

	// Execute search
	indexName := config.AppConfig.Elasticsearch.IndexName
	res, err := database.ESClient.Search(
		database.ESClient.Search.WithContext(ctx),
		database.ESClient.Search.WithIndex(indexName),
		database.ESClient.Search.WithBody(bytes.NewReader(queryJSON)),
		database.ESClient.Search.WithTrackTotalHits(true),
	)
	if err != nil {
		utils.InternalServerErrorResponse(c, fmt.Sprintf("Elasticsearch search failed: %v", err))
		return
	}
	defer res.Body.Close()

	if res.IsError() {
		var e map[string]interface{}
		if err := json.NewDecoder(res.Body).Decode(&e); err != nil {
			utils.InternalServerErrorResponse(c, fmt.Sprintf("Error parsing error response: %v", err))
		} else {
			utils.InternalServerErrorResponse(c, fmt.Sprintf("Elasticsearch error: %v", e["error"]))
		}
		return
	}

	// Parse response
	var result map[string]interface{}
	if err := json.NewDecoder(res.Body).Decode(&result); err != nil {
		utils.InternalServerErrorResponse(c, "Failed to parse search results")
		return
	}

	// Extract hits
	hits, ok := result["hits"].(map[string]interface{})
	if !ok {
		utils.InternalServerErrorResponse(c, "Invalid search response format")
		return
	}

	total, _ := hits["total"].(map[string]interface{})
	totalValue, _ := total["value"].(float64)

	hitsList, ok := hits["hits"].([]interface{})
	if !ok {
		utils.InternalServerErrorResponse(c, "Invalid hits format")
		return
	}

	// Convert to response format with only required fields (same as Search function)
	results := make([]ESDocument, 0, len(hitsList))
	for _, hit := range hitsList {
		hitMap, ok := hit.(map[string]interface{})
		if !ok {
			continue
		}

		source, ok := hitMap["_source"].(map[string]interface{})
		if !ok {
			continue
		}

		score, _ := hitMap["_score"].(float64)

		doc := ESDocument{
			Score: score,
		}

		// Extract ES unique id from _id field
		if esID, ok := hitMap["_id"]; ok {
			doc.ESID = esID
		}

		// Extract highlight if present (should be empty for recent problems, but keep for consistency)
		if highlight, ok := hitMap["highlight"].(map[string]interface{}); ok {
			doc.Highlight = make(map[string][]string)
			for k, v := range highlight {
				if arr, ok := v.([]interface{}); ok {
					strs := make([]string, len(arr))
					for i, item := range arr {
						if str, ok := item.(string); ok {
							strs[i] = str
						}
					}
					doc.Highlight[k] = strs
				}
			}
		}

		// Extract required fields from source
		if mongoID, ok := source["mongo_id"].(string); ok {
			doc.MongoID = mongoID
		}
		if userReview, ok := source["user_review"].(string); ok {
			doc.UserReview = userReview
		}
		if timestamp, ok := source["timestamp"].(float64); ok {
			doc.Timestamp = int64(timestamp)
		}
		if answer, ok := source["answer"].(float64); ok {
			doc.Answer = int(answer)
		}
		if ratio1, ok := source["ratio_1"].(float64); ok {
			doc.Ratio1 = ratio1
		}
		if ratio2, ok := source["ratio_2"].(float64); ok {
			doc.Ratio2 = ratio2
		}

		// Parse comments to calculate hot1_answer and comment_answer
		if comments, ok := source["comments"].([]interface{}); ok {
			parsedComments := parseComments(comments)
			if len(parsedComments) > 0 {
				// hot1_answer: from comments[0].choice
				hot1Answer := parsedComments[0].Choice
				doc.Hot1Answer = &hot1Answer
			}
			// comment_answer: most selected choice, excluding last if even length
			doc.CommentAnswer = calculateCommentAnswer(parsedComments)
		}

		results = append(results, doc)
	}

	utils.SuccessResponse(c, SearchResponse{
		Total:   int64(totalValue),
		Results: results,
	})
}
