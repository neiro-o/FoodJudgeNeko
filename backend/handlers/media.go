package handlers

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"mtv2/backend/config"
	"mtv2/backend/database"
	"mtv2/backend/utils"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
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

var hlsURIAttributePattern = regexp.MustCompile(`URI="([^"]+)"`)

func generateMediaHash(urlStr string) string {
	mac := hmac.New(sha256.New, []byte(config.AppConfig.Server.JWTSecret))
	mac.Write([]byte(urlStr))
	return hex.EncodeToString(mac.Sum(nil))
}

// validateHash validates the URL hash using HMAC-SHA256
func validateHash(urlStr, providedHash string) bool {
	secret := config.AppConfig.Server.JWTSecret
	if secret == "" {
		return false
	}

	expectedHash := generateMediaHash(urlStr)

	return hmac.Equal([]byte(providedHash), []byte(expectedHash))
}

func isHLSPlaylistURL(urlStr string) bool {
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return false
	}
	return strings.EqualFold(filepath.Ext(parsedURL.Path), ".m3u8")
}

func hlsProxyURL(rawReference, playlistURL, proxyPath string) (string, error) {
	reference := strings.TrimSpace(rawReference)
	if reference == "" {
		return "", fmt.Errorf("empty HLS URI")
	}
	if strings.HasPrefix(strings.ToLower(reference), "data:") {
		return reference, nil
	}

	base, err := url.Parse(playlistURL)
	if err != nil {
		return "", fmt.Errorf("parse playlist URL: %w", err)
	}
	ref, err := url.Parse(reference)
	if err != nil {
		return "", fmt.Errorf("parse HLS URI %q: %w", reference, err)
	}
	resolved := base.ResolveReference(ref).String()
	if !isAllowedDomain(resolved) {
		return "", fmt.Errorf("HLS URI is not from an allowed domain: %s", resolved)
	}

	query := url.Values{
		"url":  {resolved},
		"hash": {generateMediaHash(resolved)},
	}
	return proxyPath + "?" + query.Encode(), nil
}

// rewriteHLSPlaylist makes every playlist, segment, encryption-key, and init
// file URI go back through this proxy. Returning the upstream manifest as-is
// breaks relative URIs because the browser resolves them against /api/media/.
func rewriteHLSPlaylist(data []byte, playlistURL, proxyPath string) ([]byte, error) {
	input := string(data)
	hadTrailingNewline := strings.HasSuffix(input, "\n")
	lines := strings.Split(strings.TrimSuffix(input, "\n"), "\n")

	for i, line := range lines {
		line = strings.TrimSuffix(line, "\r")
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			lines[i] = line
			continue
		}

		if !strings.HasPrefix(trimmed, "#") {
			proxied, err := hlsProxyURL(trimmed, playlistURL, proxyPath)
			if err != nil {
				return nil, err
			}
			lines[i] = proxied
			continue
		}

		var rewriteErr error
		lines[i] = hlsURIAttributePattern.ReplaceAllStringFunc(line, func(match string) string {
			if rewriteErr != nil {
				return match
			}
			reference := match[len(`URI="`) : len(match)-1]
			proxied, err := hlsProxyURL(reference, playlistURL, proxyPath)
			if err != nil {
				rewriteErr = err
				return match
			}
			return `URI="` + proxied + `"`
		})
		if rewriteErr != nil {
			return nil, rewriteErr
		}
	}

	result := strings.Join(lines, "\n")
	if hadTrailingNewline {
		result += "\n"
	}
	return []byte(result), nil
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

// getFileExtension extracts file extension from URL, falling back to defaultExt if none is present
func getFileExtension(urlStr, defaultExt string) string {
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return defaultExt
	}

	path := parsedURL.Path
	ext := filepath.Ext(path)
	if ext == "" {
		return defaultExt
	}
	return ext
}

// getCachePath returns the cache file path for a media URL, namespaced by subdir (e.g. "img", "video", "audio")
func getCachePath(mediaURL, subdir, defaultExt string) (string, error) {
	// Create hash of URL for filename
	hash := sha256.Sum256([]byte(mediaURL))
	filename := hex.EncodeToString(hash[:]) + getFileExtension(mediaURL, defaultExt)

	// Cache directory relative to backend directory
	cacheDir := filepath.Join("cache", subdir)
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create cache directory: %w", err)
	}

	return filepath.Join(cacheDir, filename), nil
}

// downloadAndCacheMedia downloads a media file and saves it to cache
func downloadAndCacheMedia(mediaURL, cachePath string) error {
	req, err := http.NewRequest("GET", mediaURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Set Referer header
	req.Header.Set("Referer", "https://zqt.meituan.com")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to download media: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to download media: status code %d", resp.StatusCode)
	}

	// Download atomically so parallel HLS requests never observe a partial
	// playlist or segment.
	file, err := os.CreateTemp(filepath.Dir(cachePath), ".media-*")
	if err != nil {
		return fmt.Errorf("failed to create cache file: %w", err)
	}
	tempPath := file.Name()
	defer os.Remove(tempPath)

	// Copy response body to file
	if _, err := io.Copy(file, resp.Body); err != nil {
		file.Close()
		return fmt.Errorf("failed to write cache file: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("failed to close cache file: %w", err)
	}
	if err := os.Rename(tempPath, cachePath); err != nil {
		return fmt.Errorf("failed to replace cache file: %w", err)
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
	cachePath, err := getCachePath(imageURL, "img", ".jpg")
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to get cache path: "+err.Error())
		return
	}

	// Check if cached file exists
	if _, err := os.Stat(cachePath); os.IsNotExist(err) {
		// Download and cache the image
		if err := downloadAndCacheMedia(imageURL, cachePath); err != nil {
			utils.InternalServerErrorResponse(c, "Failed to download image: "+err.Error())
			return
		}
	}

	// Serve the cached file
	c.File(cachePath)
}

// LoadRandomImage picks a random comment from mtv2.comments whose "images"
// field is a non-empty array, takes its first image (images[0]), and
// downloads/serves it using the same cache-then-serve flow as LoadImage.
// GET /api/media/random_image
func LoadRandomImage(c *gin.Context) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pipeline := mongo.Pipeline{
		bson.D{{Key: "$match", Value: bson.M{
			"images": bson.M{"$exists": true, "$type": "array", "$ne": bson.A{}},
		}}},
		bson.D{{Key: "$sample", Value: bson.M{"size": 1}}},
	}

	cursor, err := database.Comments.Aggregate(ctx, pipeline)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to query comments: "+err.Error())
		return
	}
	defer cursor.Close(ctx)

	var docs []bson.M
	if err := cursor.All(ctx, &docs); err != nil {
		utils.InternalServerErrorResponse(c, "Failed to decode comment: "+err.Error())
		return
	}

	if len(docs) == 0 {
		utils.NotFoundResponse(c, "No comment with images found")
		return
	}

	images := toStringSlice(docs[0]["images"])
	if len(images) == 0 || images[0] == "" {
		utils.NotFoundResponse(c, "No comment with images found")
		return
	}

	imageURL := images[0]

	if !isAllowedDomain(imageURL) {
		utils.InternalServerErrorResponse(c, "Image URL is not from an allowed domain")
		return
	}

	cachePath, err := getCachePath(imageURL, "img", ".jpg")
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to get cache path: "+err.Error())
		return
	}

	if _, err := os.Stat(cachePath); os.IsNotExist(err) {
		if err := downloadAndCacheMedia(imageURL, cachePath); err != nil {
			utils.InternalServerErrorResponse(c, "Failed to download image: "+err.Error())
			return
		}
	}

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

	hash := generateMediaHash(req.URL)

	utils.SuccessResponse(c, gin.H{
		"url":  req.URL,
		"hash": hash,
	})
}

// LoadVideo handles video loading with download-then-serve caching
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

	// Get cache path
	cachePath, err := getCachePath(videoURL, "video", ".mp4")
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to get cache path: "+err.Error())
		return
	}

	// Check if cached file exists
	if _, err := os.Stat(cachePath); os.IsNotExist(err) {
		// Download and cache the video first, then serve from disk
		if err := downloadAndCacheMedia(videoURL, cachePath); err != nil {
			utils.InternalServerErrorResponse(c, "Failed to download video: "+err.Error())
			return
		}
	}

	if isHLSPlaylistURL(videoURL) {
		playlist, err := os.ReadFile(cachePath)
		if err != nil {
			utils.InternalServerErrorResponse(c, "Failed to read HLS playlist: "+err.Error())
			return
		}
		rewritten, err := rewriteHLSPlaylist(playlist, videoURL, c.Request.URL.Path)
		if err != nil {
			utils.InternalServerErrorResponse(c, "Failed to rewrite HLS playlist: "+err.Error())
			return
		}

		c.Header("Cache-Control", "private, max-age=300")
		c.Data(http.StatusOK, "application/vnd.apple.mpegurl", rewritten)
		return
	}

	c.Header("Accept-Ranges", "bytes")
	c.File(cachePath)
}

// LoadAudio handles audio loading with download-then-serve caching
func LoadAudio(c *gin.Context) {
	audioURL := c.Query("url")
	hash := c.Query("hash")

	if audioURL == "" || hash == "" {
		utils.BadRequestResponse(c, "Missing url or hash parameter")
		return
	}

	// Validate hash
	if !validateHash(audioURL, hash) {
		utils.BadRequestResponse(c, "Invalid hash validation")
		return
	}

	// Check if URL is from allowed domain
	if !isAllowedDomain(audioURL) {
		utils.BadRequestResponse(c, "URL is not from an allowed domain")
		return
	}

	// Get cache path
	cachePath, err := getCachePath(audioURL, "audio", ".mp3")
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to get cache path: "+err.Error())
		return
	}

	// Check if cached file exists
	if _, err := os.Stat(cachePath); os.IsNotExist(err) {
		// Download and cache the audio first, then serve from disk
		if err := downloadAndCacheMedia(audioURL, cachePath); err != nil {
			utils.InternalServerErrorResponse(c, "Failed to download audio: "+err.Error())
			return
		}
	}

	c.Header("Accept-Ranges", "bytes")
	c.File(cachePath)
}
