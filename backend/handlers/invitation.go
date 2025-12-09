package handlers

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"time"

	"mtv2/backend/database"
	"mtv2/backend/models"
	"mtv2/backend/utils"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// generateSecureInviteCode generates a cryptographically secure random invite code
func generateSecureInviteCode() (string, error) {
	// Generate 32 random bytes (256 bits of entropy)
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	// Encode to base64 URL-safe format and remove padding
	// This gives us a 43-character string with high entropy
	code := base64.URLEncoding.WithPadding(base64.NoPadding).EncodeToString(bytes)
	return code, nil
}

type GenerateInviteRequest struct {
	Count int `json:"count" binding:"min=1,max=10"`
}

func GenerateInvitationCode(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		utils.UnauthorizedResponse(c, "User not authenticated")
		return
	}

	var req GenerateInviteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		// Default to 1 if not provided
		req.Count = 1
	} else if req.Count == 0 {
		req.Count = 1
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	objID, err := primitive.ObjectIDFromHex(userID.(string))
	if err != nil {
		utils.BadRequestResponse(c, "Invalid user ID")
		return
	}

	// Get user account to check admin status and points
	var account models.Account
	err = database.Accounts.FindOne(ctx, bson.M{"_id": objID}).Decode(&account)
	if err != nil {
		utils.InternalServerErrorResponse(c, "User not found")
		return
	}

	// Calculate cost: 100 points per invite code for non-admin users
	const pointsPerCode = 100
	totalCost := req.Count * pointsPerCode

	// Check if user has enough points (only for non-admin users)
	if !account.IsAdmin {
		if account.Points < totalCost {
			utils.BadRequestResponse(c, "Insufficient points. Each invite code costs 100 points.")
			return
		}

		// Deduct points from user account
		newPoints := account.Points - totalCost
		_, err = database.Accounts.UpdateOne(
			ctx,
			bson.M{"_id": objID},
			bson.M{"$set": bson.M{"points": newPoints}},
		)
		if err != nil {
			utils.InternalServerErrorResponse(c, "Failed to deduct points")
			return
		}
	}

	// Generate invitation codes
	codes := make([]string, 0, req.Count)
	invitations := make([]interface{}, 0, req.Count)

	for i := 0; i < req.Count; i++ {
		inviteCode, err := generateSecureInviteCode()
		if err != nil {
			// If invitation creation fails, we should rollback points deduction
			if !account.IsAdmin {
				// Attempt to restore points
				_, _ = database.Accounts.UpdateOne(
					ctx,
					bson.M{"_id": objID},
					bson.M{"$set": bson.M{"points": account.Points}},
				)
			}
			utils.InternalServerErrorResponse(c, "Failed to generate secure invite code")
			return
		}
		codes = append(codes, inviteCode)

		invitation := models.Invitation{
			ID:         primitive.NewObjectID(),
			InviteCode: inviteCode,
			CreatedBy:  objID,
			CreatedAt:  time.Now(),
			Used:       false,
		}
		invitations = append(invitations, invitation)
	}

	_, err = database.Invitations.InsertMany(ctx, invitations)
	if err != nil {
		// If invitation creation fails, we should rollback points deduction
		// For simplicity, we'll just return an error
		// In production, consider using MongoDB transactions
		if !account.IsAdmin {
			// Attempt to restore points
			_, _ = database.Accounts.UpdateOne(
				ctx,
				bson.M{"_id": objID},
				bson.M{"$set": bson.M{"points": account.Points}},
			)
		}
		utils.InternalServerErrorResponse(c, "Failed to generate invitation codes")
		return
	}

	utils.SuccessResponse(c, gin.H{
		"message": "Invitation codes generated successfully",
		"codes":   codes,
		"count":   req.Count,
	})
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
