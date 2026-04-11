
# AI Tutor — Requirements

## Purpose

Provide a local-first desktop assistant for studying and knowledge work that lets users upload documents (PDF, TXT, Markdown), index them, and use LLM-powered features (search, Q&A, flashcards, quizzes, Socratic tutoring) with per-notebook scope.

## Goals

- Allow users to add and manage notebooks (uploaded documents) and topics.
- Make notebooks the primary organizing unit: each notebook can contain one or many PDFs and text sources.
- Provide reliable local storage for raw files and metadata (SQLite + uploads directory).
- Ingest documents into chunked text, generate embeddings, and support RAG-style retrieval.
- Offer a consistent, usable UI across Reader, Flashcards, Quiz, and Socratic tutor pages.
- Provide global scheduling so tasks are generated and prioritized across all notebooks/topics.
- Keep user data local by default and excluded from version control.

## Non-Goals

- Not a hosted, multi-user service (default behavior is single-user, local-only).
- Not a full replacement for enterprise content management systems.
- Not a chatbot product with free-form conversation memory.
- Not a LangChain/agent-orchestration based architecture.

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
	- Parse uploaded files to extract text and basic metadata (page counts for PDFs).
	- Chunk documents into token-aware pieces suitable for embeddings.
	- Persist chunk metadata in SQLite (`notebook_chunks`) and write vectors to `sqlite-vec` (`vec0`) using stable chunk IDs.
	- Generate embeddings locally from `asset/tokenizer.json` + `asset/model_int8.onnx` via ONNX Runtime.
	- Include a background worker to perform chunking/embedding asynchronously with progress reporting.

3. RAG and LLM Features
	- Provide an Ask/Reader view that retrieves relevant chunks and composes prompts with context.
	- Generate flashcards and quizzes from notebook content.
	- Offer a Socratic tutor mode that guides learning via multi-step questioning.
	- Enforce topic-scoped retrieval only (active `topic_id`), with parent-document expansion from matched child chunks.
	- Enforce strict token budgets during prompt assembly before any model call.
	- Keep all LLM calls stateless (single-turn request/response, no conversation memory).

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

9. Learning Workflow & Scheduling
	- Scheduling is global across all notebooks and topics.
	- Dashboard must surface daily priorities: due reviews first, then new reading tasks.
	- Topics follow lifecycle states (`unseen` -> `reading` -> `learned`) with explicit transitions.
	- Flashcard reviews use FSRS grading actions (Again, Hard, Good, Easy) and persist scheduling state locally.
	- Quiz generation is topic-scoped and derived from learned content.
	- Task generation must account for priority rating and learner state when ranking what to do next.
	- Every generated task must be one-click actionable and deep-link to the exact destination context.
	- Example: clicking a task like "Quiz for Topic X from Notebook Y" opens Quiz page with Notebook Y and Topic X preselected.

## Non-Functional Requirements

- Cross-platform: Windows, macOS, Linux (packaged via Wails build process).
- Offline-capable: functional without network access except for optional external LLM/embedding providers.
- Lightweight: modest resource usage; background tasks should be rate-limited and cancelable.
- Maintainability-first code style: simple functions over deep abstractions; readability over cleverness.
- Windows packaging for local RAG must include required native libs (`onnxruntime.dll`, `vec0.dll`).

## Architecture Guardrails (Mandatory)

- Keep implementation simple and explicit.
- Do not use LangChain or similar orchestration frameworks.
- Use OpenAI-compatible APIs with a minimal provider interface.
- Keep AI calls stateless.
- Always scope retrieval to the current `topic_id`.
- Use parent-document retrieval (child hit -> parent context).
- Enforce token limits strictly at prompt build time.
- In Go code: avoid unnecessary interfaces; use structs only when needed; use pointers only when mutation is required.
- UX guardrail: no chatbot mode; Ask AI is contextual inside reading/review flows.

## Acceptance Criteria

- Uploading a PDF/TXT/MD via the Notebook UI saves the file to `uploads/` and inserts a `notebooks` row in SQLite.
- Uploading a group of PDFs to a notebook stores all files, creates/updates notebook mappings, and shows those files under that notebook.
- The ingestion worker can chunk and create `notebook_chunks` rows for a notebook and write vectors to `sqlite-vec`.
- Embeddings are generated with ONNX Runtime using `asset/tokenizer.json` and `asset/model_int8.onnx`.
- Embedding/vector rows are synchronized to SQLite chunk records via shared chunk IDs.
- The frontend shows uploaded notebooks in the sidebar and allows selecting active notebook/topic across pages.
- Scheduler produces a single global queue of tasks across notebooks/topics and respects priority ratings.
- Clicking a scheduled task opens the exact target flow (for example Quiz) with notebook/topic context already set.
- Retrieval requests never cross topic boundaries unless explicitly enabled by a future feature flag.
- RAG answers are generated from parent-expanded, topic-scoped context and adhere to token budget limits.
- Repo remains clean: database and uploads are excluded from VCS by `.gitignore`.
- All modified/added Go code passes `golangci-lint` and `go build` succeeds.

## Implementation Notes & Next Steps

- Implement robust PDF parsing (accurate page counts and text extraction) and token-aware chunker.
- Add a configurable vector store adapter and ensure chunk IDs are synchronized between SQLite and the vector store.
- Build background ingestion queue with progress and retry semantics.
- Create unit tests for DB operations, chunking, and ingestion worker; add CI steps to run linter and tests.
- Finalize UX: global scope selector, notebook preview, and consistent notebook layout across pages.

---

If you want, I can commit this file, run the linter/formatter, and/or implement the PDF parser next.

