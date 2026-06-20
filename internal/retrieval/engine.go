// Package retrieval provides a standalone, reusable semantic search engine.
// It wraps ONNX embedding + sqlite-vec cosine search with a clean public API
// so any consumer (currently only socratic.go) can call SemanticSearch without
// importing the full RAG pipeline.
package retrieval

import (
	"container/list"
	"errors"
	"fmt"
	"log"
	"math"
	"sort"
	"strings"
	"sync"

	"ai-tutor/internal/db"
	"ai-tutor/internal/embeddings"
	"ai-tutor/internal/models"
	"ai-tutor/internal/utils"
)

var ErrInvalidNotebookContext = errors.New("invalid notebook context: notebook ID is required")

// SearchResult is a single ranked chunk returned by SemanticSearch.
type SearchResult struct {
	ChunkID         string
	Text            string
	TopicID         string
	PageNum         int
	ImportanceScore float64
	WeaknessScore   float64
	Score           float64
}

type Scope string

const (
	ScopeTopic    Scope = "topic"
	ScopeNotebook Scope = "notebook"
)

// Engine performs semantic similarity search using ONNX embeddings + sqlite-vec
// with a lexical TF-cosine fallback when ONNX is unavailable.
type Engine struct {
	repo     *db.Repository
	embedder *embeddings.OnnxEmbedder
	mu       sync.RWMutex
	// tfCache stores pre-built TF vectors for the lexical fallback path.
	tfCache map[string]map[string]float64
	// lruList maintains LRU order for cache eviction
	lruList *list.List
	// lruMap provides quick access to list elements
	lruMap map[string]*list.Element
	// maxCacheSize prevents unlimited memory growth
	maxCacheSize int
}

// NewEngine creates a retrieval engine.  embedder may be nil; the engine will
// fall back to lexical cosine similarity in that case.
func NewEngine(repo *db.Repository, embedder *embeddings.OnnxEmbedder) *Engine {
	return &Engine{
		repo:         repo,
		embedder:     embedder,
		tfCache:      make(map[string]map[string]float64),
		lruList:      list.New(),
		lruMap:       make(map[string]*list.Element),
		maxCacheSize: 10000, // Limit cache to prevent memory leaks
	}
}

// AddChunk pre-builds the TF vector for the lexical fallback path.
// Call this once per chunk at startup (mirrors rag.EmbeddingStore.AddChunk).
func (e *Engine) AddChunk(chunk models.Chunk) {
	vec := e.tfVector(chunk.Text)
	e.mu.Lock()
	defer e.mu.Unlock()

	// If chunk already exists, move it to front
	if elem, exists := e.lruMap[chunk.ID]; exists {
		e.lruList.MoveToFront(elem)
		e.tfCache[chunk.ID] = vec
		return
	}

	// Add new chunk to cache
	e.tfCache[chunk.ID] = vec
	elem := e.lruList.PushFront(chunk.ID)
	e.lruMap[chunk.ID] = elem

	// Evict oldest if cache is full
	if len(e.tfCache) > e.maxCacheSize {
		oldest := e.lruList.Back()
		if oldest != nil {
			oldestID := oldest.Value.(string)
			delete(e.tfCache, oldestID)
			delete(e.lruMap, oldestID)
			e.lruList.Remove(oldest)
		}
	}
}

// SemanticSearch returns the topK most relevant chunks for query inside the
// given topic.  Pass startPage/endPage > 0 to scope the search to a page window;
// pass 0 for both to search the whole topic.
func (e *Engine) SemanticSearch(topicID string, query string, topK int, startPage, endPage int) ([]SearchResult, error) {
	topicID = strings.TrimSpace(topicID)
	if topicID == "" {
		return nil, fmt.Errorf("topic id is required")
	}

	loadChunks := func() ([]models.Chunk, error) {
		if startPage > 0 && endPage > 0 {
			return e.repo.GetChunksForTopicPageRange(topicID, startPage, endPage)
		}
		return e.repo.GetChunksForTopic(topicID)
	}

	vectorSearch := func(queryVec []float32, k int) ([]string, error) {
		return e.repo.SearchVectorsForTopic(topicID, queryVec, k, startPage, endPage)
	}

	return e.searchWithScope("vector search", query, topK, loadChunks, vectorSearch)
}

// SemanticSearchNotebook returns the topK most relevant chunks linked to one notebook.
func (e *Engine) SemanticSearchNotebook(notebookID string, topicID string, query string, topK int) ([]SearchResult, error) {
	notebookID = strings.TrimSpace(notebookID)
	if notebookID == "" {
		return nil, ErrInvalidNotebookContext
	}
	topicID = strings.TrimSpace(topicID)

	var scopedChunksCache []models.Chunk
	var scopedChunksLoaded bool
	getScopedChunks := func() ([]models.Chunk, error) {
		if scopedChunksLoaded {
			return scopedChunksCache, nil
		}
		chunks, err := e.repo.GetChunksForNotebook(notebookID)
		if err != nil {
			return nil, err
		}
		if topicID == "" {
			scopedChunksCache = chunks
			scopedChunksLoaded = true
			return scopedChunksCache, nil
		}
		filtered := make([]models.Chunk, 0, len(chunks))
		for _, c := range chunks {
			if c.TopicID == topicID {
				filtered = append(filtered, c)
			}
		}
		scopedChunksCache = filtered
		scopedChunksLoaded = true
		return scopedChunksCache, nil
	}

	// Load chunks for notebook, optionally scoped to a topic if provided.
	loadChunks := func() ([]models.Chunk, error) {
		return getScopedChunks()
	}

	vectorSearch := func(queryVec []float32, k int) ([]string, error) {
		if topicID == "" {
			return e.repo.SearchVectorsForNotebook(notebookID, queryVec, k)
		}

		scopedChunks, err := getScopedChunks()
		if err != nil {
			return nil, err
		}
		allowed := make(map[string]struct{}, len(scopedChunks))
		for _, c := range scopedChunks {
			allowed[c.ID] = struct{}{}
		}

		filtered := make([]string, 0, k)
		seen := make(map[string]struct{}, k)
		overfetchK := k
		if overfetchK < 10 {
			overfetchK = 10
		}
		if overfetchK > 100 {
			overfetchK = 100
		}

		for {
			chunkIDs, searchErr := e.repo.SearchVectorsForNotebook(notebookID, queryVec, overfetchK)
			if searchErr != nil {
				return nil, searchErr
			}

			for _, cid := range chunkIDs {
				if _, ok := allowed[cid]; !ok {
					continue
				}
				if _, dup := seen[cid]; dup {
					continue
				}
				seen[cid] = struct{}{}
				filtered = append(filtered, cid)
				if len(filtered) >= k {
					return filtered[:k], nil
				}
			}

			if len(chunkIDs) < overfetchK || overfetchK >= 100 {
				break
			}
			nextK := overfetchK * 2
			if nextK > 100 {
				nextK = 100
			}
			if nextK == overfetchK {
				break
			}
			overfetchK = nextK
		}

		return filtered, nil
	}

	return e.searchWithScope("notebook vector search", query, topK, loadChunks, vectorSearch)
}

// --- internal helpers ---

// searchWithScope is a shared search implementation for both topic and notebook scopes.
// It handles chunk loading, embedding, vector search, and lexical fallback.
func (e *Engine) searchWithScope(
	scopeName string,
	query string,
	topK int,
	loadChunks func() ([]models.Chunk, error),
	vectorSearch func([]float32, int) ([]string, error),
) ([]SearchResult, error) {
	if topK <= 0 {
		topK = 5
	}

	chunks, err := loadChunks()
	if err != nil {
		return nil, fmt.Errorf("could not load chunks: %w", err)
	}
	if len(chunks) == 0 {
		return nil, fmt.Errorf("no chunks found")
	}

	k := topK
	if len(chunks) < k {
		k = len(chunks)
	}

	// We'll collect chunk-level results in chunkResults
	var chunkResults []SearchResult

	// --- ONNX path (preferred) ---
	if e.embedder != nil {
		queryVec, embedErr := e.embedder.Embed(query)
		if embedErr == nil {
			chunkIDs, searchErr := vectorSearch(queryVec, k)
			if searchErr == nil && len(chunkIDs) > 0 {
				byID := make(map[string]models.Chunk, len(chunks))
				for _, c := range chunks {
					byID[c.ID] = c
				}
				results := make([]SearchResult, 0, len(chunkIDs))
				for i, cid := range chunkIDs {
					c, ok := byID[cid]
					if !ok {
						continue
					}
					results = append(results, SearchResult{
						ChunkID:         c.ID,
						Text:            c.Text,
						TopicID:         c.TopicID,
						PageNum:         c.PageNum,
						ImportanceScore: c.ImportanceScore,
						WeaknessScore:   c.WeaknessScore,
						Score:           float64(len(chunkIDs) - i),
					})
				}
				chunkResults = results
			}
			if searchErr != nil {
				log.Printf("retrieval: %s unavailable, falling back to lexical: %v", scopeName, searchErr)
			}
		} else {
			log.Printf("retrieval: query embedding failed, falling back to lexical: %v", embedErr)
		}
	} else {
		utils.RagLogger.Warn("retrieval: embedder is nil, falling back to lexical search", "scope", scopeName)
	}

	// If ONNX didn't produce results, use lexical fallback
	if len(chunkResults) == 0 {
		chunkResults = e.lexicalSearch(query, chunks, k)
	}

	sort.Slice(chunkResults, func(i, j int) bool { return chunkResults[i].Score > chunkResults[j].Score })
	if len(chunkResults) > topK {
		chunkResults = chunkResults[:topK]
	}

	return chunkResults, nil
}

// --- internal helpers ---

func (e *Engine) lexicalSearch(query string, chunks []models.Chunk, k int) []SearchResult {
	qVec := e.tfVector(query)
	var results []SearchResult

	e.mu.RLock()
	defer e.mu.RUnlock()
	for _, c := range chunks {
		// Page filtering is now handled at database level
		cVec, ok := e.tfCache[c.ID]
		if !ok {
			cVec = e.tfVector(c.Text)
		}
		score := cosineSimilarity(qVec, cVec)
		results = append(results, SearchResult{
			ChunkID:         c.ID,
			Text:            c.Text,
			TopicID:         c.TopicID,
			PageNum:         c.PageNum,
			ImportanceScore: c.ImportanceScore,
			WeaknessScore:   c.WeaknessScore,
			Score:           score,
		})
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})
	if len(results) > k {
		results = results[:k]
	}
	return results
}

func (e *Engine) tfVector(text string) map[string]float64 {
	words := tokenize(text)
	vec := make(map[string]float64, len(words))
	for _, w := range words {
		vec[w]++
	}
	total := float64(len(words))
	if total > 0 {
		for k := range vec {
			vec[k] /= total
		}
	}
	return vec
}

var stopWords = map[string]bool{
	"the": true, "a": true, "an": true, "and": true, "or": true,
	"in": true, "on": true, "at": true, "to": true, "for": true,
	"is": true, "are": true, "was": true, "be": true, "been": true,
	"have": true, "has": true, "do": true, "does": true, "did": true,
	"of": true, "with": true, "by": true, "from": true, "as": true,
	"if": true, "about": true, "into": true, "it": true, "its": true,
	"that": true, "this": true, "which": true, "who": true,
}

func tokenize(text string) []string {
	text = strings.ToLower(text)
	raw := strings.FieldsFunc(text, func(r rune) bool {
		return (r < 'a' || r > 'z') && (r < '0' || r > '9')
	})
	out := make([]string, 0, len(raw))
	for _, w := range raw {
		if len(w) > 2 && !stopWords[w] {
			out = append(out, w)
		}
	}
	return out
}

func cosineSimilarity(v1, v2 map[string]float64) float64 {
	dot, mag1, mag2 := 0.0, 0.0, 0.0
	for w, f2 := range v2 {
		mag2 += f2 * f2
		if f1, ok := v1[w]; ok {
			dot += f1 * f2
		}
	}
	for _, f1 := range v1 {
		mag1 += f1 * f1
	}
	mag1 = math.Sqrt(mag1)
	mag2 = math.Sqrt(mag2)
	if mag1 == 0 || mag2 == 0 {
		return 0
	}
	return dot / (mag1 * mag2)
}
