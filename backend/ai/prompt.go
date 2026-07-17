// Package ai contains the versioned prompt(s) and the minimal OpenAI-compatible
// chat completion client used to generate AI user summaries.
package ai

import (
	"fmt"
	"strings"

	_ "embed"
)

// UserProfileSummaryPromptV1 is the system prompt template for the "犀利点评 +
// 用户画像" feature on the user stats page. It is embedded at compile time
// (not stored in MongoDB or config.yml) because it is product/prompt-engineering
// logic that should be reviewed, tested, and released together with the code
// that consumes it. Cached results only store UserProfileSummaryPromptVersion
// so a prompt change is traceable against previously generated summaries.
//
// This is a template, not the final prompt: it contains the
// negativeKeywordRulePlaceholder token in place of the sensitive
// negative-keyword rule text (see keywords.go for why that text is kept
// encrypted rather than plaintext in source). Use
// BuildUserProfileSummaryPrompt to get the final, ready-to-send prompt.
//
//go:embed prompts/user_profile_summary_v1.md
var UserProfileSummaryPromptV1 string

// UserProfileSummaryPromptVersion identifies the prompt revision used to
// produce a cached summary. Bump this whenever the prompt content changes in
// a way that would make old cached results inconsistent with new ones.
const UserProfileSummaryPromptVersion = "user_profile_summary_v1"

// BuildUserProfileSummaryPrompt returns the final system prompt to send to
// the model: the UserProfileSummaryPromptV1 template with
// negativeKeywordRulePlaceholder substituted for the decrypted
// negative-keyword rule text.
func BuildUserProfileSummaryPrompt() (string, error) {
	rule, err := decryptNegativeKeywordRule()
	if err != nil {
		return "", fmt.Errorf("build user profile summary prompt: %w", err)
	}
	if !strings.Contains(UserProfileSummaryPromptV1, negativeKeywordRulePlaceholder) {
		return "", fmt.Errorf("build user profile summary prompt: template missing placeholder %q", negativeKeywordRulePlaceholder)
	}
	return strings.Replace(UserProfileSummaryPromptV1, negativeKeywordRulePlaceholder, rule, 1), nil
}
