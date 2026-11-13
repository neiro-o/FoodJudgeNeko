package handlers

import (
	"encoding/json"
	"github.com/gin-gonic/gin"
	"mtv2/backend/middleware"
	"net/http/httptest"
	"testing"
)

func TestWordCloudRejectsInvalidQueries(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/cloud", GetWordCloud)
	r.GET("/comments", GetWordCloudComments)
	for _, url := range []string{
		"/cloud?range=all", "/cloud?range=", "/cloud?range=24H",
		"/comments?range=7d&snapshot=invalid&word=test",
		"/comments?range=all&snapshot=111111111111111111111111&word=test",
		"/comments?range=24h&snapshot=111111111111111111111111&word=test&page=0",
		"/comments?range=24h&snapshot=111111111111111111111111&word=test&page=nope",
		"/comments?range=24h&snapshot=111111111111111111111111&word=%20",
	} {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest("GET", url, nil))
		var body struct {
			Code int `json:"code"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil || body.Code != 400 {
			t.Fatalf("%s: %s", url, w.Body.String())
		}
		if w.Header().Get("Cache-Control") != "no-store" {
			t.Fatal("stats responses must not be stored by shared HTTP caches")
		}
	}
}
func TestWordCloudRequiresLogin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	protected := r.Group("/api", middleware.AuthMiddleware())
	protected.GET("/stats/wordcloud", GetWordCloud)
	protected.GET("/stats/wordcloud/comments", GetWordCloudComments)
	for _, path := range []string{"/api/stats/wordcloud", "/api/stats/wordcloud/comments"} {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
		if w.Code != 401 {
			t.Fatalf("expected unauthenticated request to be blocked: %d %s", w.Code, w.Body.String())
		}
	}
}
