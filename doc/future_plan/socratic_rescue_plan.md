# 🧭 3-Strike Socratic Rescue Pipeline Plan

This document outlines the complete architectural, structural, database, testing, and UI verification plan for the **3-Strike Socratic Rescue Pipeline**.

---

## 1. Core Architecture Idea

The goal of this feature is **cognitive damage control**. When a UPSC aspirant fails a concept/quiz three times consecutively, the system assumes they are experiencing an "illusion of competence" or a bad learning loop. Instead of letting them repeatedly attempt the same quiz, the app intervenes dynamically.

```
                      [ User Fails Pop-Quiz for 3rd Time ]
                                       │
        ┌──────────────────────────────┴──────────────────────────────┐
        ▼                                                             ▼
┌──────────────────────────────┐                              ┌──────────────────────────────┐
│  Step 1: The Clean Slate     │                              │  Step 2: Queue Interleaving  │
│  • Wipes faulty flashcards   │                              │  • Unlocks primary timeline  │
│  • Resets FSRS history       │                              │  • Injects SOCRATIC_REMEDIAL │
└──────────────────────────────┘                              └──────────────────────────────┘
                                       │
                                       ▼
                        ┌──────────────────────────────┐
                        │ Step 3: Dual-Lane Rescue UI  │
                        │ • Left: Local Socratic Chat  │
                        │ • Right: External Escape     │
                        └──────────────────────────────┘
```

---

## 2. Step-by-Step System Flow

### Step 1: The "Clean Slate" Database Intervention
- **The Action:** The backend intercepts the third consecutive failure entry for a specific text chunk during quiz grading.
- **The Logic:**
  - Marks the active `QUIZ` task as `FAILED` or `COMPLETED` depending on lifecycle rules, unlocking the primary study timeline so it doesn't freeze.
  - Executes an atomic deletion or suspension of all flashcards linked to that specific chunk or topic.
  - Resets the FSRS scheduler parameters/history for the associated topic/chunk to save the student from "ease hell" and review fatigue.

### Step 2: Inverting the Queue (Bookshelf Interleaving)
- **The Action:** The backend inserts a new task of type `SOCRATIC_REMEDIAL` into `study_queue`.
- **The Logic:**
  - Instead of stacking this task sequentially blocking the primary timeline, the task is marked with a specialized tag or placed in a way that separates it from standard linear queue progression.
  - On the dashboard, this renders in a distinct "Rescue Panel" lane. The student can continue reading new materials in their primary study stream while their conceptual debt is safely held in the rescue lane.

### Step 3: The Dual-Lane Breakdown View
When the student activates the remedial task from the dashboard, a split-pane layout is mounted:
- **Left Pane (Internal Local Socratic Chat):** Loads the raw ~2,500-word text block. It boots up the local LLM engine with explicit system instructions to act as a strict examiner who *never* gives flat summaries, only leading questions to guide the student to correct understanding.
- **Right Pane (The External Premium Escape):** A fallback card that copies the full text alongside a pre-engineered expert prompt template to their clipboard, allowing the user to instantly leverage cloud models (like Claude or ChatGPT) if local hardware capabilities are exceeded.

---

## 3. Database Schema Updates
To support this pipeline:
- `reread_attempts` or a similar tracking table will store consecutive failures.
- A new task type `SOCRATIC_REMEDIAL` added to the `study_queue` task type enum.
- Flashcard deletion or archiving routines mapped to the chunk/topic.

---

## 4. Automated Testing Strategy (`go test`)
To verify transactional integrity in isolation:
1. **State Seeding:** Initialize an in-memory SQLite database instance.
2. **Mocking:** Insert a dummy reading task and mock two consecutive quiz failures.
3. **Execution:** Submit a third quiz failure payload.
4. **Assertions:**
   - Verify that flashcards associated with that chunk are deleted/suspended (count == 0).
   - Verify that the primary blocked task is unblocked.
   - Verify that a `SOCRATIC_REMEDIAL` task is successfully inserted.

---

## 5. Physical UI Verification Strategy (Vite Dev Mode)
To keep frontend design loops frictionless:
1. **Developer Flag:** Use Vite's environment flags (`import.meta.env.DEV`) to detect local development.
2. **Sandbox Admin Panel:** Pin a floating developer panel at the bottom of the screen.
3. **Bypass Action:** A button *"💥 Force 3-Strike Rescue UI State"* which triggers a backend test-endpoint to force the DB into a post-3-strike failure state.
4. **Instant Verification:** The frontend UI catches the state and mounts the split-screen view without requiring the developer to manually fail three quizzes.
