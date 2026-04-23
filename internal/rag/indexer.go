package rag

import (
	"fmt"

	"ai-tutor/internal/db"
	"ai-tutor/internal/embeddings"
	"ai-tutor/internal/models"
	"ai-tutor/internal/utils"
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
		utils.Infof("No chunks found for topic %s", topicID)
		return nil
	}

	utils.Infof("Indexing %d chunks for topic %s", len(chunks), topicID)

	chunkHashRefs := map[string]string{}
	if vi.config.RecomputeOnHashMismatch && !vi.config.ForceReindex {
		chunkHashRefs, err = db.GetChunkEmbeddingRefsForTopic(topicID)
		if err != nil {
			return fmt.Errorf("failed to fetch embedding refs for topic %s: %w", topicID, err)
		}
	}

	// Collect chunks that need reindexing
	chunksToReindex := make([]models.Chunk, 0)
	skipped := 0

	for _, chunk := range chunks {
		shouldReindex := vi.config.ForceReindex

		if !shouldReindex && vi.config.RecomputeOnHashMismatch {
			// Check if source text hash still matches
			shouldReindex = !doesHashMatch(chunk, chunkHashRefs)
		}

		if shouldReindex {
			chunksToReindex = append(chunksToReindex, chunk)
		} else {
			skipped++
		}
	}

	if len(chunksToReindex) == 0 {
		utils.Infof("Indexing complete for topic %s: reindexed=0, skipped=%d, failed=0", topicID, skipped)
		return nil
	}

	utils.Infof("Processing %d chunks for reindexing in topic %s", len(chunksToReindex), topicID)

	// Generate embeddings for all chunks that need reindexing
	vectorBatch := make([]db.ChunkVectorBatchItem, 0, len(chunksToReindex))
	embeddingBatch := make([]db.ChunkEmbeddingBatchItem, 0, len(chunksToReindex))
	failed := 0

	for _, chunk := range chunksToReindex {
		// Generate new embedding
		vector, err := vi.embedder.Embed(chunk.Text)
		if err != nil {
			utils.Warnf("embedding failed for chunk %s: %v", chunk.ID, err)
			failed++
			continue
		}

		hash := computeTextHash(chunk.Text)

		vectorBatch = append(vectorBatch, db.ChunkVectorBatchItem{
			ChunkID: chunk.ID,
			Vector:  vector,
		})

		embeddingBatch = append(embeddingBatch, db.ChunkEmbeddingBatchItem{
			ChunkID: chunk.ID,
			Hash:    hash,
		})
	}

	if len(vectorBatch) == 0 {
		utils.Infof("Indexing complete for topic %s: reindexed=0, skipped=%d, failed=%d", topicID, skipped, failed)
		return nil
	}

	// Batch store vectors
	if err := db.UpsertChunkVectorsBatch(vectorBatch); err != nil {
		utils.Warnf("failed to batch store vectors for topic %s: %v", topicID, err)
		// Fall back to individual operations on batch failure
		for _, item := range vectorBatch {
			if err := db.UpsertChunkVector(item.ChunkID, item.Vector); err != nil {
				utils.Warnf("failed to store vector for chunk %s: %v", item.ChunkID, err)
				failed++
			}
		}
	}

	// Batch update embedding metadata
	if err := db.UpdateChunkEmbeddingsBatch(embeddingBatch); err != nil {
		utils.Warnf("failed to batch update embedding metadata for topic %s: %v", topicID, err)
		// Fall back to individual operations on batch failure
		for _, item := range embeddingBatch {
			if err := db.UpdateChunkEmbedding(item.ChunkID, item.Hash); err != nil {
				utils.Warnf("failed to update chunk embedding metadata for chunk %s: %v", item.ChunkID, err)
				failed++
			}
		}
	}

	reindexed := len(vectorBatch) - failed
	utils.Infof("Indexing complete for topic %s: reindexed=%d, skipped=%d, failed=%d", topicID, reindexed, skipped, failed)
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
			utils.Warnf("indexing failed for topic %s: %v", topicID, err)
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
	return utils.MD5Hex(text)
}
