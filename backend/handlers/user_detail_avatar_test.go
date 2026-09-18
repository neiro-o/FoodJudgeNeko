package handlers

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson"
)

func TestLatestUserAvatarOptionsSortsByNewestComment(t *testing.T) {
	want := bson.D{
		{Key: "createTime", Value: -1},
		{Key: "_id", Value: -1},
	}
	if got := latestUserAvatarOptions().Sort; !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected avatar lookup sort: got %#v, want %#v", got, want)
	}
}

func TestFindFreshUserAvatarCacheHonors48HourTTL(t *testing.T) {
	oldWorkingDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory: %v", err)
	}
	tempDir := t.TempDir()
	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("change working directory: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldWorkingDir) })

	cachePath := filepath.Join("cache", "img", "avatar_123.jpg")
	if err := os.MkdirAll(filepath.Dir(cachePath), 0755); err != nil {
		t.Fatalf("create cache directory: %v", err)
	}
	if err := os.WriteFile(cachePath, []byte("avatar"), 0644); err != nil {
		t.Fatalf("write cache file: %v", err)
	}

	now := time.Now()
	if err := os.Chtimes(cachePath, now.Add(-47*time.Hour), now.Add(-47*time.Hour)); err != nil {
		t.Fatalf("set fresh cache time: %v", err)
	}
	if got, ok := findFreshUserAvatarCache("123", now); !ok || got != cachePath {
		t.Fatalf("expected fresh cache %q, got %q (found=%v)", cachePath, got, ok)
	}

	if err := os.Chtimes(cachePath, now.Add(-49*time.Hour), now.Add(-49*time.Hour)); err != nil {
		t.Fatalf("set stale cache time: %v", err)
	}
	if got, ok := findFreshUserAvatarCache("123", now); ok {
		t.Fatalf("expected stale cache to be rejected, got %q", got)
	}
}
