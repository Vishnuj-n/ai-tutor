# Code Quality Audit
**Date:** 2026-05-27  
**Scope:** All Go source files in `internal/`, root-level Go files, and frontend composables

---

## 1. Duplicate Query Logic — Two Functions Doing the Same Thing

### 1a. `fetchExistingReviewTask` vs `getExistingReviewTaskForNotebookRepo`
**File:** `internal/db/review_session_repo.go` lines 25 and 54

Both functions run the **identical SQL query** to find an existing FLASHCARD_REVIEW task for a notebook. The only difference is the receiver: one accepts a `querier` interface (for tx/conn polymorphism), the other calls `conn` directly.

`getExistingReviewTaskForNotebookRepo` also adds a `utils.LogReviewSessionResume` call that `fetchExistingReviewTask` does not — meaning the two are not truly equivalent and the logging is inconsistently applied.

**Fix:** Delete `getExistingReviewTaskForNotebookRepo`. Call `fetchExistingReviewTask(conn, notebookID)` directly from `createReviewSessionRepo` and `GetExistingReviewTaskForNotebook`. Move the `LogReviewSessionResume` call to the public wrapper in `store.go`.

---

### 1b. `getTaskForReviewTx` vs `getTaskByIDTxRepo`
**Files:** `internal/study/review_session.go` line 159 and `internal/db/review_session_repo.go` line 489

Both functions execute the **same SELECT on `study_queue` by ID** inside a transaction. `getTaskForReviewTx` lives in the `study` package and duplicates the DB-layer function.

**Fix:** Delete `getTaskForReviewTx` from `review_session.go`. Expose `db.GetTaskByIDTx` (wrapping `getTaskByIDTxRepo`) from `store.go` and call it from `RecordCardReview`.

---

### 1c. Duplicated token-budget constants in `reader.go`
**File:** `internal/study/reader.go` lines 281–284 and 329–332

`buildReaderCompletionQuizPrompt` and `buildReaderCompletionRetryPrompt` each declare their own identical set of local constants:

```go
const systemPromptTokens = 300
const outputStructureTokens = 800
const maxModelTokens = 4096
```

**Fix:** Hoist these three constants to package-level in `reader.go` (or `service.go`) and remove the duplicated declarations inside both functions.

---

## 2. Redundant / Orphaned Schema Column

### `notebooks.syllabus_draft_json` — column added via ALTER TABLE, not in CREATE TABLE
**File:** `internal/db/schema.go` lines 286–291

The `syllabus_draft_json` column is added via a backward-compat `ALTER TABLE` block but is **not present in the `CREATE TABLE IF NOT EXISTS notebooks` statement** above it. Every new database therefore goes through an unnecessary ALTER TABLE on every startup. The column is actively used (`notebooks_repo.go`, `notebook_endpoints.go`) so it is not dead, but it should be in the canonical `CREATE TABLE` definition.

**Fix:** Add `syllabus_draft_json TEXT` to the `notebooks` CREATE TABLE block. Keep the ALTER TABLE guard for existing databases (it is idempotent).

---

## 3. Dead / Unreachable Code

### 3a. `useReaderBase.js` — composable may be dead code
**File:** `frontend/src/composables/useReaderBase.js`

Flagged in `LEGACY_AUDIT.md` item #9 as potentially dead after `Reader.vue` was refactored. No other Vue file was found importing it in the current codebase scan.

**Action:** Verify with a grep for `useReaderBase` across all `.vue` and `.js` files. If no imports exist, delete the file.

---

### 3b. `createFlashcardsRepo` — never called from public API
**File:** `internal/db/flashcard_repo.go` line 9

`createFlashcardsRepo` is the internal implementation called by `store.go:CreateFlashcards`. However, `CreateFlashcards` itself is **never called** from any non-test file — all production paths use `GetOrCreateFlashcardsForTopic` instead. `CreateFlashcards` exists only in `store.go` as a public wrapper with no callers.

**Fix:** Either delete `CreateFlashcards` + `createFlashcardsRepo` (if confirmed unused outside tests), or add a `// Used by: ...` comment to document the intended caller.

---

## 4. Inconsistent Abstraction Layer Violation

### `review_session.go` queries the DB directly instead of going through `db` package
**File:** `internal/study/review_session.go` lines 55–60

`applyFlashcardReview` queries `fsrs_review_log` directly using `db.GetConnection().QueryRow(...)` to fetch `lastReviewedAt`. This bypasses the repository pattern — the `study` package is supposed to call `db.*` functions, not raw SQL.

```go
err = db.GetConnection().QueryRow(
    `SELECT COALESCE(MAX(reviewed_at), 0) FROM fsrs_review_log WHERE activity_type = 'flashcard' AND reference_id = ?`,
    cardID,
).Scan(&lastReviewedAt)
```

**Fix:** Add `db.GetLastFlashcardReviewedAt(cardID string) (int64, error)` and `db.GetLastFlashcardReviewedAtTx(tx, cardID)` to `store.go` / `flashcard_repo.go`, and call those from `applyFlashcardReview`.

---

## 5. Dual-Path Confusion in `GetTodayPlan`

**File:** `app.go:GetTodayPlan` (flagged as DRIFT #2 in `LEGACY_AUDIT.md`)

`GetTodayPlan` calls `scheduler.BuildTodayPlan(now)` and then immediately discards its `.Tasks` if queue rows exist. The scheduler result is only used for `DueReviewCards` / `ReviewMinutes`. This means `BuildTodayPlan` is called on every dashboard load but its primary output (the task list) is thrown away.

**Fix:** Either:
- Remove the `scheduler.BuildTodayPlan` call from `GetTodayPlan` and compute `DueReviewCards` / `ReviewMinutes` directly from the DB, or
- Make `BuildTodayPlan` the single source of truth and remove the separate `GetAllActiveTasks` / `GetAllPendingTasks` merge.

---

## 6. Socratic Tutor — No Task Lifecycle (Architecture Invariant Violation)

**File:** `frontend/src/pages/Socratic.vue`, `app.go:AskAI`  
(Flagged as LEGACY #23 in `LEGACY_AUDIT.md`)

Socratic sessions are completely invisible to the queue. The Socratic prompt is also constructed on the **frontend** (`buildSocraticQuestion`) and sent as a raw string — prompt logic belongs in the backend per AGENTS.md invariant #5.

**Fix (Sprint scope):**
1. Move `buildSocraticQuestion` logic to a new `app.go:AskSocratic` binding.
2. Add a dedicated `SOCRATIC` task type or at minimum log sessions to `study_queue` as ASSESSMENT tasks.

---

## 7. `topic_progress` Table — Defined but Minimally Used

**File:** `internal/db/schema.go` line 37

The `topic_progress` table (`learned_at`, `last_read_at`, `mastery_score`, `review_enabled`) is defined in the schema. A search of the codebase shows it is written to in very few places and `mastery_score` / `review_enabled` appear to be unused by any active query path.

**Action:** Audit all reads/writes to `topic_progress`. If `mastery_score` and `review_enabled` are never read, remove those columns or document their intended future use.

---

## 8. `parents` Table — `content_text` Column Semantics Unclear

**File:** `internal/db/schema.go` line 27, `internal/db/reader_repo.go`

The `parents` table has both `heading` and `content_text`. In `GetParentPassagesForTopicPageRange`, `content_text` is used as "Context" while `chunk_text` from `chunks` is used as "Content". However, in `GetTopicContent`, `content_text` is returned as the section body. The dual role of `content_text` (sometimes a summary/context, sometimes the full section body) is ambiguous and leads to redundant data in prompts.

**Action:** Document the intended semantic of `parents.content_text` vs `chunks.chunk_text` in a comment or in `doc/SCHEMA.md`.

---

## 9. `buildPageBoundedContext` Token Budget Applied Twice

**File:** `internal/study/service.go` lines ~370–410 and `internal/study/flashcard.go:buildMarathonFlashcardPromptWithBudget`

`buildPageBoundedContext` enforces a hard `maxContextTokens = 8000` budget and returns a trimmed chunk list. Then `buildMarathonFlashcardPromptWithBudget` applies a **second** token budget pass over the same already-trimmed list. The double-pass is redundant and the first budget constant (`8000`) is hardcoded with a comment "Use conservative default since we don't have model context here" — but the model context is available at the call site in `generateMarathonFlashcards`.

**Fix:** Remove the token budget enforcement from `buildPageBoundedContext` (make it a pure data fetch). Apply the single budget pass in `buildMarathonFlashcardPromptWithBudget` where the model limits are known.

---

## Priority Summary

| # | Severity | File(s) | Issue |
|---|----------|---------|-------|
| 1a | High | `review_session_repo.go` | Duplicate `fetchExistingReviewTask` / `getExistingReviewTaskForNotebookRepo` |
| 1b | High | `review_session.go`, `review_session_repo.go` | Duplicate `getTaskForReviewTx` / `getTaskByIDTxRepo` |
| 4 | High | `review_session.go` | Direct raw SQL in study layer — bypasses repo pattern |
| 1c | Medium | `reader.go` | Duplicated token constants in two functions |
| 2 | Medium | `schema.go` | `syllabus_draft_json` missing from CREATE TABLE |
| 5 | Medium | `app.go` | `BuildTodayPlan` called but result discarded |
| 9 | Medium | `service.go`, `flashcard.go` | Double token budget enforcement |
| 3a | Low | `useReaderBase.js` | Potentially dead composable |
| 3b | Low | `flashcard_repo.go`, `store.go` | `CreateFlashcards` has no production callers |
| 6 | Low (Sprint) | `Socratic.vue`, `app.go` | No task lifecycle; prompt on frontend |
| 7 | Low | `schema.go` | `topic_progress.mastery_score` / `review_enabled` unused |
| 8 | Low | `reader_repo.go` | `parents.content_text` semantics undocumented |
