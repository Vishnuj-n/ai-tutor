# Walkthrough — Profile Isolation, Auto-Assignment, and Scoping Fixes

All changes have been successfully implemented across the codebase in a single cohesive phase. Below is a detailed summary of the architectural and UI enhancements.

## Changes Made

### 1. Settings & Profile Normalization (`internal/db/store.go`)
- Added `GetDefaultProfileID()` to query the oldest profile created in the database.
- Updated `GetUserSettings()` to dynamically check if the saved `ActiveProfileID` exists in the database. If it is empty or invalid, the system automatically falls back to the default profile and updates the database record.

### 2. Auto-Assignment on Upload (`notebook_endpoints.go`)
- Updated `resolveExplicitActiveProfileID()` to leverage the active profile (or fallback default profile if none is explicitly selected) so that uploaded notebooks are auto-assigned automatically.

### 3. Profile-Scoped manual selection in Reader (`internal/db/notebooks_repo.go`, `notebook_endpoints.go`, `internal/db/store_integration_test.go`)
- Updated `db.GetNotebookTopicTree(profileID)` to accept `profileID` and filter notebooks by that profile (including unassigned notebooks).
- Updated `GetNotebookTopicTree` endpoint in `notebook_endpoints.go` to pass the resolved active profile ID.
- Updated integration tests in `internal/db/store_integration_test.go` to pass `""` and maintain compilation.

### 4. Scoped Reading Topic Scheduling (`internal/db/topics_repo.go`)
- Updated `QueryNextReadingTopic()` to check the active profile from settings and restrict suggested topics to active notebooks matching the active profile.

### 5. Scoped Due Cards Count (`internal/db/store.go`)
- Updated `QueryDueReviewCards(now)` to fetch the active profile and use a `LEFT JOIN` on `notebook_topics` and `notebooks` to only count due cards belonging to the active profile.

### 6. Scoped Review Scheduler (`internal/db/review_session_repo.go`)
- Updated `getNextDueReviewNotebookRepo(now)` to count due cards only for notebooks belonging to the active profile.

### 7. Scoped Dashboard Empty State (`app.go`)
- Updated the `activeNotebookCount` count query in `GetTodayPlan()` to filter by the active profile so that empty states are correctly shown per profile.

---

## Verification Plan

### Automated Tests
- Run Go backend tests:
  ```powershell
  go test ./...
  ```
  Result: All package tests passed successfully.
- Run frontend build verification:
  ```powershell
  cd frontend
  npm run build
  ```
  Result: Production Vite assets built with no compile errors.

### Manual Smoke Testing
1. Switch profiles via dashboard dropdown and verify Dashboard task list updates to match only notebooks in that profile.
2. Upload a new notebook while a profile is active — verify it automatically appears in the active profile without manual assignment.
3. Open the app directly to the notebooks page when no profile is active, upload a notebook, and verify it is automatically assigned to the default profile.
4. Verify that manual topic selection in the Reader only displays notebooks from the active profile.
