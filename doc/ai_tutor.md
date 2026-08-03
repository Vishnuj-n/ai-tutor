# AI-Tutor Technical Specification

## Overview

### Purpose
AI-Tutor is a local-first desktop application that guides learners through a structured study loop using a persistent SQLite queue. It is not a chatbot, PDF viewer, or standalone flashcard app. It is a deterministic, queue-driven guided tutor system.

### Problem Solved
Solo learners lack a structured, persistent study workflow that connects reading, comprehension checking, spaced repetition, and remediation in a single loop. Existing tools either focus on one phase (e.g., flashcard apps) or introduce opaque orchestration that is hard to inspect, debug, or resume after a crash.

### Target Users
Individual learners studying from uploaded textbooks and PDFs who want a guided, local-first study experience with AI-assisted quiz generation, spaced repetition, and concept rescue.

### Core Features
- **Persistent study queue** — SQLite-backed `study_queue` table drives all user progression
- **Reading layer** — PDF ingestion, deterministic chunking, trust-based completion, contextual Ask AI
- **Quiz layer** — Synchronous LLM quiz generation, pass/fail evaluation, conditional remediation
- **Retention layer** — FSRS spaced repetition for flashcards, optional written assessments (Examiner)
- **Rescue layer** — 2-strike Socratic rescue pipeline for repeated quiz failures
- **Dashboard** — Queue-aware task display with starvation protection and streak calendar
- **Cloud sync** — Stable-identifier-based sync for cross-student analytics
- **Offline-first** — All core study functions work without network; AI requires connectivity

---

## Architecture

### High-Level Architecture
Three-tier architecture: **Go backend** (Wails host), **SQLite persistence layer**, **Vue 3 frontend** (multi-page SPA). The queue is the central coordination mechanism. All state lives in SQLite. No in-memory orchestration, no background daemons, no event buses.

### Main Components

| Component | Location | Responsibility |
|-----------|----------|----------------|
| **Wails Bridge** | `main.go`, `app.go` | Desktop shell, binds Go backend to Vue frontend |
| **Queue Router** | `internal/study/service.go` | Queries `study_queue`, routes tasks to modules, marks completion, inserts follow-ups |
| **Reader Module** | `frontend/src/pages/Reader.vue`, `internal/study/reader_ai.go` | Renders PDF content, trust-based completion, Ask AI |
| **Quiz Module** | `frontend/src/pages/Quiz.vue`, `internal/study/quiz_sync.go` | Displays quizzes, scores, drives follow-up through queue |
| **Flashcard Module** | `frontend/src/pages/Flashcards.vue`, `internal/study/flashcard.go` | FSRS review sessions, card rating |
| **Socratic Rescue** | `frontend/src/pages/SocraticRescue.vue`, `internal/study/socratic_rescue.go` | 2-strike rescue for repeated quiz failures |
| **Examiner Module** | `frontend/src/pages/WrittenAssessment.vue`, `internal/study/examiner.go` | Written short-answer assessments |
| **Ingestion Pipeline** | `internal/notebook/` | PDF upload, chapter extraction, sliding-window chunking |
| **FSRS Service** | `internal/scheduler/fsrs.go` | Spaced repetition scheduling algorithm only |
| **RAG Engine** | `internal/retrieval/engine.go` | Topic-scoped vector retrieval + LLM answering |
| **LLM Provider** | `internal/llm/provider.go` | OpenAI-compatible client, dual-tier (Fast + Heavy) |
| **Database Layer** | `internal/db/` | SQLite CRUD, schema init, transaction management |
| **Embeddings** | `internal/embeddings/` | ONNX Runtime local inference for vector embeddings |

### Data Flow

```
User Action
    │
    ▼
Vue Page (thin UI)
    │ Wails bridge call
    ▼
App struct (Wails bridge, Go)
    │ delegates to
    ▼
Queue Router (internal/study/service.go)
    │ queries study_queue, routes by task_type
    ▼
Module (Reader / Quiz / Flashcards / etc.)
    │ calls DB repo for data
    ▼
SQLite (source of truth)
    │
    ▼
On completion: Queue Router marks task, inserts follow-ups
    │
    ▼
Dashboard refreshes, shows next pending task
```

### System Flow Diagram

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                           Dashboard Vue Page                               │
└─────────────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                          Queue Router                                      │
│                       internal/study/service.go                            │
│                                                                             │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────────────────────────┐  │
│  │ GetNextTask  │  │ CompleteTask │  │    SkipTask                      │  │
│  │              │  │              │  │                                  │  │
│  │ block_id +   │  │ task_id      │  │    task_id                       │  │
│  │ context      │  │ block_id +   │  │                                  │  │
│  │              │  │ context      │  │                                  │  │
│  └──────────────┘  └──────────────┘  └──────────────────────────────────┘  │
│                                                                             │
│  Insert Follow-ups:                                                         │
│  - QUIZ → FLASHCARD_REVIEW / REREAD / SOCRATIC_REMEDIAL / MILESTONE_EXAM  │
│  - SOCRATIC_REMEDIAL → QUIZ (requiz)                                      │
│  - READING → QUIZ                                                         │
└─────────────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                          SQLite Database                                   │
│                     study_queue + supporting tables                        │
│                                                                             │
│  ┌────────────────────────────────────────────────────────────────────────┐ │
│  │                         study_queue                                    │ │
│  │  - task_type (READING/QUIZ/REREAD/FLASHCARD_REVIEW/MILESTONE_EXAM/   │ │
│  │              EXAMINER/SOCRATIC_REMEDIAL/FLASHCARD_GENERATE)           │ │
│  │  - status (PENDING/ACTIVE/COMPLETED/SKIPPED/FAILED)                  │ │
│  │  - priority (lower = higher)                                         │ │
│  │  - notebook_id, topic_id, block_id                                   │ │
│  │  - payload_json, start_page, end_page                                │ │
│  └────────────────────────────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────────────────────────┘
                                    │
        ┌───────────────────────────┼───────────────────────────┐
        │                           │                           │
        ▼                           ▼                           ▼
┌───────────────┐          ┌───────────────┐          ┌───────────────┐
│ Reader Module │          │  Quiz Module  │          │  Flashcards   │
│               │          │               │          │    Module     │
│ - Render PDF  │          │ - Display     │          │               │
│ - Trust-based │          │   questions   │          │ - FSRS reviews│
│   completion  │          │ - Score/      │          │ - Card rating │
│ - Ask AI      │          │   pass/fail   │          │ - Suspend     │
└───────────────┘          └───────────────┘          └───────────────┘
        │                           │                           │
        ▼                           ▼                           ▼
┌───────────────┐          ┌───────────────┐          ┌───────────────┐
│   Socratic    │          │   Examiner    │          │  FSRS Service │
│    Rescue     │          │    Module     │          │               │
│               │          │               │          │ - Calculate   │
│ - 2-strike    │          │ - Written     │          │   next review │
│   pipeline    │          │   assessment  │          │ - State mgmt  │
│ - External    │          │ - Grading     │          │               │
│   LLM assist  │          │               │          │               │
└───────────────┘          └───────────────┘          └───────────────┘

┌───────────────────────┐   ┌────────────────────┐   ┌────────────────────┐
│  Ingestion Pipeline   │   │    RAG Engine      │   │   LLM Provider     │
│                       │   │                    │   │                    │
│ - PDF upload          │   │ - Topic-scoped     │   │ - OpenAI-compatible│
│ - Chapter extraction  │   │   retrieval        │   │ - Dual-tier        │
│ - Sliding-window      │   │ - LLM answering    │   │   (Fast + Heavy)   │
│   chunking            │   │ - Citations        │   │ - OS keyring       │
│ - Embeddings          │   │                    │   │   API key storage  │
└───────────────────────┘   └────────────────────┘   └────────────────────┘
```

---

## Tech Stack

### Go 1.26
**Why chosen:** Native compilation, strong typing, single-binary distribution via Wails. Suitable for a solo developer building a desktop app with local SQLite persistence.

**Alternatives considered:** Python (slower startup, harder to distribute as single binary), Rust (steeper learning curve for solo dev, longer iteration time).

**Trade-offs:** Go's concurrency model (goroutines) is available but deliberately underused — no background workers or async orchestration. The codebase is synchronous by design to keep behavior deterministic and debuggable.

### Wails v2
**Why chosen:** Provides a thin bridge between Go backend and Vue 3 frontend. Produces a single desktop executable. No web server needed — the frontend is bundled as embedded assets.

**Alternatives considered:** Electron (heavier, JavaScript-heavy), Tauri (Rust-based, smaller ecosystem), raw Go HTTP server + browser (more plumbing).

**Trade-offs:** Wails v2 has a specific build tag requirement (`sqlite_extension`) for SQLite vector extensions. The `wailsjs/` generated bindings are build artifacts that must be regenerated after backend changes.

### Vue 3 (Multi-Page, Hash-Based Routing)
**Why chosen:** Lightweight, easy to integrate with Wails. Hash-based routing works without a server. Multi-page structure keeps each view self-contained.

**Alternatives considered:** React (heavier bundle, more complex state management), Single-Page App with Vue Router (unnecessary complexity for this app's navigation model).

**Trade-offs:** Multi-page approach means each page loads independently. Pinia is used only for ephemeral UI state (notifications, modals), not for business state which lives in the backend.

### SQLite + sqlite-vec (vec0 extension)
**Why chosen:** Single-file database, zero configuration, embedded in the Go binary via `go-sqlite3`. `sqlite-vec` provides vector similarity search for RAG without external services.

**Alternatives considered:** PostgreSQL (requires external server, overkill for local-first), Chroma/Weaviate (external dependencies), pure in-memory vectors (no persistence).

**Trade-offs:** SQLite connection pool is strictly limited to 1 connection (`SetMaxOpenConns(1)`) because sqlite-vec extensions are connection-scoped. This means all vector operations must serialize through a single connection. The `vec0.dll`/`vec0.so`/`vec0.dylib` must be present in the `asset/` folder at runtime.

### ONNX Runtime (yalue/onnxruntime_go)
**Why chosen:** Local embedding inference without cloud dependencies. The INT8 quantized model (`model_int8.onnx`) runs entirely offline.

**Alternatives considered:** OpenAI embeddings API (requires network, costs money, breaks offline), sentence-transformers Python (no Go bindings, requires Python runtime).

**Trade-offs:** Requires platform-specific ONNX runtime libraries (`onnxruntime.dll`, `libonnxruntime.so`, `libonnxruntime.dylib`) in the `asset/` folder. Startup validates these assets; missing assets disable ingestion/retrieval features with explicit guidance.

### go-fsrs/v4 (FSRS Spaced Repetition)
**Why chosen:** Pure Go implementation of the FSRS algorithm. No external service needed. Deterministic scheduling math.

**Alternatives considered:** Anki's scheduling (proprietary format, no Go library), custom spaced repetition (reinventing the wheel, less research-backed).

**Trade-offs:** FSRS is used as a scheduling algorithm only — it does not orchestrate review sessions or insert queue tasks. The queue router handles task insertion. New cards bypass FSRS's intraday learning phase and start in clean Review state (`StateCode: 2`, `Reps: 0`) with day-based offsets.

### OpenAI-Compatible LLM API (Dual-Tier)
**Why chosen:** Provider-agnostic (Groq, OpenAI, OpenRouter, Custom). Dual-tier (Fast + Heavy) separates cheap quick responses from expensive complex generation. API keys stored in OS keyring via `go-keyring`.

**Alternatives considered:** Local LLM (insufficient quality for quiz generation), LangChain (unnecessary abstraction, violates "no engines" principle), async LLM calls (violates deterministic synchronous design).

**Trade-offs:** All LLM calls are synchronous — the frontend shows a loading spinner while waiting. This means quiz generation blocks the UI thread (mitigated by Wails's async bridge). If the LLM is unavailable, an explicit error is shown; no fallback models or synthetic answers.

---

## Key Modules

### Queue Router (`internal/study/service.go`)

**Responsibility:** Thin query-and-route layer. Fetches next pending task from `study_queue` using deterministic ordering, mounts the correct module, marks tasks complete, inserts follow-up tasks.

**Inputs:** `task_type`, `block_id`, `related_id` from `study_queue` rows

**Outputs:** Task context passed to modules; status transitions (`PENDING → ACTIVE → COMPLETED/SKIPPED/FAILED`); follow-up task insertion

**Dependencies:** `internal/db` (study_queue repository), `internal/scheduler` (FSRS for due-date calculations), `internal/models` (Task, TaskResult types)

**Extension points:** New task types can be added by extending the `task_type` CASE statement in the ordering query and adding corresponding module mount logic.

### Reader Module

**Responsibility:** Render PDF content for reading. Trust-based completion — user decides when done. No orchestration, no completion gating.

**Inputs:** `block_id` (chunk reference), `start_page` (authoritative entry), `end_page` (informational), `related_id` (topic context)

**Outputs:** Reading progress (informational), completion signal to Queue Router

**Dependencies:** `internal/db` (chunk retrieval), `internal/retrieval` (RAG for Ask AI)

**Extension points:** Reading completion could support page-range validation in future, but currently trust-based.

### Quiz Module

**Responsibility:** Display quiz questions, collect answers, calculate score, return pass/fail. Drives follow-up through queue results.

**Inputs:** `block_id` (quiz set reference), `related_id` (topic context)

**Outputs:** Score (0-100), pass/fail boolean to Queue Router

**Dependencies:** `internal/db` (quiz set retrieval, attempt recording), `internal/llm` (synchronous quiz generation), `internal/study` (`quiz_sync.go` for generation + rescue logic)

**Extension points:** Quiz generation prompt engineering, alternative question formats.

### Flashcard Module + FSRS Service

**Responsibility:** Render flashcards for review, capture ratings (Again/Hard/Good/Easy), send ratings to FSRS for next-interval calculation. One `FLASHCARD_REVIEW` task = one review session per block (not per card).

**Inputs:** `block_id` (card set reference)

**Outputs:** Card ratings to FSRS, completion signal to Queue Router

**Dependencies:** `internal/scheduler/fsrs.go` (scheduling math), `internal/db` (card CRUD, review log)

**Extension points:** Card suspension, bulk review actions.

### Socratic Rescue Module

**Responsibility:** 2-strike rescue for repeated quiz failures. After quiz fail #1 → `REREAD` task. After quiz fail #2 → `SOCRATIC_REMEDIAL` task (blocks queue). Student uses external LLM via copy-to-clipboard, then completes rescue session to insert fresh `QUIZ` task.

**Inputs:** `task_id` (SOCRATIC_REMEDIAL task), topic context

**Outputs:** Completion signal → Queue Router inserts fresh `QUIZ` task with `source: "socratic_rescue_requiz"`

**Dependencies:** `internal/study/socratic_rescue.go`, `internal/db` (reread_attempts, topics.external_help_required)

**Extension points:** Could support in-app Socratic tutoring (currently provides source text + prompt template for external LLM).

### Examiner Module

**Responsibility:** Written short-answer assessments for mastery verification. Optional, lowest queue priority (tier 0). Triggered after mastery thresholds (e.g., quiz > 80%).

**Inputs:** `block_id` (assessment reference)

**Outputs:** Score, pass/fail, feedback

**Dependencies:** `internal/study/examiner.go`, `internal/db` (written_questions, written_user_answers)

**Extension points:** Grading rubrics, multiple assessment types.

### Ingestion Pipeline (`internal/notebook/`)

**Responsibility:** PDF → Chunks → Queue. Uploads PDFs, extracts text, detects chapter boundaries, applies deterministic sliding-window chunking (2500 words, 200-word overlap), inserts READING tasks.

**Inputs:** PDF file path, user chapter selection

**Outputs:** notebooks, topics, chunks rows in DB; READING tasks in `study_queue`

**Dependencies:** `github.com/ledongthuc/pdf` (text extraction), `internal/embeddings` (vectorization), `internal/db` (persistence)

**Extension points:** Alternative chunking strategies (currently intentionally deterministic/sliding-window only), additional file formats.

### RAG Engine (`internal/retrieval/`)

**Responsibility:** Topic-scoped vector retrieval + LLM answering. Single-turn, stateless, no conversation memory.

**Inputs:** `topic_id`, user question

**Outputs:** Answer with citations, retrieved context blocks

**Dependencies:** `internal/embeddings` (ONNX embedding inference), `internal/llm` (LLM call), `internal/db` (chunk retrieval)

**Extension points:** Cross-topic retrieval (currently intentionally disabled), conversation memory (currently intentionally excluded).

### LLM Provider (`internal/llm/`)

**Responsibility:** OpenAI-compatible API client with dual-tier support (Fast + Heavy). API keys stored in OS keyring. All calls synchronous.

**Inputs:** Prompt string, model configuration

**Outputs:** Generated text response

**Dependencies:** `go-keyring` (API key storage), provider-specific HTTP client

**Extension points:** New provider presets (Groq, OpenAI, OpenRouter, Custom), async streaming (currently intentionally excluded for determinism).

### Database Layer (`internal/db/`)

**Responsibility:** Data persistence. CRUD for all tables, transaction management, schema initialization.

**Inputs:** SQL queries, Go struct parameters

**Outputs:** Query results, transaction commits/rollbacks

**Dependencies:** `github.com/mattn/go-sqlite3` (SQLite driver with CGO), `github.com/yalue/onnxruntime_go` (vec0 extension loading)

**Extension points:** New repository methods for new tables, migration scripts for schema changes.

---

## Database & APIs

### Schema Overview
SQLite database with the following table layers:

| Layer | Tables |
|-------|--------|
| **Queue** | `study_queue`, `reading_progress`, `review_task_cards` |
| **Content** | `notebooks`, `topics`, `chunks`, `notebook_topics`, `notebook_chunks`, `topic_progress` |
| **Assessment** | `quiz_attempts`, `reread_attempts`, `written_questions`, `written_user_answers` |
| **Retention** | `fsrs_cards`, `fsrs_review_log`, `manual_flashcards` |
| **Configuration** | `user_settings`, `llm_settings`, `study_profiles` |

### Important Tables

**study_queue** — Central task table. All user progression flows through this table.

| Key fields |
|------------|
| `id` (TEXT PK) |
| `task_type` (READING/QUIZ/REREAD/FLASHCARD_REVIEW/MILESTONE_EXAM/EXAMINER/SOCRATIC_REMEDIAL/FLASHCARD_GENERATE) |
| `status` (PENDING/ACTIVE/COMPLETED/SKIPPED/FAILED) |
| `priority` (lower = higher priority) |
| `notebook_id`, `topic_id`, `payload_json`, `start_page`, `end_page` |

**Indexes:** `(status, priority, created_at)`, `(notebook_id, status)`

---

**notebooks** — Top-level container for uploaded study material. Has `priority` (1-10, higher = more frequent in queue), `profile_id` (FK → study_profiles), `study_status` (dormant/active/completed).

**topics** — Topic/section container within a notebook. Has `external_help_required` flag for Socratic rescue tracking.

**chunks** — Granular content chunks from source documents. Has `embedding_ref` for vector store reference, `token_count` for adaptive scheduling, `importance_score` and `weakness_score` for prioritization.

**fsrs_cards** — Flashcards with FSRS state. `state_json` stores the full FSRS state payload. `due_at` is a Unix timestamp. Initial state is clean Review (`StateCode: 2`, `Reps: 0`).

**user_settings** — Singleton configuration row (`id=1`). Stores all user preferences including RAG toggles, remedial strategy, theme, cloud sync config, and classroom code.

**llm_settings** — Dual-tier LLM config. Two rows: fast (Groq, openai/gpt-oss-120b, 60s timeout) and heavy (Groq, openai/gpt-oss-120b, 90s timeout). API keys stored in OS keyring.

### API Endpoints / Internal Interfaces

All communication is synchronous via Wails bridge bindings. No REST API, no HTTP server, no auth.

#### Queue Router API

| Method | Description |
|--------|-------------|
| `GetNextTask() (*Task, error)` | Returns next PENDING task with deterministic ordering |
| `CompleteTask(taskID string, result TaskResult) error` | Marks task complete, inserts follow-ups |
| `SkipTask(taskID string) error` | Marks task SKIPPED (auditable) |
| `GetTaskContext(taskID string) (*TaskContext, error)` | Returns full context for a task |

#### Reader API

| Method | Description |
|--------|-------------|
| `GetBlockContent(blockID string) (*BlockContent, error)` | Returns chunk content |
| `MarkBlockRead(blockID string, progress int) error` | Records reading progress |

#### Quiz API

| Method | Description |
|--------|-------------|
| `GetQuizSet(blockID string) (*QuizSet, error)` | Returns quiz questions |
| `SubmitQuiz(taskID string, answers []Answer) (*QuizResult, error)` | Scores quiz, triggers follow-ups |

#### Flashcard API

| Method | Description |
|--------|-------------|
| `GetDueCards(blockID string) ([]Card, error)` | Returns due cards for review |
| `RateCard(cardID string, rating int) error` | Records rating, updates FSRS state |
| `SuspendFlashcard(taskID string, cardID string) (int, error)` | Suspends a card |

#### RAG API

| Method | Description |
|--------|-------------|
| `AskQuestion(topicID string, question string) (*Answer, error)` | Topic-scoped retrieval + LLM |

#### Milestone Exam API

| Method | Description |
|--------|-------------|
| `CompleteMilestoneExam(taskID string) (*MilestoneResult, error)` | Completes aggregate exam |
| `GetQuestionsForQuizAttempts(attemptIDs []string) ([]Question, error)` | Reuses past quiz questions |

#### Socratic Rescue API

| Method | Description |
|--------|-------------|
| `CompleteSocraticRescue(taskID string) error` | Marks rescue complete, inserts fresh QUIZ task |
| `DevForceSocraticRescue(notebookID, topicID string) error` | Dev-only bypass (requires APP_ENV=dev) |

#### Settings API

| Method | Description |
|--------|-------------|
| `GetUserSettings() map[string]interface{}` | Returns all user settings |
| `UpdateUserSettings(...) map[string]interface{}` | Updates settings with validation |
| `GetLLMSettings() map[string]interface{}` | Returns dual-tier LLM config |
| `UpdateLLMSettings(settings) error` | Updates LLM provider config |

#### Ingestion API

| Method | Description |
|--------|-------------|
| `ProcessPDF(filePath string) (*ProcessingResult, error)` | Full ingestion pipeline |

#### Dashboard API

| Method | Description |
|--------|-------------|
| `GetTodayPlan() ([]ScheduledTask, error)` | Returns scheduled tasks for the day |
| `GetDashboardOverview(timezoneOffsetMinutes int) map[string]interface{}` | Queue stats + streak data |
| `GetStreakState(timezoneOffsetMinutes int) map[string]interface{}` | Streak calendar data |

#### Cloud Sync API

| Method | Description |
|--------|-------------|
| `TriggerCloudSync() error` | Initiates sync with stable identifiers (SHA-256 file hashes) |
| `GetUnsentReviewLogs() ([]SyncLogEntry, error)` | Delta sync (only unsent events) |

---

## Core Workflows

### 1. Queue Loop (Primary Study Flow)

```
Dashboard queries study_queue for next PENDING task
  → User clicks task card → status set to ACTIVE, activated_at set
  → Queue Router mounts correct module based on task_type
  → Module renders content from block_id + related_id
  → User completes/skips task
  → Module signals completion to Queue Router
  → Queue Router marks task COMPLETED/SKIPPED/FAILED
  → Queue Router inserts follow-up tasks per explicit rules
  → Dashboard refreshes, shows next PENDING task
  → Repeat
```

**Crash Recovery:** On startup, any ACTIVE tasks older than 30 minutes revert to PENDING. This ensures the queue is always resumable.

### 2. Ingestion Pipeline

```
User uploads PDF
  → PDF text extraction (ledongthuc/pdf)
  → Chapter boundary detection (user reviews/prunes)
  → AI cleanup with 3-tier fallback (LLM → bookmark chapters → single "General" chapter)
  → Sliding window chunking (2500 words, 200-word overlap)
  → Chunks inserted into chunks table
  → READING tasks inserted into study_queue (one per chunk)
  → Topics created with start_page/end_page ranges
```

### 3. Reading Flow

```
Dashboard surfaces READING task
  → User opens Reader page
  → Reader displays chunk content from block_id
  → User reads at their own pace (trust-based, no timers, no page validation)
  → User clicks "Complete Session"
  → Frontend shows loading spinner
  → Backend calls LLM synchronously for quiz generation
  → Reading task marked COMPLETED
  → QUIZ task inserted with GENERATING state
  → LLM generates quiz questions synchronously
  → On success: generation_status = READY, QUIZ task activated
  → On failure: generation_status = FAILED, dashboard surfaces error
  → Dashboard shows quiz as next task
```

### 4. Quiz Flow & Remediation

**Pass (score ≥ threshold):**
```
Quiz → COMPLETED
  → If 10th quiz for notebook: insert MILESTONE_EXAM
  → Insert FLASHCARD_REVIEW task (FSRS cards generated with day-based offsets)
  → Dashboard shows next task
```

**Fail (score < threshold, Classic strategy):**
```
Quiz → COMPLETED
  → Insert REREAD task (if reread_attempt < max)
  → Dashboard shows REREAD task
  → User re-reads → Quiz again
```

**Fail (score < threshold, Fast strategy):**
```
Quiz → COMPLETED
  → Skip REREAD, insert SOCRATIC_REMEDIAL directly
  → Delete FSRS cards for topic
  → Dashboard shows Concept Rescue
```

### 5. Socratic Rescue (2-Strike)

```
Quiz fail #1 → REREAD task inserted
  → User re-reads → Quiz again
  → Quiz fail #2 → SOCRATIC_REMEDIAL task inserted (blocks queue)
  → FSRS cards deleted for topic
  → Student opens SocraticRescue page
  → Sees source text preview + pre-engineered Socratic prompt
  → Option A: Use in-app Socratic tutor (interactive chat)
  → Option B: Copy prompt to external LLM (clipboard)
  → Student completes external tutoring session
  → Clicks "I've Completed the Session"
  → SOCRATIC_REMEDIAL → COMPLETED
  → Fresh QUIZ task inserted with source: "socratic_rescue_requiz"
  → Re-quiz: Pass → flashcards generated, topic mastered
  → Re-quiz: Fail → external_help_required flag set, queue unblocks, notice shown
```

**Single rescue cycle only. No infinite loops. `external_help_required` flag prevents further rescue cycles for that topic.**

### 6. Flashcard Review & FSRS

```
FSRS calculates card due dates based on review ratings
  → Due cards trigger FLASHCARD_REVIEW task insertion
  → Dashboard surfaces FLASHCARD_REVIEW task
  → User opens Flashcards page
  → All due cards in block loaded for session
  → User rates each card (Again/Hard/Good/Easy)
  → Each rating updates FSRS state via CalculateNextReview
  → After all cards reviewed → task marked COMPLETED
  → FSRS calculates next due_at for each card
  → Future FLASHCARD_REVIEW tasks scheduled accordingly
```

**One FLASHCARD_REVIEW task = one review session per block (not per card).** This prevents queue explosion with many cards.

### 7. Milestone Exam

```
After quiz completion, backend counts completed quizzes for notebook
  → If count % 10 == 0 and count > 0:
    → MILESTONE_EXAM task inserted (priority tier 2)
    → Payload contains correctness arrays from last 10 quiz attempts
    → No new LLM generation — reuses existing questions
    → Deduplication prevents duplicate milestone exams
  → User completes milestone exam
  → Score computed from embedded correctness arrays
  → Pass → progression; Fail → standard remediation flow
```

### 8. Cloud Sync

```
On sync trigger (manual or periodic):
  → GetUnsentReviewLogs() fetches only unsent events
  → Build payload with stable identifiers:
    - File identification: review_log.reference_id → flashcards.id → chunks.id → notebook_topics.topic_id → notebooks.file_path → filepath.Base()
    - SHA-256 file hash + page number replace local IDs
    - classroom_code for teacher-student association
  → POST payload to cloud sync URL
  → On success: SetLastSyncedAt()
  → On failure: FLASHCARD_GENERATE task inserted (priority tier 7) to prevent data loss
  → On next successful sync: FLASHCARD_GENERATE task resolved
```

---

## Important Design Decisions

### 1. SQLite as Single Source of Truth
**Problem:** Learning apps often maintain runtime state that vanishes on crash or restart, causing lost progress and confusing UX.

**Chosen solution:** All state — queue tasks, progress, flashcard scheduling, user settings — persisted in a single SQLite database. No in-memory state machines, no runtime-only queues.

**Why:** Deterministic, inspectable, resumable. Any state can be queried directly. Solo developer benefit: no complex state management to debug.

**Trade-off:** SQLite connection pool limited to 1 connection (required by sqlite-vec extension scoping). This serializes all vector operations but is acceptable for a single-user desktop app.

### 2. Deterministic Queue Ordering (No AI-Driven Prioritization)
**Problem:** AI-driven task prioritization is opaque, non-deterministic, and hard to debug. It creates hidden orchestration that violates the project's core architecture principles.

**Chosen solution:** SQL CASE statement with fixed priority tiers for task types, combined with notebook priority bias and FIFO creation-time ordering. All ordering is query-time, not runtime.

**Why:** Predictable, inspectable, reproducible. Same inputs always produce same task order. No background processes or adaptive behavior.

**Trade-off:** The queue cannot adapt to user behavior (e.g., prioritizing weak topics). This is intentional — the system is deterministic, not intelligent.

### 3. Synchronous LLM Calls (No Async/Background Workers)
**Problem:** Async LLM calls introduce complexity (callbacks, state management, error handling for abandoned requests) and make behavior non-deterministic.

**Chosen solution:** All LLM calls are synchronous. Frontend shows a loading spinner. Backend blocks the request until the LLM responds or times out.

**Why:** Simple, debuggable, predictable. No hidden goroutines, no background workers, no race conditions on LLM responses.

**Trade-off:** UI blocks during LLM calls. For a desktop app this is acceptable — the user sees a spinner and knows the system is working.

### 4. Sliding Window Chunking (No Semantic/AI Chunking)
**Problem:** AI-generated chunk boundaries introduce non-determinism, are expensive to produce, and can be inconsistent across documents. Semantic chunking adds complexity without proportional benefit for MVP.

**Chosen solution:** Fixed sliding window (2500 words, 200-word overlap). Deterministic, inspectable, sufficient for retrieval quality.

**Why:** Deterministic chunking means the same PDF always produces the same chunks. No AI dependency for ingestion. Easy to debug and verify.

**Trade-off:** Chunk boundaries may split related content. The 200-word overlap mitigates this by ensuring context spans chunk boundaries.

### 5. Trust-Based Reading Completion (No Surveillance)
**Problem:** Page-completion validation, reading timers, and engagement tracking add complexity, invade user privacy, and create false signals about learning quality.

**Chosen solution:** User decides when reading is complete. The "Complete Session" button is always enabled during an active reading task. No timers, no page-completion enforcement, no scroll tracking.

**Why:** Respects user autonomy, keeps the app lightweight, avoids surveillance concerns. Reading completion is only a task-completion signal, not a mastery signal.

**Trade-off:** Users can mark reading complete without actually reading. This is acceptable because quiz performance is the real comprehension signal, not reading time.

### 6. FSRS as Scheduling Algorithm Only (Not Orchestrator)
**Problem:** Treating FSRS as an orchestrator would blur the line between scheduling and workflow control, creating hidden automation that is hard to inspect.

**Chosen solution:** FSRS calculates next review intervals and due dates. The Queue Router handles task insertion, activation, and completion. FSRS never touches the queue directly.

**Why:** Clear separation of concerns. FSRS is a pure function (state + rating → next interval). The queue owns workflow. Each component has exactly one responsibility.

**Trade-off:** FSRS cannot autonomously insert review tasks — it requires the queue router to do so based on due-date calculations. This is intentional and keeps the system deterministic.

### 7. Dual-Tier LLM (Fast + Heavy)
**Problem:** Using a single LLM for all tasks means either cheap models produce poor quality for complex tasks, or expensive models are called for simple tasks, wasting cost.

**Chosen solution:** Two tiers — Fast (groq, openai/gpt-oss-120b, 60s timeout) for RAG answers and simple tasks; Heavy (groq, openai/gpt-oss-120b, 90s timeout) for quiz generation and complex reasoning. Both default to Groq's free tier.

**Why:** Cost optimization through model tiering. Quick responses use the fast tier; expensive generation uses the heavy tier. Provider-agnostic (user can configure any OpenAI-compatible endpoint).

**Trade-off:** The fast and heavy models currently default to the same model (openai/gpt-oss-120b) with different timeouts. In practice, users may configure different models per tier.

### 8. No Background Queue Mutation
**Problem:** Background workers that scan and modify the queue create hidden state changes