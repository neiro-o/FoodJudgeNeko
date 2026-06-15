package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"mtv2/backend/config"
	"mtv2/backend/database"
	"mtv2/backend/utils"

	"github.com/gin-gonic/gin"
)

type BotSearchRequest struct {
	Keyword string `form:"keyword" binding:"required"`
	Date    string `form:"date"`
}

type botDateRange struct {
	Start int64
	End   int64
}

type BotSearchResponse struct {
	Total   int64               `json:"total"`
	Results []BotSearchDocument `json:"results"`
}

type BotSearchDocument struct {
	ESID       interface{}         `json:"id"`
	MongoID    string              `json:"mongo_id"`
	UserReview string              `json:"user_review"`
	Timestamp  int64               `json:"timestamp"`
	Answer     int                 `json:"answer"`
	Ratio1     float64             `json:"ratio_1"`
	Ratio2     float64             `json:"ratio_2"`
	Score      float64             `json:"_score"`
	Highlight  map[string][]string `json:"_highlight,omitempty"`
}

func parseBotSearchDate(date string) (*botDateRange, *botDateRange, error) {
	if date == "" {
		return nil, nil, nil
	}

	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		return nil, nil, err
	}

	parts := regexp.MustCompile(`\D+`).Split(date, -1)
	if len(parts) != 3 || parts[0] == "" || parts[1] == "" || parts[2] == "" {
		return nil, nil, fmt.Errorf("date must contain year, month, and day")
	}

	year, err := strconv.Atoi(parts[0])
	if err != nil {
		return nil, nil, fmt.Errorf("invalid year in date")
	}
	month, err := strconv.Atoi(parts[1])
	if err != nil {
		return nil, nil, fmt.Errorf("invalid month in date")
	}
	day, err := strconv.Atoi(parts[2])
	if err != nil {
		return nil, nil, fmt.Errorf("invalid day in date")
	}

	dayStart := time.Date(year, time.Month(month), day, 0, 0, 0, 0, loc)
	if dayStart.Year() != year || int(dayStart.Month()) != month || dayStart.Day() != day {
		return nil, nil, fmt.Errorf("invalid date")
	}

	dayEnd := dayStart.AddDate(0, 0, 1)
	monthStart := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, loc)
	monthEnd := monthStart.AddDate(0, 1, 0)

	return &botDateRange{Start: dayStart.Unix(), End: dayEnd.Unix()},
		&botDateRange{Start: monthStart.Unix(), End: monthEnd.Unix()},
		nil
}

func botTimestampFilter(dateRange *botDateRange) []map[string]interface{} {
	if dateRange == nil {
		return nil
	}

	return []map[string]interface{}{
		{
			"range": map[string]interface{}{
				"timestamp": map[string]interface{}{
					"gte": dateRange.Start,
					"lt":  dateRange.End,
				},
			},
		},
	}
}

func calculateOCRMinimumShouldMatch(keyword string) string {
	length := utf8.RuneCountInString(keyword)
	switch {
	case length <= 2:
		return "100%"
	case length <= 4:
		return "75%"
	case length <= 6:
		return "60%"
	case length <= 10:
		return "55%"
	case length <= 20:
		return "50%"
	default:
		return "45%"
	}
}

func normalizeBotSearchHalfWidth(keyword string) string {
	var b strings.Builder
	for _, r := range keyword {
		switch {
		case r == '。':
			b.WriteRune('.')
		case r == '、':
			b.WriteRune(',')
		case r == '【':
			b.WriteRune('[')
		case r == '】':
			b.WriteRune(']')
		case r == '《':
			b.WriteRune('<')
		case r == '》':
			b.WriteRune('>')
		case r == '“' || r == '”':
			b.WriteRune('"')
		case r == '‘' || r == '’':
			b.WriteRune('\'')
		case r == '\u3000':
			b.WriteRune(' ')
		case r >= '！' && r <= '～':
			b.WriteRune(r - 0xFEE0)
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

func botFullWidthVariant(keyword string) string {
	punctuationMap := map[rune]rune{
		',':  '，',
		'.':  '。',
		'?':  '？',
		'!':  '！',
		':':  '：',
		';':  '；',
		'(':  '（',
		')':  '）',
		'[':  '【',
		']':  '】',
		'<':  '《',
		'>':  '》',
		'"':  '”',
		'\'': '’',
	}

	var b strings.Builder
	for _, r := range keyword {
		if fullWidth, ok := punctuationMap[r]; ok {
			b.WriteRune(fullWidth)
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

func botSearchKeywordVariants(keyword string) []string {
	variants := []string{
		keyword,
		strings.ToLower(keyword),
		strings.ToUpper(keyword),
		normalizeBotSearchHalfWidth(keyword),
	}

	halfWidth := normalizeBotSearchHalfWidth(keyword)
	variants = append(variants,
		strings.ToLower(halfWidth),
		strings.ToUpper(halfWidth),
		botFullWidthVariant(halfWidth),
		botFullWidthVariant(strings.ToLower(halfWidth)),
		botFullWidthVariant(strings.ToUpper(halfWidth)),
	)

	seen := make(map[string]bool, len(variants))
	result := make([]string, 0, len(variants))
	for _, variant := range variants {
		if variant == "" || seen[variant] {
			continue
		}
		seen[variant] = true
		result = append(result, variant)
	}
	return result
}

func botSingleCharacterExactVariants(keyword string) []interface{} {
	keywords := botSearchKeywordVariants(keyword)
	result := make([]interface{}, 0, len(keywords))
	for _, variant := range keywords {
		runes := []rune(variant)
		if len(runes) != 1 {
			continue
		}
		result = append(result, variant)

		lower := string(unicode.ToLower(runes[0]))
		upper := string(unicode.ToUpper(runes[0]))
		if lower != variant {
			result = append(result, lower)
		}
		if upper != variant {
			result = append(result, upper)
		}
	}

	seen := make(map[interface{}]bool, len(result))
	unique := make([]interface{}, 0, len(result))
	for _, variant := range result {
		if seen[variant] {
			continue
		}
		seen[variant] = true
		unique = append(unique, variant)
	}
	return unique
}

func buildBotExactSingleCharacterQuery(keyword string, dateRange *botDateRange) map[string]interface{} {
	return map[string]interface{}{
		"query": map[string]interface{}{
			"bool": map[string]interface{}{
				"must": []map[string]interface{}{
					{
						"terms": map[string]interface{}{
							"user_review.keyword": botSingleCharacterExactVariants(keyword),
						},
					},
				},
				"filter": botTimestampFilter(dateRange),
			},
		},
		"_source": []string{"mongo_id", "user_review", "timestamp", "answer", "ratio_1", "ratio_2"},
	}
}

func buildBotFieldQuery(fieldName string, keywords []string, boost float64, minimumShouldMatch string) map[string]interface{} {
	queries := make([]map[string]interface{}, 0, len(keywords)*2)
	for _, keyword := range keywords {
		queries = append(queries,
			map[string]interface{}{
				"match_phrase": map[string]interface{}{
					fieldName: map[string]interface{}{
						"query": keyword,
						"boost": boost * 4.0,
					},
				},
			},
			map[string]interface{}{
				"match": map[string]interface{}{
					fieldName: map[string]interface{}{
						"query":                keyword,
						"operator":             "or",
						"minimum_should_match": minimumShouldMatch,
						"boost":                boost * 2.0,
					},
				},
			},
		)
	}

	return map[string]interface{}{
		"dis_max": map[string]interface{}{
			"tie_breaker": 0.0,
			"queries":     queries,
		},
	}
}

func buildBotNestedQuery(path, fieldName string, keywords []string, boost float64, minimumShouldMatch string) map[string]interface{} {
	return map[string]interface{}{
		"nested": map[string]interface{}{
			"path":       path,
			"query":      buildBotFieldQuery(fieldName, keywords, boost, minimumShouldMatch),
			"score_mode": "max",
		},
	}
}

func buildBotOCRQuery(keyword string, dateRange *botDateRange) map[string]interface{} {
	minimumShouldMatch := calculateOCRMinimumShouldMatch(keyword)
	keywords := botSearchKeywordVariants(keyword)

	return map[string]interface{}{
		"query": map[string]interface{}{
			"bool": map[string]interface{}{
				"must": []map[string]interface{}{
					{
						"dis_max": map[string]interface{}{
							"tie_breaker": 0.0,
							"queries": []map[string]interface{}{
								buildBotFieldQuery("user_review", keywords, 1.0, minimumShouldMatch),
								buildBotNestedQuery("replies", "replies.content", keywords, 0.4, minimumShouldMatch),
								buildBotNestedQuery("appeals", "appeals.content", keywords, 0.6, minimumShouldMatch),
							},
						},
					},
				},
				"filter": botTimestampFilter(dateRange),
			},
		},
		"min_score": 2.0,
		"_source":   []string{"mongo_id", "user_review", "timestamp", "answer", "ratio_1", "ratio_2"},
		"highlight": map[string]interface{}{
			"fields": map[string]interface{}{
				"user_review": map[string]interface{}{
					"require_field_match": false,
				},
				"replies.content": map[string]interface{}{
					"require_field_match": false,
				},
				"appeals.content": map[string]interface{}{
					"require_field_match": false,
				},
			},
			"pre_tags":  []string{"<mark>"},
			"post_tags": []string{"</mark>"},
		},
	}
}

func executeBotESQuery(ctx context.Context, query map[string]interface{}, limit int) ([]BotSearchDocument, int64, error) {
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

	results := make([]BotSearchDocument, 0, len(hitsList))
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
		doc := BotSearchDocument{Score: score}

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

		results = append(results, doc)
	}

	return results, int64(totalValue), nil
}

func executeBotSearch(ctx context.Context, keyword string, dateRange *botDateRange) ([]BotSearchDocument, int64, error) {
	if isSingleCharacter(keyword) {
		results, total, err := executeBotESQuery(ctx, buildBotExactSingleCharacterQuery(keyword, dateRange), 1)
		if err != nil {
			return nil, 0, err
		}
		if len(results) > 0 {
			return results[:1], total, nil
		}
	}

	results, total, err := executeBotESQuery(ctx, buildBotOCRQuery(keyword, dateRange), 1)
	if err != nil {
		return nil, 0, err
	}
	if len(results) > 1 {
		results = results[:1]
	}
	return results, total, nil
}

func BotSearch(c *gin.Context) {
	var req BotSearchRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		utils.BadRequestResponse(c, fmt.Sprintf("Invalid request: %v", err))
		return
	}

	dayRange, monthRange, err := parseBotSearchDate(req.Date)
	if err != nil {
		utils.BadRequestResponse(c, err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	results, total, err := executeBotSearch(ctx, req.Keyword, dayRange)
	if err != nil {
		utils.InternalServerErrorResponse(c, fmt.Sprintf("Search failed: %v", err))
		return
	}

	if len(results) == 0 && dayRange != nil {
		results, total, err = executeBotSearch(ctx, req.Keyword, monthRange)
		if err != nil {
			utils.InternalServerErrorResponse(c, fmt.Sprintf("Search failed: %v", err))
			return
		}
	}

	utils.SuccessResponse(c, BotSearchResponse{
		Total:   total,
		Results: results,
	})
}
