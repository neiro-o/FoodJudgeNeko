package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	MongoDB struct {
		ConnectionString string `yaml:"connection_string"`
		DatabaseName     string `yaml:"database_name"`
		Collections      struct {
			Accounts      string `yaml:"accounts"`
			Invitations   string `yaml:"invitations"`
			ProcessedList string `yaml:"processed_list"`
			Problems      string `yaml:"problems"`
			Notes         string `yaml:"notes"`
			Comments      string `yaml:"comments"`
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
		Hosts     []string `yaml:"hosts"`
		IndexName string   `yaml:"index_name"`
		Username  string   `yaml:"username"`
		Password  string   `yaml:"password"`
	} `yaml:"elasticsearch"`
	Server struct {
		Port           int      `yaml:"port"`
		JWTSecret      string   `yaml:"jwt_secret"`
		AllowedOrigins []string `yaml:"allowed_origins"`
	} `yaml:"server"`
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

	AppConfig = config
	return nil
}
