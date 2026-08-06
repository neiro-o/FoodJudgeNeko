package handlers

import (
	"context"
	"testing"
)

func TestFindUploaderNameIgnoresInvalidLegacyID(t *testing.T) {
	username, err := findUploaderName(context.Background(), "legacy-uploader")
	if err != nil {
		t.Fatalf("find uploader name: %v", err)
	}
	if username != "" {
		t.Fatalf("expected empty username, got %q", username)
	}
}
