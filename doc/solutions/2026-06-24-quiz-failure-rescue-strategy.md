# Walkthrough — Quiz Failure Rescue Strategy

I have successfully implemented the user-configurable remediation strategy. Users can now toggle between the **Classic Track** (Reread first, Socratic rescue on second strike) and the **Fast Track** (Directly to Socratic AI Tutor on first failure).

## Changes

### 1. Database & Migrations
- Modified [schema.go](../../internal/db/schema.go) to add the `default_remedial_strategy TEXT DEFAULT 'CLASSIC'` column to the `user_settings` table.
- Added a startup migration block inside `InitSchema` that dynamically checks for and adds this column if it's missing (using SQLite `PRAGMA table_info`), ensuring compatibility for existing users.
- Updated `GetUserSettings` and `UpdateUserSettings` inside [store.go](../../internal/db/store.go) to query and update the strategy.
- Created `GetRemedialStrategy` and `SetRemedialStrategy` database helpers.

### 2. Backend Logic
- Added `DefaultRemedialStrategy` field to the `UserSettings` model in [models.go](../../internal/models/models.go).
- Updated `SubmitQuizAttempt` in [quiz_sync.go](../../internal/study/quiz_sync.go) to check the strategy preference *before* starting the SQLite transaction (to avoid database deadlocks).
- Implemented branching logic in `SubmitQuizAttempt` to directly transition failed quizzes into the Socratic Rescue Lane (generating `SOCRATIC_REMEDIAL` and deleting FSRS cards) if `FAST` track is selected.

### 3. Wails Bindings
- Updated `GetUserSettings` and `UpdateUserSettings` in [app_settings.go](../../app_settings.go) to expose and save `default_remedial_strategy`.
- Added the requested `GetRemedialStrategy()` and `SetRemedialStrategy(...)` bindings.

### 4. Frontend Settings UI
- Updated the API wrapper [appApi.js](../../frontend/src/services/appApi.js) to support the new parameter.
- Integrated `default_remedial_strategy` into [App.vue](../../frontend/src/App.vue) and [Dashboard.vue](../../frontend/src/pages/Dashboard.vue) to prevent profile change settings resets.
- Added the "Quiz Failure Rescue" section in General Settings on [Settings.vue](../../frontend/src/pages/Settings.vue) using premium-grade radio cards styled using system theme tokens:
  - `.strategy-option` card layout with hover and active animations.
  - Integration with existing light and dark theme colors (`var(--outline-variant)`, `var(--primary)`, `var(--surface-container-low)`, `var(--surface-container-high)`).

---

## Verification Results

### Automated Tests
I added a new test suite [remedial_strategy_test.go](../../remedial_strategy_test.go) and verified that all tests compile and pass cleanly:

```bash
go test ./...
```
Output:
```
ok  	ai-tutor	13.545s
ok  	ai-tutor/internal/db	(cached)
ok  	ai-tutor/internal/embeddings	(cached)
ok  	ai-tutor/internal/llm	(cached)
ok  	ai-tutor/internal/notebook	2.982s
ok  	ai-tutor/internal/scheduler	3.037s
```

All 3 new tests passed successfully:
1. `TestFastTrackSkipsReread`: Validates that with strategy `"FAST"`, failing a quiz inserts `SOCRATIC_REMEDIAL` and no `REREAD` task.
2. `TestClassicTrackInsertsReread`: Validates that with strategy `"CLASSIC"`, failing a quiz inserts a `REREAD` task.
3. `TestDefaultIsClassic`: Validates backward compatibility for fresh databases.
