package middleware

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

func redisIntegrationChecker(t *testing.T) (redisProblemRateLimitChecker, func()) {
	t.Helper()
	addr := os.Getenv("MTV2_REDIS_TEST_ADDR")
	if addr == "" {
		t.Skip("set MTV2_REDIS_TEST_ADDR to run Redis integration tests")
	}

	client := redis.NewClient(&redis.Options{Addr: addr})
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		client.Close()
		t.Fatalf("connect to test Redis at %s: %v", addr, err)
	}

	prefix := fmt.Sprintf("problem_rl:test:%d", time.Now().UnixNano())
	checker := redisProblemRateLimitChecker{client: client, prefix: prefix}
	cleanup := func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		var cursor uint64
		for {
			keys, next, err := client.Scan(ctx, cursor, prefix+":*", 100).Result()
			if err != nil {
				break
			}
			if len(keys) > 0 {
				_ = client.Del(ctx, keys...).Err()
			}
			cursor = next
			if cursor == 0 {
				break
			}
		}
		_ = client.Close()
	}
	return checker, cleanup
}

func checkRepeatedly(t *testing.T, checker redisProblemRateLimitChecker, userID string, weight, count int) problemRateLimitDecision {
	t.Helper()
	var decision problemRateLimitDecision
	for i := 0; i < count; i++ {
		var err error
		decision, err = checker.Check(context.Background(), userID, weight)
		if err != nil {
			t.Fatalf("rate-limit check %d: %v", i+1, err)
		}
	}
	return decision
}

func TestProblemRateLimitRedisStrictBoundaries(t *testing.T) {
	checker, cleanup := redisIntegrationChecker(t)
	defer cleanup()

	for _, testCase := range []struct {
		name        string
		userID      string
		weight      int
		allowedRuns int
	}{
		{name: "search", userID: "search-boundary", weight: 4, allowedRuns: 5},
		{name: "recent", userID: "recent-boundary", weight: 3, allowedRuns: 6},
		{name: "detail", userID: "detail-boundary", weight: 1, allowedRuns: 20},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			decision := checkRepeatedly(t, checker, testCase.userID, testCase.weight, testCase.allowedRuns)
			if decision.Status != "allowed" {
				t.Fatalf("expected exact threshold to remain allowed, got %+v", decision)
			}
			decision = checkRepeatedly(t, checker, testCase.userID, testCase.weight, 1)
			if decision.Status != "limited" || decision.Strikes != 1 {
				t.Fatalf("expected next request to trigger strike 1, got %+v", decision)
			}
		})
	}
}

func seedWeightedEvents(t *testing.T, checker redisProblemRateLimitChecker, userID string, count int, age time.Duration) {
	t.Helper()
	ctx := context.Background()
	now, err := checker.client.Time(ctx).Result()
	if err != nil {
		t.Fatalf("read Redis time: %v", err)
	}
	eventsKey := checker.prefix + ":events:" + userID
	entries := make([]redis.Z, 0, count)
	for i := 0; i < count; i++ {
		entries = append(entries, redis.Z{
			Score:  float64(now.Add(-age).UnixMilli()),
			Member: "seed:" + strconv.Itoa(i),
		})
	}
	if err := checker.client.ZAdd(ctx, eventsKey, entries...).Err(); err != nil {
		t.Fatalf("seed weighted events: %v", err)
	}
}

func TestProblemRateLimitRedisLongWindowBoundaries(t *testing.T) {
	checker, cleanup := redisIntegrationChecker(t)
	defer cleanup()

	seedWeightedEvents(t, checker, "minute-boundary", 59, 20*time.Second)
	decision, err := checker.Check(context.Background(), "minute-boundary", 1)
	if err != nil || decision.Status != "allowed" || decision.Count60 != 60 {
		t.Fatalf("expected 60-point minute boundary to be allowed: decision=%+v err=%v", decision, err)
	}
	decision, err = checker.Check(context.Background(), "minute-boundary", 1)
	if err != nil || decision.Status != "limited" || decision.Count60 != 61 {
		t.Fatalf("expected 61st minute point to be limited: decision=%+v err=%v", decision, err)
	}

	seedWeightedEvents(t, checker, "ten-minute-boundary", 239, 2*time.Minute)
	decision, err = checker.Check(context.Background(), "ten-minute-boundary", 1)
	if err != nil || decision.Status != "allowed" || decision.Count600 != 240 {
		t.Fatalf("expected 240-point ten-minute boundary to be allowed: decision=%+v err=%v", decision, err)
	}
	decision, err = checker.Check(context.Background(), "ten-minute-boundary", 1)
	if err != nil || decision.Status != "limited" || decision.Count600 != 241 {
		t.Fatalf("expected 241st ten-minute point to be limited: decision=%+v err=%v", decision, err)
	}
}

func TestProblemRateLimitRedisStateTTLsAndExpiry(t *testing.T) {
	checker, cleanup := redisIntegrationChecker(t)
	defer cleanup()
	ctx := context.Background()
	userID := "state-expiry"

	decision := checkRepeatedly(t, checker, userID, 1, 21)
	if decision.Status != "limited" || decision.Strikes != 1 {
		t.Fatalf("expected first incident, got %+v", decision)
	}
	strikesKey := checker.prefix + ":strikes:" + userID
	cooldownKey := checker.prefix + ":cooldown:" + userID
	strikeTTL, err := checker.client.TTL(ctx, strikesKey).Result()
	if err != nil || strikeTTL <= 6*24*time.Hour || strikeTTL > 7*24*time.Hour {
		t.Fatalf("unexpected strike TTL: ttl=%s err=%v", strikeTTL, err)
	}
	cooldownTTL, err := checker.client.TTL(ctx, cooldownKey).Result()
	if err != nil || cooldownTTL <= 0 || cooldownTTL > time.Minute {
		t.Fatalf("unexpected cooldown TTL: ttl=%s err=%v", cooldownTTL, err)
	}

	if err := checker.client.PExpire(ctx, strikesKey, time.Millisecond).Err(); err != nil {
		t.Fatalf("expire strikes: %v", err)
	}
	if err := checker.client.PExpire(ctx, cooldownKey, time.Millisecond).Err(); err != nil {
		t.Fatalf("expire cooldown: %v", err)
	}
	time.Sleep(10 * time.Millisecond)
	decision, err = checker.Check(ctx, userID, 1)
	if err != nil || decision.Status != "allowed" || decision.Strikes != 0 {
		t.Fatalf("expected expired strike state to reset: decision=%+v err=%v", decision, err)
	}
}

func TestProblemRateLimitRedisThreeIndependentIncidents(t *testing.T) {
	checker, cleanup := redisIntegrationChecker(t)
	defer cleanup()
	userID := "three-incidents"
	cooldownKey := checker.prefix + ":cooldown:" + userID

	for strike := int64(1); strike <= 3; strike++ {
		decision := checkRepeatedly(t, checker, userID, 1, 21)
		wantStatus := "limited"
		if strike == 3 {
			wantStatus = "blocked"
		}
		if decision.Status != wantStatus || decision.Strikes != strike {
			t.Fatalf("strike %d: got %+v, want status %s", strike, decision, wantStatus)
		}
		if strike < 3 {
			if err := checker.client.Del(context.Background(), cooldownKey).Err(); err != nil {
				t.Fatalf("simulate cooldown expiry: %v", err)
			}
		}
	}

	decision, err := checker.Check(context.Background(), userID, 1)
	if err != nil || decision.Status != "blocked" || decision.Strikes != 3 {
		t.Fatalf("blocked request changed state: decision=%+v err=%v", decision, err)
	}
	blockTTL, err := checker.client.TTL(context.Background(), checker.prefix+":blocked:"+userID).Result()
	if err != nil || blockTTL <= 29*24*time.Hour || blockTTL > 30*24*time.Hour {
		t.Fatalf("unexpected block TTL: ttl=%s err=%v", blockTTL, err)
	}
}

func TestProblemRateLimitRedisConcurrentBurstCreatesOneStrike(t *testing.T) {
	checker, cleanup := redisIntegrationChecker(t)
	defer cleanup()
	userID := "concurrent-burst"

	const requests = 80
	decisions := make(chan problemRateLimitDecision, requests)
	errors := make(chan error, requests)
	var wg sync.WaitGroup
	for i := 0; i < requests; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			decision, err := checker.Check(context.Background(), userID, 1)
			if err != nil {
				errors <- err
				return
			}
			decisions <- decision
		}()
	}
	wg.Wait()
	close(decisions)
	close(errors)
	for err := range errors {
		t.Fatalf("concurrent check: %v", err)
	}

	limited := 0
	for decision := range decisions {
		if decision.Status == "limited" {
			limited++
		}
		if decision.Strikes > 1 {
			t.Fatalf("single burst produced multiple strikes: %+v", decision)
		}
	}
	if limited != 1 {
		t.Fatalf("expected exactly one request to create the incident, got %d", limited)
	}
}
