# Walkthrough — Background Vector Indexing

We have implemented **Option A: SQLite-Backed Sequential Worker Queue** for background vector indexing to prevent SQLite database locks, ensure deterministic state via the database, and protect Socratic Tutor and Reader AI features while indexing is in progress.

## Changes Made

### 1. Database Layer (`internal/db/notebooks_repo.go`)
- Added [ResetIndexingStatus](ai-tutor/internal/db/notebooks_repo.go) to reset stuck `INDEXING` status back to `PENDING` on startup.
- Added [GetPendingNotebookIDs](fai-tutor/internal/db/notebooks_repo.go) to retrieve all pending notebooks needing indexing.
- Added [GetChunkEmbeddingRefsForNotebook](fai-tutor/internal/db/notebooks_repo.go) to get MD5 checksums for existing chunks of a notebook for change-detection.
- Added [GetNotebookIndexingProgress](fai-tutor/internal/db/notebooks_repo.go) to fetch the total chunk count, indexed chunk count (non-empty `embedding_ref`), and the current `indexing_status`.
- Added [GetNotebookIDByTopic](fai-tutor/internal/db/notebooks_repo.go) to resolve a parent notebook ID from a topic ID when the frontend only provides a topic ID.

### 2. Retrieval Layer (`internal/retrieval/`)
- Created [queue.go](fai-tutor/internal/retrieval/queue.go) with a thread-safe, sequential `VectorIndexQueue` using a background worker channel.
- Added `IndexNotebook` and `emitNotebookIndexingProgress` methods to the `VectorIndexer` in [indexer.go](ai-tutor/internal/retrieval/indexer.go) to index chunks scoped to a single notebook and emit progress.

### 3. Application Lifecycle (`app.go` & `internal/runtime/boot.go`)
- Initialized and started `indexQueue` in `startup()` if `aiReady` is active.
- Enqueued all `PENDING` notebooks at boot time to run indexing asynchronously.
- Stopped the queue worker gracefully inside `shutdown()`.
- Removed the slow, synchronous `IndexAllTopics()` call from the main boot path in [boot.go](ai-tutor/internal/runtime/boot.go) to speed up boot times.

### 4. API Endpoints protection (`app.go` & `notebook_endpoints.go`)
- Enqueued newly syllabus-confirmed notebooks in `ConfirmNotebookSyllabus` in [notebook_endpoints.go](fai-tutor/notebook_endpoints.go) to trigger background indexing.
- Added a `checkNotebookIndexingStatus` interceptor to `AskSocratic` and `AskReaderAI` in [app.go](fai-tutor/app.go) to check if the notebook is READY. If not, they return:
  ```json
  { "status": "indexing", "progress": <percentage>, "error": "AI features are disabled while this notebook is indexing." }
  ```

---

## Verification Results

### Automated Tests
Ran the suite via `go test ./...` which successfully passed. The contract tests have been updated in `app_contract_test.go` to mark the mock notebook as `READY` to bypass the interceptor:

```bash
ok  	ai-tutor	8.715s
```
