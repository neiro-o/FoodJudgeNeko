package handlers

import (
	"context"
	"strconv"
	"time"

	"mtv2/backend/database"
	"mtv2/backend/utils"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

// weeklyScoreRankingEntry is a single row of the weekly score ranking response.
type weeklyScoreRankingEntry struct {
	UserID   string  `json:"userId"`
	Username string  `json:"username"`
	Email    string  `json:"email"`
	Points   int     `json:"points"`
	Score    float64 `json:"score"`
}

// GetWeeklyScoreRanking returns the weekly score ranking (sorted score desc)
// for a given ISO year/week, joined with account info (username/email/points).
// Any authenticated account may call this (no admin check).
// GET /api/weekly_scores/ranking?year=2026&weekId=31
func GetWeeklyScoreRanking(c *gin.Context) {
	year, weekID := parseYearWeekParams(c)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pipeline := mongo.Pipeline{
		bson.D{{Key: "$match", Value: bson.M{"year": year, "weekId": weekID}}},
		bson.D{{Key: "$lookup", Value: bson.M{
			"from":         "accounts",
			"localField":   "userId",
			"foreignField": "_id",
			"as":           "account",
		}}},
		bson.D{{Key: "$unwind", Value: "$account"}},
		bson.D{{Key: "$project", Value: bson.M{
			"_id":      0,
			"userId":   bson.M{"$toString": "$userId"},
			"username": "$account.username",
			"email":    "$account.email",
			"points":   "$account.points",
			"score":    1,
		}}},
		bson.D{{Key: "$sort", Value: bson.M{"score": -1}}},
	}

	cursor, err := database.WeeklyScores.Aggregate(ctx, pipeline)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to fetch weekly score ranking")
		return
	}
	defer cursor.Close(ctx)

	rankings := make([]weeklyScoreRankingEntry, 0)
	if err := cursor.All(ctx, &rankings); err != nil {
		utils.InternalServerErrorResponse(c, "Failed to decode weekly score ranking")
		return
	}

	utils.SuccessResponse(c, gin.H{
		"year":     year,
		"weekId":   weekID,
		"rankings": rankings,
	})
}

// parseYearWeekParams reads the optional "year" and "weekId" query params,
// defaulting to the current ISO year/week when not provided or invalid.
func parseYearWeekParams(c *gin.Context) (int, int) {
	nowYear, nowWeek := time.Now().ISOWeek()

	year := nowYear
	if yearStr := c.Query("year"); yearStr != "" {
		if v, err := strconv.Atoi(yearStr); err == nil {
			year = v
		}
	}

	weekID := nowWeek
	if weekStr := c.Query("weekId"); weekStr != "" {
		if v, err := strconv.Atoi(weekStr); err == nil {
			weekID = v
		}
	}

	return year, weekID
}
