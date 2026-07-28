package retrieval

import (
	"context"
	"fmt"

	"ai-tutor/internal/db"
	"ai-tutor/internal/embeddings"
	"ai-tutor/internal/models"
	"ai-tutor/internal/utils"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
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
	repo     *db.Repository
	embedder *embeddings.OnnxEmbedder
	config   IndexerConfig
	ctx      context.Context
}

// NewVectorIndexer creates a new vector indexer.
func NewVectorIndexer(repo *db.Repository, embedder *embeddings.OnnxEmbedder, config IndexerConfig, ctx context.Context) *VectorIndexer {
	return &VectorIndexer{
		repo:     repo,
		embedder: embedder,
		config:   config,
		ctx:      ctx,
	}
}

// IndexTopicChunks generates and stores embeddings for all chunks of a topic.
// Uses hash-based incremental indexing: only recomputes vectors if source text has changed.
// Emits progress events during processing.
func (vi *VectorIndexer) IndexTopicChunks(topicID string) error {
	_, err := vi.indexChunks(
		topicID,
		"topic",
		func() ([]models.Chunk, error) { return vi.repo.GetChunksForTopic(topicID) },
		func() (map[string]string, error) { return vi.repo.GetChunkEmbeddingRefsForTopic(topicID) },
		func(processed, total, failed int) { vi.emitIndexingProgress(topicID, processed, total, failed) },
	)
	return err
}

// indexChunks consolidates shared incremental embedding generation and batch persistence logic.
func (vi *VectorIndexer) indexChunks(
	scopeID string,
	scopeType string,
	fetchChunks func() ([]models.Chunk, error),
	fetchRefs func() (map[string]string, error),
	emitProgress func(processed, total, failed int),
) (int, error) {
	if vi.embedder == nil {
		return 0, fmt.Errorf("embedder not initialized")
	}

	chunks, err := fetchChunks()
	if err != nil {
		return 0, fmt.Errorf("failed to fetch chunks for %s %s: %w", scopeType, scopeID, err)
	}

	if len(chunks) == 0 {
		utils.Infof("No chunks found for %s %s", scopeType, scopeID)
		return 0, nil
	}

	utils.Infof("Indexing %d chunks for %s %s", len(chunks), scopeType, scopeID)

	chunkHashRefs := map[string]string{}
	if vi.config.RecomputeOnHashMismatch && !vi.config.ForceReindex {
		chunkHashRefs, err = fetchRefs()
		if err != nil {
			return 0, fmt.Errorf("failed to fetch embedding refs for %s %s: %w", scopeType, scopeID, err)
		}
	}

	// Collect chunks that need reindexing
	chunksToReindex := make([]models.Chunk, 0)
	skipped := 0

	for _, chunk := range chunks {
		shouldReindex := vi.config.ForceReindex

		if !shouldReindex && vi.config.RecomputeOnHashMismatch {
			shouldReindex = !doesHashMatch(chunk, chunkHashRefs)
		}

		if shouldReindex {
			chunksToReindex = append(chunksToReindex, chunk)
		} else {
			skipped++
		}
	}

	if len(chunksToReindex) == 0 {
		utils.Infof("Indexing complete for %s %s: reindexed=0, skipped=%d, failed=0", scopeType, scopeID, skipped)
		return 0, nil
	}

	utils.Infof("Processing %d chunks for reindexing in %s %s", len(chunksToReindex), scopeType, scopeID)

	// Generate embeddings for all chunks that need reindexing
	vectorBatch := make([]db.ChunkVectorBatchItem, 0, len(chunksToReindex))
	embeddingBatch := make([]db.ChunkEmbeddingBatchItem, 0, len(chunksToReindex))
	failedChunks := make(map[string]struct{})

	for i, chunk := range chunksToReindex {
		vector, err := vi.embedder.Embed(chunk.Text)
		if err != nil {
			utils.Warnf("embedding failed for chunk %s: %v", chunk.ID, err)
			failedChunks[chunk.ID] = struct{}{}
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

		if emitProgress != nil && ((i+1)%10 == 0 || i == len(chunksToReindex)-1) {
			emitProgress(i+1, len(chunksToReindex), len(failedChunks))
		}
	}

	if len(vectorBatch) == 0 {
		utils.Infof("Indexing complete for %s %s: reindexed=0, skipped=%d, failed=%d", scopeType, scopeID, skipped, len(failedChunks))
		return len(failedChunks), nil
	}

	// Batch store vectors
	if err := vi.repo.UpsertChunkVectorsBatch(vectorBatch); err != nil {
		utils.Warnf("failed to batch store vectors for %s %s: %v", scopeType, scopeID, err)
		for _, item := range vectorBatch {
			if err := vi.repo.UpsertChunkVector(item.ChunkID, item.Vector); err != nil {
				utils.Warnf("failed to store vector for chunk %s: %v", item.ChunkID, err)
				failedChunks[item.ChunkID] = struct{}{}
			}
		}
	}

	// Batch update embedding metadata
	if err := vi.repo.UpdateChunkEmbeddingsBatch(embeddingBatch); err != nil {
		utils.Warnf("failed to batch update embedding metadata for %s %s: %v", scopeType, scopeID, err)
		for _, item := range embeddingBatch {
			if err := vi.repo.UpdateChunkEmbedding(item.ChunkID, item.Hash); err != nil {
				utils.Warnf("failed to update chunk embedding metadata for chunk %s: %v", item.ChunkID, err)
				failedChunks[item.ChunkID] = struct{}{}
			}
		}
	}

	reindexed := len(vectorBatch) - len(failedChunks)
	utils.Infof("Indexing complete for %s %s: reindexed=%d, skipped=%d, failed=%d", scopeType, scopeID, reindexed, skipped, len(failedChunks))
	return len(failedChunks), nil
}

// IndexAllTopics reindexes all topics in the database.
// Updates notebook indexing_status from PENDING -> INDEXING -> READY/FAILED.
func (vi *VectorIndexer) IndexAllTopics() error {
	topicIDs, err := vi.repo.GetAllTopicIDs()
	if err != nil {
		return fmt.Errorf("failed to get topic IDs: %w", err)
	}

	// Get all notebooks with PENDING indexing status (no profile filter for indexing)
	notebooks, err := vi.repo.GetNotebooks("", "")
	if err != nil {
		utils.Warnf("failed to fetch notebooks for indexing: %v", err)
		// Continue anyway, we'll index by topic
	}

	// Track notebook IDs that were transitioned to INDEXING
	indexingNotebookIDs := make(map[string]struct{})
	for _, nb := range notebooks {
		if nb.IndexingStatus == "PENDING" {
			if err := vi.repo.UpdateNotebookIndexingStatus(nb.ID, "INDEXING"); err == nil {
				indexingNotebookIDs[nb.ID] = struct{}{}
			}
		}
	}

	for _, topicID := range topicIDs {
		if err := vi.IndexTopicChunks(topicID); err != nil {
			utils.Warnf("indexing failed for topic %s: %v", topicID, err)
		}
	}

	// Set indexing status to READY for notebooks that were being indexed
	for notebookID := range indexingNotebookIDs {
		_ = vi.repo.UpdateNotebookIndexingStatus(notebookID, "READY")
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

// emitIndexingProgress emits lightweight progress events for semantic indexing.
func (vi *VectorIndexer) emitIndexingProgress(topicID string, processed, total, failed int) {
	if vi.ctx == nil {
		return
	}
	payload := map[string]interface{}{
		"topic_id":         topicID,
		"stage":            "indexing",
		"processed_chunks": processed,
		"total_chunks":     total,
		"failed_chunks":    failed,
		"percent":          int((float64(processed) / float64(total)) * 100),
	}
	wailsruntime.EventsEmit(vi.ctx, "ingestion-progress", payload)
}

// IndexNotebook generates and stores embeddings for all chunks of a notebook.
// Uses hash-based incremental indexing.
// Emits progress events during processing.
func (vi *VectorIndexer) IndexNotebook(notebookID string) error {
	if vi.embedder == nil {
		return fmt.Errorf("embedder not initialized")
	}

	// Update status to INDEXING
	if err := vi.repo.UpdateNotebookIndexingStatus(notebookID, "INDEXING"); err != nil {
		return fmt.Errorf("failed to update indexing status to INDEXING: %w", err)
	}

	failedCount, err := vi.indexChunks(
		notebookID,
		"notebook",
		func() ([]models.Chunk, error) { return vi.repo.GetChunksForNotebook(notebookID) },
		func() (map[string]string, error) { return vi.repo.GetChunkEmbeddingRefsForNotebook(notebookID) },
		func(processed, total, failed int) { vi.emitNotebookIndexingProgress(notebookID, processed, total, failed) },
	)

	if err != nil || failedCount > 0 {
		_ = vi.repo.UpdateNotebookIndexingStatus(notebookID, "FAILED")
		return err
	}

	_ = vi.repo.UpdateNotebookIndexingStatus(notebookID, "READY")
	return nil
}

// emitNotebookIndexingProgress emits progress events for notebook semantic indexing.
func (vi *VectorIndexer) emitNotebookIndexingProgress(notebookID string, processed, total, failed int) {
	if vi.ctx == nil {
		return
	}
	payload := map[string]interface{}{
		"notebook_id":      notebookID,
		"stage":            "indexing",
		"processed_chunks": processed,
		"total_chunks":     total,
		"failed_chunks":    failed,
		"percent":          int((float64(processed) / float64(total)) * 100),
	}
	wailsruntime.EventsEmit(vi.ctx, "notebook-indexing-progress", payload)
}
