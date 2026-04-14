# SPRINT.md — AI Tutor

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
## 📍 Sprint 6: The "Command & Review" Loop (Do this right now)
**Goal:** Wire Vue frontend to FSRS backend so review flow is usable.
* **1. Dashboard UI:** Hook `Dashboard.vue` to `due_at` and `suspended`.
    * Show "X Cards Due Today" from `service.go` Daily Plan.
* **2. Flashcards UI:**
    * Send `Again (1)`, `Hard (2)`, `Good (3)`, `Easy (4)` ratings from `Flashcards.vue` to `RecordFlashcardReview`.
    * Show next review using `scheduled_days` (e.g. "See you in 3 days!").
* **Outcome:** Flashcards review session is wired end-to-end with FSRS.

## 📍 Sprint 7: The "Augmented Reader" (The Split-Screen Hub)
**Goal:** Build the "Encoding" phase. This is where the student actually learns the PDF before FSRS tests them.
* **1. PDF.js Integration:** Embed a PDF viewer in the left pane of `Reader.vue`.
* **2. Linear Navigation:** Use the `parent` chunks from your RAG database to find the `page_num`. When a user clicks a Topic, tell the PDF viewer to jump to that exact page.
* **3. The AI Companion (Right Pane):** Add the chat interface on the right side. When the student highlights a tricky paragraph in the PDF, let them click "Explain this" to trigger an LLM clarification without leaving the page.
* **4. The "Mark Learned" Trigger:** At the end of the section, the user clicks "Mark as Learned", which generates the Flashcards and pushes them into your FSRS engine.
* **Outcome:** A highly impressive, professional Split-Screen learning environment that doesn't hallucinate because the PDF is always visible.

## 📍 Sprint 8: The "AI Examiner" (Written Testing)
**Goal:** Replace the old Socratic Tutor with a graded, short-answer assessment tool.
* **1. Prompt Engineering:** Build an LLM prompt that asks a question based on the topic, reads the student's typed answer, and grades it out of 10.
* **2. FSRS Hook:** Translate that 1-10 score into an FSRS rating (1=Again, 4=Easy).
* **3. Generic Logging:** Save this interaction using your newly built `fsrs_review_log` with `activity_type = "short_answer"`.
* **Outcome:** You prove your FSRS engine is extensible beyond just flashcards.

## 📍 Sprint 9: Scalability & Polish (The Backlog)
**Goal:** Clean up the rough edges for a production-ready feel.
* **1. SQLite Connection Fix:** Implement WAL mode and connection pooling to stop the UI from locking during heavy 100-page PDF ingestion.
* **2. Multi-Notebook Support:** Add the UI routing to switch between "Physics 101" and "Computer Architecture" databases.
* **Outcome:** The app is ready for massive textbooks and multiple subjects.

---

**Your immediate next step:** Create a new branch (e.g., `feat/sprint-6-dashboard-ui`). Do not touch the backend. Focus entirely on `Dashboard.vue` and `Flashcards.vue` to make your FSRS engine come to life.