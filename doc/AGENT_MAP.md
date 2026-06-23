# Agent Map: Component Responsibilities
- **Legacy term note:** The term `blocks` has been replaced by `chunks`. The API still uses `block_id` as the identifier for a chunk. See `doc/SCHEMA.md` for mapping.

## Overview

Strict module boundaries for the Persistent Queue Architecture. Each module has exactly one responsibility. The queue router is intentionally thin—task routing only, no orchestration engine.

Canonical checkpoint flow:
Dashboard -> Reader -> Quiz -> Dashboard

Reader completes the reading task only. The backend generates and activates the QUIZ follow-up task, and the Dashboard regains ownership after quiz submission and evaluation. Any Reader -> Quiz handoff is queue-owned and applies only to generated follow-up quiz tasks.

- **Reading Layer**: Reading, Quiz, Reread (Immediate comprehension validation).
- **Retention Layer**: Flashcards, Examiner, FSRS (Long-term retention scheduling).

**Orchestration Constraints:** See Queue Router section (below) for comprehensive list of prohibited orchestration behaviors. Individual modules focus on their specific responsibilities only.

---

## Queue Router (Thin Task Router)

**File:** `internal/study/service.go`

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
- Handle SOCRATIC_REMEDIAL tasks (concept rescue) with queue-blocking semantics
- Handle FLASHCARD_SYNC tasks for cloud sync recovery

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

**File:** `frontend/src/pages/Reader.vue` + `internal/study/reader_ai.go`

**Responsibility:** Render PDF content for reading (execution surface only, Reading Layer)

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

**File:** `frontend/src/pages/Quiz.vue` + `internal/study/quiz_sync.go`

**Responsibility:** Display and score quizzes (execution surface only, Reading Layer)

**Does:**
- Load quiz from `block_id` (quiz_set reference)
- Display questions
- Collect answers
- Calculate score
- Return pass/fail
- Handle `GENERATING`, `READY`, `FAILED` generation states
- Show explicit error for `FAILED` generation
- Drive queue follow-up outcomes after submission/evaluation (e.g., reread insertion)

**Important**: Quizzes validate immediate comprehension and do NOT update FSRS memory state.

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

**File:** `frontend/src/pages/Flashcards.vue` + `internal/study/flashcard.go`

**Responsibility:** Render and rate flashcards (execution surface only, Retention Layer)

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

**File:** `internal/scheduler/fsrs.go`

**Responsibility:** Scheduling algorithm for long-term retention (Retention Layer)

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

**File:** `frontend/src/pages/WrittenAssessment.vue` + `internal/study/examiner.go`

**Responsibility:** Written assessments (Retention Layer)

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

## SocraticRescue Module

**File:** `frontend/src/pages/SocraticRescue.vue` + `internal/study/socratic_rescue.go`

**Responsibility:** 2-strike rescue for repeated quiz failures (Rescue Layer)

**Does:**
- Display source text preview for the topic's page range
- Show pre-engineered Socratic prompt for copy-to-clipboard
- Provide "I've Completed the Session" button
- Call `CompleteSocraticRescue(taskID)` on completion
- Redirect to dashboard (fresh QUIZ task appears in queue)

**Does NOT:**
- Integrate local LLM (external clipboard only)
- Generate flashcards (re-quiz does that)
- Skip or bypass queue ordering

**API:**
```go
func (s *StudyService) CompleteSocraticRescue(taskID string) error
```

**Backend behavior:**
- Validates task is SOCRATIC_REMEDIAL and ACTIVE
- Marks task COMPLETED
- Inserts fresh QUIZ task for same topic with `source: "socratic_rescue_requiz"` in payload
- Transactional — both complete + insert happen atomically

**Props from Queue Router:**
- `task_id`: SOCRATIC_REMEDIAL task to complete

**Triggered by:**
- Quiz fail #2 (after 1 reread attempt) → SOCRATIC_REMEDIAL task inserted
- `external_help_required` flag on topic prevents further rescue cycles

**Flow:**
1. Student opens rescue page → sees source text + Socratic prompt
2. Copies prompt to external LLM (e.g., ChatGPT)
3. Completes Socratic tutoring session externally
4. Clicks "I've Completed the Session"
5. Fresh QUIZ task inserted into queue

---

## Ingestion Pipeline

**File:** `internal/notebook/` (upload.go, ingestion.go, pdfcpu.go, syllabus.go)

**Responsibility:** PDF → Chunks → Queue

**Does:**
- Extract text from PDF
- Extract chapter boundaries
- Sliding window chunking (2500 words, 200 overlap)
 - Create chunks in database (legacy docs may call these `blocks`)
- Insert READING tasks into queue

**Does NOT:**
- Use AI for chunking
- Use semantic boundaries

**API:**
```go
func ProcessPDF(filePath string) (*ProcessingResult, error)
func CreateChunks(text string, topicID string) ([]Chunk, error)
func InsertReadingTasks(chunks []Chunk) error
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

**File:** `internal/retrieval/engine.go`

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
┌─────────────┐  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐  ┌─────────────────┐
│   Reader    │  │    Quiz     │  │ Flashcards  │  │  Examiner   │  │ SocraticRescue  │
│             │  │             │  │             │  │             │  │                 │
│ (No routing │  │ (No routing │  │ (No routing │  │ (No routing │  │ (No routing     │
│  logic)     │  │  logic)     │  │  logic)     │  │  logic)     │  │  logic)         │
└──────┬──────┘  └──────┬──────┘  └──────┬──────┘  └──────┬──────┘  └──────┬──────────┘
       │                │                │                │                 │
       │ MarkComplete() │ SubmitQuiz()   │ RateCard()     │ Submit()        │ CompleteRescue()
       │                │                │                │                 │
       └────────────────┴────────────────┴────────────────┴─────────────────┘
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
  study/             # Study session logic
    service.go       # Core study service
    flashcard.go     # Flashcard review session
    examiner.go      # Written assessment session
    quiz_sync.go     # Synchronous quiz generation + 2-strike rescue logic
    reader_ai.go     # Reader AI interactions
    socratic.go      # Socratic tutor session
    socratic_rescue.go # SOCRATIC_REMEDIAL completion handler (re-quiz insertion)
    sync.go          # Cloud sync + FLASHCARD_SYNC task management
  scheduler/         # Scheduling algorithms
    fsrs.go          # FSRS spaced repetition algorithm
    service.go       # Scheduler service wrapper
  notebook/          # Upload + ingestion
    upload.go        # PDF upload handling
    ingestion.go     # PDF processing pipeline
    pdfcpu.go        # PDF text extraction
    syllabus.go      # Chapter boundary detection
  embeddings/        # Local embedding inference
    onnx.go          # ONNX Runtime embedding model
    text.go          # Text preprocessing
  retrieval/         # RAG retrieval pipeline
    engine.go        # Search and retrieval engine
    indexer.go       # Index management
    queue.go         # Queue-based retrieval
  llm/               # LLM provider adapter
    provider.go      # OpenAI-compatible client
    keyring.go       # OS keyring for API keys
  runtime/           # Application bootstrap
    boot.go          # Startup initialization
    asset_manager.go # Asset validation
  models/            # Domain types
    models.go        # Task, Block, Quiz types
  db/                # Data persistence
    store.go         # Database initialization
    schema.go        # Table definitions
    study_queue_repo.go # Queue CRUD operations

frontend/src/pages/
  Dashboard.vue      # Task display
  Reader.vue         # Reading module
  Quiz.vue           # Quiz module
  Flashcards.vue     # Flashcard module
  WrittenAssessment.vue # Written assessment (Examiner)
  Socratic.vue       # Socratic tutor
  SocraticRescue.vue # Concept rescue (2-strike Socratic prompt)
  Notebook.vue       # Notebook management
  Onboarding.vue     # First-time setup
  Settings.vue       # Provider config
```

---

## Testing Boundaries

Each module can be tested independently:

- **Reader:** Mock block content, test rendering
- **Quiz:** Mock quiz set, test scoring
- **Flashcards:** Mock cards, test rating flow
- **Queue Router:** Mock database, test routing
- **FSRS:** Pure algorithm, test scheduling math
