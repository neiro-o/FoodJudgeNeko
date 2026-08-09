package middleware

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"mtv2/backend/utils"

	"github.com/gin-gonic/gin"
)

type fakeProblemRateLimitChecker struct {
	mu       sync.Mutex
	decision problemRateLimitDecision
	err      error
	calls    int
	userID   string
	weight   int
}

func (f *fakeProblemRateLimitChecker) Check(_ context.Context, userID string, weight int) (problemRateLimitDecision, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	f.userID = userID
	f.weight = weight
	return f.decision, f.err
}

func runProblemRateLimitMiddleware(
	t *testing.T,
	checker problemRateLimitChecker,
	userID string,
	isAdmin bool,
	weight int,
	query string,
) (*httptest.ResponseRecorder, bool) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	handlerCalled := false
	router := gin.New()
	router.GET("/test",
		func(c *gin.Context) {
			c.Set("user_id", userID)
			c.Set(utils.CtxKeyIsAdmin, isAdmin)
			c.Next()
		},
		problemRateLimitWithChecker("/api/problem/search", weight, checker),
		func(c *gin.Context) {
			handlerCalled = true
			c.Status(http.StatusNoContent)
		},
	)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/test"+query, nil)
	router.ServeHTTP(recorder, request)
	return recorder, handlerCalled
}

func decodeRateLimitResponse(t *testing.T, recorder *httptest.ResponseRecorder) utils.Response {
	t.Helper()
	var response utils.Response
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return response
}

func TestProblemRateLimitAllowsRequestAndUsesAuthenticatedAccount(t *testing.T) {
	checker := &fakeProblemRateLimitChecker{decision: problemRateLimitDecision{Status: "allowed"}}
	recorder, called := runProblemRateLimitMiddleware(t, checker, "account-from-jwt", false, 4, "?accountId=another-account")

	if recorder.Code != http.StatusNoContent || !called {
		t.Fatalf("expected handler to run with 204, got status=%d called=%v", recorder.Code, called)
	}
	if checker.calls != 1 || checker.userID != "account-from-jwt" || checker.weight != 4 {
		t.Fatalf("unexpected checker call: calls=%d userID=%q weight=%d", checker.calls, checker.userID, checker.weight)
	}
}

func TestProblemRateLimitAdminBypassesRedis(t *testing.T) {
	checker := &fakeProblemRateLimitChecker{err: errors.New("must not be called")}
	recorder, called := runProblemRateLimitMiddleware(t, checker, "admin-account", true, 4, "")

	if recorder.Code != http.StatusNoContent || !called {
		t.Fatalf("expected admin handler to run with 204, got status=%d called=%v", recorder.Code, called)
	}
	if checker.calls != 0 {
		t.Fatalf("expected no Redis check for admin, got %d calls", checker.calls)
	}
}

func TestProblemRateLimitReturnsReal429(t *testing.T) {
	for _, status := range []string{"limited", "cooldown", "blocked"} {
		t.Run(status, func(t *testing.T) {
			checker := &fakeProblemRateLimitChecker{decision: problemRateLimitDecision{
				Status:  status,
				Retry:   57,
				Count10: 21,
				Strikes: 1,
			}}
			recorder, called := runProblemRateLimitMiddleware(t, checker, "limited-account", false, 1, "")

			if recorder.Code != http.StatusTooManyRequests || called {
				t.Fatalf("expected 429 without handler, got status=%d called=%v", recorder.Code, called)
			}
			if got := recorder.Header().Get("Retry-After"); got != "57" {
				t.Fatalf("expected Retry-After 57, got %q", got)
			}
			response := decodeRateLimitResponse(t, recorder)
			if response.Code != http.StatusTooManyRequests || response.Message != "请求过于频繁，请稍后再试" {
				t.Fatalf("unexpected response: %+v", response)
			}
		})
	}
}

func TestProblemRateLimitFailsClosedWhenRedisFails(t *testing.T) {
	checker := &fakeProblemRateLimitChecker{err: errors.New("redis unavailable")}
	recorder, called := runProblemRateLimitMiddleware(t, checker, "account", false, 3, "")

	if recorder.Code != http.StatusServiceUnavailable || called {
		t.Fatalf("expected 503 without handler, got status=%d called=%v", recorder.Code, called)
	}
	response := decodeRateLimitResponse(t, recorder)
	if response.Code != http.StatusServiceUnavailable || response.Message != "服务暂时不可用" {
		t.Fatalf("unexpected response: %+v", response)
	}
}

func TestRedisResultInt64(t *testing.T) {
	for _, testCase := range []struct {
		name  string
		value interface{}
		want  int64
	}{
		{name: "integer", value: int64(42), want: 42},
		{name: "string", value: "43", want: 43},
		{name: "bytes", value: []byte("44"), want: 44},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			got, err := redisResultInt64(testCase.value)
			if err != nil || got != testCase.want {
				t.Fatalf("redisResultInt64(%T) = %d, %v; want %d", testCase.value, got, err, testCase.want)
			}
		})
	}
}
