package retrieval

import (
	"context"
	"sync"

	"ai-tutor/internal/db"
	"ai-tutor/internal/embeddings"
	"ai-tutor/internal/utils"
)

// VectorIndexQueue manages a sequential queue of notebooks that need semantic indexing.
type VectorIndexQueue struct {
	repo      *db.Repository
	embedder  *embeddings.OnnxEmbedder
	ctx       context.Context
	queue     chan string
	mu        sync.Mutex
	active    map[string]bool
	wg        sync.WaitGroup
	cancel    context.CancelFunc
	workerCtx context.Context
}

// NewVectorIndexQueue creates a new indexing queue.
func NewVectorIndexQueue(repo *db.Repository, embedder *embeddings.OnnxEmbedder, ctx context.Context) *VectorIndexQueue {
	workerCtx, cancel := context.WithCancel(context.Background())
	q := &VectorIndexQueue{
		repo:      repo,
		embedder:  embedder,
		ctx:       ctx,
		queue:     make(chan string, 1000),
		active:    make(map[string]bool),
		cancel:    cancel,
		workerCtx: workerCtx,
	}
	return q
}

// Start launches the sequential background worker.
func (q *VectorIndexQueue) Start() {
	q.wg.Add(1)
	go func() {
		defer q.wg.Done()
		for {
			select {
			case <-q.workerCtx.Done():
				return
			case notebookID, ok := <-q.queue:
				if !ok {
					return
				}
				q.processNotebook(notebookID)
			}
		}
	}()
}

// Stop stops the queue worker.
func (q *VectorIndexQueue) Stop() {
	q.cancel()
	q.wg.Wait()
}

// Enqueue adds a notebook to the queue if not already enqueued or being processed.
func (q *VectorIndexQueue) Enqueue(notebookID string) {
	q.mu.Lock()
	defer q.mu.Unlock()

	if q.active[notebookID] {
		return
	}
	q.active[notebookID] = true
	
	// Non-blocking write to channel
	select {
	case q.queue <- notebookID:
	default:
		// Channel full, reset active state so it can be retried
		delete(q.active, notebookID)
		utils.Warnf("VectorIndexQueue queue channel is full; skipped enqueuing %s", notebookID)
	}
}

func (q *VectorIndexQueue) processNotebook(notebookID string) {
	defer func() {
		q.mu.Lock()
		delete(q.active, notebookID)
		q.mu.Unlock()
	}()

	indexer := NewVectorIndexer(q.repo, q.embedder, IndexerConfig{RecomputeOnHashMismatch: true}, q.ctx)
	if err := indexer.IndexNotebook(notebookID); err != nil {
		utils.Warnf("failed to index notebook %s in queue: %v", notebookID, err)
	}
}
