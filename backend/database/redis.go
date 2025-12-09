package database

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"mtv2/backend/config"
)

var (
	RedisClient *redis.Client
)

func ConnectRedis() error {
	RedisClient = redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%d", config.AppConfig.Redis.Host, config.AppConfig.Redis.Port),
		Password: config.AppConfig.Redis.Password,
		DB:       config.AppConfig.Redis.DB,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := RedisClient.Ping(ctx).Result()
	if err != nil {
		return fmt.Errorf("failed to connect to Redis: %w", err)
	}

	fmt.Println("Connected to Redis successfully")
	return nil
}

func DisconnectRedis() error {
	if RedisClient != nil {
		return RedisClient.Close()
	}
	return nil
}

