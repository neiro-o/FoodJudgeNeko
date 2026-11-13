// Package stats builds immutable, shared comment word-cloud snapshots.
package stats

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

const FreshFor = 10 * time.Minute
const RetainFor = 20 * time.Minute
const PageSize = 20
const TopWords = 100

var ErrExpired = errors.New("word cloud snapshot expired")
var ErrWord = errors.New("word not in snapshot")

// UserID is used only while filtering and is never returned or cached. Anonymous
// names are cleared before a comment is added to a snapshot.
type Comment struct {
	ID          string `json:"id"`
	Content     string `json:"content"`
	CreateTime  int64  `json:"createTime"`
	Likes       int64  `json:"likes"`
	Name        string `json:"name"`
	IsAnonymous bool   `json:"isAnonymous"`
	UserID      string `json:"-"`
}
type Word struct {
	Text    string    `json:"text"`
	Count   int       `json:"count"`
	Preview []Comment `json:"preview"`
}
type Window struct {
	Range        string `json:"range"`
	From         int64  `json:"from"`
	To           int64  `json:"to"`
	CommentCount int    `json:"commentCount"`
	Words        []Word `json:"words"`
	// Indices refer to the snapshot's sorted comments, never substring searches.
	Matches map[string][]int `json:"matches"`
}
type Snapshot struct {
	ID          string             `json:"id"`
	GeneratedAt int64              `json:"generatedAt"`
	Windows     map[string]*Window `json:"windows"`
	Comments    []Comment          `json:"comments"`
}
type CloudResponse struct {
	Snapshot     string `json:"snapshot"`
	GeneratedAt  int64  `json:"generatedAt"`
	ExpiresAt    int64  `json:"expiresAt"`
	Range        string `json:"range"`
	From         int64  `json:"from"`
	To           int64  `json:"to"`
	CommentCount int    `json:"commentCount"`
	Words        []Word `json:"words"`
}
type CommentsResponse struct {
	Snapshot   string    `json:"snapshot"`
	Word       string    `json:"word"`
	Comments   []Comment `json:"comments"`
	Total      int       `json:"total"`
	Page       int       `json:"page"`
	TotalPages int       `json:"totalPages"`
}
type Source interface {
	MaliciousUserIDs(context.Context) (map[string]bool, error)
	Scan(context.Context, int64, int64, func(Comment) error) error
}
type WordTokenizer interface{ Words(string) []string }

func ValidRange(r string) bool { return r == "24h" || r == "7d" }

func Build(ctx context.Context, source Source, tokenizer WordTokenizer, now time.Time) (*Snapshot, error) {
	blocked, err := source.MaliciousUserIDs(ctx)
	if err != nil {
		return nil, fmt.Errorf("read malicious users: %w", err)
	}
	end := now.Unix()
	s := &Snapshot{ID: primitive.NewObjectID().Hex(), GeneratedAt: end, Windows: map[string]*Window{}, Comments: []Comment{}}
	for r, seconds := range map[string]int64{"24h": 86400, "7d": 604800} {
		s.Windows[r] = &Window{Range: r, From: end - seconds, To: end, Words: []Word{}, Matches: map[string][]int{}}
	}
	// Retain token sets only during generation, in parallel with the comments.
	tokens := map[string][]string{}
	seen := map[string]bool{}
	err = source.Scan(ctx, end-604800, end, func(c Comment) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if c.ID == "" || seen[c.ID] || c.CreateTime < end-604800 || c.CreateTime >= end || c.CreateTime <= 0 || strings.TrimSpace(c.Content) == "" || blocked[c.UserID] {
			return nil
		}
		seen[c.ID] = true
		if c.IsAnonymous {
			c.Name = ""
		}
		c.UserID = ""
		s.Comments = append(s.Comments, c)
		tokens[c.ID] = tokenizer.Words(c.Content)
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(s.Comments, func(i, j int) bool {
		a, b := s.Comments[i], s.Comments[j]
		if a.Likes != b.Likes {
			return a.Likes > b.Likes
		}
		if a.CreateTime != b.CreateTime {
			return a.CreateTime > b.CreateTime
		}
		return a.ID < b.ID
	})
	for i, c := range s.Comments {
		for _, w := range s.Windows {
			if c.CreateTime < w.From {
				continue
			}
			w.CommentCount++
			unique := map[string]bool{}
			for _, word := range tokens[c.ID] {
				if word != "" && !unique[word] {
					w.Matches[word] = append(w.Matches[word], i)
					unique[word] = true
				}
			}
		}
	}
	for _, w := range s.Windows {
		for text, ids := range w.Matches {
			w.Words = append(w.Words, Word{Text: text, Count: len(ids)})
		}
		sort.Slice(w.Words, func(i, j int) bool {
			if w.Words[i].Count != w.Words[j].Count {
				return w.Words[i].Count > w.Words[j].Count
			}
			return w.Words[i].Text < w.Words[j].Text
		})
		if len(w.Words) > TopWords {
			w.Words = w.Words[:TopWords]
		}
		matches := map[string][]int{}
		for i := range w.Words {
			word := &w.Words[i]
			ids := w.Matches[word.Text]
			matches[word.Text] = ids
			word.Preview = []Comment{}
			for _, id := range ids[:min(3, len(ids))] {
				word.Preview = append(word.Preview, s.Comments[id])
			}
		}
		w.Matches = matches
	}
	return s, nil
}

func (s *Snapshot) Cloud(r string) CloudResponse {
	w := s.Windows[r]
	return CloudResponse{Snapshot: s.ID, GeneratedAt: s.GeneratedAt, ExpiresAt: s.GeneratedAt + int64(RetainFor/time.Second), Range: r, From: w.From, To: w.To, CommentCount: w.CommentCount, Words: w.Words}
}
func (s *Snapshot) Page(r, word string, page int) (CommentsResponse, error) {
	w, ok := s.Windows[r]
	if !ok {
		return CommentsResponse{}, ErrWord
	}
	ids, ok := w.Matches[word]
	if !ok {
		return CommentsResponse{}, ErrWord
	}
	total := len(ids)
	pages := (total + PageSize - 1) / PageSize
	if page < 1 || page > max(1, pages) {
		return CommentsResponse{}, fmt.Errorf("page out of range")
	}
	result := CommentsResponse{Snapshot: s.ID, Word: word, Total: total, Page: page, TotalPages: pages, Comments: []Comment{}}
	start := (page - 1) * PageSize
	for _, id := range ids[start:min(start+PageSize, total)] {
		result.Comments = append(result.Comments, s.Comments[id])
	}
	return result, nil
}
