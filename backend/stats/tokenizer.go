package stats

import (
	"embed"
	"regexp"
	"strings"
	"sync"
	"unicode"
	"unicode/utf8"

	"github.com/go-ego/gse"
)

//go:embed stopwords.txt dictionary.txt
var vocabulary embed.FS

var links = regexp.MustCompile(`(?i)(?:https?://|www\.)[^\s<>]+`)

// Tokenizer owns its dictionary. GSE's HMM uses mutable working state, so callers
// must not share a segmenter concurrently without the mutex.
type Tokenizer struct {
	mu        sync.Mutex
	segmenter gse.Segmenter
	stop      map[string]bool
}

func NewTokenizer() (*Tokenizer, error) {
	seg, err := gse.NewEmbed("zh", "alpha")
	if err != nil {
		return nil, err
	}
	dict, err := vocabulary.ReadFile("dictionary.txt")
	if err != nil {
		return nil, err
	}
	if err = seg.LoadDictStr(string(dict)); err != nil {
		return nil, err
	}
	stop, err := vocabulary.ReadFile("stopwords.txt")
	if err != nil {
		return nil, err
	}
	t := &Tokenizer{segmenter: seg, stop: map[string]bool{}}
	for _, word := range strings.Fields(string(stop)) {
		t.stop[word] = true
	}
	return t, nil
}

// Words returns a set: repeating a word within one comment adds no extra heat.
func (t *Tokenizer) Words(content string) []string {
	content = strings.ToLower(links.ReplaceAllString(content, " "))
	t.mu.Lock()
	words := t.segmenter.Cut(content, true)
	t.mu.Unlock()
	seen := map[string]bool{}
	result := make([]string, 0, len(words))
	for _, word := range words {
		word = strings.TrimSpace(word)
		if utf8.RuneCountInString(word) < 2 || t.stop[word] || seen[word] {
			continue
		}
		hasLetter, valid := false, true
		for _, r := range word {
			if unicode.IsLetter(r) {
				hasLetter = true
			} else if !unicode.IsDigit(r) {
				valid = false
				break
			}
		}
		if valid && hasLetter {
			seen[word] = true
			result = append(result, word)
		}
	}
	return result
}
