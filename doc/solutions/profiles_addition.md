# Walkthrough — Active Shelf, Study Profiles, Settings, and Cloud Sync

All changes have been successfully implemented across the codebase in a single cohesive phase. Below is a detailed summary of the architectural and UI enhancements.

## Changes Made

### 1. Database Schema & Migration (`internal/db/schema.go`, `internal/db/store.go`)
- Added `study_profiles` table to support grouping of notebooks under a single exam deadline.
- Added `profile_id` and `study_status` columns to `notebooks` table.
- Added `active_profile_id`, `skip_to_reading_active`, `cloud_sync_url`, and `cloud_api_token` columns to `user_settings` table.
- Implemented robust start-up schema migrations that automatically check and add new columns to existing databases safely.

### 2. Active Shelf & Profile Repositories (`internal/db/notebooks_repo.go`, `internal/db/store.go`)
- Created CRUD functions for settings and study profiles.
- Implemented `AssignNotebookToProfile` and `UpdateNotebookStudyStatus` (enforcing a hard gate of maximum 4 active textbooks per profile).
- Added `AutoSwapCompletedNotebook` which automatically marks finished active books as completed and pulls the next dormant book from that profile into the active shelf.

### 3. Study Queue & Macro-Interleaving (`internal/db/study_queue_repo.go`)
- Rewrote `GetAllPendingTasks` using SQLite window functions (`ROW_NUMBER() OVER (PARTITION BY notebook_id ORDER BY ...)`) to retrieve exactly one task per active textbook (macro-interleaving).
- Integrated active profile filtering (when a profile is selected).
- Added FSRS review fadeout support: due flashcards for dormant books still show up in reviews, but new reading tasks are strictly gated to active books.
- Programmed the **Escape Hatch** sorting index: when enabled, it temporarily deprioritizes review tasks to the bottom of the queue.

### 4. Background Sync Service (`internal/study/sync.go`, `internal/runtime/boot.go`)
- Developed a periodic background synchronization loop (running every 15 minutes) that pushes local study logs to the cloud sync endpoint and downloads/registers new notebooks assigned by teachers.

### 5. Frontend UI & Onboarding Flow (`frontend/src/`)
- **Conditional Routing Guard (`router/index.js` & `App.vue`):** Enforces redirection to `/onboarding` for new users while hiding the sidebar for a distraction-free setup experience.
- **Onboarding Wizard (`Onboarding.vue`):** Walkthrough to setup daily minutes, the first study profile name, its deadline, and cloud sync URL.
- **Settings & Profiles Manager (`Settings.vue`):** Dedicated tabs to manage study profiles, textbook profile assignments, daily minutes, and manual cloud sync execution.
- **Enhanced Dashboard (`Dashboard.vue`):** Shows the current profile picker, active profile deadline telemetry widget, quick-toggle Escape Hatch button, and side-by-side active shelf and dormant textbook lists with sleep/activate controls.

---

## Verification Plan

### Automated Tests
- Integration tests in `internal/db/profile_test.go` cover creating profiles, updating user settings, active shelf limit checking, and cleanup of deleted profile references.
- All Go tests compile and pass successfully (`go test ./...` in the backend project root), using backward-compatible fallback paths when no active profile is selected.
- Frontend builds cleanly for production (`npm run build` in the `frontend` folder compiles Vite bundles successfully).

### Manual Smoke Testing
1. Re-launch the app via `wails dev`.
2. Clean database: verify the onboarding screen is shown first.
3. Complete onboarding: verify you are redirected to the dashboard.
4. Upload textbooks, assign to the profile, and verify that only up to 4 can be activated simultaneously on the smart shelf.
5. Toggle "Skip to Reading" and observe review tasks shifting down.
