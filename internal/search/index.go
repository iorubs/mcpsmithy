package search

import (
	"cmp"
	"math"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
)

// LiveIndex provides safe atomic access to a swappable Index.
// Create with NewLiveIndex. Safe for concurrent use — Get and Len
// can be called freely alongside Swap.
type LiveIndex struct {
	ptr atomic.Pointer[Index]
}

// NewLiveIndex returns a LiveIndex backed by an empty index.
func NewLiveIndex() *LiveIndex {
	h := &LiveIndex{}
	h.ptr.Store(NewIndex(nil))
	return h
}

// Get returns the current index. Safe for concurrent use.
func (h *LiveIndex) Get() *Index {
	return h.ptr.Load()
}

// Swap atomically replaces the current index with idx.
func (h *LiveIndex) Swap(idx *Index) {
	h.ptr.Store(idx)
}

// Len returns the number of indexed chunks, or 0 if the index is empty.
// Safe for concurrent use.
func (h *LiveIndex) Len() int {
	return h.ptr.Load().Len()
}

// Search runs a query against the current index. Safe for concurrent use.
func (h *LiveIndex) Search(query string, maxResults int) []Result {
	return h.ptr.Load().Search(query, maxResults)
}

// Searcher is implemented by both *Index (static) and *LiveIndex (live/swappable).
// Use this type when callers do not need to distinguish between the two.
type Searcher interface {
	Search(query string, maxResults int) []Result
	Len() int
}

// Result is a scored search result.
type Result struct {
	Chunk Chunk
	Score float64
}

// Index is an in-memory search index over chunks.
// It is safe for concurrent use: Search acquires a read lock,
// Merge acquires a write lock.
type Index struct {
	mu sync.RWMutex

	chunks []Chunk
	// inverted maps stemmed term → list of chunk indices.
	inverted map[string][]int
	// termFreq maps "term:docIdx" → count of that term in the document.
	termFreq map[string]int
	// titleInv maps stemmed term → chunk indices where term appears in title.
	titleInv map[string][]int
	// tagInv maps lowercased tag → chunk indices.
	tagInv map[string][]int
	// docLen stores the token count per chunk.
	docLen []int
	// avgLen is the average document length.
	avgLen float64
	// totalBodyLen is the sum of all document lengths, stored for Merge calculations.
	totalBodyLen int
}

// termFreqKey builds the composite key for the termFreq map.
func termFreqKey(term string, docIdx int) string {
	buf := make([]byte, 0, len(term)+12)
	buf = append(buf, term...)
	buf = append(buf, ':')
	buf = strconv.AppendInt(buf, int64(docIdx), 10)
	return string(buf)
}

// NewIndex creates an index from chunks.
func NewIndex(chunks []Chunk) *Index {
	idx := &Index{
		chunks:   chunks,
		inverted: make(map[string][]int),
		termFreq: make(map[string]int),
		titleInv: make(map[string][]int),
		tagInv:   make(map[string][]int),
		docLen:   make([]int, len(chunks)),
	}

	totalLen := 0
	for i, c := range chunks {
		n := indexChunk(idx, i, c)
		idx.docLen[i] = n
		totalLen += n
	}

	idx.totalBodyLen = totalLen
	if len(chunks) > 0 {
		idx.avgLen = float64(totalLen) / float64(len(chunks))
	}
	return idx
}

// Merge appends new chunks to the index. It is safe for concurrent use
// with Search — callers can merge in additional content while searches
// continue. Returns the number of chunks added.
func (idx *Index) Merge(chunks []Chunk) int {
	if len(chunks) == 0 {
		return 0
	}

	idx.mu.Lock()
	defer idx.mu.Unlock()

	baseIdx := len(idx.chunks)
	idx.chunks = append(idx.chunks, chunks...)
	newDocLen := make([]int, len(chunks))

	addedLen := 0
	for i, c := range chunks {
		docIdx := baseIdx + i
		n := indexChunk(idx, docIdx, c)
		newDocLen[i] = n
		addedLen += n
	}

	idx.docLen = append(idx.docLen, newDocLen...)
	idx.totalBodyLen += addedLen
	if len(idx.chunks) > 0 {
		idx.avgLen = float64(idx.totalBodyLen) / float64(len(idx.chunks))
	}
	return len(chunks)
}

// Search returns the top-N chunks matching the query, ranked by score.
// Scoring: body match (BM25) + title boost (3x) + tag exact match (5x).
// It is safe for concurrent use with Merge.
func (idx *Index) Search(query string, maxResults int) []Result {
	if maxResults <= 0 {
		maxResults = 10
	}

	terms := tokenize(query)
	if len(terms) == 0 {
		return nil
	}

	idx.mu.RLock()
	defer idx.mu.RUnlock()

	scores := make(map[int]float64)
	n := float64(len(idx.chunks))

	// BM25 parameters.
	k1 := 1.2
	b := 0.75

	for _, term := range terms {
		// Body scoring (BM25 with real term frequency).
		if postings, ok := idx.inverted[term]; ok {
			idf := math.Log(1 + (n-float64(len(postings))+0.5)/(float64(len(postings))+0.5))
			for _, docIdx := range postings {
				dl := float64(idx.docLen[docIdx])
				tf := float64(idx.termFreq[termFreqKey(term, docIdx)])
				score := idf * (tf * (k1 + 1)) / (tf + k1*(1-b+b*dl/max(idx.avgLen, 1)))
				scores[docIdx] += score
			}
		}

		// Title boost (3x).
		if postings, ok := idx.titleInv[term]; ok {
			for _, docIdx := range postings {
				scores[docIdx] += 3.0
			}
		}

		// Tag exact match (5x).
		if postings, ok := idx.tagInv[term]; ok {
			for _, docIdx := range postings {
				scores[docIdx] += 5.0
			}
		}
	}

	type entry struct {
		idx   int
		score float64
	}
	var ranked []entry
	for docIdx, score := range scores {
		ranked = append(ranked, entry{docIdx, score})
	}
	slices.SortFunc(ranked, func(a, b entry) int {
		return cmp.Compare(b.score, a.score)
	})

	if len(ranked) > maxResults {
		ranked = ranked[:maxResults]
	}

	results := make([]Result, len(ranked))
	for i, e := range ranked {
		results[i] = Result{Chunk: idx.chunks[e.idx], Score: e.score}
	}
	return results
}

// indexChunk indexes a single chunk at docIdx into idx and returns the body token count.
func indexChunk(idx *Index, docIdx int, c Chunk) int {
	bodyTokens := tokenize(c.Body)

	freq := map[string]int{}
	for _, tok := range bodyTokens {
		freq[tok]++
	}
	for tok, count := range freq {
		idx.inverted[tok] = append(idx.inverted[tok], docIdx)
		idx.termFreq[termFreqKey(tok, docIdx)] = count
	}

	for _, tok := range tokenize(c.Title) {
		idx.titleInv[tok] = append(idx.titleInv[tok], docIdx)
	}

	for _, tag := range c.Tags {
		key := stem(strings.ToLower(tag))
		idx.tagInv[key] = append(idx.tagInv[key], docIdx)
	}

	return len(bodyTokens)
}

// Len returns the number of indexed chunks.
func (idx *Index) Len() int {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	return len(idx.chunks)
}

// stopWords are common English words removed during tokenization to
// prevent them from diluting IDF scores.
var stopWords = map[string]bool{
	"the": true, "is": true, "at": true, "which": true,
	"on": true, "an": true, "in": true, "for": true,
	"to": true, "of": true, "and": true, "or": true,
	"it": true, "be": true, "as": true, "by": true,
	"was": true, "are": true, "with": true, "that": true,
	"this": true, "from": true, "not": true, "but": true,
	"have": true, "has": true, "had": true, "its": true,
	"can": true, "will": true, "do": true, "if": true,
}

// tokenize splits text into lowercase, stemmed tokens with stop words removed.
func tokenize(text string) []string {
	text = strings.ToLower(text)
	var tokens []string
	var current strings.Builder
	for _, r := range text {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' || r == '-' {
			current.WriteRune(r)
		} else {
			if current.Len() > 0 {
				tok := current.String()
				if len(tok) > 1 && !stopWords[tok] {
					tokens = append(tokens, stem(tok))
				}
				current.Reset()
			}
		}
	}
	if current.Len() > 1 {
		tok := current.String()
		if !stopWords[tok] {
			tokens = append(tokens, stem(tok))
		}
	}
	return tokens
}

// stem applies lightweight suffix stripping to normalize word forms.
// Not a full Porter stemmer — just common English suffixes with minimum
// length guards to avoid over-stemming short words.
func stem(word string) string {
	n := len(word)

	// Order matters: try longer suffixes first.
	switch {
	case n > 6 && strings.HasSuffix(word, "ation"):
		return word[:n-5]
	case n > 5 && strings.HasSuffix(word, "tion"):
		return word[:n-4]
	case n > 5 && strings.HasSuffix(word, "ment"):
		return word[:n-4]
	case n > 5 && strings.HasSuffix(word, "ness"):
		return word[:n-4]
	case n > 5 && strings.HasSuffix(word, "ying"):
		return word[:n-4]
	case n > 5 && strings.HasSuffix(word, "ies"):
		return word[:n-3] + "y"
	case n > 5 && strings.HasSuffix(word, "able"):
		return word[:n-4]
	case n > 4 && strings.HasSuffix(word, "ing"):
		return word[:n-3]
	case n > 4 && strings.HasSuffix(word, "ble"):
		return word[:n-3]
	case n > 3 && strings.HasSuffix(word, "ed"):
		return word[:n-2]
	case n > 3 && strings.HasSuffix(word, "ly"):
		return word[:n-2]
	case n > 4 && strings.HasSuffix(word, "ers"):
		return word[:n-3]
	case n > 3 && strings.HasSuffix(word, "er"):
		return word[:n-2]
	case n > 3 && strings.HasSuffix(word, "es"):
		return word[:n-2]
	case n > 3 && strings.HasSuffix(word, "s") && word[n-2] != 's':
		// Strip trailing 's' but not from words ending in "ss" (e.g. "class").
		return word[:n-1]
	}
	return word
}
