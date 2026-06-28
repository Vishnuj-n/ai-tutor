# Cloud Sync — Local Testing Guide

## Overview

The app syncs study data (notebooks + FSRS review logs) to a cloud server via HTTP POST. This doc explains how to verify the sync pipeline works end-to-end in a local environment.

**Key files:**
- `internal/study/sync.go` — sync logic, payload construction, retry loop
- `internal/db/fsrs_review_log_repo.go` — delta query (`GetReviewLogsSinceWithFileInfo`)
- `internal/models/models.go` — `SyncLogEntry` struct with `filename`/`page_number`
- `internal/db/store.go` — `SetLastSyncedAt`, user settings read/write
- `app_settings.go` — Wails binding `TriggerCloudSync()`
- `frontend/src/pages/Settings.vue` — "Sync with Cloud Now" button (dev-only URL field)

---

## How Sync Works

1. **Trigger**: Background loop (every 15 min + on startup) OR manual click "Sync with Cloud Now" in Settings.
2. **Payload**: POST to `cloud_sync_url` with JSON body:

```json
{
  "user_token": "<api_token>",
  "classroom_code": "<code>",
  "notebooks": [
    { "filename": "document.pdf", "title": "My Notebook", "study_status": "uploaded" }
  ],
  "logs": [
    {
      "id": "log-uuid",
      "file_hash": "a1b2c3d4e5f6...",
      "page_number": 5,
      "activity_type": "flashcard",
      "reference_id": "card-uuid",
      "reviewed_at": 1719500000,
      "rating": 3,
      "scheduled_days": 4,
      "state_before_json": "{...}",
      "state_after_json": "{...}"
    }
  ]
}
```

> **Note**: `file_hash` is the SHA-256 of the notebook file, computed at upload time and stored in `notebooks.file_hash`. `page_number` comes from the flashcard's source chunk. These provide stable, cross-install identifiers for the cloud dashboard.

3. **Delta sync**: Only logs with `reviewed_at > last_synced_at` are sent. After success, `last_synced_at` advances to current time.
4. **Retry**: 3 attempts, 1s delay between retries. On failure, a `FLASHCARD_SYNC` task is inserted into the queue.
5. **Response**: Server returns `new_notebooks` array. Each entry triggers a download + registration of a teacher-assigned PDF.

---

## Method 1: Mock Server (Recommended)

Spin up a local HTTP server that accepts the sync POST and logs the payload.

### Node.js mock server

```js
// sync-mock-server.js
const http = require('http');

const server = http.createServer((req, res) => {
  if (req.method === 'POST') {
    let body = '';
    req.on('data', chunk => { body += chunk; });
    req.on('end', () => {
      console.log('\n=== SYNC REQUEST ===');
      console.log('Headers:', JSON.stringify(req.headers, null, 2));
      const payload = JSON.parse(body);
      console.log('User token:', payload.user_token);
      console.log('Classroom code:', payload.classroom_code);
      console.log('Notebooks:', payload.notebooks.length);
      payload.notebooks.forEach(nb => {
        console.log(`  - ${nb.filename} (${nb.title}) [${nb.study_status}]`);
      });
      console.log('Review logs:', payload.logs.length);
      payload.logs.forEach(log => {
        console.log(`  - ${log.activity_type} rating=${log.rating} @ ${new Date(log.reviewed_at * 1000).toISOString()}`);
      });
      console.log('====================\n');

      // Return empty assignments (or add new_notebooks for testing download)
      res.writeHead(200, { 'Content-Type': 'application/json' });
      res.end(JSON.stringify({ new_notebooks: [] }));
    });
  } else {
    res.writeHead(405);
    res.end('Method not allowed');
  }
});

server.listen(3099, () => {
  console.log('Sync mock server running at http://localhost:3099');
  console.log('Set this as your Sync Server URL in Settings (dev mode)');
});
```

Run it:

```bash
node sync-mock-server.js
```

### Configure the app

1. Open Settings in the app (dev mode required for URL field visibility).
2. Set **Sync Server URL** to `http://localhost:3099`.
3. Set **API Token** to any value (e.g. `test-token-123`).
4. Set **Classroom Code** if needed.
5. Click **"Sync with Cloud Now"**.

### What to look for in the mock server output

- `Notebooks: N` — confirms notebook metadata is sent (filename, title, status).
- `Review logs: N` — confirms delta logs are sent. First sync sends all logs (since `last_synced_at` starts at 0). Subsequent syncs only send new logs.
- `User token` / `Classroom code` — confirms credentials are included.

---

## Method 2: curl (Direct Payload Inspection)

If the app is running and you know the sync URL, you can manually POST a test payload to verify the server accepts the format:

```bash
curl -X POST http://localhost:3099 \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer test-token" \
  -d '{
    "user_token": "test-token",
    "classroom_code": "CLS101",
    "notebooks": [
      {"filename": "test.pdf", "title": "Test Notebook", "study_status": "uploaded"}
    ],
    "logs": [
      {
        "id": "test-log-1",
        "topic_id": "topic-1",
        "activity_type": "FLASHCARD",
        "reference_id": "card-1",
        "reviewed_at": 1719500000,
        "rating": 3,
        "scheduled_days": 4,
        "state_before_json": "{}",
        "state_after_json": "{}"
      }
    ]
  }'
```

This bypasses the app and tests your server's endpoint directly.

---

## Method 3: Environment Variables (Without UI)

The app falls back to env vars if SQLite settings are empty:

```bash
# PowerShell
$env:CLOUD_SYNC_URL = "http://localhost:3099"
$env:CLOUD_API_TOKEN = "test-token-123"

# Then start the app — the background sync loop will use these
wails dev
```

This skips needing to configure Settings in the UI.

---

## Method 4: Go Unit Test (Automated)

The existing test in `app_test.go:623` (`TestTriggerCloudSyncRetriesAndFailSafe`) demonstrates the pattern using `httptest.NewServer`:

```go
server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
    // Inspect r.Body, return mock response
    var payload study.SyncPayload
    json.NewDecoder(r.Body).Decode(&payload)
    // Assert payload.Notebooks, payload.Logs, etc.

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(map[string]interface{}{
        "new_notebooks": []interface{}{},
    })
}))
defer server.Close()

// Point app settings to mock server
testRepo.ExecForTest(`UPDATE user_settings SET cloud_sync_url = ?, cloud_api_token = 'token' WHERE id = 1`, server.URL)

err := study.TriggerCloudSync(testRepo)
```

Run with:

```bash
go test -run TestTriggerCloudSync -v ./...
```

---

## Testing Delta Sync Behavior

To verify only new logs are sent on subsequent syncs:

1. Start mock server, run first sync. Note `Review logs: N` in output.
2. Complete a flashcard review in the app (creates a new FSRS review log).
3. Run sync again. Output should show `Review logs: 1` (only the new log).
4. Run sync again without new reviews. Output should show `Review logs: 0`.

The `last_synced_at` timestamp in `user_settings` tracks the cursor. You can inspect it:

```sql
SELECT last_synced_at FROM user_settings WHERE id = 1;
```

Or reset it to force a full re-sync:

```sql
UPDATE user_settings SET last_synced_at = 0 WHERE id = 1;
```

---

## Testing Teacher Assignment Download

To test the `new_notebooks` download path, have your mock server return assigned notebooks:

```js
res.end(JSON.stringify({
  new_notebooks: [
    {
      id: "assigned-001",
      title: "Assigned Chapter 5",
      download_url: "http://localhost:3099/downloads/ch5.pdf"
    }
  ]
}));
```

Serve the PDF at that URL. The app will download it to `%APPDATA%/ai-tutor/notebooks/assigned-001.pdf` and register it in SQLite.

---

## Troubleshooting

| Symptom | Cause | Fix |
|---------|-------|-----|
| "Sync Server URL" field not visible | Not in dev mode | Run `wails dev`, not `wails build` |
| Sync silently does nothing | `cloud_sync_url` is empty | Set URL in Settings or via env var |
| `Review logs: 0` on first sync | `last_synced_at` already advanced | Reset: `UPDATE user_settings SET last_synced_at = 0 WHERE id = 1` |
| Sync fails with network error | Server not running / wrong port | Verify mock server is running, check URL |
| `FLASHCARD_SYNC` task appears in queue | Sync failed 3 times | Fix server, next sync attempt will resolve it |
