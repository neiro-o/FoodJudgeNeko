package middleware

import (
	"context"
	"github.com/gin-gonic/gin"
	"io"
	"mtv2/backend/config"
	"mtv2/backend/utils"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestRemoteAuthentication(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cases := []struct {
		name, body   string
		status, want int
		delay        bool
	}{
		{"valid", `{"code":0,"data":{"user_id":"0123456789abcdef01234567","username":"alice","is_admin":false}}`, 200, 204, false},
		{"unauthorized", "", 401, 401, false}, {"invalid_json", "{}", 200, 503, false},
		{"bad_id", `{"code":0,"data":{"user_id":"bad","username":"alice","is_admin":false}}`, 200, 503, false},
		{"missing_admin", `{"code":0,"data":{"user_id":"0123456789abcdef01234567","username":"alice"}}`, 200, 503, false},
		{"timeout", "", 200, 503, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/api/auth/verify" || r.Header.Get("Authorization") != "Bearer original" {
					t.Error("incorrect upstream request")
				}
				if tc.delay {
					time.Sleep(200 * time.Millisecond)
				}
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer upstream.Close()
			config.AppConfig = &config.Config{}
			config.AppConfig.PrimaryAPIURL = upstream.URL
			r := gin.New()
			r.Use(AuthMiddleware())
			r.GET("/local", func(c *gin.Context) {
				if id, _ := c.Get("user_id"); id != "0123456789abcdef01234567" {
					t.Error("missing authenticated ID")
				}
				if admin, _ := c.Get(utils.CtxKeyIsAdmin); admin != false {
					t.Error("invalid admin context")
				}
				c.Status(204)
			})
			req := httptest.NewRequest("GET", "/local", nil)
			req.Header.Set("Authorization", "Bearer original")
			if tc.delay {
				ctx, cancel := context.WithTimeout(req.Context(), 40*time.Millisecond)
				defer cancel()
				req = req.WithContext(ctx)
			}
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			if w.Code != tc.want {
				t.Fatalf("got %d want %d", w.Code, tc.want)
			}
		})
	}
}

type testTransport func(*http.Request) (*http.Response, error)

func (f testTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestPrivatePrimaryAndPublicRejection(t *testing.T) {
	old := authHTTPClient
	defer func() { authHTTPClient = old }()
	called := false
	authHTTPClient = &http.Client{Transport: testTransport(func(r *http.Request) (*http.Response, error) {
		called = true
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"code":0,"data":{"user_id":"0123456789abcdef01234567","username":"alice","is_admin":false}}`))}, nil
	})}
	for _, tc := range []struct {
		url    string
		want   int
		called bool
	}{{"http://172.30.90.1:18081", 204, true}, {"http://8.8.8.8:18081", 503, false}, {"http://u:p@172.30.90.1:18081", 503, false}} {
		called = false
		config.AppConfig = &config.Config{}
		config.AppConfig.PrimaryAPIURL = tc.url
		r := gin.New()
		r.Use(AuthMiddleware())
		r.GET("/local", func(c *gin.Context) { c.Status(204) })
		req := httptest.NewRequest("GET", "/local", nil)
		req.Header.Set("Authorization", "Bearer original")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != tc.want || called != tc.called {
			t.Errorf("%s status=%d called=%v", tc.url, w.Code, called)
		}
	}
}
