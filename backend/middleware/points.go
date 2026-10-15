package middleware

import (
	"mtv2/backend/utils"

	"github.com/gin-gonic/gin"
)

// RequireSearchQuota gates the search endpoints on the 5-hour and weekly
// search quotas of the billed account: the caller itself, or the target of an
// admin-only accountId override. Admin and is_in_group accounts are exempt.
//
// The resolved account is stashed on the context so handlers can finalize
// billing (utils.FinalizeSearchBilling) against the same account that was
// checked here.
func RequireSearchQuota() gin.HandlerFunc {
	return func(c *gin.Context) {
		acct, ok := utils.ResolveSearchAccount(c)
		if !ok {
			c.Abort()
			return
		}
		if !utils.CheckSearchQuota(c, acct) {
			c.Abort()
			return
		}
		c.Next()
	}
}
