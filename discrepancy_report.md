# Doc-Code Discrepancy Report

## 1. Executive Summary

The audit reveals a **major divergence** in the deterministic queue progression rules. The frontend scheduling behavior and query structures currently bypass the documented architectural invariants. Overall database schemas and API bindings are generally well-aligned with the specs. The alignment percentage is approximately 80%, but the violations found constitute high-risk drift because they directly compromise the system's foundational principle ("Data, Not Engines").

## 2. Discrepancy Breakdown Table

| Component / Module | Documented Spec | Actual Code Implementation | Severity | Affected Files |
|---|---|---|---|---|
| **Task Queue & Priority Hierarchy** | Deterministic strict priority: `FLASHCARD_GENERATE > SOCRATIC_REMEDIAL > FLASHCARD_REVIEW > REREAD > QUIZ > MILESTONE_EXAM > READING > EXAMINER` | `getPendingTasksWithProfile` and `getNextTaskWithProfile` override this deterministic priority hierarchy based on `skipVal` (via frontend settings `skip_to_reading_active`). The logic rewrites priority (`CASE WHEN ? = 1 THEN...`) which is undocumented and violates queue priority rules. | **Critical** | `internal/db/study_queue_repo.go` |
| **Database Schema & Data Models** | Matches `doc/SCHEMA.md`. | Mostly aligned. A new table `analytics_events` and some columns (e.g. `analytics_enabled`, `target_session_words`, `student_username` on `user_settings`) were found in `schema.go` that do not exist in `doc/SCHEMA.md`. | **Minor** | `internal/db/schema.go`, `doc/SCHEMA.md` |
| **Wails IPC API Contracts** | Matches `doc/DATA_API.md`. | Aligned. Wails APIs match documentation. | **None** | - |
| **Task Lifecycle & Canonical Flow** | `Dashboard -> Reader -> Quiz -> Dashboard` state progression. | Correctly implemented via SQLite state writes. | **None** | - |

## 3. Architecture Invariant Violations

The codebase contains several forbidden patterns violating the rules explicitly listed in `AGENTS.md` and `doc/ARCHITECTURE.md`.

- ❌ **Background Schedulers / Event Driven Flows**: The frontend Vue app uses a background scheduling poll `setTimeout` loop in `App.vue` (`schedulerTimeout` handling `study_start_time` and `study_end_time`) to trigger events asynchronously. This violates the "No hidden orchestration state / No background schedulers" architectural invariants.
- ❌ **Engagement Surveillance (Scroll Tracking)**: The `Reader.vue` component implements scroll event listeners (`handleViewportScroll`), debounce logic (`scrollDebounceId`), programmatic scroll timeouts (`programmaticScrollTimeoutId`), and explicitly tracks `currentVisiblePage` mapping to `reader.updateCurrentPage(page)`. This is a direct violation of the strict prohibition against "Engagement surveillance (timers, scroll tracking)" and "Behavioral tracking beyond completion validation".

## 4. Actionable Recommendations

**To re-establish 1:1 alignment, the codebase should be updated to strictly adhere to the specs:**

1. **Remove Dynamic Task Priorities (`internal/db/study_queue_repo.go`)**:
   - Delete the `CASE WHEN ? = 1` conditional block from all queue fetching logic.
   - Ensure only the single documented deterministic `ORDER BY` hierarchy is used.

2. **Remove the App.vue Background Scheduler**:
   - Remove `schedulerTimeout` and background notification dispatching logic in `App.vue`. Rely entirely on the backend queue controller.

3. **Remove Reader.vue Surveillance Logic**:
   - Remove `scroll` event listeners and `scrollDebounceId` from the Reader page.
   - Fall back to the "trust-based" completion model as specified in the docs.

4. **Update `doc/SCHEMA.md`**:
   - Add the `analytics_events` table and the new columns in the `user_settings` table to keep documentation current with the database schema state.
