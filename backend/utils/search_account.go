package utils

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/url"
	"strings"
	"time"

	"mtv2/backend/database"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// SearchAccount is the account a search is billed to: the caller itself, or
// the target of an admin-only accountId override.
type SearchAccount struct {
	ID primitive.ObjectID
	// Unlimited accounts (admins and is_in_group accounts) have no quota.
	Unlimited bool
}

// ResolveSearchAccount validates the optional admin-only accountId query
// override and loads the account whose search quota applies. On failure it
// writes the error response and returns false (callers should abort).
func ResolveSearchAccount(c *gin.Context) (SearchAccount, bool) {
	callerID, exists := GetUserID(c)
	if !exists {
		UnauthorizedResponse(c, "User not authenticated")
		return SearchAccount{}, false
	}

	accountIDHex := callerID
	if override := c.Query("accountId"); override != "" {
		if !IsAdmin(c) {
			UnauthorizedResponse(c, "Admin access required to set accountId")
			return SearchAccount{}, false
		}
		if _, err := primitive.ObjectIDFromHex(override); err != nil {
			BadRequestResponse(c, "Invalid accountId: must be a valid MongoDB ObjectId")
			return SearchAccount{}, false
		}
		accountIDHex = override
	}

	objID, err := primitive.ObjectIDFromHex(accountIDHex)
	if err != nil {
		UnauthorizedResponse(c, "Invalid user ID")
		return SearchAccount{}, false
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var account struct {
		IsAdmin   bool `bson:"is_admin"`
		IsInGroup bool `bson:"is_in_group"`
	}
	if err := database.Accounts.FindOne(ctx, bson.M{"_id": objID}).Decode(&account); err != nil {
		NotFoundResponse(c, "Account not found")
		return SearchAccount{}, false
	}

	return SearchAccount{ID: objID, Unlimited: account.IsAdmin || account.IsInGroup}, true
}

// searchDedupKey identifies "the same search by the same billed account":
// route plus the query string without the accountId override. It lets a
// repeated request (e.g. a prefetch followed by the explicit submit) reuse the
// quota unit that the first request already consumed.
func searchDedupKey(c *gin.Context, accountID primitive.ObjectID) string {
	query := url.Values{}
	for key, values := range c.Request.URL.Query() {
		if key == "accountId" {
			continue
		}
		for _, value := range values {
			query.Add(key, strings.TrimSpace(value))
		}
	}
	sum := sha256.Sum256([]byte(c.FullPath() + "?" + query.Encode()))
	return searchDedupPrefix + accountID.Hex() + ":" + hex.EncodeToString(sum[:])
}
