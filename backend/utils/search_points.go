package utils

import (
	"context"
	"time"

	"mtv2/backend/database"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

const (
	CtxKeySearchAccountID = "search_account_id"
	CtxKeySearchIsInGroup = "search_is_in_group"
	MinSearchPoints       = -3
	dailyFreeSearchLimit  = 15
)

// HasDailyFreeSearch uses the UTC+8 calendar shared with weekly scores.
func HasDailyFreeSearch(day string, used int) bool {
	return day != NowSGT().Format("2006-01-02") || used < dailyFreeSearchLimit
}

// FinalizeSearchBilling consumes one daily free search or charges one point
// after a search with results. Empty results and failed searches do neither.
// It sends an error response and returns false if billing cannot complete.
func FinalizeSearchBilling(c *gin.Context, total int64) bool {
	if total <= 0 {
		return true
	}

	isInGroupVal, exists := c.Get(CtxKeySearchIsInGroup)
	if !exists {
		InternalServerErrorResponse(c, "Search account context missing")
		return false
	}
	if isInGroup, _ := isInGroupVal.(bool); isInGroup {
		return true
	}

	accountIDVal, exists := c.Get(CtxKeySearchAccountID)
	accountID, ok := accountIDVal.(primitive.ObjectID)
	if !exists || !ok {
		InternalServerErrorResponse(c, "Search account context missing")
		return false
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	day := NowSGT().Format("2006-01-02")
	// MongoDB atomically checks and reserves one of today's 15 free searches.
	// An update pipeline resets the counter on the first search of a new day.
	freeFilter := bson.M{
		"_id": accountID,
		"$or": bson.A{
			bson.M{"search_free_day": bson.M{"$ne": day}},
			bson.M{"search_free_used": bson.M{"$lt": dailyFreeSearchLimit}},
		},
	}
	usedToday := bson.M{"$cond": bson.A{
		bson.M{"$eq": bson.A{"$search_free_day", day}},
		bson.M{"$ifNull": bson.A{"$search_free_used", 0}},
		0,
	}}
	freeUpdate := bson.A{bson.M{"$set": bson.M{
		"search_free_day":  day,
		"search_free_used": bson.M{"$add": bson.A{usedToday, 1}},
	}}}
	freeResult, err := database.Accounts.UpdateOne(ctx, freeFilter, freeUpdate)
	if err != nil {
		InternalServerErrorResponse(c, "Failed to record free search")
		return false
	}
	if freeResult.MatchedCount > 0 {
		return true
	}

	// The paid update checks the minimum balance atomically too, including
	// when another request used the final free search in the meantime.
	paidResult, err := database.Accounts.UpdateOne(ctx,
		bson.M{"_id": accountID, "points": bson.M{"$gt": MinSearchPoints}},
		bson.M{"$inc": bson.M{"points": -1}},
	)
	if err != nil {
		InternalServerErrorResponse(c, "Failed to deduct search point")
		return false
	}
	if paidResult.MatchedCount == 0 {
		InsufficientPointsResponse(c)
		return false
	}
	return true
}
