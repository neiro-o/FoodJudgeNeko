package middleware

import (
	"context"
	"encoding/json"
	"io"
	"mtv2/backend/config"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"mtv2/backend/database"
	"mtv2/backend/utils"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// getClientIP extracts the client IP address from the request
func getClientIP(c *gin.Context) string { return c.ClientIP() }

func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization header required"})
			c.Abort()
			return
		}

		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid authorization header format"})
			c.Abort()
			return
		}

		if config.AppConfig.PrimaryAPIURL != "" {
			remoteAuthenticate(c, authHeader)
			return
		}
		token := parts[1]
		claims, err := utils.ValidateToken(token)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired token"})
			c.Abort()
			return
		}

		// Check if token is blacklisted in Redis
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		blacklisted, err := database.RedisClient.Exists(ctx, "blacklist:token:"+token).Result()
		if err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Authentication service unavailable"})
			c.Abort()
			return
		}
		if blacklisted > 0 {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Token has been invalidated"})
			c.Abort()
			return
		}

		// Validate token version against database
		objID, err := primitive.ObjectIDFromHex(claims.UserID)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid user ID in token"})
			c.Abort()
			return
		}

		var account struct {
			TokenVersion int  `bson:"token_version"`
			IsAdmin      bool `bson:"is_admin"`
		}
		err = database.Accounts.FindOne(ctx, bson.M{"_id": objID}).Decode(&account)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "User not found"})
			c.Abort()
			return
		}

		if claims.TokenVersion != account.TokenVersion {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Token version mismatch - please login again"})
			c.Abort()
			return
		}

		c.Set("user_id", claims.UserID)
		c.Set("username", claims.Username)
		c.Set(utils.CtxKeyIsAdmin, account.IsAdmin)
		c.Next()
	}
}

var authHTTPClient = &http.Client{Timeout: 3 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}

func remoteAuthenticate(c *gin.Context, authorization string) {
	fail := func(status int) {
		c.JSON(status, gin.H{"error": "Authentication service unavailable or rejected credentials"})
		c.Abort()
	}
	base, err := url.Parse(config.AppConfig.PrimaryAPIURL)
	if err != nil || (base.Scheme != "http" && base.Scheme != "https") || base.User != nil || base.Host == "" {
		fail(http.StatusServiceUnavailable)
		return
	}
	host := base.Hostname()
	ip := net.ParseIP(host)
	if host != "localhost" && (ip == nil || (!ip.IsLoopback() && !ip.IsPrivate())) {
		fail(http.StatusServiceUnavailable)
		return
	}
	base.Path = "/api/auth/verify"
	base.RawPath = ""
	base.RawQuery = ""
	base.Fragment = ""
	ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base.String(), nil)
	if err != nil {
		fail(http.StatusServiceUnavailable)
		return
	}
	req.Header.Set("Authorization", authorization)
	req.Header.Set("X-Forwarded-For", c.ClientIP())
	response, err := authHTTPClient.Do(req)
	if err != nil {
		fail(http.StatusServiceUnavailable)
		return
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusUnauthorized || response.StatusCode == http.StatusForbidden {
		fail(http.StatusUnauthorized)
		return
	}
	if response.StatusCode != http.StatusOK {
		fail(http.StatusServiceUnavailable)
		return
	}
	var result struct {
		Code *int `json:"code"`
		Data *struct {
			UserID   string `json:"user_id"`
			Username string `json:"username"`
			IsAdmin  *bool  `json:"is_admin"`
		} `json:"data"`
	}
	dec := json.NewDecoder(io.LimitReader(response.Body, 16384))
	if dec.Decode(&result) != nil || result.Code == nil || *result.Code != 0 || result.Data == nil || result.Data.Username == "" || result.Data.IsAdmin == nil {
		fail(http.StatusServiceUnavailable)
		return
	}
	var extra interface{}
	if dec.Decode(&extra) != io.EOF {
		fail(http.StatusServiceUnavailable)
		return
	}
	if _, err := primitive.ObjectIDFromHex(result.Data.UserID); err != nil {
		fail(http.StatusServiceUnavailable)
		return
	}
	c.Set("user_id", result.Data.UserID)
	c.Set("username", result.Data.Username)
	c.Set(utils.CtxKeyIsAdmin, *result.Data.IsAdmin)
	c.Next()
}
