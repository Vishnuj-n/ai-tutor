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
