package handlers

import (
	"net/url"
	"strings"
	"testing"

	"mtv2/backend/config"
)

func TestRewriteHLSPlaylistProxiesRelativeResources(t *testing.T) {
	config.AppConfig = &config.Config{}
	config.AppConfig.Server.JWTSecret = "test-secret"
	playlistURL := "https://s3plus.sankuai.com/s3plus-video-sloth/m3u8/video.m3u8"
	manifest := "#EXTM3U\n#EXT-X-KEY:METHOD=AES-128,URI=\"keys/key.bin\"\n#EXTINF:4.0,\nchunks/0.ts\n#EXT-X-STREAM-INF:BANDWIDTH=1280000\nvariant/index.m3u8\n"

	rewritten, err := rewriteHLSPlaylist([]byte(manifest), playlistURL, "/api/media/video")
	if err != nil {
		t.Fatalf("rewrite playlist: %v", err)
	}
	output := string(rewritten)

	expectedURLs := []string{
		"https://s3plus.sankuai.com/s3plus-video-sloth/m3u8/keys/key.bin",
		"https://s3plus.sankuai.com/s3plus-video-sloth/m3u8/chunks/0.ts",
		"https://s3plus.sankuai.com/s3plus-video-sloth/m3u8/variant/index.m3u8",
	}
	for _, expectedURL := range expectedURLs {
		encoded := url.QueryEscape(expectedURL)
		if !strings.Contains(output, encoded) {
			t.Errorf("rewritten playlist does not contain proxied URL %q:\n%s", expectedURL, output)
		}
		if !strings.Contains(output, url.QueryEscape(generateMediaHash(expectedURL))) {
			t.Errorf("rewritten playlist does not contain hash for %q", expectedURL)
		}
	}
	if strings.Contains(output, "\nchunks/0.ts\n") || strings.Contains(output, `URI="keys/key.bin"`) {
		t.Fatalf("relative HLS resource was left unproxied:\n%s", output)
	}
	if !strings.HasSuffix(output, "\n") {
		t.Fatal("expected trailing newline to be preserved")
	}
}

func TestRewriteHLSPlaylistRejectsDisallowedResourceDomain(t *testing.T) {
	config.AppConfig = &config.Config{}
	config.AppConfig.Server.JWTSecret = "test-secret"
	_, err := rewriteHLSPlaylist(
		[]byte("#EXTM3U\nhttps://evil.example/segment.ts\n"),
		"https://s3plus.sankuai.com/video/index.m3u8",
		"/api/media/video",
	)
	if err == nil {
		t.Fatal("expected disallowed HLS resource domain to be rejected")
	}
}
