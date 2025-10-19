package controllers

import (
	"context"
	"time"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"golang.org/x/crypto/bcrypt"
	"mt-backend/config"
	"mt-backend/middleware"
	"mt-backend/models"
	"mt-backend/utils"
)

// Register creates a new user account
func Register(c *gin.Context) {
	var req models.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(c, err.Error())
		return
	}

	ctx := context.Background()
	cfg := config.GetConfig()
	userCollection := config.MongoDB.Collection(cfg.MongoDB.Collections.Users)
	inviteCollection := config.MongoDB.Collection(cfg.MongoDB.Collections.InviteCodes)

	var existingUser models.User
	err := userCollection.FindOne(ctx, bson.M{
		"$or": []bson.M{
			{"username": req.Username},
			{"email": req.Email},
		},
	}).Decode(&existingUser)
	if err == nil {
		utils.ConflictResponse(c, utils.MsgUsernameOrEmailExists)
		return
	}

	// Validate invite code
	var inviteCode models.InviteCode
	err = inviteCollection.FindOne(ctx, bson.M{"invite_code": req.InviteCode}).Decode(&inviteCode)
	if err != nil {
		utils.BadRequestResponse(c, utils.MsgInvalidInviteCode)
		return
	}

	// Check if invite code is expired
	if time.Now().After(inviteCode.ExpireTime) {
		utils.BadRequestResponse(c, utils.MsgInviteCodeExpired)
		return
	}

	// Check if invite code is already used
	if inviteCode.Used {
		utils.BadRequestResponse(c, utils.MsgInviteCodeUsed)
		return
	}

	// Hash password using bcrypt
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		utils.InternalServerErrorResponse(c, utils.MsgFailedToHashPassword)
		return
	}

	// Start transaction
	session, err := config.MongoDB.Client().StartSession()
	if err != nil {
		utils.InternalServerErrorResponse(c, "failed to start database session")
		return
	}
	defer session.EndSession(ctx)

	// Execute transaction
	var result interface{}
	err = mongo.WithSession(ctx, session, func(sc mongo.SessionContext) error {
		// Start transaction
		if err := session.StartTransaction(); err != nil {
			return err
		}

		// Create user in MongoDB
		user := models.User{
			Username:     req.Username,
			PasswordHash: string(passwordHash),
			Email:        req.Email,
			Role:         "user",
			InvitedBy:    &inviteCode.UserID,
			CreatedAt:    time.Now(),
			UpdatedAt:    time.Now(),
		}

		userResult, err := userCollection.InsertOne(sc, user)
		if err != nil {
			session.AbortTransaction(sc)
			return err
		}

		// Mark invite code as used
		_, err = inviteCollection.UpdateOne(
			sc,
			bson.M{"_id": inviteCode.ID},
			bson.M{"$set": bson.M{"used": true}},
		)
		if err != nil {
			session.AbortTransaction(sc)
			return err
		}

		// Commit transaction
		if err := session.CommitTransaction(sc); err != nil {
			return err
		}

		result = userResult.InsertedID
		return nil
	})

	if err != nil {
		utils.InternalServerErrorResponse(c, "failed to create user account")
		return
	}

	utils.SuccessResponse(c, utils.MsgUserRegistered, gin.H{
		"user_id": result.(primitive.ObjectID).Hex(),
	})
}

// Login authenticates user and returns session ID
func Login(c *gin.Context) {
	var req models.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(c, err.Error())
		return
	}

	// Find user in MongoDB
	ctx := context.Background()
	cfg := config.GetConfig()
	collection := config.MongoDB.Collection(cfg.MongoDB.Collections.Users)

	var user models.User
	err := collection.FindOne(ctx, bson.M{"username": req.Username}).Decode(&user)
	if err != nil {
		utils.UnauthorizedResponse(c, utils.MsgInvalidCredentials)
		return
	}

	// Verify password hash
	err = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password))
	if err != nil {
		utils.UnauthorizedResponse(c, utils.MsgInvalidCredentials)
		return
	}

	// Generate session
	session, err := middleware.CreateSession(
		user.ID,
		c.ClientIP(),
		c.GetHeader("User-Agent"),
		cfg.Session.MaxAge,
	)
	if err != nil {
		utils.InternalServerErrorResponse(c, utils.MsgFailedToCreateSession)
		return
	}

	// Set session cookie
	c.SetCookie(
		"session_id",
		session.SessionID,
		cfg.Session.MaxAge,
		"/",
		"",
		cfg.Session.SecureCookie,
		cfg.Session.HttpOnly,
	)

	utils.SuccessResponse(c, utils.MsgUserLoginSuccess, models.LoginResponse{
		SessionID: session.SessionID,
		UserID:    user.ID.Hex(),
		Username:  user.Username,
		ExpiresAt: session.ExpiresAt.Unix(),
	})
}

// Logout invalidates user's session
func Logout(c *gin.Context) {
	sessionID, exists := c.Get("session_id")
	if !exists {
		utils.BadRequestResponse(c, utils.MsgNoActiveSession)
		return
	}

	// Invalidate session
	err := middleware.InvalidateSession(sessionID.(string))
	if err != nil {
		utils.InternalServerErrorResponse(c, utils.MsgFailedToLogout)
		return
	}

	// Clear cookie
	c.SetCookie("session_id", "", -1, "/", "", false, true)

	utils.SuccessResponse(c, utils.MsgUserLogoutSuccess, nil)
}

// ChangePassword updates user's password
func ChangePassword(c *gin.Context) {
	userID, ok := middleware.GetUserID(c)
	if !ok {
		utils.ForbiddenResponse(c, utils.MsgUnauthorized)
		return
	}

	var req models.ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(c, err.Error())
		return
	}

	// Find user in MongoDB
	ctx := context.Background()
	cfg := config.GetConfig()
	collection := config.MongoDB.Collection(cfg.MongoDB.Collections.Users)

	var user models.User
	err := collection.FindOne(ctx, bson.M{"_id": userID}).Decode(&user)
	if err != nil {
		utils.NotFoundResponse(c, utils.MsgUserNotFound)
		return
	}

	// Verify old password
	err = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.OldPassword))
	if err != nil {
		utils.UnauthorizedResponse(c, utils.MsgIncorrectOldPassword)
		return
	}

	// Hash new password
	newPasswordHash, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		utils.InternalServerErrorResponse(c, utils.MsgFailedToHashPassword)
		return
	}

	// Update password in MongoDB
	_, err = collection.UpdateOne(
		ctx,
		bson.M{"_id": userID},
		bson.M{"$set": bson.M{
			"password_hash": string(newPasswordHash),
			"updated_at":    time.Now(),
		}},
	)
	if err != nil {
		utils.InternalServerErrorResponse(c, utils.MsgFailedToUpdatePassword)
		return
	}

	utils.SuccessResponse(c, utils.MsgPasswordChanged, nil)
}
