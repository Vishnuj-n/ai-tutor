# AI Tutor RAG Architecture

**Legacy term note:** Older docs referred to `blocks` and `block_vectors`. The live schema uses `parents` + `chunks` for content and an external/virtual embedding store (see `doc/SCHEMA.md` for mapping). This document uses the current names `chunks` / `parents` and refers to embedding storage as the RAG embedding store or `sqlite-vec` where applicable.

## 1. Purpose

### What

The retrieval-augmented generation layer powers contextual AI for the current topic only.

### Why

- Keep answers grounded in the material the learner is actively studying
- Prevent cross-topic drift and conversational behavior
- Preserve predictable latency, cost, and token usage

### How

- Retrieve only from the active `topic_id`
- Expand matched child chunks to their parent sections before prompting
- Build a single-turn prompt and send one stateless LLM request

## 2. Scope and Boundaries

### What

RAG is used for contextual explanation and topic-scoped assistance, primarily in Reader and Flashcards Explain flows.

### Why

The app is a guided tutor, not a chatbot. Retrieval must support the learning flow instead of replacing it.

### How

- Allowed: ask for clarification on the active topic, explain a flashcard, summarize a section, answer content-specific questions
- Not allowed: free-form general chat, long-lived memory, cross-topic search by default, autonomous multi-step research

## 3. Retrieval Inputs

### What

The pipeline consumes:

- Active `block_id` (alias for chunk id) from current task context
- User question or explain request
- Topic content from `chunks` table (sliding window chunks)
- Token budget and output constraints

### Why

RAG must be deterministic about what it can see and how much it can send to the model.

### How

- The UI sends the active `block_id` with the request (from current task)
- Backend validates that the block exists
- Retrieval queries the RAG embedding store (sqlite-vec virtual table) filtered by `block_id` scope
- Return full block content for context (no parent expansion needed with sliding window)

## 4. Content Structure

### What

Source material is stored in **chunks** (with `parents` for section headings) created by **sliding window chunking**:

- **Chunk**: Content unit of ~2500 words with 200-word overlap
- **Storage**: `chunks` table with `parent_id` referencing `parents` for section headings
- **Retrieval**: Top-k chunks via the RAG embedding store (sqlite-vec) within `block_id` scope

### Why

We intentionally simplified from semantic chunking:

- **Deterministic**: No AI involvement in boundary decisions
- **Inspectable**: Easy to verify chunk contents
- **Sufficient**: MVP does not require semantic boundaries
- **Removed**: LLM-drafted boundaries, parent-child hierarchy, semantic chunking

### How

**Sliding Window Chunking:**
```
Text → [2500 words] → [2500 words with 200 overlap] → [next 2500 words]...
```

**Storage in `chunks` table:**

| Field | Purpose |
|-------|---------|
| `id` | Unique block identifier |
| `topic_id` | Parent topic reference |
| `block_type` | `CHUNK` |
| `content` | Text content |
| `word_count` | For progress tracking |
| `order_index` | Sequence within topic |
| `start_page`, `end_page` | Page provenance |

**Retrieval scope:**
- Retrieve from the RAG embedding store (sqlite-vec virtual table)
- Filter by `block_id` (chunk id from active task context)
- Expand to full chunk content before prompt assembly

**What changed:**
- Removed: parent-child hierarchy, semantic boundaries, LLM-drafted sections
- Added: sliding window, uniform block storage, simpler retrieval

## 5. Retrieval Pipeline

### What

The pipeline is a single pass from query to response.

### Why

Simple control flow is easier to debug and keeps AI behavior predictable.

### How

```text
User question
  -> validate active topic
  -> embed query
  -> search topic-scoped child chunks
  -> ApplyHeuristicScoring (V1: no-op/basic boost, V2: weak-area boosting)
  -> select top-k matches
  -> expand matches to parent sections
  -> assemble prompt within token budget
  -> call OpenAI-compatible model once
  -> return answer with section labels/citations
```

Heuristic scoring contract:

- `ApplyHeuristicScoring` must be a named pipeline step, even if minimal in V1
- V1 behavior can be pass-through or simple deterministic boosts
- V2 plugs in learner-state-aware ranking (for example weakness-based boost)

## 5.1 Vector Storage and Retrieval Implementation

### What

Embeddings are stored in a `sqlite-vec` virtual table (RAG embedding store). Retrieval is simplified with chunk-based scope.

### Why

- SQLite extensions are connection-scoped, single persistent connection required
- The `sqlite-vec` virtual table requires integer rowids
- Simplified retrieval: no parent expansion needed with sliding window chunks

### How

**Storage:**
- Single SQLite connection with vec0 extension loaded (`db.Init()`)
- Embeddings stored in the sqlite-vec virtual table (RAG embedding store); relational rows reference chunk ids and maintain metadata

**Retrieval (Simplified):**
1. Get `block_id` (chunk id) from current task context
2. Query the sqlite-vec embedding store for that chunk's vector
3. Calculate similarity to query embedding
4. Return chunk content directly (no parent expansion)

**Changes from previous architecture:**
- Removed: two-step pre-filtering, parent expansion, page_num bounds checking
- Simpler: direct block lookup by `block_id`

**Architectural Constraints:**
- Connection pool fixed at 1 (`SetMaxOpenConns(1)`)
- Embeddings JSON-serialized before SQL binding

## 6. Prompt Assembly

### What

Prompt assembly combines the user request with the minimum supporting context needed to answer well.

### Why

The model should see enough material to stay grounded, but never exceed the token budget or receive unnecessary context.

### How

Prompt payload should include:

- User question
- Active topic metadata
- Retrieved parent sections or section excerpts
- Output instructions

Embedding metadata requirements (ingestion-time):

- Persist `topic_id`, `parent_id`, and `id` in SQLite chunk rows.
- Persist vectors in sqlite-vec by integer SQLite rowid, resolved from relational `id`.
- Keep metadata minimal but sufficient for fast topic-filtered retrieval.

Prompt rules:

- Keep only the most relevant sections
- Remove duplicate context where child hits map to the same parent
- Enforce a strict max token budget before the API call
- Prefer concise answers unless the UI explicitly requests a longer explanation

## 7. Token Budgeting

### What

Token budgeting limits how much context is assembled for the model.

### Why

This prevents truncated prompts, wasted spend, and unstable responses.

### How

- Reserve tokens for the model response first
- Allocate the remainder to retrieved context
- Drop lower-ranked chunks when the budget is exceeded
- Prefer fewer high-signal parent sections over many shallow fragments

Practical rule:

- If a section cannot fit in the remaining budget, do not partially force it in unless the parser can trim it cleanly

## 8. Answer Behavior

### What

RAG responses are grounded explanations, not open-ended chat history.

### Why

The learner should get a direct answer tied to the current topic and the source material.

### How

- Cite or label the section used for the answer when possible
- Keep the answer focused on the user question
- Ask the user to return to the Reader if the topic context is insufficient
- Avoid inventing knowledge not present in retrieved context unless the product explicitly allows brief synthesis

## 9. Failure Modes

### What

RAG can fail because of missing topic context, retrieval problems, or model/API unavailability.

### Why

Users need clear feedback instead of hidden fallback behavior.

### How

- If no active topic exists, stop and show a clear guidance message
- If retrieval returns nothing useful, state that the topic content is insufficient for the request
- If the AI API is unavailable, show an explicit online-required error
- Never fabricate an answer or silently switch to a different topic

## 10. What RAG Does Not Do

### What

These are deliberate exclusions.

### Why

The app stays simpler, more predictable, and easier to maintain.

### How

- No global knowledge search across all topics
- No chat memory between requests
- No agent planning or multi-step tool use
- No background autonomous retriever that rewrites study flow

## 11. Related Data

### What

The retrieval layer depends on the content stored as `chunks` (and `parents` for section headings) and on the RAG embedding store (sqlite-vec virtual table).

### Why

RAG should be traceable back to the source material and the current study state.

### How

- `chunks` table stores content; `parents` holds section headings and hierarchy
- Embeddings live in the sqlite-vec virtual table (RAG embedding store) and are referenced from `chunks` via `embedding_ref`
- Current task provides `block_id` (a chunk id) for scoped retrieval
- UI shows chunk reference for traceability

**Schema implementation:** See `internal/db/schema.go` for the authoritative table definitions and `internal/rag` for the embedding/retrieval implementation.

**Note:** Previous `importance_score` and `weakness_score` hooks removed. Scoring now handled by FSRS state on cards or quiz results.

## 12. Local Embedding Pipeline (Implementation Plan)

### What

Embeddings are generated locally with ONNX Runtime and stored in SQLite + `sqlite-vec`.

### Why

- Keeps the full RAG stack local-first and portable.
- Removes dependency on external vector database services.
- Supports deterministic retrieval with transparent SQL-level inspection.
- Auditability and bias mitigation: unlike opaque "database-does-AI" extensions, separating ONNX embedding from SQLite vector indexing keeps tokenization and retrieval fully controllable, deterministic, explainable, and auditable.

### How

Step 1: Tokenize text with `asset/tokenizer.json`

- Use a tokenizer compatible with Hugging Face tokenizer JSON format.
- Apply the same tokenizer for document chunks and user queries.
- Recommended implementation: use `github.com/daulet/tokenizers` (CGO wrapper over Hugging Face tokenizers) to parse `asset/tokenizer.json` directly in Go.

Step 2: Generate embeddings with ONNX

- Use `yalue/onnxruntime_go` to load `asset/model_int8.onnx`.
- Build tensors for token IDs and attention mask.
- Run inference and extract a fixed-size embedding vector.

Step 3: Persist in SQLite + `sqlite-vec`

- Store chunk text and metadata in relational tables.
- Store vectors in a `sqlite-vec` virtual table (for example `vec0`).
- Keep a stable key mapping so vector rows map 1:1 to chunk rows.

Step 4: Retrieve top-k for active topic (Two-Step Fast Retrieval)

- Embed user query using the same tokenizer and ONNX model.
- Step A: Pre-filter target `rowid`s by querying `topic_id` and `page_num` boundaries from the `chunks` table.
- Step B: Execute vector similarity search on the `sqlite-vec` virtual table, restricted to the pre-filtered `rowid` set (avoiding virtual table joins).
- Expand child hits to parent sections before prompt assembly.

Step 5: Generate answer

- Build a token-budgeted prompt from retrieved parent sections plus the user question.
- Call the configured OpenAI-compatible LLM once (stateless).
- Return answer plus section labels/citations.

## 13. Windows Runtime Assets

Required for local Windows builds (these must be physically present in the `asset/` folder):

- `asset/onnxruntime.dll`
- `asset/vec0.dll`

If either dependency is missing from the `asset/` folder, ingestion/retrieval must fail with an explicit setup error instead of synthetic fallback output.

## 14. Build and Compilation Constraints

### What

The Go application relies on C bindings to interact with local runtime libraries.

### Why

Both `onnxruntime_go` and `sqlite-vec` operate outside pure Go memory for performance-critical inference and vector search.

### How

- CGO required: build with `CGO_ENABLED=1`.
- SQLite extension loading: compile with sqlite extension support, for example `go build -tags sqlite_extension .`.