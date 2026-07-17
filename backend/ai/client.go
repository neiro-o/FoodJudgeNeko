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
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// TruncatedResponseError indicates the provider cut the completion short
// (finish_reason == "length") before it could finish writing valid JSON,
// most commonly because max_tokens was too small for the requested output.
// Content still holds whatever partial text the model produced, so callers
// can surface it for debugging instead of only reporting a generic JSON
// parse failure.
type TruncatedResponseError struct {
	Content string
}

func (e *TruncatedResponseError) Error() string {
	return "response was truncated by the provider (finish_reason=length), likely because max_tokens is too small for the requested output"
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
		// The structured output can include up to 300 evidence-linked
		// samples worth of context and a fairly large JSON schema
		// (roast + profile + up to 5 evidence items + limitations).
		// Chinese text also tends to burn more tokens per character than
		// English, so a small budget here silently truncates the JSON
		// mid-object (finish_reason=length) rather than erroring cleanly.
		// 4096 gives enough headroom while still bounding cost/latency.
		MaxTokens: 4096,
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

	choice := parsed.Choices[0]
	if choice.FinishReason == "length" {
		return "", &TruncatedResponseError{Content: choice.Message.Content}
	}

	return choice.Message.Content, nil
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
