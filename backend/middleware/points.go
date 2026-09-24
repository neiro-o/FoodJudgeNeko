package middleware

import (
	"context"
	"time"

	"mtv2/backend/database"
	"mtv2/backend/utils"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// resolveSearchAccountID validates an optional admin-only accountId query
// override and returns the account id that search quota/points should be
// evaluated against (either the override, or the caller's own account id).
func resolveSearchAccountID(c *gin.Context, callerAccountID string) (string, bool) {
	overrideAccountID := c.Query("accountId")
	if overrideAccountID == "" {
		return callerAccountID, true
	}

	if !utils.IsAdmin(c) {
		utils.UnauthorizedResponse(c, "Admin access required to set accountId")
		return "", false
	}

	if _, err := primitive.ObjectIDFromHex(overrideAccountID); err != nil {
		utils.BadRequestResponse(c, "Invalid accountId: must be a valid MongoDB ObjectId")
		return "", false
	}

	return overrideAccountID, true
}

// RequireSearchQuota lets group accounts and accounts with remaining daily
// free searches through. Once the 15 free result-bearing searches are used,
// non-group accounts need points above utils.MinSearchPoints.
//
// The resolved account id and its is_in_group flag are stashed on the
// context (utils.CtxKeySearchAccountID / utils.CtxKeySearchIsInGroup) so
// handlers can finalize billing against the same account that was checked here.
func RequireSearchQuota() gin.HandlerFunc {
	return func(c *gin.Context) {
		callerAccountID, exists := utils.GetUserID(c)
		if !exists {
			utils.UnauthorizedResponse(c, "User not authenticated")
			c.Abort()
			return
		}

		accountIDHex, ok := resolveSearchAccountID(c, callerAccountID)
		if !ok {
			c.Abort()
			return
		}

		objID, err := primitive.ObjectIDFromHex(accountIDHex)
		if err != nil {
			utils.UnauthorizedResponse(c, "Invalid user ID")
			c.Abort()
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		var account struct {
			IsInGroup      bool   `bson:"is_in_group"`
			Points         int    `bson:"points"`
			SearchFreeDay  string `bson:"search_free_day"`
			SearchFreeUsed int    `bson:"search_free_used"`
		}
		if err := database.Accounts.FindOne(ctx, bson.M{"_id": objID}).Decode(&account); err != nil {
			utils.NotFoundResponse(c, "Account not found")
			c.Abort()
			return
		}

		c.Set(utils.CtxKeySearchAccountID, objID)
		c.Set(utils.CtxKeySearchIsInGroup, account.IsInGroup)

		if account.IsInGroup {
			c.Next()
			return
		}

		if account.Points <= utils.MinSearchPoints && !utils.HasDailyFreeSearch(account.SearchFreeDay, account.SearchFreeUsed) {
			utils.InsufficientPointsResponse(c)
			c.Abort()
			return
		}

		c.Next()
	}
}
