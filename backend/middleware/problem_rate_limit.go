package middleware

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"mtv2/backend/database"
	"mtv2/backend/utils"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

const (
	problemRateLimitPrefix  = "problem_rl:v1"
	problemCooldownSeconds  = int64(60)
	problemStrikeTTLSeconds = int64(7 * 24 * 60 * 60)
	problemBlockSeconds     = int64(30 * 24 * 60 * 60)
)

var problemRateLimitScript = redis.NewScript(`
local events_key = KEYS[1]
local strikes_key = KEYS[2]
local cooldown_key = KEYS[3]
local blocked_key = KEYS[4]

local weight = tonumber(ARGV[1])
local nonce = ARGV[2]
local cooldown_seconds = tonumber(ARGV[3])
local strike_ttl_seconds = tonumber(ARGV[4])
local block_seconds = tonumber(ARGV[5])

local function retry_after(key, fallback)
    local ttl = redis.call('PTTL', key)
    if ttl < 1 then
        return fallback
    end
    return math.floor((ttl + 999) / 1000)
end

if redis.call('EXISTS', blocked_key) == 1 then
    local strikes = tonumber(redis.call('GET', strikes_key) or '3')
    return {'blocked', retry_after(blocked_key, block_seconds), 0, 0, 0, strikes}
end

if redis.call('EXISTS', cooldown_key) == 1 then
    local strikes = tonumber(redis.call('GET', strikes_key) or '0')
    return {'cooldown', retry_after(cooldown_key, cooldown_seconds), 0, 0, 0, strikes}
end

local redis_time = redis.call('TIME')
local now_ms = tonumber(redis_time[1]) * 1000 + math.floor(tonumber(redis_time[2]) / 1000)
redis.call('ZREMRANGEBYSCORE', events_key, '-inf', now_ms - 600000)

for point = 1, weight do
    redis.call('ZADD', events_key, now_ms, tostring(now_ms) .. ':' .. nonce .. ':' .. tostring(point))
end
redis.call('EXPIRE', events_key, 601)

local count_10s = tonumber(redis.call('ZCOUNT', events_key, now_ms - 10000, now_ms))
local count_60s = tonumber(redis.call('ZCOUNT', events_key, now_ms - 60000, now_ms))
local count_600s = tonumber(redis.call('ZCOUNT', events_key, now_ms - 600000, now_ms))

if count_10s > 20 or count_60s > 60 or count_600s > 240 then
    local strikes = tonumber(redis.call('INCR', strikes_key))
    redis.call('DEL', events_key)

    if strikes >= 3 then
        redis.call('SET', blocked_key, '1', 'EX', block_seconds)
        redis.call('EXPIRE', strikes_key, block_seconds)
        redis.call('DEL', cooldown_key)
        return {'blocked', block_seconds, count_10s, count_60s, count_600s, strikes}
    end

    redis.call('EXPIRE', strikes_key, strike_ttl_seconds)
    redis.call('SET', cooldown_key, '1', 'EX', cooldown_seconds)
    return {'limited', cooldown_seconds, count_10s, count_60s, count_600s, strikes}
end

local strikes = tonumber(redis.call('GET', strikes_key) or '0')
return {'allowed', 0, count_10s, count_60s, count_600s, strikes}
`)

type problemRateLimitDecision struct {
	Status   string
	Retry    int64
	Count10  int64
	Count60  int64
	Count600 int64
	Strikes  int64
}

type problemRateLimitChecker interface {
	Check(ctx context.Context, userID string, weight int) (problemRateLimitDecision, error)
}

type redisProblemRateLimitChecker struct {
	client redis.UniversalClient
	prefix string
}

func (r redisProblemRateLimitChecker) Check(ctx context.Context, userID string, weight int) (problemRateLimitDecision, error) {
	if r.client == nil {
		return problemRateLimitDecision{}, errors.New("redis client is not configured")
	}
	if weight <= 0 {
		return problemRateLimitDecision{}, fmt.Errorf("invalid problem rate-limit weight: %d", weight)
	}

	nonceBytes := make([]byte, 12)
	if _, err := rand.Read(nonceBytes); err != nil {
		return problemRateLimitDecision{}, fmt.Errorf("generate rate-limit event nonce: %w", err)
	}

	base := r.prefix + ":"
	keys := []string{
		base + "events:" + userID,
		base + "strikes:" + userID,
		base + "cooldown:" + userID,
		base + "blocked:" + userID,
	}
	result, err := problemRateLimitScript.Run(ctx, r.client, keys,
		weight,
		hex.EncodeToString(nonceBytes),
		problemCooldownSeconds,
		problemStrikeTTLSeconds,
		problemBlockSeconds,
	).Result()
	if err != nil {
		return problemRateLimitDecision{}, err
	}

	values, ok := result.([]interface{})
	if !ok || len(values) != 6 {
		return problemRateLimitDecision{}, fmt.Errorf("unexpected rate-limit script result: %T", result)
	}

	status, ok := values[0].(string)
	if !ok {
		return problemRateLimitDecision{}, fmt.Errorf("unexpected rate-limit status: %T", values[0])
	}
	numbers := make([]int64, 5)
	for i := 1; i < len(values); i++ {
		value, err := redisResultInt64(values[i])
		if err != nil {
			return problemRateLimitDecision{}, fmt.Errorf("parse rate-limit result %d: %w", i, err)
		}
		numbers[i-1] = value
	}

	return problemRateLimitDecision{
		Status:   status,
		Retry:    numbers[0],
		Count10:  numbers[1],
		Count60:  numbers[2],
		Count600: numbers[3],
		Strikes:  numbers[4],
	}, nil
}

func redisResultInt64(value interface{}) (int64, error) {
	switch typed := value.(type) {
	case int64:
		return typed, nil
	case string:
		return strconv.ParseInt(typed, 10, 64)
	case []byte:
		return strconv.ParseInt(string(typed), 10, 64)
	default:
		return 0, fmt.Errorf("unexpected integer type %T", value)
	}
}

// ProblemRateLimit protects the problem browsing endpoints with a shared,
// account-scoped rate limit. The caller's authenticated user_id is always used;
// query parameters such as accountId do not affect the limiter identity.
func ProblemRateLimit(route string, weight int) gin.HandlerFunc {
	return problemRateLimitWithChecker(route, weight, redisProblemRateLimitChecker{
		client: database.RedisClient,
		prefix: problemRateLimitPrefix,
	})
}

func problemRateLimitWithChecker(route string, weight int, checker problemRateLimitChecker) gin.HandlerFunc {
	return func(c *gin.Context) {
		if isAdmin, exists := c.Get(utils.CtxKeyIsAdmin); exists {
			if value, ok := isAdmin.(bool); ok && value {
				c.Next()
				return
			}
		}

		userID, exists := utils.GetUserID(c)
		if !exists {
			c.AbortWithStatusJSON(http.StatusUnauthorized, utils.Response{
				Code:    http.StatusUnauthorized,
				Message: "User not authenticated",
			})
			return
		}

		ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
		defer cancel()
		decision, err := checker.Check(ctx, userID, weight)
		if err != nil {
			log.Printf("problem rate limit redis_error user_id=%s route=%s weight=%d error=%v", userID, route, weight, err)
			c.AbortWithStatusJSON(http.StatusServiceUnavailable, utils.Response{
				Code:    http.StatusServiceUnavailable,
				Message: "服务暂时不可用",
			})
			return
		}

		switch decision.Status {
		case "allowed":
			c.Next()
			return
		case "limited", "cooldown", "blocked":
			retry := decision.Retry
			if retry < 1 {
				retry = 1
			}
			c.Header("Retry-After", strconv.FormatInt(retry, 10))
			log.Printf(
				"problem rate limit user_id=%s route=%s weight=%d count_10s=%d count_60s=%d count_600s=%d strikes=%d result=%s",
				userID, route, weight, decision.Count10, decision.Count60, decision.Count600, decision.Strikes, decision.Status,
			)
			c.AbortWithStatusJSON(http.StatusTooManyRequests, utils.Response{
				Code:    http.StatusTooManyRequests,
				Message: "请求过于频繁，请稍后再试",
			})
			return
		default:
			log.Printf("problem rate limit invalid_result user_id=%s route=%s weight=%d status=%q", userID, route, weight, decision.Status)
			c.AbortWithStatusJSON(http.StatusServiceUnavailable, utils.Response{
				Code:    http.StatusServiceUnavailable,
				Message: "服务暂时不可用",
			})
		}
	}
}
