# AI Tutor RAG Architecture

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

- Active topic context
- User question or explain request
- Topic content stored as parent sections and child chunks
- Token budget and output constraints

### Why

RAG must be deterministic about what it can see and how much it can send to the model.

### How

- The UI sends the active topic identifier with the request
- Backend validates that the topic exists and is eligible for retrieval
- Retrieval queries only the embeddings associated with that topic
- Each chunk uses one canonical string `chunk_id` in relational tables, mapped to integer SQLite rowids for `sqlite-vec` storage.

## 4. Content Structure

### What

Source material is stored in a parent-child retrieval layout:

- Parent section: heading-level or section-level content block
- Child chunk: smaller embedded unit used for similarity search

### Why

- Parent sections preserve readable context
- Child chunks improve recall and semantic matching
- Parent expansion prevents awkward fragment-only answers

### How

- Parse content into heading-aware parent sections first
- Split oversized sections with token-aware fallback chunking
- Assign a stable, unique chunk_id to every child chunk at ingest time
- Store embeddings on child chunks
- Keep parent and topic references so retrieval can return original section text and enforce scope

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

Embeddings are stored in a `sqlite-vec` virtual table with integer rowids and JSON serialization.

### Why

- SQLite extensions are connection-scoped, so a single persistent connection is required.
- The `sqlite-vec` virtual table requires integer rowids, not string IDs.
- `database/sql` parameter binding requires concrete Go types; float32 slices must be serialized.

### How

**Storage:**
- Application maintains a single SQLite connection with vec0 extension loaded (via `db.Init()` and connection pool constraints).
- Each chunk has a stable string `chunk_id` stored in the `chunks` table.
- The `chunk_vectors` table maps chunk IDs to integer SQLite rowids and stores embeddings as JSON strings.
- On insert, `UpsertChunkVector()` resolves the string chunk_id to its integer rowid before inserting into vec0.

**Serialization:**
- `vectorToJSON()` converts float32 slices to compact JSON strings before passing to database parameters.
- This avoids database/sql type binding errors and keeps the storage format compatible with direct SQL inspection.

**Retrieval:**
- `SearchVectorsForTopic()` embeds the query, searches the vec0 table for cosine-distance matches within the topic, returns matching chunk IDs and distances.
- Results are joined with chunks/parents to populate context for prompt assembly.
- Integer rowid-to-chunk_id mapping is transparent to the RAG pipeline layer.

**Architectural Constraints:**
- Connection pool is fixed at 1 active connection (`SetMaxOpenConns(1)`). Do not change this.
- String chunk IDs must always be resolved to integer rowids before vec0 operations.
- Embeddings must always be JSON-serialized before SQL binding.

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

- Persist `topic_id`, `parent_id`, and `chunk_id` in SQLite chunk rows.
- Persist vectors in sqlite-vec by integer SQLite rowid, resolved from relational chunk_id.
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

The retrieval layer depends on the topic, parent, and chunk records stored locally.

### Why

RAG should be traceable back to the source material and the current study state.

### How

- Topic records identify the active scope
- Parent records store human-readable section text
- Chunk records store identifiers, retrieval metadata, and scoring hooks
- The UI uses the returned section labels to show where the answer came from

SQLite schema hooks (required now to avoid later migrations):

- Keep scoring columns on chunk-level records, including:
  - `importance_score`
  - `weakness_score`
- Keep `topic_id` and `parent_id` persisted with each chunk row
- Treat these fields as forward-compatible hooks in V1 (they may be default/unused initially)

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

Step 4: Retrieve top-k for active topic

- Embed user query using the same tokenizer and ONNX model.
- Execute topic-scoped vector similarity search (cosine or equivalent supported metric).
- Expand child hits to parent sections before prompt assembly.

Step 5: Generate answer

- Build a token-budgeted prompt from retrieved parent sections plus the user question.
- Call the configured OpenAI-compatible LLM once (stateless).
- Return answer plus section labels/citations.

## 13. Windows Runtime Assets

Required for local Windows builds:

- `asset/onnxruntime.dll`
- `asset/vec0.dll`

If either dependency is missing, ingestion/retrieval must fail with an explicit setup error instead of synthetic fallback output.

## 14. Build and Compilation Constraints

### What

The Go application relies on C bindings to interact with local runtime libraries.

### Why

Both `onnxruntime_go` and `sqlite-vec` operate outside pure Go memory for performance-critical inference and vector search.

### How

- CGO required: build with `CGO_ENABLED=1`.
- SQLite extension loading: compile with sqlite extension support, for example `go build -tags sqlite_extension .`.