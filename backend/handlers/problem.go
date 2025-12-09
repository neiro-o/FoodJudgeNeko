package handlers

import (
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

func UploadProblem(c *gin.Context) {
	// Get authenticated user ID
	accountID, exists := c.Get("user_id")
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
	uploadIP := c.ClientIP()

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
		AccountID: accountID.(string),
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
	// Get authenticated user ID
	accountID, exists := c.Get("user_id")
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
	uploadIP := c.ClientIP()

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
			AccountID: accountID.(string),
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

	utils.SuccessResponse(c, gin.H{
		"message": fmt.Sprintf("Bulk upload completed: %d successful, %d failed", successCount, failedCount),
		"data": gin.H{
			"total":    len(results),
			"success":  successCount,
			"failed":   failedCount,
			"results":  results,
			"uploadIP": uploadIP,
		},
	})
}
