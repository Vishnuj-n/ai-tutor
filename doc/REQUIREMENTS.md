# AI Tutor — Requirements

## Purpose

Local-first desktop study assistant. Users upload documents, system creates a deterministic queue of learning tasks (reading → quiz → review), users work through queue.

**NOT:** autonomous AI tutor, hidden orchestration, proactive scheduling.

---

## Goals

- Upload documents (PDF, TXT, Markdown)
- **Sliding window chunking**: 2500 words, 200 overlap — deterministic, no AI boundaries
- **Persistent queue**: `study_queue` drives all user flows
- SQLite = source of truth, no runtime-only state
- **Synchronous quiz generation**: immediate comprehension validation
- **FSRS**: queue-driven flashcard reviews for long-term retention
- Simple, inspectable, debuggable architecture
- Local-first data

## Non-Goals

- Not hosted/multi-user service
- Not a chatbot with memory
- Not LangChain/agent-based
- Not async/background jobs
- Not semantic chunking
- Not proactive scheduling

## Users

- Individual learners wanting offline, private study assistant
- Developers/researchers running local RAG experiments

## Functional Requirements

1. **Notebook Management**
   - Upload PDF/TXT/MD, batch upload supported
   - Many files per notebook, many topics per notebook
   - Metadata: title, filename, upload time, optional topic_id
   - List, preview, delete; notebook/topic priority (1-5 stars)

2. **Ingestion & Indexing**
   - Parse files, extract text and metadata
   - Sliding window chunking: 2500 words, 200 overlap
   - Persist chunks in `chunks` table (linked to topics)
   - Store embeddings in sqlite-vec; reference from `chunks` via `embedding_ref`
   - Insert `READING` tasks into `study_queue` during ingestion
   - Synchronous processing

3. **RAG & LLM**
   - Reader view with Ask AI panel
   - Synchronous quiz generation
   - Flashcards from content (queue-driven, not autonomous)
   - Topic-scoped retrieval only via `block_id`
   - Strict token budgets; stateless, synchronous LLM calls

4. **Frontend**
   - Vue pages: Notebook, Reader, Flashcards, Quiz, Socratic, Settings
   - Global notebook/topic scope selector
   - Responsive upload with drag/drop
   - Ask AI contextual inside reading/review, not general chat

5. **Backend/API**
   - Wails desktop backend (Go): `UploadNotebook`, `GetNotebooks`, `DeleteNotebook`, ingestion endpoints
   - `internal/notebook` for file writes, sanitization, metadata
   - `internal/db` for `notebooks`, `notebook_chunks`
   - OpenAI-compatible interface (base_url, api_key, model, timeout)

6. **Data Storage**
   - Local per-user config dir (e.g. `<config>/ai-tutor/`)
   - SQLite DB (`ai-tutor.db`) + `uploads/` folder
   - UUID-prefixed filenames
   - `.gitignore` patterns for DB/uploads

7. **Security & Privacy**
   - All data local by default
   - Optional DB/file encryption

8. **Quality**
   - Pass `golangci-lint`
   - Unit tests for DB, chunker, ingestion
   - Integration tests for ingestion and retrieval

9. **Queue-Driven Workflow**
   - SQLite `study_queue` = scheduler (no separate engine)
   - Dashboard queries: `SELECT * FROM study_queue WHERE status = 'PENDING' ORDER BY priority ASC` (lower = higher priority)
   - Task types: `READING`, `QUIZ`, `REREAD`, `FLASHCARD_REVIEW`, `EXAMINER`
   - Thin orchestrator: fetch, mount, complete, insert follow-ups
   - Stateless modules
   - Flashcards: FSRS calculates due dates, inserts `FLASHCARD_REVIEW` tasks
   - Remediation: failed quiz → `REREAD` task

## Non-Functional Requirements

- Cross-platform (Wails build)
- Offline-capable (except optional LLM/embedding providers)
- Lightweight resource usage
- Maintainability-first code style
- Windows packaging includes native libs

## Architecture Guardrails

- SQLite `study_queue` = source of truth
- Thin orchestrator — routes tasks, no flow control
- Stateless modules
- No LangChain or similar frameworks
- OpenAI-compatible APIs, minimal provider interface
- Stateless, synchronous LLM calls
- Scope retrieval to current `block_id`
- Strict token limits at prompt build time
- Sliding window chunking only
- No chatbot mode; Ask AI contextual inside reading/review

## Acceptance Criteria

- `study_queue` table exists, dashboard queries it
- Clicking task mounts correct module with `block_id` context
- Completing task → status `COMPLETED`, follow-ups inserted
- PDF upload → sliding window chunks → `READING` tasks inserted
- Quiz: Complete → spinner → sync LLM → quiz appears
- Failed quiz → REREAD task (optional, user can skip)
- FSRS schedules, inserts `FLASHCARD_REVIEW` tasks
- No runtime-only queues, no background workers
- SQLite = source of truth
