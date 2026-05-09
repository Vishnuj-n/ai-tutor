# AI Tutor Architecture

## 1. Architecture Goals: Persistent Queue Model

### What

A **Persistent Guided Study Queue** - NOT an autonomous AI tutor, hidden orchestration engine, or proactive scheduling system. The queue is the recommended guided progression path, but manual and exploratory entry points are intentionally supported when they reuse the same canonical bootstrap and topic ownership semantics.

Advanced learning systems (quizzes, FSRS, remediation) are treated as **"Data, not Engines."** They create queue tasks but do NOT control orchestration directly.

**SQLite is the source of truth.**

### Why

- **Deterministic**: Predictable, inspectable flow
- **Debuggable**: Queue state is queryable SQL
- **Resumable**: No runtime-only state that vanishes on restart
- **Simple**: Solo development requires low-complexity architecture

### How

- Go + Wails host core services and desktop runtime
- Vue multi-page UI invokes typed backend commands
- **SQLite `study_queue` table drives all user flows**
- SQLite + sqlite-vec store topic-scoped embeddings locally
- ONNX Runtime for local embedding inference via `yalue/onnxruntime_go`
- OpenAI-compatible API for reasoning tasks only

---

## 1.1 The Queue Loop (Core Pattern)

```
┌─────────────┐     ┌──────────────┐     ┌─────────────┐
│  Dashboard  │────▶│  Fetch Next  │────▶│  Mount      │
│             │     │  PENDING Task│     │  Module     │
└─────────────┘     └──────────────┘     └─────────────┘
                                                 │
                    ┌──────────────┐            ▼
                    │  Insert      │◄────┌─────────────┐
                    │  Follow-up   │     │  Complete   │
                    │  Tasks       │     │  Task       │
                    └──────────────┘     └─────────────┘
```

The queue router ONLY, for queue-driven progression:
- Fetches next pending task from `study_queue` (deterministic ordering)
- Mounts correct module/view based on `task_type`
- Marks tasks complete
- Inserts follow-up queue tasks (explicit rules only)

Manual study entry points may invoke the same module bootstrap and retrieval helpers directly. They must not introduce separate lifecycle implementations.

The router does NOT:
- Manage hidden state machines
- Proactively schedule flows
- Own remediation logic
- Run autonomous pipelines
- Mutate queue in background without trigger

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

These pages can be opened either from a queue task or from a manual exploratory action; both paths should converge on the same initialization pipeline.

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

Relational structure with JSON extensions, centered on the **persistent queue**.

### Why

- SQL tables give strong queryability for scheduling and progress
- JSON keeps quiz and card payloads flexible
- **Queue persistence** enables resumable, debuggable flows

### Core Tables

**study_queue (NEW - The Central Queue)**
| Field | Type | Description |
|-------|------|-------------|
| `id` | TEXT PK | Unique task identifier |
| `task_type` | TEXT | `READING`, `QUIZ`, `REREAD`, `FLASHCARD_REVIEW`, `EXAMINER` |
| `block_id` | TEXT | Reference to content block (chunk, quiz_set, etc.) |
| `related_id` | TEXT | Optional related entity (topic_id, parent_id) |
| `status` | TEXT | `PENDING`, `ACTIVE`, `COMPLETED` |
| `priority` | INTEGER | Lower = higher priority |
| `created_at` | INTEGER | Unix timestamp |
| `completed_at` | INTEGER | Unix timestamp (NULL if pending) |

**Supporting Tables**

- `topics` - id, title, status, start_page, end_page, current_page_cursor, created_at
- `blocks` - id, topic_id, block_type, content, word_count, order_index
- `quiz_sets` - id, topic_id, block_id, payload_json, created_at
- `fsrs_cards` - id, topic_id, block_id, prompt, answer, state_json, due_at
- `app_events` (optional, prunable) - id, event_type, payload_json, created_at

### What the Queue Replaces

- Runtime-only queues
- Hidden orchestrators
- In-memory session engines
- Proactive scheduling systems
- Complex state machines

## 5. Chunking: Sliding Window (Deterministic)

### What

**Sliding Window Chunking** - deterministic, inspectable, sufficient for MVP.

### Why

We intentionally removed:
- Semantic topic chunking
- AI-generated chunk boundaries
- Advanced syllabus graphing
- Autonomous chunk orchestration

**Reason**: Deterministic chunking is simpler, inspectable, and sufficient for MVP.

### How

**Sliding Window Parameters:**
- **Chunk size**: 2500 words
- **Overlap**: 200 words between chunks

**Pipeline:**

1. PDF Upload → Extract text with page numbers
2. Chapter Selection → User reviews/prunes extracted chapters
3. Sliding Window Chunking → Deterministic boundaries (no AI)
4. **Insert READING tasks** → One task per chunk into `study_queue`

**Block Storage:**

| Field | Purpose |
|-------|---------|
| `id` | Unique block identifier |
| `topic_id` | Parent topic reference |
| `block_type` | `CHUNK`, `QUIZ`, `FLASHCARD` |
| `content` | Text content or JSON payload |
| `word_count` | For progress tracking |
| `order_index` | Sequence within topic |
| `start_page`, `end_page` | Page provenance |

### Retrieval

RAG pipeline remains topic-scoped:
1. Validate active topic context
2. Embed user query
3. Retrieve top-k chunks within `block_id` scope
4. Build prompt with retrieved context
5. Execute one LLM request

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

## 7. Scheduling: Queue-Driven (Simplified)

### What

**FSRS is a scheduling algorithm ONLY** - not an orchestrator, session manager, or hidden engine.

### Multi-Notebook Priority System

We officially support multiple notebooks with deterministic biasing:

- Notebooks have `priority INTEGER DEFAULT 5` (1-10 scale)
- Higher priority notebooks surface more frequently
- Lower priority notebooks still eventually appear
- Notebook priority is a **bias**, NOT absolute control

### Queue Ordering Rules

**Ordering is: deterministic → priority-biased → anti-starvation balanced**

**NOT:** adaptive scheduling, autonomous pacing, or AI-driven prioritization.

Explicit priority hierarchy with notebook biasing:

| Order | Task Type | Rationale |
|-------|-----------|-----------|
| 1 | `FLASHCARD_REVIEW` (due reviews) | Time-sensitive spaced repetition |
| 2 | `REREAD` (remediation) | Timely follow-up on failed material |
| 3 | `QUIZ` | Assessment after reading |
| 4 | `READING` | New material after obligations |
| 5 | `EXAMINER` | Optional advanced assessment |

**Deterministic Query-Time Rules:**
- Same `study_queue` state always produces same task order
- No runtime adaptation based on user behavior
- No AI-driven dynamic reprioritization
- Notebook priority is a static bias coefficient, not adaptive weighting

**Ordering Query:**
```sql
SELECT * FROM study_queue sq
LEFT JOIN notebooks n ON sq.notebook_id = n.id
WHERE sq.status = 'PENDING'
ORDER BY 
  CASE sq.task_type
    WHEN 'FLASHCARD_REVIEW' THEN 1
    WHEN 'REREAD' THEN 2
    WHEN 'QUIZ' THEN 3
    WHEN 'READING' THEN 4
    WHEN 'EXAMINER' THEN 5
  END,
  n.priority DESC,
  sq.priority ASC,
  sq.created_at ASC;
```

### How FSRS Integrates with Queue

1. When cards become **due** (per FSRS calculation):
   - Insert `FLASHCARD_REVIEW` task into `study_queue` (one task per block)
   - Set `priority` based on overdue duration

2. Dashboard queries `study_queue` with ordering rules above

3. User completes flashcard session → FSRS calculates next interval

4. New `FLASHCARD_REVIEW` task scheduled for future due date

### Task Lifecycle Semantics

Explicit state transitions:

```
PENDING → ACTIVE (when user opens task)
ACTIVE → COMPLETED (on successful completion)
ACTIVE → SKIPPED (on user bypass)
ACTIVE → FAILED (on quiz generation error)
```

**Crash Recovery:**
- ACTIVE tasks older than 30-minute timeout revert to PENDING on startup
- Ensures restart-safe queue recovery
- `activated_at` timestamp tracks activation time

### Dashboard Starvation Protection

To prevent review monopolization (e.g., 500 flashcards blocking reading):

**Deterministic Balancing Rule (Query-Time Only):**
After 5 review tasks (`FLASHCARD_REVIEW` or `REREAD`), surface 1 `READING` task.

- Implemented as SQL query logic, not background process
- No autonomous queue rebalancing
- No hidden scheduling daemon
- Explicit, inspectable, reproducible behavior

**Anti-Drift Safeguard:** Balancing rules are static SQL ordering constraints, not adaptive runtime systems. No behavioral learning, no dynamic pacing, no runtime adaptation.

### Reread Loop Protection

Maximum reread attempts: **3** (default)

- `reread_attempt` counter tracked per block
- After max reached: stop auto-inserting reread tasks
- Show recommendation message to user
- Allow manual retry if user chooses
- Continue queue progression

Prevents infinite queue pollution from remediation loops.

### Quiz Generation States

Explicit generation lifecycle for QUIZ tasks:

| State | Meaning |
|-------|---------|
| `GENERATING` | LLM call in progress |
| `READY` | Quiz ready for user |
| `FAILED` | Generation error |

**Flow:**
1. User completes reading
2. QUIZ task inserted with `GENERATING` state
3. LLM called synchronously
4. On success: `generation_status = READY`
5. On failure: `generation_status = FAILED` (dashboard surfaces explicitly)

**MVP Simplification Note:**
Generation status is colocated on the QUIZ task row. This intentionally mixes:
- Task lifecycle (`PENDING` → `ACTIVE` → `COMPLETED`)
- Generation lifecycle (`GENERATING` → `READY`/`FAILED`)

This is acceptable for MVP. Future refactoring may separate generation state to `quiz_sets` table.

### Flashcard Review Granularity

**One `FLASHCARD_REVIEW` task = one review session for a block/chunk.**

- Do NOT create one queue task per flashcard
- Single task represents "review all due cards in this block"
- Prevents queue explosion with many cards

### Task Priority Order (Legacy Reference)

| Priority | Task Type | Source |
|----------|-----------|--------|
| 1 | Overdue FLASHCARD_REVIEW | FSRS due date passed |
| 2 | PENDING QUIZ | Reading completion |
| 3 | PENDING READING | New material ingestion |
| 4 | REREAD (remediation) | Failed quiz |
| 5 | EXAMINER | Mastery threshold met |

### Examiner Task Policy

EXAMINER tasks:
- Inserted after mastery thresholds met (e.g., quiz scores > 80%)
- Assigned elevated queue priority (appear naturally in flow)
- Remain optional (user can skip)
- Appear through deterministic queue ordering, NOT hidden orchestration

Prevents starvation: EXAMINER tasks are tier 5 in priority hierarchy, ensuring reviews and reading are not blocked.

### Reading Validation

Minimal validation before allowing task completion:

- User must reach final assigned page (`current_page_cursor >= end_page`)
- Complete button disabled until validation passes
- No surveillance logic, reading timers, or engagement tracking
- Lightweight MVP approach

### Skip Semantics

Explicit terminal states preserve audit trail:

| Status | Meaning | Resurfacing |
|--------|---------|-------------|
| `COMPLETED` | Successfully finished | No |
| `SKIPPED` | User bypassed | Possible (manual retry) |
| `FAILED` | Error/generation failure | Can retry |

Skipped tasks are auditable and can resurface if needed. Do NOT silently mark skipped tasks as completed.

### No Proactive Scheduling

- No background workers scanning for "what's next"
- No autonomous flow engines
- Queue is the **only** source of next actions
- Deterministic MVP > premature optimization

## 8. LLM Layer: Synchronous Only

### What

Minimal provider interface for OpenAI-compatible APIs. **All generation is synchronous.**

### Why

- No background workers
- No async orchestration
- No hidden goroutines
- Deterministic MVP > premature optimization

### How

**Provider config fields:**
- base_url
- api_key
- model
- timeout_ms

**Synchronous Flow:**

| Step | Action |
|------|--------|
| 1 | User clicks Complete |
| 2 | Frontend shows loading spinner |
| 3 | Backend calls LLM synchronously |
| 4 | Content returned in response |
| 5 | Task inserted into `study_queue` |

**Interface operations:**
- `generate_answer(prompt)` - RAG responses
- `generate_quiz(topic_context)` - Quiz creation

**Non-goals:**
- No LangChain
- No autonomous agents
- No multi-step orchestration framework
- No async job queues

## 9. Offline Strategy

### What

Offline-first core with explicit online-only AI operations.

### Why

Users must keep studying even without network access.

### How

**Offline enabled:**
- Reading from `blocks` table
- FSRS review cycles (queue-driven)
- Queue progress tracking

**Online required:**
- Ask AI (RAG + LLM)
- Quiz generation (synchronous LLM call)

**Failure mode:**
- Immediate, explicit UI error
- No hidden fallback models
- No synthetic placeholder answers

**Queue Persistence Enables Offline:**
- `study_queue` is local SQLite
- Task state survives app restarts
- No runtime-only queues that vanish

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

## 10. Queue Router (Thin Task Router)

### What

The queue router is a **query-and-route layer**, not a flow engine or orchestration system.

### Responsibilities

The router ONLY:
1. **Fetch next pending task** from `study_queue` (using deterministic ordering rules)
2. **Mount correct module** based on `task_type`
3. **Pass context** (`block_id`, `related_id`) to module
4. **Mark tasks complete** when module signals completion
5. **Insert follow-up tasks** based on explicit completion rules

### What It Does NOT Do

- Manage hidden state machines
- Proactively schedule flows
- Own remediation logic
- Run autonomous pipelines
- Control dual timer engines
- Manage event buses

### Hard Invariant: No Background Queue Mutation

**"No background queue mutation without explicit trigger."**

All queue mutations MUST originate from:
- Explicit user actions (clicking complete, skip)
- Deterministic startup recovery (timeout stale ACTIVE tasks)
- Synchronous completion flows (task A completes → task B inserted)

**Prohibited:**
- Daemon loops scanning and modifying queue
- Auto-balancers running on timers
- Hidden startup repair jobs
- Autonomous queue injectors
- Event-driven queue mutation

### Example: Quiz Completion Flow

```
Quiz Module reports score: 60% (below threshold)
→ Queue router marks QUIZ task COMPLETED
→ Queue router inserts REREAD task
→ Dashboard shows REREAD as next pending
```

User can complete or skip the REREAD task. The queue router does NOT force loops.

---

## 11. Technical Debt Strategy

### Context

Previous architecture review identified `app.go` and `notebook_endpoints.go` as potentially oversized coordination files.

### Current State

After cleanup and modularization work:
- `app.go`: ~600-700 lines (acceptable MVP scale)
- `notebook_endpoints.go`: ~600-700 lines (acceptable MVP scale)

### Decision

**Do NOT aggressively split them further during Sprint 1.**

Extract further only if:
- Duplication increases
- Navigation degrades
- Responsibilities become unclear

**Avoid premature fragmentation.**

### Acceptance Criteria

- Files remain under ~800 lines
- Clear separation of concerns is maintained
- No action required unless complexity metrics degrade

---

## 12. Task-to-Page Execution Contract

### What

Dashboard tasks open target pages with context preloaded.

### Why

A guided tutor must convert queue tasks into immediate action.

### How

1. Dashboard queries `study_queue` for next `PENDING` task
2. Task card displays `task_type` and context
3. User clicks task → Router navigates to module
4. Module receives `block_id` and `related_id` from task
5. Module loads content and renders

**Example:**
- Task: `QUIZ` with `block_id: "quiz-set-123"`
- Click → Quiz module mounts
- Quiz module loads quiz_set by `block_id`
- User completes → Queue router marks complete → Next task appears
