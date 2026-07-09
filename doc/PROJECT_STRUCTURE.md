# Project Structure

Directory organization and package ownership. For architecture, see `ARCHITECTURE.md`.

---

## Backend (Go + Wails)

### Top-Level Files

| File | Responsibility |
|------|----------------|
| `main.go` | Wails bootstrap only |
| `app.go` | Core Wails methods (startup, RAG, reader, topics) |
| `app_study.go` | Study endpoints (quiz, flashcards, rescue, sync) |
| `app_settings.go` | Settings and profile endpoints |
| `notebook_endpoints.go` | Notebook API endpoints |

### Internal Packages

```
internal/
  db/                 # Data persistence (24 files)
    store.go          # Database init + connection
    schema.go         # Table definitions + migrations
    study_queue_repo.go # Queue CRUD
    reader_repo.go    # Reader state queries
    flashcard_repo.go # Flashcard operations
    topics_repo.go    # Topic/chunk queries
    notebooks_repo.go # Notebook management
    vector_repo.go    # Embedding vector storage
    tx.go             # Transaction helpers
    types.go          # Shared DB types

  study/              # Study session logic (9 files)
    service.go        # Core study service + LLM routing
    flashcard.go      # Flashcard generation
    examiner.go       # Written assessment
    quiz_sync.go      # Sync quiz generation + 2-strike rescue
    reader_ai.go      # Reader AI interactions
    socratic.go       # Socratic tutor (in-app + retrieval)
    socratic_rescue.go # SOCRATIC_REMEDIAL completion handler
    review_session.go # Review session + suspend
    sync.go           # Cloud sync + FLASHCARD_GENERATE tasks

  notebook/           # Upload + ingestion (4 files)
    upload.go         # PDF upload
    ingestion.go      # PDF processing pipeline
    pdfcpu.go         # PDF text extraction
    syllabus.go       # Chapter boundary detection

  scheduler/          # Scheduling algorithms (2 files)
    fsrs.go           # FSRS spaced repetition
    service.go        # Scheduler service wrapper

  embeddings/         # Local embedding inference (3 files)
    onnx.go           # ONNX Runtime embedding model
    text.go           # Text preprocessing
    tokenizer_utils.go # Tokenizer utilities

  retrieval/          # RAG retrieval pipeline (3 files)
    engine.go         # Search + retrieval engine
    indexer.go        # Index management
    queue.go          # Queue-based retrieval

  llm/                # LLM provider adapter (2 files)
    provider.go       # OpenAI-compatible client
    keyring.go        # OS keyring for API keys

  runtime/            # Application bootstrap (2 files)
    boot.go           # Startup initialization
    asset_manager.go  # Asset validation + management

  models/             # Domain types (1 file)
    models.go         # Task, Block, Quiz types

  utils/              # Shared utilities (2 files)
    hash.go           # Hashing functions
    logging.go        # Structured logging
```

### Package Rules

One responsibility per package. No cross-package orchestration. State in SQLite, not code. Handlers are thin.

---

## Frontend (Vue)

```
frontend/src/
  pages/
    Dashboard.vue        # Pending tasks from queue
    Reader.vue           # PDF reading module
    Quiz.vue             # Quiz generation + scoring
    Flashcards.vue       # Flashcard review with FSRS
    WrittenAssessment.vue # Written assessment (Examiner)
    Socratic.vue         # Socratic tutor chat (in-app)
    SocraticRescue.vue   # Concept rescue (dual-lane)
    Notebook.vue         # Notebook management
    Onboarding.vue       # First-time setup
    Settings.vue         # Provider config, themes, profiles

  components/
    Sidebar.vue          # Navigation sidebar (7 items)
    BaseButton.vue       # Reusable button
    ErrorMessage.vue     # Error display
    ReaderChat.vue       # Ask AI panel for Reader
    StudyPageLayout.vue  # Shared study page layout

  services/
    appApi.js            # Wails backend bridge
    markdown.js          # Markdown rendering utilities
```

Sidebar: Dashboard, Reader, Notebooks, Quiz, Flashcards, Examiner, Tutor. Settings + Sync at bottom.

---

## Queue Contract (V1)

```sql
SELECT * FROM study_queue sq
JOIN notebooks n ON sq.notebook_id = n.id
WHERE sq.status = 'PENDING'
ORDER BY
  CASE sq.task_type
    WHEN 'FLASHCARD_GENERATE' THEN 7
    WHEN 'SOCRATIC_REMEDIAL' THEN 6
    WHEN 'FLASHCARD_REVIEW' THEN 5
    WHEN 'REREAD' THEN 4
    WHEN 'QUIZ' THEN 3
    WHEN 'READING' THEN 2
    WHEN 'EXAMINER' THEN 1
    ELSE 0
  END DESC,
  COALESCE(n.priority, 5) DESC,
  sq.priority ASC,
  sq.created_at ASC;
```

Task shape: `id` (TEXT), `task_type` (READING/QUIZ/REREAD/FLASHCARD_REVIEW/EXAMINER/SOCRATIC_REMEDIAL/FLASHCARD_GENERATE), `block_id`, `related_id`, `status`, `priority`, `created_at`.

---

## Debugging

**UI data wrong:** Check `study_queue` table → check queue router logs → check module API response.

**Flow stuck:** Check task status in `study_queue` → check completion errors.

**RAG fails:** Check `chunks` table → check sqlite-vec for embeddings → verify `block_id` in task context.
