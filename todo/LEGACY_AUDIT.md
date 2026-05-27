# Legacy Code Audit — Queue Pipeline Trace
**Date:** 2026-05-27  
**Method:** Traced each pipeline from Dashboard "Start" button → frontend → Wails bridge → backend service → DB

---

## How to Read This Report

Each section follows one pipeline end-to-end and flags:
- **LEGACY** — code that contradicts AGENTS.md invariants or uses deprecated patterns
- **DRIFT** — code that works but diverges from the canonical architecture
- **OK** — clean, correct

---

## Step 0 — Dashboard Queue Load

**Files:** `Dashboard.vue` → `appApi.js:getTodayPlan` → `app.go:GetTodayPlan` → `db/study_queue_repo.go`

### Trace

1. `onMounted` → `loadAgenda()` → `getTodayPlan()` (Wails bridge)
2. `GetTodayPlan` calls `scheduler.BuildTodayPlan(now)` then **also** calls `db.GetAllActiveTasks()` + `db.GetAllPendingTasks()` and merges them, overwriting the scheduler result if queue tasks exist.
3. `GetAllPendingTasks` / `GetAllActiveTasks` query `study_queue` with deterministic ORDER BY (task_type priority → notebook priority → task priority → created_at).
4. `queueTaskToScheduledTask` converts DB rows to `ScheduledTask` for the frontend.
5. Dashboard renders task cards; user clicks **Start** → `startTask(task)` → `router.push` to the correct page.

### Issues Found

| # | Location | Severity | Issue |
|---|----------|----------|-------|
| 1 | `Dashboard.vue` line ~25 | **RESOLVED** | Header terminology updated to `"Study Queue"` / `"Today's Tasks"`. |
| 2 | `app.go:GetTodayPlan` | **DRIFT** | Calls `scheduler.BuildTodayPlan` then immediately discards its `.Tasks` if queue rows exist. The scheduler result is only used for `DueReviewCards` / `ReviewMinutes`. This dual-path is confusing — the scheduler is effectively dead for task listing. |
| 3 | `app.go:GetTodayPlan` | **RESOLVED** | `plan_source` updated to remove deprecated terminology. |
| 4 | `Dashboard.vue` `startTask` | **DRIFT** | Falls back to `routePath = '/dashboard'` for unknown action types silently. Should surface an explicit error to the user per the "Return explicit errors" rule. |

---

## Step 1 — Reading Pipeline

**Files:** `Dashboard.vue:startTask` → `Reader.vue` → `appApi.js:initializeReadingSession` → `app.go:InitializeReadingSession` → `db/study_queue_repo.go:GetReadingTask` + `ActivateTask`

### Trace

1. `startTask` with `action === 'reading'` → `router.push('/reader', { query: { taskId, topicId, notebookId, startPage, endPage } })`
2. Reader mounts → calls `initializeReadingSession(taskID, notebookID, topicID, startPage, endPage)`
3. `InitializeReadingSession` in `app.go`:
   - If task missing → inserts a new READING row in `study_queue`
   - If task terminal (COMPLETED/FAILED/SKIPPED) → creates a new UUID task row (rematerialization)
   - Calls `db.ActivateTask(taskID)` → `UPDATE study_queue SET status='ACTIVE'`
   - Calls `db.GetReadingTask(taskID)` → joins `study_queue` + `reading_progress`
   - Calls `db.GetReaderTopicBundle(topicID, notebookID)`
4. User clicks "Mark as Learned" → `completeReading(taskID)` → `app.go:CompleteReading`
5. `CompleteReading`:
   - Validates task is ACTIVE
   - Calls `db.GetChunkIDsForTopicPageRange` → gets chunk IDs
   - Calls `studyService.GenerateQuizSync(topicID, chunkIDs, nil)` → LLM call → `QuizTaskPayload`
   - Calls `db.CompleteReadingWithGeneratedQuiz(taskID, quizPayload)` → transaction:
     - Updates `topics.current_page_cursor`
     - `CompleteTaskTx` → sets reading task to COMPLETED, inserts QUIZ follow-up with payload

### Issues Found

| # | Location | Severity | Issue |
|---|----------|----------|-------|
| 5 | `db/study_queue_repo.go:ValidateReadingCompletion` | **RESOLVED** | Deprecated helper removed; tests now use `PersistReadingProgress`. |
| 6 | `db/study_queue_repo.go:CompleteReading` | **RESOLVED** | Legacy completion helper removed; queue flow uses `CompleteReadingWithGeneratedQuiz`. |
| 7 | `app.go:CompleteReadingSession` | **RESOLVED** | Legacy pre-queue binding removed. |
| 8 | `app.go:GetNextTask` | **RESOLVED** | Legacy Wails binding removed. |
| 9 | `useReaderBase.js` | **DRIFT** | Composable exists but `Reader.vue` was recently refactored. Need to verify it is still used — if not, it is dead code. |

---

## Step 2 — Quiz Pipeline

**Files:** `Quiz.vue` → `appApi.js:submitQuizAttempt` → `app.go:SubmitQuizAttempt` → `study/quiz_sync.go:SubmitQuizAttempt` → `db/study_queue_repo.go:CompleteTaskTx`

### Trace

1. Dashboard routes to `/quiz?taskId=...` → Quiz.vue mounts, activates task via `activateTask(taskID)`
2. User answers questions → clicks Submit → `submitQuizAttempt(taskID, answers)`
3. `app.go:SubmitQuizAttempt` → `studyService.SubmitQuizAttempt(taskID, answers)`
4. `SubmitQuizAttempt` in `quiz_sync.go`:
   - Loads task, validates QUIZ + ACTIVE
   - Unmarshals `payload_json` → `QuizTaskPayload`
   - Scores answers, calculates pass/fail
   - If **passed**: resets reread attempt count, sets `flashcardsPending = true`
   - If **failed**: increments reread attempt count, inserts REREAD follow-up (up to 3 attempts)
   - Calls `db.CompleteTaskTx` → marks QUIZ COMPLETED, inserts REREAD follow-up if failed
   - Returns `QuizResult` with `FlashcardsPending: true` if passed
5. User sees result screen → clicks **Continue** → `generateFlashcardsForQuizTask(taskID)` → `app.go:GenerateFlashcardsForQuizTask`
6. `GenerateFlashcardsForQuizTask` → `studyService.GenerateFlashcardsAfterQuiz(notebookID, topicID, startPage, endPage)` → `generateMarathonFlashcards` → LLM → `db.GetOrCreateFlashcardsForTopic`
7. Frontend redirects to `/dashboard?flashcardsCreated=N`

### Issues Found

| # | Location | Severity | Issue |
|---|----------|----------|-------|
| 10 | `app.go:GenerateQuizForPageRange` | **LEGACY** | Exposed Wails binding for manual quiz generation by page range. Creates a **synthetic topic ID** (`quiz-manual-{notebookID}-p{start}-{end}`) and calls `GenerateQuizSync`. This synthetic topic is never linked to the queue. The result is returned raw to the frontend with no task lifecycle. This is a side-channel that bypasses the queue — violates invariant #1. |
| 11 | `app.go:GenerateQuizSync` | **LEGACY** | Exposed Wails binding that generates a quiz payload without any task context. Returns raw `quiz_task` payload. No queue row is created. Used by `Quiz.vue` in manual mode. Side-channel — violates invariant #1. |
| 12 | `quiz_sync.go:GenerateQuizSync` | **DRIFT** | The `chunkTextByID` parameter is `nil` when called from `CompleteReading`, causing a fallback DB lookup (`db.GetChunksForTopic`). This is a second DB round-trip that could be avoided by passing the already-fetched chunk text. |
| 13 | `quiz_sync.go:GenerateQuizSync` | **DRIFT** | Hardcodes `"Generate exactly 5 multiple-choice questions"` in the prompt string. Sprint 14 goal was density-scaled question count (`scaledQuizQuestionCount`). The scaling function exists in `service.go` but is **not used** in `GenerateQuizSync`. |
| 14 | `db/study_queue_repo.go:SaveQuizAttemptTx` | **DRIFT** | `quiz_attempts` table stores `answers_json` as a raw JSON blob. There is no per-question FSRS update during `SubmitQuizAttempt` — FSRS is only updated via `ScoreAnswer` (the per-question scoring path in `app.go`). The two scoring paths are inconsistent. |

---

## Step 3 — Flashcard Review Pipeline

**Files:** `Flashcards.vue` → `appApi.js:getReviewSession` → `app.go:GetReviewSession` → `db/review_session_repo.go:createReviewSessionRepo` → `db/flashcard_repo.go`

### Trace

1. Dashboard routes to `/flashcards?taskId=...` → Flashcards.vue mounts
2. If `taskId` is the synthetic `ReviewTaskDailyID` → `GetReviewSession` materializes a real session:
   - Calls `db.GetNextDueReviewNotebook(now)` to find notebook with most due cards
   - Calls `db.CreateReviewSession(notebookID)` → checks for existing PENDING/ACTIVE review task → if none, queries due cards → inserts `study_queue` row (FLASHCARD_REVIEW) + `review_task_cards` links
3. `studyService.GetReviewSession(taskID)` → `db.GetReviewSession` → loads task + all linked cards from `review_task_cards JOIN fsrs_cards`
4. User rates a card → `recordCardReview(taskID, cardID, rating)` → `app.go:RecordCardReview` → `studyService.RecordCardReview`:
   - Begins transaction
   - `db.MarkReviewTaskCardReviewedTx` → sets `review_task_cards.status = 'reviewed'`
   - `applyFlashcardReview` → `scheduler.NextFSRSState` → updates `fsrs_cards.state_json`, `due_at`, inserts `fsrs_review_log`
   - Returns remaining card count
5. User finishes all cards → `completeReviewSession(taskID)` → `db.CompleteReviewSession` → validates 0 remaining → sets task COMPLETED

### Issues Found

| # | Location | Severity | Issue |
|---|----------|----------|-------|
| 15 | `Flashcards.vue` | **KEEP** | Manual generation tabs are intentional. Comprehensive (page-range) and Semantic Discovery are allowed side-channels for manual study; they intentionally bypass the queue. |
| 16 | `app.go:RecordFlashcardReview` | **RESOLVED** | Standalone flashcard review binding removed; queue review uses `RecordCardReview`. |
| 17 | `app.go:GetFlashcards` | **RESOLVED** | Standalone flashcard fetch binding removed. |
| 18 | `app.go:GenerateReviewTasks` | **DRIFT** | Exposed Wails binding that manually triggers review task creation for a notebook. This is a manual side-trigger outside the queue's normal materialization path. Should only be needed for debugging. |
| 19 | `db/review_session_repo.go:createReviewSessionRepo` | **DRIFT** | Uses `cards[0].TopicID` as the `topic_id` for the FLASHCARD_REVIEW task. If the session spans multiple topics (multi-topic notebook), only the first card's topic is recorded. This is a data quality issue. |

---

## Step 4 — Socratic Tutor Pipeline

**Files:** `Socratic.vue` → `appApi.js:askAI` → `app.go:AskAI` → `rag/pipeline.go:ProcessQuery`

### Trace

1. User selects topic/notebook, types question → `submitQuestion()` → `askAIRequest(topicID, buildSocraticQuestion(question))`
2. `buildSocraticQuestion` prepends Socratic instructions to the user question (client-side prompt construction)
3. `app.go:AskAI` → `ragPipeline.ProcessQuery(topicID, question, 0, 0)` → vector retrieval + LLM
4. Returns `{ answer, cited_sections, chunks_retrieved, sections_used }`
5. No task lifecycle — Socratic is entirely stateless

### Issues Found

| # | Location | Severity | Issue |
|---|----------|----------|-------|
| 20 | `Socratic.vue:buildSocraticQuestion` | **DRIFT** | Socratic prompt instructions are constructed **on the frontend** and sent as part of the user question string. This means the system prompt is client-controlled and visible in network traffic. Prompt construction should happen in the backend. |
| 21 | `Socratic.vue` | **DRIFT** | Uses `askAI` (the generic RAG endpoint) rather than a dedicated Socratic endpoint. The Socratic mode is indistinguishable from a regular Ask AI call at the backend level — no separate logging, no separate retrieval tuning. |
| 22 | `app.go:AskAI` | **DRIFT** | `AskAI` is used for both the Reader "Ask AI" panel and the Socratic tutor. There is no routing differentiation. The `AskReaderAI` endpoint exists for the Reader (with scope/page params) but Socratic still uses the generic `AskAI`. |
| 23 | `Socratic.vue` | **LEGACY** | No task lifecycle at all. Socratic sessions are not recorded in `study_queue`. Per AGENTS.md, the Examiner creates ASSESSMENT tasks — Socratic should similarly create or link to a task for audit trail. Currently it is a completely invisible side-channel. |

---

## Summary Table

| # | File | Type | Description |
|---|------|------|-------------|
| 1 | `Dashboard.vue` | RESOLVED | Terminology updated to "Study Queue" / "Today's Tasks" |
| 2 | `app.go:GetTodayPlan` | DRIFT | Scheduler result discarded; dual-path confusing |
| 3 | `app.go:GetTodayPlan` | RESOLVED | `plan_source` renamed to non-deprecated value |
| 4 | `Dashboard.vue:startTask` | DRIFT | Silent fallback to dashboard for unknown action types |
| 5 | `db/study_queue_repo.go:ValidateReadingCompletion` | RESOLVED | Deprecated helper removed |
| 6 | `db/study_queue_repo.go:CompleteReading` | RESOLVED | Legacy completion helper removed |
| 7 | `app.go:CompleteReadingSession` | RESOLVED | Legacy Wails binding removed |
| 8 | `app.go:GetNextTask` | RESOLVED | Legacy Wails binding removed |
| 9 | `useReaderBase.js` | DRIFT | Composable may be dead code — needs verification |
| 10 | `app.go:GenerateQuizForPageRange` | KEEP | Manual quiz generation is intentionally allowed outside the queue. |
| 11 | `app.go:GenerateQuizSync` | KEEP | Manual quiz payload generation is intentionally allowed outside the queue. |
| 12 | `quiz_sync.go:GenerateQuizSync` | DRIFT | Unnecessary second DB round-trip for chunk text |
| 13 | `quiz_sync.go:GenerateQuizSync` | DRIFT | Hardcoded 5 questions — density scaling not wired in |
| 14 | `quiz_sync.go:SubmitQuizAttempt` | DRIFT | Per-question FSRS not updated during batch submission |
| 15 | `Flashcards.vue` | KEEP | Comprehensive + Semantic tabs are intentionally manual (no queue lifecycle) |
| 16 | `app.go:RecordFlashcardReview` | RESOLVED | Standalone review binding removed |
| 17 | `app.go:GetFlashcards` | RESOLVED | Standalone flashcard fetch removed |
| 18 | `app.go:GenerateReviewTasks` | DRIFT | Manual review task trigger outside normal queue path |
| 19 | `db/review_session_repo.go` | DRIFT | Multi-topic session uses only first card's topic_id |
| 20 | `Socratic.vue:buildSocraticQuestion` | DRIFT | Prompt construction on frontend, should be backend |
| 21 | `Socratic.vue` | DRIFT | Uses generic `askAI` not a dedicated Socratic endpoint |
| 22 | `app.go:AskAI` | DRIFT | No routing differentiation between Reader AI and Socratic |
| 23 | `Socratic.vue` | LEGACY | No task lifecycle — invisible side-channel |

---

## Priority Cleanup Order

### P0 — Queue Invariant Violations (fix before new features)
- (Resolved) Legacy standalone flashcard review/fetch removed

### P0A — Manual side-channels (explicitly kept)
- **#10, #11** — `GenerateQuizForPageRange` + `GenerateQuizSync` are allowed manual paths (no queue lifecycle)
- **#15** — `Flashcards.vue` Comprehensive/Semantic tabs are allowed manual paths (no queue lifecycle)

### P1 — Dead Code (safe to delete)
- (Resolved) Deprecated queue helpers removed

### P2 — Terminology / Naming
- (Resolved) Dashboard terminology and `plan_source` updated

### P3 — Architecture Drift (Sprint 14/15 scope)
- **#13** — Wire `scaledQuizQuestionCount` into `GenerateQuizSync` prompt
- **#20, #21, #22** — Move Socratic prompt to backend, add dedicated endpoint
- **#23** — Add task lifecycle to Socratic sessions
