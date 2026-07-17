package ai

import (
	"strings"
	"testing"
)

func validResultJSON() string {
	return `{
		"roast": "这个人评论言简意赅，但一言不合就开始阴阳怪气。",
		"profile": {
			"summary": "偏爱简短直接的表达，观点鲜明，互动较少。",
			"expressionStyle": ["简短直接", "偶尔讽刺"],
			"opinionTendency": ["倾向支持用户"],
			"interactionPattern": ["很少回复"],
			"genderGuess": {"value": "无法从文本判断", "confidence": "high", "disclaimer": "ignored"},
			"mbtiGuess": {"value": "ENTP", "confidence": "high", "disclaimer": "ignored"}
		},
		"evidence": [
			{"claim": "表达简短", "evidenceIds": ["c1", "c99"], "reason": "多条评论都很短。"}
		],
		"limitations": ["样本有限"]
	}`
}

func TestParseAndSanitizeUserSummaryResult_Valid(t *testing.T) {
	allowed := map[string]bool{"c1": true, "c2": true}
	result, err := ParseAndSanitizeUserSummaryResult(validResultJSON(), allowed)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if result.Roast == "" {
		t.Error("expected roast to be set")
	}
	if result.Profile.Summary == "" {
		t.Error("expected profile.summary to be set")
	}

	// Disclaimers must always be the hardcoded safety text, regardless of
	// what the model returned, so a model that ignores instructions can't
	// drop the "entertainment only" framing.
	if result.Profile.GenderGuess.Disclaimer != GenderGuessDisclaimer {
		t.Errorf("expected gender disclaimer to be forced to %q, got %q", GenderGuessDisclaimer, result.Profile.GenderGuess.Disclaimer)
	}
	if result.Profile.MBTIGuess.Disclaimer != MBTIGuessDisclaimer {
		t.Errorf("expected mbti disclaimer to be forced to %q, got %q", MBTIGuessDisclaimer, result.Profile.MBTIGuess.Disclaimer)
	}
	if result.Profile.GenderGuess.Confidence != "low" || result.Profile.MBTIGuess.Confidence != "low" {
		t.Error("expected confidence to always be forced to 'low'")
	}

	// c99 is not in the allowed set (only 120 real samples exist), so it
	// must be dropped even though c1 survives.
	if len(result.Evidence) != 1 {
		t.Fatalf("expected 1 evidence item, got %d", len(result.Evidence))
	}
	ids := result.Evidence[0].EvidenceIDs
	if len(ids) != 1 || ids[0] != "c1" {
		t.Errorf("expected evidenceIds to be filtered to allowed IDs only, got %v", ids)
	}
}

func TestParseAndSanitizeUserSummaryResult_StripsCodeFence(t *testing.T) {
	wrapped := "```json\n" + validResultJSON() + "\n```"
	result, err := ParseAndSanitizeUserSummaryResult(wrapped, map[string]bool{"c1": true})
	if err != nil {
		t.Fatalf("expected code-fence-wrapped JSON to parse, got error: %v", err)
	}
	if result.Roast == "" {
		t.Error("expected roast to survive code fence stripping")
	}
}

func TestParseAndSanitizeUserSummaryResult_MissingRoastFails(t *testing.T) {
	_, err := ParseAndSanitizeUserSummaryResult(`{"roast": "", "profile": {"summary": "x"}}`, nil)
	if err == nil {
		t.Fatal("expected an error for missing roast")
	}
}

func TestParseAndSanitizeUserSummaryResult_MissingSummaryFails(t *testing.T) {
	_, err := ParseAndSanitizeUserSummaryResult(`{"roast": "x", "profile": {"summary": ""}}`, nil)
	if err == nil {
		t.Fatal("expected an error for missing profile.summary")
	}
}

func TestParseAndSanitizeUserSummaryResult_InvalidJSONFails(t *testing.T) {
	_, err := ParseAndSanitizeUserSummaryResult("not json at all", nil)
	if err == nil {
		t.Fatal("expected an error for invalid JSON")
	}
}

func TestParseAndSanitizeUserSummaryResult_EmptyGuessGetsForcedFallback(t *testing.T) {
	raw := `{
		"roast": "x",
		"profile": {
			"summary": "y",
			"genderGuess": {"value": ""},
			"mbtiGuess": {"value": ""}
		}
	}`
	result, err := ParseAndSanitizeUserSummaryResult(raw, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Profile.GenderGuess.Value == "" || isAvoidantGuess(result.Profile.GenderGuess.Value) {
		t.Errorf("expected a concrete forced gender guess, got %q", result.Profile.GenderGuess.Value)
	}
	if result.Profile.MBTIGuess.Value == "" || isAvoidantGuess(result.Profile.MBTIGuess.Value) {
		t.Errorf("expected a concrete forced mbti guess, got %q", result.Profile.MBTIGuess.Value)
	}
	if result.Profile.GenderGuess.Confidence != "low" || result.Profile.MBTIGuess.Confidence != "low" {
		t.Errorf("expected forced guesses to keep confidence=low")
	}
}

func TestParseAndSanitizeUserSummaryResult_AvoidantGuessGetsForcedFallback(t *testing.T) {
	raw := `{
		"roast": "x",
		"profile": {
			"summary": "y",
			"genderGuess": {"value": "无法从文本判断"},
			"mbtiGuess": {"value": "不确定"}
		}
	}`
	result, err := ParseAndSanitizeUserSummaryResult(raw, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if isAvoidantGuess(result.Profile.GenderGuess.Value) {
		t.Errorf("expected avoidant gender guess to be replaced, got %q", result.Profile.GenderGuess.Value)
	}
	if isAvoidantGuess(result.Profile.MBTIGuess.Value) {
		t.Errorf("expected avoidant mbti guess to be replaced, got %q", result.Profile.MBTIGuess.Value)
	}
}

func TestParseAndSanitizeUserSummaryResult_ClampsOversizedLists(t *testing.T) {
	raw := `{
		"roast": "x",
		"profile": {
			"summary": "y",
			"expressionStyle": ["1", "2", "3", "4", "5"]
		},
		"evidence": [
			{"claim": "a", "reason": "r"},
			{"claim": "b", "reason": "r"},
			{"claim": "c", "reason": "r"},
			{"claim": "d", "reason": "r"},
			{"claim": "e", "reason": "r"},
			{"claim": "f", "reason": "r"}
		]
	}`
	result, err := ParseAndSanitizeUserSummaryResult(raw, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Profile.ExpressionStyle) != maxFeatureItems {
		t.Errorf("expected expressionStyle clamped to %d items, got %d", maxFeatureItems, len(result.Profile.ExpressionStyle))
	}
	if len(result.Evidence) != maxEvidenceItems {
		t.Errorf("expected evidence clamped to %d items, got %d", maxEvidenceItems, len(result.Evidence))
	}
}

func TestParseAndSanitizeUserSummaryResult_TruncatesLongStrings(t *testing.T) {
	longRoast := strings.Repeat("犀", maxRoastRunes+50)
	raw := `{"roast": "` + longRoast + `", "profile": {"summary": "y"}}`
	result, err := ParseAndSanitizeUserSummaryResult(raw, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len([]rune(result.Roast)) != maxRoastRunes {
		t.Errorf("expected roast truncated to %d runes, got %d", maxRoastRunes, len([]rune(result.Roast)))
	}
}

func TestParseAndSanitizeUserSummaryResult_MissingLimitationsGetsDefault(t *testing.T) {
	raw := `{"roast": "x", "profile": {"summary": "y"}, "limitations": []}`
	result, err := ParseAndSanitizeUserSummaryResult(raw, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Limitations) == 0 {
		t.Error("expected a default limitation to be filled in when the model omits it")
	}
}
