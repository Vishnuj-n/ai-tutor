# Sprint History — AI Tutor

Created: 2026-04-12. Canonical history of completed sprints for onboarding, release notes, and auditing.

---

## Sprint 1 — UI Shell & Navigation
- **Completed:** 2026-04-11
- **Goal:** Minimal navigable UI shell (Dashboard, Reader, Quiz, Flashcards, Socratic).
- **Outcome:** Full Vue + Wails skeleton with sidebar and routes.
- **Key files:** `App.vue`, `Sidebar.vue`, `pages/*.vue`, `wails.json`
- **Tests:** Manual UI validation.

## Sprint 2 — Reader + Basic RAG
- **Completed:** 2026-04-11
- **Goal:** Reader with RAG retrieval + LLM (Ask AI).
- **Outcome:** Retrieval pipeline, LLM prompt assembly, Reader UI via Wails bindings.
- **Key files:** `internal/rag/*`, `internal/llm/*`, `app.go`, `Reader.vue`
- **API:** `AskAI(topicID, question)` added.

## Sprint 3 — Notebook Ingestion & Embeddings
- **Completed:** 2026-04-11
- **Goal:** Accept documents, extract sections, chunk text, ingest to DB, index vectors.
- **Outcome:** Upload, extraction, deterministic chunking, transactional ingestion, background indexing.
- **Key files:** `upload.go`, `store.go`, `onnx.go`, `notebook_endpoints.go`

## Sprint 4 — Quiz Generation
- **Completed:** 2026-04-12
- **Goal:** Topic-scoped MCQ generation, scoring, persist attempts.
- **Outcome:** LLM-based MCQ generation, question storage, scoring, Quiz UI wired end-to-end.
- **Key files:** `app.go`, `quiz_repo.go`, `models.go`, `Quiz.vue`
- **API:** `GenerateQuiz(topicID)`, `ScoreAnswer(questionID, userAnswer)` added.

## Sprint 6 — FSRS Review UI + Backend
- **Completed:** 2026-04-14
- **Goal:** Connect Dashboard/Flashcards UI to FSRS backend, record ratings.
- **Outcome:** Dashboard surfaces due-count, Flashcards sends ratings, shows next review.
- **Key files:** `Dashboard.vue`, `Flashcards.vue`, `appApi.js`, `app.go`, `service.go`
- **API:** `GetTodayPlan()`, `GetFlashcards(topicID, true)`, `RecordFlashcardReview(cardID, rating)`

## Sprint 15 — Simplified FSRS Calibration & Enhanced Features
- **Completed:** 2026-06-28
- **Goal:** Simplify FSRS calibration, add cloud sync, streak tracking, UI enhancements.
- **Outcome:** FSRS flashcards start in clean Review state with day-based offsets. Cloud sync with stable identifiers. Streak tracking + UI enhancements.
- **Key files:** `flashcard.go`, `quiz_flashcard_test.go`, various doc files
- **API:** `GenerateFlashcardsForQuizTask` updated, cloud sync + streak APIs added.

---

## Architecture Rules (All Sprints)

- No LangChain, no complex orchestration
- LLM calls = direct HTTP (OpenAI-compatible)
- Business logic in Go; UI wires results
- One request in, one response out
- Repository pattern for SQLite access
- Pointers only when modifying data
- No premature optimization

## How to Run

```bash
export LLM_BASE_URL=... LLM_API_KEY=... LLM_MODEL=...
wails dev -tags sqlite_extension
go test ./...
npm --prefix frontend run build
```
