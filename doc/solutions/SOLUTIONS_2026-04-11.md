# Solutions Log - 2026-04-11

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

## Notes
- Current notebook behavior is by design: uploads without selected topic are stored as `uploaded_unlinked` and do not create chunks/index vectors.
- To enable RAG retrieval for a notebook, upload with a linked topic (or implement a post-upload link+ingest action).
