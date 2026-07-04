# SPRINT_HISTORY.md — AI Tutor

Created: 2026-04-12

This file is a single canonical history of completed sprints. Use this for onboarding, release notes, and auditing changes across sprints. Each entry includes goals, outcomes, key files changed, API/UI surface changes, test status, and short TODOs.

---

## Sprint 1 — UI Shell & Navigation
- Completed: by 2026-04-11
- Goal: Build a minimal, navigable UI shell with primary pages (Dashboard, Reader, Quiz, Flashcards, Socratic).
- Outcome: Full Vue + Wails UI skeleton with sidebar and routes.
- Key files changed:
  - frontend/src/App.vue
  - frontend/src/components/Sidebar.vue
  - frontend/src/pages/*.vue (Dashboard, Reader, Quiz, Flashcards, Socratic)
  - wails.json
- API / UI changes: None (UI-only scaffold), routes and page components added.
- Tests status: Manual UI validation; no backend tests required for this sprint.
- TODOs: N/A

---

## Sprint 2 — Reader + Basic RAG (Ask AI)
- Completed: by 2026-04-11
- Goal: Implement Reader page with RAG retrieval + LLM (Ask AI) integration.
- Outcome: Working retrieval pipeline, LLM prompt assembly, and Reader UI connected via Wails bindings.
- Key files changed:
  - internal/rag/* (RAG pipeline and retrieval code)
  - internal/llm/* (LLM provider adapter)
  - app.go (exposed APIs: `GetTopicContent`, `GetAvailableTopics`, `AskAI`)
  - frontend/src/pages/Reader.vue
- API / UI changes: `AskAI(topicID, question)` added; Reader page shows citations and sections.
- Tests status: Unit/integration tests for retrieval and backend pass in CI-local runs.
- TODOs: Continue to improve retrieval quality and fallback heuristics.

---

## Sprint 3 — Notebook Ingestion & Embeddings
- Completed: by 2026-04-11
- Goal: Accept uploaded documents, extract sections, chunk text deterministically, ingest to DB, and index vectors.
- Outcome: Notebook upload, extraction, deterministic chunking, transactional ingestion, topic extraction, and background indexing.
- Key files changed:
  - internal/notebook/upload.go
  - internal/db/store.go
  - internal/embeddings/onnx.go
  - notebook_endpoints.go (upload & ingestion events)
- API / UI changes: Notebook upload UI and ingestion progress events; `GetNotebooks()` and ingestion endpoints available.
- Tests status: Integration tests for ingestion and DB rollback behavior pass (Windows-friendly cleanup included).
- TODOs: Improve chapter/topic extraction quality and UI for notebook→topic linking.

---

## Sprint 4 — Quiz Generation (Condensed)
- Completed: 2026-04-11 → 2026-04-12
- Goal: Generate topic-scoped multiple-choice quizzes, score answers, and persist attempts for later review.
- Outcome: LLM-based MCQ generation (strict JSON), storage of questions and user attempts, answer scoring, and Quiz UI wired end-to-end.
- Key files changed:
  - app.go (GenerateQuiz, ScoreAnswer, prompt assembly)
  - internal/db/quiz_repo.go (quiz persistence)
  - internal/models/models.go (QuizQuestion / QuizScore types)
  - frontend/src/pages/Quiz.vue (notebook → topic selector + quiz UI)
  - frontend/src/services/appApi.js (bridge functions)
  - internal/rag/indexer.go and internal/db/vector_repo.go (related vector/persistence fixes)
- API / UI changes:
  - `GenerateQuiz(topicID)` and `ScoreAnswer(questionID, userAnswer)` added
  - Quiz page updated to notebook-first cascade selector (notebook → topic)
- Tests status: Backend tests pass (`go test ./...`); frontend build passes; linting clean.
- TODOs: End-to-end runtime validation via `wails dev`; optional improvements include difficulty tagging and quiz history.

---

## Sprint 6 — FSRS Review UI + Backend Wiring
- Completed: 2026-04-14
- Goal: Connect Dashboard and Flashcards UI to FSRS backend and record review ratings.
- Outcome: Dashboard surfaces due-count from the daily plan; Flashcards page sends rating choices and shows next scheduled review.
- Key files changed:
  - frontend/src/pages/Dashboard.vue
  - frontend/src/pages/Flashcards.vue
  - frontend/src/services/appApi.js
  - app.go (`GetTodayPlan`, `GetFlashcards`, `RecordFlashcardReview`)
  - internal/scheduler/service.go
  - internal/db/flashcard_repo.go
  - internal/db/store.go
- API / UI changes:
  - `GetTodayPlan()` added
  - `GetFlashcards(topicID, true)` wired to due-card loading
  - `RecordFlashcardReview(cardID, rating)` wired to review actions
- Tests status: Backend db and scheduler tests pass; frontend review flow wired.
- TODOs: Validate full Wails end-to-end flow; polish review copy and dashboard messaging.

---

## How to run / verify locally

1. Start dev app (requires assets and env vars):

```bash
export LLM_BASE_URL=... LLM_API_KEY=... LLM_MODEL=...
cd <repo-root>
wails dev -tags sqlite_extension
```

2. Run backend tests:

```bash
go test ./...
```

3. Build frontend separately (if needed):

```bash
npm --prefix frontend run build
```

## Current State

**Sprints 1–3: Complete.**

Delivered the UI shell (all pages navigable), Ask AI in the Reader (RAG-based retrieval + LLM), and Socratic tutor mode (guided follow-up questions instead of direct answers). Backend uses SQLite, lexical retrieval, and OpenAI-compatible LLM calls. Frontend wires results via Wails bindings. No LangChain, no chat memory, no over-engineering.

PR size: ~6900 lines across backend/frontend/database because the work spans SQLite schema, embeddings, RAG pipeline, UI pages, and Wails bindings for each feature. UI page like Socratic.vue runs 535 lines on its own (multi-section state, styling, API calls). Normal for full-stack without scaffolding tools.

---

# Sprint 4 — Quiz Generation

**Status: Completed — 2026-04-12.** See `doc/SPRINT_HISTORY.md` for full details.

## Goal

Generate quiz questions from reading material and score answers.

---

For more details see `doc/solutions/SOLUTIONS_2026-04-11.md` and the linked code in `internal/`.

## Core Work

1. **FSRS algorithm**
   - Implement FSRS (or integrate proven library) in Go
   - Calculate next review date based on answer quality
   - Store review history in SQLite

2. **Flashcards page**
   - Show cards due for review today
   - User marks each as easy/good/hard
   - App calculates next review and moves to next card
   - Display running stats (cards learned, cards in learning, new cards)

3. **Progress dashboard**
   - Total cards reviewed
   - Cards mastered
   - Review calendar for next 30 days
   - Streak (optional)

4. **Data model**
   - Link quiz answers to FSRS state
   - Track intervals and difficulty of each card
   - Persist all review history

## Backend API

- `GetFlashcards(topicID string, dueOnly bool) map[string]interface{}` – returns cards for a topic, optionally filtered by due status
- `RecordFlashcardReview(cardID string, rating string) map[string]interface{}` – updates FSRS state and returns review result
- SQLite: `fsrs_cards` and `fsrs_review_log` tables

## Workflow

1. User answers quiz → stored in `user_answers`
2. First quiz answer creates flashcard entry in `card_state` (new)
3. User reviews on Flashcards page, marks easy/good/hard
4. Backend recalculates interval, updates `review_history` and `card_state`
5. Dashboard pulls from `card_state` for progress counts

## Dependencies

- Quiz scores feed into cards (no quiz changes needed)
- Reader unchanged
- Ask AI unchanged

## Definition of Done

- Flashcards page shows due cards
- FSRS calculation works (mark easy/good/hard)
- Next review date updates correctly
- Dashboard shows progress
- Data persists across sessions

---

## Architecture Rules

Across all sprints:

- No LangChain, no complex orchestration
- LLM calls are direct HTTP (OpenAI-compatible API)
- Business logic lives in Go; UI wires the results
- One request in, one response out
- Repository pattern for all SQLite access
- Pointers only when modifying data
- Avoid unnecessary interfaces
- No premature optimization

## Goal (Overall)

Build a **working skeleton of the app with visible UI + one core intelligent feature**.

Priority:

1. Basic UI (all pages visible and navigable)
2. Functional RAG-based **Ask AI (Socratic Tutor)**
3. Then FSRS scheduler (after)

---

# Why This Order

Do NOT start with FSRS.

Reason:

* FSRS depends on:

  * quiz generation
  * user progress
  * review flow
* High dependency chain → slows you down

Start with:

> **RAG Ask AI (Socratic Tutor)**

Because:

* Directly usable feature
* Validates your core architecture (RAG + LLM)
* Easier to implement and debug

---
## 📍 Sprint 6: The "Command & Review" Loop (Do this right now)
**Goal:** Wire Vue frontend to FSRS backend so review flow is usable.
* **1. Dashboard UI:** Hook `Dashboard.vue` to `due_at` and `suspended`.
    * Show "X Cards Due Today" from `service.go` Daily Plan.
* **2. Flashcards UI:**
    * Send `Again (1)`, `Hard (2)`, `Good (3)`, `Easy (4)` ratings from `Flashcards.vue` to `RecordFlashcardReview`.
    * Show next review using `scheduled_days` (e.g. "See you in 3 days!").
* **Outcome:** Flashcards review session is wired end-to-end with FSRS.

## 🏗️ Architecture Pivot Note

The project previously experimented with a proactive orchestration model.

The architecture has now been simplified into a deterministic SQLite-driven Persistent Queue Architecture.

Current sprint planning and implementation should follow the queue model exclusively

---

## Sprint 15 — Simplified FSRS Calibration & Enhanced Features
- Completed: 2026-06-28
- Goal: Simplify FSRS calibration with clean initial states, add cloud sync, streak tracking, and UI enhancements.
- Outcome: FSRS flashcards now start in clean Review state with day-based offsets based on quiz performance. Cloud sync with stable identifiers implemented. Streak tracking and UI enhancements added.
- Key files changed:
  - internal/study/flashcard.go (simplified FSRS calibration)
  - quiz_flashcard_test.go (updated calibration tests)
  - doc/ARCHITECTURE.md (updated FSRS calibration documentation)
  - doc/DATA_API.md (added GenerateFlashcardsForQuizTask API)
  - doc/SCHEMA.md (added FSRS calibration notes)
  - doc/SPRINT.md (added Sprint 15)
- API / UI changes:
  - `GenerateFlashcardsForQuizTask` now sets clean Review state with day-based offsets
  - Added cloud sync functionality with stable identifiers
  - Added streak tracking with calendar widget
  - Added flip-back button to flashcards
  - Enhanced sidebar animations and scroll progress bar
- Tests status: All tests pass including updated `TestFSRSCalibrationEasyAndDoubleGood`
- TODOs: Validate full cloud sync end-to-end flow