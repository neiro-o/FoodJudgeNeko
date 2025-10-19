package controllers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"mt-backend/middleware"
	"mt-backend/models"
)

// GetLLMAnswer generates AI answer for a problem using LLM
func GetLLMAnswer(c *gin.Context) {
	userID, ok := middleware.GetUserID(c)
	if !ok {
		c.JSON(http.StatusForbidden, gin.H{"error": "unauthorized"})
		return
	}

	var req models.LLMAnswerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// TODO: Fetch problem from MongoDB
	// TODO: Call LLM service with problem context
	// TODO: Store answer in MongoDB
	// TODO: Index answer in Elasticsearch for search
	// TODO: Return generated answer

	c.JSON(http.StatusOK, gin.H{
		"message":    "GetLLMAnswer endpoint",
		"user_id":    userID.Hex(),
		"problem_id": req.ProblemID,
	})
}
