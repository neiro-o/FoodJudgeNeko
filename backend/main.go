package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"strings"
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
	configPath := filepath.Join("..", "config.yml")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		// Try current directory
		configPath = "config.yml"
	}

	if v := os.Getenv("MTV2_CONFIG"); v != "" {
		configPath = v
	}
	if err := config.LoadConfig(configPath); err != nil {
		log.Fatalf("Failed to load config: %v", err)
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
	if err := r.SetTrustedProxies(config.AppConfig.Server.TrustedProxies); err != nil {
		log.Fatal(err)
	}

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

	if config.AppConfig.PrimaryAPIURL != "" {
		target, err := url.Parse(config.AppConfig.PrimaryAPIURL)
		if err != nil || (target.Scheme != "http" && target.Scheme != "https") || target.Host == "" {
			log.Fatal("invalid PRIMARY_API_URL")
		}
		r.Use(primaryProxy(target, config.AppConfig.PrimaryReadPrefixes, config.AppConfig.PrimaryReadPaths, config.AppConfig.Server.TrustedProxies))
	}
	r.GET("/ready", func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
		defer cancel()
		if database.Client.Ping(ctx, nil) != nil || database.StateDB.Client().Ping(ctx, nil) != nil || database.RedisClient.Ping(ctx).Err() != nil {
			c.Status(http.StatusServiceUnavailable)
			return
		}
		res, err := database.ESClient.Info(database.ESClient.Info.WithContext(ctx))
		if err != nil {
			c.Status(http.StatusServiceUnavailable)
			return
		}
		defer res.Body.Close()
		if res.IsError() {
			c.Status(http.StatusServiceUnavailable)
			return
		}
		c.Status(http.StatusOK)
	})

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
		protected.GET("/auth/verify", func(c *gin.Context) {
			uid, _ := c.Get("user_id")
			name, _ := c.Get("username")
			admin, _ := c.Get(utils.CtxKeyIsAdmin)
			utils.SuccessResponse(c, gin.H{"user_id": uid, "username": name, "is_admin": admin})
		})
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

		// Points leaderboard (any authenticated account)
		protected.GET("/points/leaderboard", handlers.GetPointsLeaderboard)
	}

	// Start server
	port := net.JoinHostPort(config.AppConfig.Server.Host, fmt.Sprint(config.AppConfig.Server.Port))
	fmt.Printf("Server starting on port %s\n", port)
	if err := r.Run(port); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

func shouldProxyRead(path string, prefixes, paths []string) bool {
	if prefixes == nil {
		prefixes = []string{"/api/admin/", "/api/points/", "/api/weekly_scores/"}
	}
	if paths == nil {
		paths = []string{"/api/user_detail/account_points", "/api/problem/quota"}
	}
	for _, p := range prefixes {
		if p != "" && strings.HasPrefix(path, p) {
			return true
		}
	}
	for _, p := range paths {
		if p == path {
			return true
		}
	}
	return false
}

func shouldProxy(path, method string, prefixes, paths []string) bool {
	if !strings.HasPrefix(path, "/api/") || path == "/api/user_detail/ai_summary" || method == "OPTIONS" {
		return false
	}
	return (method != "GET" && method != "HEAD") || shouldProxyRead(path, prefixes, paths)
}

func primaryProxy(target *url.URL, prefixes, paths, trusted []string) gin.HandlerFunc {
	proxy := &httputil.ReverseProxy{Rewrite: func(pr *httputil.ProxyRequest) {
		pr.SetURL(target)
		pr.Out.Host = pr.In.Host
		pr.Out.Header.Set("X-Forwarded-For", pr.In.Header.Get("X-MTV2-Verified-IP"))
		pr.Out.Header.Del("X-MTV2-Verified-IP")
		pr.Out.Header.Set("X-Forwarded-Proto", pr.In.Header.Get("X-MTV2-Verified-Proto"))
		pr.Out.Header.Del("X-MTV2-Verified-Proto")
	}}
	return func(c *gin.Context) {
		path := c.Request.URL.Path
		method := c.Request.Method
		if shouldProxy(path, method, prefixes, paths) {
			ip := c.ClientIP()
			proto := "http"
			host, _, _ := net.SplitHostPort(c.Request.RemoteAddr)
			if trustedProxy(host, trusted) && c.GetHeader("X-Forwarded-Proto") == "https" {
				proto = "https"
			}
			c.Request.Header.Set("X-MTV2-Verified-IP", ip)
			c.Request.Header.Set("X-MTV2-Verified-Proto", proto)
			proxy.ServeHTTP(c.Writer, c.Request)
			c.Abort()
			return
		}
		c.Next()
	}
}

func trustedProxy(host string, proxies []string) bool {
	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}
	for _, p := range proxies {
		if v := net.ParseIP(p); v != nil && v.Equal(ip) {
			return true
		}
		if _, subnet, err := net.ParseCIDR(p); err == nil && subnet.Contains(ip) {
			return true
		}
	}
	return false
}
