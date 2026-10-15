package handlers

import (
	"context"
	"time"

	"mtv2/backend/database"
	"mtv2/backend/models"
	"mtv2/backend/utils"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// GenerateInvitationCode is disabled: generating new invitation codes is no
// longer allowed for any account (including admins). Existing codes stay
// valid for registration and visible via ListInvitations.
func GenerateInvitationCode(c *gin.Context) {
	utils.ForbiddenResponse(c, "邀请码生成功能已关闭，暂不允许生成新的邀请码。")
}

func ListInvitations(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		utils.UnauthorizedResponse(c, "User not authenticated")
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	objID, err := primitive.ObjectIDFromHex(userID.(string))
	if err != nil {
		utils.BadRequestResponse(c, "Invalid user ID")
		return
	}

	// Get user account to fetch points
	var account models.Account
	err = database.Accounts.FindOne(ctx, bson.M{"_id": objID}).Decode(&account)
	if err != nil {
		utils.InternalServerErrorResponse(c, "User not found")
		return
	}

	cursor, err := database.Invitations.Find(ctx, bson.M{"created_by": objID})
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to fetch invitations")
		return
	}
	defer cursor.Close(ctx)

	var invitations []models.Invitation
	if err = cursor.All(ctx, &invitations); err != nil {
		utils.InternalServerErrorResponse(c, "Failed to decode invitations")
		return
	}

	utils.SuccessResponse(c, gin.H{
		"points":      account.Points,
		"invitations": invitations,
	})
}
