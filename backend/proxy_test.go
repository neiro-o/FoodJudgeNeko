package main

import (
	"github.com/gin-gonic/gin"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

type closeRecorder struct{ *httptest.ResponseRecorder }

func (c *closeRecorder) CloseNotify() <-chan bool { return make(chan bool) }

func TestProxyRouting(t *testing.T) {
	for _, tc := range []struct {
		path, method string
		want         bool
	}{
		{"/api/login", "POST", true}, {"/api/problem/search", "GET", false}, {"/api/user_detail/ai_summary", "POST", false}, {"/api/weekly_scores/ranking", "GET", true}, {"/api/problem/quota", "GET", true}, {"/api/problem/upload", "OPTIONS", false}, {"/health", "POST", false},
	} {
		if got := shouldProxy(tc.path, tc.method, nil, nil); got != tc.want {
			t.Errorf("%s %s = %v", tc.method, tc.path, got)
		}
	}
	if !shouldProxy("/api/custom/x", "GET", []string{"/api/custom/"}, []string{}) {
		t.Fatal("custom route not proxied")
	}
}
func TestProxyForwardsAndRejectsSpoofedClient(t *testing.T) {
	seen := make(chan *http.Request, 1)
	bodies := make(chan string, 1)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		bodies <- string(b)
		seen <- r.Clone(r.Context())
		w.WriteHeader(201)
	}))
	defer upstream.Close()
	u, _ := url.Parse(upstream.URL)
	gin.SetMode(gin.TestMode)
	g := gin.New()
	if err := g.SetTrustedProxies([]string{"127.0.0.1", "::1"}); err != nil {
		t.Fatal(err)
	}
	g.Use(primaryProxy(u, nil, nil, []string{"127.0.0.1", "::1"}))
	g.POST("/api/login", func(c *gin.Context) { c.Status(500) })
	req := httptest.NewRequest("POST", "http://site.example/api/login?q=yes", strings.NewReader("payload"))
	req.Host = "site.example"
	req.RemoteAddr = "203.0.113.4:4567"
	req.Header.Set("X-Forwarded-For", "198.51.100.9")
	req.Header.Set("X-Forwarded-Proto", "https")
	w := &closeRecorder{httptest.NewRecorder()}
	g.ServeHTTP(w, req)
	if w.Code != 201 {
		t.Fatalf("status %d", w.Code)
	}
	r := <-seen
	if r.Host != "site.example" || r.URL.RawQuery != "q=yes" || <-bodies != "payload" || r.Header.Get("X-Forwarded-For") != "203.0.113.4" || r.Header.Get("X-Forwarded-Proto") != "http" {
		t.Fatal("proxy did not preserve request or rejected spoofing incorrectly")
	}
	req = httptest.NewRequest("POST", "http://site.example/api/login", nil)
	req.RemoteAddr = "127.0.0.1:4567"
	req.Header.Set("X-Forwarded-For", "198.51.100.9")
	req.Header.Set("X-Forwarded-Proto", "https")
	w = &closeRecorder{httptest.NewRecorder()}
	g.ServeHTTP(w, req)
	r = <-seen
	<-bodies
	if r.Header.Get("X-Forwarded-For") != "198.51.100.9" || r.Header.Get("X-Forwarded-Proto") != "https" {
		t.Fatal("trusted forwarded headers not preserved")
	}
}

func TestBridgeGatewayForwardedHeaders(t *testing.T) {
	seen := make(chan *http.Request, 1)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { seen <- r.Clone(r.Context()); w.WriteHeader(204) }))
	defer upstream.Close()
	u, _ := url.Parse(upstream.URL)
	g := gin.New()
	trusted := []string{"172.30.90.22"}
	if err := g.SetTrustedProxies(trusted); err != nil {
		t.Fatal(err)
	}
	g.Use(primaryProxy(u, nil, nil, trusted))
	g.POST("/api/login", func(c *gin.Context) { c.Status(500) })
	req := httptest.NewRequest("POST", "http://site/api/login", nil)
	req.RemoteAddr = "172.30.90.22:1234"
	req.Header.Set("X-Forwarded-For", "198.51.100.12")
	req.Header.Set("X-Forwarded-Proto", "https")
	w := &closeRecorder{httptest.NewRecorder()}
	g.ServeHTTP(w, req)
	r := <-seen
	if w.Code != 204 || r.Header.Get("X-Forwarded-For") != "198.51.100.12" || r.Header.Get("X-Forwarded-Proto") != "https" {
		t.Fatal("bridge gateway forwarded headers not trusted")
	}
	if trustedProxy("172.30.90.21", trusted) || !trustedProxy("172.30.90.22", trusted) {
		t.Fatal("trusted gateway scope incorrect")
	}
}
