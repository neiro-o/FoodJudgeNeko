package bootstrap

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"time"

	"mtv2/backend/config"
	"mtv2/backend/stats"

	"github.com/redis/go-redis/v9"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

// InitStats ensures only the additive time index and warms the same shared
// snapshot used by the API. It neither creates accounts nor changes comments.
func InitStats(ctx context.Context) error {
	client, err := mongoClient(ctx)
	if err != nil {
		return err
	}
	defer client.Disconnect(context.Background())
	cfg := config.AppConfig
	db := client.Database(cfg.MongoDB.DatabaseName)
	comments := db.Collection(cfg.MongoDB.Collections.Comments)
	if _, err = comments.Indexes().CreateOne(ctx, mongo.IndexModel{Keys: bson.D{{Key: "createTime", Value: 1}}}); err != nil {
		return fmt.Errorf("ensure comments time index: %w", err)
	}
	cache := redis.NewClient(&redis.Options{Addr: net.JoinHostPort(cfg.Redis.Host, strconv.Itoa(cfg.Redis.Port)), Password: cfg.Redis.Password, DB: cfg.Redis.DB})
	defer cache.Close()
	service := &stats.Service{Redis: cache, Source: stats.MongoSource{Comments: comments, Malicious: db.Collection(cfg.MongoDB.Collections.Malicious)}}
	start := time.Now()
	snapshot, err := service.Latest(ctx)
	if err != nil {
		return err
	}
	for _, r := range []string{"24h", "7d"} {
		window := snapshot.Cloud(r)
		fmt.Printf("stats %s: comments=%d words=%d generated_at=%s expires_at=%s\n", r, window.CommentCount, len(window.Words), time.Unix(window.GeneratedAt, 0).UTC().Format(time.RFC3339), time.Unix(window.ExpiresAt, 0).UTC().Format(time.RFC3339))
	}
	fmt.Printf("stats initialization elapsed=%s\n", time.Since(start))
	return nil
}
