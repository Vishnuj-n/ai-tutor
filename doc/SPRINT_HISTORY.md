<<<<<<< HEAD
# Sprint History

## Sprint 4 — Quiz Generation (Completed 2026-04-12)

- Summary: Implemented quiz generation from content, answer scoring, Quiz UI, accessibility improvements, and database integrity fixes.

### Completed
- Generated multiple-choice questions from content and persisted them in SQLite
- Answer scoring (backend) with feedback and hints
- Quiz UI with one-question-per-screen, accessibility fixes (fieldset/legend, grouped radios)
- State cleanup on topic change and frontend linting
- Foreign-key integrity fixes for quiz regeneration and backend tests passing

### Remaining
- Minor frontend TODOs in `frontend/src/pages/Notebook.vue` (UX improvements, download/preview). No blocking Sprint 4 issues.

### Goal

Generate quiz questions from reading material and score answers.

### Core Work

1. Generate questions from content
	- Receive topic ID from frontend
	- LLM reads section and creates multiple-choice questions linked to specific passages
	- Store in SQLite with source references

2. Score answers
	- Backend scores user response against key
	- Return numeric score, explanation, and hint for wrong answers
	- Use LLM for open-ended questions where a rubric doesn't fit

3. Quiz page UI
	- Display one question per screen
	- Show feedback immediately
	- User advances manually (no auto-progress)

### Backend API

- `GenerateQuiz(topicID string) []Question` – returns generated questions with answer keys
- `ScoreAnswer(questionID, userAnswer string) Score` – returns score and feedback
- SQLite: `questions`, `user_answers` tables

### Dependencies

- Reuse existing content from Reader (no change)
- Reuse existing LLM provider (extend it)
- No FSRS yet; just store attempts

### Definition of Done

- Quiz page displays questions without crashing
- Answers score correctly
- Wrong answers include a hint
- App handles network failure gracefully

### Validation

- `go test ./...` — passing
- `golangci-lint run ./...` — passing
- `npm --prefix frontend run lint` — passing

## Sprint 3 — Socratic Tutor (Completed)

- Summary: Delivered guided Socratic tutor mode, prompt updates, and Reader UX improvements.

## Sprint 2 — Reader + Basic RAG (Completed)

- Summary: Implemented Reader page, a working RAG pipeline, and LLM integration for Ask AI.

## Sprint 1 — UI Skeleton (Completed)

- Summary: Built the navigable UI shell, pages, and basic layout components.
=======
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

## Notes & References
- Canonical solutions log: `doc/solutions/SOLUTIONS_2026-04-11.md`
- Sprint summary and next steps: `doc/SPRINT.md`
- Current working branch for these changes: `feature/quiz`

If you want, I can split each sprint into per-file release notes under `doc/sprints/` or add a small generator script that derives entries from commit/PR metadata.
>>>>>>> 24f30705a197865c33ca221458957fb5d6a22075
