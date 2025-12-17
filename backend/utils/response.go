package utils

import (
	"context"
	"errors"
	"net/http"
	"time"

	"mtv2/backend/database"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Response represents the standard API response format
type Response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// SuccessResponse sends a successful response
func SuccessResponse(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "success",
		Data:    data,
	})
}

// ErrorResponse sends an error response
func ErrorResponse(c *gin.Context, code int, message string) {
	c.JSON(http.StatusOK, Response{
		Code:    code,
		Message: message,
	})
}

// BadRequestResponse sends a bad request error response
func BadRequestResponse(c *gin.Context, message string) {
	ErrorResponse(c, 400, message)
}

// UnauthorizedResponse sends an unauthorized error response
func UnauthorizedResponse(c *gin.Context, message string) {
	ErrorResponse(c, 401, message)
}

// InternalServerErrorResponse sends an internal server error response
func InternalServerErrorResponse(c *gin.Context, message string) {
	ErrorResponse(c, 500, message)
}

// ConflictResponse sends a conflict error response
func ConflictResponse(c *gin.Context, message string) {
	ErrorResponse(c, 409, message)
}

// NotFoundResponse sends a not found error response
func NotFoundResponse(c *gin.Context, message string) {
	ErrorResponse(c, 404, message)
}

// GetUserID returns the authenticated user's MongoDB ObjectID as a hex string
// Returns empty string and false if user is not authenticated
func GetUserID(c *gin.Context) (string, bool) {
	userID, exists := c.Get("user_id")
	if !exists {
		return "", false
	}
	userIDStr, ok := userID.(string)
	if !ok {
		return "", false
	}
	return userIDStr, true
}

// GetUserObjectID returns the authenticated user's MongoDB ObjectID
// Returns error if user is not authenticated or ID is invalid
func GetUserObjectID(c *gin.Context) (primitive.ObjectID, error) {
	userID, exists := GetUserID(c)
	if !exists {
		return primitive.NilObjectID, errors.New("user not authenticated")
	}
	return primitive.ObjectIDFromHex(userID)
}

// IsAdmin checks if the authenticated user is an admin
// Returns false if user is not authenticated or not an admin
func IsAdmin(c *gin.Context) bool {
	userID, exists := GetUserID(c)
	if !exists {
		return false
	}

	objID, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		return false
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var account struct {
		IsAdmin bool `bson:"is_admin"`
	}
	err = database.Accounts.FindOne(ctx, bson.M{"_id": objID}).Decode(&account)
	if err != nil {
		return false
	}

	return account.IsAdmin
}
