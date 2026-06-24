package utils

import (
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"
)

// NormalizeHalfWidthPunctuation converts full-width punctuation and CJK symbols
// to their ASCII half-width equivalents. The n-gram tokenizer treats them as
// different tokens, so we normalise at query time to cover both forms.
func NormalizeHalfWidthPunctuation(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		switch {
		case r == '。':
			b.WriteRune('.')
		case r == '、':
			b.WriteRune(',')
		case r == '【':
			b.WriteRune('[')
		case r == '】':
			b.WriteRune(']')
		case r == '《':
			b.WriteRune('<')
		case r == '》':
			b.WriteRune('>')
		case r == '\u201c' || r == '\u201d':
			b.WriteRune('"')
		case r == '\u2018' || r == '\u2019':
			b.WriteRune('\'')
		case r == '\u3000': // ideographic space
			b.WriteRune(' ')
		case r == '—' || r == '–': // em dash, en dash
			b.WriteRune('-')
		case r == '＊':
			b.WriteRune('*')
		case r >= '！' && r <= '～': // fullwidth ASCII variants block
			b.WriteRune(r - 0xFEE0)
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// fullWidthPunctuationVariant converts common ASCII punctuation characters to
// their full-width CJK equivalents (the reverse direction of
// NormalizeHalfWidthPunctuation).
func fullWidthPunctuationVariant(s string) string {
	punctuationMap := map[rune]rune{
		',':  '，',
		'.':  '。',
		'?':  '？',
		'!':  '！',
		':':  '：',
		';':  '；',
		'(':  '（',
		')':  '）',
		'[':  '【',
		']':  '】',
		'<':  '《',
		'>':  '》',
		'"':  '\u201c',
		'\'':	'\u2019',
		'-':  '—',
		'*':  '＊',
	}
	var b strings.Builder
	b.Grow(len(s) * 3) // CJK chars are 3 bytes each
	for _, r := range s {
		if fw, ok := punctuationMap[r]; ok {
			b.WriteRune(fw)
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// SearchKeywordVariants returns a deduplicated slice of search variants for the
// given keyword that covers common punctuation and case differences:
//
//  1. Original keyword
//  2. Lowercase / uppercase of original
//  3. Half-width (ASCII) normalised form
//  4. Lowercase / uppercase of half-width form
//  5. Full-width (CJK) form of half-width variant
//  6. Lowercase / uppercase of full-width form
//
// This ensures that a query like "好," also matches indexed text "好，" and vice
// versa, without any change to the index mapping.
func SearchKeywordVariants(keyword string) []string {
	halfWidth := NormalizeHalfWidthPunctuation(keyword)
	fullWidth := fullWidthPunctuationVariant(halfWidth)

	candidates := []string{
		keyword,
		strings.ToLower(keyword),
		strings.ToUpper(keyword),
		halfWidth,
		strings.ToLower(halfWidth),
		strings.ToUpper(halfWidth),
		fullWidth,
		strings.ToLower(fullWidth),
		strings.ToUpper(fullWidth),
	}

	seen := make(map[string]bool, len(candidates))
	result := make([]string, 0, len(candidates))
	for _, v := range candidates {
		if v == "" || seen[v] {
			continue
		}
		seen[v] = true
		result = append(result, v)
	}
	return result
}

// SingleCharVariants returns the set of single-rune variants for a single-
// character keyword (for use in term/terms queries on keyword fields). It
// applies the same punctuation + case normalisation as SearchKeywordVariants
// but filters out any multi-rune strings that may arise from the mapping.
func SingleCharVariants(keyword string) []interface{} {
	variants := SearchKeywordVariants(keyword)
	seen := make(map[interface{}]bool)
	var result []interface{}

	for _, v := range variants {
		runes := []rune(v)
		if len(runes) != 1 {
			continue
		}
		lower := string(unicode.ToLower(runes[0]))
		upper := string(unicode.ToUpper(runes[0]))
		for _, candidate := range []string{v, lower, upper} {
			if !seen[candidate] {
				seen[candidate] = true
				result = append(result, candidate)
			}
		}
	}
	return result
}

// ----- Date helpers --------------------------------------------------------

// SearchDate represents a parsed date suffix stripped from the search keyword.
// Exactly one of (FullDate, MonthDayOnly) is populated.
type SearchDate struct {
	// FullDate is set when a complete YYYY-MM-DD date was parsed.
	FullDate *DateRange
	// MonthDayOnly is set when only MM-DD was parsed (all years, that month+day).
	MonthDayOnly *MonthDay
}

// DateRange is a half-open Unix-second interval [Start, End).
type DateRange struct {
	Start int64
	End   int64
}

// MonthDay represents a calendar month and day (1-indexed).
type MonthDay struct {
	Month int
	Day   int
}

// shanghaLoc is cached after the first successful load.
var shanghaiLoc *time.Location

func getShanghaiLoc() *time.Location {
	if shanghaiLoc != nil {
		return shanghaiLoc
	}
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		return time.UTC
	}
	shanghaiLoc = loc
	return shanghaiLoc
}

// yearMonthDayRE matches a trailing date with a full year: YYYY[sep]M[sep]D
// where sep can be /  - .  or 年/月 (the final 日 is optional).
var yearMonthDayRE = regexp.MustCompile(
	`\s+(\d{4})[年/.\-](\d{1,2})[月/.\-](\d{1,2})日?\s*$`,
)

// monthDayRE matches a trailing month-day without year: M-D or MM-DD.
// We deliberately require a hyphen to avoid false positives on things like
// ratio keywords (50-50).  Slash format (6/23) is intentionally excluded for
// month-day-only to prevent ambiguity with numeric ratios.
var monthDayRE = regexp.MustCompile(
	`\s+(\d{1,2})-(\d{1,2})\s*$`,
)

// ExtractTrailingDate strips a trailing date suffix from keyword and returns
// the clean keyword together with the parsed date (or nil if no date found).
// The function does not strip if the resulting keyword would be empty.
func ExtractTrailingDate(keyword string) (cleanKeyword string, date *SearchDate) {
	// Try full year+month+day first.
	if loc := yearMonthDayRE.FindStringIndex(keyword); loc != nil {
		m := yearMonthDayRE.FindStringSubmatch(keyword)
		year, _ := strconv.Atoi(m[1])
		month, _ := strconv.Atoi(m[2])
		day, _ := strconv.Atoi(m[3])

		clean := strings.TrimSpace(keyword[:loc[0]])
		if clean == "" {
			return keyword, nil
		}

		tz := getShanghaiLoc()
		t := time.Date(year, time.Month(month), day, 0, 0, 0, 0, tz)
		// Validate (time.Date normalises invalid dates, check round-trip)
		if t.Year() != year || int(t.Month()) != month || t.Day() != day {
			return keyword, nil
		}
		return clean, &SearchDate{
			FullDate: &DateRange{
				Start: t.Unix(),
				End:   t.AddDate(0, 0, 1).Unix(),
			},
		}
	}

	// Try month-day only.
	if loc := monthDayRE.FindStringIndex(keyword); loc != nil {
		m := monthDayRE.FindStringSubmatch(keyword)
		month, _ := strconv.Atoi(m[1])
		day, _ := strconv.Atoi(m[2])

		// Range-validate to filter out things like "50-50".
		if month < 1 || month > 12 || day < 1 || day > 31 {
			return keyword, nil
		}

		// Extra sanity: the day must actually exist in that month.
		// Use a leap year (2000) to be permissive for Feb 29.
		probe := time.Date(2000, time.Month(month), day, 0, 0, 0, 0, time.UTC)
		if int(probe.Month()) != month || probe.Day() != day {
			return keyword, nil
		}

		clean := strings.TrimSpace(keyword[:loc[0]])
		if clean == "" {
			return keyword, nil
		}
		return clean, &SearchDate{MonthDayOnly: &MonthDay{Month: month, Day: day}}
	}

	return keyword, nil
}

// TimestampRangeFilter builds an ES range filter clause for a full-date
// DateRange. It does NOT affect relevance scoring when used inside bool.filter.
func TimestampRangeFilter(dr *DateRange) map[string]interface{} {
	return map[string]interface{}{
		"range": map[string]interface{}{
			"timestamp": map[string]interface{}{
				"gte": dr.Start,
				"lt":  dr.End,
			},
		},
	}
}

// TimestampMonthDayFilter builds an ES script filter that matches documents
// whose timestamp falls on the given month+day in Asia/Shanghai, across all
// years. Script filters bypass relevance scoring but are cached by ES.
func TimestampMonthDayFilter(md *MonthDay) map[string]interface{} {
	return map[string]interface{}{
		"script": map[string]interface{}{
			"script": map[string]interface{}{
				"source": `
					def zdt = Instant.ofEpochSecond(doc['timestamp'].value)
					                 .atZone(ZoneId.of('Asia/Shanghai'));
					return zdt.getMonthValue() == params.month
					    && zdt.getDayOfMonth() == params.day;
				`,
				"params": map[string]interface{}{
					"month": md.Month,
					"day":   md.Day,
				},
			},
		},
	}
}

// BuildDateFilter converts a SearchDate into an ES filter clause (range or
// script). Returns nil when sd is nil.
func BuildDateFilter(sd *SearchDate) map[string]interface{} {
	if sd == nil {
		return nil
	}
	if sd.FullDate != nil {
		return TimestampRangeFilter(sd.FullDate)
	}
	return TimestampMonthDayFilter(sd.MonthDayOnly)
}
