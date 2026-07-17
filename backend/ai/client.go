package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Provider identifies one OpenAI-compatible chat completion endpoint to try.
type Provider struct {
	// Name is a short identifier used for logging and for the "provider"
	// field stored alongside a cached result (e.g. "deepseek", "openrouter").
	Name    string
	BaseURL string
	APIKey  string
	Model   string
}

// Message is a single OpenAI-compatible chat message.
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatCompletionRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	Temperature float64   `json:"temperature"`
	MaxTokens   int       `json:"max_tokens"`
}

type chatCompletionResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// ProviderError wraps a failure from a single provider attempt so callers can
// report which provider failed and why.
type ProviderError struct {
	Provider string
	Err      error
}

func (e *ProviderError) Error() string {
	return fmt.Sprintf("provider %s: %v", e.Provider, e.Err)
}

func (e *ProviderError) Unwrap() error {
	return e.Err
}

// requestChatCompletion performs a single OpenAI-compatible
// "/chat/completions" call against the given provider and returns the raw
// assistant message content.
func requestChatCompletion(ctx context.Context, p Provider, messages []Message, timeout time.Duration) (string, error) {
	reqCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	body, err := json.Marshal(chatCompletionRequest{
		Model:       p.Model,
		Messages:    messages,
		Temperature: 0.7,
		MaxTokens:   1400,
	})
	if err != nil {
		return "", fmt.Errorf("failed to encode request: %w", err)
	}

	url := strings.TrimRight(p.BaseURL, "/") + "/chat/completions"
	httpReq, err := http.NewRequestWithContext(reqCtx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("failed to build request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+p.APIKey)

	client := &http.Client{Timeout: timeout}
	resp, err := client.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20)) // 2MB cap
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	var parsed chatCompletionResponse
	if jsonErr := json.Unmarshal(respBody, &parsed); jsonErr != nil {
		return "", fmt.Errorf("status %d: failed to parse response: %w", resp.StatusCode, jsonErr)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if parsed.Error != nil && parsed.Error.Message != "" {
			return "", fmt.Errorf("status %d: %s", resp.StatusCode, parsed.Error.Message)
		}
		return "", fmt.Errorf("status %d", resp.StatusCode)
	}

	if len(parsed.Choices) == 0 || parsed.Choices[0].Message.Content == "" {
		return "", fmt.Errorf("empty completion from provider")
	}

	return parsed.Choices[0].Message.Content, nil
}

// GenerateWithFallback tries each provider in order (typically DeepSeek then
// OpenRouter) and returns the first successful completion. onAttemptFailed,
// if non-nil, is called with the error for every failed attempt (including
// ones followed by a successful fallback) so the caller can log/record the
// reason without failing the whole request.
func GenerateWithFallback(
	ctx context.Context,
	providers []Provider,
	messages []Message,
	timeout time.Duration,
	onAttemptFailed func(provider string, err error),
) (content string, usedProvider Provider, err error) {
	var lastErr error
	attempted := false

	for _, p := range providers {
		if p.BaseURL == "" || p.APIKey == "" || p.Model == "" {
			continue
		}
		attempted = true

		result, callErr := requestChatCompletion(ctx, p, messages, timeout)
		if callErr != nil {
			wrapped := &ProviderError{Provider: p.Name, Err: callErr}
			lastErr = wrapped
			if onAttemptFailed != nil {
				onAttemptFailed(p.Name, callErr)
			}
			continue
		}

		return result, p, nil
	}

	if !attempted {
		return "", Provider{}, fmt.Errorf("no AI provider is configured")
	}
	return "", Provider{}, fmt.Errorf("all AI providers failed: %w", lastErr)
}
