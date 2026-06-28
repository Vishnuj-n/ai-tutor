# Solution: Dashboard Streak Calendar

## Overview

To improve study habits and encourage consistent learning, we added a compact Monthly Streak Calendar widget in the study dashboard sidebar. The calendar highlights the days when the user completes at least one guided study task (reading, quiz, socratic tutor, or review session) and tracks their current and longest learning streaks.

---

## Architectural Details

### 1. Database Layer (SQLite)
The SQLite `study_queue` table tracks the completion time of all tasks in UTC. We query these times via the following repository addition:

- **Method**: `GetCompletedTaskTimes() ([]time.Time, error)`
- **File**: [`internal/db/study_queue_repo.go`](file:///c:/Users/vishn/PROJECT/ai-tutor/internal/db/study_queue_repo.go)
- **Details**: It queries all rows where `status = 'COMPLETED'` and `completed_at` is set, parsing the strings using a multi-format timestamp parser (`parseSQLiteTimestamp`) to ensure robustness.

### 2. Timezone Alignment (Go Backend)
Since the database stores completion times in UTC, we must align these times with the user's local day boundaries (e.g., UTC+5:30 or UTC-5:00) before computing streaks.
- **Method**: `GetStreakState(timezoneOffsetMinutes int) map[string]interface{}`
- **File**: [`app_study.go`](file:///c:/Users/vishn/PROJECT/ai-tutor/app_study.go)
- **Algorithm**:
  1. Computes the user's local client time zone using the provided offset: `loc := time.FixedZone("ClientZone", -timezoneOffsetMinutes*60)`.
  2. Converts all UTC timestamps to local client dates in `YYYY-MM-DD` format and deduplicates them.
  3. Evaluates consecutive dates to calculate `current_streak` and `longest_streak`.
  4. Returns the metrics and a list of active dates.

### 3. Dashboard UI & Layout Optimization (Vue 3 / Vanilla CSS)
- **File**: [`frontend/src/pages/Dashboard.vue`](file:///c:/Users/vishn/PROJECT/ai-tutor/frontend/src/pages/Dashboard.vue)
- **Streak Calendar**:
  - Automatically fetches calendar data based on the client's current date and timezone offset.
  - Dynamically calculates the current month's layout, prepending blank days based on the 1st day of the week to align days correctly with the weekday headers.
  - Highlights active days using the primary container color and displays custom tooltip overlays detailing user activity on hover.
  - Features a glowing fire icon (`🔥`) that pulses when the user completes a task today.
- **Task Hierarchy Optimizations**:
  - **Flashcard Reviews Hero Card**: Partitioned the daily spaced repetition review task (`task-review-daily`) and rendered it as an explicit, high-priority dashboard widget showing the user's due count for the session alongside their remaining overdue deck size.
  - **Action Contexts**: Added clean "Continue Reading" titles and changed button actions from "Start" to "Resume" for active textbook readings.
  - **Telemetry Widget Relocation**: Pushed the large Profile Study Pacing telemetry widget to the bottom of the main column, giving focal priority to actionable daily study tasks.

---

## Testing Verification

### Existing Tests
Yes, unit tests were created to verify this implementation:
- **Test Case**: `TestGetCompletedTaskTimes`
- **File**: [`internal/db/study_queue_repo_test.go`](file:///c:/Users/vishn/PROJECT/ai-tutor/internal/db/study_queue_repo_test.go)
- **Verification**: Assures that inserting and completing tasks increments the list of completed timestamps correctly, and validates that timestamps are correctly parsed and stored close to the execution time.

### Commands to Run
To run the database unit tests:
```powershell
go test -v ./internal/db/... -run TestGetCompletedTaskTimes
```

To verify the entire project suite compiles and passes:
```powershell
go test ./...
```
