package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"time"

	"mtv2/backend/config"
	"mtv2/backend/database"
	"mtv2/backend/models"
	"mtv2/backend/utils"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

type UploadProblemRequest struct {
	UserID string `json:"userId" binding:"required"`
	TaskID string `json:"taskId" binding:"required"`
}

type UploadMultipleProblemsRequest struct {
	Problems []UploadProblemRequest `json:"problems" binding:"required,min=1"`
}

type QueueItem struct {
	UserID    string `json:"userId"`
	TaskID    string `json:"taskId"`
	AccountID string `json:"accountId"`
	UploadIP  string `json:"uploadIP"`
}

type DailyQueueItem struct {
	UserID    string `json:"userId"`
	DateID    string `json:"dateId"`
	AccountID string `json:"accountId"`
	UploadIP  string `json:"uploadIP"`
}

type UploadDailyRequest struct {
	UserID string `json:"userId" binding:"required"`
	DateID string `json:"dateId" binding:"required"`
}

func UploadProblem(c *gin.Context) {
	// Get authenticated user ID (MongoDB ObjectID hex string)
	accountID, exists := utils.GetUserID(c)
	if !exists {
		utils.UnauthorizedResponse(c, "User not authenticated")
		return
	}

	// Parse request
	var req UploadProblemRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(c, err.Error())
		return
	}

	// Get client IP
	uploadIP := getClientIP(c)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Check if userId and taskId combination already exists in MongoDB processed_list
	var existingItem models.ProcessedListItem
	err := database.ProcessedList.FindOne(ctx, bson.M{
		"user_id": req.UserID,
		"task_id": req.TaskID,
	}).Decode(&existingItem)
	if err == nil {
		// Item already exists in processed list
		utils.ConflictResponse(c, "Problem has already exists")
		return
	} else if err != mongo.ErrNoDocuments {
		// Database error
		utils.InternalServerErrorResponse(c, "Database error")
		return
	}

	// Check if userId and taskId combination already exists in MongoDB problems collection
	var existingProblem bson.M
	err = database.Problems.FindOne(ctx, bson.M{
		"userId": req.UserID,
		"taskId": req.TaskID,
	}).Decode(&existingProblem)
	if err == nil {
		// Problem already exists
		utils.ConflictResponse(c, "Problem has already exists")
		return
	} else if err != mongo.ErrNoDocuments {
		// Database error
		utils.InternalServerErrorResponse(c, "Database error")
		return
	}

	// Check if userId and taskId combination already exists in Redis queue
	queueKey := fmt.Sprintf("%s:%s", req.UserID, req.TaskID)
	existsCount, err := database.RedisClient.Exists(ctx, queueKey).Result()
	if err != nil {
		utils.InternalServerErrorResponse(c, "Redis error")
		return
	}
	if existsCount > 0 {
		// Item already exists in Redis queue
		utils.ConflictResponse(c, "Problem has already exists")
		return
	}

	// Create queue item
	queueItem := QueueItem{
		UserID:    req.UserID,
		TaskID:    req.TaskID,
		AccountID: accountID, // MongoDB ObjectID hex string from accounts collection
		UploadIP:  uploadIP,
	}

	// Serialize queue item to JSON
	itemJSON, err := json.Marshal(queueItem)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to serialize queue item")
		return
	}

	// Push to Redis queue
	queueName := config.AppConfig.Redis.Fields.ProblemsQueue
	_, err = database.RedisClient.LPush(ctx, queueName, itemJSON).Result()
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to push to queue")
		return
	}

	// Set a marker in Redis to track this userId:taskId combination
	// This prevents duplicate entries even if the item is still in queue
	err = database.RedisClient.Set(ctx, queueKey, "1", 24*time.Hour).Err()
	if err != nil {
		// Log error but don't fail the request
		// The item is already in the queue
	}

	utils.SuccessResponse(c, gin.H{
		"message": "Problem uploaded successfully",
		"data": gin.H{
			"userId":   req.UserID,
			"taskId":   req.TaskID,
			"uploadIP": uploadIP,
		},
	})
}

type ProblemUploadResult struct {
	UserID  string `json:"userId"`
	TaskID  string `json:"taskId"`
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
}

func UploadMultipleProblems(c *gin.Context) {
	// Get authenticated user ID (MongoDB ObjectID hex string)
	accountID, exists := utils.GetUserID(c)
	if !exists {
		utils.UnauthorizedResponse(c, "User not authenticated")
		return
	}

	// Parse request
	var req UploadMultipleProblemsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(c, err.Error())
		return
	}

	// Get client IP
	uploadIP := getClientIP(c)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var results []ProblemUploadResult
	queueName := config.AppConfig.Redis.Fields.ProblemsQueue

	// Process each problem
	for _, problem := range req.Problems {
		result := ProblemUploadResult{
			UserID:  problem.UserID,
			TaskID:  problem.TaskID,
			Success: false,
		}

		// Check if userId and taskId combination already exists in MongoDB processed_list
		var existingItem models.ProcessedListItem
		err := database.ProcessedList.FindOne(ctx, bson.M{
			"user_id": problem.UserID,
			"task_id": problem.TaskID,
		}).Decode(&existingItem)
		if err == nil {
			// Item already exists in processed list
			result.Message = "Problem already exists in processed list"
			results = append(results, result)
			continue
		} else if err != mongo.ErrNoDocuments {
			// Database error
			result.Message = "Database error while checking processed list"
			results = append(results, result)
			continue
		}

		// Check if userId and taskId combination already exists in MongoDB problems collection
		var existingProblem bson.M
		err = database.Problems.FindOne(ctx, bson.M{
			"userId": problem.UserID,
			"taskId": problem.TaskID,
		}).Decode(&existingProblem)
		if err == nil {
			// Problem already exists
			result.Message = "Problem already exists"
			results = append(results, result)
			continue
		} else if err != mongo.ErrNoDocuments {
			// Database error
			result.Message = "Database error while checking problems collection"
			results = append(results, result)
			continue
		}

		// Check if userId and taskId combination already exists in Redis queue
		queueKey := fmt.Sprintf("%s:%s", problem.UserID, problem.TaskID)
		existsCount, err := database.RedisClient.Exists(ctx, queueKey).Result()
		if err != nil {
			result.Message = "Redis error while checking queue"
			results = append(results, result)
			continue
		}
		if existsCount > 0 {
			// Item already exists in Redis queue
			result.Message = "Problem already exists in queue"
			results = append(results, result)
			continue
		}

		// Create queue item
		queueItem := QueueItem{
			UserID:    problem.UserID,
			TaskID:    problem.TaskID,
			AccountID: accountID, // MongoDB ObjectID hex string from accounts collection
			UploadIP:  uploadIP,
		}

		// Serialize queue item to JSON
		itemJSON, err := json.Marshal(queueItem)
		if err != nil {
			result.Message = "Failed to serialize queue item"
			results = append(results, result)
			continue
		}

		// Push to Redis queue
		_, err = database.RedisClient.LPush(ctx, queueName, itemJSON).Result()
		if err != nil {
			result.Message = "Failed to push to queue"
			results = append(results, result)
			continue
		}

		// Set a marker in Redis to track this userId:taskId combination
		// This prevents duplicate entries even if the item is still in queue
		err = database.RedisClient.Set(ctx, queueKey, "1", 24*time.Hour).Err()
		if err != nil {
			// Log error but don't fail the request
			// The item is already in the queue
		}

		// Success
		result.Success = true
		result.Message = "Problem uploaded successfully"
		results = append(results, result)
	}

	// Count successful and failed uploads
	successCount := 0
	failedCount := 0
	for _, result := range results {
		if result.Success {
			successCount++
		} else {
			failedCount++
		}
	}

	// Create response with message in the outer level and data directly
	responseMessage := fmt.Sprintf("Bulk upload completed: %d successful, %d failed", successCount, failedCount)
	c.JSON(200, gin.H{
		"code":    0,
		"message": responseMessage,
		"data": gin.H{
			"total":    len(results),
			"success":  successCount,
			"failed":   failedCount,
			"results":  results,
			"uploadIP": uploadIP,
		},
	})
}

// CountItems returns the count of items in Elasticsearch, MongoDB, and Redis queue
func CountItems(c *gin.Context) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	counts := gin.H{
		"elasticsearch": int64(0),
		"mongodb":       int64(0),
		"redis":         int64(0),
	}

	// Count Elasticsearch documents
	indexName := config.AppConfig.Elasticsearch.IndexName
	query := map[string]interface{}{
		"query": map[string]interface{}{
			"match_all": map[string]interface{}{},
		},
	}
	queryJSON, err := json.Marshal(query)
	if err == nil {
		res, err := database.ESClient.Count(
			database.ESClient.Count.WithContext(ctx),
			database.ESClient.Count.WithIndex(indexName),
			database.ESClient.Count.WithBody(bytes.NewReader(queryJSON)),
		)
		if err == nil && !res.IsError() {
			var result map[string]interface{}
			if err := json.NewDecoder(res.Body).Decode(&result); err == nil {
				if count, ok := result["count"].(float64); ok {
					counts["elasticsearch"] = int64(count)
				}
			}
			res.Body.Close()
		}
	}

	// Count MongoDB documents
	mongoCount, err := database.Problems.CountDocuments(ctx, bson.M{})
	if err == nil {
		counts["mongodb"] = mongoCount
	}

	// Count Redis queue items
	queueName := config.AppConfig.Redis.Fields.ProblemsQueue
	redisCount, err := database.RedisClient.LLen(ctx, queueName).Result()
	if err == nil {
		counts["redis"] = redisCount
	}

	utils.SuccessResponse(c, gin.H{
		"counts": counts,
	})
}

// UploadDaily uploads a daily report to the daily queue
// POST /api/problem/upload_daily
func UploadDaily(c *gin.Context) {
	// Get authenticated user ID (MongoDB ObjectID hex string)
	accountID, exists := utils.GetUserID(c)
	if !exists {
		utils.UnauthorizedResponse(c, "User not authenticated")
		return
	}

	// Parse request
	var req UploadDailyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(c, err.Error())
		return
	}

	// Get client IP
	uploadIP := getClientIP(c)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Create daily queue item
	queueItem := DailyQueueItem{
		UserID:    req.UserID,
		DateID:    req.DateID,
		AccountID: accountID,
		UploadIP:  uploadIP,
	}

	// Serialize queue item to JSON
	itemJSON, err := json.Marshal(queueItem)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to serialize queue item")
		return
	}

	// Push to Redis daily queue
	queueName := config.AppConfig.Redis.Fields.DailyQueue
	_, err = database.RedisClient.LPush(ctx, queueName, itemJSON).Result()
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to push to daily queue")
		return
	}

	utils.SuccessResponse(c, gin.H{
		"message": "Daily report uploaded successfully",
		"data": gin.H{
			"userId":   req.UserID,
			"dateId":   req.DateID,
			"uploadIP": uploadIP,
		},
	})
}
