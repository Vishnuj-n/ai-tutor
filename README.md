# AI Tutor

AI Tutor is a local-first desktop app that guides learners through a structured study loop.

It is not a chatbot, PDF viewer, or standalone flashcard app. It is a guided tutor system:

1. Read concepts
2. Understand with contextual AI help
3. Review with FSRS spaced repetition

## Product Overview

### What

- A guided learning workflow that always suggests the next best action
- Local storage for content, embeddings, progress, and scheduling state
- Topic-scoped AI for explanations and quiz generation only

### Why

- Keep user data private and fully local by default
- Reduce decision fatigue with a clear daily study flow
- Keep implementation simple and maintainable for a solo developer

### How

- Backend and desktop shell: Go + Wails
- Frontend: Vue multi-page app with left sidebar navigation
- Local data: SQLite + sqlite-vec embeddings
- LLM layer: OpenAI-compatible API (stateless requests)

## Core Features

- Dashboard:
	- Today tasks (due reviews, new topics)
	- Progress summary
- Reader:
	- Structured content view (not raw PDF)
	- Ask AI panel (primary placement)
	- Mark as Learned action
- Quiz:
	- Topic-based quiz sets generated from learned content
	- JSON-backed quiz payloads
- Flashcards:
	- FSRS actions: Again, Hard, Good, Easy
	- Explain action (secondary Ask AI placement)
- Socratic Tutor:
	- Guided questioning mode scoped to current topic
	- Enter to send, Shift+Enter for new line
- Settings:
	- Base URL, API key, model, phase-2 cloud endpoint placeholder
- Sync button:
	- Manual trigger for future sync (timestamp + hash versioning)

## Local-First and Offline Behavior

Works offline:

- Reading
- FSRS review
- Scheduling
- Local progress and content access

Requires internet:

- Ask AI
- Quiz generation

Failure rule:

- If AI is unavailable, show clear error and do not simulate output

## Tech Stack

- Go
- Wails
- Vue (multi-page)
- SQLite
- sqlite-vec
- onnxruntime_go + ONNX INT8 embedding model
- OpenAI-compatible LLM API

## Quick Start

### Prerequisites

- Go 1.22+
- Node.js 20+
- Wails CLI
- CGO-capable compiler toolchain
- Local RAG assets in `asset/`:
	- `tokenizer.json`
	- `model_int8.onnx`
	- `onnxruntime.dll` (Windows) / `libonnxruntime.dylib` (macOS) / `libonnxruntime.so` (Linux)
	- `vec0.dll` (Windows) / `vec0.dylib` (macOS) / `vec0.so` (Linux)

Run dependency checks before development:

```bash
./sync-deps.sh
```

### Development

```bash
wails doctor
wails dev -tags sqlite_extension
```

### Build

```bash
wails build -tags sqlite_extension
```

## Local RAG Troubleshooting

- `Ask AI unavailable` on startup:
	- Run `./sync-deps.sh`
	- Confirm all required files exist under `asset/`
- `no such module: vec0`:
	- Ensure build includes `-tags sqlite_extension`
	- Ensure platform-specific `vec0` library exists in `asset/`
- ONNX runtime load failure:
	- Ensure platform-specific ONNX runtime library exists in `asset/`
	- Rebuild with `CGO_ENABLED=1`
- Build fails due to missing C compiler:
	- Install MSVC Build Tools (Windows) or equivalent platform toolchain

## Documentation

- System design: [doc/ARCHITECTURE.md](doc/ARCHITECTURE.md)
- User and interaction flow: [doc/APP_FLOW.md](doc/APP_FLOW.md)
- Retrieval pipeline: [doc/RAG.md](doc/RAG.md)
- Project structure: [doc/PROJECT_STRUCTURE.md](doc/PROJECT_STRUCTURE.md)

## Constraints

- Keep the system simple and implementation-ready
- Avoid unnecessary abstraction and premature optimization
- Do not use LangChain, agent orchestration, or chatbot-style memory


## Sprint 14: FSRS Integration & Smart Scaling
**Goal:** Tie generated assessments to memory algorithms and automate background generation.

* **FSRS Hookup:** Connect the FSRS scoring algorithm to the quiz and Socratic examiner outputs. Track success/failure on individual generated questions.
* **Density Scaling:** Replace hardcoded assessment counts. Pass the total chunk length to the FAST_LLM and instruct it to scale the number of flashcards and quiz questions to match the material density.
* **Background Queue:** Implement a Go routine worker. Identify the next two reading tasks in the schedule. Pre-build the quizzes and flashcards for these upcoming sessions while the user reads the current text.

## Sprint 15: Task Management & Dashboard Routing
**Goal:** Finalize the user dashboard experience.

* **Persistent Checklist:** Build a task checklist in the left sidebar. Allow users to tick off items to log completed work.
* **State Routing:** Wire the dashboard buttons to control application state. Clicking a reading task mounts `Reader.vue`, loads the topic, and physically locks the context to the assigned pages.
* **Completion State:** Clear the dashboard state when the user completes the daily queue.

## Sprint 16: Concurrency & Tools Sidebar
**Goal:** Optimize speed and add specific learning utilities.

* **Concurrent Ingestion:** Rewrite the PDF indexing pipeline to use Go routines. Process chapter chunking and ONNX embedding concurrently.
* **Acronym Generator:** Add an acronym tool to the sidebar. Pass the locked active page context to the FAST_LLM and request mnemonic devices. 
* **Mindmap Generator:** Add a mindmap tool that reads the locked page context and outputs structured JSON for a frontend rendering library.
* **Documentation Rewrite:** Update `/doc` files to document the dual-LLM routing, the context-locked schema, and the two-step vector retrieval.