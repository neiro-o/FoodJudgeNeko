package utils

// BuildBoolQuery constructs the Elasticsearch bool query for keyword search.
//
// Design goals (n-gram index, min_gram=2, max_gram=3, lowercase filter):
//
//  1. Exact phrase match ("不好吃" in one review) must score HIGHER than a
//     review where the phrase appears multiple times.  BM25 TF causes the
//     multi-occurrence document to win, so we use constant_score + filter for
//     the high-priority tiers to eliminate term-frequency influence entirely.
//
//  2. Full punctuation / case tolerance: we expand the keyword into all
//     half-width / full-width / case variants via SearchKeywordVariants and
//     run each tier across all variants, taking the best score with dis_max.
//
//  3. Fuzzy tier (T4) is kept as a regular scored clause so that typo matches
//     earn a lower but real score — they are ranked last but still returned.
//
// Tier table (user_review field, boost multiplied by fieldBoost per field):
//
//	T0  constant 500 – term on keyword subfield (review == keyword exactly)
//	T1  constant 100 – match_phrase exact
//	T2  constant  40 – match_phrase slop=2
//	T3  constant  15 – match operator=and (all n-gram tokens present)
//	T4  scored     5 – match fuzzy fuzziness=1, prefix_length=1 (typo / punc)
//
// An optional dateFilter clause is injected into bool.filter (does not affect
// scoring).
func BuildBoolQuery(keyword string, fuzziness int, dateFilter map[string]interface{}) map[string]interface{} {
	variants := SearchKeywordVariants(keyword)

	effectiveFuzziness := fuzziness
	if effectiveFuzziness > 1 {
		effectiveFuzziness = 1
	}
	if len([]rune(keyword)) <= 1 {
		effectiveFuzziness = 0
	}

	// buildTierQueries returns the T0-T4 sub-queries for one field.
	buildTierQueries := func(fieldName string, fieldBoost float64) []map[string]interface{} {
		var qs []map[string]interface{}

		// T0: exact whole-field match (term on .keyword subfield, if available).
		// Only user_review has a .keyword sub-field; for other fields we skip.
		if fieldName == "user_review" {
			termValues := make([]interface{}, len(variants))
			for i, v := range variants {
				termValues[i] = v
			}
			qs = append(qs, map[string]interface{}{
				"constant_score": map[string]interface{}{
					"filter": map[string]interface{}{
						"terms": map[string]interface{}{
							"user_review.keyword": termValues,
						},
					},
					"boost": 500.0 * fieldBoost,
				},
			})
		}

		// T1: match_phrase exact — one clause per variant, take best.
		var t1Clauses []map[string]interface{}
		for _, v := range variants {
			t1Clauses = append(t1Clauses, map[string]interface{}{
				"match_phrase": map[string]interface{}{
					fieldName: map[string]interface{}{
						"query": v,
					},
				},
			})
		}
		qs = append(qs, map[string]interface{}{
			"constant_score": map[string]interface{}{
				"filter": map[string]interface{}{
					"bool": map[string]interface{}{
						"should":               t1Clauses,
						"minimum_should_match": 1,
					},
				},
				"boost": 100.0 * fieldBoost,
			},
		})

		// T2: match_phrase with slop=2.
		var t2Clauses []map[string]interface{}
		for _, v := range variants {
			t2Clauses = append(t2Clauses, map[string]interface{}{
				"match_phrase": map[string]interface{}{
					fieldName: map[string]interface{}{
						"query": v,
						"slop":  2,
					},
				},
			})
		}
		qs = append(qs, map[string]interface{}{
			"constant_score": map[string]interface{}{
				"filter": map[string]interface{}{
					"bool": map[string]interface{}{
						"should":               t2Clauses,
						"minimum_should_match": 1,
					},
				},
				"boost": 40.0 * fieldBoost,
			},
		})

		// T3: match operator=and (all n-gram tokens must appear).
		var t3Clauses []map[string]interface{}
		for _, v := range variants {
			t3Clauses = append(t3Clauses, map[string]interface{}{
				"match": map[string]interface{}{
					fieldName: map[string]interface{}{
						"query":    v,
						"operator": "and",
					},
				},
			})
		}
		qs = append(qs, map[string]interface{}{
			"constant_score": map[string]interface{}{
				"filter": map[string]interface{}{
					"bool": map[string]interface{}{
						"should":               t3Clauses,
						"minimum_should_match": 1,
					},
				},
				"boost": 15.0 * fieldBoost,
			},
		})

		// T4: fuzzy — regular scoring so typo matches rank below exact, but still
		// score enough to clear min_score.
		if effectiveFuzziness > 0 {
			var t4Clauses []map[string]interface{}
			for _, v := range variants {
				t4Clauses = append(t4Clauses, map[string]interface{}{
					"match": map[string]interface{}{
						fieldName: map[string]interface{}{
							"query":         v,
							"fuzziness":     effectiveFuzziness,
							"prefix_length": 1,
							"operator":      "and",
							"boost":         5.0 * fieldBoost,
						},
					},
				})
			}
			qs = append(qs, map[string]interface{}{
				"dis_max": map[string]interface{}{
					"queries":     t4Clauses,
					"tie_breaker": 0.0,
				},
			})
		}

		return qs
	}

	// buildFieldQuery wraps the tier queries in a bool/should for a flat field.
	buildFieldQuery := func(fieldName string, fieldBoost float64) map[string]interface{} {
		tierQs := buildTierQueries(fieldName, fieldBoost)
		return map[string]interface{}{
			"dis_max": map[string]interface{}{
				"queries":     tierQs,
				"tie_breaker": 0.0,
			},
		}
	}

	// buildNestedQuery wraps the tier queries inside a nested query.
	buildNestedQuery := func(path, fieldName string, fieldBoost float64) map[string]interface{} {
		tierQs := buildTierQueries(fieldName, fieldBoost)
		return map[string]interface{}{
			"nested": map[string]interface{}{
				"path": path,
				"query": map[string]interface{}{
					"dis_max": map[string]interface{}{
						"queries":     tierQs,
						"tie_breaker": 0.0,
					},
				},
				"score_mode": "max",
			},
		}
	}

	// The document must match at least one required field.
	requiredFieldQueries := []map[string]interface{}{
		buildFieldQuery("user_review", 1.0),
		buildFieldQuery("others", 0.15),
		buildNestedQuery("replies", "replies.content", 0.35),
		buildNestedQuery("appeals", "appeals.content", 0.5),
	}

	boolMust := []map[string]interface{}{
		{
			"dis_max": map[string]interface{}{
				"queries":     requiredFieldQueries,
				"tie_breaker": 0.0,
			},
		},
	}

	boolQuery := map[string]interface{}{
		"must": boolMust,
	}

	// Inject date filter if provided (does not affect scoring).
	if dateFilter != nil {
		boolQuery["filter"] = []map[string]interface{}{dateFilter}
	}

	return boolQuery
}
