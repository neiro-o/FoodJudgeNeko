package ai

import (
	"encoding/json"
	"fmt"
	"hash/fnv"
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

// genderGuessFallbacks and mbtiGuessFallbacks are the concrete, always-committal
// values used when the model leaves a guess empty or dodges with an avoidance
// phrase (e.g. "无法判断"). We deliberately never fall back to an "unknown"
// placeholder here: the guess is explicitly entertainment-only and disclaimed,
// so it's safe to always force a specific pick rather than a copout.
var genderGuessFallbacks = []string{"更像男性表达", "更像女性表达"}

var mbtiGuessFallbacks = []string{
	"INTJ", "INTP", "ENTJ", "ENTP",
	"INFJ", "INFP", "ENFJ", "ENFP",
	"ISTJ", "ISFJ", "ESTJ", "ESFJ",
	"ISTP", "ISFP", "ESTP", "ESFP",
}

// avoidancePhrases are fragments that indicate the model dodged the guess
// instead of committing to one, despite the prompt forbidding this.
var avoidancePhrases = []string{
	"无法判断", "无法从文本判断", "不确定", "难以判断", "不清楚", "未知", "无法确定", "不好判断",
}

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

	// Seed the deterministic fallback pick off content the model actually
	// generated for this user, so the forced guess is stable across
	// re-parses of the same response but still varies between users.
	seed := result.Roast + "|" + result.Profile.Summary

	result.Profile.GenderGuess = sanitizeGuess(result.Profile.GenderGuess, GenderGuessDisclaimer, genderGuessFallbacks, seed+"|gender")
	result.Profile.MBTIGuess = sanitizeGuess(result.Profile.MBTIGuess, MBTIGuessDisclaimer, mbtiGuessFallbacks, seed+"|mbti")

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

func sanitizeGuess(guess UserSummaryGuess, disclaimer string, fallbacks []string, seed string) UserSummaryGuess {
	value := strings.TrimSpace(guess.Value)
	if value == "" || isAvoidantGuess(value) {
		value = pickDeterministic(fallbacks, seed)
	}
	return UserSummaryGuess{
		Value:      truncateRunes(value, maxFeatureItemRunes),
		Confidence: "low",
		Disclaimer: disclaimer,
	}
}

// isAvoidantGuess reports whether the model dodged committing to a guess
// (e.g. "无法判断") instead of following the prompt's instruction to always
// give a specific, low-confidence, entertainment-only pick.
func isAvoidantGuess(value string) bool {
	for _, phrase := range avoidancePhrases {
		if strings.Contains(value, phrase) {
			return true
		}
	}
	return false
}

// pickDeterministic selects one of the candidates based on a hash of seed,
// so the same model output always resolves to the same forced fallback.
func pickDeterministic(candidates []string, seed string) string {
	if len(candidates) == 0 {
		return ""
	}
	h := fnv.New32a()
	_, _ = h.Write([]byte(seed))
	return candidates[h.Sum32()%uint32(len(candidates))]
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
