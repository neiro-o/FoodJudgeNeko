package handlers

import (
	"context"
	"testing"
)

func TestFindAccountUsernamesLeavesInvalidLegacyIDsEmpty(t *testing.T) {
	usernames, err := findAccountUsernames(context.Background(), []string{"legacy-uploader", ""})
	if err != nil {
		t.Fatalf("find account usernames: %v", err)
	}
	if usernames["legacy-uploader"] != "" {
		t.Fatalf("expected empty username, got %q", usernames["legacy-uploader"])
	}
}
