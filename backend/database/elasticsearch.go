package database

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"mtv2/backend/config"

	"github.com/elastic/go-elasticsearch/v8"
)

var (
	ESClient *elasticsearch.Client
)

func ConnectElasticsearch() error {
	cfg := elasticsearch.Config{
		Addresses: config.AppConfig.Elasticsearch.Hosts,
	}

	// Add authentication if provided
	if config.AppConfig.Elasticsearch.Username != "" && config.AppConfig.Elasticsearch.Password != "" {
		cfg.Username = config.AppConfig.Elasticsearch.Username
		cfg.Password = config.AppConfig.Elasticsearch.Password
	}

	// Only configure TLS transport for HTTPS connections
	// Check if any host uses HTTPS
	useHTTPS := false
	for _, host := range config.AppConfig.Elasticsearch.Hosts {
		if strings.HasPrefix(strings.ToLower(host), "https://") {
			useHTTPS = true
			break
		}
	}

	if useHTTPS {
		// Configure for development (skip TLS verification for HTTPS connections)
		transport := &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		}
		cfg.Transport = transport
	}
	// For HTTP connections, use default transport (no TLS configuration needed)

	client, err := elasticsearch.NewClient(cfg)
	if err != nil {
		return fmt.Errorf("failed to create Elasticsearch client: %w", err)
	}

	// Test connection with longer timeout for HTTPS
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	res, err := client.Info(client.Info.WithContext(ctx))
	if err != nil {
		return fmt.Errorf("failed to ping Elasticsearch: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		var e map[string]interface{}
		if err := json.NewDecoder(res.Body).Decode(&e); err == nil {
			return fmt.Errorf("Elasticsearch error: %v", e)
		}
		return fmt.Errorf("Elasticsearch error: %s", res.Status())
	}

	ESClient = client
	fmt.Println("Connected to Elasticsearch successfully")
	return nil
}

func DisconnectElasticsearch() error {
	// Elasticsearch client doesn't need explicit disconnection
	return nil
}
