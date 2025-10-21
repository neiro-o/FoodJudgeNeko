package controllers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"mt-backend/config"
	"mt-backend/middleware"
	"mt-backend/models"
	"mt-backend/utils"
)

// SearchProblems searches problems from Elasticsearch using a single keyword
func SearchProblems(c *gin.Context) {
	userID, ok := middleware.GetUserID(c)
	if !ok {
		utils.ForbiddenResponse(c, utils.MsgUnauthorized)
		return
	}

	var req models.SearchProblemsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(c, err.Error())
		return
	}

	// Set default pagination values
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 20
	}

	// Validate page size (10-25)
	if req.PageSize < 10 || req.PageSize > 25 {
		utils.BadRequestResponse(c, utils.MsgInvalidPageSize)
		return
	}

	// Build Elasticsearch query
	query := map[string]interface{}{
		"query": map[string]interface{}{
			"multi_match": map[string]interface{}{
				"query":  req.Query,
				"fields": []string{
					"user_review",
					"replies.content",
					"appeals.content",
					"others",
				},
				"type": "best_fields",
			},
		},
		"from": (req.Page - 1) * req.PageSize,
		"size": req.PageSize,
		"sort": []map[string]interface{}{
			{"_score": map[string]string{"order": "desc"}},
		},
	}

	// Execute search
	ctx := context.Background()
	cfg := config.GetConfig()
	res, err := config.ES.Search(
		config.ES.Search.WithContext(ctx),
		config.ES.Search.WithIndex(cfg.Elasticsearch.Index),
		config.ES.Search.WithBody(config.ES.Search.WithBody().Body(query)),
	)
	if err != nil {
		utils.InternalServerErrorResponse(c, utils.MsgElasticsearchError)
		return
	}
	defer res.Body.Close()

	// Parse response
	var searchResponse map[string]interface{}
	if err := json.NewDecoder(res.Body).Decode(&searchResponse); err != nil {
		utils.InternalServerErrorResponse(c, utils.MsgSearchFailed)
		return
	}

	// Extract results
	hits, ok := searchResponse["hits"].(map[string]interface{})
	if !ok {
		utils.InternalServerErrorResponse(c, utils.MsgSearchFailed)
		return
	}

	total, _ := hits["total"].(map[string]interface{})
	totalValue, _ := total["value"].(float64)

	hitsList, ok := hits["hits"].([]interface{})
	if !ok {
		utils.InternalServerErrorResponse(c, utils.MsgSearchFailed)
		return
	}

	// Convert results to SearchResult format
	var results []models.SearchResult
	for _, hit := range hitsList {
		hitMap, ok := hit.(map[string]interface{})
		if !ok {
			continue
		}

		source, ok := hitMap["_source"].(map[string]interface{})
		if !ok {
			continue
		}

		// Extract and format data
		result := models.SearchResult{}

		// ID
		if id, ok := source["id"].(string); ok {
			result.ID = id
		}

		// Match score from Elasticsearch
		if score, ok := hitMap["_score"].(float64); ok {
			result.MatchScore = score
		}

		// Review content logic
		if userReview, ok := source["user_review"].(string); ok && userReview != "" {
			result.Review = userReview
		} else if appeals, ok := source["appeals"].([]interface{}); ok && len(appeals) > 0 {
			if appeal, ok := appeals[0].(map[string]interface{}); ok {
				if content, ok := appeal["content"].(string); ok {
					result.Review = content
				}
			}
		}
		if result.Review == "" {
			result.Review = ""
		}

		// Generate time formatting
		if timestamp, ok := source["timestamp"].(float64); ok {
			loc, _ := time.LoadLocation("Asia/Shanghai")
			t := time.Unix(int64(timestamp), 0).In(loc)
			result.GenerateTime = t.Format("2006-01-02 15:04:05")
		}

		// Other fields
		if uploader, ok := source["uploader"].(string); ok {
			result.Uploader = uploader
		}
		if answer, ok := source["answer"].(float64); ok {
			result.Answer = int(answer)
		}
		if ratio1, ok := source["ratio_1"].(float64); ok {
			result.Ratio1 = int(ratio1)
		}
		if ratio2, ok := source["ratio_2"].(float64); ok {
			result.Ratio2 = int(ratio2)
		}

		results = append(results, result)
	}

	utils.SuccessResponse(c, utils.MsgSearchSuccess, models.SearchProblemsResponse{
		Results:  results,
		Total:    int64(totalValue),
		Page:     req.Page,
		PageSize: req.PageSize,
	})
}

// AddProblems adds parsed links to the processing queue
func AddProblems(c *gin.Context) {
	userID, ok := middleware.GetUserID(c)
	if !ok {
		utils.ForbiddenResponse(c, utils.MsgUnauthorized)
		return
	}

	var req models.AddProblemsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(c, err.Error())
		return
	}

	uploadTime := time.Now()
	uploadIP := c.ClientIP()

	// Create queue items for each parsed link
	var queueItems []models.QueueItem
	for _, parsedLink := range req.ParsedLinks {
		queueItem := models.QueueItem{
			UserID:     userID,
			MtUserID:   parsedLink.MtUserID,
			TaskID:     parsedLink.TaskID,
			UploadTime: uploadTime,
			UploadIP:   uploadIP,
		}
		queueItems = append(queueItems, queueItem)
	}

	// TODO: Push queue items to Redis queue for processing
	// TODO: Store queue items in MongoDB for tracking
	// TODO: Implement background worker to process queue items
	// TODO: Fetch data from external API using mtuserid and task_id
	// TODO: Parse and store raw data in MongoDB
	// TODO: Extract structured data and index in Elasticsearch

	utils.SuccessResponse(c, "Parsed links queued successfully", gin.H{
		"user_id":      userID.Hex(),
		"count":        len(queueItems),
		"upload_time":  uploadTime,
		"upload_ip":    uploadIP,
		"status":       "queued_for_processing",
	})
}

// GetProblemDetail fetches detailed information about a single problem
func GetProblemDetail(c *gin.Context) {
	userID, ok := middleware.GetUserID(c)
	if !ok {
		utils.ForbiddenResponse(c, utils.MsgUnauthorized)
		return
	}

	problemIDStr := c.Param("id")
	problemID, err := primitive.ObjectIDFromHex(problemIDStr)
	if err != nil {
		utils.BadRequestResponse(c, "invalid problem ID")
		return
	}

	// TODO: Query Elasticsearch for problem details
	// For now, return mock response
	problem := &models.Problem{
		ID:         problemID,
		UserReview: "Sample review content",
		Timestamp:  time.Now().Unix(),
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	utils.SuccessResponse(c, "Problem details retrieved successfully", models.ProblemDetailResponse{
		Problem: problem,
	})
}

// GetRawProblem fetches the original raw problem data from MongoDB
func GetRawProblem(c *gin.Context) {
	userID, ok := middleware.GetUserID(c)
	if !ok {
		utils.ForbiddenResponse(c, utils.MsgUnauthorized)
		return
	}

	problemIDStr := c.Param("id")
	problemID, err := primitive.ObjectIDFromHex(problemIDStr)
	if err != nil {
		utils.BadRequestResponse(c, "invalid problem ID")
		return
	}

	ctx := context.Background()
	collection := config.MongoDB.Collection("raw_problems")

	var rawProblem models.RawProblem
	err = collection.FindOne(ctx, bson.M{"_id": problemID}).Decode(&rawProblem)
	if err != nil {
		utils.NotFoundResponse(c, "raw problem not found")
		return
	}

	utils.SuccessResponse(c, "Raw problem retrieved successfully", gin.H{
		"raw_problem": rawProblem,
		"user_id":     userID.Hex(),
	})
}

// ProcessRawProblems manually triggers processing of raw problems
func ProcessRawProblems(c *gin.Context) {
	userID, ok := middleware.GetUserID(c)
	if !ok {
		utils.ForbiddenResponse(c, utils.MsgUnauthorized)
		return
	}

	// TODO: Implement manual processing trigger
	// This would typically be an admin-only endpoint
	utils.SuccessResponse(c, "Processing triggered", gin.H{
		"user_id": userID.Hex(),
	})
}
