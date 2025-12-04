package utils

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Response represents the standard API response format
type Response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// SuccessResponse sends a successful response
func SuccessResponse(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "success",
		Data:    data,
	})
}

// ErrorResponse sends an error response
func ErrorResponse(c *gin.Context, code int, message string) {
	c.JSON(http.StatusOK, Response{
		Code:    code,
		Message: message,
	})
}

// BadRequestResponse sends a bad request error response
func BadRequestResponse(c *gin.Context, message string) {
	ErrorResponse(c, 400, message)
}

// UnauthorizedResponse sends an unauthorized error response
func UnauthorizedResponse(c *gin.Context, message string) {
	ErrorResponse(c, 401, message)
}

// InternalServerErrorResponse sends an internal server error response
func InternalServerErrorResponse(c *gin.Context, message string) {
	ErrorResponse(c, 500, message)
}

// ConflictResponse sends a conflict error response
func ConflictResponse(c *gin.Context, message string) {
	ErrorResponse(c, 409, message)
}
