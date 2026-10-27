package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	SharedState struct {
		URI      string `yaml:"mongodb_uri"`
		Database string `yaml:"mongodb_database"`
	} `yaml:"shared_state"`
	PrimaryAPIURL       string   `yaml:"primary_api_url"`
	PrimaryReadPrefixes []string `yaml:"primary_read_prefixes"`
	PrimaryReadPaths    []string `yaml:"primary_read_paths"`
	MongoDB             struct {
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
		CAFile             string   `yaml:"ca_file"`
		InsecureSkipVerify bool     `yaml:"insecure_skip_verify"`
		Username           string   `yaml:"username"`
		Password           string   `yaml:"password"`
	} `yaml:"elasticsearch"`
	Server struct {
		Host           string   `yaml:"host"`
		TrustedProxies []string `yaml:"trusted_proxies"`
		Port           int      `yaml:"port"`
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

	set := func(k string, p *string) {
		if v, ok := os.LookupEnv(k); ok {
			*p = v
		}
	}
	integer := func(k string, p *int) error {
		if v, ok := os.LookupEnv(k); ok {
			n, e := strconv.Atoi(v)
			if e != nil {
				return fmt.Errorf("invalid %s", k)
			}
			*p = n
		}
		return nil
	}
	for _, x := range []struct {
		k string
		p *string
	}{
		{"MTV2_STATE_MONGODB_URI", &config.SharedState.URI}, {"MTV2_STATE_MONGODB_DATABASE", &config.SharedState.Database}, {"PRIMARY_API_URL", &config.PrimaryAPIURL},
		{"MTV2_MONGODB_URI", &config.MongoDB.ConnectionString}, {"MTV2_MONGODB_DATABASE", &config.MongoDB.DatabaseName},
		{"MTV2_REDIS_HOST", &config.Redis.Host}, {"MTV2_REDIS_PASSWORD", &config.Redis.Password},
		{"MTV2_ES_USERNAME", &config.Elasticsearch.Username}, {"MTV2_ES_PASSWORD", &config.Elasticsearch.Password}, {"MTV2_ES_INDEX", &config.Elasticsearch.IndexName}, {"MTV2_ES_CA_FILE", &config.Elasticsearch.CAFile},
		{"MTV2_SERVER_HOST", &config.Server.Host}, {"MTV2_JWT_SECRET", &config.Server.JWTSecret},
		{"MTV2_DEEPSEEK_KEY", &config.AI.DeepSeek.Key}, {"MTV2_DEEPSEEK_BASE_URL", &config.AI.DeepSeek.BaseURL}, {"MTV2_DEEPSEEK_MODEL", &config.AI.DeepSeek.Model},
		{"MTV2_OPENROUTER_KEY", &config.AI.OpenRouter.Key}, {"MTV2_OPENROUTER_BASE_URL", &config.AI.OpenRouter.BaseURL}, {"MTV2_OPENROUTER_MODEL", &config.AI.OpenRouter.Model},
		{"MTV2_PROBLEMS_QUEUE", &config.Redis.Fields.ProblemsQueue}, {"MTV2_DAILY_QUEUE", &config.Redis.Fields.DailyQueue},
	} {
		set(x.k, x.p)
	}
	for _, x := range []struct {
		k string
		p *int
	}{{"MTV2_SERVER_PORT", &config.Server.Port}, {"MTV2_REDIS_PORT", &config.Redis.Port}, {"MTV2_REDIS_DB", &config.Redis.DB}} {
		if err := integer(x.k, x.p); err != nil {
			return err
		}
	}
	if v, ok := os.LookupEnv("MTV2_ES_HOSTS"); ok {
		config.Elasticsearch.Hosts = strings.Split(v, ",")
	}
	if v, ok := os.LookupEnv("MTV2_ALLOWED_ORIGINS"); ok {
		config.Server.AllowedOrigins = strings.Split(v, ",")
	}
	if v, ok := os.LookupEnv("MTV2_ES_INSECURE_SKIP_VERIFY"); ok {
		b, e := strconv.ParseBool(v)
		if e != nil {
			return fmt.Errorf("invalid MTV2_ES_INSECURE_SKIP_VERIFY")
		}
		config.Elasticsearch.InsecureSkipVerify = b
	}
	if v, ok := os.LookupEnv("MTV2_TRUSTED_PROXIES"); ok {
		config.Server.TrustedProxies = []string{}
		for _, item := range strings.Split(v, ",") {
			if v := strings.TrimSpace(item); v != "" {
				config.Server.TrustedProxies = append(config.Server.TrustedProxies, v)
			}
		}
	}
	if config.Server.TrustedProxies == nil {
		config.Server.TrustedProxies = []string{"127.0.0.1", "::1"}
	}
	if config.Server.Host == "" {
		config.Server.Host = "127.0.0.1"
	}
	AppConfig = config
	return nil
}
