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

// CreateAccountRequest is the payload for creating a new account via the admin API.
type CreateAccountRequest struct {
	Username string `json:"username" binding:"required"`
	Email    string `json:"email" binding:"required,email"`
}

// CreateAccount allows an admin to create a new account with is_admin = false.
// The account id is always generated server-side (primitive.NewObjectID); it
// is never accepted from the request.
// POST /api/admin/accounts/create
func CreateAccount(c *gin.Context) {
	if !utils.IsAdmin(c) {
		utils.UnauthorizedResponse(c, "Admin access required")
		return
	}

	var req CreateAccountRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(c, err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Ensure the username is not already in use
	var existingAccount models.Account
	err := database.Accounts.FindOne(ctx, bson.M{"username": req.Username}).Decode(&existingAccount)
	if err == nil {
		utils.ConflictResponse(c, "Username already exists")
		return
	} else if err != mongo.ErrNoDocuments {
		utils.InternalServerErrorResponse(c, "Database error")
		return
	}

	// Ensure the email is not already in use
	err = database.Accounts.FindOne(ctx, bson.M{"email": req.Email}).Decode(&existingAccount)
	if err == nil {
		utils.ConflictResponse(c, "Email already exists")
		return
	} else if err != mongo.ErrNoDocuments {
		utils.InternalServerErrorResponse(c, "Database error")
		return
	}

	// Generate a random initial password, since the admin doesn't supply one.
	initialPassword, err := utils.GenerateRandomPassword(16)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to generate initial password")
		return
	}

	hashedPassword, err := utils.HashPassword(initialPassword)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to hash password")
		return
	}

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

	utils.SuccessResponse(c, gin.H{
		"message": "Account created successfully",
		"user": gin.H{
			"id":       account.ID.Hex(),
			"username": account.Username,
			"email":    account.Email,
			"points":   account.Points,
			"is_admin": account.IsAdmin,
		},
		// Returned once, in plaintext, at creation time only. It is never
		// stored or retrievable again after this response — only its bcrypt
		// hash is persisted. Make sure to relay it to the account owner.
		"initial_password": initialPassword,
	})
}

// QueryAccountRequest is the payload for looking up an account's email and points.
// Exactly one of ID, Username, or Email must be provided.
type QueryAccountRequest struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	Email    string `json:"email"`
}

// QueryAccount allows an admin to look up an account's email and points by id, username, or email.
// POST /api/admin/accounts/query
func QueryAccount(c *gin.Context) {
	if !utils.IsAdmin(c) {
		utils.UnauthorizedResponse(c, "Admin access required")
		return
	}

	var req QueryAccountRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(c, err.Error())
		return
	}

	provided := 0
	filter := bson.M{}
	if req.ID != "" {
		objID, err := primitive.ObjectIDFromHex(req.ID)
		if err != nil {
			utils.BadRequestResponse(c, "Invalid id: must be a valid 24-character hex ObjectID")
			return
		}
		filter["_id"] = objID
		provided++
	}
	if req.Username != "" {
		filter["username"] = req.Username
		provided++
	}
	if req.Email != "" {
		filter["email"] = req.Email
		provided++
	}

	if provided != 1 {
		utils.BadRequestResponse(c, "Provide exactly one of id, username, or email")
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var account models.Account
	err := database.Accounts.FindOne(ctx, filter).Decode(&account)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			utils.NotFoundResponse(c, "Account not found")
			return
		}
		utils.InternalServerErrorResponse(c, "Database error")
		return
	}

	utils.SuccessResponse(c, gin.H{
		"id":       account.ID.Hex(),
		"username": account.Username,
		"email":    account.Email,
		"points":   account.Points,
	})
}
