package routes

import (
	"github.com/gin-gonic/gin"
	"mt-backend/controllers"
	"mt-backend/middleware"
)

func SetupRoutes(router *gin.Engine) {
	// User routes
	user := router.Group("/api/user")
	{
		user.POST("/register", controllers.Register)
		user.POST("/login", controllers.Login)
		user.POST("/logout", middleware.AuthRequired(), controllers.Logout)
		user.POST("/changepasswd", middleware.AuthRequired(), controllers.ChangePassword)
	}

	// Problem routes (auth required)
	problem := router.Group("/api/problems", middleware.AuthRequired())
	{
		problem.POST("/search", controllers.SearchProblems)
		problem.POST("/add", controllers.AddProblems)
		problem.GET("/detail/:id", controllers.GetProblemDetail)
	}

	// AI routes (auth required)
	ai := router.Group("/api/llm", middleware.AuthRequired())
	{
		ai.POST("/answer", controllers.GetLLMAnswer)
	}
}
