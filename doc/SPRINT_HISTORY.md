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

---
## Notes & References
- Canonical solutions log: `doc/solutions/SOLUTIONS_2026-04-11.md`
- Sprint summary and next steps: `doc/SPRINT.md`

If you want, I can split each sprint into per-file release notes under `doc/sprints/` or add a small generator script that derives entries from commit/PR metadata.
