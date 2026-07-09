# Cloud Sync — Local Testing Guide

## Overview

App syncs study data (notebooks + FSRS review logs) to cloud via HTTP POST.

**Key files:**
- `internal/study/sync.go` — sync logic, payload, retry
- `internal/db/fsrs_review_log_repo.go` — delta query
- `internal/models/models.go` — `SyncLogEntry` struct
- `internal/db/store.go` — `SetLastSyncedAt`
- `app_settings.go` — `TriggerCloudSync()` binding
- `frontend/src/pages/Settings.vue` — "Sync with Cloud Now" button

---

## How Sync Works

1. **Trigger:** Background loop (15 min + on startup) OR manual "Sync with Cloud Now"
2. **Payload:** POST to `cloud_sync_url` with JSON:

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

> `file_hash` = SHA-256 of notebook file. `page_number` = from flashcard's source chunk. Both are stable, cross-install identifiers.

3. **Delta sync:** Only logs with `reviewed_at > last_synced_at` sent. After success, `last_synced_at` advances.
4. **Retry:** 3 attempts, 1s delay. On failure → `FLASHCARD_GENERATE` task inserted.
5. **Response:** Server returns `new_notebooks` array for teacher-assigned PDF downloads.

---

## Method 1: Mock Server (Recommended)

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
});
```

Run: `node sync-mock-server.js`

**Configure:** Settings → Sync Server URL = `http://localhost:3099`, API Token = any value. Click "Sync with Cloud Now".

**Check:** `Notebooks: N` confirms metadata sent. `Review logs: N` confirms delta logs.

---

## Method 2: curl

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

---

## Method 3: Environment Variables

```bash
# PowerShell
$env:CLOUD_SYNC_URL = "http://localhost:3099"
$env:CLOUD_API_TOKEN = "test-token-123"
wails dev
```

---

## Method 4: Go Unit Test

```go
server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
    var payload study.SyncPayload
    json.NewDecoder(r.Body).Decode(&payload)
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(map[string]interface{}{
        "new_notebooks": []interface{}{},
    })
}))
defer server.Close()
testRepo.ExecForTest(`UPDATE user_settings SET cloud_sync_url = ?, cloud_api_token = 'token' WHERE id = 1`, server.URL)
err := study.TriggerCloudSync(testRepo)
```

Run: `go test -run TestTriggerCloudSync -v ./...`

---

## Testing Delta Sync

1. Start mock server, run first sync → note `Review logs: N`
2. Complete a flashcard review → creates new FSRS log
3. Run sync → should show `Review logs: 1`
4. Run again → should show `Review logs: 0`

Inspect: `SELECT last_synced_at FROM user_settings WHERE id = 1;`
Reset: `UPDATE user_settings SET last_synced_at = 0 WHERE id = 1;`

---

## Troubleshooting

| Symptom | Cause | Fix |
|---------|-------|-----|
| "Sync Server URL" field hidden | Not in dev mode | Run `wails dev` |
| Sync does nothing | `cloud_sync_url` empty | Set URL in Settings or env var |
| `Review logs: 0` first sync | `last_synced_at` already advanced | Reset to 0 |
| Network error | Server not running | Verify mock server |
| `FLASHCARD_GENERATE` in queue | Sync failed 3 times | Fix server, next sync resolves |
