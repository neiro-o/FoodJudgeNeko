package stats

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// A versioned prefix prevents snapshots built with an older token contract
// from being reused when the dictionary or aggregation rules change.
const cachePrefix = "stats:wordcloud:v1:"

type Service struct {
	Redis  redis.UniversalClient
	Source Source
	// Now and Tokenizer may be supplied by tests. Set them before serving requests.
	Now          func() time.Time
	Tokenizer    WordTokenizer
	once         sync.Once
	tokenizerErr error
}

func (s *Service) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}
func (s *Service) read(ctx context.Context, id string) (*Snapshot, error) {
	raw, err := s.Redis.Get(ctx, cachePrefix+"snapshot:"+id).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, ErrExpired
	}
	if err != nil {
		return nil, err
	}
	var result Snapshot
	if err = json.Unmarshal(raw, &result); err != nil {
		return nil, err
	}
	if result.ID != id || s.now().Unix() >= result.GeneratedAt+int64(RetainFor/time.Second) {
		return nil, ErrExpired
	}
	return &result, nil
}
func (s *Service) Snapshot(ctx context.Context, id string) (*Snapshot, error) { return s.read(ctx, id) }
func (s *Service) current(ctx context.Context) (*Snapshot, error) {
	id, err := s.Redis.Get(ctx, cachePrefix+"current").Result()
	if errors.Is(err, redis.Nil) {
		return nil, ErrExpired
	}
	if err != nil {
		return nil, err
	}
	result, err := s.read(ctx, id)
	if err != nil {
		return nil, err
	}
	if s.now().Unix() >= result.GeneratedAt+int64(FreshFor/time.Second) {
		return nil, ErrExpired
	}
	return result, nil
}

var releaseLock = redis.NewScript(`if redis.call('GET',KEYS[1])==ARGV[1] then return redis.call('DEL',KEYS[1]) end return 0`)
var publishSnapshot = redis.NewScript(`
if redis.call('GET',KEYS[1])~=ARGV[1] then return 0 end
redis.call('SET',KEYS[2],ARGV[2],'PX',ARGV[3])
redis.call('SET',KEYS[3],ARGV[4],'PX',ARGV[5])
redis.call('DEL',KEYS[1])
return 1`)

// Latest coalesces refreshes across API instances with an owner-checked lock.
// The pointer and complete snapshot are published atomically only after success.
func (s *Service) Latest(ctx context.Context) (*Snapshot, error) {
	ctx, cancel := context.WithTimeout(ctx, 70*time.Second)
	defer cancel()
	for {
		current, err := s.current(ctx)
		if err == nil {
			return current, nil
		}
		if !errors.Is(err, ErrExpired) {
			return nil, err
		}
		token := primitive.NewObjectID().Hex()
		acquired, err := s.Redis.SetNX(ctx, cachePrefix+"lock", token, 90*time.Second).Result()
		if err != nil {
			return nil, err
		}
		if acquired {
			return s.refresh(ctx, token)
		}
		timer := time.NewTimer(200 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, ctx.Err()
		case <-timer.C:
		}
	}
}
func (s *Service) refresh(ctx context.Context, token string) (*Snapshot, error) {
	defer func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = releaseLock.Run(cleanup, s.Redis, []string{cachePrefix + "lock"}, token).Err()
	}()
	if current, err := s.current(ctx); err == nil {
		return current, nil
	} else if !errors.Is(err, ErrExpired) {
		return nil, err
	}
	s.once.Do(func() {
		if s.Tokenizer == nil {
			s.Tokenizer, s.tokenizerErr = NewTokenizer()
		}
	})
	if s.tokenizerErr != nil {
		return nil, s.tokenizerErr
	}
	start := s.now()
	buildCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	snapshot, err := Build(buildCtx, s.Source, s.Tokenizer, start)
	if err != nil {
		return nil, err
	}
	raw, err := json.Marshal(snapshot)
	if err != nil {
		return nil, err
	}
	remaining := time.Unix(snapshot.GeneratedAt, 0).Add(RetainFor).Sub(s.now())
	freshRemaining := time.Unix(snapshot.GeneratedAt, 0).Add(FreshFor).Sub(s.now())
	if freshRemaining <= 0 {
		return nil, fmt.Errorf("snapshot generation exceeded freshness window")
	}
	published, err := publishSnapshot.Run(ctx, s.Redis, []string{cachePrefix + "lock", cachePrefix + "snapshot:" + snapshot.ID, cachePrefix + "current"}, token, raw, remaining.Milliseconds(), snapshot.ID, freshRemaining.Milliseconds()).Int()
	if err != nil {
		return nil, err
	}
	if published != 1 {
		return nil, fmt.Errorf("word cloud refresh lock expired")
	}
	log.Printf("word cloud snapshot built: comments=%d words24h=%d words7d=%d elapsed=%s bytes=%d", len(snapshot.Comments), len(snapshot.Windows["24h"].Words), len(snapshot.Windows["7d"].Words), s.now().Sub(start), len(raw))
	return snapshot, nil
}
