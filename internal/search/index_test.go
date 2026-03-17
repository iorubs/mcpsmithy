package search

import (
	"testing"
)

func TestBuildAndSearch(t *testing.T) {
	chunks := []Chunk{
		{Source: "a.md", Title: "Kubernetes Pods", Body: "Pods are the smallest deployable unit in Kubernetes."},
		{Source: "b.md", Title: "Docker Containers", Body: "Containers package application code with dependencies."},
		{Source: "c.md", Title: "Helm Charts", Body: "Helm manages Kubernetes applications via charts."},
	}
	idx := NewIndex(chunks)
	if idx.Len() != 3 {
		t.Fatalf("expected 3 indexed chunks, got %d", idx.Len())
	}
	results := idx.Search("kubernetes", 10)
	if len(results) == 0 {
		t.Fatal("expected results for 'kubernetes'")
	}
	if results[0].Chunk.Title != "Kubernetes Pods" {
		t.Fatalf("expected 'Kubernetes Pods' first, got %q", results[0].Chunk.Title)
	}
}

func TestSearchNoResults(t *testing.T) {
	idx := NewIndex([]Chunk{
		{Source: "a.md", Title: "Hello", Body: "World"},
	})
	results := idx.Search("nonexistent", 10)
	if len(results) != 0 {
		t.Fatalf("expected no results, got %d", len(results))
	}
}

func TestSearchTagBoost(t *testing.T) {
	chunks := []Chunk{
		{Source: "a.md", Title: "General Doc", Body: "controller reconciliation loop", Tags: []string{}},
		{Source: "b.md", Title: "Tagged Doc", Body: "some content", Tags: []string{"controller"}},
	}
	idx := NewIndex(chunks)
	results := idx.Search("controller", 10)
	if len(results) < 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if results[0].Chunk.Title != "Tagged Doc" {
		t.Fatalf("expected tagged doc first, got %q", results[0].Chunk.Title)
	}
}

func TestSearchTitleBoost(t *testing.T) {
	chunks := []Chunk{
		{Source: "a.md", Title: "Introduction", Body: "This guide covers ingress configuration and routing."},
		{Source: "b.md", Title: "Ingress Configuration", Body: "Details about setup."},
	}
	idx := NewIndex(chunks)
	results := idx.Search("ingress", 10)
	if len(results) < 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if results[0].Chunk.Title != "Ingress Configuration" {
		t.Fatalf("expected 'Ingress Configuration' first, got %q", results[0].Chunk.Title)
	}
}

func TestSearchMaxResults(t *testing.T) {
	var chunks []Chunk
	for range 20 {
		chunks = append(chunks, Chunk{Source: "x.md", Title: "Topic", Body: "keyword content"})
	}
	idx := NewIndex(chunks)
	results := idx.Search("keyword", 5)
	if len(results) != 5 {
		t.Fatalf("expected max 5 results, got %d", len(results))
	}
}

func TestTokenize(t *testing.T) {
	tokens := tokenize("Hello, World! Go-lang 2024")
	// "hello" → stemmed to "hello", "world" → "world", "go-lang" kept, "2024" kept
	expected := []string{"hello", "world", "go-lang", "2024"}
	if len(tokens) != len(expected) {
		t.Fatalf("expected %d tokens, got %d: %v", len(expected), len(tokens), tokens)
	}
	for i, exp := range expected {
		if tokens[i] != exp {
			t.Fatalf("token %d: expected %q, got %q", i, exp, tokens[i])
		}
	}
}

func TestTokenizeStopWords(t *testing.T) {
	tokens := tokenize("the quick brown fox is in the forest")
	// "the", "is", "in" are stop words → removed
	for _, tok := range tokens {
		if stopWords[tok] {
			t.Fatalf("stop word %q should have been removed", tok)
		}
	}
	if len(tokens) != 4 {
		t.Fatalf("expected 4 tokens after stop word removal, got %d: %v", len(tokens), tokens)
	}
}

func TestStem(t *testing.T) {
	tests := []struct {
		input, want string
	}{
		{"controllers", "controll"},
		{"running", "runn"},
		{"testing", "test"},
		{"deployed", "deploy"},
		{"quickly", "quick"},
		{"application", "applic"},
		{"configuration", "configur"},
		{"management", "manage"},
		{"readiness", "readi"},
		{"entries", "entry"},
		{"searchable", "search"},
		{"go", "go"},       // too short to stem
		{"class", "class"}, // "ss" guard
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := stem(tt.input)
			if got != tt.want {
				t.Errorf("stem(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestStemMatchesAcrossQuery(t *testing.T) {
	// "controllers" in body should match "controller" in query.
	chunks := []Chunk{
		{Source: "a.md", Title: "Controllers", Body: "controllers reconcile resources"},
		{Source: "b.md", Title: "Other", Body: "unrelated content here"},
	}
	idx := NewIndex(chunks)
	results := idx.Search("controller", 10)
	if len(results) == 0 {
		t.Fatal("stemming should match 'controller' to 'controllers'")
	}
	if results[0].Chunk.Source != "a.md" {
		t.Fatalf("expected a.md, got %s", results[0].Chunk.Source)
	}
}

func TestTermFrequencyBoost(t *testing.T) {
	// Document with "kubernetes" appearing 3 times should rank higher
	// than one with it appearing once.
	chunks := []Chunk{
		{Source: "once.md", Title: "Once", Body: "kubernetes is mentioned once here with other words"},
		{Source: "many.md", Title: "Many", Body: "kubernetes kubernetes kubernetes core concept"},
	}
	idx := NewIndex(chunks)
	results := idx.Search("kubernetes", 10)
	if len(results) < 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if results[0].Chunk.Source != "many.md" {
		t.Fatalf("expected many.md first (higher tf), got %s", results[0].Chunk.Source)
	}
	if results[0].Score <= results[1].Score {
		t.Fatal("higher tf doc should have higher score")
	}
}

func TestEmptyIndex(t *testing.T) {
	idx := NewIndex(nil)
	if idx.Len() != 0 {
		t.Fatal("empty index should have 0 chunks")
	}
	results := idx.Search("anything", 10)
	if len(results) != 0 {
		t.Fatal("empty index should return no results")
	}
}

func TestMerge(t *testing.T) {
	tests := []struct {
		name      string
		initial   []Chunk
		merge     []Chunk
		wantAdded int
		wantLen   int
		check     func(*testing.T, *Index)
	}{
		{
			name: "merge two chunks",
			initial: []Chunk{
				{Source: "a.md", Title: "Initial", Body: "original content"},
			},
			merge: []Chunk{
				{Source: "b.md", Title: "Second", Body: "additional content"},
				{Source: "c.md", Title: "Third", Body: "more content"},
			},
			wantAdded: 2,
			wantLen:   3,
			check: func(t *testing.T, idx *Index) {
				// Search should find content from both original and merged chunks.
				results := idx.Search("content", 10)
				if len(results) != 3 {
					t.Fatalf("expected 3 results, got %d", len(results))
				}
			},
		},
		{
			name: "merge nil",
			initial: []Chunk{
				{Source: "a.md", Title: "Initial", Body: "content"},
			},
			merge:     nil,
			wantAdded: 0,
			wantLen:   1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			idx := NewIndex(tt.initial)
			added := idx.Merge(tt.merge)
			if added != tt.wantAdded {
				t.Fatalf("expected %d chunks added, got %d", tt.wantAdded, added)
			}
			if idx.Len() != tt.wantLen {
				t.Fatalf("expected %d total chunks, got %d", tt.wantLen, idx.Len())
			}
			if tt.check != nil {
				tt.check(t, idx)
			}
		})
	}
}

func TestMergeConcurrent(t *testing.T) {
	idx := NewIndex(nil)

	// Merge and search concurrently.
	done := make(chan bool)
	go func() {
		for range 100 {
			idx.Merge([]Chunk{
				{Source: "x.md", Title: "Concurrent", Body: "concurrent content"},
			})
		}
		done <- true
	}()
	go func() {
		for range 100 {
			idx.Search("concurrent", 10)
		}
		done <- true
	}()
	<-done
	<-done

	// Should have merged all 100 chunks.
	if idx.Len() != 100 {
		t.Fatalf("expected 100 chunks after concurrent merge, got %d", idx.Len())
	}
}
