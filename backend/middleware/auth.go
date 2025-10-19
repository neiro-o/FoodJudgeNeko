package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// AuthRequired middleware validates session and extracts user_id
func AuthRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		sessionID := extractSessionID(c)
		if sessionID == "" {
			c.JSON(http.StatusForbidden, gin.H{"error": "missing or invalid session"})
			c.Abort()
			return
		}

		userID, valid := ValidateSession(sessionID)
		if !valid {
			c.JSON(http.StatusForbidden, gin.H{"error": "session expired or invalid"})
			c.Abort()
			return
		}

		// Store user_id in context for later use
		c.Set("user_id", userID)
		c.Set("session_id", sessionID)
		c.Next()
	}
}

// extractSessionID retrieves session ID from Authorization header or cookie
func extractSessionID(c *gin.Context) string {
	// Try Authorization header first (Bearer <session_id>)
	authHeader := c.GetHeader("Authorization")
	if authHeader != "" {
		parts := strings.Split(authHeader, " ")
		if len(parts) == 2 && parts[0] == "Bearer" {
			return parts[1]
		}
	}

	// Fallback to cookie
	sessionID, err := c.Cookie("session_id")
	if err == nil && sessionID != "" {
		return sessionID
	}

	return ""
}

// GetUserID retrieves user_id from context (must be called after AuthRequired)
func GetUserID(c *gin.Context) (primitive.ObjectID, bool) {
	userID, exists := c.Get("user_id")
	if !exists {
		return primitive.NilObjectID, false
	}
	
	objectID, ok := userID.(primitive.ObjectID)
	return objectID, ok
}
