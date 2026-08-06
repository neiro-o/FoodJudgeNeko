package handlers

import (
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type avatarRoundTripFunc func(*http.Request) (*http.Response, error)

func (f avatarRoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestAccountAvatarSourceURLs(t *testing.T) {
	sources := accountAvatarSourceURLs("  123456@qq.com ")
	if len(sources) != 2 {
		t.Fatalf("expected QQ and Gravatar sources, got %d", len(sources))
	}

	qqURL, err := url.Parse(sources[0])
	if err != nil {
		t.Fatalf("parse QQ URL: %v", err)
	}
	if qqURL.Host != "q1.qlogo.cn" || qqURL.Query().Get("nk") != "123456" {
		t.Fatalf("unexpected QQ URL: %s", sources[0])
	}

	const expectedHash = "84059b07d4be67b806386c0aad8070a23f18836bbaae342275dc0a83414c32ee"
	gravatarSources := accountAvatarSourceURLs("MyEmailAddress@example.com ")
	if len(gravatarSources) != 1 {
		t.Fatalf("expected only Gravatar source, got %d", len(gravatarSources))
	}
	if !strings.Contains(gravatarSources[0], expectedHash) || !strings.Contains(gravatarSources[0], "d=404") {
		t.Fatalf("unexpected Gravatar URL: %s", gravatarSources[0])
	}
}

func TestFetchFirstAccountAvatarFallsBack(t *testing.T) {
	pngHeader := []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n', 0, 0, 0, 0}
	client := &http.Client{Transport: avatarRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		status := http.StatusOK
		body := pngHeader
		if req.URL.Path == "/qq" {
			status = http.StatusBadGateway
			body = []byte("upstream failed")
		}
		return &http.Response{
			StatusCode: status,
			Header:     http.Header{"Content-Type": {"image/png"}},
			Body:       io.NopCloser(strings.NewReader(string(body))),
			Request:    req,
		}, nil
	})}

	data, err := fetchFirstAccountAvatar(client, []string{"https://example.test/qq", "https://example.test/gravatar"})
	if err != nil {
		t.Fatalf("fetch avatar with fallback: %v", err)
	}
	if string(data) != string(pngHeader) {
		t.Fatalf("unexpected avatar data: %v", data)
	}
}

func TestFetchAccountAvatarRejectsNonImage(t *testing.T) {
	client := &http.Client{Transport: avatarRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": {"text/html"}},
			Body:       io.NopCloser(strings.NewReader("<html>not an avatar</html>")),
			Request:    req,
		}, nil
	})}

	if _, err := fetchAccountAvatar(client, "https://example.test/avatar"); err == nil {
		t.Fatal("expected non-image response to be rejected")
	}
}

func TestIsFreshAccountAvatarCache(t *testing.T) {
	cachePath := filepath.Join(t.TempDir(), "avatar.img")
	if err := os.WriteFile(cachePath, []byte("avatar"), 0644); err != nil {
		t.Fatalf("write cache file: %v", err)
	}

	now := time.Now()
	if err := os.Chtimes(cachePath, now.Add(-6*24*time.Hour), now.Add(-6*24*time.Hour)); err != nil {
		t.Fatalf("set fresh cache time: %v", err)
	}
	if !isFreshAccountAvatarCache(cachePath, now) {
		t.Fatal("expected six-day-old cache to be fresh")
	}

	if err := os.Chtimes(cachePath, now.Add(-8*24*time.Hour), now.Add(-8*24*time.Hour)); err != nil {
		t.Fatalf("set stale cache time: %v", err)
	}
	if isFreshAccountAvatarCache(cachePath, now) {
		t.Fatal("expected eight-day-old cache to be stale")
	}
}
