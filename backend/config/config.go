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
			Accounts    string `yaml:"accounts"`
			Invitations string `yaml:"invitations"`
		} `yaml:"collections"`
	} `yaml:"mongodb"`
	Server struct {
		Port      int    `yaml:"port"`
		JWTSecret string `yaml:"jwt_secret"`
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

