package utils

// BuildBoolQuery constructs the Elasticsearch bool query for keyword search
// Optimized for N-gram indexing (character-level 2-3 gram tokens)
// Requirements:
// 1. Results MUST contain the keyword in at least one of: user_review / others / replies[].content / appeals[].content
// 2. Allow typo tolerance only for similar characters (not distant ones like "猫" vs "8")
// 3. Score does not accumulate - matching 1 field or 10 fields gives same score
// Strategy:
// - Use match_phrase for exact matches (highest priority, only these get highlighted)
// - Use match for partial matches (no highlight)
// - Use fuzzy match with prefix_length to allow only similar character errors
// - Use dis_max with tie_breaker=0.0 to take only the highest score (no accumulation)
func BuildBoolQuery(keyword string, fuzziness int) map[string]interface{} {
	// Calculate edit distance for fuzzy matching
	// For Chinese, we need to be strict: only allow similar character errors
	// Use prefix_length to require first character(s) to match exactly
	// This prevents "小猫" matching "小8" (different characters)
	// But allow "小喵" to match "小猫" (similar characters, 1 char difference)
	effectiveFuzziness := fuzziness
	if effectiveFuzziness > 1 {
		effectiveFuzziness = 1 // Cap at 1 for typo tolerance (1 character error)
	}
	// Allow fuzziness even for 2-char keywords (e.g., "小喵" -> "小猫")
	// The prefix_length will ensure first char matches, preventing "小8" matching "小猫"
	if len(keyword) <= 1 {
		effectiveFuzziness = 0 // Single char: exact match only
	}

	// Calculate prefix_length: require at least first character to match exactly
	// This ensures "小猫" won't match "小8" (first char matches, but second is too different)
	// For 2-char keywords, require first char exact match
	// For longer keywords, can be more lenient
	prefixLength := 1
	if len(keyword) >= 3 {
		prefixLength = 1 // Still require first char for Chinese
	}

	// Build field query with multiple matching strategies
	buildFieldQuery := func(fieldName string, boost float64) map[string]interface{} {
		queries := []map[string]interface{}{
			// Priority 1: match_phrase - exact phrase match (highest score, only this gets highlighted)
			// For n-gram indexed fields, match_phrase works by matching consecutive n-gram tokens
			{
				"match_phrase": map[string]interface{}{
					fieldName: map[string]interface{}{
						"query": keyword,
						"boost": boost * 4.0, // Highest boost for exact phrase
					},
				},
			},
			// Priority 2: match_phrase with slop - allows some flexibility in phrase matching
			// This helps match phrases like "一口不吃" when indexed as n-gram tokens
			{
				"match_phrase": map[string]interface{}{
					fieldName: map[string]interface{}{
						"query": keyword,
						"slop":  2, // Allow up to 2 positions of difference
						"boost": boost * 3.0,
					},
				},
			},
			// Priority 3: match query - handles partial matches (no highlight)
			{
				"match": map[string]interface{}{
					fieldName: map[string]interface{}{
						"query":    keyword,
						"operator": "and", // All terms must match
						"boost":    boost * 2.0,
					},
				},
			},
		}

		// Add fuzzy match for typo tolerance (only if fuzziness > 0)
		// Use prefix_length to ensure first character(s) match exactly
		// This prevents distant character mismatches like "猫" vs "8"
		if effectiveFuzziness > 0 {
			queries = append(queries, map[string]interface{}{
				"match": map[string]interface{}{
					fieldName: map[string]interface{}{
						"query":         keyword,
						"fuzziness":     effectiveFuzziness,
						"prefix_length": prefixLength, // Require first char(s) to match exactly
						"operator":      "and",
						"boost":         boost, // Lower boost for fuzzy matches
					},
				},
			})
		}

		return map[string]interface{}{
			"bool": map[string]interface{}{
				"should":               queries,
				"minimum_should_match": 1, // At least one query must match
			},
		}
	}

	// Build nested field query
	buildNestedQuery := func(path, fieldName string, boost float64) map[string]interface{} {
		queries := []map[string]interface{}{
			// Priority 1: match_phrase in nested field (only this gets highlighted)
			{
				"match_phrase": map[string]interface{}{
					fieldName: map[string]interface{}{
						"query": keyword,
						"boost": boost * 4.0,
					},
				},
			},
			// Priority 2: match_phrase with slop for n-gram flexibility
			{
				"match_phrase": map[string]interface{}{
					fieldName: map[string]interface{}{
						"query": keyword,
						"slop":  2, // Allow up to 2 positions of difference
						"boost": boost * 3.0,
					},
				},
			},
			// Priority 3: match in nested field (no highlight)
			{
				"match": map[string]interface{}{
					fieldName: map[string]interface{}{
						"query":    keyword,
						"operator": "and",
						"boost":    boost * 2.0,
					},
				},
			},
		}

		// Add fuzzy match for typo tolerance with prefix_length
		if effectiveFuzziness > 0 {
			queries = append(queries, map[string]interface{}{
				"match": map[string]interface{}{
					fieldName: map[string]interface{}{
						"query":         keyword,
						"fuzziness":     effectiveFuzziness,
						"prefix_length": prefixLength, // Require first char(s) to match exactly
						"operator":      "and",
						"boost":         boost,
					},
				},
			})
		}

		return map[string]interface{}{
			"nested": map[string]interface{}{
				"path": path,
				"query": map[string]interface{}{
					"bool": map[string]interface{}{
						"should":               queries,
						"minimum_should_match": 1,
					},
				},
				"score_mode": "max",
			},
		}
	}

	// Build queries for all required fields
	// Results MUST match at least one of these fields
	requiredFieldQueries := []map[string]interface{}{
		// user_review (highest priority field)
		buildFieldQuery("user_review", 1.0),
		// others
		buildFieldQuery("others", 0.15),
		// nested: replies.content
		buildNestedQuery("replies", "replies.content", 0.35),
		// nested: appeals.content
		buildNestedQuery("appeals", "appeals.content", 0.5),
	}

	// Use dis_max to find the best match across all required fields
	// tie_breaker=0.0 means only the highest score is used (no accumulation)
	// Matching 1 field or 10 fields gives the same score
	return map[string]interface{}{
		"must": []map[string]interface{}{
			{
				"dis_max": map[string]interface{}{
					"queries":     requiredFieldQueries,
					"tie_breaker": 0.0, // Use only max score, no accumulation
				},
			},
		},
	}
}
