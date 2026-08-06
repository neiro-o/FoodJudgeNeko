package handlers

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"mtv2/backend/database"
	"mtv2/backend/utils"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

const (
	accountAvatarCacheTTL = 7 * 24 * time.Hour
	maxAccountAvatarBytes = 5 << 20
)

var (
	qqAccountEmailPattern   = regexp.MustCompile(`^([0-9]+)@qq\.com$`)
	accountAvatarHTTPClient = &http.Client{Timeout: 10 * time.Second}
)

func accountAvatarCachePath(accountID string) (string, error) {
	cacheDir := filepath.Join("cache", "img")
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return "", fmt.Errorf("create avatar cache directory: %w", err)
	}

	return filepath.Join(cacheDir, "account_avatar_"+accountID+".img"), nil
}

func isFreshAccountAvatarCache(cachePath string, now time.Time) bool {
	info, err := os.Stat(cachePath)
	if err != nil || !info.Mode().IsRegular() || info.Size() == 0 {
		return false
	}

	age := now.Sub(info.ModTime())
	return age >= 0 && age < accountAvatarCacheTTL
}

func accountAvatarSourceURLs(email string) []string {
	normalizedEmail := strings.ToLower(strings.TrimSpace(email))
	sources := make([]string, 0, 2)

	if matches := qqAccountEmailPattern.FindStringSubmatch(normalizedEmail); len(matches) == 2 {
		query := url.Values{
			"b":  {"qq"},
			"nk": {matches[1]},
			"s":  {"640"},
		}
		sources = append(sources, "https://q1.qlogo.cn/g?"+query.Encode())
	}

	emailHash := sha256.Sum256([]byte(normalizedEmail))
	gravatarQuery := url.Values{
		"d": {"404"},
		"s": {"256"},
	}
	sources = append(sources, "https://gravatar.com/avatar/"+hex.EncodeToString(emailHash[:])+"?"+gravatarQuery.Encode())

	return sources
}

func fetchAccountAvatar(client *http.Client, sourceURL string) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, sourceURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Accept", "image/*")
	req.Header.Set("User-Agent", "mtv2-avatar-cache/1.0")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request avatar: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d", resp.StatusCode)
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, maxAccountAvatarBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read avatar: %w", err)
	}
	if len(data) == 0 {
		return nil, fmt.Errorf("empty avatar response")
	}
	if len(data) > maxAccountAvatarBytes {
		return nil, fmt.Errorf("avatar exceeds %d bytes", maxAccountAvatarBytes)
	}

	contentType := http.DetectContentType(data)
	if !strings.HasPrefix(contentType, "image/") {
		return nil, fmt.Errorf("response is not an image (%s)", contentType)
	}

	return data, nil
}

func fetchFirstAccountAvatar(client *http.Client, sources []string) ([]byte, error) {
	var errors []string
	for _, sourceURL := range sources {
		data, err := fetchAccountAvatar(client, sourceURL)
		if err == nil {
			return data, nil
		}
		errors = append(errors, err.Error())
	}

	return nil, fmt.Errorf("no avatar source succeeded: %s", strings.Join(errors, "; "))
}

func writeAccountAvatarCache(cachePath string, data []byte) error {
	tempFile, err := os.CreateTemp(filepath.Dir(cachePath), ".account-avatar-*")
	if err != nil {
		return fmt.Errorf("create temporary cache file: %w", err)
	}
	tempPath := tempFile.Name()
	defer os.Remove(tempPath)

	if _, err := tempFile.Write(data); err != nil {
		tempFile.Close()
		return fmt.Errorf("write temporary cache file: %w", err)
	}
	if err := tempFile.Chmod(0644); err != nil {
		tempFile.Close()
		return fmt.Errorf("set cache file permissions: %w", err)
	}
	if err := tempFile.Close(); err != nil {
		return fmt.Errorf("close temporary cache file: %w", err)
	}
	if err := os.Rename(tempPath, cachePath); err != nil {
		return fmt.Errorf("replace avatar cache file: %w", err)
	}

	return nil
}

func serveAccountAvatar(c *gin.Context, cachePath string) error {
	file, err := os.Open(cachePath)
	if err != nil {
		return fmt.Errorf("open avatar cache: %w", err)
	}

	header := make([]byte, 512)
	n, readErr := file.Read(header)
	if closeErr := file.Close(); readErr != nil && readErr != io.EOF {
		return fmt.Errorf("read avatar cache: %w", readErr)
	} else if closeErr != nil {
		return fmt.Errorf("close avatar cache: %w", closeErr)
	}

	contentType := http.DetectContentType(header[:n])
	if !strings.HasPrefix(contentType, "image/") {
		return fmt.Errorf("cached avatar is not an image (%s)", contentType)
	}

	c.Header("Content-Type", contentType)
	c.File(cachePath)
	return nil
}

// GetAccountAvatar returns the website account avatar derived from accounts.email.
// GET /api/account/avatar?id=<accounts._id>
func GetAccountAvatar(c *gin.Context) {
	accountID := c.Query("id")
	if accountID == "" {
		utils.BadRequestResponse(c, "Missing id parameter")
		return
	}

	objectID, err := primitive.ObjectIDFromHex(accountID)
	if err != nil {
		utils.BadRequestResponse(c, "Invalid id: must be a valid 24-character hex ObjectID")
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	var account struct {
		Email string `bson:"email"`
	}
	if err := database.Accounts.FindOne(ctx, bson.M{"_id": objectID}).Decode(&account); err != nil {
		if err == mongo.ErrNoDocuments {
			utils.NotFoundResponse(c, "Account not found")
			return
		}
		utils.InternalServerErrorResponse(c, "Database error")
		return
	}

	cachePath, err := accountAvatarCachePath(accountID)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to get avatar cache path")
		return
	}
	if isFreshAccountAvatarCache(cachePath, time.Now()) {
		if err := serveAccountAvatar(c, cachePath); err == nil {
			return
		}
	}

	avatar, err := fetchFirstAccountAvatar(accountAvatarHTTPClient, accountAvatarSourceURLs(account.Email))
	if err != nil {
		log.Printf("No account avatar available for %s: %v", accountID, err)
		c.Status(http.StatusNoContent)
		return
	}
	if err := writeAccountAvatarCache(cachePath, avatar); err != nil {
		utils.InternalServerErrorResponse(c, "Failed to cache avatar")
		return
	}

	if err := serveAccountAvatar(c, cachePath); err != nil {
		utils.InternalServerErrorResponse(c, "Failed to serve avatar")
	}
}
