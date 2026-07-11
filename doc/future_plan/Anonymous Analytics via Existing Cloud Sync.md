# Anonymous Analytics via Existing Cloud Sync

## Context

User wants anonymous usage data collection (file hash + page number identification) with a consent toggle during onboarding. The existing cloud sync infrastructure (`internal/study/sync.go`) already handles `SyncLogEntry` with `FileHash` and `PageNumber` — we piggyback on that.

## Approach

Add a local `analytics_events` table, a consent toggle in `user_settings`, and extend the existing `SyncPayload` to include analytics events. No new API endpoints, no new transport logic.

---

## Changes

### 1. Schema: Add analytics table + consent column

**File:** `internal/db/schema.go`

- Add `analytics_events` table to `InitSchema`:
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

- Add `analytics_enabled` to `alterStatements`:
  ```sql
  ALTER TABLE user_settings ADD COLUMN analytics_enabled BOOLEAN DEFAULT 0
  ```

- Add index: `idx_analytics_events_synced ON analytics_events(synced)`

### 2. Model: Add field to UserSettings

**File:** `internal/models/models.go`

- Add `AnalyticsEnabled bool` to `UserSettings` struct

### 3. Repository: Read/write settings + analytics events

**File:** `internal/db/store.go`

- Update `GetUserSettings` SELECT to include `analytics_enabled`
- Update `UpdateUserSettings` INSERT/UPDATE to include `analytics_enabled`
- Update default fallback to set `AnalyticsEnabled: false`

### 4. Analytics repository

**File:** `internal/db/analytics_repo.go` (new)

```go
type AnalyticsEvent struct {
    ID         int64
    EventType  string
    FileHash   string
    PageNumber int
    Metadata   string
    Synced     bool
    CreatedAt  time.Time
}

func (r *Repository) TrackAnalyticsEvent(eventType, fileHash string, pageNumber int, metadata string) error
func (r *Repository) GetUnsyncedAnalyticsEvents() ([]AnalyticsEvent, error)
func (r *Repository) MarkAnalyticsSynced(ids []int64) error
```

### 5. Extend SyncPayload

**File:** `internal/models/models.go`

- Add `AnalyticsEventSync` struct (slim, no local IDs):
  ```go
  type AnalyticsEventSync struct {
      EventType  string `json:"event_type"`
      FileHash   string `json:"file_hash"`
      PageNumber int    `json:"page_number"`
      Metadata   string `json:"metadata"`
      CreatedAt  int64  `json:"created_at"`
  }
  ```

**File:** `internal/study/sync.go`

- Add `Analytics []models.AnalyticsEventSync` to `SyncPayload`
- In `TriggerCloudSync`: if `settings.AnalyticsEnabled`, fetch unsynced events, add to payload, mark synced on success

### 6. Wails binding

**File:** `app_settings.go`

- Add `TrackAnalyticsEvent(eventType, fileHash string, pageNumber int, metadata string) map[string]interface{}` method
- Checks `analytics_enabled` from user settings before writing
- Update `UpdateUserSettings` signature to include `analyticsEnabled bool` parameter (add as last param)

### 7. Frontend API bridge

**File:** `frontend/src/services/appApi.js`

- Add `trackAnalyticsEvent(eventType, fileHash, pageNumber, metadata)`

### 8. Onboarding consent

**File:** `frontend/src/pages/Onboarding.vue`

- Add a consent section in Step 1 (after reminders checkbox) or as a new Step 3.5
- Simple checkbox: "Help improve the app by sharing anonymous usage data"
- Pass `analyticsEnabled` to `updateUserSettings`

**File:** `frontend/src/services/appApi.js`

- Update `updateUserSettings` signature to accept `analyticsEnabled`

### 9. Tracking call sites

**File:** `frontend/src/pages/Reader.vue`

- Track `page_view` when user navigates pages (debounced, not every scroll)

**File:** `frontend/src/pages/Quiz.vue`

- Track `quiz_complete` after submission with score in metadata

**File:** `frontend/src/pages/Flashcards.vue`

- Track `flashcard_review` after rating a card

**File:** `frontend/src/pages/Reader.vue`

- Track `reading_complete` when user completes reading task

---

## Files to modify

| File | Change |
|------|--------|
| `internal/db/schema.go` | Add table + alter statement |
| `internal/models/models.go` | Add `AnalyticsEnabled` to UserSettings, add `AnalyticsEventSync` |
| `internal/db/store.go` | Update settings read/write |
| `internal/db/analytics_repo.go` | **New** — Track, GetUnsynced, MarkSynced |
| `internal/study/sync.go` | Extend SyncPayload, include events in sync |
| `app_settings.go` | Add Wails binding, update UpdateUserSettings |
| `frontend/src/services/appApi.js` | Add trackAnalyticsEvent, update updateUserSettings |
| `frontend/src/pages/Onboarding.vue` | Add consent checkbox |
| `frontend/src/pages/Reader.vue` | Track page_view, reading_complete |
| `frontend/src/pages/Quiz.vue` | Track quiz_complete |
| `frontend/src/pages/Flashcards.vue` | Track flashcard_review |
| `doc/SCHEMA.md` | Document new table |

---

## What NOT to do

- No new API endpoints
- No new transport/auth logic
- No external analytics services
- No real-time event streaming
- No complex batching (SQLite writes are fast enough)

---

## Verification

1. `go test ./...` passes
2. `wails dev` loads onboarding with consent checkbox
3. Toggle consent ON → events appear in `analytics_events` table
4. Trigger cloud sync → events included in payload, marked synced
5. Toggle consent OFF → no events written


SUGGESTION FROM AI WHO READ YOUR PLAN : 

Option B: Fallback in the Sync Logic (Code Level)
In internal/study/sync.go, modify the HTTP transport logic:

Go
syncURL := settings.CloudSyncURL
syncToken := settings.CloudAPIToken

// If the user hasn't set a custom URL, but opted into analytics, route to the research server
if syncURL == "" && settings.AnalyticsEnabled {
    syncURL = "https://your-research-supabase.supabase.co/rest/v1/rpc/handle_cloud_sync"
    syncToken = "YOUR_ANON_KEY"
}

// If it's still empty, abort sync
if syncURL == "" {
    return nil 
}
Option B is highly recommended. It keeps your database clean, ensures that a user's personal settings aren't polluted with backend URLs, and routes the data dynamically exactly when it is needed.

Are you planning to use Option B in sync.go, or would you prefer to inject the research endpoint directly into the database row during onboarding?