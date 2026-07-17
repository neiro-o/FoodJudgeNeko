package ai

import (
	"encoding/json"
	"fmt"
	"strings"
	"unicode/utf8"
)

// UserSummaryResult is the structured "犀利点评 + 用户画像" produced by the model.
// Field names/tags mirror the JSON schema in prompts/user_profile_summary_v1.md.
type UserSummaryResult struct {
	Roast       string                `json:"roast" bson:"roast"`
	Profile     UserSummaryProfile    `json:"profile" bson:"profile"`
	Evidence    []UserSummaryEvidence `json:"evidence" bson:"evidence"`
	Limitations []string              `json:"limitations" bson:"limitations"`
}

type UserSummaryProfile struct {
	Summary            string           `json:"summary" bson:"summary"`
	ExpressionStyle    []string         `json:"expressionStyle" bson:"expressionStyle"`
	OpinionTendency    []string         `json:"opinionTendency" bson:"opinionTendency"`
	InteractionPattern []string         `json:"interactionPattern" bson:"interactionPattern"`
	GenderGuess        UserSummaryGuess `json:"genderGuess" bson:"genderGuess"`
	MBTIGuess          UserSummaryGuess `json:"mbtiGuess" bson:"mbtiGuess"`
}

type UserSummaryGuess struct {
	Value      string `json:"value" bson:"value"`
	Confidence string `json:"confidence" bson:"confidence"`
	Disclaimer string `json:"disclaimer" bson:"disclaimer"`
}

type UserSummaryEvidence struct {
	Claim       string   `json:"claim" bson:"claim"`
	EvidenceIDs []string `json:"evidenceIds" bson:"evidenceIds"`
	Reason      string   `json:"reason" bson:"reason"`
}

const (
	// UnknownGuessValue is what we force into gender/MBTI guesses that the
	// model left empty or that reference no valid evidence sample.
	UnknownGuessValue = "无法从文本判断"
	// GenderGuessDisclaimer and MBTIGuessDisclaimer are always re-applied by
	// ParseAndSanitizeUserSummaryResult regardless of what the model
	// returned, so the "entertainment only, not fact" framing can never be
	// silently dropped by a model that ignores instructions.
	GenderGuessDisclaimer = "非事实、不可用于判断真实身份"
	MBTIGuessDisclaimer   = "非心理测量、不可用于判断真实人格"

	maxFeatureItems        = 3
	maxFeatureItemRunes    = 24
	maxEvidenceItems       = 5
	maxEvidenceReasonRunes = 80
	maxRoastRunes          = 400
	maxSummaryRunes        = 200
)

// ParseAndSanitizeUserSummaryResult decodes the raw model output (tolerating
// accidental markdown code fences) into a UserSummaryResult, validates that
// the minimally-required fields are present, and clamps/sanitizes the rest so
// a model that doesn't perfectly follow instructions can't produce an unsafe
// or oversized payload. allowedEvidenceIDs restricts UserSummaryEvidence.EvidenceIDs
// to sample IDs we actually sent (e.g. "c1".."c120"); unknown IDs are dropped.
func ParseAndSanitizeUserSummaryResult(raw string, allowedEvidenceIDs map[string]bool) (*UserSummaryResult, error) {
	cleaned := stripCodeFence(raw)

	var result UserSummaryResult
	if err := json.Unmarshal([]byte(cleaned), &result); err != nil {
		return nil, fmt.Errorf("failed to parse model output as JSON: %w", err)
	}

	result.Roast = strings.TrimSpace(result.Roast)
	if result.Roast == "" {
		return nil, fmt.Errorf("model output missing required field: roast")
	}
	result.Roast = truncateRunes(result.Roast, maxRoastRunes)

	result.Profile.Summary = strings.TrimSpace(result.Profile.Summary)
	if result.Profile.Summary == "" {
		return nil, fmt.Errorf("model output missing required field: profile.summary")
	}
	result.Profile.Summary = truncateRunes(result.Profile.Summary, maxSummaryRunes)

	result.Profile.ExpressionStyle = sanitizeFeatureList(result.Profile.ExpressionStyle)
	result.Profile.OpinionTendency = sanitizeFeatureList(result.Profile.OpinionTendency)
	result.Profile.InteractionPattern = sanitizeFeatureList(result.Profile.InteractionPattern)

	result.Profile.GenderGuess = sanitizeGuess(result.Profile.GenderGuess, GenderGuessDisclaimer)
	result.Profile.MBTIGuess = sanitizeGuess(result.Profile.MBTIGuess, MBTIGuessDisclaimer)

	result.Evidence = sanitizeEvidence(result.Evidence, allowedEvidenceIDs)

	result.Limitations = sanitizeLimitations(result.Limitations)

	return &result, nil
}

func stripCodeFence(raw string) string {
	s := strings.TrimSpace(raw)
	if !strings.HasPrefix(s, "```") {
		return s
	}
	s = strings.TrimPrefix(s, "```")
	if idx := strings.Index(s, "\n"); idx != -1 && strings.TrimSpace(s[:idx]) != "" && !strings.Contains(s[:idx], "{") {
		// First line was a language tag like "json"
		s = s[idx+1:]
	}
	s = strings.TrimSuffix(strings.TrimSpace(s), "```")
	return strings.TrimSpace(s)
}

func truncateRunes(s string, max int) string {
	if utf8.RuneCountInString(s) <= max {
		return s
	}
	runes := []rune(s)
	return string(runes[:max])
}

func sanitizeFeatureList(items []string) []string {
	result := make([]string, 0, maxFeatureItems)
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		result = append(result, truncateRunes(item, maxFeatureItemRunes))
		if len(result) >= maxFeatureItems {
			break
		}
	}
	return result
}

func sanitizeGuess(guess UserSummaryGuess, disclaimer string) UserSummaryGuess {
	value := strings.TrimSpace(guess.Value)
	if value == "" {
		value = UnknownGuessValue
	}
	return UserSummaryGuess{
		Value:      truncateRunes(value, maxFeatureItemRunes),
		Confidence: "low",
		Disclaimer: disclaimer,
	}
}

func sanitizeEvidence(items []UserSummaryEvidence, allowedEvidenceIDs map[string]bool) []UserSummaryEvidence {
	result := make([]UserSummaryEvidence, 0, maxEvidenceItems)
	for _, item := range items {
		claim := strings.TrimSpace(item.Claim)
		if claim == "" {
			continue
		}
		ids := make([]string, 0, len(item.EvidenceIDs))
		for _, id := range item.EvidenceIDs {
			if allowedEvidenceIDs == nil || allowedEvidenceIDs[id] {
				ids = append(ids, id)
			}
		}
		result = append(result, UserSummaryEvidence{
			Claim:       truncateRunes(claim, maxFeatureItemRunes*2),
			EvidenceIDs: ids,
			Reason:      truncateRunes(strings.TrimSpace(item.Reason), maxEvidenceReasonRunes),
		})
		if len(result) >= maxEvidenceItems {
			break
		}
	}
	return result
}

func sanitizeLimitations(items []string) []string {
	result := make([]string, 0, len(items))
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		result = append(result, truncateRunes(item, 120))
	}
	if len(result) == 0 {
		result = append(result, "样本仅覆盖点赞数最高的历史评论，可能无法代表用户全部发言。")
	}
	return result
}
