package handlers

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
)

// toInt64Value coerces a value coming from a decoded bson document into an int64.
func toInt64Value(v interface{}) int64 {
	switch val := v.(type) {
	case int32:
		return int64(val)
	case int64:
		return val
	case float64:
		return int64(val)
	case string:
		if parsed, err := strconv.ParseInt(val, 10, 64); err == nil {
			return parsed
		}
	}
	return 0
}

// toStringSlice coerces a value coming from a decoded bson document (bson.A,
// []interface{}, []string, or a single string) into a []string.
func toStringSlice(v interface{}) []string {
	if v == nil {
		return []string{}
	}
	switch val := v.(type) {
	case bson.A:
		result := make([]string, 0, len(val))
		for _, item := range val {
			if s, ok := item.(string); ok {
				result = append(result, s)
			}
		}
		return result
	case []interface{}:
		result := make([]string, 0, len(val))
		for _, item := range val {
			if s, ok := item.(string); ok {
				result = append(result, s)
			}
		}
		return result
	case []string:
		return val
	case string:
		if val == "" {
			return []string{}
		}
		return []string{val}
	}
	return []string{}
}

// toCommentAudios parses the "audios" field, which is an array of objects:
// [{"url": "...", "duration": 2, "audioText": "..."}]
func toCommentAudios(v interface{}) []gin.H {
	result := []gin.H{}
	if v == nil {
		return result
	}

	var items []interface{}
	switch val := v.(type) {
	case bson.A:
		items = val
	case []interface{}:
		items = val
	default:
		return result
	}

	for _, item := range items {
		var itemMap map[string]interface{}
		switch m := item.(type) {
		case bson.M:
			itemMap = m
		case map[string]interface{}:
			itemMap = m
		default:
			continue
		}

		audio := gin.H{
			"url":       "",
			"duration":  0,
			"audioText": "",
		}
		if u, ok := itemMap["url"].(string); ok {
			audio["url"] = u
		}
		if d, ok := itemMap["duration"]; ok {
			audio["duration"] = int(toInt64Value(d))
		}
		if t, ok := itemMap["audioText"].(string); ok {
			audio["audioText"] = t
		}
		result = append(result, audio)
	}
	return result
}
