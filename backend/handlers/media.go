package handlers

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"mtv2/backend/config"
	"mtv2/backend/utils"

	"github.com/gin-gonic/gin"
)

// Allowed domains for media loading
var allowedDomains = []string{
	"meituan.com",
	"meituan.net",
	"sankuai.com",
}

type LoadMediaRequest struct {
	URL  string `json:"url" binding:"required"`
	Hash string `json:"hash" binding:"required"`
}

// validateHash validates the URL hash using HMAC-SHA256
func validateHash(urlStr, providedHash string) bool {
	secret := config.AppConfig.Server.JWTSecret
	if secret == "" {
		return false
	}

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(urlStr))
	expectedHash := hex.EncodeToString(mac.Sum(nil))

	return hmac.Equal([]byte(providedHash), []byte(expectedHash))
}

// isAllowedDomain checks if the URL is from an allowed domain
func isAllowedDomain(urlStr string) bool {
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return false
	}

	host := strings.ToLower(parsedURL.Host)
	// Remove port if present
	if idx := strings.Index(host, ":"); idx != -1 {
		host = host[:idx]
	}

	// Check exact match or subdomain match
	for _, allowedDomain := range allowedDomains {
		if host == allowedDomain || strings.HasSuffix(host, "."+allowedDomain) {
			return true
		}
	}

	return false
}

// getFileExtension extracts file extension from URL
func getFileExtension(urlStr string) string {
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return ""
	}

	path := parsedURL.Path
	ext := filepath.Ext(path)
	if ext == "" {
		// Try to determine from query parameters or default to jpg for images
		return ".jpg"
	}
	return ext
}

// getCachePath returns the cache file path for an image URL
func getCachePath(imageURL string) (string, error) {
	// Create hash of URL for filename
	hash := sha256.Sum256([]byte(imageURL))
	filename := hex.EncodeToString(hash[:]) + getFileExtension(imageURL)

	// Cache directory relative to backend directory
	cacheDir := filepath.Join("cache", "img")
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create cache directory: %w", err)
	}

	return filepath.Join(cacheDir, filename), nil
}

// downloadAndCacheImage downloads an image and saves it to cache
func downloadAndCacheImage(imageURL, cachePath string) error {
	req, err := http.NewRequest("GET", imageURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Set Referer header
	req.Header.Set("Referer", "https://zqt.meituan.com")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to download image: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to download image: status code %d", resp.StatusCode)
	}

	// Create cache file
	file, err := os.Create(cachePath)
	if err != nil {
		return fmt.Errorf("failed to create cache file: %w", err)
	}
	defer file.Close()

	// Copy response body to file
	_, err = io.Copy(file, resp.Body)
	if err != nil {
		return fmt.Errorf("failed to write cache file: %w", err)
	}

	return nil
}

// LoadImage handles image loading with caching
func LoadImage(c *gin.Context) {
	imageURL := c.Query("url")
	hash := c.Query("hash")

	if imageURL == "" || hash == "" {
		utils.BadRequestResponse(c, "Missing url or hash parameter")
		return
	}

	// Validate hash
	if !validateHash(imageURL, hash) {
		utils.BadRequestResponse(c, "Invalid hash validation")
		return
	}

	// Check if URL is from allowed domain
	if !isAllowedDomain(imageURL) {
		utils.BadRequestResponse(c, "URL is not from an allowed domain")
		return
	}

	// Get cache path
	cachePath, err := getCachePath(imageURL)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to get cache path: "+err.Error())
		return
	}

	// Check if cached file exists
	if _, err := os.Stat(cachePath); os.IsNotExist(err) {
		// Download and cache the image
		if err := downloadAndCacheImage(imageURL, cachePath); err != nil {
			utils.InternalServerErrorResponse(c, "Failed to download image: "+err.Error())
			return
		}
	}

	// Serve the cached file
	c.File(cachePath)
}

// GenerateMediaHash generates a hash for a given URL (protected endpoint)
func GenerateMediaHash(c *gin.Context) {
	var req struct {
		URL string `json:"url" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(c, "Invalid request: "+err.Error())
		return
	}

	// Generate hash
	secret := config.AppConfig.Server.JWTSecret
	if secret == "" {
		utils.InternalServerErrorResponse(c, "Server configuration error")
		return
	}

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(req.URL))
	hash := hex.EncodeToString(mac.Sum(nil))

	utils.SuccessResponse(c, gin.H{
		"url":  req.URL,
		"hash": hash,
	})
}

// LoadVideo handles video loading without caching
func LoadVideo(c *gin.Context) {
	videoURL := c.Query("url")
	hash := c.Query("hash")

	if videoURL == "" || hash == "" {
		utils.BadRequestResponse(c, "Missing url or hash parameter")
		return
	}

	// Validate hash
	if !validateHash(videoURL, hash) {
		utils.BadRequestResponse(c, "Invalid hash validation")
		return
	}

	// Check if URL is from allowed domain
	if !isAllowedDomain(videoURL) {
		utils.BadRequestResponse(c, "URL is not from an allowed domain")
		return
	}

	// Fetch video from URL and stream it
	req, err := http.NewRequest("GET", videoURL, nil)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to create request: "+err.Error())
		return
	}

	// Set Referer header
	req.Header.Set("Referer", "https://zqt.meituan.com")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to fetch video: "+err.Error())
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		utils.InternalServerErrorResponse(c, fmt.Sprintf("Failed to fetch video: status code %d", resp.StatusCode))
		return
	}

	// Set appropriate headers
	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "video/mp4" // Default content type
	}
	c.Header("Content-Type", contentType)
	if resp.ContentLength > 0 {
		c.Header("Content-Length", fmt.Sprintf("%d", resp.ContentLength))
	}
	c.Header("Accept-Ranges", "bytes")

	// Stream the video
	contentLength := resp.ContentLength
	if contentLength < 0 {
		contentLength = 0 // Unknown length
	}
	c.DataFromReader(http.StatusOK, contentLength, contentType, resp.Body, nil)
}
