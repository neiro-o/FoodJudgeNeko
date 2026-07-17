package ai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func newTestServer(t *testing.T, statusCode int, content string, checkAuth string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if checkAuth != "" && r.Header.Get("Authorization") != "Bearer "+checkAuth {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.WriteHeader(statusCode)
		if statusCode >= 200 && statusCode < 300 {
			resp := chatCompletionResponse{}
			resp.Choices = []struct {
				Message struct {
					Content string `json:"content"`
				} `json:"message"`
			}{
				{Message: struct {
					Content string `json:"content"`
				}{Content: content}},
			}
			_ = json.NewEncoder(w).Encode(resp)
		}
	}))
}

func TestGenerateWithFallback_FirstProviderSucceeds(t *testing.T) {
	server := newTestServer(t, http.StatusOK, "hello from primary", "primary-key")
	defer server.Close()

	providers := []Provider{
		{Name: "primary", BaseURL: server.URL, APIKey: "primary-key", Model: "m"},
		{Name: "secondary", BaseURL: "http://should-not-be-called.invalid", APIKey: "k", Model: "m"},
	}

	content, used, err := GenerateWithFallback(context.Background(), providers, []Message{{Role: "user", Content: "hi"}}, 5*time.Second, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if content != "hello from primary" {
		t.Errorf("expected content from primary provider, got %q", content)
	}
	if used.Name != "primary" {
		t.Errorf("expected primary provider to be used, got %q", used.Name)
	}
}

func TestGenerateWithFallback_FallsBackOnFailure(t *testing.T) {
	failing := newTestServer(t, http.StatusInternalServerError, "", "")
	defer failing.Close()
	working := newTestServer(t, http.StatusOK, "hello from fallback", "fallback-key")
	defer working.Close()

	providers := []Provider{
		{Name: "deepseek", BaseURL: failing.URL, APIKey: "any", Model: "m"},
		{Name: "openrouter", BaseURL: working.URL, APIKey: "fallback-key", Model: "m"},
	}

	var failedProviders []string
	content, used, err := GenerateWithFallback(context.Background(), providers, []Message{{Role: "user", Content: "hi"}}, 5*time.Second, func(provider string, err error) {
		failedProviders = append(failedProviders, provider)
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if content != "hello from fallback" {
		t.Errorf("expected fallback content, got %q", content)
	}
	if used.Name != "openrouter" {
		t.Errorf("expected openrouter to be used after deepseek failed, got %q", used.Name)
	}
	if len(failedProviders) != 1 || failedProviders[0] != "deepseek" {
		t.Errorf("expected deepseek recorded as a failed attempt, got %v", failedProviders)
	}
}

func TestGenerateWithFallback_AllProvidersFail(t *testing.T) {
	failing1 := newTestServer(t, http.StatusInternalServerError, "", "")
	defer failing1.Close()
	failing2 := newTestServer(t, http.StatusInternalServerError, "", "")
	defer failing2.Close()

	providers := []Provider{
		{Name: "deepseek", BaseURL: failing1.URL, APIKey: "any", Model: "m"},
		{Name: "openrouter", BaseURL: failing2.URL, APIKey: "any", Model: "m"},
	}

	_, _, err := GenerateWithFallback(context.Background(), providers, []Message{{Role: "user", Content: "hi"}}, 5*time.Second, nil)
	if err == nil {
		t.Fatal("expected an error when all providers fail")
	}
}

func TestGenerateWithFallback_SkipsUnconfiguredProviders(t *testing.T) {
	working := newTestServer(t, http.StatusOK, "hello", "k")
	defer working.Close()

	providers := []Provider{
		{Name: "deepseek", BaseURL: "", APIKey: "", Model: ""}, // not configured
		{Name: "openrouter", BaseURL: working.URL, APIKey: "k", Model: "m"},
	}

	content, used, err := GenerateWithFallback(context.Background(), providers, []Message{{Role: "user", Content: "hi"}}, 5*time.Second, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if content != "hello" || used.Name != "openrouter" {
		t.Errorf("expected the configured provider to be used, got content=%q used=%q", content, used.Name)
	}
}

func TestGenerateWithFallback_NoProvidersConfigured(t *testing.T) {
	providers := []Provider{
		{Name: "deepseek", BaseURL: "", APIKey: "", Model: ""},
		{Name: "openrouter", BaseURL: "", APIKey: "", Model: ""},
	}

	_, _, err := GenerateWithFallback(context.Background(), providers, []Message{{Role: "user", Content: "hi"}}, 5*time.Second, nil)
	if err == nil {
		t.Fatal("expected an error when no provider is configured")
	}
}
