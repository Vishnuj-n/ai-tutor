# AI Tutor Architecture

## Technology Stack

| Layer | Technology | Reason |
|---|---|---|
| Desktop shell | Wails v2 (Go + WebView) | Native window, file system access, no Electron overhead |
| Backend language | Go | Concurrency for greedy ingestion, go-fsrs library |
| Frontend framework | Vue.js | Simple, reactive framework, Wails support |
| Database | SQLite (via modernc/sqlite) | Embedded, zero-dependency, single file |
| FSRS engine | go-fsrs | Correct FSRS v4 implementation |
| PDF extraction | pdftotext (poppler-utils) | -layout flag preserves prose structure |
| LLM interface | HTTP (OpenAI-compatible) | Works with OpenAI, Anthropic, or any local proxy |

## Layer Diagram

```
┌─────────────────────────────────────────────────────────────┐
│                    React Frontend (WebView)                   │
│                                                              │
│  components/     hooks/        stores/        lib/           │
│  reader          useSession    sessionStore   wailsBindings  │
│  quiz            useTimer      fsrsStore      api            │
│  flashcard       useFSRS       notebookStore                 │
│  dashboard       useQuiz                                     │
│  settings        types/                                      │
│  shared          mission / notebook / fsrs / quiz / settings │
└──────────────────────────┬──────────────────────────────────┘
                           │  window.go.*  (Wails bridge)
                           │  Only wailsBindings.ts crosses this line
┌──────────────────────────▼──────────────────────────────────┐
│                       app.go (Wails bindings)                │
│               All exposed Go methods live here               │
└──┬──────────┬──────────┬──────────┬───────────┬────────────┘
   │          │          │          │           │
   ▼          ▼          ▼          ▼           ▼
orchestr-  scheduler  tutor/    parser/     fsrs/
ator/      session_   client    pdf.go      engine.go
engine.go  blocks.go  pipeline  cleaner     scoring.go
velocity   quota.go   prompts   syllabus
remediat.  alerts.go  retry     ocr.go
   │          │          │          │           │
   └──────────┴──────────┴──────────┴───────────┘
                           │
┌──────────────────────────▼──────────────────────────────────┐
│                     internal/db                              │
│          schema.sql  queries.go  migrations.go               │
│                       SQLite                                 │
└─────────────────────────────────────────────────────────────┘
```

## Go Package Responsibilities

### `app.go` — Wails Bridge
The single translation layer between React and Go. Every method here maps directly to a UI action. No business logic lives here — it delegates immediately to the relevant internal package.

### `internal/orchestrator`
- Task orchestration and agenda building
- Manages daily task prioritization

### `internal/scheduler`
- FSRS scheduling logic
- Daily planning and quota management
- Session block tracking

### `internal/db`
- `schema.sql` — Canonical schema. See Schema section below.
- `queries.go` — All SQL. No raw queries outside this file. Exports typed Go structs.
- `migrations.go` — Version table, sequential forward migrations only.

### `internal/llm`
- LLM provider abstraction
- OpenAI-compatible HTTP interface
- Handles auth, timeouts

### `internal/study`
- Study session management
- Quiz generation and scoring
- Flashcard generation and review
- Reading session completion

### `internal/notebook`
- Notebook ingestion and management
- PDF parsing
- Topic and section extraction

### `internal/embeddings`
- ONNX-based embedding generation
- Local model runtime management

### `internal/rag`
- RAG pipeline for Ask AI
- Vector indexing and retrieval

### `internal/retrieval`
- Retrieval engine for Socratic mode
- Topic-scoped search

### `internal/runtime`
- Asset validation
- Runtime preparation

### `internal/models`
- Shared data models

### `internal/subtopic`
- Subtopic processing

### `internal/utils`
- Shared utilities

## Frontend Architecture Rules

1. **Wails bindings bridge Go and Vue**. The `app.go` file exposes methods that Vue components call via Wails runtime.

2. **Components are organized by feature**. Each page has its own component structure.

3. **State management**. Vue's reactivity system manages component state.

4. **Business logic in Go**. The frontend is thin; most logic lives in Go services.

## Concurrency Model

```
Main goroutine          Background goroutine        Background goroutine
(session flow)          (greedy ingestion)           (Phase 2 — break)

Reader displayed  ────► Parse next mission PDF   ──►  Generate flashcards
                        Generate next quiz             Generate examiner Q
                        Store in SQLite                Store in SQLite
                                                       Timeout: 5 min
                                                       On timeout: queue
                                                         for tomorrow
```

Greedy ingestion and Phase 2 write to separate SQLite rows with a `status` column (`pending` / `ready` / `failed`). The orchestrator only serves missions where the quiz status is `ready`.

## LLM Call Structure

```
POST {llm_endpoint}/v1/chat/completions
{
  "model": "{model_name}",
  "messages": [
    { "role": "system", "content": "{reader_level_prompt}" },
    { "role": "user",   "content": "CONTEXT (previous 500 words or chapter title):\n{context}\n\nLESSON:\n{mission_text}\n\nTASK:\n{quiz_or_flashcard_instruction}" }
  ]
}
```

Response is parsed for JSON containing questions array. If JSON parse fails or count < 2: retry once.

## Data Flow: PDF → Mission → Quiz

```
Upload PDF
  └─► parser/pdf.go           pdftotext -layout → raw text file
  └─► parser/cleaner.go       Join hyphens, strip headers → clean paragraphs
  └─► parser/chunker.go       Build blocks ~2500 words, track page ranges
  └─► parser/syllabus.go      Detect chapter boundaries → tag blocks
  └─► db/queries.go           Store blocks with start_page, end_page, chapter_tag

Orchestrator picks notebook
  └─► velocity.go             Calculate score, pick winner
  └─► engine.go               Find next unread block
  └─► tutor/pipeline.go       Phase 1: send block → receive quizzes
  └─► db/queries.go           Store mission + quizzes (status: ready)

Session serves mission
  └─► app.go                  GetNextMission() → React
  └─► Reader.tsx              Display block text, lock boundary
  └─► QuizGate.tsx            Serve questions after read
  └─► useQuiz.ts              Score → wailsBindings → fsrs/scoring.go
  └─► fsrs/engine.go          Update card stability in SQLite
```

---

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

## 3. Frontend Structure (Vue.js)

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
  - id, title, status, start_page, end_page, current_page_cursor, created_at, updated_at
- parents
  - id, topic_id, heading, order_index, content_text
- chunks
  - id, topic_id, parent_id, page_num, chunk_text, token_count, embedding_ref
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

Context-Locked Session Architecture with LLM-drafted boundaries and exact page provenance.

### Why

- Automated regex chapter extraction was dropped in favor of reliable LLM-drafted boundaries.
- `page_num` provenance provides exact location tracking for chunks.
- Parent expansion gives coherent context without full-document load.

### How

1. Parse source into parent sections using LLM-drafted boundaries (dropping automated regex parsers).
2. Create child chunks from each parent section, recording exact `page_num` provenance.
3. If a section exceeds token target, split by token budget.
4. Tokenize chunk text using `asset/tokenizer.json`.
5. Generate embeddings with `asset/model_int8.onnx` via ONNX Runtime.
6. Persist vectors in a `sqlite-vec` virtual table and keep chunk metadata in SQLite relational tables.
7. On retrieval, fetch top-k child chunks then expand to parent sections.

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

The embedding pipeline depends on local model/runtime assets located in the `asset/` folder.

### Why

Embedding generation must be deterministic and available without external vector services.

### How

- Required assets (must be present in the `asset/` folder):
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
