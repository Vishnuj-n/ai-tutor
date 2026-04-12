# 🥷 Solutions Log - 2026-04-11

This file records the completed solutions delivered in this development cycle.

## 1) Socratic Tutor Chat Interface
- Replaced placeholder Socratic page with a real chat-style interface for RAG validation.
- Added topic-aware flow, compact chat UI, loading/error states, and citations display.
- Updated surface style variable used by the page.

## 2) Frontend Linting and Formatting Setup
- Added ESLint + Prettier setup for the frontend.
- Added lint/format scripts and supporting config files.
- Fixed follow-up script/config issues so lint can run cleanly.

## 3) Removed Native Tokenizers Build-Tag Dependency
- Removed prior split approach that required tokenizers build tags.
- Standardized on a pure-Go path for tokenizer handling.
- Cleaned VS Code build tags configuration for default builds.

## 4) Notebook Ingestion - Phase A
- Implemented extraction + normalization for TXT/MD/PDF.
- Added deterministic chunk planning and transactional relational ingestion.
- Added notebook status and chunk_count updates in database flow.

## 5) Notebook Ingestion - Phase B
- Implemented indexing loop with progress events and cancellation checks.
- Added backend progress emission and frontend progress subscription.
- Added status transitions for indexed/partial states.

## 6) Real ONNX Embeddings
- Upgraded embedder to run real inference with:
  - `github.com/sugarme/tokenizer`
  - `github.com/yalue/onnxruntime_go`
- Added runtime/session setup, model I/O inspection, tokenization, inference, pooling, and vector normalization.
- Added proper resource cleanup for session/runtime lifecycle.

## 7) Embedding Diagnostics Endpoint
- Added backend endpoint to run a live embedding and return:
  - vector length
  - declared dimension match
  - sample L2 norm
  - sample vector values
- Purpose: verify embedding math and runtime health before UI integration.

## 8) Lint Fix (ineffassign)
- Fixed ineffectual `status` assignment in notebook upload flow.
- Confirmed clean lint/build after the fix.

## 9) SQLite Connection Pool Isolation Fix
- **Problem:** Vec0 sqlite extension loaded on one connection, but connection pool created new connections without extension, causing "no such module: vec0" errors during indexing.
- **Root Cause:** SQLite extension modules are connection-scoped; pooling multiple connections breaks extension persistence.
- **Solution:** Constrained SQLite to single connection pool (`SetMaxOpenConns(1)`, `SetMaxIdleConns(1)`) in `internal/db/store.go`.
- **Impact:** All DB operations now use the same handle, ensuring vec0 extension remains loaded throughout application lifetime.
- **Code Changes:** Added pool constraints in `Init()` function and driver-level extension loading via `sqliteConn.LoadExtension()`.

## 10) Vector Storage Type Handling (JSON + rowid Mapping)
- **Problem:** Encountered two type errors: `unsupported type []float32` and `Only integers allowed for primary key values on chunk_vectors`.
- **Root Cause:** (1) database/sql cannot bind Go slices directly; (2) vec0 virtual table requires integer rowids but application uses string chunk IDs.
- **Solution:** Added two helper functions:
  - `vectorToJSON()`: Serializes float32 slice to JSON string before DB binding
  - `lookupChunkRowID()`: Resolves string chunk IDs to SQLite integer rowids before vec0 operations
- **Impact:** Vector inserts and queries now use correct types: JSON strings and integer rowids respectively.
- **Code Changes:** `internal/db/store.go` - Updated `UpsertChunkVector()`, `SearchVectorsForTopic()`, and related functions to use rowid-based retrieval.

## 11) Local AI Runtime Readiness Gate Fix
- **Problem:** "Ask AI unavailable: local AI runtime is not ready" message blocked all queries despite ONNX embedder and vec0 being fully initialized.
- **Root Cause:** Readiness gate required `aiInitError == ""`, so any non-fatal warning (e.g., "failed to create vec0 table") kept AI disabled indefinitely.
- **Solution:** Changed `aiReady` gate in `app.go` from `embedder != nil && aiInitError == ""` to `embedder != nil` only.
- **Impact:** Ask AI becomes available immediately after ONNX embedder loads, independent of non-fatal vector table warnings.
- **Code Changes:** `app.go` - Simplified readiness check and made vector table failures non-fatal with lexical fallback.

## 12) Async Background Indexing
- **Problem:** Full chunk indexing blocked Ask AI availability until all topics were indexed, causing startup delays on large topic sets.
- **Root Cause:** Synchronous indexing in main init path tied Ask AI readiness to complete indexing duration.
- **Solution:** Moved indexing to background goroutine after embedder initialization completes. Ask AI readiness is set before indexing starts.
- **Impact:** Ask AI becomes available immediately after embedder init (~1-2 seconds) instead of after full topic indexing (~N seconds for N topics).
- **Code Changes:** `app.go` - Wrapped `indexer.IndexAllTopics()` in `go func() {}` after `aiReady = true` is set.

## 13) Windows Test Cleanup Fixes
- **Problem:** Tests fail on Windows with "unlinkat: The process cannot access the file because it is being used by another process" when temp directory cleanup tries to delete locked DB files.
- **Root Cause:** Global `db.conn` was never closed before temp DB file removal, leaving file handle locked and preventing cleanup.
- **Solution:** Added `db.Close()` function to explicitly release database connection and added `t.Cleanup(db.Close())` hooks in test setup functions.
- **Impact:** All tests now pass on Windows without file lock errors. Proper resource cleanup is guaranteed before temp file removal.
- **Code Changes:** `internal/db/store.go` - Added `Close()` function; `app_contract_test.go` and `internal/rag/pipeline_test.go` - Added cleanup hooks in test initialization.

## 14) Keyboard UX Normalization
- **Problem:** Socratic chat required Ctrl+Enter to send messages, non-standard for chat interfaces.
- **Root Cause:** Initial implementation used Vue's `@keydown.enter.ctrl` modifier.
- **Solution:** Replaced with custom `handleComposerKeydown()` function implementing industry-standard behavior:
  - Enter (without Shift): Sends message
  - Shift+Enter: Inserts newline
- **Impact:** Chat interface now follows familiar UX patterns expected by most users.
- **Code Changes:** `frontend/src/pages/Socratic.vue` - Implemented `handleComposerKeydown()` handler and updated helper text to "Enter to send, Shift+Enter for new line".

## Build & Runtime Notes
- Build command: `wails build -tags sqlite_extension` or `wails dev -tags sqlite_extension` (CGO_ENABLED=1 required)
- Asset requirements: `model_int8.onnx`, `tokenizer.json`, `onnxruntime.dll`, `vec0.dll` must be present in `asset/` folder
- SQLite pool constraint is permanent architectural pattern; do not increase `MaxOpenConns`
- All vector operations require rowid integers (not string IDs) and JSON serialization

## Notes
- Current notebook behavior is by design: uploads without selected topic are stored as `uploaded_unlinked` and do not create chunks/index vectors.
- To enable RAG retrieval for a notebook, upload with a linked topic (or implement a post-upload link+ingest action).
- All tests pass without Windows file lock errors; cleanup pattern applies to future tests involving temp DB files.
