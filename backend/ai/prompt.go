// Package ai contains the versioned prompt(s) and the minimal OpenAI-compatible
// chat completion client used to generate AI user summaries.
package ai

import _ "embed"

// UserProfileSummaryPromptV1 is the system prompt for the "犀利点评 + 用户画像"
// feature on the user stats page. It is embedded at compile time (not stored in
// MongoDB or config.yml) because it is product/prompt-engineering logic that
// should be reviewed, tested, and released together with the code that
// consumes it. Cached results only store UserProfileSummaryPromptVersion so a
// prompt change is traceable against previously generated summaries.
//
//go:embed prompts/user_profile_summary_v1.md
var UserProfileSummaryPromptV1 string

// UserProfileSummaryPromptVersion identifies the prompt revision used to
// produce a cached summary. Bump this whenever the prompt content changes in
// a way that would make old cached results inconsistent with new ones.
const UserProfileSummaryPromptVersion = "user_profile_summary_v1"
