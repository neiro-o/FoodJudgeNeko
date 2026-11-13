package handlers

import (
	"errors"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"

	"mtv2/backend/database"
	"mtv2/backend/stats"
	"mtv2/backend/utils"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

var cloudService struct {
	sync.Once
	service *stats.Service
}

func wordCloudService() *stats.Service {
	cloudService.Do(func() {
		cloudService.service = &stats.Service{Redis: database.RedisClient, Source: stats.MongoSource{Comments: database.Comments, Malicious: database.Malicious}}
	})
	return cloudService.service
}

func GetWordCloud(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	r := c.DefaultQuery("range", "24h")
	if !stats.ValidRange(r) {
		utils.BadRequestResponse(c, "range must be 24h or 7d")
		return
	}
	start := time.Now()
	snapshot, err := wordCloudService().Latest(c.Request.Context())
	if err != nil {
		log.Printf("word cloud: %v", err)
		utils.InternalServerErrorResponse(c, "Failed to load word cloud; ensure the comments createTime index has been initialized")
		return
	}
	log.Printf("word cloud request: range=%s elapsed=%s", r, time.Since(start))
	utils.SuccessResponse(c, snapshot.Cloud(r))
}
func GetWordCloudComments(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	r, id, word := c.Query("range"), c.Query("snapshot"), c.Query("word")
	page, err := strconv.Atoi(c.DefaultQuery("page", "1"))
	if !stats.ValidRange(r) || !primitive.IsValidObjectID(id) || strings.TrimSpace(word) == "" || len(word) > 512 || err != nil || page < 1 {
		utils.BadRequestResponse(c, "Invalid range, snapshot, word or page")
		return
	}
	snapshot, err := wordCloudService().Snapshot(c.Request.Context(), id)
	if errors.Is(err, stats.ErrExpired) {
		utils.ErrorResponse(c, 410, "Word cloud snapshot expired")
		return
	}
	if err != nil {
		log.Printf("word cloud comments: %v", err)
		utils.InternalServerErrorResponse(c, "Failed to load word cloud comments")
		return
	}
	result, err := snapshot.Page(r, word, page)
	if errors.Is(err, stats.ErrWord) {
		utils.NotFoundResponse(c, "Word not in snapshot")
		return
	}
	if err != nil {
		utils.BadRequestResponse(c, err.Error())
		return
	}
	utils.SuccessResponse(c, result)
}
