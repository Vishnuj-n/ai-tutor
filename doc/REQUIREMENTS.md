
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

