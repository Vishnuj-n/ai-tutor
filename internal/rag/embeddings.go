package rag

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"ai-tutor/internal/db"
	"ai-tutor/internal/models"
)

// EmbeddingStore manages embeddings and retrieval
type EmbeddingStore struct {
	vectors map[string]VectorEntry
}

// VectorEntry stores a chunk vector and metadata for retrieval-time filtering/scoring.
type VectorEntry struct {
	Vector          map[string]float64
	ChunkID         string
	TopicID         string
	ParentID        string
	ImportanceScore float64
	WeaknessScore   float64
}

// NewEmbeddingStore creates a new embedding store
func NewEmbeddingStore() *EmbeddingStore {
	return &EmbeddingStore{
		vectors: make(map[string]VectorEntry),
	}
}

// AddChunk embeds and stores a chunk
func (s *EmbeddingStore) AddChunk(chunk models.Chunk) {
	vector := s.TFVector(chunk.Text)
	s.vectors[chunk.ID] = VectorEntry{
		Vector:          vector,
		ChunkID:         chunk.ID,
		TopicID:         chunk.TopicID,
		ParentID:        chunk.ParentID,
		ImportanceScore: chunk.ImportanceScore,
		WeaknessScore:   chunk.WeaknessScore,
	}
}

// TFVector creates a simple term frequency vector from text
func (s *EmbeddingStore) TFVector(text string) map[string]float64 {
	words := s.Tokenize(text)
	vector := make(map[string]float64)

	for _, word := range words {
		vector[word]++
	}

	totalWords := float64(len(words))
	for key := range vector {
		vector[key] = vector[key] / totalWords
	}

	return vector
}

// Tokenize breaks text into lowercase words
func (s *EmbeddingStore) Tokenize(text string) []string {
	text = strings.ToLower(text)
	words := strings.FieldsFunc(text, func(r rune) bool {
		return !((r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'))
	})

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

	for word, freq2 := range vec2 {
		magnitude2 += freq2 * freq2
		if freq1, exists := vec1[word]; exists {
			dotProduct += freq1 * freq2
		}
	}

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
	ChunkID         string
	Text            string
	TopicID         string
	ParentID        string
	ImportanceScore float64
	WeaknessScore   float64
	Score           float64
}

// SearchTopK retrieves the top-k most similar chunks for a query
func (s *EmbeddingStore) SearchTopK(query string, chunks []models.Chunk, k int) []RetrievalResult {
	queryVector := s.TFVector(query)

	var results []RetrievalResult
	for _, chunk := range chunks {
		chunkID := chunk.ID
		text := chunk.Text

		if entry, exists := s.vectors[chunkID]; exists {
			score := s.CosineSimilarity(queryVector, entry.Vector)
			results = append(results, RetrievalResult{
				ChunkID:         chunkID,
				Text:            text,
				TopicID:         entry.TopicID,
				ParentID:        entry.ParentID,
				ImportanceScore: entry.ImportanceScore,
				WeaknessScore:   entry.WeaknessScore,
				Score:           score,
			})
		}
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	if len(results) > k {
		results = results[:k]
	}

	return results
}

// RetrievalContext holds the retrieved context for RAG
type RetrievalContext struct {
	TopicID   string
	Sections  map[string]string
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
			section, err := db.GetParentSection(result.ParentID)
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

// ApplyHeuristicScoring is an explicit retrieval-stage hook for reranking.
func ApplyHeuristicScoring(results []RetrievalResult) []RetrievalResult {
	return results
}
