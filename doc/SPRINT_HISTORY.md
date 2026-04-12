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
