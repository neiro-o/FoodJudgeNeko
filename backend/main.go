package main

import (
	"log"

	"github.com/gin-gonic/gin"
	"mt-backend/config"
	"mt-backend/middleware"
	"mt-backend/routes"
)

func main() {
	cfg := config.LoadConfig() // Initial load in main.go is fine

	config.InitMongoDB(cfg)
	config.InitES(cfg)
	config.InitRedis(cfg)

	router := gin.Default()

	router.Use(middleware.Logger())
	router.Use(middleware.ErrorHandler())

	routes.SetupRoutes(router)

	port := cfg.Server.Port
	log.Printf("Starting server on port %s", port)
	if err := router.Run(":" + port); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
