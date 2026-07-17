package ai

import (
	"strings"
	"testing"
)

func TestDecryptNegativeKeywordRule(t *testing.T) {
	rule, err := decryptNegativeKeywordRule()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rule == "" {
		t.Fatal("expected a non-empty decrypted rule")
	}
	// Sanity check that we decrypted the expected rule content, without
	// hardcoding the literal trigger tokens themselves in this test file.
	if !strings.Contains(rule, "黑话规则") {
		t.Errorf("expected decrypted rule to contain the rule heading, got: %q", rule)
	}
}

func TestBuildUserProfileSummaryPrompt(t *testing.T) {
	prompt, err := BuildUserProfileSummaryPrompt()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(prompt, negativeKeywordRulePlaceholder) {
		t.Error("expected placeholder to be substituted in the built prompt")
	}
	if !strings.Contains(prompt, "黑话规则") {
		t.Error("expected built prompt to contain the decrypted rule text")
	}
}
