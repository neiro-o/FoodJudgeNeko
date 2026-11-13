package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"mtv2/backend/config"
	"mtv2/backend/database"
	"mtv2/backend/handlers"
	"mtv2/backend/middleware"
	"mtv2/backend/utils"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func main() {
	// Load configuration
	if err := config.LoadConfig(config.Path()); err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}
	if err := config.AppConfig.Validate(); err != nil {
		log.Fatal(err)
	}
	if len(os.Args) > 1 && os.Args[1] == "check-config" {
		fmt.Println("Configuration is valid")
		return
	}

	// Connect to MongoDB
	if err := database.Connect(); err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer database.Disconnect()

	// Connect to Redis
	if err := database.ConnectRedis(); err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}
	defer database.DisconnectRedis()

	// Connect to Elasticsearch
	if err := database.ConnectElasticsearch(); err != nil {
		log.Fatalf("Failed to connect to Elasticsearch: %v", err)
	}
	defer database.DisconnectElasticsearch()

	// Setup router
	r := gin.Default()

	// Configure CORS
	allowedOrigins := config.AppConfig.Server.AllowedOrigins
	if len(allowedOrigins) == 0 {
		// Default to localhost for development
		allowedOrigins = []string{"http://localhost:3000", "http://127.0.0.1:3000"}
	}

	log.Printf("CORS allowed origins: %v", allowedOrigins)

	r.Use(cors.New(cors.Config{
		AllowOrigins:     allowedOrigins,
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Length", "Content-Type", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	// Add logging middleware to debug CORS issues
	r.Use(func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if origin != "" {
			log.Printf("Request from origin: %s, method: %s, path: %s", origin, c.Request.Method, c.Request.URL.Path)
		}
		c.Next()
	})

	// Health check
	r.GET("/health", func(c *gin.Context) {
		utils.SuccessResponse(c, gin.H{"status": "ok"})
	})
	r.GET("/ready", func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
		defer cancel()
		if err := database.Ready(ctx); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "unavailable"})
			return
		}
		utils.SuccessResponse(c, gin.H{"status": "ok"})
	})

	// Public routes
	api := r.Group("/api")
	{
		api.POST("/login", handlers.Login)
		api.POST("/register", handlers.Register)
		// Media endpoints are public but validate hash (hash generation is protected)
		api.GET("/media/image", handlers.LoadImage)
		api.GET("/media/video", handlers.LoadVideo)
		api.GET("/media/audio", handlers.LoadAudio)
		api.GET("/media/random_image", handlers.LoadRandomImage)
		api.GET("/account/avatar", handlers.GetAccountAvatar)
		// Avatar endpoint accepts token via query parameter (for use in <img src="">)
		api.GET("/user_detail/avatar", handlers.GetUserAvatar)
	}

	// Protected routes
	protected := api.Group("/")
	protected.Use(middleware.AuthMiddleware())
	{
		protected.POST("/logout", handlers.Logout)
		protected.POST("/change-password", handlers.ChangePassword)
		protected.POST("/invitations/generate", handlers.GenerateInvitationCode)
		protected.GET("/invitations", handlers.ListInvitations)
		protected.POST("/problem/upload", handlers.UploadProblem)
		protected.POST("/problem/upload-multiple", handlers.UploadMultipleProblems)
		protected.POST("/problem/upload_daily", handlers.UploadDaily)
		protected.POST("/problem/refresh-comments/:mongo_id", handlers.RefreshProblemComments)
		protected.GET("/problem/search", middleware.ProblemRateLimit("/api/problem/search", 4), middleware.RequireSearchQuota(), handlers.Search)
		protected.GET("/problem/quicksearch", middleware.RequireSearchQuota(), handlers.QuickSearch)
		protected.GET("/bot/search", middleware.RequireSearchQuota(), handlers.BotSearch)
		// Quota query: read-only, never consumes quota.
		protected.GET("/problem/quota", handlers.GetSearchQuota)
		protected.GET("/problem/recent", middleware.ProblemRateLimit("/api/problem/recent", 3), handlers.GetRecentProblems)
		protected.GET("/problem/count", handlers.CountItems)
		// Short alias used by the search landing-page count switcher.
		protected.GET("/count", handlers.CountItems)
		protected.GET("/problem/by-esid/:id", middleware.ProblemRateLimit("/api/problem/by-esid/:id", 1), handlers.SearchByESID)
		protected.GET("/problem/by-mongoid/:id", middleware.ProblemRateLimit("/api/problem/by-mongoid/:id", 1), handlers.SearchByMongoID)
		protected.GET("/problem/uploaders/:id", handlers.GetProblemUploaders)
		protected.GET("/problem/comments/:problemId", handlers.GetProblemComments)
		protected.POST("/media/hash", handlers.GenerateMediaHash)

		// Notes routes
		protected.GET("/notes/search", handlers.SearchNotes)

		// User detail routes
		protected.GET("/user_detail/account_points", handlers.GetAccountPoints)
		protected.GET("/user_detail/user_info", handlers.GetUserInfo)
		protected.GET("/user_detail/comments", handlers.GetUserComments)
		protected.GET("/user_detail/vanished/:userId", handlers.GetVanishedComments)
		protected.GET("/user_detail/rankings", handlers.GetRankings)
		protected.GET("/user_detail/search_users", handlers.SearchUsers)
		protected.POST("/user_detail/toggle_malicious", handlers.ToggleMaliciousUser)
		protected.GET("/user_detail/ai_summary", handlers.GetAIUserSummary)
		protected.POST("/user_detail/ai_summary", handlers.PostAIUserSummary)

		// Admin account management routes
		protected.POST("/admin/accounts/create", handlers.CreateAccount)
		protected.POST("/admin/accounts/query", handlers.QueryAccount)

		// Weekly score ranking (any authenticated account)
		protected.GET("/weekly_scores/ranking", handlers.GetWeeklyScoreRanking)

		// Comment statistics (any authenticated account; no search quota)
		protected.GET("/stats/wordcloud", handlers.GetWordCloud)
		protected.GET("/stats/wordcloud/comments", handlers.GetWordCloudComments)

		// Points leaderboard (any authenticated account)
		protected.GET("/points/leaderboard", handlers.GetPointsLeaderboard)
	}

	// Start server
	port := net.JoinHostPort(config.AppConfig.Server.Host, fmt.Sprint(config.AppConfig.Server.Port))
	fmt.Printf("Server starting on port %s\n", port)
	server := &http.Server{Addr: port, Handler: r, ReadHeaderTimeout: 10 * time.Second}
	stopped, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() {
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatal(err)
		}
	}()
	<-stopped.Done()
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		log.Printf("HTTP shutdown: %v", err)
	}
}
