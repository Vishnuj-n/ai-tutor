## Plan: Local ONNX + sqlite-vec RAG Rollout

Replace the current in-memory lexical retrieval with a deterministic local pipeline that uses `asset/tokenizer.json` + `asset/model_int8.onnx` for embeddings and `sqlite-vec` (`asset/vec0.dll`) for vector search, while preserving the existing Ask AI frontend/backend contract.

**Steps**
1. Phase 1 - Baseline and guardrails:
Define a V1 migration boundary: keep `AskAI(topicID, question)` request/response shape unchanged, keep parent expansion and topic scope rules unchanged, and treat retrieval-engine replacement as backend-only. Record current behavior baselines for `chunks_retrieved`, `sections_used`, and error messages for regression checks.
2. Phase 2 - Build/runtime prerequisites (*blocks later phases*):
Add and verify native build constraints for local RAG: `CGO_ENABLED=1`, sqlite extension build tag support, and runtime discovery of `asset/onnxruntime.dll` + `asset/vec0.dll` + model/tokenizer assets. Fail fast at startup with explicit diagnostics if assets are missing or extension/model load fails.
3. Phase 3 - Embedding engine module (*depends on 2*):
Create a dedicated embedding package (for example `internal/embeddings`) that owns tokenizer+ONNX lifecycle: load `asset/tokenizer.json` via `github.com/daulet/tokenizers`, load `asset/model_int8.onnx` via `github.com/yalue/onnxruntime_go`, expose `Embed(text) -> vector` and dimension metadata, and reuse one initialized runtime/session instance.
4. Phase 4 - SQLite vector store integration (*depends on 2 and 3*):
Extend DB initialization to load sqlite extension from absolute path to `asset/vec0.dll`, create `vec0` virtual table with the embedding dimension discovered/validated from the model, and add DB functions for vector upsert/search mapped by stable `chunk_id`. Keep relational metadata in existing chunk tables and use `chunk_id` as canonical join key.
5. Phase 5 - Ingestion and indexing wiring (*depends on 3 and 4*):
Replace startup in-memory `EmbeddingStore.AddChunk` indexing with persistent indexing flow: fetch chunks, embed each chunk text, store vectors in `vec0`, and maintain idempotent indexing semantics (skip/recompute policy documented). Remove hardcoded topic assumptions during indexing bootstrap and derive topic/chunk sets from DB.
6. Phase 6 - Retrieval path swap in RAG pipeline (*depends on 4 and 5*):
Replace lexical `SearchTopK` internals with DB-backed vector search that enforces active `topic_id` filter before ranking, then preserve existing `ApplyHeuristicScoring` hook and parent expansion behavior. Keep pipeline step order unchanged from `doc/RAG.md` and maintain deterministic top-k behavior.
7. Phase 7 - Startup/service composition cleanup (*depends on 3, 4, 5, 6*):
Refactor app startup to inject concrete dependencies (embedder, vector store access) into pipeline creation, remove temporary hardcoded topic list, and centralize startup health checks for local RAG readiness.
8. Phase 8 - Contract and UX stability checks (*parallel with 7 after 6*):
Confirm Wails API outputs from `AskAI` remain unchanged (`answer`, `cited_sections`, `chunks_retrieved`, `sections_used`, `error`) so frontend files do not require behavior-changing updates. Only adjust user-facing error wording if needed for clearer local dependency failures.
9. Phase 9 - Test strategy and verification automation (*depends on 6; can run in parallel with 7/8 finalization*):
Add tests for: tokenizer+ONNX embedding smoke, sqlite extension load, vector table CRUD/search, topic-scoped retrieval invariants, parent-expansion deduplication, and end-to-end `AskAI` happy/error paths.
10. Phase 10 - Packaging and release hardening (*depends on 2-9*):
Ensure Windows build/package includes required native assets (`onnxruntime.dll`, `vec0.dll`) and runtime model/tokenizer files, verify behavior on a clean machine, and document troubleshooting for missing DLL, wrong architecture, or extension-tag build mismatch.

**Relevant files**
- `doc/RAG.md` - source-of-truth pipeline contract and runtime constraints to implement.
- `app.go` - startup composition, dependency wiring, current hardcoded indexing bootstrap.
- `internal/rag/pipeline.go` - retrieval orchestration order, heuristic hook, parent expansion integration.
- `internal/rag/embeddings.go` - current lexical retrieval path to replace with vector DB-backed search.
- `internal/db/store.go` - DB init, schema creation, extension loading, vector search helper insertion points.
- `internal/llm/provider.go` - keep stateless model call behavior unchanged while swapping retrieval backend.
- `go.mod` - add/lock tokenizer + ONNX runtime dependencies.
- `asset/tokenizer.json` - canonical tokenizer rules for chunk/query embedding consistency.
- `asset/model_int8.onnx` - local embedding model artifact used by ONNX runtime.
- `asset/onnxruntime.dll` - Windows runtime dependency for ONNX inference.
- `asset/vec0.dll` - sqlite-vec extension binary for vector search.
- `frontend/src/services/appApi.js` - confirm API contract remains stable (no behavioral contract break).
- `frontend/src/pages/Reader.vue` - verify Ask AI flow remains contextual and unchanged.

**Verification**
1. Build validation: compile with required CGO and sqlite extension settings and confirm successful startup on Windows.
2. Runtime dependency checks: startup logs clearly confirm tokenizer load, ONNX session creation, sqlite extension load, and vec table readiness.
3. Indexing validation: for seeded topic(s), vectors are persisted for each chunk and re-runs do not create inconsistent duplicates.
4. Retrieval correctness: query returns only chunks for active `topic_id`, with deterministic top-k ordering and parent expansion producing coherent section context.
5. API regression: `AskAI` response JSON keys and error contract remain unchanged for frontend compatibility.
6. End-to-end smoke: Reader Ask AI returns grounded answer + citations; offline/API-unavailable states still fail explicitly per product rules.
7. Packaging validation: production artifact includes required asset DLL/model/tokenizer files and works on a clean Windows machine.

**Decisions**
- In scope: backend retrieval-engine migration to ONNX + sqlite-vec, build/runtime constraints, deterministic topic-scoped retrieval, and compatibility-preserving API behavior.
- Out of scope: chatbot memory, cross-topic retrieval mode, V2 adaptive reranking logic, and major frontend redesign.
- Assumption: embedding dimensionality is fixed per current `asset/model_int8.onnx`; vec schema will be aligned to that discovered dimension.

**Further Considerations**
1. Indexing Strategy Recommendation
Go with Option B (Incremental checksum-based reindex).
Because this application is designed to ingest large textbooks, a full re-index on every startup (Option A) will quickly result in unacceptable latency and CPU spiking, ruining the local desktop experience. You can manage this cleanly in SQLite: hash the raw text of the parent sections upon ingestion and store that hash. On startup, check the hashes; if they match what is already in the database, skip the embedding pipeline entirely.

2. Extension-Loading Strategy Recommendation
Go with Option B (Copy to app-data and load absolute path).
Because you are building a compiled, portable desktop binary, relying on a relative asset/ path (Option A) is extremely fragile in production. Depending on how the OS executes the binary or how it is packaged, the working directory may not be what you expect. A robust pattern for Go desktop applications is to embed the vec0.dll and onnxruntime.dll directly into the binary (using go:embed if possible, though large DLLs might require a separate installer payload), extract them to a known, stable directory in the user's AppData folder on first launch, and load them using strict absolute paths.

3. Validation Mode Recommendation
Go with Option B (Disable Ask AI features with clear UI state).
While Option A (hard-fail) enforces strict deterministic behavior, it creates a hostile user experience. If a local dependency fails to load (e.g., an antivirus temporarily locks the DLL), the user should not be locked out of the entire application. The core "Reader" and "Flashcards" features should remain fully functional. The "Ask AI" button should simply be disabled with a clear, explicit tooltip explaining that the local AI engine failed to initialize, preserving the transparent nature of your architecture without breaking the app.