package rag

import (
	"crypto/md5"
	"fmt"
	"log"
	"time"

	"ai-tutor/internal/db"
	"ai-tutor/internal/embeddings"
	"ai-tutor/internal/models"
)

// IndexerConfig holds indexing configuration
type IndexerConfig struct {
	// RecomputeOnHashMismatch: if true, recompute vectors when source text hash changes
	RecomputeOnHashMismatch bool
	// ForceReindex: if true, force full reindex regardless of stored hashes
	ForceReindex bool
}

// VectorIndexer manages persistent vector indexing with checksum-based incremental updates.
type VectorIndexer struct {
	embedder *embeddings.OnnxEmbedder
	config   IndexerConfig
}

// NewVectorIndexer creates a new vector indexer.
func NewVectorIndexer(embedder *embeddings.OnnxEmbedder, config IndexerConfig) *VectorIndexer {
	return &VectorIndexer{
		embedder: embedder,
		config:   config,
	}
}

// IndexTopicChunks generates and stores embeddings for all chunks of a topic.
// Uses hash-based incremental indexing: only recomputes vectors if source text has changed.
func (vi *VectorIndexer) IndexTopicChunks(topicID string) error {
	if vi.embedder == nil {
		return fmt.Errorf("embedder not initialized")
	}

	// Fetch all chunks for the topic
	chunks, err := db.GetChunksForTopic(topicID)
	if err != nil {
		return fmt.Errorf("failed to fetch chunks for topic %s: %w", topicID, err)
	}

	if len(chunks) == 0 {
		log.Printf("No chunks found for topic %s", topicID)
		return nil
	}

	log.Printf("Indexing %d chunks for topic %s", len(chunks), topicID)

	chunkHashRefs := map[string]string{}
	if vi.config.RecomputeOnHashMismatch && !vi.config.ForceReindex {
		chunkHashRefs, err = db.GetChunkEmbeddingRefsForTopic(topicID)
		if err != nil {
			return fmt.Errorf("failed to fetch embedding refs for topic %s: %w", topicID, err)
		}
	}

	// Index each chunk
	reindexed := 0
	skipped := 0
	failed := 0
	embeddingElapsed := time.Duration(0)
	dbElapsed := time.Duration(0)
	startedAt := time.Now()

	pending := make([]db.ChunkVectorBatchItem, 0, len(chunks))

	for _, chunk := range chunks {
		shouldReindex := vi.config.ForceReindex

		if !shouldReindex && vi.config.RecomputeOnHashMismatch {
			// Check if source text hash still matches
			shouldReindex = !doesHashMatch(chunk, chunkHashRefs)
		}

		if shouldReindex {
			// Generate new embedding
			embedStart := time.Now()
			vector, err := vi.embedder.Embed(chunk.Text)
			embeddingElapsed += time.Since(embedStart)
			if err != nil {
				log.Printf("Warning: embedding failed for chunk %s: %v", chunk.ID, err)
				failed++
				continue
			}

			hash := computeTextHash(chunk.Text)
			pending = append(pending, db.ChunkVectorBatchItem{
				ChunkID:      chunk.ID,
				Vector:       vector,
				EmbeddingRef: hash,
			})
		} else {
			skipped++
		}
	}

	if len(pending) > 0 {
		dbStart := time.Now()
		if err := db.UpsertChunkVectorsBatch(pending); err != nil {
			log.Printf("Warning: batch vector persistence failed for topic %s: %v; falling back to per-chunk writes", topicID, err)
			for _, item := range pending {
				if err := db.UpsertChunkVector(item.ChunkID, item.Vector); err != nil {
					log.Printf("Warning: failed to store vector for chunk %s: %v", item.ChunkID, err)
					failed++
					continue
				}
				if err := db.UpdateChunkEmbedding(item.ChunkID, item.EmbeddingRef); err != nil {
					log.Printf("Warning: failed to update chunk embedding metadata for %s: %v", item.ChunkID, err)
					failed++
					continue
				}
				reindexed++
			}
		} else {
			reindexed = len(pending)
		}
		dbElapsed += time.Since(dbStart)
	}

	log.Printf(
		"Indexing complete for topic %s: reindexed=%d, skipped=%d, failed=%d, embed_ms=%d, db_ms=%d, total_ms=%d",
		topicID,
		reindexed,
		skipped,
		failed,
		embeddingElapsed.Milliseconds(),
		dbElapsed.Milliseconds(),
		time.Since(startedAt).Milliseconds(),
	)
	return nil
}

// IndexAllTopics reindexes all topics in the database.
func (vi *VectorIndexer) IndexAllTopics() error {
	topicIDs, err := db.GetAllTopicIDs()
	if err != nil {
		return fmt.Errorf("failed to get topic IDs: %w", err)
	}

	for _, topicID := range topicIDs {
		if err := vi.IndexTopicChunks(topicID); err != nil {
			log.Printf("Warning: indexing failed for topic %s: %v", topicID, err)
		}
	}

	return nil
}

// doesHashMatch checks if a chunk's source text hash matches the prefetched stored hash.
func doesHashMatch(chunk models.Chunk, chunkHashRefs map[string]string) bool {
	storedHash, ok := chunkHashRefs[chunk.ID]
	if !ok {
		return false
	}
	if storedHash == "" {
		return false
	}

	currentHash := computeTextHash(chunk.Text)
	return storedHash == currentHash
}

// computeTextHash computes MD5 hash of text for change detection.
func computeTextHash(text string) string {
	hash := md5.Sum([]byte(text))
	return fmt.Sprintf("%x", hash)
}
