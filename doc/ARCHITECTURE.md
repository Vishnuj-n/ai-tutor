# AI Tutor Architecture

## 1. Architecture Goals

### What

A local-first desktop tutoring system with deterministic workflows and topic-scoped AI reasoning.

### Why

- Privacy and reliability depend on local persistence
- Guided learning quality depends on strict workflow control
- Solo development requires low-complexity architecture

### How

- Go + Wails host core services and desktop runtime
- Vue multi-page UI invokes typed backend commands
- SQLite is the source of truth for study state
- SQLite + sqlite-vec store and query topic-scoped embeddings locally
- ONNX Runtime is used for local embedding inference via `yalue/onnxruntime_go`
- OpenAI-compatible API is used only for reasoning tasks

## 2. High-Level Component Design

### What

Core components:

- Desktop shell and backend services
- Frontend pages and sidebar navigation
- Local data layer (SQLite + embedding index)
- LLM provider adapter
- Scheduler services (reading + FSRS)

### Why

Separates concerns clearly while keeping boundaries simple.

### How

- UI sends command-style requests to backend
- Backend executes retrieval, scheduling, and persistence
- AI requests are stateless and scoped to current topic only

## 3. Frontend Structure (Vue Multi-Page)

### What

Sidebar sections:

1. Dashboard
2. Reader
3. Quiz
4. Flashcards
5. Socratic Tutor
6. Settings (bottom)
7. Sync button (bottom)

### Why

Enforces the guided flow and keeps AI contextual rather than conversational.

### How

- Dashboard reads daily task queue from scheduler service
- Reader renders parsed sections with Ask AI panel
- Quiz loads topic quiz sets and shows generation status
- Flashcards run FSRS reviews and optional Explain
- Settings stores provider config securely in local app config

## 4. Data Model

### What

Relational structure with JSON extensions.

### Why

- SQL tables give strong queryability for scheduling and progress
- JSON keeps quiz and card payloads flexible

### How

Suggested schema:

- topics
  - id, title, status, source_ref, created_at, updated_at
- parents
  - id, topic_id, heading, order_index, content_text
- chunks
  - id, topic_id, parent_id, chunk_text, token_count, embedding_ref
- quiz_sets
  - id, topic_id, version, payload_json, created_at
- topic_progress
  - topic_id, learned_at, last_read_at, mastery_score, review_enabled
- fsrs_cards
  - id, topic_id, prompt, answer, state_json, due_at
- app_events (optional, prunable)
  - id, event_type, payload_json, created_at

## 5. Chunking and Retrieval

### What

Hybrid chunking with parent-document retrieval extension.

### Why

- Heading-aware chunks preserve semantic boundaries
- Token fallback prevents oversized or malformed sections
- Parent expansion gives coherent context without full-document load

### How

1. Parse source into heading-based parent sections.
2. Create child chunks from each parent section.
3. If a section exceeds token target, split by token budget.
4. Tokenize chunk text using `asset/tokenizer.json`.
5. Generate embeddings with `asset/model_int8.onnx` via ONNX Runtime.
6. Persist vectors in a `sqlite-vec` virtual table and keep chunk metadata in SQLite relational tables.
5. On retrieval, fetch top-k child chunks then expand to parent sections.

## 6. RAG Pipeline (Topic-Scoped)

### What

Deterministic single-turn pipeline for Ask AI and Explain use cases.

### Why

Maintains control, cost, and predictable behavior.

### How

1. Validate active topic context.
2. Embed the user query.
3. Retrieve top-k chunks within topic scope.
4. Expand chunk hits to parent sections.
5. Build a structured prompt with:
   - User question
   - Topic metadata
   - Retrieved context blocks
   - Output constraints
6. Execute one LLM request.
7. Return response with citations/section labels.

Constraints:

- No global retrieval by default
- Strict token budget at prompt assembly stage
- Stateless requests, no conversation memory

## 6.1 Local Embedding Runtime Dependencies

### What

The embedding pipeline depends on local model/runtime assets.

### Why

Embedding generation must be deterministic and available without external vector services.

### How

- Required assets:
  - `asset/tokenizer.json`
  - `asset/model_int8.onnx`
  - `asset/onnxruntime.dll` (Windows runtime)
  - `asset/vec0.dll` (sqlite-vec extension on Windows builds)
- At startup, validate these assets before enabling ingestion/retrieval features.
- If a required local dependency is missing, show explicit setup guidance and fail clearly.

## 6.2 SQLite Connection Pool and vec0 Extension Management

### What

SQLite database maintains a single persistent connection with the sqlite-vec (vec0) extension loaded.

### Why

SQLite extensions are connection-scoped. If the application opens multiple DB connections (via pooling), only the first connection will have the extension loaded. Subsequent connections will fail to access the vec0 virtual table with "no such module: vec0" errors.

### How

- **Single Connection Pool:** `SetMaxOpenConns(1)` and `SetMaxIdleConns(1)` enforce exactly one active database connection.
- **Extension Loading:** At `db.Init()`, the SQLite connection loads the vec0 extension via driver-level `sqliteConn.LoadExtension()` (not SQL `LOAD_EXTENSION`).
- **Vector Table Storage:** All vectors are stored in a vec0 virtual table with integer rowids (not string IDs). Application chunk IDs are mapped to SQLite rowids before insert/query operations.
- **Vector Serialization:** Float32 embedding vectors are serialized to JSON strings before binding to database parameters, since `database/sql` does not support slice types directly.

**Architectural Constraints:**
- Do not increase `MaxOpenConns` from 1; this is a permanent requirement.
- All vector operations must resolve string chunk IDs to integer rowids first (via `lookupChunkRowID()`).
- All embeddings must be JSON-serialized before DB binding (via `vectorToJSON()`).

**Resource Cleanup:**
- Call `db.Close()` in test cleanup handlers to release the connection before temp directory removal (prevents Windows file lock errors).
- On application shutdown, the connection is automatically closed by the database driver.

## 7. Scheduling System

## 7.1 Reading Scheduler

### What

Topic lifecycle: unseen -> reading -> learned.

### Why

Guarantees a manageable intake of new material.

### How

- Daily cap for new topics: 1 to 3
- Move topic to reading when user starts Reader flow
- Move topic to learned on explicit Mark as Learned action

## 7.2 FSRS Review Scheduler

### What

Review scheduling for generated flashcards after learning.

### Why

Improves retention while minimizing review overload.

### How

- Activate FSRS cards only when topic status is learned
- Map grading buttons deterministically:
  - Again -> low recall
  - Hard -> partial recall
  - Good -> expected recall
  - Easy -> strong recall
- Use conservative intervals for early learning stages

Daily priority order:

1. Due reviews
2. New reading topics
3. Optional exploration

## 8. LLM Layer

### What

Minimal provider interface for OpenAI-compatible APIs.

### Why

Supports provider switching without framework overhead.

### How

Provider config fields:

- base_url
- api_key
- model
- timeout_ms

Interface operations:

- generate_answer(prompt)
- generate_quiz(topic_context)

Non-goals:

- No LangChain
- No autonomous agents
- No multi-step orchestration framework

## 9. Offline Strategy

### What

Offline-first core with explicit online-only AI operations.

### Why

Users must keep studying even without network access.

### How

Offline enabled:

- Reading structured content
- FSRS review cycles
- Daily scheduling and progress tracking

Online required:

- Ask AI
- Quiz generation

Failure mode:

- Immediate, explicit UI error
- No hidden fallback models
- No synthetic placeholder answers

## 10. Retention Policy

### What

Keep durable learning state, prune transient operational artifacts.

### Why

Preserves learning continuity while controlling local growth.

### How

Retain:

- FSRS card state
- Topic progress
- User-facing summaries

Prune:

- Debug logs
- Intermediate AI outputs
- Temporary retrieval traces

## 11. Task-to-Page Execution Contract

### What

Dashboard tasks must open target pages with context preloaded.

### Why

A guided tutor must convert recommendations into immediate action.

### How

- Task includes action_type and topic_id
- Router navigates to page with route params
- Target page resolves data and shows loading state when needed
- Example:
  - Task: Quiz for Topic 1
  - Click -> Quiz page opens with Topic 1 quiz preloaded or loading indicator until ready
