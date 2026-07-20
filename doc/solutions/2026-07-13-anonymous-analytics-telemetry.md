# Walkthrough: Anonymous Analytics Telemetry

This solution implements an anonymous usage telemetry pipeline using the existing cloud sync pipeline and a PostgreSQL RLS protected Supabase schema. It also includes critical frontend performance optimizations to prevent database locks and application lag.

## Implementation Details

### 1. Database Schema & Migration
- **[`internal/db/schema.go`](file:///c:/Users/vishn/PROJECT/ai-tutor/internal/db/schema.go)**:
  * Created the `analytics_events` SQLite table:
    ```sql
    CREATE TABLE IF NOT EXISTS analytics_events (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        event_type TEXT NOT NULL,
        file_hash TEXT DEFAULT '',
        page_number INTEGER DEFAULT 0,
        metadata TEXT DEFAULT '',
        synced BOOLEAN DEFAULT 0,
        created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
    )
    ```
  * Added the index `idx_analytics_events_synced` on the `synced` column.
  * Added the `analytics_enabled` column migration to `user_settings`.

### 2. Domain Models
- **[`internal/models/models.go`](file:///c:/Users/vishn/PROJECT/ai-tutor/internal/models/models.go)**:
  * Added `AnalyticsEnabled` field (`analytics_enabled` JSON key) to `UserSettings`.
  * Defined `AnalyticsEventSync` representation for sync payload transfers.

### 3. Repository Layer
- **[`internal/db/store.go`](file:///c:/Users/vishn/PROJECT/ai-tutor/internal/db/store.go)**:
  * Updated user settings configuration queries and update statements to persist `analytics_enabled`.
- **[`internal/db/analytics_repo.go`](file:///c:/Users/vishn/PROJECT/ai-tutor/internal/db/analytics_repo.go)**:
  * Implemented `TrackAnalyticsEvent(eventType, fileHash string, pageNumber int, metadata string)` to record telemetry (validates event types: `page_view`, `reading_complete`, `quiz_complete`, `flashcard_review`, and limits metadata size to 2KB).
  * Implemented `GetUnsyncedAnalyticsEvents()` to fetch unsynced records.
  * Implemented `MarkAnalyticsSynced(ids []int64)` to update sync status flags to 1 on success.
- **[`internal/db/study_queue_repo.go`](file:///c:/Users/vishn/PROJECT/ai-tutor/internal/db/study_queue_repo.go)**:
  * Extended `GetReadingTask` query to return the notebook's corresponding `file_hash`.

### 4. Synchronization Pipeline
- **[`internal/study/sync.go`](file:///c:/Users/vishn/PROJECT/ai-tutor/internal/study/sync.go)**:
  * Included analytics data within the regular `SyncPayload` payload if a custom sync URL is configured.
  * Implemented `syncAnalyticsFallback(...)`: if no cloud sync URL is configured but the user has consented to analytics, unsynced events are automatically batched and pushed to the Supabase research endpoint anonymously via HTTP POST.

### 5. Frontend Integration & Performance Optimizations

To prevent Wails IPC congestion and SQLite database locks during high-frequency events (like scrolling/paging), the frontend directly gates telemetry.

- **[`frontend/src/services/appApi.js`](file:///c:/Users/vishn/PROJECT/ai-tutor/frontend/src/services/appApi.js)**:
  * Bound the `TrackAnalyticsEvent` function to the Wails runtime.
- **[`Onboarding.vue`](file:///c:/Users/vishn/PROJECT/ai-tutor/frontend/src/pages/Onboarding.vue)** & **[`Settings.vue`](file:///c:/Users/vishn/PROJECT/ai-tutor/frontend/src/pages/Settings.vue)**:
  * Provided an opt-in checkbox enabling the user to control their anonymous data consent state (defaults to opt-out).
- **[`Reader.vue`](file:///c:/Users/vishn/PROJECT/ai-tutor/frontend/src/pages/Reader.vue)**:
  * Tracks `page_view` events (debounced to 1s) and `reading_complete` events.
  * *Optimization*: Loads settings on mount and checks `analyticsEnabled.value` in the frontend watcher before triggering Wails calls, preventing useless DB writes and IPC roundtrips when opted out.
  * *Cleanup*: Clears the active `trackPageDebounceId` timer on component `onUnmounted` to prevent timer leaks.
- **[`Quiz.vue`](file:///c:/Users/vishn/PROJECT/ai-tutor/frontend/src/pages/Quiz.vue)**:
  * Tracks `quiz_complete` results containing scores and passing outcomes.
  * *Optimization*: Only schedules events if settings confirm `analytics_enabled` is active.
- **[`Flashcards.vue`](file:///c:/Users/vishn/PROJECT/ai-tutor/frontend/src/pages/Flashcards.vue)**:
  * Tracks `flashcard_review` events including card ID and scoring rating.
  * *Optimization*: Only schedules events if settings confirm `analytics_enabled` is active.

### 6. Supabase Provisioning
- **[`supabase_analytics_schema.sql`](file:///c:/Users/vishn/PROJECT/ai-tutor/supabase_analytics_schema.sql)**:
  * Generated SQL definition to set up the remote Supabase PostgreSQL target table `anonymous_analytics_events`. RLS policies restrict users to write-only permissions (`INSERT` is permitted; `SELECT`/`UPDATE`/`DELETE` operations are blocked globally for security).

---

## Verification & Tests

- Unit tests written in `internal/db/analytics_repo_test.go` verifying the pipeline lifecycle (opt-out enforcement, recording events, retrieving batches, and marking sync).
- All unit and integration tests successfully verified using `go test ./...`.
