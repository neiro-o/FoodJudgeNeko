package config

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"time"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/go-redis/redis/v8"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"gopkg.in/yaml.v2"
)

type Config struct {
	Server struct {
		Port string
	}
	MongoDB struct {
		URI         string
		Database    string
		Collections struct {
			Users      string
			InviteCodes string
		}
	}
	Redis struct {
		Host     string
		Port     string
		Password string
		DB       int
	}
	Elasticsearch struct {
		Addresses []string
		Index     string
	}
	Session struct {
		Secret        string
		MaxAge        int
		SecureCookie  bool
		HttpOnly      bool
		SameSite      string
	}
}

var (
	MongoDB *mongo.Database
	ES      *elasticsearch.Client
	RD      *redis.Client
	// Static config instance
	staticConfig *Config
)

func LoadConfig() *Config {
	// Return static config if already loaded
	if staticConfig != nil {
		return staticConfig
	}
	
	cfg := &Config{}
	
	// Read YAML file
	yamlFile, err := ioutil.ReadFile("config.yml")
	if err != nil {
		log.Fatalf("Failed to read config.yml: %v", err)
	}
	
	// Parse YAML
	err = yaml.Unmarshal(yamlFile, cfg)
	if err != nil {
		log.Fatalf("Failed to parse config.yml: %v", err)
	}
	
	// Store static config
	staticConfig = cfg
	return cfg
}

// GetConfig returns the static config instance without reloading
func GetConfig() *Config {
	if staticConfig == nil {
		return LoadConfig()
	}
	return staticConfig
}

// ResetConfig resets the static config (useful for testing)
func ResetConfig() {
	staticConfig = nil
}

func InitMongoDB(cfg *Config) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	clientOptions := options.Client().ApplyURI(cfg.MongoDB.URI)
	client, err := mongo.Connect(ctx, clientOptions)
	if err != nil {
		log.Fatalf("Failed to connect to MongoDB: %v", err)
	}

	err = client.Ping(ctx, nil)
	if err != nil {
		log.Fatalf("Failed to ping MongoDB: %v", err)
	}

	MongoDB = client.Database(cfg.MongoDB.Database)
	log.Println("MongoDB connected successfully")
}

func InitES(cfg *Config) {
	var err error
	ES, err = elasticsearch.NewClient(elasticsearch.Config{
		Addresses: cfg.Elasticsearch.Addresses,
	})
	if err != nil {
		log.Fatalf("Failed to connect to Elasticsearch: %v", err)
	}
	log.Println("Elasticsearch connected successfully")
}

func InitRedis(cfg *Config) {
	RD = redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%s", cfg.Redis.Host, cfg.Redis.Port),
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})

	ctx := context.Background()
	_, err := RD.Ping(ctx).Result()
	if err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}
	log.Println("Redis connected successfully")
}