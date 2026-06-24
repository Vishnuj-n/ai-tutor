# Walkthrough — Study Schedule Reminders & Max Flashcard Capping

We have successfully renamed `daily_study_minutes` to `max_flashcards_per_session` globally (across schema, models, backend logic, and frontend components) and implemented the Intelligent Scheduler Flow in `App.vue` using exact-time `setTimeout` delays.

---

## Changes Implemented

### 1. Database & Model Modernization
- **[internal/db/schema.go](internal/db/schema.go)**: 
  - Updated `CREATE TABLE user_settings` to drop `daily_study_minutes` and define:
    - `max_flashcards_per_session INTEGER NOT NULL DEFAULT 30`
    - `study_start_time TEXT DEFAULT '17:00'`
    - `study_end_time TEXT DEFAULT '18:00'`
    - `reminders_enabled BOOLEAN DEFAULT 1`
  - Updated default insert values on database creation.
- **[internal/models/models.go](internal/models/models.go)**: Refactored `UserSettings` struct fields to match the database properties.
- **[internal/db/store.go](internal/db/store.go)**: Refactored `GetUserSettings` and `UpdateUserSettings` to retrieve/save the new columns. Deleted the legacy `GetDailyStudyMinutes` and `UpsertDailyStudyMinutes` methods.

### 2. Business Logic & Capping
- **[internal/scheduler/service.go](internal/scheduler/service.go)**: Updated the scheduler to cap flashcard reviews directly by `max_flashcards_per_session` instead of time ratios. Added time/duration parsing helpers (`calculateDurationMinutes`). Always sets the reading budget to a clean 2500-word target limit.
- **[app_study.go](app_study.go)**: Simplified the queue aggregation today-plan review card limits to cap by `max_flashcards_per_session` directly.

### 3. Wails Bridge & Frontend Settings
- **[app_settings.go](app_settings.go)** & **[frontend/src/services/appApi.js](frontend/src/services/appApi.js)**: Cleaned up deprecated wails settings endpoints and updated settings parameters.
- **[frontend/src/pages/Onboarding.vue](frontend/src/pages/Onboarding.vue)** & **[frontend/src/pages/Settings.vue](frontend/src/pages/Settings.vue)**: Replaced the old "Daily study goal" inputs with "Max Flashcards per Session", "Study Start Time", "Study End Time" time pickers, and "Enable Study Reminders" checkboxes.

### 4. Intelligent Scheduler Flow
- **[frontend/src/App.vue](frontend/src/App.vue)**: 
  - Added targeted `setTimeout` event scheduler (`syncScheduler`) that triggers alerts only at the exact moments start/end times are hit.
  - Implemented desktop toast notifications (native browser Notification API) and top-bar overlay banner alerts.
  - End-of-study event checks daily queue tasks: if any tasks remain unfinished, the banner shows quick-action buttons to extend the study window by `+15` or `+30` minutes, instantly updating settings and re-syncing the scheduler.

---

## Verification Results

### Automated Tests
All repository and scheduler unit tests pass successfully.
- `go test -v ./internal/db/...` -> **PASS**
- `go test -v ./internal/scheduler/...` -> **PASS**
