package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEnvironmentOverridesAndLegacyDefaults(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yml")
	if err := os.WriteFile(path, []byte("mongodb:\n  connection_string: mongodb://legacy:27017\nredis:\n  host: legacy\nelasticsearch:\n  hosts: [http://legacy:9200]\nserver:\n  jwt_secret: 0123456789012345678901234567890123456789\n"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("MTV2_MONGODB_URI", "mongodb://new:27017")
	t.Setenv("MTV2_REDIS_PASSWORD", "")
	t.Setenv("MTV2_ES_HOSTS", "http://one:9200, http://two:9200")
	t.Setenv("MTV2_SERVER_PORT", "18080")
	if err := LoadConfig(path); err != nil {
		t.Fatal(err)
	}
	if err := AppConfig.Validate(); err != nil {
		t.Fatal(err)
	}
	if AppConfig.MongoDB.ConnectionString != "mongodb://new:27017" || AppConfig.Server.Port != 18080 || len(AppConfig.Elasticsearch.Hosts) != 2 || AppConfig.MongoDB.Collections.ProcessedList != "processed_list" {
		t.Fatalf("environment or defaults not applied")
	}
	t.Setenv("MTV2_SERVER_PORT", "wrong")
	if err := LoadConfig(path); err == nil {
		t.Fatal("invalid port must fail")
	}
}
