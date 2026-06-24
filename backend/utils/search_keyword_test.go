package utils

import (
	"testing"
)

// ----- SearchKeywordVariants -----------------------------------------------

func TestSearchKeywordVariants_deduplication(t *testing.T) {
	// A plain ASCII keyword with no full-width equivalent should still deduplicate.
	v := SearchKeywordVariants("hello")
	seen := map[string]bool{}
	for _, s := range v {
		if seen[s] {
			t.Errorf("duplicate variant %q", s)
		}
		seen[s] = true
	}
}

func TestSearchKeywordVariants_containsHalfAndFullWidth(t *testing.T) {
	// Keyword with full-width comma: variants must include ASCII comma version.
	variants := SearchKeywordVariants("好，")
	halfWidthFound := false
	for _, v := range variants {
		if v == "好," {
			halfWidthFound = true
		}
	}
	if !halfWidthFound {
		t.Errorf("expected half-width variant \"好,\" in %v", variants)
	}
}

func TestSearchKeywordVariants_halfWidthToFullWidth(t *testing.T) {
	// Keyword with ASCII comma: variants must include full-width comma version.
	variants := SearchKeywordVariants("好,")
	fullWidthFound := false
	for _, v := range variants {
		if v == "好，" {
			fullWidthFound = true
		}
	}
	if !fullWidthFound {
		t.Errorf("expected full-width variant \"好，\" in %v", variants)
	}
}

func TestSearchKeywordVariants_quotes(t *testing.T) {
	// ASCII double-quote should produce a curly-quote variant (or vice versa).
	variants := SearchKeywordVariants(`"test"`)
	hasOriginal := false
	for _, v := range variants {
		if v == `"test"` {
			hasOriginal = true
		}
	}
	if !hasOriginal {
		t.Errorf("original keyword must always be first variant; got %v", variants)
	}
	if len(variants) < 2 {
		t.Errorf("expected multiple variants for %q, got %v", `"test"`, variants)
	}
}

func TestSearchKeywordVariants_originalAlwaysFirst(t *testing.T) {
	kw := "Test，"
	variants := SearchKeywordVariants(kw)
	if len(variants) == 0 || variants[0] != kw {
		t.Errorf("first variant should equal original keyword; got %v", variants)
	}
}

// ----- NormalizeHalfWidthPunctuation ----------------------------------------

func TestNormalizeHalfWidthPunctuation_chinesePunctuation(t *testing.T) {
	cases := []struct{ in, want string }{
		{"好。", "好."},
		{"好，", "好,"},
		{"\u3000", " "},
		{"A—B", "A-B"},
	}
	for _, c := range cases {
		got := NormalizeHalfWidthPunctuation(c.in)
		if got != c.want {
			t.Errorf("NormalizeHalfWidthPunctuation(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// ----- ExtractTrailingDate --------------------------------------------------

func TestExtractTrailingDate_noDate(t *testing.T) {
	kw, sd := ExtractTrailingDate("不好吃")
	if kw != "不好吃" || sd != nil {
		t.Errorf("expected no extraction; got kw=%q sd=%v", kw, sd)
	}
}

func TestExtractTrailingDate_fullDateSlash(t *testing.T) {
	kw, sd := ExtractTrailingDate("不好吃 2026/6/23")
	if kw != "不好吃" {
		t.Errorf("wrong keyword %q", kw)
	}
	if sd == nil || sd.FullDate == nil {
		t.Fatal("expected FullDate to be set")
	}
	if sd.MonthDayOnly != nil {
		t.Error("MonthDayOnly should be nil")
	}
	// Start should be unix timestamp of 2026-06-23 00:00:00 Asia/Shanghai
	if sd.FullDate.Start <= 0 || sd.FullDate.End <= sd.FullDate.Start {
		t.Errorf("invalid date range %+v", sd.FullDate)
	}
}

func TestExtractTrailingDate_fullDateDash(t *testing.T) {
	kw, sd := ExtractTrailingDate("不好吃 2026-6-23")
	if kw != "不好吃" || sd == nil || sd.FullDate == nil {
		t.Errorf("expected full date; got kw=%q sd=%v", kw, sd)
	}
}

func TestExtractTrailingDate_fullDateChinese(t *testing.T) {
	kw, sd := ExtractTrailingDate("不好吃 2026年6月23日")
	if kw != "不好吃" || sd == nil || sd.FullDate == nil {
		t.Errorf("expected full date; got kw=%q sd=%v", kw, sd)
	}
}

func TestExtractTrailingDate_monthDayOnly(t *testing.T) {
	kw, sd := ExtractTrailingDate("不好吃 6-23")
	if kw != "不好吃" {
		t.Errorf("wrong keyword %q", kw)
	}
	if sd == nil || sd.MonthDayOnly == nil {
		t.Fatal("expected MonthDayOnly to be set")
	}
	if sd.MonthDayOnly.Month != 6 || sd.MonthDayOnly.Day != 23 {
		t.Errorf("wrong month/day %+v", sd.MonthDayOnly)
	}
}

func TestExtractTrailingDate_monthDayZeroPadded(t *testing.T) {
	kw, sd := ExtractTrailingDate("好吃 06-23")
	if kw != "好吃" || sd == nil || sd.MonthDayOnly == nil {
		t.Errorf("expected MonthDayOnly; got kw=%q sd=%v", kw, sd)
	}
}

func TestExtractTrailingDate_ratioNotDate(t *testing.T) {
	// "50-50" looks like a month-day but month 50 is invalid → keep intact.
	kw, sd := ExtractTrailingDate("50-50")
	if kw != "50-50" || sd != nil {
		t.Errorf("should not extract date from ratio; got kw=%q sd=%v", kw, sd)
	}
}

func TestExtractTrailingDate_keywordOnlyIsDate_notStripped(t *testing.T) {
	// If stripping would leave an empty keyword, don't strip.
	kw, sd := ExtractTrailingDate("2026/6/23")
	if kw != "2026/6/23" || sd != nil {
		t.Errorf("should not strip when keyword would be empty; got kw=%q sd=%v", kw, sd)
	}
}

func TestExtractTrailingDate_invalidDay(t *testing.T) {
	// 2-32 is an invalid date (day 32 doesn't exist)
	kw, sd := ExtractTrailingDate("好 2-32")
	if sd != nil {
		t.Errorf("should not parse invalid day; got kw=%q sd=%v", kw, sd)
	}
}

// ----- BuildDateFilter -------------------------------------------------------

func TestBuildDateFilter_nil(t *testing.T) {
	if BuildDateFilter(nil) != nil {
		t.Error("nil SearchDate should return nil filter")
	}
}

func TestBuildDateFilter_fullDate(t *testing.T) {
	_, sd := ExtractTrailingDate("不好吃 2026/6/23")
	f := BuildDateFilter(sd)
	if _, ok := f["range"]; !ok {
		t.Errorf("expected range filter for full date; got %v", f)
	}
}

func TestBuildDateFilter_monthDay(t *testing.T) {
	_, sd := ExtractTrailingDate("不好吃 6-23")
	f := BuildDateFilter(sd)
	if _, ok := f["script"]; !ok {
		t.Errorf("expected script filter for month-day; got %v", f)
	}
}
