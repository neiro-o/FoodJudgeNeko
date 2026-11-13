package stats

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type fakeSource struct {
	comments              []Comment
	blocked               map[string]bool
	maliciousErr, scanErr error
	scans                 atomic.Int32
}

func (s *fakeSource) MaliciousUserIDs(context.Context) (map[string]bool, error) {
	return s.blocked, s.maliciousErr
}
func (s *fakeSource) Scan(ctx context.Context, from, to int64, visit func(Comment) error) error {
	s.scans.Add(1)
	for _, c := range s.comments {
		if err := visit(c); err != nil {
			return err
		}
	}
	return s.scanErr
}

type splitTokenizer struct{}

func (splitTokenizer) Words(s string) []string { return strings.Fields(s) }
func fixtureSource(now time.Time) *fakeSource {
	end := now.Unix()
	return &fakeSource{blocked: map[string]bool{"blocked": true}, comments: []Comment{
		{ID: "day-start", Content: "外卖 外卖 退款", CreateTime: end - 86400, Likes: 3},
		{ID: "day-end", Content: "外卖", CreateTime: end - 1, Likes: 8, Name: "hidden", IsAnonymous: true},
		{ID: "week-start", Content: "外卖", CreateTime: end - 604800, Likes: 100},
		{ID: "too-old", Content: "outside", CreateTime: end - 604801},
		{ID: "future", Content: "outside", CreateTime: end},
		{ID: "bad-time", Content: "outside", CreateTime: 0},
		{ID: "blank", Content: " \n ", CreateTime: end - 1},
		{ID: "evil", Content: "outside", CreateTime: end - 1, UserID: "blocked"},
		{ID: "other", Content: "外卖员", CreateTime: end - 1, Likes: 1},
	}}
}
func TestBuildWindowsAndExactMembership(t *testing.T) {
	now := time.Unix(1800000000, 0)
	source := fixtureSource(now)
	snapshot, err := Build(context.Background(), source, splitTokenizer{}, now)
	if err != nil {
		t.Fatal(err)
	}
	if source.scans.Load() != 1 {
		t.Fatal("both windows must use one scan")
	}
	day, week := snapshot.Cloud("24h"), snapshot.Cloud("7d")
	if day.CommentCount != 3 || week.CommentCount != 4 {
		t.Fatalf("counts: day=%d week=%d", day.CommentCount, week.CommentCount)
	}
	if day.Words[0].Text != "外卖" || day.Words[0].Count != 2 || week.Words[0].Count != 3 {
		t.Fatalf("unexpected words: %+v %+v", day.Words, week.Words)
	}
	result, err := snapshot.Page("24h", "外卖", 1)
	if err != nil {
		t.Fatal(err)
	}
	if result.Total != 2 || result.Comments[0].ID != "day-end" || result.Comments[1].ID != "day-start" {
		t.Fatalf("wrong membership/order: %+v", result)
	}
	if result.Comments[0].Name != "" {
		t.Fatal("anonymous name leaked")
	}
	raw, _ := json.Marshal(snapshot)
	if strings.Contains(string(raw), "hidden") || strings.Contains(string(raw), "userId") {
		t.Fatalf("private fields leaked: %s", raw)
	}
	if _, err = snapshot.Page("24h", "外", 1); !errors.Is(err, ErrWord) {
		t.Fatal("substring must not match")
	}
}
func TestTopWordsPreviewsAndPaging(t *testing.T) {
	now := time.Unix(1800000000, 0)
	source := &fakeSource{}
	// Same likes/time force the comment ID tie breaker; repeated words still count once.
	for i := 44; i >= 0; i-- {
		source.comments = append(source.comments, Comment{ID: fmt.Sprintf("%03d", i), Content: "common common", Likes: 10, CreateTime: now.Unix() - 1})
	}
	for i := 0; i < 120; i++ {
		source.comments = append(source.comments, Comment{ID: fmt.Sprintf("rare%03d", i), Content: fmt.Sprintf("word%03d", i), CreateTime: now.Unix() - 1})
	}
	snapshot, err := Build(context.Background(), source, splitTokenizer{}, now)
	if err != nil {
		t.Fatal(err)
	}
	words := snapshot.Cloud("24h").Words
	if len(words) != 100 || words[0].Count != 45 || len(words[0].Preview) != 3 {
		t.Fatalf("top/preview: %+v", words[0])
	}
	for page, count := range map[int]int{1: 20, 2: 20, 3: 5} {
		result, err := snapshot.Page("24h", "common", page)
		if err != nil {
			t.Fatal(err)
		}
		if len(result.Comments) != count || result.Total != 45 || result.TotalPages != 3 {
			t.Fatalf("wrong page: %+v", result)
		}
		if result.Comments[0].ID != fmt.Sprintf("%03d", (page-1)*20) {
			t.Fatal("unstable order")
		}
	}
	for _, page := range []int{0, 4, int(^uint(0) >> 1)} {
		if _, err = snapshot.Page("24h", "common", page); err == nil {
			t.Fatal("expected invalid page")
		}
	}
	if _, err = snapshot.Page("24h", "word119", 1); !errors.Is(err, ErrWord) {
		t.Fatal("unpublished words cannot be paged")
	}
}
func TestBuildFailureDoesNotReturnPartialSnapshot(t *testing.T) {
	now := time.Now()
	for _, stage := range []string{"malicious", "cursor"} {
		source := fixtureSource(now)
		if stage == "malicious" {
			source.maliciousErr = errors.New("blocked read failed")
		} else {
			source.scanErr = errors.New("cursor failed")
		}
		snapshot, err := Build(context.Background(), source, splitTokenizer{}, now)
		if err == nil || snapshot != nil {
			t.Fatalf("%s returned partial result", stage)
		}
		if stage == "malicious" && source.scans.Load() != 0 {
			t.Fatal("must fail closed before scanning")
		}
	}
}
func TestDecodeCommentRejectsInvalidContentAndTime(t *testing.T) {
	base := func() bson.M {
		return bson.M{"_id": primitive.NewObjectID(), "content": "外卖", "createTime": int32(1790000000), "userId": "123", "approveCount": int32(5), "isAnonymous": true, "userName": "private"}
	}
	for _, value := range []any{nil, "1790000000", 1790000000.5, bson.M{}} {
		row := base()
		row["createTime"] = value
		if _, ok := decodeComment(row); ok {
			t.Fatalf("accepted invalid time %v", value)
		}
	}
	row := base()
	row["content"] = bson.A{"text"}
	if _, ok := decodeComment(row); ok {
		t.Fatal("accepted non-text content")
	}
	c, ok := decodeComment(base())
	if !ok || c.CreateTime != 1790000000 || c.Likes != 5 {
		t.Fatalf("invalid decode: %+v", c)
	}
}
func TestRealTokenizer(t *testing.T) {
	tokenizer, err := NewTokenizer()
	if err != nil {
		t.Fatal(err)
	}
	words := tokenizer.Words("美团 外卖 外卖 差评 食品安全 APP app 123 你 的 因为 但是 https://example.com/退款 www.example.com/秘密 !!!")
	got := map[string]int{}
	for _, word := range words {
		got[word]++
	}
	for _, word := range []string{"美团", "外卖", "差评", "食品安全", "app"} {
		if got[word] != 1 {
			t.Fatalf("expected %s once, got %v", word, words)
		}
	}
	for _, word := range []string{"123", "你", "的", "因为", "但是", "退款", "秘密", "example", "com"} {
		if got[word] != 0 {
			t.Fatalf("noise %s: %v", word, words)
		}
	}
}
func newCacheService(t *testing.T) (*Service, *miniredis.Miniredis, *fakeSource, *time.Time) {
	t.Helper()
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { client.Close() })
	now := time.Unix(1800000000, 0)
	source := fixtureSource(now)
	service := &Service{Redis: client, Source: source, Tokenizer: splitTokenizer{}, Now: func() time.Time { return now }}
	return service, server, source, &now
}
func TestCacheCoalescesConcurrentRequests(t *testing.T) {
	service, _, source, _ := newCacheService(t)
	var wg sync.WaitGroup
	ids := make(chan string, 12)
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			result, err := service.Latest(context.Background())
			if err != nil {
				t.Error(err)
				return
			}
			ids <- result.ID
		}()
	}
	wg.Wait()
	close(ids)
	id := ""
	for value := range ids {
		if id != "" && id != value {
			t.Fatal("different snapshots")
		}
		id = value
	}
	if source.scans.Load() != 1 {
		t.Fatalf("expected one scan, got %d", source.scans.Load())
	}
}
func TestCacheFreshnessRetentionAndFailedRefresh(t *testing.T) {
	service, server, source, now := newCacheService(t)
	ctx := context.Background()
	first, err := service.Latest(ctx)
	if err != nil {
		t.Fatal(err)
	}
	*now = now.Add(FreshFor - time.Second)
	server.FastForward(FreshFor - time.Second)
	cached, err := service.Latest(ctx)
	if err != nil || cached.ID != first.ID {
		t.Fatal("cache expired too early", err)
	}
	*now = now.Add(time.Second)
	server.FastForward(time.Second)
	source.maliciousErr = errors.New("read denied")
	if _, err = service.Latest(ctx); err == nil {
		t.Fatal("refresh must fail closed")
	}
	if _, err = service.Snapshot(ctx, first.ID); err != nil {
		t.Fatal("old snapshot should survive failed refresh", err)
	}
	source.maliciousErr = nil
	second, err := service.Latest(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if second.ID == first.ID {
		t.Fatal("expected new snapshot after 10 minutes")
	}
	if _, err = service.Snapshot(ctx, first.ID); err != nil {
		t.Fatal("old snapshot lost before 20 minutes")
	}
	*now = now.Add(FreshFor)
	server.FastForward(FreshFor)
	if _, err = service.Snapshot(ctx, first.ID); !errors.Is(err, ErrExpired) {
		t.Fatal("expected expiration at 20 minutes", err)
	}
	if _, err = service.Snapshot(ctx, second.ID); err != nil {
		t.Fatal("newer snapshot expired prematurely", err)
	}
}
func TestCacheRejectsLostLockAndKeepsNoPartialResult(t *testing.T) {
	service, _, _, _ := newCacheService(t)
	result, err := service.refresh(context.Background(), "not-the-owner")
	if err == nil || result != nil {
		t.Fatal("published snapshot without lock ownership")
	}
	if service.Redis.Exists(context.Background(), cachePrefix+"current").Val() != 0 {
		t.Fatal("partial pointer was published")
	}
}
