package handlers

import (
	"encoding/json"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson"
)

func TestNormalizeVanishedUserID(t *testing.T) {
	for _, test := range []struct {
		raw  string
		want string
		ok   bool
	}{
		{raw: "123", want: "123", ok: true},
		{raw: " 00123 ", want: "123", ok: true},
		{raw: "0", want: "0", ok: true},
		{raw: "12x", ok: false},
		{raw: "", ok: false},
	} {
		got, ok := normalizeVanishedUserID(test.raw)
		if got != test.want || ok != test.ok {
			t.Fatalf("normalizeVanishedUserID(%q) = %q, %v; want %q, %v", test.raw, got, ok, test.want, test.ok)
		}
	}
}

func TestProblemCommentIDs(t *testing.T) {
	problem := bson.M{"comment": bson.A{
		bson.M{"pageContent": bson.A{bson.M{"commentId": "present"}}},
		bson.M{"pageContent": bson.A{bson.M{"commentId": int64(123)}}},
	}}
	ids := problemCommentIDs(problem)
	if _, ok := ids["present"]; !ok {
		t.Fatal("expected string comment ID")
	}
	if _, ok := ids["123"]; !ok {
		t.Fatal("expected numeric comment ID to be normalized")
	}
}

func TestFormatVanishedTimeSGT(t *testing.T) {
	const seconds int64 = 1_700_000_000
	want := time.Unix(seconds, 0).In(vanishedSGT).Format("2006-01-02 15:04:05-07:00")
	if got := formatVanishedTimeSGT(seconds); got != want {
		t.Fatalf("seconds formatted as %q, want %q", got, want)
	}
	if got := formatVanishedTimeSGT(seconds * 1000); got != want {
		t.Fatalf("milliseconds formatted as %q, want %q", got, want)
	}
}

func TestBuildProblemShareURL(t *testing.T) {
	want := "https://zqt.meituan.com/xiaomei/vote/jury/api/r/rediectByScene?jumpScene=mockTaskShare&userId=user+id&channel=mockTaskShare&encryptMockTaskNo=task%2B%2F%3D"
	if got := buildProblemShareURL("user id", "task+/="); got != want {
		t.Fatalf("URL = %q, want %q", got, want)
	}
}

func TestVanishedESFieldsFromSourceFallsBackToFirstAppeal(t *testing.T) {
	fields := vanishedESFieldsFromSource(map[string]interface{}{
		"user_review": "  ",
		"timestamp":   json.Number("1700000000"),
		"appeals": []interface{}{
			map[string]interface{}{"content": "first appeal"},
			map[string]interface{}{"content": "second appeal"},
		},
	})
	if fields.UserReview != "first appeal" {
		t.Fatalf("UserReview = %q, want first appeal", fields.UserReview)
	}
	if fields.Timestamp != 1_700_000_000 {
		t.Fatalf("Timestamp = %d, want 1700000000", fields.Timestamp)
	}
}
