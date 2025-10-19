package middleware

import (
	"log"

	"github.com/gin-gonic/gin"
)

func ErrorHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
		// Handle any errors that occurred during request processing
		if len(c.Errors) > 0 {
			err := c.Errors.Last()
			log.Printf("Error: %v", err)
		}
	}
}

func Logger() gin.HandlerFunc {
	return func(c *gin.Context) {
		log.Printf("%s %s %s", c.Request.Method, c.Request.URL.Path, c.ClientIP())
		c.Next()
	}
}
