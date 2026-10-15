package handlers

import (
	"context"
	"time"

	"mtv2/backend/utils"

	"github.com/gin-gonic/gin"
)

// GetSearchQuota returns the 5-hour and weekly search quota (limit / used /
// remaining), the next reset time and whether searching is currently blocked.
// It never consumes quota. Admins may pass accountId to inspect another
// account; admin and is_in_group accounts report unlimited = true.
// GET /api/problem/quota[?accountId=<ObjectId>]
func GetSearchQuota(c *gin.Context) {
	acct, ok := utils.ResolveSearchAccount(c)
	if !ok {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	status, _, err := utils.GetSearchQuotaStatus(ctx, acct)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to load search quota")
		return
	}
	utils.SuccessResponse(c, status)
}
