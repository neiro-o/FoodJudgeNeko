package utils

// Response codes for API responses
const (
	// Success codes
	CodeSuccess = 0

	// Client error codes (4xx)
	CodeBadRequest        = 400
	CodeUnauthorized      = 401
	CodeForbidden         = 403
	CodeNotFound          = 404
	CodeConflict          = 409
	CodeValidationError   = 422

	// Server error codes (5xx)
	CodeInternalServerError = 500
	CodeDatabaseError       = 501
	CodeSessionError        = 502
	CodePasswordHashError   = 503
)

// Error messages for common scenarios
const (
	MsgSuccess                = "success"
	MsgBadRequest             = "bad request"
	MsgUnauthorized           = "unauthorized"
	MsgForbidden              = "forbidden"
	MsgNotFound              = "not found"
	MsgConflict               = "conflict"
	MsgValidationError        = "validation error"
	MsgInternalServerError    = "internal server error"
	MsgDatabaseError          = "database error"
	MsgSessionError           = "session error"
	MsgPasswordHashError      = "password hash error"

	// User specific messages
	MsgUserRegistered         = "user registered successfully"
	MsgUserLoginSuccess       = "login successful"
	MsgUserLogoutSuccess      = "logged out successfully"
	MsgPasswordChanged        = "password changed successfully"
	MsgUsernameExists         = "username already exists"
	MsgInvalidCredentials     = "invalid username or password"
	MsgUserNotFound           = "user not found"
	MsgIncorrectOldPassword   = "incorrect old password"
	MsgNoActiveSession        = "no active session"
	MsgFailedToCreateUser     = "failed to create user"
	MsgFailedToCreateSession  = "failed to create session"
	MsgFailedToLogout         = "failed to logout"
	MsgFailedToUpdatePassword = "failed to update password"
	MsgFailedToHashPassword   = "failed to hash password"

	// Invite code specific messages
	MsgUsernameOrEmailExists  = "username or email already exists"
	MsgInvalidInviteCode      = "invalid invite code"
	MsgInviteCodeExpired      = "invalid invite code"
	MsgInviteCodeUsed         = "invalid invite code"
	MsgFailedToMarkInviteUsed = "failed to mark invite code as used"

	// Search specific messages
	MsgSearchSuccess          = "search completed successfully"
	MsgInvalidPageSize        = "page size must be between 10 and 25"
	MsgSearchFailed           = "search failed"
	MsgElasticsearchError     = "elasticsearch error"
)
