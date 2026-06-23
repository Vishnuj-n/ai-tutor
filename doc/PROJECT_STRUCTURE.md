# Project Structure

Directory organization and package ownership. For architecture details, see `ARCHITECTURE.md`.

---

## Backend (Go + Wails)

### Top-Level Files

| File | Responsibility |
|------|----------------|
| `main.go` | Wails bootstrap only |
| `app.go` | Core Wails-facing methods (startup, RAG, reader, topics) |
| `app_study.go` | Study-mode Wails endpoints (quiz, flashcards, rescue, sync) |
| `app_settings.go` | Settings and profile Wails endpoints |
| `notebook_endpoints.go` | Notebook API endpoints |

### Internal Packages

```
internal/
  db/                 # Data persistence (24 files)
    store.go          # Database initialization and connection
    schema.go         # Table definitions and migrations
    study_queue_repo.go # Queue CRUD operations
    reader_repo.go    # Reader state queries
    reader_bundle_repo.go # Reader topic bundle queries
    flashcard_repo.go # Flashcard card operations
    topics_repo.go    # Topic/chunk queries
    notebooks_repo.go # Notebook management
    vector_repo.go    # Embedding vector storage
    review_session_repo.go # Review session queries
    reread_attempts_repo.go # Reread attempt tracking
    fsrs_review_log_repo.go # FSRS review log queries
    assessment_repo.go # Written assessment queries
    tx.go             # Transaction helpers
    types.go          # Shared DB types
    extension_cgo.go  # CGO-enabled vec0 extension loading
    extension_nocgo.go # No-CGO fallback

  study/              # Study session logic (9 files)
    service.go        # Core study service + LLM routing
    flashcard.go      # Flashcard generation
    examiner.go       # Written assessment session
    quiz_sync.go      # Synchronous quiz generation + 2-strike rescue
    reader_ai.go      # Reader AI interactions
    socratic.go       # Socratic tutor session (in-app + retrieval)
    socratic_rescue.go # SOCRATIC_REMEDIAL completion handler
    review_session.go # Review session management + suspend
    sync.go           # Cloud sync + FLASHCARD_SYNC task management

  notebook/           # Upload + ingestion (4 files)
    upload.go         # PDF upload handling
    ingestion.go      # PDF processing pipeline
    pdfcpu.go         # PDF text extraction
    syllabus.go       # Chapter boundary detection

  scheduler/          # Scheduling algorithms (2 files)
    fsrs.go           # FSRS spaced repetition algorithm
    service.go        # Scheduler service wrapper

  embeddings/         # Local embedding inference (3 files)
    onnx.go           # ONNX Runtime embedding model
    text.go           # Text preprocessing
    tokenizer_utils.go # Tokenizer utilities

  retrieval/          # RAG retrieval pipeline (3 files)
    engine.go         # Search and retrieval engine
    indexer.go        # Index management
    queue.go          # Queue-based retrieval

  llm/                # LLM provider adapter (2 files)
    provider.go       # OpenAI-compatible client
    keyring.go        # OS keyring for API keys

  runtime/            # Application bootstrap (2 files)
    boot.go           # Startup initialization
    asset_manager.go  # Asset validation and management

  models/             # Domain types (1 file)
    models.go         # Task, Block, Quiz types

  utils/              # Shared utilities (2 files)
    hash.go           # Hashing functions
    logging.go        # Structured logging
```

### Package Ownership Rules

| Rule | Rationale |
|------|-----------|
| One responsibility per package | Clear boundaries |
| No cross-package orchestration | Queue controls progression |
| State in SQLite, not code | Persistent queue architecture |
| Handlers are thin | UI logic in frontend, backend in `internal/` |

---

## Frontend (Vue)

```
frontend/src/
  pages/
    Dashboard.vue        # Shows pending tasks from queue
    Reader.vue           # PDF reading module
    Quiz.vue             # Quiz generation and scoring
    Flashcards.vue       # Flashcard review with FSRS
    WrittenAssessment.vue # Written assessment (Examiner)
    Socratic.vue         # Socratic tutor chat (in-app)
    SocraticRescue.vue   # Concept rescue (dual-lane: in-app + external)
    Notebook.vue         # Notebook management
    Onboarding.vue       # First-time setup wizard
    Settings.vue         # Provider config, themes, profiles

  components/
    Sidebar.vue          # Navigation sidebar (7 items)
    BaseButton.vue       # Reusable button component
    ErrorMessage.vue     # Error display component
    ReaderChat.vue       # Ask AI panel for Reader
    StudyPageLayout.vue  # Shared study page layout

  services/
    appApi.js            # Wails backend bridge
    markdown.js          # Markdown rendering utilities

  composables/           # Vue composables
  config/                # App configuration
  router/
    index.js             # Route definitions
```

### Sidebar Items (7)

1. Dashboard
2. Reader
3. Notebooks
4. Quiz
5. Flashcards
6. Examiner (WrittenAssessment)
7. Tutor (Socratic)

Plus: Settings and Sync at bottom.

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
| `task_type` | TEXT | `READING`, `QUIZ`, `REREAD`, `FLASHCARD_REVIEW`, `EXAMINER`, `SOCRATIC_REMEDIAL`, `FLASHCARD_SYNC` |
| `block_id` | TEXT | Content reference |
| `related_id` | TEXT | Topic reference |
| `status` | TEXT | `PENDING`, `ACTIVE`, `COMPLETED`, `SKIPPED`, `FAILED` |
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
1. Check `chunks` table for content
2. Check the RAG embedding store (sqlite-vec) for embeddings
3. Verify `block_id` in task context
**Note:** The live schema uses `chunks` and an embedding store (see `doc/SCHEMA.md`). Replace `blocks` → `chunks` and `block_vectors` → RAG embedding store where applicable.
