package utils

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Response represents the standard API response format
type Response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Content interface{} `json:"content,omitempty"`
}

// SuccessResponse creates a successful response
func SuccessResponse(c *gin.Context, message string, content interface{}) {
	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: message,
		Content: content,
	})
}

// ErrorResponse creates an error response
func ErrorResponse(c *gin.Context, httpStatus int, code int, message string) {
	c.JSON(httpStatus, Response{
		Code:    code,
		Message: message,
	})
}

// BadRequestResponse creates a bad request response
func BadRequestResponse(c *gin.Context, message string) {
	ErrorResponse(c, http.StatusBadRequest, 400, message)
}

// UnauthorizedResponse creates an unauthorized response
func UnauthorizedResponse(c *gin.Context, message string) {
	ErrorResponse(c, http.StatusUnauthorized, 401, message)
}

// ForbiddenResponse creates a forbidden response
func ForbiddenResponse(c *gin.Context, message string) {
	ErrorResponse(c, http.StatusForbidden, 403, message)
}

// NotFoundResponse creates a not found response
func NotFoundResponse(c *gin.Context, message string) {
	ErrorResponse(c, http.StatusNotFound, 404, message)
}

// ConflictResponse creates a conflict response
func ConflictResponse(c *gin.Context, message string) {
	ErrorResponse(c, http.StatusConflict, 409, message)
}

// InternalServerErrorResponse creates an internal server error response
func InternalServerErrorResponse(c *gin.Context, message string) {
	ErrorResponse(c, http.StatusInternalServerError, 500, message)
}
