package main

import (
	"fmt"
	"math"
	"sort"
	"strings"
)

// EmbeddingStore manages embeddings and retrieval
type EmbeddingStore struct {
	// Maps chunk_id -> vector representation (for now, just term frequencies)
	vectors map[string]map[string]float64
}

// NewEmbeddingStore creates a new embedding store
func NewEmbeddingStore() *EmbeddingStore {
	return &EmbeddingStore{
		vectors: make(map[string]map[string]float64),
	}
}

// AddChunk embeds and stores a chunk
func (s *EmbeddingStore) AddChunk(chunkID string, text string) {
	vector := s.TFVector(text)
	s.vectors[chunkID] = vector
}

// TFVector creates a simple term frequency vector from text
func (s *EmbeddingStore) TFVector(text string) map[string]float64 {
	words := s.Tokenize(text)
	vector := make(map[string]float64)

	for _, word := range words {
		vector[word]++
	}

	// Normalize
	totalWords := float64(len(words))
	for key := range vector {
		vector[key] = vector[key] / totalWords
	}

	return vector
}

// Tokenize breaks text into lowercase words
func (s *EmbeddingStore) Tokenize(text string) []string {
	// Simple tokenization: lowercase and split on non-alphanumeric
	text = strings.ToLower(text)
	words := strings.FieldsFunc(text, func(r rune) bool {
		return !((r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'))
	})

	// Filter stop words
	stopWords := map[string]bool{
		"the": true, "a": true, "an": true, "and": true, "or": true,
		"in": true, "on": true, "at": true, "to": true, "for": true,
		"is": true, "are": true, "was": true, "be": true, "been": true,
		"have": true, "has": true, "do": true, "does": true, "did": true,
		"of": true, "with": true, "by": true, "from": true, "as": true,
		"if": true, "about": true, "into": true, "through": true, "during": true,
		"it": true, "its": true, "that": true, "this": true, "which": true,
		"who": true, "what": true, "where": true, "when": true, "why": true,
	}

	var filtered []string
	for _, word := range words {
		if len(word) > 2 && !stopWords[word] {
			filtered = append(filtered, word)
		}
	}

	return filtered
}

// CosineSimilarity computes cosine similarity between two vectors
func (s *EmbeddingStore) CosineSimilarity(vec1, vec2 map[string]float64) float64 {
	dotProduct := 0.0
	magnitude1 := 0.0
	magnitude2 := 0.0

	// Compute dot product and magnitude for vec2
	for word, freq2 := range vec2 {
		magnitude2 += freq2 * freq2
		if freq1, exists := vec1[word]; exists {
			dotProduct += freq1 * freq2
		}
	}

	// Compute magnitude for vec1
	for _, freq1 := range vec1 {
		magnitude1 += freq1 * freq1
	}

	magnitude1 = math.Sqrt(magnitude1)
	magnitude2 = math.Sqrt(magnitude2)

	if magnitude1 == 0 || magnitude2 == 0 {
		return 0
	}

	return dotProduct / (magnitude1 * magnitude2)
}

// RetrievalResult represents a single chunk result
type RetrievalResult struct {
	ChunkID  string
	Text     string
	ParentID string
	Score    float64
}

// SearchTopK retrieves the top-k most similar chunks for a query
func (s *EmbeddingStore) SearchTopK(query string, chunks []map[string]string, k int) []RetrievalResult {
	queryVector := s.TFVector(query)

	var results []RetrievalResult
	for _, chunk := range chunks {
		chunkID := chunk["id"]
		text := chunk["text"]
		parentID := chunk["parent_id"]

		if vector, exists := s.vectors[chunkID]; exists {
			score := s.CosineSimilarity(queryVector, vector)
			results = append(results, RetrievalResult{
				ChunkID:  chunkID,
				Text:     text,
				ParentID: parentID,
				Score:    score,
			})
		}
	}

	// Sort by score descending
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	// Return top-k
	if len(results) > k {
		results = results[:k]
	}

	return results
}

// RetrievalContext holds the retrieved context for RAG
type RetrievalContext struct {
	TopicID   string
	Sections  map[string]string // parent_id -> section content
	ChunkHits int
}

// BuildContext builds context from retrieval results by expanding chunks to parents
func BuildContext(results []RetrievalResult, topicID string) (*RetrievalContext, error) {
	context := &RetrievalContext{
		TopicID:   topicID,
		Sections:  make(map[string]string),
		ChunkHits: len(results),
	}

	seenParents := make(map[string]bool)

	for _, result := range results {
		if !seenParents[result.ParentID] {
			section, err := GetParentSection(result.ParentID)
			if err != nil {
				return nil, err
			}
			heading := section["heading"]
			content := section["content"]
			context.Sections[result.ParentID] = fmt.Sprintf("**%s**\n%s", heading, content)
			seenParents[result.ParentID] = true
		}
	}

	return context, nil
}
