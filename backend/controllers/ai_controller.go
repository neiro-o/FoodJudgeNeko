package controllers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"mt-backend/middleware"
	"mt-backend/models"
	"mt-backend/utils"
)

// GetLLMAnswer generates AI answer for a problem using LLM
func GetLLMAnswer(c *gin.Context) {
	userID, ok := middleware.GetUserID(c)
	if !ok {
		utils.ForbiddenResponse(c, utils.MsgUnauthorized)
		return
	}

	var req models.LLMAnswerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(c, err.Error())
		return
	}

	// TODO: Fetch problem from MongoDB
	// TODO: Call LLM service with problem context
	// TODO: Store answer in MongoDB
	// TODO: Index answer in Elasticsearch for search
	// TODO: Return generated answer

	utils.SuccessResponse(c, "GetLLMAnswer endpoint", gin.H{
		"user_id":    userID.Hex(),
		"problem_id": req.ProblemID,
	})
}
