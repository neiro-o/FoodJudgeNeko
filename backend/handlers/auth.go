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
	"go.mongodb.org/mongo-driver/mongo"
)

type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type ChangePasswordRequest struct {
	OldPassword string `json:"old_password" binding:"required"`
	NewPassword string `json:"new_password" binding:"required"`
}

type RegisterRequest struct {
	Username   string `json:"username" binding:"required"`
	Password   string `json:"password" binding:"required"`
	Email      string `json:"email" binding:"required,email"`
	InviteCode string `json:"invite_code" binding:"required"`
}

func Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(c, err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var account models.Account
	err := database.Accounts.FindOne(ctx, bson.M{"username": req.Username}).Decode(&account)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			utils.UnauthorizedResponse(c, "Invalid username or password")
			return
		}
		utils.InternalServerErrorResponse(c, "Database error")
		return
	}

	if !utils.CheckPasswordHash(req.Password, account.Password) {
		utils.UnauthorizedResponse(c, "Invalid username or password")
		return
	}

	token, err := utils.GenerateToken(account.ID.Hex(), account.Username)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to generate token")
		return
	}

	utils.SuccessResponse(c, gin.H{
		"token": token,
		"user": gin.H{
			"id":       account.ID.Hex(),
			"username": account.Username,
			"email":    account.Email,
			"points":   account.Points,
			"is_admin": account.IsAdmin,
		},
	})
}

func Logout(c *gin.Context) {
	// Since we're using JWT tokens, logout is handled client-side by discarding the token
	// In a production system, you might want to maintain a blacklist of tokens
	utils.SuccessResponse(c, gin.H{"message": "Logged out successfully"})
}

func ChangePassword(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		utils.UnauthorizedResponse(c, "User not authenticated")
		return
	}

	var req ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(c, err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	objID, err := primitive.ObjectIDFromHex(userID.(string))
	if err != nil {
		utils.BadRequestResponse(c, "Invalid user ID")
		return
	}

	var account models.Account
	err = database.Accounts.FindOne(ctx, bson.M{"_id": objID}).Decode(&account)
	if err != nil {
		utils.InternalServerErrorResponse(c, "User not found")
		return
	}

	if !utils.CheckPasswordHash(req.OldPassword, account.Password) {
		utils.UnauthorizedResponse(c, "Invalid old password")
		return
	}

	hashedPassword, err := utils.HashPassword(req.NewPassword)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to hash password")
		return
	}

	_, err = database.Accounts.UpdateOne(
		ctx,
		bson.M{"_id": objID},
		bson.M{"$set": bson.M{"password": hashedPassword}},
	)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to update password")
		return
	}

	utils.SuccessResponse(c, gin.H{"message": "Password changed successfully"})
}

func Register(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(c, err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Check if invitation code is valid and unused
	var invitation models.Invitation
	err := database.Invitations.FindOne(ctx, bson.M{"invite_code": req.InviteCode, "used": false}).Decode(&invitation)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			utils.BadRequestResponse(c, "Invalid or already used invitation code")
			return
		}
		utils.InternalServerErrorResponse(c, "Database error")
		return
	}

	// Check if username already exists
	var existingAccount models.Account
	err = database.Accounts.FindOne(ctx, bson.M{"username": req.Username}).Decode(&existingAccount)
	if err == nil {
		utils.ConflictResponse(c, "Username already exists")
		return
	} else if err != mongo.ErrNoDocuments {
		utils.InternalServerErrorResponse(c, "Database error")
		return
	}

	// Check if email already exists
	err = database.Accounts.FindOne(ctx, bson.M{"email": req.Email}).Decode(&existingAccount)
	if err == nil {
		utils.ConflictResponse(c, "Email already exists")
		return
	} else if err != mongo.ErrNoDocuments {
		utils.InternalServerErrorResponse(c, "Database error")
		return
	}

	// Hash password
	hashedPassword, err := utils.HashPassword(req.Password)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to hash password")
		return
	}

	// Create account
	account := models.Account{
		ID:        primitive.NewObjectID(),
		Username:  req.Username,
		Password:  hashedPassword,
		Email:     req.Email,
		Points:    0,
		IsAdmin:   false,
		CreatedAt: time.Now(),
	}

	_, err = database.Accounts.InsertOne(ctx, account)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to create account")
		return
	}

	// Mark invitation as used
	_, err = database.Invitations.UpdateOne(
		ctx,
		bson.M{"_id": invitation.ID},
		bson.M{"$set": bson.M{"used": true, "used_by": account.ID}},
	)
	if err != nil {
		// Log error but don't fail registration
		// In production, you might want to handle this more carefully
	}

	utils.SuccessResponse(c, gin.H{
		"message": "Account created successfully",
		"user": gin.H{
			"id":       account.ID.Hex(),
			"username": account.Username,
			"email":    account.Email,
			"points":   account.Points,
			"is_admin": account.IsAdmin,
		},
	})
}
