# SPRINT.md — AI Tutor

## Current State

**Sprints 1–3: Complete.**

Delivered the UI shell (all pages navigable), Ask AI in the Reader (RAG-based retrieval + LLM), and Socratic tutor mode (guided follow-up questions instead of direct answers). Backend uses SQLite, lexical retrieval, and OpenAI-compatible LLM calls. Frontend wires results via Wails bindings. No LangChain, no chat memory, no over-engineering.

PR size: ~6900 lines across backend/frontend/database because the work spans SQLite schema, embeddings, RAG pipeline, UI pages, and Wails bindings for each feature. UI page like Socratic.vue runs 535 lines on its own (multi-section state, styling, API calls). Normal for full-stack without scaffolding tools.

---

# Sprint 4 — Quiz Generation

## Goal

Generate quiz questions from reading material and score answers.

🥷 SPRINT.md — AI Tutor (Compressed Completed Summary)

## Summary

This document condenses completed work through Sprint 4. Key outcomes are listed concisely; implementation details are preserved in the repository's solutions and docs.

## Completed (high level)

- Sprints 1–3: UI shell, navigation, Reader with RAG (Ask AI), and Socratic tutor — all implemented and integrated with Wails.
- Notebook ingestion: extraction, deterministic chunking, transactional ingestion, status and progress events, and topic-chapter extraction.
- Embeddings & retrieval: ONNX embedder + vec0 integration, runtime diagnostics, and background indexing with SQLite pool constraints to ensure extension stability.
- Performance: batch vector persistence (single-transaction writes) and timing instrumentation to reduce indexing overhead.
- Quiz feature (Sprint 4): LLM-based MCQ generation (strict JSON), persistence (`questions`, `user_answers`), scoring API, and a Notebook→Topic-aware Quiz UI.
- Build & QA: Vite Windows build fix, linting and formatting, Windows-friendly test cleanup, and repository tests passing.

## Sprint 4 — Quiz Generation (Condensed)

- Goal: Produce topic-scoped multiple-choice quizzes, score answers, and persist attempts for review.
- APIs: `GenerateQuiz(topicID)` and `ScoreAnswer(questionID, userAnswer)` implemented and wired to the frontend.
- UI: Quiz page shows one question at a time, immediate feedback, manual progression.
- Status: Code, frontend build, and tests for the feature are complete and passing.

## Next Steps

- Run end-to-end validation in `wails dev` to verify runtime behavior and LLM responses.
- Continue with Sprint 5: FSRS scheduler and Flashcards, using quiz results as card seeds.

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

- `GetFlashcards(dueOnly bool) []Card` – returns cards with next review dates
- `RecordReview(cardID string, quality int) Card` – updates FSRS state and returns next card
- `GetProgress() Progress` – returns metrics
- SQLite: `flashcards`, `review_history`, `card_state` tables

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
# SPRINT.md — AI Tutor (Sprint 1 → Sprint 3)

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

# Sprint 1 — UI Skeleton + Navigation

## Goal

Build a fully navigable UI shell with no backend logic.

## Constraints

* UI only
* No API calls
* No state management
* No backend logic

## Required Work

* Setup Wails + Vue + Vue Router
* Build `App.vue` layout:
  * fixed sidebar
  * flex main content
  * page background `#f9f9fb`
* Create `Sidebar.vue` and pages:
  * Dashboard
  * Reader
  * Quiz
  * Flashcards
  * Socratic Tutor
  * Settings
* Implement route navigation and active highlighting
* Build Dashboard layout to mirror screenshot:
  * hero greeting
  * current session card
  * due reviews card
  * weekly insights
  * curated focus list

## Output

* App runs
* Sidebar navigation works
* All pages render
* Dashboard matches the reference structure
* UI follows design system: no borders, layered surfaces, whitespace-based separation

## Done

* No broken routes
* No console errors
* Clean, minimal UI
* Simple, readable code

---
---
# Sprint 2 — Reader + Basic RAG (Ask AI)

## Goal

Make **Reader + Ask AI actually work**

---

## Summary

Sprint 2 delivered a working RAG-based "Ask AI" feature integrated into the Reader page. Key components: a small SQLite seed dataset (topic "os-scheduling"), temporary lexical retrieval for MVP validation, a simple RAG pipeline that expands chunks to parent sections, prompt assembly, and an OpenAI‑compatible LLM call. The frontend connects via Wails to `AskAI(topicID, question)` and displays answers with citations.

## Included

- **Data & DB**: SQLite schema and seed data (`db.go`)  
- **Embeddings & Retrieval (MVP)**: lexical vectors, tokenization, cosine similarity, top‑k search (`embeddings.go`)  
- **RAG Pipeline**: retrieval → parent expansion → prompt assembly → LLM call → citations (`rag.go`)  
- **Backend API**: `GetTopicContent`, `GetAvailableTopics`, `AskAI` (`app.go`)  
- **Reader UI**: topic sections + Ask AI panel (`frontend/src/pages/Reader.vue`)

## Run (dev)

- Install SQLite driver: `go get -u github.com/mattn/go-sqlite3`  
- Set LLM env vars: `LLM_BASE_URL`, `LLM_API_KEY`, `LLM_MODEL`  
- Start dev server: `wails dev`

## Limitations (MVP)

- Single hardcoded topic, temporary non-neural retrieval, DB in temp, requires online LLM, no quiz/FSRS yet.

## Next steps

- Settings UI, persistent config, add topics, quiz generation, FSRS, ONNX INT8 embeddings + `sqlite-vec` retrieval.

---

# Sprint 3 — Socratic Tutor + Improve UX

## Goal

Turn Ask AI into **guided learning (Socratic style)**

---

## Tasks

### 1. Socratic Mode (Simple)

Instead of:

> “Here is answer”

Do:

* Ask follow-up questions
* Guide thinking

Prompt change only (no complex system)

---

### 2. Socratic Page

* Input question OR start session
* Show:

  * AI question
  * user answer
  * next question

Keep stateless per step (no chat history needed initially)

---

### 3. Improve Reader UX

* Better layout
* Split:

  * content
  * AI panel

---

### 4. Error Handling

* If no internet:

  * show “AI unavailable”

---

### 5. Code Cleanup

* Separate:

  * RAG logic
  * DB logic
* Introduce basic repository pattern

---

## Output of Sprint 3

* Reader works
* Ask AI works
* Socratic Tutor works (basic)
* Clean UI foundation

---

# What You Will Have After Sprint 3

* Full UI structure ✅
* Working RAG system ✅
* First “intelligent” feature ✅
* Clean architecture base ✅

---

# What Comes NEXT (Sprint 4+ Preview)

* Quiz generation
* FSRS scheduler
* Flashcards system
* Progress tracking

---

# Rules During These Sprints

* Keep everything simple
* No over-engineering
* No LangChain
* No complex state management
* One feature at a time

---

# Final Recommendation

Start with:

> UI → RAG → Socratic → then FSRS

This ensures:

* visible progress
* motivation
* stable foundation

---

# Definition of Done (Sprint 3)

* You can:

  * open app
  * navigate pages
  * read topic
  * ask AI
  * get contextual answer

If this works, your foundation is correct.
