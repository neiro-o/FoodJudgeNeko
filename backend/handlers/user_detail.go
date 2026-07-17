package handlers

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"mtv2/backend/database"
	"mtv2/backend/utils"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// validateTokenFromQuery validates the token passed as query parameter
// Returns true if valid, false otherwise
func validateTokenFromQuery(c *gin.Context) bool {
	token := c.Query("token")
	if token == "" {
		return false
	}

	claims, err := utils.ValidateToken(token)
	if err != nil {
		return false
	}

	// Note: intentionally not validating claims.IPAddress against the current
	// client IP here. Unlike AuthMiddleware (used for normal Bearer-token
	// requests), this endpoint used to reject requests whenever the client's
	// IP changed (mobile network switch, VPN, carrier NAT reassignment, etc.),
	// causing avatars to silently fall back to the default image. Keep the
	// validation consistent with AuthMiddleware: signature + blacklist +
	// token version only.

	// Check if token is blacklisted in Redis
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	blacklisted, err := database.RedisClient.Exists(ctx, "blacklist:token:"+token).Result()
	if err == nil && blacklisted > 0 {
		return false
	}

	// Validate token version against database
	objID, err := primitive.ObjectIDFromHex(claims.UserID)
	if err != nil {
		return false
	}

	var account struct {
		TokenVersion int `bson:"token_version"`
	}
	err = database.Accounts.FindOne(ctx, bson.M{"_id": objID}).Decode(&account)
	if err != nil {
		return false
	}

	if claims.TokenVersion != account.TokenVersion {
		return false
	}

	return true
}

// getAvatarCachePath returns the cache file path for an avatar
func getAvatarCachePath(userID string, ext string) (string, error) {
	if ext == "" {
		ext = ".jpg"
	}
	filename := fmt.Sprintf("avatar_%s%s", userID, ext)

	// Cache directory relative to backend directory
	cacheDir := filepath.Join("cache", "img")
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create cache directory: %w", err)
	}

	return filepath.Join(cacheDir, filename), nil
}

// getExtensionFromURL extracts file extension from URL
func getExtensionFromURL(urlStr string) string {
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return ".jpg"
	}

	path := parsedURL.Path
	ext := filepath.Ext(path)
	if ext == "" {
		return ".jpg"
	}
	return ext
}

// downloadAvatar downloads an avatar image from URL
func downloadAvatar(avatarURL, cachePath string) error {
	req, err := http.NewRequest("GET", avatarURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Set Referer header
	req.Header.Set("Referer", "https://zqt.meituan.com")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to download avatar: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to download avatar: status code %d", resp.StatusCode)
	}

	// Create cache file
	file, err := os.Create(cachePath)
	if err != nil {
		return fmt.Errorf("failed to create cache file: %w", err)
	}
	defer file.Close()

	// Copy response body to file
	_, err = io.Copy(file, resp.Body)
	if err != nil {
		return fmt.Errorf("failed to write cache file: %w", err)
	}

	return nil
}

// GetUserAvatar returns the user's avatar image
// GET /api/user_detail/avatar?userId=xxx&token=xxx
// Token is passed via query parameter since this endpoint is used in <img src="">
func GetUserAvatar(c *gin.Context) {
	// Validate token from query parameter
	if !validateTokenFromQuery(c) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid or missing token"})
		return
	}

	userID := c.Query("userId")
	if userID == "" {
		utils.BadRequestResponse(c, "Missing userId parameter")
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Check if cached avatar exists
	// First check with common extensions
	extensions := []string{".jpg", ".png", ".jpeg", ".gif", ".webp"}
	for _, ext := range extensions {
		cachePath, err := getAvatarCachePath(userID, ext)
		if err != nil {
			continue
		}
		if _, err := os.Stat(cachePath); err == nil {
			// Serve cached file
			c.File(cachePath)
			return
		}
	}

	// No cache found, search in MongoDB
	var comment bson.M
	err := database.Comments.FindOne(ctx, bson.M{
		"userId":      userID,
		"isAnonymous": false,
	}).Decode(&comment)

	if err != nil {
		if err == mongo.ErrNoDocuments {
			utils.NotFoundResponse(c, "No non-anonymous comment found for this user")
			return
		}
		utils.InternalServerErrorResponse(c, "Database error")
		return
	}

	// Get userPic
	userPic, ok := comment["userPic"].(string)
	if !ok || userPic == "" {
		utils.NotFoundResponse(c, "User has no avatar")
		return
	}

	// Determine extension from URL
	ext := getExtensionFromURL(userPic)
	cachePath, err := getAvatarCachePath(userID, ext)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to get cache path")
		return
	}

	// Download and cache the avatar
	if err := downloadAvatar(userPic, cachePath); err != nil {
		utils.InternalServerErrorResponse(c, "Failed to download avatar: "+err.Error())
		return
	}

	// Serve the cached file
	c.File(cachePath)
}

// UserInfoResponse represents the response for user info
type UserInfoResponse struct {
	UserName  string `json:"userName"`
	Likes     int64  `json:"likes"`
	Replies   int64  `json:"replies"`
	Malicious bool   `json:"malicious"`
}

// GetUserInfo returns user information including name, total likes, and total replies
// GET /api/user_detail/user_info?userId=xxx
func GetUserInfo(c *gin.Context) {
	userID := c.Query("userId")
	if userID == "" {
		utils.BadRequestResponse(c, "Missing userId parameter")
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// First, find a non-anonymous comment to get the userName
	var comment bson.M
	err := database.Comments.FindOne(ctx, bson.M{
		"userId":      userID,
		"isAnonymous": false,
	}).Decode(&comment)

	if err != nil {
		if err == mongo.ErrNoDocuments {
			utils.NotFoundResponse(c, "No non-anonymous comment found for this user")
			return
		}
		utils.InternalServerErrorResponse(c, "Database error")
		return
	}

	userName, _ := comment["userName"].(string)

	// Aggregate to get sum of approveCount and replyTotal for all comments (including anonymous)
	pipeline := mongo.Pipeline{
		{{Key: "$match", Value: bson.M{"userId": userID}}},
		{{Key: "$group", Value: bson.M{
			"_id":          nil,
			"totalLikes":   bson.M{"$sum": "$approveCount"},
			"totalReplies": bson.M{"$sum": "$replyTotal"},
		}}},
	}

	cursor, err := database.Comments.Aggregate(ctx, pipeline)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to aggregate user stats")
		return
	}
	defer cursor.Close(ctx)

	var totalLikes int64 = 0
	var totalReplies int64 = 0

	if cursor.Next(ctx) {
		var result bson.M
		if err := cursor.Decode(&result); err == nil {
			if likes, ok := result["totalLikes"]; ok {
				switch v := likes.(type) {
				case int32:
					totalLikes = int64(v)
				case int64:
					totalLikes = v
				case float64:
					totalLikes = int64(v)
				}
			}
			if replies, ok := result["totalReplies"]; ok {
				switch v := replies.(type) {
				case int32:
					totalReplies = int64(v)
				case int64:
					totalReplies = v
				case float64:
					totalReplies = int64(v)
				}
			}
		}
	}

	// Check if user is malicious
	isMalicious := false
	var maliciousDoc bson.M
	err = database.Malicious.FindOne(ctx, bson.M{"userId": userID}).Decode(&maliciousDoc)
	if err == nil {
		isMalicious = true
	}

	utils.SuccessResponse(c, gin.H{
		"userName":  userName,
		"likes":     totalLikes,
		"replies":   totalReplies,
		"malicious": isMalicious,
	})
}

// CommentItem represents a comment in the response
type CommentItem struct {
	ID           string `json:"id"`
	ProblemID    string `json:"problemId"`
	CommentID    string `json:"commentId"`
	UserID       string `json:"userId"`
	UserName     string `json:"userName"`
	UserPic      string `json:"userPic"`
	CreateTime   int64  `json:"createTime"`
	Content      string `json:"content"`
	ApproveCount int64  `json:"approveCount"`
	ReplyTotal   int64  `json:"replyTotal"`
	IsAnonymous  bool   `json:"isAnonymous"`
	VoteOperate  string `json:"voteOperate"`
	Choice       int    `json:"choice"`
}

// GetUserComments returns paginated comments for a user
// GET /api/user_detail/comments?userId=xxx&page=1&limit=10
func GetUserComments(c *gin.Context) {
	userID := c.Query("userId")
	if userID == "" {
		utils.BadRequestResponse(c, "Missing userId parameter")
		return
	}

	// Parse pagination parameters
	pageStr := c.DefaultQuery("page", "1")
	limitStr := c.DefaultQuery("limit", "10")

	page, err := strconv.Atoi(pageStr)
	if err != nil || page < 1 {
		page = 1
	}

	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit < 1 {
		limit = 10
	}
	if limit > 100 {
		limit = 100 // Max limit
	}

	skip := (page - 1) * limit

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Build query
	filter := bson.M{"userId": userID}

	// Get total count
	totalCount, err := database.Comments.CountDocuments(ctx, filter)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to count comments")
		return
	}

	// Find comments with sorting and pagination
	findOptions := options.Find().
		SetSort(bson.D{
			{Key: "approveCount", Value: -1},
			{Key: "createTime", Value: -1},
		}).
		SetSkip(int64(skip)).
		SetLimit(int64(limit))

	cursor, err := database.Comments.Find(ctx, filter, findOptions)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to find comments")
		return
	}
	defer cursor.Close(ctx)

	var comments []gin.H
	for cursor.Next(ctx) {
		var doc bson.M
		if err := cursor.Decode(&doc); err != nil {
			continue
		}

		comment := gin.H{}

		// Convert ObjectId to string for id
		if id, ok := doc["_id"].(primitive.ObjectID); ok {
			comment["id"] = id.Hex()
		}

		// Copy other fields
		if v, ok := doc["problemId"].(string); ok {
			comment["problemId"] = v
		}
		if v, ok := doc["commentId"].(string); ok {
			comment["commentId"] = v
		}
		if v, ok := doc["userId"].(string); ok {
			comment["userId"] = v
		}
		if v, ok := doc["userName"].(string); ok {
			comment["userName"] = v
		}
		if v, ok := doc["userPic"].(string); ok {
			comment["userPic"] = v
		}
		if v, ok := doc["content"].(string); ok {
			comment["content"] = v
		}
		if v, ok := doc["voteOperate"].(string); ok {
			comment["voteOperate"] = v
		}

		// Handle createTime (could be int32, int64, or float64)
		if v, ok := doc["createTime"]; ok {
			switch ct := v.(type) {
			case int32:
				comment["createTime"] = int64(ct)
			case int64:
				comment["createTime"] = ct
			case float64:
				comment["createTime"] = int64(ct)
			case string:
				if parsed, err := strconv.ParseInt(ct, 10, 64); err == nil {
					comment["createTime"] = parsed
				}
			}
		}

		// Handle approveCount
		if v, ok := doc["approveCount"]; ok {
			switch ac := v.(type) {
			case int32:
				comment["approveCount"] = int64(ac)
			case int64:
				comment["approveCount"] = ac
			case float64:
				comment["approveCount"] = int64(ac)
			}
		}

		// Handle replyTotal
		if v, ok := doc["replyTotal"]; ok {
			switch rt := v.(type) {
			case int32:
				comment["replyTotal"] = int64(rt)
			case int64:
				comment["replyTotal"] = rt
			case float64:
				comment["replyTotal"] = int64(rt)
			}
		}

		// Handle isAnonymous
		if v, ok := doc["isAnonymous"].(bool); ok {
			comment["isAnonymous"] = v
		}

		// Handle choice
		if v, ok := doc["choice"]; ok {
			switch ch := v.(type) {
			case int32:
				comment["choice"] = int(ch)
			case int64:
				comment["choice"] = int(ch)
			case float64:
				comment["choice"] = int(ch)
			}
		}

		comment["images"] = toStringSlice(doc["images"])
		comment["audios"] = toCommentAudios(doc["audios"])

		comments = append(comments, comment)
	}

	if comments == nil {
		comments = []gin.H{}
	}

	// Calculate total pages
	totalPages := (totalCount + int64(limit) - 1) / int64(limit)

	utils.SuccessResponse(c, gin.H{
		"comments":   comments,
		"total":      totalCount,
		"page":       page,
		"limit":      limit,
		"totalPages": totalPages,
	})
}

// Helper function to check if domain is allowed for avatar
func isAvatarDomainAllowed(urlStr string) bool {
	allowedDomains := []string{
		"meituan.com",
		"meituan.net",
		"sankuai.com",
	}

	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return false
	}

	host := strings.ToLower(parsedURL.Host)
	// Remove port if present
	if idx := strings.Index(host, ":"); idx != -1 {
		host = host[:idx]
	}

	for _, allowedDomain := range allowedDomains {
		if host == allowedDomain || strings.HasSuffix(host, "."+allowedDomain) {
			return true
		}
	}

	return false
}

// RankingItem represents a user in the rankings
type RankingItem struct {
	UserID       string `json:"userId"`
	UserName     string `json:"userName"`
	Likes        int64  `json:"likes"`
	CommentCount int64  `json:"commentCount"`
	Rank         int64  `json:"rank"`
}

// rankingsPageSize is the fixed number of users returned per rankings page.
const rankingsPageSize = 100

// toInt64 converts common BSON numeric types to int64
func toInt64(v interface{}) int64 {
	switch val := v.(type) {
	case int32:
		return int64(val)
	case int64:
		return val
	case float64:
		return int64(val)
	default:
		return 0
	}
}

// GetRankings returns users ranked by total likes, paginated at 100 per page.
// The ranking data itself is precomputed by setup/comment_sync.py
// (sync_user_rankings) into the user_rankings collection, so this endpoint
// only does a cheap indexed lookup instead of aggregating the full
// comments collection on every request.
// GET /api/user_detail/rankings?page=1
func GetRankings(c *gin.Context) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	page, err := strconv.Atoi(c.DefaultQuery("page", "1"))
	if err != nil || page < 1 {
		page = 1
	}

	total, err := database.UserRankings.CountDocuments(ctx, bson.M{})
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to count rankings")
		return
	}

	totalPages := int64(0)
	if total > 0 {
		totalPages = (total + rankingsPageSize - 1) / rankingsPageSize
	}

	// Clamp page to the last available page once we know how many exist,
	// so an out-of-range page doesn't just return an empty list.
	if totalPages > 0 && int64(page) > totalPages {
		page = int(totalPages)
	}

	findOptions := options.Find().
		SetSort(bson.M{"rank": 1}).
		SetSkip(int64(page-1) * rankingsPageSize).
		SetLimit(rankingsPageSize)

	cursor, err := database.UserRankings.Find(ctx, bson.M{}, findOptions)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to fetch rankings")
		return
	}
	defer cursor.Close(ctx)

	var rankings []gin.H
	for cursor.Next(ctx) {
		var doc bson.M
		if err := cursor.Decode(&doc); err != nil {
			continue
		}

		item := gin.H{
			"userId":       "",
			"userName":     "",
			"likes":        int64(0),
			"commentCount": int64(0),
			"rank":         int64(0),
		}

		if v, ok := doc["userId"].(string); ok {
			item["userId"] = v
		}
		if v, ok := doc["userName"].(string); ok {
			item["userName"] = v
		}
		if v, ok := doc["likes"]; ok {
			item["likes"] = toInt64(v)
		}
		if v, ok := doc["commentCount"]; ok {
			item["commentCount"] = toInt64(v)
		}
		if v, ok := doc["rank"]; ok {
			item["rank"] = toInt64(v)
		}

		rankings = append(rankings, item)
	}

	if rankings == nil {
		rankings = []gin.H{}
	}

	utils.SuccessResponse(c, gin.H{
		"rankings":   rankings,
		"total":      total,
		"page":       page,
		"pageSize":   rankingsPageSize,
		"totalPages": totalPages,
	})
}

// ToggleMaliciousUser toggles the malicious status of a user (admin only)
// POST /api/user_detail/toggle_malicious?userId=xxx
func ToggleMaliciousUser(c *gin.Context) {
	// Check if user is admin
	if !utils.IsAdmin(c) {
		utils.UnauthorizedResponse(c, "Admin access required")
		return
	}

	targetUserID := c.Query("userId")
	if targetUserID == "" {
		utils.BadRequestResponse(c, "Missing userId parameter")
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Check if user is currently marked as malicious
	var maliciousDoc bson.M
	err := database.Malicious.FindOne(ctx, bson.M{"userId": targetUserID}).Decode(&maliciousDoc)

	isCurrentlyMalicious := err == nil

	if isCurrentlyMalicious {
		// Remove from malicious collection
		_, err = database.Malicious.DeleteOne(ctx, bson.M{"userId": targetUserID})
		if err != nil {
			utils.InternalServerErrorResponse(c, "Failed to remove malicious tag")
			return
		}
		utils.SuccessResponse(c, gin.H{
			"malicious": false,
			"message":   "User untagged as malicious",
		})
	} else {
		// Add to malicious collection
		maliciousDoc = bson.M{
			"userId":    targetUserID,
			"tagged":    true,
			"tagged_at": time.Now(),
			"tagged_by": "admin_manual",
		}
		_, err = database.Malicious.ReplaceOne(
			ctx,
			bson.M{"userId": targetUserID},
			maliciousDoc,
			options.Replace().SetUpsert(true),
		)
		if err != nil {
			utils.InternalServerErrorResponse(c, "Failed to tag user as malicious")
			return
		}
		utils.SuccessResponse(c, gin.H{
			"malicious": true,
			"message":   "User tagged as malicious",
		})
	}
}

// searchUsersMaxLimit caps how many distinct users a single search can
// return, so an overly broad keyword can't trigger an unbounded response.
const searchUsersMaxLimit = 20

// SearchUsers finds distinct users whose nickname (as it appears on any of
// their comments) matches the given keyword, case-insensitive substring
// match. A user may have posted under slightly different nicknames over
// time; each match still resolves to one row per userId (first nickname
// seen), since userId is what the rest of the user_detail API keys off of.
// GET /api/user_detail/search_users?keyword=xxx&limit=20
func SearchUsers(c *gin.Context) {
	keyword := strings.TrimSpace(c.Query("keyword"))
	if keyword == "" {
		utils.BadRequestResponse(c, "Missing keyword parameter")
		return
	}

	limit, err := strconv.Atoi(c.DefaultQuery("limit", "10"))
	if err != nil || limit < 1 {
		limit = 10
	}
	if limit > searchUsersMaxLimit {
		limit = searchUsersMaxLimit
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// QuoteMeta escapes any regex metacharacters in the user-supplied
	// keyword so it's always treated as a literal substring, never as an
	// (expensive or malformed) regex pattern.
	pattern := regexp.QuoteMeta(keyword)

	pipeline := mongo.Pipeline{
		{{Key: "$match", Value: bson.M{
			"isAnonymous": false,
			"userName":    bson.M{"$regex": pattern, "$options": "i"},
		}}},
		{{Key: "$group", Value: bson.M{
			"_id":      "$userId",
			"userName": bson.M{"$first": "$userName"},
		}}},
		{{Key: "$limit", Value: limit}},
	}

	cursor, err := database.Comments.Aggregate(ctx, pipeline)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to search users")
		return
	}
	defer cursor.Close(ctx)

	users := []gin.H{}
	for cursor.Next(ctx) {
		var row struct {
			UserID   string `bson:"_id"`
			UserName string `bson:"userName"`
		}
		if err := cursor.Decode(&row); err != nil {
			continue
		}
		if row.UserID == "" {
			continue
		}
		users = append(users, gin.H{
			"userId":   row.UserID,
			"userName": row.UserName,
		})
	}

	utils.SuccessResponse(c, gin.H{"users": users})
}
