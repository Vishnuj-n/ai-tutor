# DOCUMENT GOD FILE
Generated: 2026-05-09T18:27:18.708098

Purpose:
- Aggregate all documentation files from /doc
- Preserve file boundaries and locations
- Allow architecture review in AI chats
- Provide a single copy-paste context file

IMPORTANT:
- Original files inside /doc remain source of truth
- This file is generated automatically
- Do not manually edit this file



====================================================================================================
FILE: doc\AGENT_MAP.md
ABSOLUTE: C:\Users\vishn\PROJECT\ai-tutor\doc\AGENT_MAP.md
====================================================================================================

# Agent Map: Component Responsibilities

## Overview

Strict module boundaries for the Persistent Queue Architecture. Each module has exactly one responsibility. The queue router is intentionally thin—task routing only, no orchestration engine.

Canonical checkpoint flow:
Dashboard -> Reader -> Quiz -> Dashboard

Reader completes the reading task only. The backend generates and activates the QUIZ follow-up task, and the Dashboard regains ownership after quiz submission and evaluation. Any Reader -> Quiz handoff is queue-owned and applies only to generated follow-up quiz tasks.

**Orchestration Constraints:** See Queue Router section (below) for comprehensive list of prohibited orchestration behaviors. Individual modules focus on their specific responsibilities only.

---

## Queue Router (Thin Task Router)

**File:** `internal/orchestrator/service.go`

**Responsibility:** Route tasks between queue and modules. This is a lightweight query-and-route layer, not a flow engine.

**Does:**
- Query `study_queue` for next pending task (with deterministic ordering rules)
- Set task status to `ACTIVE` with `activated_at` timestamp when opened
- Mount correct module based on `task_type`
- Pass `block_id` and `related_id` to modules
- Mark tasks `COMPLETED`, `SKIPPED`, or `FAILED` on module signal
- Insert follow-up tasks per explicit rules (respecting max reread attempts)
- Crash recovery: reset stale ACTIVE tasks on startup (30-min timeout)
- Allow immediate activation of generated QUIZ follow-up tasks after Reader completion when they are the next pending queue item

**Explicitly Deterministic:**
- No adaptive scheduling
- No hidden state machines
- All behavior defined by query-time rules in SQL

**Does NOT:**
- Manage hidden state machines
- Proactively schedule flows
- Own remediation logic
- Run autonomous pipelines
- Control dual timers
- Manage event buses
- Route arbitrary module-to-module transitions

**API:**
```go
func GetNextTask() (*Task, error)
func CompleteTask(taskID string, result TaskResult) error
func GetTaskContext(taskID string) (*TaskContext, error)
```

---

## Reader Module

**File:** `frontend/src/pages/Reader.vue` + `internal/reader/`

**Responsibility:** Render PDF content for reading (execution surface only)

**Does:**
- Display content from `block_id`
- Open to `start_page` (authoritative entry point)
- Show assigned page range (`start_page` to `end_page`)
- Track reading progress (`current_page_cursor` for information only)
- Provide "Complete Session" button (always enabled during active task)
- Call "Complete" when user signals completion (trust-based)
- Provide "Ask AI" panel (RAG)
- Complete the reading task only

**Does NOT:**
- Generate quizzes
- Schedule next tasks
- Know about other modules
- Validate or gate completion
- Own progression semantics
- Enforce page-completion validation
- Route to other modules
- Require returning to Dashboard before a generated QUIZ task is mounted

Generated follow-up QUIZ tasks may be activated immediately after Reader completion through the queue loop only.

**API:**
```go
func GetBlockContent(blockID string) (*BlockContent, error)
func MarkBlockRead(blockID string, progress int) error
```

**Props from Queue Router:**
- `block_id`: Content to display
- `related_id`: Topic context
- `start_page`: Page to open (authoritative)
- `end_page`: Informational page bound

---

## Quiz Module

**File:** `frontend/src/pages/Quiz.vue` + `internal/quiz/`

**Responsibility:** Display and score quizzes

**Does:**
- Load quiz from `block_id` (quiz_set reference)
- Display questions
- Collect answers
- Calculate score
- Return pass/fail
- Handle `GENERATING`, `READY`, `FAILED` generation states
- Show explicit error for `FAILED` generation
- Drive queue follow-up outcomes after submission/evaluation

**Does NOT:**
- Generate quizzes (synchronous LLM call happens before task creation)
- Insert follow-up tasks
- Know about Reader module
- Silently handle generation failures
- Own workflow orchestration

**API:**
```go
func GetQuizSet(blockID string) (*QuizSet, error)
func SubmitQuiz(blockID string, answers []Answer) (*QuizResult, error)
```

**Props from Queue Router:**
- `block_id`: Quiz set to display
- `related_id`: Topic for context

**Returns to Queue Router:**
- Score (0-100)
- Passed (boolean)

---

## Flashcard Module

**File:** `frontend/src/pages/Flashcards.vue` + `internal/flashcards/`

**Responsibility:** Render and rate flashcards

**Does:**
- Load cards for review from `block_id` (one task = all due cards in block)
- Display card front
- Flip to show answer
- Capture rating (Again/Hard/Good/Easy)
- Send ratings to FSRS
- Complete task after reviewing all due cards in block

**Does NOT:**
- Calculate next review dates (FSRS does this)
- Create one task per flashcard
- Know about other modules

**API:**
```go
func GetDueCards(blockID string) ([]Card, error)
func RateCard(cardID string, rating Rating) error
```

**Props from Queue Router:**
- `block_id`: Card set to review

---

## FSRS Service

**File:** `internal/study/fsrs.go`

**Responsibility:** Scheduling algorithm only

**Does:**
- Calculate next review intervals
- Update card state
- Determine when cards are due
- Provide due card list

**Does NOT:**
- Orchestrate review sessions
- Insert queue tasks
- Manage UI state

**Note:** FSRS is a scheduling algorithm only. Queue coordination and task insertion are handled by the Queue Router.

**API:**
```go
func CalculateNextReview(currentState FSRSState, rating int) FSRSResult
func GetDueCards(topicID string) ([]Card, error)
func LogReview(cardID string, rating int) error
```

**Called By:**
- Queue Router (when creating review tasks)
- Flashcard module (when rating cards)

---

## Examiner Module

**File:** `frontend/src/pages/Examiner.vue` + `internal/examiner/`

**Responsibility:** Written assessments

**Does:**
- Display written assessment questions
- Capture written answers
- Submit for evaluation
- Show results

**Does NOT:**
- Trigger automatically
- Know about other modules

**API:**
```go
func GetAssessment(blockID string) (*Assessment, error)
func SubmitAssessment(blockID string, answers []Answer) (*AssessmentResult, error)
```

**Props from Queue Router:**
- `block_id`: Assessment to display

---

## Ingestion Pipeline

**File:** `internal/ingestion/` + `internal/chunking/`

**Responsibility:** PDF → Chunks → Queue

**Does:**
- Extract text from PDF
- Extract chapter boundaries
- Sliding window chunking (2500 words, 200 overlap)
- Create blocks in database
- Insert READING tasks into queue

**Does NOT:**
- Use AI for chunking
- Use semantic boundaries

**API:**
```go
func ProcessPDF(filePath string) (*ProcessingResult, error)
func CreateChunks(text string, topicID string) ([]Block, error)
func InsertReadingTasks(blocks []Block) error
```

---

## Dashboard Module

**File:** `frontend/src/pages/Dashboard.vue`

**Responsibility:** Display pending tasks with starvation protection

**Does:**
- Query queue router for next task (with multi-notebook priority biasing)
- Render task card with priority and notebook context
- Handle task click → route to module
- Show empty state when queue is clear
- Apply starvation protection (after N reviews, show reading)
- Surface quiz generation failures explicitly
- Regain ownership after quiz submission and evaluation

**Does NOT:**
- Calculate priorities (follows queue ordering rules)
- Schedule tasks
- Know about module internals

**API:**
```go
func GetNextTask() (*Task, error)
```

**Starvation Protection:**
- After 5 review tasks, surface 1 READING task
- Lightweight query-time bias (NOT autonomous orchestration)

---

## RAG / Ask AI Service

**File:** `internal/rag/pipeline.go`

**Responsibility:** Topic-scoped retrieval and answering

**Does:**
- Embed user query
- Retrieve chunks within topic scope
- Build prompt with context
- Call LLM
- Return answer

**Does NOT:**
- Cross-topic retrieval
- Maintain conversation memory

**API:**
```go
func AskQuestion(topicID string, question string) (*Answer, error)
func RetrieveContext(topicID string, query string, limit int) ([]Context, error)
```

---

## Database Layer

**File:** `internal/db/`

**Responsibility:** Data persistence

**Does:**
- CRUD for all tables
- Transaction management
- Query execution

**Does NOT:**
- Business logic

---

## Module Interaction Diagram

```
┌─────────────┐
│  Dashboard  │
└──────┬──────┘
       │ GetNextTask()
       ▼
┌─────────────┐     ┌─────────────────────────────────────┐
│   Queue     │────▶│ study_queue (SQLite source of     │
│    Router   │     │              truth)                 │
└──────┬──────┘     └─────────────────────────────────────┘
       │ Route by task_type
       ▼
┌─────────────┐  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐
│   Reader    │  │    Quiz     │  │ Flashcards  │  │  Examiner   │
│             │  │             │  │             │  │             │
│ (No routing │  │ (No routing │  │ (No routing │  │ (No routing │
│  logic)     │  │  logic)     │  │  logic)     │  │  logic)     │
└──────┬──────┘  └──────┬──────┘  └──────┬──────┘  └──────┬──────┘
       │                │                │                │
       │ MarkComplete() │ SubmitQuiz()   │ RateCard()     │ Submit()
       │                │                │                │
       └────────────────┴────────────────┴────────────────┘
                          │
                          ▼
                   ┌─────────────┐
                   │   Queue     │
                   │    Router   │
                   │ (mark task  │
                   │  complete,  │
                   │  insert     │
                   │  follow-up) │
                   └─────────────┘
```

Generated Reader -> Quiz handoffs flow through the queue router only; they are not direct module-to-module routes.

---

## Communication Rules

### Allowed

1. **Module → Queue Router:**
   - "I am complete"
   - "Here is my result"
   - "I need context"

1. **Queue Router → Module:**
   - "Mount with this context"
   - "Here is your task data"

3. **Service → Database:**
   - CRUD operations
   - Queries

### NOT Allowed

1. **Module → Module:** Direct communication
2. **Module → Database:** Bypass queue router
3. **Service → Module:** Services are stateless
4. **Router → Router:** No self-routing

---

## Code Organization

```
internal/
  orchestrator/       # Thin task router (queue router)
    service.go
  reader/
    handler.go       # Reader module backend
  quiz/
    handler.go       # Quiz module backend
  flashcards/
    handler.go       # Flashcard module backend
  fsrs/
    scheduler.go     # FSRS algorithm only
  examiner/
    handler.go       # Examiner module backend
  ingestion/
    pdf.go           # PDF extraction
    chunking.go      # Sliding window
  rag/
    pipeline.go      # Retrieval and answering
  db/
    store.go         # All SQL operations

frontend/src/pages/
  Dashboard.vue      # Task display
  Reader.vue         # Reading module
  Quiz.vue           # Quiz module
  Flashcards.vue     # Flashcard module
  Examiner.vue       # Examiner module
```

---

## Testing Boundaries

Each module can be tested independently:

- **Reader:** Mock block content, test rendering
- **Quiz:** Mock quiz set, test scoring
- **Flashcards:** Mock cards, test rating flow
- **Queue Router:** Mock database, test routing
- **FSRS:** Pure algorithm, test scheduling math




====================================================================================================
FILE: doc\AGENTS.md
ABSOLUTE: C:\Users\vishn\PROJECT\ai-tutor\doc\AGENTS.md
====================================================================================================

# doc/ — Agent Instructions

## Purpose

Single source of truth for project documentation. All architectural decisions, APIs, and plans live here.

---

## Document Reference

| File | Purpose | Read When |
|------|---------|-----------|
| `SPRINT.md` | Current sprint roadmap | Starting any work |
| `SPRINT_HISTORY.md` | Completed sprints | Understanding history |
| `ARCHITECTURE.md` | System architecture | Understanding big picture |
| `AGENT_MAP.md` | Module responsibilities | Adding new features |
| `SCHEMA.md` | Database schema | Writing DB queries |
| `DATA_API.md` | API contracts | Implementing endpoints |
| `APP_FLOW.md` | User flows | Building UI features |
| `DESIGN.md` | UI/UX design | Frontend work |
| `RAG.md` | Retrieval system | RAG changes |

---

## Rules

### ✅ DO

- Update relevant doc when code changes
- Keep SPRINT.md current with active work
- Add decision records for major changes
- Link related documents

### ❌ DON'T

- Let docs drift from implementation
- Document deprecated patterns (remove instead)
- Duplicate information across files

---

## Generated Assets

Vendor and generated assets are expected and NOT architectural concerns:

| Asset | Purpose | Status |
|-------|---------|--------|
| `tokenizer.json` | Tokenization vocabulary | Required runtime asset |
| `*.onnx` | Compiled embedding model | Required runtime asset |
| `wailsjs/` | Wails generated bindings | Build artifact |
| `frontend/dist/` | Compiled frontend | Build artifact |

Treat these as dependencies, not maintainability failures.

---

## Documentation Standards

### SPRINT.md

- Sprints are sequential
- Each sprint has clear goal and deliverables
- Checklist format for tracking
- No deprecated orchestration terminology

### SCHEMA.md

- SQL definitions first
- Index explanations
- Migration notes
- Data flow diagrams

### API Contracts

```markdown
## Endpoint: CompleteTask

**Input:**
- `taskID string` — Task to complete
- `result CompletionResult` — Result payload

**Output:**
- `error` — nil on success

**Behavior:**
- Marks task COMPLETED
- Inserts follow-up task per rules
- Returns error on validation failure
```

---

## Key Principles (Documented)

All docs must reinforce:

1. **Queue-driven** — Everything flows through `study_queue`
2. **Deterministic** — No hidden orchestration
3. **Explicit** — State transitions are clear
4. **SQLite-backed** — Single source of truth

---

## When Adding New Docs

1. Does existing doc cover this? (Update vs new)
2. Link from relevant files
3. Follow established format
4. Add to this AGENTS.md index

---

*Last updated: 2026-05-08*




====================================================================================================
FILE: doc\APP_FLOW.md
ABSOLUTE: C:\Users\vishn\PROJECT\ai-tutor\doc\APP_FLOW.md
====================================================================================================

# AI Tutor App Flow

## Core Philosophy: Persistent Guided Study Queue

**Reference:** `ARCHITECTURE.md` for complete system design, queue ordering rules, and architectural philosophy.

This document describes **runtime flow, user interaction sequence, and lifecycle behavior**. Queue-driven progression is deterministic and recommended, and manual study entry points are also supported. Both paths use SQLite as the source of truth and must converge on the same canonical bootstrap and ownership semantics.

Canonical checkpoint flow:

Dashboard -> Reader -> Quiz -> Dashboard

Reader completes the reading task only. The backend generates and activates the QUIZ follow-up task, and the Dashboard owns progression again after quiz submission and evaluation.

---

## 1. The Queue Loop (Primary Flow)

### What

The application follows a deterministic SQLite-driven queue:

```
Dashboard fetches next pending task
→ User clicks task → Status becomes ACTIVE
→ Mount correct module/view
→ User completes/skips task
→ Mark task COMPLETED/SKIPPED/FAILED
→ Insert follow-up tasks (if any)
→ Repeat
```

### Multi-Notebook Priority

Multiple notebooks are supported with deterministic prioritization:

- Notebooks have `priority INTEGER DEFAULT 5` (1-10 scale)
- Higher priority notebooks surface more frequently
- Lower priority notebooks still eventually appear
- Priority is a **deterministic bias** (query-time rule, not adaptive scheduling)

### Queue Ordering Rules

**Reference:** `ARCHITECTURE.md` Section 7 for complete priority hierarchy and SQL query.

Explicit priority hierarchy (task type first, then notebook priority):

| Order | Task Type |
|-------|-----------|
| 1 | `FLASHCARD_REVIEW` (due reviews) |
| 2 | `REREAD` |
| 3 | `QUIZ` |
| 4 | `READING` |
| 5 | `EXAMINER` |

Then apply notebook priority bias within each tier.

### Why

**Reference:** `ARCHITECTURE.md` Section 1 for architectural rationale.

Runtime benefits:

### How

1. **Dashboard queries** `study_queue` for next `PENDING` task (with ordering rules)
2. **User clicks task** → Status becomes `ACTIVE`, `activated_at` timestamp set
3. **Router opens** correct module with context
4. **Module renders** content based on `task_type` and `block_id`
5. **User completes task** → Module calls `CompleteTask(taskID, result)`
6. **Backend marks** task `COMPLETED`/`SKIPPED`/`FAILED`, inserts follow-up tasks
7. **Dashboard refreshes** showing next pending task

Manual study actions, such as opening Quiz, Flashcards, Reader, or Written Assessment directly, are valid when they call the same backend initialization and retrieval helpers instead of re-implementing them per route. Generated QUIZ follow-up tasks may be activated immediately after Reader completion as part of the same checkpoint chain, but that remains a queue transition rather than arbitrary module-to-module routing.

### Task Lifecycle Semantics

Explicit state machine:

```
PENDING → ACTIVE (when user opens task)
  ↓
COMPLETED (on success)
  ↓
SKIPPED (on user bypass - auditable)
  ↓
FAILED (on generation error - can retry)
```

**Crash Recovery:** On startup, any `ACTIVE` tasks older than 30-minute timeout revert to `PENDING`. This ensures restart-safe queue recovery.

---

## 2. Ingestion Pipeline

### What

PDF upload → Chapter selection → Sliding window chunking → READING tasks inserted

### Why

**Reference:** `ARCHITECTURE.md` Section 5 for chunking rationale.

### How

1. **PDF Upload**: User uploads PDF, system extracts text
2. **Chapter Selection**: User reviews/prunes extracted chapters
3. **Sliding Window Chunking**:
   - 2500-word chunks
   - 200-word overlap between chunks
   - Deterministic, no AI involvement in boundary decisions
4. **READING Tasks Inserted**: One task per chunk into `study_queue`

---

## 3. Reading Flow

### What

User completes reading task → Backend generates follow-up QUIZ task

### Why

**Reference:** `ARCHITECTURE.md` Section 8 for synchronous generation rationale.

### Reading Completion (Trust-Based)

Reading tasks use trust-based completion:

- User decides when reading is complete
- Complete Session button stays enabled during active reading task
- No enforced page-completion validation
- No engagement surveillance, timers, or tracking

### How

1. User clicks **Complete Session** when they feel ready (button always enabled)
2. Frontend shows **loading spinner**
3. Backend calls LLM synchronously
4. Reading completion closes the reading task only; it does not measure mastery or progression quality
5. Backend inserts and activates the generated **QUIZ task** in `study_queue`
6. Dashboard may immediately surface the quiz as the next pending checkpoint

### Quiz Generation States

QUIZ tasks have explicit generation lifecycle:

| State | Meaning |
|-------|---------|
| `GENERATING` | LLM call in progress |
| `READY` | Quiz ready for user |
| `FAILED` | Generation error - dashboard surfaces explicitly |

**Flow:**
1. Reading complete → QUIZ task inserted with `GENERATING` state
2. LLM called synchronously
3. Success → `generation_status = READY`
4. Failure → `generation_status = FAILED` (user sees explicit error)

The Reader does not route itself to Quiz. Only generated follow-up quiz tasks may transition immediately through the queue loop.

---

## 4. Quiz Flow & Remediation

### What

Quiz submission/evaluation → Queue-driven follow-up

### Why

Remediation is lightweight queue insertion, NOT:

- Forced loops
- Hidden state machines  
- User traps

The app only **recommends** revisiting material.

### How

**IF PASS:**
```
QUIZ task → mark COMPLETED
→ Optionally insert FLASHCARD_REVIEW task or move to next queued task
→ Dashboard shows next pending task
```

**IF FAIL (below threshold):**
```
QUIZ task → mark COMPLETED
→ Insert REREAD task for the material or other remediation follow-ups (if under max attempts)
→ Generate lightweight AI feedback
→ Dashboard shows REREAD as next pending task
```

User can:
- Complete the REREAD task
- Skip it (mark SKIPPED - auditable, can resurface)
- The system does NOT force remediation loops

Dashboard regains orchestration ownership after quiz submission and evaluation.

### Reread Loop Protection

Maximum reread attempts: **3** (default per block)

- `reread_attempt` counter tracked per block
- After max reached: stop auto-inserting reread tasks
- Show recommendation message to user
- Allow manual retry if user chooses
- Continue queue progression

Prevents infinite queue pollution.

---

## 5. Flashcards & FSRS

### What

FSRS is a scheduling algorithm only. It calculates intervals; it does not control flow.

### Flashcard Review Granularity

**One `FLASHCARD_REVIEW` task = one review session for a block/chunk.**

- Do NOT create one queue task per flashcard
- A single task represents "review all due cards in this block"
- Prevents queue explosion with many cards

### How

1. When reviews become **due** (per FSRS calculation):
   - Insert `FLASHCARD_REVIEW` task into `study_queue` (one task per block)
2. Dashboard fetches pending review task
3. User completes flashcard session (reviews all due cards in block)
4. FSRS calculates next review interval
5. New `FLASHCARD_REVIEW` task scheduled for future due date

Flashcards become **queue-driven review tasks**, not autonomous review systems.

---

## 6. Examiner Mode

### What

Optional advanced queue task for written assessments.

### How

- Triggered after mastery thresholds (e.g., quiz scores > 80%)
- Appears as `EXAMINER` task type in `study_queue`
- Dashboard-driven, user-triggered
- NOT a hidden autonomous system

### Examiner Task Policy

- Inserted after mastery thresholds
- Assigned elevated queue priority (tier 5, after reviews/quizzes/reading)
- Remain optional (user can skip)
- Appear naturally in queue flow through deterministic ordering
- NOT through hidden orchestration

Prevents starvation: EXAMINER is tier 5, ensuring reviews and reading are not blocked.

---

## 7. Navigation and Layout

### What

Left sidebar navigation with persistent sections:

1. Dashboard (default landing)
2. Reader
3. Quiz
4. Flashcards
5. Settings (bottom)

### Why

Stable mental model; users can always access any module directly, but the **Dashboard queue is the primary workflow**.

---

## 8. Synchronous Generation

**Reference:** `ARCHITECTURE.md` Section 8 for LLM layer design.

All AI generation is synchronous. User clicks Complete → Loading spinner → LLM call → Response → Task inserted.

---

## 9. Error and State Feedback

### What

Consistent status signaling for loading, success, and failure.

### How

- **Loading**: Show spinner for synchronous LLM calls
- **Empty Queue**: "All caught up! Upload a new PDF to continue."
- **AI Unavailable**: Explicit error, no fallback
- **Queue State**: Always visible and queryable via SQLite
- **Quiz Generation Failed**: Explicit error state, user can retry
- **Max Rereads Reached**: Recommendation message, manual retry available

### Skip Semantics

Explicit terminal states preserve audit trail:

| Status | Meaning | Can Resurface |
|--------|---------|---------------|
| `COMPLETED` | Successfully finished | No |
| `SKIPPED` | User bypassed task | Yes (manual retry) |
| `FAILED` | Generation error | Yes (can retry) |

Skipped tasks are auditable and can resurface if needed. Do NOT silently mark skipped tasks as completed.

---

## 10. Module Boundaries (Strict)

### Reader Module
- Renders PDF pages
- Displays content from assigned page range
- StartPage is authoritative for opening context
- EndPage is informational only
- Trust-based completion (user signals when done)
- Reader only completes the reading task
- No orchestration logic
- No completion validation or gating
- No arbitrary routing to other modules

Generated follow-up QUIZ tasks may be activated immediately after Reader completion through the queue loop only.

### Quiz Module
- Displays quiz
- Returns score
- Drives mastery/remediation follow-up through queue results
- No orchestration logic

### Flashcard Module
- Renders cards
- Captures ratings (Again/Hard/Good/Easy)
- No orchestration logic

### Examiner Module
- Renders written assessments
- No orchestration logic

**Queue Router only**: fetch next pending task, mount correct module, mark complete, insert follow-up tasks.




====================================================================================================
FILE: doc\ARCHITECTURE.md
ABSOLUTE: C:\Users\vishn\PROJECT\ai-tutor\doc\ARCHITECTURE.md
====================================================================================================

# AI Tutor Architecture

## 1. Architecture Goals: Persistent Queue Model

### What

A **Persistent Guided Study Queue** - NOT an autonomous AI tutor, hidden orchestration engine, or proactive scheduling system. The queue is the recommended guided progression path, but manual and exploratory entry points are intentionally supported when they reuse the same canonical bootstrap and topic ownership semantics.

Advanced learning systems (quizzes, FSRS, remediation) are treated as **"Data, not Engines."** They create queue tasks but do NOT control orchestration directly.

Canonical checkpoint flow:
Dashboard -> Reader -> Quiz -> Dashboard

Reader completes the reading task only. The backend generates and activates the QUIZ follow-up task, and the Dashboard regains ownership after quiz submission and evaluation. A Reader -> Quiz transition is allowed only for generated follow-up quiz tasks and only through the queue loop.

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

If a reading task produces a quiz checkpoint, the generated QUIZ task may be activated immediately as the next queue item. That is a queue transition, not direct module-to-module orchestration.

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

Reader can be followed immediately by Quiz when the backend generates the follow-up quiz task. This is the only Reader -> Quiz path that is allowed.

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
1. User signals reading complete (trust-based)
2. Reading completion closes the reading task only; it does not determine mastery or remediation quality
3. QUIZ task inserted with `GENERATING` state
4. LLM called synchronously
5. On success: `generation_status = READY`
6. On failure: `generation_status = FAILED` (dashboard surfaces explicitly)

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

### Reading Completion (Trust-Based)

Reading tasks use trust-based completion:

- User decides when reading is complete
- Complete Session button stays enabled during active reading task
- StartPage is authoritative for opening context
- EndPage is informational only
- No enforced page-completion validation
- No surveillance logic, reading timers, or engagement tracking
- Lightweight MVP approach

Reading completion does not measure quality or mastery. It only closes the reading task and allows the backend to generate the follow-up quiz checkpoint.

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

Generated follow-up quiz tasks may be mounted immediately after Reader completion if they are the next pending queue item. The router still owns the transition; the Reader does not.

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
→ Queue router inserts REREAD task and other follow-up tasks as needed
→ Dashboard regains ownership and shows the next pending task
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

Reader completion may immediately surface a generated Quiz task as the next queue item. That is a Dashboard/queue-router handoff, not a direct Reader-to-Quiz module route.

**Example:**
- Task: `QUIZ` with `block_id: "quiz-set-123"`
- Click → Quiz module mounts
- Quiz module loads quiz_set by `block_id`
- User completes → Queue router marks complete → Next task appears




====================================================================================================
FILE: doc\CLAUDE.md
ABSOLUTE: C:\Users\vishn\PROJECT\ai-tutor\doc\CLAUDE.md
====================================================================================================

# Compatibility Notice

This repository now uses `AGENTS.md` as the canonical AI instruction file.

Please refer to:
- `AGENTS.md`

Do not duplicate or diverge instructions between files.




====================================================================================================
FILE: doc\DATA_API.md
ABSOLUTE: C:\Users\vishn\PROJECT\ai-tutor\doc\DATA_API.md
====================================================================================================

# Data API Contracts

## Overview

API contracts between frontend, queue router, and modules. All communication is synchronous and explicit.

---

## Queue Router API

### GetNextTask

Returns the next pending task from the queue.

**Endpoint:** `GetNextTask() → Task`

**Request:** None

**Response:**
```json
{
  "id": "task-uuid",
  "task_type": "READING",
  "block_id": "block-uuid",
  "related_id": "topic-uuid",
  "status": "PENDING",
  "priority": 1,
  "created_at": 1234567890,
  "context": {
    "topic_title": "Neural Networks",
    "word_count": 2500,
    "progress": 0
  }
}
```

**Errors:**
- `ErrNoPendingTasks` - Queue is empty

---

### CompleteTask

Marks a task complete and triggers follow-up logic.

**Endpoint:** `CompleteTask(taskID string, result TaskResult) → error`

**Request:**
```json
{
  "task_id": "task-uuid",
  "result": {
    "type": "quiz_result",
    "score": 75,
    "passed": true
  }
}
```

**Result Types:**

| Type | Use Case | Data Fields |
|------|----------|-------------|
| `quiz_result` | Quiz completion | `score`, `passed` |
| `read_complete` | Reading completion | `pages_read` (informational only; not a mastery signal) |
| `flashcard_review` | Flashcard session | `cards_reviewed`, `ratings` |
| `skip` | User skips task | `reason` (optional) |

**Response:** Success or error

**Side Effects:**
- Updates task status to `COMPLETED`, `SKIPPED`, or `FAILED`
- May insert follow-up tasks based on result
- Reading completion only closes the reading task; it does not determine mastery or remediation quality
- Quiz submission/evaluation may insert reread, retry, next task, spaced repetition follow-ups, or mastery progression tasks
- Dashboard regains ownership after quiz evaluation
- Skipped tasks preserve audit trail and can resurface

### SkipTask

Explicitly marks a task as skipped (auditable bypass).

**Endpoint:** `SkipTask(taskID string, reason string) → error`

**Request:**
```json
{
  "task_id": "task-uuid",
  "reason": "User chose to skip remediation"
}
```

**Response:** Success or error

**Side Effects:**
- Updates task status to `SKIPPED`
- Task remains auditable in database
- Can be resurfaced via manual retry if needed
- No follow-up tasks inserted

---

### GetTaskContext

Returns full context for a task.

**Endpoint:** `GetTaskContext(taskID string) → TaskContext`

**Response:**
```json
{
  "task": {
    "id": "task-uuid",
    "task_type": "READING",
    "block_id": "block-uuid"
  },
  "block": {
    "id": "block-uuid",
    "content": "...",
    "word_count": 2500,
    "start_page": 10,
    "end_page": 15
  },
  "topic": {
    "id": "topic-uuid",
    "title": "Neural Networks"
  }
}
```

---

## Reader Module API

### GetBlockContent

Returns content for a reading block.

**Endpoint:** `GetBlockContent(blockID string) → BlockContent`

**Response:**
```json
{
  "id": "block-uuid",
  "content": "Full text content...",
  "word_count": 2500,
  "start_page": 10,
  "end_page": 15,
  "order_index": 3,
  "topic_id": "topic-uuid"
}
```

---

### MarkBlockRead

Records reading progress.

**Endpoint:** `MarkBlockRead(blockID string, progress int) → error`

**Request:**
```json
{
  "block_id": "block-uuid",
  "progress": 100
}
```

---

## Quiz Module API

### GetQuizSet

Returns quiz questions for a block.

**Endpoint:** `GetQuizSet(blockID string) → QuizSet`

**Response:**
```json
{
  "id": "quiz-set-uuid",
  "block_id": "block-uuid",
  "topic_id": "topic-uuid",
  "questions": [
    {
      "id": "q-1",
      "question": "What is backpropagation?",
      "options": ["A", "B", "C", "D"],
      "correct_answer": 0
    }
  ],
  "threshold": 70
}
```

---

### SubmitQuiz

Submits answers and returns score.

**Endpoint:** `SubmitQuiz(blockID string, answers []Answer) → QuizResult`

**Request:**
```json
{
  "block_id": "quiz-set-uuid",
  "answers": [
    {"question_id": "q-1", "selected": 0},
    {"question_id": "q-2", "selected": 2}
  ]
}
```

**Response:**
```json
{
  "score": 75,
  "passed": true,
  "correct_count": 3,
  "total_count": 4,
  "feedback": "Good understanding of concepts..."
}
```

Quiz results are evaluated by the backend to determine follow-up behavior such as reread, retry, next task, spaced repetition follow-ups, or mastery progression.

---

## Flashcard Module API

### GetDueCards

Returns cards due for review.

**Endpoint:** `GetDueCards(blockID string) → []Card`

**Response:**
```json
{
  "cards": [
    {
      "id": "card-uuid",
      "prompt": "What is gradient descent?",
      "answer": "An optimization algorithm...",
      "due_at": 1234567890
    }
  ]
}
```

---

### RateCard

Records user rating and updates FSRS state.

**Endpoint:** `RateCard(cardID string, rating Rating) → error`

**Request:**
```json
{
  "card_id": "card-uuid",
  "rating": 3
}
```

**Rating Values:**
- 1 = Again
- 2 = Hard
- 3 = Good
- 4 = Easy

---

## FSRS Service API

### CalculateNextReview

Pure function for FSRS scheduling.

**Endpoint:** `CalculateNextReview(state FSRSState, rating int) → FSRSResult`

**Request:**
```json
{
  "state": {
    "stability": 1.5,
    "difficulty": 5.0,
    "elapsed_days": 1
  },
  "rating": 3
}
```

**Response:**
```json
{
  "next_interval_days": 3,
  "new_state": {
    "stability": 2.8,
    "difficulty": 4.8
  }
}
```

---

### GetDueCards

Returns all cards due for a topic.

**Endpoint:** `GetDueCards(topicID string) → []Card`

---

## RAG / Ask AI API

### AskQuestion

Answers a question using topic-scoped retrieval.

**Endpoint:** `AskQuestion(topicID string, question string) → Answer`

**Request:**
```json
{
  "topic_id": "topic-uuid",
  "question": "Explain backpropagation"
}
```

**Response:**
```json
{
  "answer": "Backpropagation is...",
  "context_blocks": ["block-uuid-1", "block-uuid-2"],
  "confidence": 0.95
}
```

---

## Ingestion API

### ProcessPDF

Extracts text and creates chunks.

**Endpoint:** `ProcessPDF(filePath string) → ProcessingResult`

**Response:**
```json
{
  "topic_id": "topic-uuid",
  "title": "Neural Networks",
  "blocks_created": 12,
  "tasks_inserted": 12
}
```

---

## Type Definitions

### Task Types

```go
type TaskType string

const (
  TaskTypeReading         TaskType = "READING"
  TaskTypeQuiz            TaskType = "QUIZ"
  TaskTypeReread          TaskType = "REREAD"
  TaskTypeFlashcardReview TaskType = "FLASHCARD_REVIEW"
  TaskTypeExaminer        TaskType = "EXAMINER"
)
```

### Task Status

```go
type TaskStatus string

const (
  StatusPending   TaskStatus = "PENDING"
  StatusActive    TaskStatus = "ACTIVE"
  StatusCompleted TaskStatus = "COMPLETED"
  StatusSkipped   TaskStatus = "SKIPPED"
  StatusFailed    TaskStatus = "FAILED"
)
```

**Status Semantics:**

| Status | Meaning | Terminal |
|--------|---------|----------|
| `PENDING` | Waiting in queue | No |
| `ACTIVE` | Currently being worked | No |
| `COMPLETED` | Successfully finished | Yes |
| `SKIPPED` | User bypassed task | Yes (auditable) |
| `FAILED` | Generation error | Yes (can retry) |

### Generation Status (Quiz Tasks)

```go
type GenerationStatus string

const (
  StatusGenerating GenerationStatus = "GENERATING"
  StatusReady      GenerationStatus = "READY"
  StatusFailedGen  GenerationStatus = "FAILED"
)
```

### Task Result Types

```go
type TaskResult struct {
  Type   string      // "quiz_result", "read_complete", "flashcard_review"
  Data   interface{} // Type-specific data
}

type QuizResult struct {
  Score   int  // 0-100
  Passed  bool
}

type FlashcardReviewResult struct {
  CardsReviewed int
  Ratings       []int
}
```

---

## Error Handling

### Standard Errors

| Error | Code | Description |
|-------|------|-------------|
| ErrNotFound | 404 | Resource not found |
| ErrNoPendingTasks | 204 | Queue is empty |
| ErrInvalidInput | 400 | Invalid request |
| ErrLLMUnavailable | 503 | LLM service down |
| ErrQuizGenerationFailed | 500 | Quiz generation error |
| ErrMaxRereadsReached | 409 | Max reread attempts exceeded |
| ErrTaskNotActive | 409 | Task is not in ACTIVE status |

### Error Response Format

```json
{
  "error": "ErrNoPendingTasks",
  "message": "No pending tasks in queue",
  "code": 204
}
```

---

## API Call Patterns

### Standard Flow

```
1. Dashboard calls GetNextTask()
2. User clicks task
3. Router mounts module with task.context
4. Module calls its API (GetBlockContent, GetQuizSet, etc.)
5. User completes task
6. Module calls CompleteTask(taskID, result)
7. Queue router marks complete, inserts follow-ups
8. Dashboard refreshes, shows next task
```

Reader completion may immediately lead to a generated Quiz task becoming the next pending queue item. That transition is queue-owned, not a direct module-to-module route.

### No Async Patterns

- No callbacks
- No event listeners
- No webhooks
- No background job status polling

All calls are:
- Synchronous request/response
- Immediate result
- Loading state shown in UI

---

## Authentication / Security

Local-only app - no authentication required.

All APIs:
- Run on localhost
- Bound to Wails bridge
- No CORS needed
- No tokens needed




====================================================================================================
FILE: doc\DESIGN.md
ABSOLUTE: C:\Users\vishn\PROJECT\ai-tutor\doc\DESIGN.md
====================================================================================================

# Design System Specification: The Academic Curator

## 1. Overview & Creative North Star
The Creative North Star for this design system is **"The Digital Sanctuary."** 

In an academic context, cognitive load is the enemy. This system moves beyond "minimalism" into a realm of intentional editorial clarity. We are not just building a tool; we are building an environment for deep work. The aesthetic breaks the "template" look by favoring extreme white space, asymmetric type treatments, and a structural philosophy that treats the screen like a physical gallery space. 

Instead of boxes within boxes, we use **Tonal Nesting** and **Atmospheric Depth** to guide the eye. The interface should feel like a high-end architectural blueprint—precise, quiet, and profoundly functional.

---

## 2. Colors & Surface Philosophy
The palette is rooted in a "High-Value Gray" scale, using blue only as a surgical instrument for interaction.

### The "No-Line" Rule
Traditional 1px borders are strictly prohibited for sectioning content. Boundaries must be defined through background shifts. 
*   **Implementation:** A `surface-container-low` card sitting on a `background` provides all the separation necessary. If a container needs more prominence, elevate it to `surface-container-lowest` (pure white) to make it "pop" against the slightly off-white page.

### Surface Hierarchy & Nesting
Treat the UI as a series of stacked sheets of vellum.
*   **Base Layer:** `background` (#f9f9fb)
*   **Secondary Content Areas:** `surface-container` (#ebeef2)
*   **Interactive/Floating Elements:** `surface-container-lowest` (#ffffff)
*   **System Overlays:** Use `surface-bright` with a 20px backdrop-blur to create a "Glassmorphism" effect for navigation bars and floating action menus.

### The "Glass & Gradient" Rule
To prevent the UI from feeling "flat" or "cheap," CTAs should utilize a subtle, 15-degree linear gradient from `primary` (#005bc1) to `primary_dim` (#004faa). This adds a microscopic level of curvature and "soul" to the crisp blue accent.

---

## 3. Typography: Editorial Authority
We utilize a dual-typeface system to create an "Academic Journal" feel. **Manrope** provides a geometric, authoritative voice for headers, while **Inter** ensures maximum legibility for long-form research text.

*   **Display (Manrope):** Use `display-lg` for empty states or dashboard greetings. Tracking should be set to -2% to feel tighter and more premium.
*   **Body (Inter):** All body text uses `body-md` or `body-lg`. We rely on **Font Weight** (SemiBold vs Regular) rather than color to distinguish between headers and metadata.
*   **Hierarchy Tip:** A `headline-sm` in Bold is more effective than a medium headline in a different color. Keep the `on-surface` (#2d3338) for almost all text to maintain high contrast and accessibility.

---

## 4. Elevation & Depth
In this system, "Shadows" are an admission of failure in layout. Use them only when an element is physically "above" the workflow (e.g., Modals).

*   **The Layering Principle:** 
    *   Level 0: `background`
    *   Level 1: `surface-container-low` (Content groupings)
    *   Level 2: `surface-container-lowest` (Active cards/Primary focus)
*   **Ambient Shadows:** If a shadow is required for a floating Modal, use a "Soft Ambient" style: 
    *   `box-shadow: 0 20px 40px rgba(45, 51, 56, 0.06);` (Using a tinted version of `on-surface`).
*   **The "Ghost Border":** For input fields or search bars, use a 1px stroke of `outline-variant` at **20% opacity**. This creates a "suggestion" of a container without breaking the airy aesthetic.

---

## 5. Components

### Buttons
*   **Primary:** High-gloss `primary` gradient with `on-primary` text. Roundedness: `xl` (0.75rem).
*   **Secondary:** `surface-container-highest` background with `primary` text. No border.
*   **Tertiary:** Text-only, SemiBold, using `primary` color. Reserved for "Cancel" or low-priority actions.

### Cards & Lists
*   **Forbidden:** Horizontal divider lines (`<hr>`).
*   **Replacement:** Use `1.5rem` of vertical white space or a subtle shift from `surface` to `surface-container-low` to distinguish between list items.
*   **Interactive State:** On hover, a card should transition from `surface-container-low` to `surface-container-lowest` and gain a 2px "Soft Ambient" shadow.

### Input Fields
*   **Style:** Minimalist underline or "Ghost Border." 
*   **Focus State:** The border opacity increases to 100% of `primary`, and the label (`label-md`) shifts to `primary` color. 
*   **Error:** Use `error` (#9f403d) only for the helper text; the input box should remain neutral to avoid "visual shouting."

### Specialized Academic Components
*   **The "Focus Mode" Toggle:** A floating `full` rounded chip using Glassmorphism (`surface-bright` @ 70% opacity + blur).
*   **Citation Chips:** Small `label-sm` chips using `secondary-container` backgrounds to keep them secondary to the main thesis text.

---

## 6. Do’s and Don’ts

### Do
*   **Use Asymmetry:** Align large headlines to the left with wide right margins to mimic modern editorial layouts.
*   **Trust the White Space:** If a screen feels "empty," it is likely working. Avoid the urge to add icons or illustrations.
*   **Respect the 8px Grid:** Ensure all spacing is a multiple of 8 to maintain the mathematical rigor expected in an academic app.

### Don't
*   **Don't use pure black:** Use `on-surface` (#2d3338) for text; it is softer on the eyes for long study sessions.
*   **Don't use "Apple Blue" for everything:** Save #007AFF (Primary) for the *single* most important action on the screen.
*   **Don't use standard shadows:** Never use a `0,0,0,0.5` shadow. It destroys the "Digital Sanctuary" feel. Always use low-opacity, tinted blurs.




====================================================================================================
FILE: doc\PLAN_SCOPE.md
ABSOLUTE: C:\Users\vishn\PROJECT\ai-tutor\doc\PLAN_SCOPE.md
====================================================================================================

# Plan Scope: Boundaries and Exclusions

## Purpose

Define explicit boundaries: what is IN scope and EXPLICITLY OUT of scope.

**Reference:** `ARCHITECTURE.md` for system design; `AGENT_MAP.md` for module responsibilities.

---

## IN Scope

### 1. Core Queue System

**IN:**
- `study_queue` table with 5 task types
- Status enum: `PENDING`, `ACTIVE`, `COMPLETED`, `SKIPPED`, `FAILED`
- Priority-based task ordering
- Task lifecycle semantics (crash recovery, timeout handling)
- SQLite as source of truth

**NOT:** Runtime-only queues, hidden state machines, in-memory task lists.

### 2. Ingestion Pipeline

**IN:**
- PDF upload and text extraction
- Chapter extraction and user pruning
- **Sliding window chunking**: 2500 words, 200-word overlap
- Automatic `READING` task insertion

**NOT:** Semantic chunking, AI-generated boundaries, autonomous orchestration.

### 3. Quiz System

**IN:**
- Synchronous quiz generation (LLM call)
- Quiz-taking interface
- Pass/fail threshold evaluation
- Remediation task insertion on fail
- Explicit generation states: `GENERATING`, `READY`, `FAILED`
- Failed quiz generation surfaces explicit error to user

**NOT:** Async generation, background jobs, forced loops, silent failures.

### 4. Flashcards & FSRS

**IN:**
- FSRS as scheduling algorithm
- Due date calculation
- `FLASHCARD_REVIEW` task insertion
- Card rating (Again/Hard/Good/Easy)

**NOT:** FSRS as queue router or session manager.

### 5. Remediation

**IN:**
- Lightweight `REREAD` task insertion
- AI-generated feedback on failed quizzes
- User can complete OR skip remediation
- Reread loop protection (max 3 attempts default)
- Auditable skip states

**NOT:** Forced loops, user traps, mandatory repetition.

### 6. Examiner Mode

**IN:**
- Written assessment tasks
- User-triggered after mastery thresholds
- Queue-driven appearance (tier 5 priority)
- Optional (user can skip)

**NOT:** Autonomous triggering, background generation, task starvation.

### 7. Queue Router

**IN:**
- Fetch next pending task (with deterministic ordering rules)
- Mount correct module
- Mark tasks complete/skipped/failed
- Insert follow-up tasks per explicit rules
- Task lifecycle management (ACTIVE → terminal states)
- Crash recovery (timeout stale ACTIVE tasks)

**NOT:** Proactive scheduling, event buses, workflow builders, background mutation.

### 8. Multi-Notebook Support

**IN:**
- Multiple notebooks with deterministic priority biasing
- Notebook `priority` field (1-10, default 5)
- Higher priority notebooks surface more frequently
- Lower priority notebooks still eventually appear
- Queue ordering with notebook bias

**NOT:** AI-driven scheduling, velocity orchestration, autonomous switching.

### 9. Dashboard Starvation Protection

**IN:**
- Deterministic balancing rule: after N reviews, allow 1 reading
- Default: after 5 review tasks, surface 1 READING task
- Query-time bias (NOT autonomous orchestration)
- Prevents review monopolization

**NOT:** Autonomous balancing, AI-driven pacing.

**Balancing rules are static SQL ordering constraints, not adaptive runtime systems.**

### 10. RAG / Ask AI

**IN:**
- Topic-scoped retrieval
- Single-turn stateless requests
- Sliding window chunk retrieval

**NOT:** Semantic retrieval, cross-topic search, conversation memory.

### 9. Synchronous Generation

**IN:**
- All LLM calls are synchronous
- Loading spinners during generation
- Immediate response with content

**NOT:** Background workers, async queues, proactive generation.

---

## EXPLICITLY OUT of Scope

### Architecture Patterns (DO NOT ADD)

| Pattern | Status | Reason |
|---------|--------|--------|
| Proactive orchestration | OUT | Use queue query instead |
| Hidden scheduling systems | OUT | SQLite queue is visible |
| Autonomous AI pipelines | OUT | Synchronous calls only |
| Dual timer engines | OUT | Single queue source |
| Event buses | OUT | Direct API calls |
| Workflow builders | OUT | Fixed queue types |
| Drag-drop orchestration | OUT | Static queue flow |
| Runtime-only state | OUT | Persistent SQLite |
| Async background jobs | OUT | Synchronous MVP |
| Multi-step agents | OUT | Stateless single-turn |
| LangChain | OUT | Explicit architecture |

### Features (DO NOT ADD)

| Feature | Status | Reason |
|---------|--------|--------|
| Semantic chunking | OUT | Sliding window is sufficient |
| AI chunk boundaries | OUT | Deterministic boundaries |
| Syllabus graphing | OUT | Overkill for MVP |
| Multi-device sync | OUT | Local-first MVP |
| Cloud backup | OUT | Phase 2 consideration |
| Social features | OUT | Single-user focus |
| Gamification | OUT | Queue simplicity |
| Advanced analytics | OUT | SQLite queries suffice |
| Plugin system | OUT | Fixed modules |
| Theme customization | OUT | Single design system |
| AI-driven scheduling | OUT | Deterministic bias only |
| Velocity orchestration | OUT | Query-driven only |
| Hidden balancing logic | OUT | Explicit rules only |
| Reading surveillance | OUT | No timers/tracking |
| Engagement tracking | OUT | Lightweight validation only |

---

## Scope Boundaries

### Queue as Source of Truth

All flows go through `study_queue`. See `ARCHITECTURE.md` Section 4 for data model.

### Stateless Modules

Modules render content for `block_id`; they do not route or schedule.

### SQLite as State Machine

State is queryable SQL, not in-memory code. See `ARCHITECTURE.md` Section 10 for state transition semantics.

---

## Decision Log

### Why Sliding Window?

**Decision:** Use sliding window chunking (2500 words, 200 overlap)

**Rationale:**
- Deterministic and inspectable
- No AI dependency for boundaries
- Sufficient for MVP
- Easy to debug

**Rejected:**
- Semantic chunking (too complex)
- AI boundaries (non-deterministic)
- Topic modeling (overkill)

### Why Synchronous Generation?

**Decision:** All LLM calls are synchronous

**Rationale:**
- Deterministic MVP > premature optimization
- No background worker complexity
- User sees immediate feedback
- Easier to debug

**Rejected:**
- Async job queues
- Background workers
- Event-driven architecture

### Why Persistent Queue?

**Decision:** SQLite `study_queue` drives all flows

**Rationale:**
- Resumable across app restarts
- Queryable and debuggable
- No runtime-only state
- Simple and explicit

**Rejected:**
- Runtime task lists
- Hidden queue routers
- In-memory queues
- Complex state machines

---

## Success Criteria

The architecture is correct if:

1. All user flows start from `study_queue` query
2. No runtime-only queues exist
3. All state transitions are explicit SQL updates
4. Modules have no orchestration logic
5. Quiz generation is synchronous with loading spinner
6. Remediation is optional (user can skip)
7. FSRS only schedules, does not orchestrate
8. Dashboard only shows pending tasks
9. No hidden state machines
10. SQLite is source of truth




====================================================================================================
FILE: doc\PLATFORM_SUPPORT.md
ABSOLUTE: C:\Users\vishn\PROJECT\ai-tutor\doc\PLATFORM_SUPPORT.md
====================================================================================================

# Platform Support

## Current Status: Windows-First

**Primary Target:** Windows 10/11 (x64)

Windows is the exclusive build target for the MVP phase. This constraint eliminates cross-platform native library complexity while the core RAG pipeline and queue architecture stabilize.

### Windows-Specific Dependencies

| Component | File | Purpose |
|-----------|------|---------|
| ONNX Runtime | `onnxruntime.dll` | Local embedding inference |
| Vector Storage | `vec0.dll` | SQLite vector search extension |
| Build Scripts | `sync-deps.sh`, `windows-sync-deps.ps1` | Dependency management |

### Build Requirements

- Go 1.21+ with CGO enabled (MSYS2/MinGW on Windows)
- MSVC or MinGW toolchain
- PowerShell for dependency sync scripts

---

## Future Platforms

### macOS (Intel/Apple Silicon)

**Required Changes:**
- Replace `onnxruntime.dll` with `libonnxruntime.dylib`
- Compile `vec0.dylib` for Darwin
- Update `app.go` app-data directory handling for macOS paths
- Add macOS-specific build constraints

### Linux (x64/ARM64)

**Required Changes:**
- Replace `onnxruntime.dll` with `libonnxruntime.so`
- Compile `vec0.so` for target architecture
- Validate CGO build requirements across distributions
- Handle Linux-specific path conventions

---

## Rationale

Single-platform focus during MVP enables:

1. **Deterministic Testing:** ONNX-to-SQLite pipeline stabilizes without OS-specific memory/driver variables
2. **Simplified Asset Management:** Single `asset/` folder with Windows-only binaries
3. **Faster Iteration:** No conditional compilation paths or abstraction layers required

---

## Implementation Notes

Platform-specific code should use Go build constraints:

```go
//go:build windows
// +build windows

package embeddings
```

Remove half-finished `runtime.GOOS` switches. Platform support is either implemented or documented as future work—no intermediate states.




====================================================================================================
FILE: doc\PROJECT_STRUCTURE.md
ABSOLUTE: C:\Users\vishn\PROJECT\ai-tutor\doc\PROJECT_STRUCTURE.md
====================================================================================================

# Project Structure

Directory organization and package ownership. For architecture details, see `ARCHITECTURE.md`.

---

## Backend (Go + Wails)

### Top-Level Files (Simplified)

| File | Responsibility |
|------|----------------|
| `main.go` | Wails bootstrap only |
| `app.go` | Wails-facing methods (~600-700 lines, acceptable) |
| `notebook_endpoints.go` | Notebook API (~600-700 lines, acceptable) |

### Internal Packages

```
internal/
  orchestrator/       # Queue router (thin task router)
    service.go
  
  db/                 # Data persistence
    store.go          # All SQL operations
    schema.go         # Table definitions
  
  models/             # Domain types
    task.go           # TaskType, TaskStatus, Task
    block.go          # Block, BlockType
    quiz.go           # QuizSet, QuizResult
  
  ingestion/          # PDF → Chunks → Queue
    pdf.go            # PDF extraction
    chunking.go       # Sliding window (2500 words, 200 overlap)
  
  reader/             # Reading module backend
    handler.go
  
  quiz/               # Quiz module backend
    handler.go
    generator.go      # Synchronous LLM quiz generation
  
  flashcards/         # Flashcard module backend
    handler.go
  
  fsrs/               # FSRS scheduling algorithm only
    scheduler.go      # CalculateNextReview, GetDueCards
  
  examiner/           # Examiner module backend
    handler.go
  
  rag/                # Retrieval and answering
    pipeline.go
    embeddings.go     # Vector storage
  
  llm/                # OpenAI-compatible client
    provider.go       # Synchronous only
```

### Package Ownership Rules

| Rule | Rationale |
|------|-----------|
| One responsibility per package | Clear boundaries |
| No cross-package orchestration | See `ARCHITECTURE.md` queue router pattern |
| State in SQLite, not code | Persistent queue architecture |
| Handlers are thin | UI logic in frontend, backend in `internal/` |

---

## Frontend (Vue)

```
frontend/src/
  pages/
    Dashboard.vue       # Shows pending tasks from queue
    Reader.vue          # Reading module (stateless)
    Quiz.vue            # Quiz module (stateless)
    Flashcards.vue      # Flashcard module (stateless)
    Examiner.vue        # Examiner module (stateless)
    Settings.vue        # Configuration
  
  components/
    Sidebar.vue         # Navigation
    TaskCard.vue        # Queue task display
    LoadingSpinner.vue  # For synchronous LLM calls
  
  services/
    appApi.js           # Backend bridge
    queueCoordinator.js # Queue API wrapper
  
  router/
    index.js            # Route definitions
```

### Module Contract

Each module receives:
- `task_id`: Current task identifier
- `block_id`: Content to render
- `related_id`: Topic context

Each module returns:
- Completion signal with result
- No routing decisions
- No task scheduling

---

## Data Flow

```
┌─────────────┐     ┌──────────────┐     ┌─────────────┐
│  Dashboard  │────▶│  GetNextTask │────▶│   Render    │
│             │     │  (queue API) │     │   TaskCard  │
└─────────────┘     └──────────────┘     └─────────────┘
                            │
       User clicks task ◄───┘
              │
              ▼
       ┌─────────────┐
       │ Route to    │
       │ module with │
       │ task context│
       └─────────────┘
              │
    ┌─────────┼─────────┐
    ▼         ▼         ▼
┌───────┐ ┌───────┐ ┌───────────┐
│Reader │ │ Quiz  │ │Flashcards│
└───┬───┘ └───┬───┘ └─────┬─────┘
    │         │           │
    │         ▼           │
    │    ┌─────────┐       │
    │    │Complete │       │
    │    │  Task   │       │
    │    └────┬────┘       │
    │         │            │
    └─────────┴────────────┘
              │
              ▼
       ┌─────────────┐
       │   Queue     │
       │    Router   │
       │ marks COMPLETE
       │ inserts next│
       └─────────────┘
```

---

## Queue Contract (V1)

Dashboard queries `study_queue` directly:

```sql
SELECT * FROM study_queue 
WHERE status = 'PENDING' 
ORDER BY priority ASC, created_at ASC;
```

Task shape:

| Field | Type | Description |
|-------|------|-------------|
| `id` | TEXT | Task UUID |
| `task_type` | TEXT | `READING`, `QUIZ`, `REREAD`, `FLASHCARD_REVIEW`, `EXAMINER` |
| `block_id` | TEXT | Content reference |
| `related_id` | TEXT | Topic reference |
| `status` | TEXT | `PENDING`, `ACTIVE`, `COMPLETED` |
| `priority` | INTEGER | Lower = higher priority |
| `created_at` | INTEGER | Unix timestamp |

---

## Debugging Rules

**If UI data is wrong:**
1. Check `study_queue` table: `SELECT * FROM study_queue WHERE status = 'PENDING'`
2. Check queue router service logs
3. Check module API response

**If flow is stuck:**
1. Check task status: `SELECT id, task_type, status FROM study_queue`
2. Check for errors in task completion

**If RAG fails:**
1. Check `blocks` table for content
2. Check `block_vectors` for embeddings
3. Verify `block_id` in task context




====================================================================================================
FILE: doc\RAG.md
ABSOLUTE: C:\Users\vishn\PROJECT\ai-tutor\doc\RAG.md
====================================================================================================

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

- Active `block_id` from current task context
- User question or explain request
- Topic content from `blocks` table (sliding window chunks)
- Token budget and output constraints

### Why

RAG must be deterministic about what it can see and how much it can send to the model.

### How

- The UI sends the active `block_id` with the request (from current task)
- Backend validates that the block exists
- Retrieval queries the `block_vectors` table filtered by `block_id` scope
- Return full block content for context (no parent expansion needed with sliding window)

## 4. Content Structure

### What

Source material is stored in **blocks** created by **sliding window chunking**:

- **Block**: Content unit of ~2500 words with 200-word overlap
- **Storage**: `blocks` table with `block_type = CHUNK`
- **Retrieval**: Top-k blocks from `block_vectors` within `block_id` scope

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

**Storage in `blocks` table:**

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
- Retrieve from `block_vectors` table
- Filter by `block_id` (from active task context)
- Expand to full block content before prompt assembly

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

Embeddings are stored in a `sqlite-vec` virtual table. Retrieval is simplified with block-based scope.

### Why

- SQLite extensions are connection-scoped, single persistent connection required
- The `sqlite-vec` virtual table requires integer rowids
- Simplified retrieval: no parent expansion needed with sliding window chunks

### How

**Storage:**
- Single SQLite connection with vec0 extension loaded (`db.Init()`)
- `block_vectors` table maps blocks to embeddings
- Embeddings stored as JSON strings for database/sql compatibility

**Retrieval (Simplified):**
1. Get `block_id` from current task context
2. Query `block_vectors` for that specific block's embedding
3. Calculate similarity to query embedding
4. Return block content directly (no parent expansion)

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

The retrieval layer depends on `blocks` and `block_vectors` tables.

### Why

RAG should be traceable back to the source material and the current study state.

### How

- `blocks` table stores content with `block_type = CHUNK`
- `block_vectors` stores embeddings by `block_id`
- Current task provides `block_id` for scoped retrieval
- UI shows block reference for traceability

**Schema:**

```sql
-- Content blocks (sliding window chunks)
CREATE TABLE blocks (
  id TEXT PRIMARY KEY,
  topic_id TEXT NOT NULL,
  block_type TEXT NOT NULL,  -- 'CHUNK', 'QUIZ', 'FLASHCARD'
  content TEXT,
  word_count INTEGER,
  order_index INTEGER,
  start_page INTEGER,
  end_page INTEGER,
  created_at INTEGER
);

-- Embeddings via sqlite-vec
CREATE VIRTUAL TABLE block_vectors USING vec0(
  embedding float[384]
);
```

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




====================================================================================================
FILE: doc\REQUIREMENTS.md
ABSOLUTE: C:\Users\vishn\PROJECT\ai-tutor\doc\REQUIREMENTS.md
====================================================================================================

# AI Tutor — Requirements

## Purpose

A **Persistent Guided Study Queue** - local-first desktop assistant for studying. Users upload documents, the system creates a deterministic queue of learning tasks (reading → quiz → review), and users work through the queue.

**NOT:** An autonomous AI tutor, hidden orchestration engine, or proactive scheduling system.

---

## Goals

- Allow users to upload documents (PDF, TXT, Markdown)
- **Sliding window chunking** creates deterministic content blocks (2500 words, 200 overlap)
- **Persistent queue**: `study_queue` table drives all user flows
- SQLite is the source of truth - no runtime-only state
- Synchronous LLM calls for quiz generation
- Queue-driven flashcard reviews (FSRS creates tasks, not orchestrates)
- Simple, inspectable, debuggable architecture
- Keep user data local by default

## Non-Goals

- Not a hosted, multi-user service (single-user, local-only)
- Not a full enterprise CMS
- Not a chatbot with conversation memory
- **Not LangChain/agent-orchestration based**
- **Not async/background job based** (synchronous MVP)
- **Not semantic chunking** (sliding window is sufficient)
- **Not proactive scheduling** (queue query is the scheduler)

## Users & Personas

- Individual learners who want an offline, private study assistant.
- Developers/researchers who want to run local RAG experiments and prototype workflows.

## Functional Requirements

1. Notebook Management
	- Upload files (PDF, .txt, .md) via the Notebook UI.
	- Support batch upload of multiple PDFs into a selected notebook in one action.
	- A notebook can contain many source files and many topics.
	- Store metadata: title, source filename, upload timestamp, optional topic_id.
	- List, preview, and delete notebooks from the UI.
	- Allow notebook/topic priority input with a user-friendly rating (for example 1-5 stars) and store it for scheduling.

2. Ingestion & Indexing
	- Parse uploaded files to extract text and metadata (page counts for PDFs)
	- **Sliding window chunking**: 2500-word chunks with 200-word overlap
	- **NO semantic chunking** - deterministic boundaries only
	- Persist blocks in `blocks` table with `block_type = CHUNK`
	- Write embeddings to `block_vectors` via `sqlite-vec`
	- **Insert READING tasks** into `study_queue` during ingestion
	- **Synchronous processing** - no background workers for MVP

3. RAG and LLM Features
	- Provide Reader view with Ask AI panel for contextual questions
	- **Synchronous quiz generation**: User clicks Complete → LLM called → Quiz returned directly
	- Generate flashcards from content (queue-driven reviews, not autonomous)
	- **Topic-scoped retrieval only** via `block_id` from current task
	- Enforce strict token budgets during prompt assembly
	- **All LLM calls stateless and synchronous**

4. Frontend
	- Vue-based pages: Notebook (upload/list), Reader, Flashcards, Quiz, Socratic, Settings.
	- Global notebook/topic scope selector consumed by feature pages.
	- Responsive upload control with drag/drop and clear CTA.
	- Ask AI appears as contextual assistance within Reader/Review flows, not as a general chat mode.

5. Backend/API
	- Wails desktop backend (Go) exposing methods: `UploadNotebook`, `GetNotebooks`, `DeleteNotebook`, and ingestion control endpoints.
	- `internal/notebook` service to handle safe file writes, sanitization, and metadata extraction.
	- `internal/db` repository to manage `notebooks` and `notebook_chunks` tables.
	- LLM provider uses a simple OpenAI-compatible interface (base_url, api_key, model, timeout) and avoids unnecessary abstractions.

6. Data Storage & Organization
	- Local-first storage under the per-user config directory (platform-specific path), e.g. `<config>/ai-tutor/`.
	- SQLite DB file (e.g. `ai-tutor.db`) and an `uploads/` folder for raw files.
	- Filenames saved using sanitized, UUID-prefixed names to avoid collisions.
	- Add patterns to `.gitignore` to prevent committing DB and uploads (`*.db`, `uploads/`, `.config/`).

7. Security & Privacy
	- Default behavior: all data stored locally and never uploaded externally without explicit user opt-in.
	- Consider optional encryption of the DB and files for advanced privacy use-cases.

8. Quality & Tooling
	- Code must pass formatter and `golangci-lint` checks; run linter as part of development workflow.
	- Unit tests for DB layer, chunker/tokenizer, and ingestion logic; integration tests for end-to-end ingestion and retrieval.

9. Queue-Driven Learning Workflow
	- **SQLite `study_queue` is the scheduler** - no separate scheduling engine
	- Dashboard queries queue: `SELECT * FROM study_queue WHERE status = 'PENDING' ORDER BY priority`
	- Task types: `READING`, `QUIZ`, `REREAD`, `FLASHCARD_REVIEW`, `EXAMINER`
	- **Orchestrator is thin**: fetches task, mounts module, marks complete, inserts follow-ups
	- Modules are **stateless**: no orchestration logic
	- Flashcard reviews: FSRS calculates due dates, orchestrator inserts `FLASHCARD_REVIEW` tasks
	- Remediation: Failed quiz inserts `REREAD` task (optional, user can skip)
	- Every task is one-click actionable with `block_id` context preloaded

## Non-Functional Requirements

- Cross-platform: Windows, macOS, Linux (packaged via Wails build process).
- Offline-capable: functional without network access except for optional external LLM/embedding providers.
- Lightweight: modest resource usage; background tasks should be rate-limited and cancelable.
- Maintainability-first code style: simple functions over deep abstractions; readability over cleverness.
- Windows packaging for local RAG must include required native libs (`onnxruntime.dll`, `vec0.dll`).

## Architecture Guardrails (Mandatory)

- **SQLite `study_queue` is the source of truth** - no runtime-only queues
- **Orchestrator is thin** - only routes tasks, no flow control
- **Modules are stateless** - no orchestration logic in Reader/Quiz/Flashcards
- Do not use LangChain or similar orchestration frameworks
- Use OpenAI-compatible APIs with minimal provider interface
- Keep AI calls **stateless and synchronous** (no async workers)
- Scope retrieval to current `block_id` (from task context)
- Enforce token limits strictly at prompt build time
- **Sliding window chunking only** - no semantic chunking
- In Go: avoid unnecessary interfaces, use structs, pointers only when needed
- UX guardrail: no chatbot mode; Ask AI is contextual inside reading/review flows
- **Deterministic MVP > premature optimization**

## Acceptance Criteria

### Queue System
- `study_queue` table exists with correct schema
- Dashboard queries `study_queue` for pending tasks
- Clicking task mounts correct module with `block_id` context
- Completing task updates status to `COMPLETED`
- Follow-up tasks insert correctly based on completion rules

### Ingestion
- PDF upload creates blocks via sliding window (2500 words, 200 overlap)
- No semantic chunking or AI-generated boundaries
- READING tasks auto-inserted into `study_queue` during ingestion
- Embeddings generated with ONNX Runtime and stored in `block_vectors`

### Quiz Flow (Synchronous)
- User clicks Complete → loading spinner shown
- Backend calls LLM synchronously
- Reading completion closes the reading task only
- Backend generates and activates the QUIZ follow-up task
- Dashboard may immediately surface the quiz task next

### Remediation
- Failed quiz (score < threshold) inserts REREAD task
- User can complete OR skip REREAD task
- No forced remediation loops

### Flashcards & FSRS
- FSRS calculates due dates only (not orchestrator)
- When cards due, `FLASHCARD_REVIEW` task inserted
- Dashboard shows flashcard task
- User ratings update FSRS state

### General
- Repo clean: database/uploads in `.gitignore`
- All Go code passes `golangci-lint`
- No runtime-only queues
- No background workers for MVP
- SQLite is source of truth

## Implementation Notes & Next Steps

- Implement robust PDF parsing (accurate page counts and text extraction) and token-aware chunker.
- Add a configurable vector store adapter and ensure chunk IDs are synchronized between SQLite and the vector store.
- Build background ingestion queue with progress and retry semantics.
- Create unit tests for DB operations, chunking, and ingestion worker; add CI steps to run linter and tests.
- Finalize UX: global scope selector, notebook preview, and consistent notebook layout across pages.

---

If you want, I can commit this file, run the linter/formatter, and/or implement the PDF parser next.




====================================================================================================
FILE: doc\SCHEMA.md
ABSOLUTE: C:\Users\vishn\PROJECT\ai-tutor\doc\SCHEMA.md
====================================================================================================

# AI Tutor Database Schema

## Overview

SQLite is the source of truth. The `study_queue` table drives all user flows. All tables support the persistent queue architecture.

---

## Core Queue Table

### `study_queue`

The central queue that drives all application flow.

| Field | Type | Description |
|-------|------|-------------|
| `id` | TEXT PRIMARY KEY | Unique task identifier (UUID) |
| `notebook_id` | TEXT NOT NULL | Reference to notebooks for priority biasing |
| `topic_id` | TEXT | Reference to topic for task context |
| `task_type` | TEXT NOT NULL | `READING`, `QUIZ`, `REREAD`, `FLASHCARD_REVIEW`, `EXAMINER` |
| `status` | TEXT NOT NULL | `PENDING`, `ACTIVE`, `COMPLETED`, `SKIPPED`, `FAILED` |
| `priority` | INTEGER DEFAULT 0 | Lower = higher priority (0 = urgent) |
| `created_at` | TIMESTAMP DEFAULT CURRENT_TIMESTAMP | When task was created |
| `activated_at` | TIMESTAMP | When task became ACTIVE (NULL if never active) |
| `completed_at` | TIMESTAMP | When task was completed (NULL if pending) |
| `payload_json` | TEXT | Optional JSON payload for task-specific data |
| `start_page` | INTEGER | Start page for READING tasks |
| `end_page` | INTEGER | End page for READING tasks |

**Indexes:**
```sql
CREATE INDEX idx_study_queue_status_priority_created ON study_queue(status, priority, created_at);
CREATE INDEX idx_study_queue_notebook_status ON study_queue(notebook_id, status);
```

**Task Types:**

| Type | Purpose | Created By |
|------|---------|------------|
| `READING` | Read a content block | Ingestion pipeline |
| `QUIZ` | Take a generated quiz | Reading completion |
| `REREAD` | Revisit material (remediation) | Failed quiz |
| `FLASHCARD_REVIEW` | Review due flashcards (block-level) | FSRS scheduler |
| `EXAMINER` | Written assessment | Mastery threshold |

**Task Status Values:**

| Status | Meaning | Transition |
|--------|---------|------------|
| `PENDING` | Waiting in queue | → ACTIVE (on open) |
| `ACTIVE` | Currently being worked | → COMPLETED/SKIPPED/FAILED |
| `COMPLETED` | Successfully finished | Terminal |
| `SKIPPED` | User bypassed task | Terminal, auditable |
| `FAILED` | Quiz generation failed or error | Terminal, can retry |

---

## Content Tables

### `notebooks`

Top-level container for study materials (multi-notebook support).

| Field | Type | Description |
|-------|------|-------------|
| `id` | TEXT PRIMARY KEY | Unique notebook identifier |
| `title` | TEXT NOT NULL | Notebook name |
| `priority` | INTEGER DEFAULT 5 | 1-10 (higher = more frequent in queue) |
| `created_at` | INTEGER | Unix timestamp |
| `updated_at` | INTEGER | Unix timestamp |

### `topics`

Organizational unit for study material within a notebook.

| Field | Type | Description |
|-------|------|-------------|
| `id` | TEXT PRIMARY KEY | Unique topic identifier |
| `notebook_id` | TEXT NOT NULL | Parent notebook reference |
| `title` | TEXT NOT NULL | Topic name |
| `status` | TEXT | `unseen`, `reading`, `learned` |
| `start_page` | INTEGER | First page in source |
| `end_page` | INTEGER | Last page in source |
| `current_page_cursor` | INTEGER | Last read position |
| `created_at` | INTEGER | Unix timestamp |
| `updated_at` | INTEGER | Unix timestamp |

### `blocks`

Content blocks created by sliding window chunking.

| Field | Type | Description |
|-------|------|-------------|
| `id` | TEXT PRIMARY KEY | Unique block identifier |
| `topic_id` | TEXT NOT NULL | Parent topic reference |
| `block_type` | TEXT NOT NULL | `CHUNK`, `QUIZ`, `FLASHCARD` |
| `content` | TEXT | Text content or JSON payload |
| `word_count` | INTEGER | For progress tracking |
| `order_index` | INTEGER | Sequence within topic |
| `start_page` | INTEGER | Source page start |
| `end_page` | INTEGER | Source page end |
| `reread_count` | INTEGER | Number of reread cycles completed |
| `created_at` | INTEGER | Unix timestamp |

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
| `reread_count` | Number of reread cycles completed |

**Indexes:**
```sql
CREATE INDEX idx_blocks_topic ON blocks(topic_id, order_index);
CREATE INDEX idx_blocks_type ON blocks(block_type, topic_id);
```

### `block_vectors`

Embedding storage via sqlite-vec virtual table.

| Field | Type | Description |
|-------|------|-------------|
| `block_id` | TEXT | Reference to blocks table |
| `embedding` | JSON | Float32 vector as JSON string |

---

## Quiz Tables

### `quiz_sets`

Generated quiz content.

| Field | Type | Description |
|-------|------|-------------|
| `id` | TEXT PRIMARY KEY | Quiz set identifier |
| `topic_id` | TEXT NOT NULL | Parent topic |
| `block_id` | TEXT | Source block reference |
| `payload_json` | TEXT NOT NULL | Quiz questions/answers JSON |
| `created_at` | INTEGER | Unix timestamp |
| `score_threshold` | INTEGER | Pass threshold (default 70) |

### `quiz_attempts`

User quiz submissions.

| Field | Type | Description |
|-------|------|-------------|
| `id` | TEXT PRIMARY KEY | Attempt identifier |
| `quiz_set_id` | TEXT NOT NULL | Reference to quiz_sets |
| `score` | INTEGER | Percentage score (0-100) |
| `passed` | BOOLEAN | Score >= threshold |
| `answers_json` | TEXT | User answers |
| `completed_at` | INTEGER | Unix timestamp |

---

## Flashcard Tables

### `fsrs_cards`

Individual flashcards with FSRS state.

| Field | Type | Description |
|-------|------|-------------|
| `id` | TEXT PRIMARY KEY | Card identifier |
| `topic_id` | TEXT NOT NULL | Parent topic |
| `block_id` | TEXT | Source content block |
| `prompt` | TEXT NOT NULL | Front of card |
| `answer` | TEXT NOT NULL | Back of card |
| `state_json` | TEXT | FSRS scheduling state |
| `due_at` | INTEGER | Next review timestamp |
| `created_at` | INTEGER | Unix timestamp |

**Indexes:**
```sql
CREATE INDEX idx_fsrs_due ON fsrs_cards(due_at);
CREATE INDEX idx_fsrs_topic ON fsrs_cards(topic_id);
```

### `fsrs_review_log`

Audit trail of all reviews.

| Field | Type | Description |
|-------|------|-------------|
| `id` | INTEGER PRIMARY KEY | Auto-increment |
| `card_id` | TEXT NOT NULL | Reference to fsrs_cards |
| `rating` | INTEGER | 1=Again, 2=Hard, 3=Good, 4=Easy |
| `before_state` | TEXT | FSRS state before review |
| `after_state` | TEXT | FSRS state after review |
| `reviewed_at` | INTEGER | Unix timestamp |

---

## Source Tables

### `sources`

Original uploaded files.

| Field | Type | Description |
|-------|------|-------------|
| `id` | TEXT PRIMARY KEY | Source file identifier |
| `filename` | TEXT NOT NULL | Original filename |
| `file_path` | TEXT | Local storage path |
| `file_type` | TEXT | `pdf`, `txt`, `md` |
| `page_count` | INTEGER | Total pages |
| `created_at` | INTEGER | Unix timestamp |

---

## Configuration Tables

### `app_config`

User settings and preferences.

| Field | Type | Description |
|-------|------|-------------|
| `key` | TEXT PRIMARY KEY | Config key |
| `value` | TEXT | Config value |

---

## Schema Design Principles

### 1. Queue-Centric

All user flows originate from `study_queue`. The dashboard queries:

```sql
SELECT * FROM study_queue 
WHERE status = 'PENDING' 
ORDER BY priority ASC, created_at ASC 
LIMIT 1;
```

### 2. Deterministic Task Types

Task types are explicit enums, not dynamic strings:
- `READING` - Content consumption
- `QUIZ` - Knowledge assessment
- `REREAD` - Remediation
- `FLASHCARD_REVIEW` - Spaced repetition
- `EXAMINER` - Written assessment

### 3. Block-Based Content

All content stored in `blocks` table with uniform schema:
- `CHUNK` blocks for reading
- `QUIZ` blocks for assessments
- `FLASHCARD` blocks for review

### 4. FSRS Integration

FSRS is data-only:
- Calculates due dates
- Updates `state_json` on cards
- Creates `FLASHCARD_REVIEW` tasks when `due_at <= now`
- Does NOT orchestrate flow

### 5. Audit Trail

Key tables have review logs:
- `fsrs_review_log` - All card reviews
- `quiz_attempts` - All quiz submissions
- `app_events` (optional) - System events

### 6. Multi-Notebook Priority Biasing

Notebooks have priority (1-10, default 5). Higher priority notebooks surface more frequently in the queue.

Queue ordering applies this priority hierarchy FIRST, then notebook priority as bias:

| Order | Task Type | Rationale |
|-------|-----------|-----------|
| 1 | `FLASHCARD_REVIEW` (due reviews) | Spaced repetition is time-sensitive |
| 2 | `REREAD` | Remediation should be timely |
| 3 | `QUIZ` | Assessment follows reading |
| 4 | `READING` | New material after obligations |
| 5 | `EXAMINER` | Optional advanced assessment |

Within each tier, notebook priority biases ordering:

```sql
-- Priority hierarchy with notebook bias
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

### 7. Task Lifecycle Semantics

Explicit state transitions:

```
PENDING → ACTIVE (when user opens task)
ACTIVE → COMPLETED (on success)
ACTIVE → SKIPPED (on user skip)
ACTIVE → FAILED (on error/generation failure)
```

**Crash Recovery:**
- ACTIVE tasks older than timeout threshold (e.g., 30 minutes) revert to PENDING on startup
- This ensures restart-safe queue recovery
- `activated_at` timestamp tracks when task became active

```sql
-- Crash recovery: reset stale ACTIVE tasks
UPDATE study_queue 
SET status = 'PENDING', activated_at = NULL
WHERE status = 'ACTIVE' 
  AND activated_at < (strftime('%s', 'now') - 1800); -- 30 min timeout
```

### 8. Dashboard Starvation Protection

To prevent review monopolization (e.g., 500 flashcards blocking all reading):

**Deterministic Balancing Rule:**
After N review tasks (`FLASHCARD_REVIEW` or `REREAD`), allow one `READING` task.

Recommended: N = 5 (after 5 reviews, surface 1 reading)

This is a lightweight query-time bias, NOT autonomous orchestration.

**Balancing rules are static SQL ordering constraints, not adaptive runtime systems.**

### 9. Reading Completion (Trust-Based)

Reading tasks use trust-based completion:

- User decides when reading is complete
- Complete Session button stays enabled during active reading task
- `start_page` is authoritative for opening context
- `end_page` is informational only (for reference)
- `current_page_cursor` tracked for informational progress only
- No enforced page-completion validation
- No `currentPage >= endPage` gating
- No surveillance logic, timers, or engagement tracking

### 10. Flashcard Review Granularity

**One `FLASHCARD_REVIEW` task = one review session for a block/chunk.**

- Do NOT create one queue task per flashcard
- A single task represents "review all due cards in this block"
- Prevents queue explosion with many cards

---

## What This Replaces

| Old Approach | New Approach |
|--------------|--------------|
| Runtime-only queues | `study_queue` table |
| Hidden orchestrators | Explicit orchestrator service |
| In-memory session engines | Persistent SQLite state |
| Proactive scheduling | Query-driven task fetching |
| Complex state machines | Status enum transitions |

---

## Query Examples

### Get Dashboard Tasks
```sql
SELECT 
  sq.id,
  sq.task_type,
  sq.priority,
  t.title as topic_title,
  b.word_count
FROM study_queue sq
LEFT JOIN topics t ON sq.related_id = t.id
LEFT JOIN blocks b ON sq.block_id = b.id
WHERE sq.status = 'PENDING'
ORDER BY sq.priority ASC, sq.created_at ASC;
```

### Get Reading Progress
```sql
SELECT 
  COUNT(CASE WHEN status = 'COMPLETED' THEN 1 END) as completed,
  COUNT(*) as total
FROM study_queue
WHERE task_type = 'READING' AND related_id = ?;
```

### Get Due Flashcards (Create Tasks)
```sql
SELECT * FROM fsrs_cards 
WHERE due_at <= strftime('%s', 'now');
```

### Mark Task Complete
```sql
UPDATE study_queue 
SET status = 'COMPLETED', completed_at = strftime('%s', 'now')
WHERE id = ?;
```




====================================================================================================
FILE: doc\SPRINT.md
ABSOLUTE: C:\Users\vishn\PROJECT\ai-tutor\doc\SPRINT.md
====================================================================================================

# SPRINT.md — AI Tutor

**Status:** Active roadmap for Persistent Queue Architecture  
**Last Updated:** 2026-05-08  
**Architecture:** SQLite-backed deterministic queue (NOT autonomous orchestration)

---

## Architecture Foundation

This application is: **A Persistent Guided Study Queue**

NOT:
- An autonomous AI tutor
- A mission engine  
- A hidden orchestrator
- A proactive scheduler

**Core Principle:** Advanced learning systems are **Data, not Engines**.

- Quizzes create queue tasks
- FSRS creates review tasks
- Remediation creates reread tasks  
- Examiner creates assessment tasks

**None of these systems own orchestration.** SQLite is the single source of truth.

---

## Sprint Implementation Rule

Each sprint implementation must:

1. Read only the directly relevant documentation
2. Respect AGENTS.md hierarchy
3. Avoid introducing new architecture patterns
4. Preserve deterministic queue behavior
5. Prefer explicit state transitions over hidden automation

---

## Queue Model

All progression flows through: `study_queue`

**Task Lifecycle:**
```
PENDING → ACTIVE → COMPLETED
           ↓
        FAILED / SKIPPED
```

**Task Types (Priority Order):**
1. `FLASHCARD_REVIEW` — Highest priority
2. `REREAD` — Remediation tasks
3. `QUIZ` — Assessment tasks  
4. `READING` — Content consumption
5. `EXAMINER` — Mastery verification

**Queue Ordering Rules:**
1. Task type priority (as above)
2. Notebook priority (higher = more frequent)
3. Task priority (explicit override)
4. Creation time (FIFO within tier)

**Note:** Ordering is evaluated deterministically at query time, not via background mutation.

**Notebook Priority Biasing:**
- Higher priority notebooks appear more frequently in queue ordering
- Lower priority notebooks still surface (starvation prevention)
- Priority is deterministic ordering bias, NOT autonomous scheduling

---

## Sprint Roadmap

---

### Sprint 1: Queue Foundation

**Goal:** Establish the `study_queue` schema and core task lifecycle.

**Required Context:**

- **Documentation:** SCHEMA.md, DATA_API.md, ARCHITECTURE.md
- **Agent Instructions:** /AGENTS.md
- **Relevant Packages:** internal/db/
- **Important Constraints:** No hidden queue mutation, queue ordering must remain deterministic

**Schema Requirements:**
```sql
CREATE TABLE study_queue (
    id TEXT PRIMARY KEY,
    notebook_id TEXT NOT NULL,
    topic_id TEXT,
    task_type TEXT NOT NULL,  -- FLASHCARD_REVIEW, REREAD, QUIZ, READING, EXAMINER
    status TEXT NOT NULL,     -- PENDING, ACTIVE, COMPLETED, FAILED, SKIPPED
    priority INTEGER DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    activated_at TIMESTAMP,
    completed_at TIMESTAMP,
    -- Task-specific payload (JSON)
    payload_json TEXT,
    -- For reading tasks: page bounds
    start_page INTEGER,
    end_page INTEGER,
    -- Foreign keys
    FOREIGN KEY (notebook_id) REFERENCES notebooks(id),
    FOREIGN KEY (topic_id) REFERENCES topics(id)
);

CREATE INDEX idx_study_queue_status_priority_created 
    ON study_queue(status, priority, created_at);
CREATE INDEX idx_study_queue_notebook_status 
    ON study_queue(notebook_id, status);
```

**API Surface:**
- `GetNextTask(notebookID string) StudyTask` — Fetch next pending task by ordering rules
- `ActivateTask(taskID string) error` — Move PENDING → ACTIVE
- `CompleteTask(taskID string, result CompletionResult) error` — Move ACTIVE → COMPLETED, trigger follow-up insertion
- `SkipTask(taskID string) error` — Move to SKIPPED (user-initiated)
- `GetQueueState(notebookID string) QueueState` — Pending count by task type

**Deliverables:**
- [ ] `study_queue` table and indexes
- [ ] Queue repository with CRUD operations
- [ ] Task lifecycle state machine
- [ ] Basic Wails bindings for task operations
- [ ] Queue state query for Dashboard

---

### Sprint 2: Reading Flow (Trust-Based)

**Goal:** Implement trust-based reading tasks with simple completion flow.

**Required Context:**

- **Documentation:** SCHEMA.md, APP_FLOW.md
- **Agent Instructions:** /AGENTS.md, /internal/AGENTS.md
- **Relevant Packages:** internal/db/, frontend/src/pages/
- **Important Constraints:** Trust-based completion, no engagement surveillance, no enforced validation

**Reading Task Flow:**
1. User opens reading task from queue
2. PDF viewer opens to `start_page` (authoritative entry point)
3. User reads freely within assigned range
4. User clicks Complete Session when ready (button always enabled)
5. Backend marks task complete → QUIZ task inserted

**Trust-Based Completion:**
- User decides when reading is complete
- Complete Session button stays enabled during active reading task
- `start_page` is authoritative for opening context
- `end_page` is informational only
- No enforced page-completion validation
- No `currentPage >= endPage` gating
- No `can_complete` enforcement logic

**API Surface:**
- `GetReadingTask(taskID string) ReadingTask` — Get task with page bounds
- `CompleteReading(taskID string) error` — Complete task (trust-based), trigger quiz insertion

**Schema Additions:**
```sql
-- reading_progress tracks per-task progress (restart-safe)
CREATE TABLE reading_progress (
    task_id TEXT PRIMARY KEY,
    current_page INTEGER DEFAULT 0,
    last_accessed_at TIMESTAMP,
    FOREIGN KEY (task_id) REFERENCES study_queue(id)
);
```

**Rules:**
- Trust-based completion (user decides when done)
- NO engagement surveillance (no timers, no scroll tracking)
- NO completion validation or gating
- Reader module does NOT own progression semantics
- Dashboard owns workflow routing

**Deliverables:**
- [ ] Reading task payload with page bounds
- [ ] PDF viewer with start_page opening
- [ ] Reading progress persistence (optional)
- [ ] Trust-based completion (always-enabled Complete button)
- [ ] Quiz task auto-insertion on completion

---

### Sprint 3: Synchronous Quiz Generation

**Goal:** Implement quiz generation as synchronous, queue-triggered flow.

**Required Context:**

- **Documentation:** SCHEMA.md, DATA_API.md, AGENT_MAP.md
- **Agent Instructions:** /AGENTS.md, /internal/AGENTS.md
- **Relevant Packages:** internal/db/, internal/llm/, frontend/src/pages/
- **Important Constraints:** Synchronous generation only, no background quiz generation

**Quiz Flow:**
1. User completes reading task
2. Frontend shows loading spinner
3. Backend synchronously calls LLM for question generation
4. QUIZ task created with generated questions in payload
5. User proceeds to quiz UI
6. User submits answers → scored immediately

**API Surface:**
- `GenerateQuizSync(topicID string, chunkIDs []string) (QuizTask, error)` — Synchronous generation
- `SubmitQuizAttempt(taskID string, answers []Answer) QuizResult` — Score and record

**Quiz Task Payload:**
```json
{
  "questions": [
    {
      "id": "q_...",
      "prompt": "What is...",
      "options": ["A", "B", "C", "D"],
      "correct_answer": "B",
      "source_chunk_id": "chk_..."
    }
  ],
  "passing_score": 70
}
```

**Scoring Outcomes:**
- Score >= threshold → Mark COMPLETED, optionally insert FLASHCARD_REVIEW
- Score < threshold → Insert REREAD task, generate lightweight AI feedback

**Rules:**
- Synchronous generation — queue waits, user sees spinner
- NO background/async quiz generation
- Questions stored in task payload (ephemeral, not persisted to questions table until scored)

**Deliverables:**
- [ ] Synchronous quiz generation endpoint
- [ ] Quiz task payload structure
- [ ] Quiz UI with loading state
- [ ] Immediate scoring and feedback
- [ ] Conditional reread insertion on failure

---

### Sprint 4: Reread Remediation & Loop Protection

**Goal:** Implement reread tasks with retry limits to prevent infinite loops.

**Required Context:**

- **Documentation:** SCHEMA.md, DATA_API.md
- **Agent Instructions:** /AGENTS.md, /internal/AGENTS.md
- **Relevant Packages:** internal/db/
- **Important Constraints:** Max reread attempts must be enforced, queue progression must continue after max failures

**Reread Flow:**
1. Quiz score below threshold
2. REREAD task inserted for same content
3. User completes reread
4. New QUIZ task generated
5. If still failing after max attempts → stop automatic insertion

**Loop Protection:**
```sql
-- Track reread attempts per topic
CREATE TABLE reread_attempts (
    topic_id TEXT PRIMARY KEY,
    attempt_count INTEGER DEFAULT 0,
    last_attempt_at TIMESTAMP
);
```

**Config:**
- `max_reread_attempts = 3`

**After Max Failures:**
- Task marked COMPLETED (no further auto-remediation)
- Manual review recommended to user
- Queue progression continues with next task

**API Surface:**
- `InsertRereadTask(notebookID, topicID string, reason string) error`
- `CheckRereadLimit(topicID string) (attempts int, allowed bool)`

**Deliverables:**
- [ ] Reread task type and payload
- [ ] Reread attempt tracking table
- [ ] Max attempt enforcement
- [ ] Manual review recommendation UI
- [ ] Queue progression after max failures

---

### Sprint 5: Flashcard Review Tasks

**Goal:** Integrate FSRS with queue — due cards become review tasks.

**Required Context:**

- **Documentation:** SCHEMA.md, DATA_API.md, AGENT_MAP.md
- **Agent Instructions:** /AGENTS.md, /internal/AGENTS.md
- **Relevant Packages:** internal/db/, internal/study/
- **Important Constraints:** FSRS is scheduling algorithm only, not orchestrator; one task per review session not per card

**FSRS Role Clarification:**
- FSRS is ONLY: scheduling algorithm + interval calculator
- FSRS is NOT: orchestrator, mission engine, hidden scheduler

**Review Task Model:**
- One `FLASHCARD_REVIEW` task = one review session
- NOT one task per card (prevents queue explosion)
- Task payload contains list of due cards for the session

**Daily Flow:**
1. On dashboard load or explicit refresh: Query `fsrs_cards` for due cards
2. Group by notebook, create `FLASHCARD_REVIEW` tasks
3. Tasks enter queue at highest priority
4. User activates task → review session begins
5. Each card rating updates FSRS state
6. Session complete → mark task COMPLETED

**API Surface:**
- `GenerateReviewTasks(notebookID string) ([]StudyTask, error)` — Create tasks for due cards
- `GetReviewSession(taskID string) ReviewSession` — Get cards for this session
- `RecordCardReview(taskID, cardID string, rating int) error` — Update FSRS state
- `CompleteReviewSession(taskID string) error` — Mark task done

**Schema:**
```sql
-- Link review tasks to cards being reviewed
CREATE TABLE review_task_cards (
    task_id TEXT,
    card_id TEXT,
    status TEXT DEFAULT 'pending', -- pending, reviewed
    PRIMARY KEY (task_id, card_id)
);
```

**Rules:**
- One session task can contain 10-20 cards (configurable)
- Cards due together are batched into same session
- Queue priority ensures review happens before new reading

**Deliverables:**
- [ ] FLASHCARD_REVIEW task type
- [ ] Due card query and batching
- [ ] Review session payload structure
- [ ] FSRS rating integration (existing code)
- [ ] Session completion flow

---

### Sprint 6: Examiner Tasks & Mastery Triggers

**Goal:** Implement Examiner mode as queue-driven optional tasks.

**Required Context:**

- **Documentation:** SCHEMA.md, DATA_API.md, AGENT_MAP.md
- **Agent Instructions:** /AGENTS.md, /internal/AGENTS.md
- **Relevant Packages:** internal/db/, internal/assessment/
- **Important Constraints:** No hidden examiner orchestration, tasks are optional and queue-driven

**Examiner Tasks:**
- Inserted after mastery thresholds (e.g., 3 quizzes passed at 90%+)
- Appear naturally in queue at priority 5 (lowest)
- Optional — user can skip without penalty
- NOT interrupting, NOT autonomous

**Mastery Detection:**
```sql
-- Simple threshold-based trigger
SELECT topic_id, COUNT(*) as passed_count
FROM user_answers ua
JOIN questions q ON ua.question_id = q.id
WHERE ua.score >= 90
GROUP BY q.topic_id
HAVING passed_count >= 3;
```

**Examiner Task Payload:**
```json
{
  "written_question_ids": ["wq_...", "wq_..."],
  "triggered_by": "mastery_threshold",
  "optional": true
}
```

**API Surface:**
- `CheckMasteryTriggers(notebookID string) []MasteryTrigger` — Detect thresholds
- `InsertExaminerTask(notebookID, topicID string) error`
- `GetWrittenQuestions(taskID string) []WrittenQuestion`
- `SubmitWrittenAnswer(taskID, questionID, answer string) WrittenScore`

**Rules:**
- NO hidden examiner orchestration
- NO autonomous examiner flows
- Tasks are optional, queue-driven, user-initiated

**Deliverables:**
- [ ] Examiner task type and payload
- [ ] Mastery threshold detection
- [ ] Optional task handling (skip allowed)
- [ ] Written question integration
- [ ] Queue-driven examiner flow

---

### Sprint 7: Queue Balancing & Polish

**Goal:** Ensure fair queue distribution and recovery robustness.

**Required Context:**

- **Documentation:** SCHEMA.md, DATA_API.md
- **Agent Instructions:** /AGENTS.md, /internal/AGENTS.md
- **Relevant Packages:** internal/db/, internal/study/
- **Important Constraints:** Queue ordering must remain deterministic, no background queue mutation daemons

**Queue Balancing:**

1. **Starvation Prevention**
   - Lower priority notebooks get minimum quota
   - Config: `min_tasks_per_notebook_per_day = 2`

2. **Priority Decay (Query-Time Only)**
   - Old PENDING tasks get higher priority in ordering calculation
   - Implemented as SQL ORDER BY logic, NOT background mutation
   - Prevents infinite deferral while remaining deterministic

3. **Session Boundaries**
   - Configurable max tasks per session: `max_session_tasks = 10`
   - Soft limit — user can continue if desired

**Crash Recovery:**

1. **ACTIVE Task Handling**
   - On startup: Mark stale ACTIVE tasks back to PENDING
   - Stale threshold: `task_active_timeout = 24 hours`

2. **Reading Progress Recovery**
   - `reading_progress` table preserves cursor
   - User resumes at last page on restart

3. **Quiz Generation Idempotency**
   - Quiz generation keyed by (task_id, attempt_num)
   - Re-generation on crash produces identical questions

**API Surface:**
- `ApplyQueueOrderingRules(notebookID string) error` — Apply priority adjustments (query-time only)
- `RecoverStaleTasks() error` — Mark timed-out ACTIVE tasks
- `GetQueueStats() QueueStats` — Per-notebook pending counts

**Monitoring:**
```sql
-- Health check queries
SELECT notebook_id, task_type, status, COUNT(*) 
FROM study_queue 
GROUP BY notebook_id, task_type, status;
```

**Deliverables:**
- [ ] Starvation prevention logic
- [ ] Priority decay for old tasks
- [ ] Session task limits
- [ ] Stale task recovery
- [ ] Queue health monitoring
- [ ] Crash-resilient reading progress

---

## Technical Implementation Notes

### Queue Router

The queue router ONLY:
1. Fetches next pending task via ordering rules
2. Mounts correct module based on `task_type`
3. Marks tasks complete when module signals completion
4. Inserts follow-up tasks per completion rules

It does NOT:
- Dynamically generate agendas
- Proactively schedule sessions
- Own remediation systems
- Run hidden orchestration logic

### Dashboard Role (Revised)

The Dashboard is now:
- A deterministic task launcher

It is NOT:
- A mission planner
- A scheduling engine
- An AI agenda system

Dashboard simply:
1. Fetches next queue task
2. Displays queue state (counts by type)
3. Launches task modules on user action

### Ingestion Pipeline (Retained)

Current pipeline remains:
- PDF upload → chapter extraction → chapter pruning

Chunking strategy:
- 2500-word chunks
- 200-word overlap

**Explicitly removed:**
- Semantic topic chunking
- AI-generated chunk boundaries
- Autonomous chunk planning

---

## Terminology Guide

| Use This | NOT This |
|----------|----------|
| `study_queue` | DailyAgenda |
| Task type | Mission type |
| Queue ordering | Scheduling engine |
| Task lifecycle | Orchestration flow |
| Priority bias | Autonomous prioritization |
| Deterministic | AI-driven |
| Insert task | Generate mission |
| Activate task | Launch session |
| Complete task | Finish mission |
| FSRS algorithm | FSRS orchestrator |
| Reading task | Encoding phase |
| Quiz task | Assessment mission |
| Notebook priority | Study plan weight |

---

## Definition of Done (All Sprints)

Each sprint is complete when:

1. Schema migrations applied (if any)
2. Repository layer implemented with tests
3. Wails bindings exposed
4. Frontend UI wired (if applicable)
5. `go test ./...` passes
6. `wails dev` smoke test passes
7. No deprecated orchestration terminology in code/comments

---

## Current Status

- **Sprint 1:** Not started — Queue schema design complete
- **Sprint 2-7:** Planned, pending Sprint 1 completion

---

*For historical sprints (pre-queue architecture), see `doc/SPRINT_HISTORY.md`.*




====================================================================================================
FILE: doc\SPRINT_HISTORY.md
ABSOLUTE: C:\Users\vishn\PROJECT\ai-tutor\doc\SPRINT_HISTORY.md
====================================================================================================

# SPRINT_HISTORY.md — AI Tutor

Created: 2026-04-12

This file is a single canonical history of completed sprints. Use this for onboarding, release notes, and auditing changes across sprints. Each entry includes goals, outcomes, key files changed, API/UI surface changes, test status, and short TODOs.

---

## Sprint 1 — UI Shell & Navigation
- Completed: by 2026-04-11
- Goal: Build a minimal, navigable UI shell with primary pages (Dashboard, Reader, Quiz, Flashcards, Socratic).
- Outcome: Full Vue + Wails UI skeleton with sidebar and routes.
- Key files changed:
  - frontend/src/App.vue
  - frontend/src/components/Sidebar.vue
  - frontend/src/pages/*.vue (Dashboard, Reader, Quiz, Flashcards, Socratic)
  - wails.json
- API / UI changes: None (UI-only scaffold), routes and page components added.
- Tests status: Manual UI validation; no backend tests required for this sprint.
- TODOs: N/A

---

## Sprint 2 — Reader + Basic RAG (Ask AI)
- Completed: by 2026-04-11
- Goal: Implement Reader page with RAG retrieval + LLM (Ask AI) integration.
- Outcome: Working retrieval pipeline, LLM prompt assembly, and Reader UI connected via Wails bindings.
- Key files changed:
  - internal/rag/* (RAG pipeline and retrieval code)
  - internal/llm/* (LLM provider adapter)
  - app.go (exposed APIs: `GetTopicContent`, `GetAvailableTopics`, `AskAI`)
  - frontend/src/pages/Reader.vue
- API / UI changes: `AskAI(topicID, question)` added; Reader page shows citations and sections.
- Tests status: Unit/integration tests for retrieval and backend pass in CI-local runs.
- TODOs: Continue to improve retrieval quality and fallback heuristics.

---

## Sprint 3 — Notebook Ingestion & Embeddings
- Completed: by 2026-04-11
- Goal: Accept uploaded documents, extract sections, chunk text deterministically, ingest to DB, and index vectors.
- Outcome: Notebook upload, extraction, deterministic chunking, transactional ingestion, topic extraction, and background indexing.
- Key files changed:
  - internal/notebook/upload.go
  - internal/db/store.go
  - internal/embeddings/onnx.go
  - notebook_endpoints.go (upload & ingestion events)
- API / UI changes: Notebook upload UI and ingestion progress events; `GetNotebooks()` and ingestion endpoints available.
- Tests status: Integration tests for ingestion and DB rollback behavior pass (Windows-friendly cleanup included).
- TODOs: Improve chapter/topic extraction quality and UI for notebook→topic linking.

---

## Sprint 4 — Quiz Generation (Condensed)
- Completed: 2026-04-11 → 2026-04-12
- Goal: Generate topic-scoped multiple-choice quizzes, score answers, and persist attempts for later review.
- Outcome: LLM-based MCQ generation (strict JSON), storage of questions and user attempts, answer scoring, and Quiz UI wired end-to-end.
- Key files changed:
  - app.go (GenerateQuiz, ScoreAnswer, prompt assembly)
  - internal/db/quiz_repo.go (quiz persistence)
  - internal/models/models.go (QuizQuestion / QuizScore types)
  - frontend/src/pages/Quiz.vue (notebook → topic selector + quiz UI)
  - frontend/src/services/appApi.js (bridge functions)
  - internal/rag/indexer.go and internal/db/vector_repo.go (related vector/persistence fixes)
- API / UI changes:
  - `GenerateQuiz(topicID)` and `ScoreAnswer(questionID, userAnswer)` added
  - Quiz page updated to notebook-first cascade selector (notebook → topic)
- Tests status: Backend tests pass (`go test ./...`); frontend build passes; linting clean.
- TODOs: End-to-end runtime validation via `wails dev`; optional improvements include difficulty tagging and quiz history.

---

## Sprint 6 — FSRS Review UI + Backend Wiring
- Completed: 2026-04-14
- Goal: Connect Dashboard and Flashcards UI to FSRS backend and record review ratings.
- Outcome: Dashboard surfaces due-count from the daily plan; Flashcards page sends rating choices and shows next scheduled review.
- Key files changed:
  - frontend/src/pages/Dashboard.vue
  - frontend/src/pages/Flashcards.vue
  - frontend/src/services/appApi.js
  - app.go (`GetTodayPlan`, `GetFlashcards`, `RecordFlashcardReview`)
  - internal/scheduler/service.go
  - internal/db/flashcard_repo.go
  - internal/db/store.go
- API / UI changes:
  - `GetTodayPlan()` added
  - `GetFlashcards(topicID, true)` wired to due-card loading
  - `RecordFlashcardReview(cardID, rating)` wired to review actions
- Tests status: Backend db and scheduler tests pass; frontend review flow wired.
- TODOs: Validate full Wails end-to-end flow; polish review copy and dashboard messaging.

---

## How to run / verify locally

1. Start dev app (requires assets and env vars):

```bash
export LLM_BASE_URL=... LLM_API_KEY=... LLM_MODEL=...
cd <repo-root>
wails dev -tags sqlite_extension
```

2. Run backend tests:

```bash
go test ./...
```

3. Build frontend separately (if needed):

```bash
npm --prefix frontend run build
```

## Current State

**Sprints 1–3: Complete.**

Delivered the UI shell (all pages navigable), Ask AI in the Reader (RAG-based retrieval + LLM), and Socratic tutor mode (guided follow-up questions instead of direct answers). Backend uses SQLite, lexical retrieval, and OpenAI-compatible LLM calls. Frontend wires results via Wails bindings. No LangChain, no chat memory, no over-engineering.

PR size: ~6900 lines across backend/frontend/database because the work spans SQLite schema, embeddings, RAG pipeline, UI pages, and Wails bindings for each feature. UI page like Socratic.vue runs 535 lines on its own (multi-section state, styling, API calls). Normal for full-stack without scaffolding tools.

---

# Sprint 4 — Quiz Generation

**Status: Completed — 2026-04-12.** See `doc/SPRINT_HISTORY.md` for full details.

## Goal

Generate quiz questions from reading material and score answers.

---

For more details see `doc/solutions/SOLUTIONS_2026-04-11.md` and the linked code in `internal/`.

## Core Work

1. **FSRS algorithm**
   - Implement FSRS (or integrate proven library) in Go
   - Calculate next review date based on answer quality
   - Store review history in SQLite

2. **Flashcards page**
   - Show cards due for review today
   - User marks each as easy/good/hard
   - App calculates next review and moves to next card
   - Display running stats (cards learned, cards in learning, new cards)

3. **Progress dashboard**
   - Total cards reviewed
   - Cards mastered
   - Review calendar for next 30 days
   - Streak (optional)

4. **Data model**
   - Link quiz answers to FSRS state
   - Track intervals and difficulty of each card
   - Persist all review history

## Backend API

- `GetFlashcards(topicID string, dueOnly bool) map[string]interface{}` – returns cards for a topic, optionally filtered by due status
- `RecordFlashcardReview(cardID string, rating string) map[string]interface{}` – updates FSRS state and returns review result
- SQLite: `fsrs_cards` and `fsrs_review_log` tables

## Workflow

1. User answers quiz → stored in `user_answers`
2. First quiz answer creates flashcard entry in `card_state` (new)
3. User reviews on Flashcards page, marks easy/good/hard
4. Backend recalculates interval, updates `review_history` and `card_state`
5. Dashboard pulls from `card_state` for progress counts

## Dependencies

- Quiz scores feed into cards (no quiz changes needed)
- Reader unchanged
- Ask AI unchanged

## Definition of Done

- Flashcards page shows due cards
- FSRS calculation works (mark easy/good/hard)
- Next review date updates correctly
- Dashboard shows progress
- Data persists across sessions

---

## Architecture Rules

Across all sprints:

- No LangChain, no complex orchestration
- LLM calls are direct HTTP (OpenAI-compatible API)
- Business logic lives in Go; UI wires the results
- One request in, one response out
- Repository pattern for all SQLite access
- Pointers only when modifying data
- Avoid unnecessary interfaces
- No premature optimization

## Goal (Overall)

Build a **working skeleton of the app with visible UI + one core intelligent feature**.

Priority:

1. Basic UI (all pages visible and navigable)
2. Functional RAG-based **Ask AI (Socratic Tutor)**
3. Then FSRS scheduler (after)

---

# Why This Order

Do NOT start with FSRS.

Reason:

* FSRS depends on:

  * quiz generation
  * user progress
  * review flow
* High dependency chain → slows you down

Start with:

> **RAG Ask AI (Socratic Tutor)**

Because:

* Directly usable feature
* Validates your core architecture (RAG + LLM)
* Easier to implement and debug

---
## 📍 Sprint 6: The "Command & Review" Loop (Do this right now)
**Goal:** Wire Vue frontend to FSRS backend so review flow is usable.
* **1. Dashboard UI:** Hook `Dashboard.vue` to `due_at` and `suspended`.
    * Show "X Cards Due Today" from `service.go` Daily Plan.
* **2. Flashcards UI:**
    * Send `Again (1)`, `Hard (2)`, `Good (3)`, `Easy (4)` ratings from `Flashcards.vue` to `RecordFlashcardReview`.
    * Show next review using `scheduled_days` (e.g. "See you in 3 days!").
* **Outcome:** Flashcards review session is wired end-to-end with FSRS.

## 🏗️ Architecture Pivot Note

The project previously experimented with a proactive orchestration model.

The architecture has now been simplified into a deterministic SQLite-driven Persistent Queue Architecture.

Current sprint planning and implementation should follow the queue model exclusively

