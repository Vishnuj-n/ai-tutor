# Agent Map: Component Responsibilities

## Overview

Strict module boundaries for the Persistent Queue Architecture. Each module has exactly one responsibility. The queue router is intentionally thin—task routing only, no orchestration engine.

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

**Does NOT:**
- Generate quizzes
- Schedule next tasks
- Know about other modules
- Validate or gate completion
- Own progression semantics
- Enforce page-completion validation
- Route to other modules

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

**Does NOT:**
- Generate quizzes (synchronous LLM call happens before task creation)
- Insert follow-up tasks
- Know about Reader module
- Silently handle generation failures

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
