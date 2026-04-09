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

Sprint 2 delivered a working RAG-based "Ask AI" feature integrated into the Reader page. Key components: a small SQLite seed dataset (topic "os-scheduling"), TF‑IDF embeddings for retrieval, a simple RAG pipeline that expands chunks to parent sections, prompt assembly, and an OpenAI‑compatible LLM call. The frontend connects via Wails to `AskAI(topicID, question)` and displays answers with citations.

## Included

- **Data & DB**: SQLite schema and seed data (`db.go`)  
- **Embeddings & Retrieval**: TF‑IDF vectors, tokenization, cosine similarity, top‑k search (`embeddings.go`)  
- **RAG Pipeline**: retrieval → parent expansion → prompt assembly → LLM call → citations (`rag.go`)  
- **Backend API**: `GetTopicContent`, `GetAvailableTopics`, `AskAI` (`app.go`)  
- **Reader UI**: topic sections + Ask AI panel (`frontend/src/pages/Reader.vue`)

## Run (dev)

- Install SQLite driver: `go get -u github.com/mattn/go-sqlite3`  
- Set LLM env vars: `LLM_BASE_URL`, `LLM_API_KEY`, `LLM_MODEL`  
- Start dev server: `wails dev`

## Limitations (MVP)

- Single hardcoded topic, TF‑IDF embeddings (non‑neural), DB in temp, requires online LLM, no quiz/FSRS yet.

## Next steps

- Settings UI, persistent config, add topics, quiz generation, FSRS, neural embeddings.

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
