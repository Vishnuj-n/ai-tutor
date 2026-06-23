# AI Tutor App Flow
- **Legacy term note:** The term `blocks` has been replaced by `chunks`. The API still uses `block_id` as the identifier for a chunk. See `doc/SCHEMA.md` for mapping.

## Core Philosophy: Persistent Guided Study Queue

**Reference:** `ARCHITECTURE.md` for complete system design, queue ordering rules, and architectural philosophy.

This document describes **runtime flow, user interaction sequence, and lifecycle behavior**. Queue-driven progression is deterministic and recommended, and manual study entry points are also supported. Both paths use SQLite as the source of truth and must converge on the same canonical bootstrap and ownership semantics.

Canonical checkpoint flow:

Dashboard -> Reader -> Quiz -> Dashboard

**Reading Layer**: Focuses on immediate comprehension validation and progression readiness.
**Retention Layer**: Focuses on long-term retention using FSRS-based spaced repetition.

Reader completes the reading task only. The backend generates and activates the QUIZ follow-up task, and the Dashboard owns progression again after quiz submission and evaluation.

---

## 1. The Queue Loop (Primary Flow)

### What

The application follows a deterministic SQLite-driven queue:

```
Dashboard fetches next pending task
→ User clicks task → Status becomes ACTIVE
→ Mount correct module/view
→ User completes/skips task
→ Mark task COMPLETED/SKIPPED/FAILED
→ Insert follow-up tasks (if any)
→ Repeat
```

### Multi-Notebook Priority

Multiple notebooks are supported with deterministic prioritization:

- Notebooks have `priority INTEGER DEFAULT 5` (1-10 scale)
- Higher priority notebooks surface more frequently
- Lower priority notebooks still eventually appear
- Priority is a **deterministic bias** (query-time rule, not adaptive scheduling)

### Queue Ordering Rules

**Reference:** `ARCHITECTURE.md` Section 7 for complete priority hierarchy and SQL query.

Explicit priority hierarchy (task type first, then notebook priority):

| Order | Task Type |
|-------|-----------|
| 1 | `FLASHCARD_SYNC` (cloud sync pending) |
| 2 | `FLASHCARD_REVIEW` (due reviews) |
| 3 | `REREAD` |
| 4 | `QUIZ` |
| 5 | `READING` |
| 6 | `SOCRATIC_REMEDIAL` (concept rescue) |
| 7 | `EXAMINER` |

Then apply notebook priority bias within each tier.

### Why

**Reference:** `ARCHITECTURE.md` Section 1 for architectural rationale.

Runtime benefits:

### How

1. **Dashboard queries** `study_queue` for next `PENDING` task (with ordering rules)
2. **User clicks task** → Status becomes `ACTIVE`, `activated_at` timestamp set
3. **Router opens** correct module with context
4. **Module renders** content based on `task_type` and `block_id`
5. **User completes task** → Module calls `CompleteTask(taskID, result)`
6. **Backend marks** task `COMPLETED`/`SKIPPED`/`FAILED`, inserts follow-up tasks
7. **Dashboard refreshes** showing next pending task

Manual study actions, such as opening Quiz, Flashcards, Reader, or Written Assessment directly, are valid when they call the same backend initialization and retrieval helpers instead of re-implementing them per route. Generated QUIZ follow-up tasks may be activated immediately after Reader completion as part of the same checkpoint chain, but that remains a queue transition rather than arbitrary module-to-module routing.

### Task Lifecycle Semantics

Explicit state machine:

```
PENDING → ACTIVE (when user opens task)
  ↓
COMPLETED (on success)
  ↓
SKIPPED (on user bypass - auditable)
  ↓
FAILED (on generation error - can retry)
```

**Crash Recovery:** On startup, any `ACTIVE` tasks older than 30-minute timeout revert to `PENDING`. This ensures restart-safe queue recovery.

---

## 2. Ingestion Pipeline

### What

PDF upload → Chapter selection → Sliding window chunking → READING tasks inserted

### Why

**Reference:** `ARCHITECTURE.md` Section 5 for chunking rationale.

### How

1. **PDF Upload**: User uploads PDF, system extracts text
2. **Chapter Selection**: User reviews/prunes extracted chapters
3. **Sliding Window Chunking**:
   - 2500-word chunks
   - 200-word overlap between chunks
   - Deterministic, no AI involvement in boundary decisions
4. **READING Tasks Inserted**: One task per chunk into `study_queue`

---

## 3. Reading Flow

### What

User completes reading task → Backend generates follow-up QUIZ task

### Why

**Reference:** `ARCHITECTURE.md` Section 8 for synchronous generation rationale.

### Reading Completion (Trust-Based)

Reading tasks use trust-based completion:

- User decides when reading is complete
- Complete Session button stays enabled during active reading task
- No enforced page-completion validation
- No engagement surveillance, timers, or tracking

### How

1. User clicks **Complete Session** when they feel ready (button always enabled)
2. Frontend shows **loading spinner**
3. Backend calls LLM synchronously
4. Reading completion closes the reading task only; it does not measure mastery or progression quality
5. Backend inserts and activates the generated **QUIZ task** in `study_queue`
6. Dashboard may immediately surface the quiz as the next pending checkpoint

### Quiz Generation States

QUIZ tasks have explicit generation lifecycle:

| State | Meaning |
|-------|---------|
| `GENERATING` | LLM call in progress |
| `READY` | Quiz ready for user |
| `FAILED` | Generation error - dashboard surfaces explicitly |

**Flow:**
1. Reading complete → QUIZ task inserted with `GENERATING` state
2. LLM called synchronously
3. Success → `generation_status = READY`
4. Failure → `generation_status = FAILED` (user sees explicit error)

The Reader does not route itself to Quiz. Only generated follow-up quiz tasks may transition immediately through the queue loop.

---

## 4. Quiz Flow & Remediation

### What

Quiz submission/evaluation → Queue-driven follow-up

### Why

Remediation is lightweight queue insertion, NOT:

- Forced loops
- Hidden state machines  
- User traps

The app only **recommends** revisiting material.

### How

**IF PASS:**
```
QUIZ task → mark COMPLETED
→ Optionally insert FLASHCARD_REVIEW task or move to next queued task
→ Dashboard shows next pending task
```

**IF FAIL (below threshold):**
```
QUIZ task → mark COMPLETED
→ Insert REREAD task for the material or other remediation follow-ups (if under max attempts)
→ Generate lightweight AI feedback
→ Dashboard shows REREAD as next pending task
```

User can:
- Complete the REREAD task
- Skip it (mark SKIPPED - auditable, can resurface)
- The system does NOT force remediation loops

Dashboard regains orchestration ownership after quiz submission and evaluation.

---

## 4a. Socratic Rescue Pipeline (2-Strike)

### What

When a student fails a quiz twice on the same topic, the system intervenes with a guided rescue flow.

### How

**Strike 1 (quiz fail):**
```
QUIZ task → mark COMPLETED
→ Insert REREAD task (if reread_attempt <= maxAutomaticRereadAttempts=1)
→ Dashboard shows REREAD as next task
```

**Strike 2 (quiz fail again after reread):**
```
QUIZ task → mark COMPLETED
→ SOCRATIC_REMEDIAL task inserted (blocks queue)
→ FSRS flashcards for topic deleted (protect purity)
→ Dashboard shows Concept Rescue as next task
```

**Rescue session:**
```
Student opens SocraticRescue page
→ Source text preview + pre-engineered Socratic prompt
→ Student copies prompt to external LLM (copy-to-clipboard)
→ Student completes external Socratic tutoring session
→ Clicks "I've Completed the Session"
→ SOCRATIC_REMEDIAL task marked COMPLETED
→ Fresh QUIZ task inserted for same topic
```

**Re-quiz outcomes:**
```
[Pass] → Flashcards generated → Topic mastered → Next task
[Fail] → external_help_required flag set on topic → Queue unblocks → Notice shown
```

### Key Behaviors

- SOCRATIC_REMEDIAL **blocks the queue** — student must complete rescue before progressing
- Single rescue cycle only — no infinite loops
- No flashcards generated for failed concepts at any point
- External prompt mode — no local LLM integration, student uses external tool
- `external_help_required` flag on topic prevents further rescue cycles
- Re-quiz identified by `"source": "socratic_rescue_requiz"` in task payload

---

## 4b. FLASHCARD_SYNC (Cloud Sync Recovery)

### What

When cloud sync fails after retry exhaustion, a `FLASHCARD_SYNC` task is inserted to ensure pending sync data is not lost.

### How

1. Cloud sync fails after all retry attempts
2. `FLASHCARD_SYNC` task inserted into queue (priority tier 7, highest)
3. On next sync attempt, if successful → `FLASHCARD_SYNC` tasks resolved (COMPLETED)
4. If sync fails again → new `FLASHCARD_SYNC` task inserted

---

## 5. Flashcards & FSRS

### Retention Layer: Spaced Retrieval

FSRS is a scheduling algorithm only for long-term retention. It calculates intervals for flashcards and examiner tasks; it does not control immediate progression or comprehension validation. Quizzes are separate and do NOT update FSRS state.

### Flashcard Review Granularity

**One `FLASHCARD_REVIEW` task = one review session for a block/chunk.**

- Do NOT create one queue task per flashcard
- A single task represents "review all due cards in this block"
- Prevents queue explosion with many cards

### How

1. When reviews become **due** (per FSRS calculation):
   - Insert `FLASHCARD_REVIEW` task into `study_queue` (one task per block)
2. Dashboard fetches pending review task
3. User completes flashcard session (reviews all due cards in block)
4. FSRS calculates next review interval
5. New `FLASHCARD_REVIEW` task scheduled for future due date

Flashcards become **queue-driven review tasks**, not autonomous review systems.

---

## 6. Examiner Mode

### What

Optional advanced queue task for written assessments.

### How

- Triggered after mastery thresholds (e.g., quiz scores > 80%)
- Appears as `EXAMINER` task type in `study_queue`
- Dashboard-driven, user-triggered
- NOT a hidden autonomous system

### Examiner Task Policy

- Inserted after mastery thresholds
- Assigned elevated queue priority (tier 5, after reviews/quizzes/reading)
- Remain optional (user can skip)
- Appear naturally in queue flow through deterministic ordering
- NOT through hidden orchestration

Prevents starvation: EXAMINER is tier 5, ensuring reviews and reading are not blocked.

---

## 7. Navigation and Layout

### What

Left sidebar navigation with persistent sections:

1. Dashboard (default landing)
2. Reader
3. Notebooks
4. Quiz
5. Flashcards
6. Examiner (WrittenAssessment)
7. Tutor (Socratic)
8. SocraticRescue (Concept Rescue — queue-driven, not in sidebar)
9. Settings (bottom)
10. Sync (bottom)

### Why

Stable mental model; users can always access any module directly, but the **Dashboard queue is the primary workflow**.

---

## 8. Synchronous Generation

**Reference:** `ARCHITECTURE.md` Section 8 for LLM layer design.

All AI generation is synchronous. User clicks Complete → Loading spinner → LLM call → Response → Task inserted.

---

## 9. Error and State Feedback

### What

Consistent status signaling for loading, success, and failure.

### How

- **Loading**: Show spinner for synchronous LLM calls
- **Empty Queue**: "All caught up! Upload a new PDF to continue."
- **AI Unavailable**: Explicit error, no fallback
- **Queue State**: Always visible and queryable via SQLite
- **Quiz Generation Failed**: Explicit error state, user can retry
- **Max Rereads Reached**: Recommendation message, manual retry available

### Skip Semantics

Explicit terminal states preserve audit trail:

| Status | Meaning | Can Resurface |
|--------|---------|---------------|
| `COMPLETED` | Successfully finished | No |
| `SKIPPED` | User bypassed task | Yes (manual retry) |
| `FAILED` | Generation error | Yes (can retry) |

Skipped tasks are auditable and can resurface if needed. Do NOT silently mark skipped tasks as completed.

---

## 10. Module Boundaries (Strict)

### Reader Module
- Renders PDF pages
- Displays content from assigned page range
- StartPage is authoritative for opening context
- EndPage is informational only
- Trust-based completion (user signals when done)
- Reader only completes the reading task
- No orchestration logic
- No completion validation or gating
- No arbitrary routing to other modules

Generated follow-up QUIZ tasks may be activated immediately after Reader completion through the queue loop only.

### Quiz Module
- Displays quiz
- Returns score
- Drives mastery/remediation follow-up through queue results
- No orchestration logic

### Flashcard Module
- Renders cards
- Captures ratings (Again/Hard/Good/Easy)
- No orchestration logic

### Examiner Module
- Renders written assessments
- No orchestration logic

### SocraticRescue Module
- Renders source text preview + pre-engineered Socratic prompt
- Provides "Copy to Clipboard" for external LLM interaction
- "I've Completed the Session" calls `CompleteSocraticRescue(taskID)`
- Inserts fresh QUIZ task for re-quiz
- No orchestration logic

**Queue Router only**: fetch next pending task, mount correct module, mark complete, insert follow-up tasks.
