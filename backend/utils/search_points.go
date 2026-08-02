package utils

import (
	"context"
	"time"

	"mtv2/backend/database"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Context keys set by middleware.RequireSearchQuota to record which account
// a search request was evaluated against (the caller, or an admin-specified
// accountId override), so handlers can bill points against the same account
// after a successful search.
const (
	CtxKeySearchAccountID = "search_account_id"
	CtxKeySearchIsInGroup = "search_is_in_group"
)

// DeductSearchPointIfApplicable decrements the resolved search account's
// points by 1 when both are true:
//   - the account is not in the group (is_in_group == false), and
//   - the search returned at least one result (total > 0).
//
// It is a no-op when total <= 0 (no results found), when the account is in
// the group, or when the request never went through
// middleware.RequireSearchQuota (no context values set).
func DeductSearchPointIfApplicable(c *gin.Context, total int64) {
	if total <= 0 {
		return
	}

	isInGroupVal, exists := c.Get(CtxKeySearchIsInGroup)
	if !exists {
		return
	}
	if isInGroup, _ := isInGroupVal.(bool); isInGroup {
		return
	}

	accountIDVal, exists := c.Get(CtxKeySearchAccountID)
	if !exists {
		return
	}
	accountID, ok := accountIDVal.(primitive.ObjectID)
	if !ok {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Errors are intentionally swallowed: the search itself already
	// succeeded and this bookkeeping should not fail the response.
	_, _ = database.Accounts.UpdateOne(ctx, bson.M{"_id": accountID}, bson.M{"$inc": bson.M{"points": -1}})
}
