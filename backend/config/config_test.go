package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEnvironmentOverridesAndEmptySecret(t *testing.T) {
	p := filepath.Join(t.TempDir(), "config.yml")
	if err := os.WriteFile(p, []byte("mongodb:\n  connection_string: mongodb://old\n  database_name: old\nai:\n  deepseek:\n    key: old-secret\nserver:\n  port: 8080\n"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("MTV2_MONGODB_URI", "mongodb://new")
	t.Setenv("MTV2_STATE_MONGODB_URI", "mongodb://state")
	t.Setenv("MTV2_DEEPSEEK_KEY", "")
	t.Setenv("MTV2_SERVER_PORT", "18080")
	t.Setenv("MTV2_TRUSTED_PROXIES", "172.30.90.22, 127.0.0.1")
	if err := LoadConfig(p); err != nil {
		t.Fatal(err)
	}
	if AppConfig.MongoDB.ConnectionString != "mongodb://new" || AppConfig.SharedState.URI != "mongodb://state" || AppConfig.AI.DeepSeek.Key != "" || AppConfig.Server.Port != 18080 || AppConfig.Server.Host != "127.0.0.1" || len(AppConfig.Server.TrustedProxies) != 2 || AppConfig.Server.TrustedProxies[0] != "172.30.90.22" {
		t.Fatal("environment override failed")
	}
}
func TestInvalidIntegerRejected(t *testing.T) {
	p := filepath.Join(t.TempDir(), "c.yml")
	if err := os.WriteFile(p, []byte("{}"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("MTV2_SERVER_PORT", "invalid")
	if LoadConfig(p) == nil {
		t.Fatal("invalid port accepted")
	}
}
