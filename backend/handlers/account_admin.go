package handlers

import (
	"context"
	"strconv"
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

// leaderboardEntry is a single row of the points leaderboard response.
type leaderboardEntry struct {
	ID       string   `json:"id"`
	Username string   `json:"username"`
	Points   int      `json:"points"`
	Score    *float64 `json:"score"`
}

type leaderboardFacetResult struct {
	Data       []leaderboardEntry `bson:"data"`
	TotalCount []struct {
		Count int64 `bson:"count"`
	} `bson:"totalCount"`
}

// GetPointsLeaderboard returns a paginated points leaderboard built from
// mtv2.accounts left-joined with this week's mtv2.weekly_scores entry
// (matched on weekly_scores.userId == accounts._id and the current ISO
// week/year in Singapore Time). Accounts without a weekly score entry for
// the current week are included with score = null.
// Sorted by score descending (accounts with no score sort last), then by
// points descending.
// Requires authentication only (no admin check).
// GET /api/points/leaderboard?page=1&limit=50
func GetPointsLeaderboard(c *gin.Context) {
	pageStr := c.DefaultQuery("page", "1")
	limitStr := c.DefaultQuery("limit", "50")

	page, err := strconv.Atoi(pageStr)
	if err != nil || page < 1 {
		page = 1
	}

	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit < 1 {
		limit = 50
	}
	if limit > 100 {
		limit = 100 // Max limit
	}

	skip := (page - 1) * limit

	year, weekID := utils.ISOYearWeekSGT()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pipeline := mongo.Pipeline{
		bson.D{{Key: "$lookup", Value: bson.M{
			"from": "weekly_scores",
			"let":  bson.M{"uid": "$_id"},
			"pipeline": mongo.Pipeline{
				bson.D{{Key: "$match", Value: bson.M{
					"$expr": bson.M{"$and": bson.A{
						bson.M{"$eq": bson.A{"$userId", "$$uid"}},
						bson.M{"$eq": bson.A{"$year", year}},
						bson.M{"$eq": bson.A{"$weekId", weekID}},
					}},
				}}},
			},
			"as": "weeklyScore",
		}}},
		bson.D{{Key: "$unwind", Value: bson.M{
			"path":                       "$weeklyScore",
			"preserveNullAndEmptyArrays": true,
		}}},
		bson.D{{Key: "$project", Value: bson.M{
			"_id":      bson.M{"$toString": "$_id"},
			"username": 1,
			"points":   1,
			"score":    bson.M{"$ifNull": bson.A{"$weeklyScore.score", nil}},
		}}},
		bson.D{{Key: "$sort", Value: bson.D{
			{Key: "score", Value: -1},
			{Key: "points", Value: -1},
		}}},
		bson.D{{Key: "$facet", Value: bson.M{
			"data": mongo.Pipeline{
				bson.D{{Key: "$skip", Value: skip}},
				bson.D{{Key: "$limit", Value: limit}},
				bson.D{{Key: "$project", Value: bson.M{
					"id":       "$_id",
					"username": 1,
					"points":   1,
					"score":    1,
				}}},
			},
			"totalCount": mongo.Pipeline{
				bson.D{{Key: "$count", Value: "count"}},
			},
		}}},
	}

	cursor, err := database.Accounts.Aggregate(ctx, pipeline)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to fetch leaderboard")
		return
	}
	defer cursor.Close(ctx)

	var results []leaderboardFacetResult
	if err := cursor.All(ctx, &results); err != nil {
		utils.InternalServerErrorResponse(c, "Failed to decode leaderboard")
		return
	}

	entries := make([]leaderboardEntry, 0)
	var total int64
	if len(results) > 0 {
		entries = results[0].Data
		if entries == nil {
			entries = make([]leaderboardEntry, 0)
		}
		if len(results[0].TotalCount) > 0 {
			total = results[0].TotalCount[0].Count
		}
	}

	totalPages := int64(0)
	if limit > 0 {
		totalPages = (total + int64(limit) - 1) / int64(limit)
	}

	utils.SuccessResponse(c, gin.H{
		"year":       year,
		"weekId":     weekID,
		"rankings":   entries,
		"total":      total,
		"page":       page,
		"limit":      limit,
		"totalPages": totalPages,
	})
}
