# AI Tutor

AI Tutor is a local-first desktop app that guides learners through a structured study loop.

It is not a chatbot, PDF viewer, or standalone flashcard app. It is a guided tutor system:

1. Read concepts
2. Understand with contextual AI help
3. Review with FSRS spaced repetition

## Product Overview

### What

- A guided learning workflow driven by a persistent SQLite study queue
- Local storage for content, embeddings, progress, and scheduling state
- Topic-scoped AI for explanations and quiz generation only

### Why

- Keep user data private and fully local by default
- Reduce decision fatigue with a clear daily study flow
- Keep implementation simple and maintainable for a solo developer

### How

- Backend and desktop shell: Go + Wails
- Frontend: Vue 3 multi-page app with left sidebar navigation
- Local data: SQLite + sqlite-vec embeddings
- LLM layer: OpenAI-compatible API with dual-tier support (Fast + Heavy)

## Core Features

- **Dashboard:**
	- Today tasks (due reviews, new topics, generated quizzes)
	- Progress summary
	- Starvation protection (review-heavy queues surface reading tasks)
- **Reader:**
	- Structured content view with page locking
	- Ask AI panel (primary placement)
	- Trust-based reading completion (no surveillance)
- **Quiz:**
	- Topic-based quiz sets generated from learned content
	- Synchronous LLM generation with loading state
	- Pass/fail evaluation with reread remediation
- **Flashcards:**
	- FSRS spaced repetition: Again, Hard, Good, Easy
	- Queue-driven review sessions (one task per chunk)
	- Explain action (secondary Ask AI placement)
- **Socratic Tutor:**
	- Guided questioning mode scoped to current topic
	- 2-strike rescue pipeline for struggling topics
	- Enter to send, Shift+Enter for new line
- **SocraticRescue (Concept Rescue):**
	- 2-strike rescue pipeline for repeated quiz failures
	- Pre-engineered Socratic prompt for external LLM copy-to-clipboard
	- Queue-blocking until rescue session completed
- **Examiner (Written Assessment):**
	- Advanced assessment tasks for mastery verification
	- Notebook selection and scoring
- **Notebooks:**
	- Bookshelf for managing uploaded textbooks
	- Batch PDF upload with chapter extraction
	- Per-notebook priority (1-10 scale)
- **Settings:**
	- Dual-tier LLM config (Fast + Heavy) with provider presets (Groq, OpenAI, OpenRouter, Custom)
	- API key storage via OS keyring
	- RAG toggles and configuration
	- Theme selector (Light Classic, Warm Sepia, Deep Indigo Night, Nord Frost, Forest Emerald)
	- Study profiles with deadlines
	- Cloud sync (URL + token)
- **Onboarding:**
	- Multi-step first-run setup wizard

## Local-First and Offline Behavior

**Works offline:**

- Reading
- FSRS review
- Scheduling
- Local progress and content access

**Requires internet:**

- Ask AI
- Quiz generation

**Failure rule:**

- If AI is unavailable, show clear error and do not simulate output

## Tech Stack

- Go 1.26
- Wails v2
- Vue 3 (multi-page, hash-based routing)
- SQLite + sqlite-vec (vec0 extension)
- ONNX Runtime (local INT8 embedding model)
- go-fsrs/v4 (FSRS spaced repetition algorithm)
- OpenAI-compatible LLM API (dual-tier: Fast + Heavy)
- go-keyring (OS keyring for API key storage)

## Quick Start

### Prerequisites

- Go 1.26+
- Node.js 20+
- Wails CLI
- CGO-capable compiler toolchain
- Local RAG assets in `asset/`:
	- `tokenizer.json`
	- `model_int8.onnx`
	- `onnxruntime.dll` (Windows) / `libonnxruntime.dylib` (macOS) / `libonnxruntime.so` (Linux)
	- `vec0.dll` (Windows) / `vec0.dylib` (macOS) / `vec0.so` (Linux)

Run dependency checks and asset download:

```bash
# macOS / Linux
./sync-deps.sh

# Windows
./windows-sync-deps.ps1
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
	- Run `./sync-deps.sh` (or `./windows-sync-deps.ps1` on Windows)
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
- App flow and user interactions: [doc/APP_FLOW.md](doc/APP_FLOW.md)
- Database schema: [doc/SCHEMA.md](doc/SCHEMA.md)
- API contracts: [doc/DATA_API.md](doc/DATA_API.md)
- Module responsibilities: [doc/AGENT_MAP.md](doc/AGENT_MAP.md)
- Current sprint roadmap: [doc/SPRINT.md](doc/SPRINT.md)
- RAG pipeline: [doc/RAG.md](doc/RAG.md)
- UI/UX design: [doc/DESIGN.md](doc/DESIGN.md)
- Platform support: [doc/PLATFORM_SUPPORT.md](doc/PLATFORM_SUPPORT.md)

## Constraints

- Keep the system simple and implementation-ready
- Avoid unnecessary abstraction and premature optimization
- Do not use LangChain, agent orchestration, or chatbot-style memory