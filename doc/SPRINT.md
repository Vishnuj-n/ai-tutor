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

Get a **fully navigable app UI** with no real logic.

---

## Tasks

### 1. Setup

* Initialize Wails v2 + Vue
* Setup Vue Router

---

### 2. Sidebar Layout

Create:

* Sidebar.vue

Sections:

* Dashboard
* Reader
* Quiz
* Flashcards
* Socratic Tutor
* Settings (bottom)
* Sync button (bottom)

---

### 3. Pages (Empty but Visible)

Create pages:

```text
pages/
- Dashboard.vue
- Reader.vue
- Quiz.vue
- Flashcards.vue
- Socratic.vue
- Settings.vue
```

Each page should:

* Render title
* Have placeholder content

---

### 4. Routing

Ensure:

* Clicking sidebar changes page
* No broken navigation

---

## Output of Sprint 1

* App opens
* Sidebar works
* All pages visible
* Clean layout

---

# Sprint 2 — Reader + Basic RAG (Ask AI)

## Goal

Make **Reader + Ask AI actually work**

---

## Tasks

### 1. Minimal Data Setup

Hardcode 1–2 topics:

```go
Topic: "Operating Systems"
Content: "Round Robin Scheduling..."
```

Store in SQLite or even in-memory (initially OK)

---

### 2. Chunking (Basic)

* Split content into small chunks
* Assign parent_id

Keep simple (no over-engineering)

---

### 3. Embeddings

* Use local embedding model
* Store vectors

---

### 4. RAG Pipeline (Core)

Implement:

```text
Question → Embed → Search → Parent → Prompt → LLM → Answer
```

---

### 5. Backend Function

```go
func AskAI(topicID string, question string) string
```

---

### 6. Reader UI

* Show topic content
* Add Ask AI panel:

  * input
  * button
  * response area

---

### 7. Connect Frontend → Backend

* Call AskAI from Vue
* Display result

---

## Output of Sprint 2

* Open Reader
* Read topic
* Ask question
* Get answer from your content

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
