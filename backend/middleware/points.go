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

// minSearchPoints is the points threshold at or below which a non-group
// account is blocked from using the search endpoints.
const minSearchPoints = -3

// RequireSearchQuota allows the request through when the account belongs to
// the group (is_in_group == true). Otherwise it checks the account's points
// balance and blocks the request with utils.CodeInsufficientPoints when
// points <= minSearchPoints.
func RequireSearchQuota() gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, exists := utils.GetUserID(c)
		if !exists {
			utils.UnauthorizedResponse(c, "User not authenticated")
			c.Abort()
			return
		}

		objID, err := primitive.ObjectIDFromHex(userID)
		if err != nil {
			utils.UnauthorizedResponse(c, "Invalid user ID in token")
			c.Abort()
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		var account struct {
			IsInGroup bool `bson:"is_in_group"`
			Points    int  `bson:"points"`
		}
		if err := database.Accounts.FindOne(ctx, bson.M{"_id": objID}).Decode(&account); err != nil {
			utils.UnauthorizedResponse(c, "User not found")
			c.Abort()
			return
		}

		if account.IsInGroup {
			c.Next()
			return
		}

		if account.Points <= minSearchPoints {
			utils.InsufficientPointsResponse(c)
			c.Abort()
			return
		}

		c.Next()
	}
}
