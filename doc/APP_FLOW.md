# AI Tutor App Flow

**Legacy note:** `blocks` → `chunks`. API still uses `block_id`. See `doc/SCHEMA.md`.

Queue-driven progression is deterministic. Manual entry points also supported. Both use SQLite as source of truth.

**Canonical flow:** Dashboard → Reader → Quiz → Dashboard

---

## 1. Queue Loop (Primary Flow)

```
Dashboard fetches next PENDING task
→ User clicks → status ACTIVE
→ Mount correct module
→ User completes/skips
→ Mark COMPLETED/SKIPPED/FAILED
→ Insert follow-up tasks
→ Repeat
```

**Multi-Notebook Priority:** Notebooks have `priority INTEGER DEFAULT 5` (1-10). Higher = more frequent. This is a deterministic bias, not adaptive scheduling.

**Queue Ordering:**

| Order | Task Type |
|-------|-----------|
| 1 | `FLASHCARD_GENERATE` |
| 2 | `SOCRATIC_REMEDIAL` |
| 3 | `FLASHCARD_REVIEW` |
| 4 | `REREAD` |
| 5 | `QUIZ` |
| 6 | `MILESTONE_EXAM` |
| 7 | `READING` |
| 8 | `EXAMINER` |

Then apply notebook priority bias within each tier.

**How:**
1. Dashboard queries `study_queue` for next PENDING task
2. User clicks → status ACTIVE, `activated_at` set
3. Router opens module with context
4. Module renders content from `task_type` + `block_id`
5. User completes → `CompleteTask(taskID, result)`
6. Backend marks status, inserts follow-ups
7. Dashboard refreshes

**Task Lifecycle:**
```
PENDING → ACTIVE → COMPLETED
           ↓
        SKIPPED / FAILED
```

Crash recovery: ACTIVE tasks > 30 min revert to PENDING on startup.

---

## 2. Ingestion Pipeline

PDF upload → Chapter selection → Sliding window chunking → READING tasks inserted

1. PDF Upload: user uploads, system extracts text
2. Chapter Selection: user reviews/prunes chapters
3. Sliding Window: 2500-word chunks, 200-word overlap, deterministic
4. READING tasks inserted (one per chunk)

---

## 3. Reading Flow

Reading completion → Backend generates QUIZ task

**Trust-based:** User decides when done. Complete Session button always enabled. No page-completion validation. No timers or tracking.

1. User clicks Complete Session
2. Frontend shows loading spinner
3. Backend calls LLM synchronously
4. Reading closes reading task only (not mastery signal)
5. Backend inserts + activates QUIZ task
6. Dashboard surfaces quiz as next task

**Quiz Generation States:** `GENERATING` → `READY` or `FAILED`

Reader does not route to Quiz. Only generated follow-up quiz tasks transition through queue.

---

## 4. Quiz Flow & Remediation

**Pass:**
```
QUIZ → COMPLETED
→ Insert FLASHCARD_REVIEW or move to next task
→ If 10th quiz for notebook: insert MILESTONE_EXAM
→ Dashboard shows next
```

**Fail (below threshold):**

Depends on `default_remedial_strategy`:

**Classic (default):**
```
QUIZ → COMPLETED → Insert REREAD task (if under max attempts)
→ Generate AI feedback → Dashboard shows REREAD
```

**Fast (direct rescue):**
```
QUIZ → COMPLETED → Skip REREAD, insert SOCRATIC_REMEDIAL directly
→ Delete FSRS flashcards → Dashboard shows Concept Rescue
```

User can complete or skip REREAD. No forced loops.

---

## 4a. Socratic Rescue (2-Strike)

**Strike 1 (quiz fail):**
```
QUIZ → COMPLETED → Insert REREAD (if reread_attempt ≤ max)
→ Dashboard shows REREAD
```

**Strike 2 (quiz fail again):**
```
QUIZ → COMPLETED → Insert SOCRATIC_REMEDIAL (blocks queue)
→ Delete FSRS flashcards → Dashboard shows Concept Rescue
```

**Rescue session:**
1. Student opens SocraticRescue page → source text + Socratic prompt
2. Option A: In-app Socratic tutor (interactive chat)
3. Option B: Copy prompt to external LLM
4. Clicks "I've Completed the Session"
5. SOCRATIC_REMEDIAL → COMPLETED, fresh QUIZ task inserted

**Re-quiz:** Pass → flashcards generated, topic mastered. Fail → `external_help_required` flag set, queue unblocks.

Key behaviors: SOCRATIC_REMEDIAL blocks queue, single rescue cycle only, no flashcards for failed concepts, `external_help_required` prevents further rescue.

---

## 4b. FLASHCARD_GENERATE (Cloud Sync Recovery)

On sync failure after retries → `FLASHCARD_GENERATE` task inserted (priority tier 7). Resolved when sync succeeds. Prevents data loss.

---

## 5. Flashcards & FSRS

**FSRS = scheduling algorithm only** for long-term retention. Quizzes do NOT update FSRS state.

**One `FLASHCARD_REVIEW` task = one review session per block** (not per flashcard). Prevents queue explosion.

1. Reviews become due (per FSRS) → `FLASHCARD_REVIEW` task inserted
2. Dashboard fetches review task
3. User reviews all due cards in block
4. FSRS calculates next interval
5. New task scheduled for future due date

---

## 6. Milestone Exam

Aggregate exam from past quiz attempts for a notebook. Triggered every 10 completed quizzes per notebook.

**Flow:**
1. After quiz completion, backend counts completed quizzes for the notebook
2. If count is a multiple of 10 (`count % 10 == 0`) → `MILESTONE_EXAM` task inserted
3. Milestone exam reuses questions from last 10 quiz attempts (no new LLM generation)
4. User completes the aggregate exam → score computed from embedded correctness arrays
5. Pass → topic/notebook progression. Fail → standard remediation flow.

**Key behaviors:**
- Reuses existing quiz questions (no LLM call for generation)
- Deduplicates: prevents duplicate milestone exams for same quiz attempt
- Priority tier 2 (between QUIZ at tier 3 and READING at tier 1)
- Skips corrupt quiz attempts gracefully

---

## 6a. Examiner Mode

Optional written assessments. Triggered after mastery (e.g., quiz > 80%). Lowest queue priority (tier 0). Reviews/reading not blocked. User-triggered, not hidden.

---

## 6b. Dashboard Streak Calendar

Monthly calendar widget tracks study consistency. Timezone-aware, highlights active days, shows current/longest streak, fire icon pulses on today's completion.

---

## 6c. Cloud Sync

Stable SHA-256 file hashes + page numbers replace local IDs for cross-student analytics. Delta sync (only unsent logs). `classroom_code` for teacher-student association.

---

## 7. Navigation

Left sidebar: Dashboard, Reader, Notebooks, Quiz, Flashcards, Examiner, Tutor, Settings, Sync. Dashboard queue is primary workflow.

---

## 8. Synchronous Generation

All AI generation sync. Click Complete → Spinner → LLM call → Response → Task inserted.

---

## 9. Error and State Feedback

- Loading: spinner for LLM calls
- Empty queue: "All caught up! Upload a new PDF."
- AI unavailable: explicit error, no fallback
- Queue state: always visible via SQLite
- AI cleanup failed: graceful fallback to bookmark chapters or single "General" chapter
- Quiz gen failed: explicit error, retry available

**Skip semantics:**
| Status | Can Resurface |
|--------|---------------|
| COMPLETED | No |
| SKIPPED | Yes (manual retry) |
| FAILED | Yes (can retry) |

---

## 10. Module Boundaries

**Reader:** Renders PDF, trust-based completion, no orchestration, no completion gating.

**Quiz:** Displays quiz, scores, drives follow-up through queue results, no orchestration.

**Flashcards:** Renders cards, captures ratings, no orchestration.

**Examiner:** Renders assessments, no orchestration.

**SocraticRescue:** Source text preview + prompt, "Copy to Clipboard", "I've Completed" → `CompleteSocraticRescue(taskID)`, inserts QUIZ.

**Queue Router only:** fetch next task, mount module, mark complete, insert follow-ups.
