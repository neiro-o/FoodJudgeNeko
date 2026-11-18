package handlers

import (
	"encoding/json"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
)

func TestBuildProblemLink(t *testing.T) {
	tests := []struct {
		name    string
		item    bson.M
		reply   string
		task    string
		wantErr bool
	}{
		{name: "first reply only", task: "TASK &?#中文", reply: "reply+&?#", item: bson.M{
			"detail": bson.M{"taskInfo": bson.M{"voteTaskNo": "TASK &?#中文"}},
			"comment": bson.A{
				bson.M{"pageContent": bson.A{bson.M{"commentId": "reply+&?#"}, bson.M{"commentId": "other"}}},
				bson.M{"pageContent": bson.A{bson.M{"commentId": "another page"}}},
			},
		}},
		{name: "missing comments", task: "TASK", item: bson.M{"detail": bson.M{"taskInfo": bson.M{"voteTaskNo": "TASK"}}}},
		{name: "empty first page", task: "TASK", item: bson.M{
			"detail":  bson.M{"taskInfo": bson.M{"voteTaskNo": "TASK"}},
			"comment": bson.A{bson.M{"pageContent": bson.A{}}, bson.M{"pageContent": bson.A{bson.M{"commentId": "must not use"}}}},
		}},
		{name: "numeric reply", task: "TASK", reply: "1754799712775200769", item: bson.M{
			"detail":  bson.M{"taskInfo": bson.M{"voteTaskNo": "TASK"}},
			"comment": bson.A{bson.M{"pageContent": bson.A{bson.M{"commentId": int64(1754799712775200769)}}}},
		}},
		{name: "malformed optional comments", task: "TASK", item: bson.M{"detail": bson.M{"taskInfo": bson.M{"voteTaskNo": "TASK"}}, "comment": "legacy"}},
		{name: "missing detail", wantErr: true, item: bson.M{}},
		{name: "missing task", wantErr: true, item: bson.M{"detail": bson.M{"taskInfo": bson.M{}}}},
		{name: "empty task", wantErr: true, item: bson.M{"detail": bson.M{"taskInfo": bson.M{"voteTaskNo": ""}}}},
		{name: "null task", wantErr: true, item: bson.M{"detail": bson.M{"taskInfo": bson.M{"voteTaskNo": nil}}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			raw, err := bson.Marshal(tt.item)
			if err != nil {
				t.Fatal(err)
			}
			link, err := buildProblemLink(raw)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr = %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			parsed, err := url.Parse(link)
			if err != nil {
				t.Fatal(err)
			}
			// Keep complete URL templates out of test source as well.
			if parsed.Scheme != "https" || parsed.Host != strings.Join([]string{"zqt", "meituan", "com"}, ".") || parsed.Path != "/xiaomei/static/inbox-task-detail" {
				t.Fatalf("unexpected URL target: %v", parsed)
			}
			query := parsed.Query()
			if query.Get("targetReplyId") != tt.reply || query.Get("voteTaskNo") != tt.task {
				t.Fatalf("unexpected link parameters: %v", query)
			}
			for _, key := range []string{"bizId", "gameTaskId", "login", "originChannel", "xmeiss"} {
				if !query.Has(key) || query.Get(key) != "" {
					t.Fatalf("missing empty parameter %q", key)
				}
			}
			if query.Get("channel") != "openwebview" || query.Get("fromPage") != "inbox-message" {
				t.Fatalf("unexpected fixed parameters: %v", query)
			}
		})
	}
}

func TestGetProblemLinkRejectsInvalidID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/api/problem/link/:id", GetProblemLink)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest("GET", "/api/problem/link/invalid", nil))
	var response struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Code != 400 || response.Message == "" {
		t.Fatalf("unexpected response: %s", recorder.Body)
	}
	if recorder.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("link response must not be cached")
	}
}
