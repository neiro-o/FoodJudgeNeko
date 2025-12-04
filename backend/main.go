package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"mtv2/backend/config"
	"mtv2/backend/database"
	"mtv2/backend/handlers"
	"mtv2/backend/middleware"
	"mtv2/backend/utils"
)

func main() {
	// Load configuration
	configPath := filepath.Join("..", "config.yml")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		// Try current directory
		configPath = "config.yml"
	}

	if err := config.LoadConfig(configPath); err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Connect to MongoDB
	if err := database.Connect(); err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer database.Disconnect()

	// Setup router
	r := gin.Default()

	// Configure CORS
	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"http://localhost:3000", "http://127.0.0.1:3000"},
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Length", "Content-Type", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	// Health check
	r.GET("/health", func(c *gin.Context) {
		utils.SuccessResponse(c, gin.H{"status": "ok"})
	})

	// Public routes
	api := r.Group("/api")
	{
		api.POST("/login", handlers.Login)
		api.POST("/register", handlers.Register)
	}

	// Protected routes
	protected := api.Group("/")
	protected.Use(middleware.AuthMiddleware())
	{
		protected.POST("/logout", handlers.Logout)
		protected.POST("/change-password", handlers.ChangePassword)
		protected.POST("/invitations/generate", handlers.GenerateInvitationCode)
		protected.GET("/invitations", handlers.ListInvitations)
	}

	// Start server
	port := fmt.Sprintf(":%d", config.AppConfig.Server.Port)
	fmt.Printf("Server starting on port %s\n", port)
	if err := r.Run(port); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

