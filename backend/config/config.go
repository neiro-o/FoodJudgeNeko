package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	MongoDB struct {
		ConnectionString string `yaml:"connection_string"`
		DatabaseName     string `yaml:"database_name"`
		Collections      struct {
			Accounts        string `yaml:"accounts"`
			Invitations     string `yaml:"invitations"`
			ProcessedList   string `yaml:"processed_list"`
			Problems        string `yaml:"problems"`
			Notes           string `yaml:"notes"`
			Comments        string `yaml:"comments"`
			Malicious       string `yaml:"malicious"`
			UserRankings    string `yaml:"user_rankings"`
			AIUserSummaries string `yaml:"ai_user_summaries"`
			WeeklyScores    string `yaml:"weekly_scores"`
			SearchQuotas    string `yaml:"search_quotas"`
		} `yaml:"collections"`
	} `yaml:"mongodb"`
	Redis struct {
		Host     string `yaml:"host"`
		Port     int    `yaml:"port"`
		Password string `yaml:"password"`
		DB       int    `yaml:"db"`
		Fields   struct {
			ProblemsQueue string `yaml:"problems_queue"`
			DailyQueue    string `yaml:"daily_queue"`
		} `yaml:"fields"`
	} `yaml:"redis"`
	Elasticsearch struct {
		Hosts              []string `yaml:"hosts"`
		IndexName          string   `yaml:"index_name"`
		Username           string   `yaml:"username"`
		Password           string   `yaml:"password"`
		CAFile             string   `yaml:"ca_file"`
		InsecureSkipVerify bool     `yaml:"insecure_skip_verify"`
	} `yaml:"elasticsearch"`
	Server struct {
		Port           int      `yaml:"port"`
		Host           string   `yaml:"host"`
		JWTSecret      string   `yaml:"jwt_secret"`
		AllowedOrigins []string `yaml:"allowed_origins"`
	} `yaml:"server"`
	AI struct {
		DeepSeek   AIProviderConfig `yaml:"deepseek"`
		OpenRouter AIProviderConfig `yaml:"openrouter"`
	} `yaml:"ai"`
}

// AIProviderConfig holds the connection settings for a single OpenAI-compatible
// chat completion provider (e.g. DeepSeek, OpenRouter).
type AIProviderConfig struct {
	BaseURL string `yaml:"base_url"`
	Key     string `yaml:"key"`
	Model   string `yaml:"model"`
}

// Configured reports whether this provider has enough settings to be called.
func (p AIProviderConfig) Configured() bool {
	return p.BaseURL != "" && p.Key != "" && p.Model != ""
}

var AppConfig *Config

// Path preserves the local development layout while allowing an explicit mount.
func Path() string {
	if path := os.Getenv("MTV2_CONFIG"); path != "" {
		return path
	}
	if _, err := os.Stat("../config.yml"); err == nil {
		return "../config.yml"
	}
	return "config.yml"
}

func LoadConfig(configPath string) error {
	config := &Config{}

	file, err := os.Open(configPath)
	if err != nil {
		return fmt.Errorf("failed to open config file: %w", err)
	}
	defer file.Close()

	decoder := yaml.NewDecoder(file)
	if err := decoder.Decode(config); err != nil {
		return fmt.Errorf("failed to decode config file: %w", err)
	}
	if err := config.applyEnvironment(); err != nil {
		return err
	}
	config.defaults()

	AppConfig = config
	return nil
}

func (c *Config) defaults() {
	if c.Server.Port == 0 {
		c.Server.Port = 8080
	}
	if c.Redis.Port == 0 {
		c.Redis.Port = 6379
	}
	if c.MongoDB.DatabaseName == "" {
		c.MongoDB.DatabaseName = "mtv2"
	}
	if c.Elasticsearch.IndexName == "" {
		c.Elasticsearch.IndexName = "problems"
	}
	defaults := map[*string]string{
		&c.MongoDB.Collections.Accounts: "accounts", &c.MongoDB.Collections.Invitations: "invitations",
		&c.MongoDB.Collections.ProcessedList: "processed_list", &c.MongoDB.Collections.Problems: "problems",
		&c.MongoDB.Collections.Notes: "notes", &c.MongoDB.Collections.Comments: "comments",
		&c.MongoDB.Collections.Malicious: "malicious", &c.MongoDB.Collections.UserRankings: "user_rankings",
		&c.MongoDB.Collections.AIUserSummaries: "ai_user_summaries", &c.MongoDB.Collections.WeeklyScores: "weekly_scores",
		&c.MongoDB.Collections.SearchQuotas: "search_quotas",
		&c.Redis.Fields.ProblemsQueue:       "problems_queue", &c.Redis.Fields.DailyQueue: "daily_queue",
	}
	for field, value := range defaults {
		if *field == "" {
			*field = value
		}
	}
}

func (c *Config) applyEnvironment() error {
	fields := map[string]*string{
		"MTV2_MONGODB_URI": &c.MongoDB.ConnectionString, "MTV2_MONGODB_DATABASE": &c.MongoDB.DatabaseName,
		"MTV2_REDIS_HOST": &c.Redis.Host, "MTV2_REDIS_PASSWORD": &c.Redis.Password,
		"MTV2_PROBLEMS_QUEUE": &c.Redis.Fields.ProblemsQueue, "MTV2_DAILY_QUEUE": &c.Redis.Fields.DailyQueue,
		"MTV2_ES_INDEX": &c.Elasticsearch.IndexName, "MTV2_ES_USERNAME": &c.Elasticsearch.Username,
		"MTV2_ES_PASSWORD": &c.Elasticsearch.Password, "MTV2_ES_CA_FILE": &c.Elasticsearch.CAFile,
		"MTV2_SERVER_HOST": &c.Server.Host, "MTV2_JWT_SECRET": &c.Server.JWTSecret,
		"MTV2_DEEPSEEK_BASE_URL": &c.AI.DeepSeek.BaseURL, "MTV2_DEEPSEEK_KEY": &c.AI.DeepSeek.Key,
		"MTV2_DEEPSEEK_MODEL": &c.AI.DeepSeek.Model, "MTV2_OPENROUTER_BASE_URL": &c.AI.OpenRouter.BaseURL,
		"MTV2_OPENROUTER_KEY": &c.AI.OpenRouter.Key, "MTV2_OPENROUTER_MODEL": &c.AI.OpenRouter.Model,
	}
	for name, field := range fields {
		if value, ok := os.LookupEnv(name); ok {
			*field = value
		}
	}
	for name, field := range map[string]*int{"MTV2_SERVER_PORT": &c.Server.Port, "MTV2_REDIS_PORT": &c.Redis.Port, "MTV2_REDIS_DB": &c.Redis.DB} {
		if value, ok := os.LookupEnv(name); ok {
			n, err := strconv.Atoi(value)
			if err != nil || n < 0 || (name != "MTV2_REDIS_DB" && (n == 0 || n > 65535)) {
				return fmt.Errorf("invalid %s", name)
			}
			*field = n
		}
	}
	for name, field := range map[string]*[]string{"MTV2_ES_HOSTS": &c.Elasticsearch.Hosts, "MTV2_ALLOWED_ORIGINS": &c.Server.AllowedOrigins} {
		if value, ok := os.LookupEnv(name); ok {
			*field = nil
			for _, item := range strings.Split(value, ",") {
				if item = strings.TrimSpace(item); item != "" {
					*field = append(*field, item)
				}
			}
		}
	}
	if value, ok := os.LookupEnv("MTV2_ES_INSECURE_SKIP_VERIFY"); ok {
		b, err := strconv.ParseBool(value)
		if err != nil {
			return fmt.Errorf("invalid MTV2_ES_INSECURE_SKIP_VERIFY")
		}
		c.Elasticsearch.InsecureSkipVerify = b
	}
	return nil
}

func (c *Config) Validate() error {
	if c.MongoDB.ConnectionString == "" || c.Redis.Host == "" || len(c.Elasticsearch.Hosts) == 0 {
		return fmt.Errorf("MongoDB, Redis and Elasticsearch connection settings are required")
	}
	if len(c.Server.JWTSecret) < 32 {
		return fmt.Errorf("JWT secret must contain at least 32 characters")
	}
	return nil
}
