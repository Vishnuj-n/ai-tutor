# Cloud Sync Payload: Stable Identifiers & Teacher Alerts

**Date:** 2026-06-28  
**Author:** AI Pair Programmer  
**Status:** Completed

---

## 1. Goal

Update the cloud sync payload to use a stable **SHA-256 file hash** and **page number** as the key identifiers instead of local database IDs (`topic_id` or `notebook_id`). This ensures that the cloud analytical dashboard can cross-reference study activities from multiple students studying the exact same content, even if their filenames or local database IDs differ across different app installs.

Additionally, implement a **"Red Alert" signal** for teachers to identify when a student has failed both the Classic and Fast-Track remediation paths for a topic and requires external review.

---

## 2. Key Architecture & Design Decisions

### A. Stable Identifiers vs. Local IDs
* **The Problem:** Local IDs (`topic_id` or `notebook_id`) are generated as UUIDs on a per-install basis. If 30 students upload the same biology chapter, they will each have different local IDs. Aggregating these on a teacher dashboard is impossible.
* **The Solution:** Use the file's **SHA-256 hash** as the cross-install identifier. This hash is computed once on file upload. 
* **The Page Number:** Page numbers from the document's chunks identify the location within that file, replacing local `chunk_id` values.

### B. Teacher Red Alerts (Remediation Failures)
* **The Problem:** The teacher needs to know when a student fails to master a concept after all local tutoring resources are exhausted (Quiz Fail #1 → REREAD → Quiz Fail #2 → Socratic Rescue → Re-quiz Fail).
* **The Solution:** When a student fails the final re-quiz, the local engine sets the `topics.external_help_required = 1` flag. We fetch this flag per notebook during sync and pass it as `external_help_required` in the `NotebookSyncRecord` payload.

---

## 3. Database & Schema Modifications

### A. Schema Updates (`internal/db/schema.go`)
* Added `file_hash` column (`TEXT DEFAULT ''`) to the `notebooks` table.
* Added a migration step to automatically add the `file_hash` column to existing notebooks tables if missing.

### B. File Hashing on Ingestion
* Added `utils.FileSHA256()` helper to compute SHA-256 checksums.
* Modified the notebook upload flow (`notebook_endpoints.go`) to automatically calculate and save the file hash when a notebook is created using the new `SetNotebookFileHash` repository method.

---

## 4. Query & Payload Design

### A. FSRS Review Logs Query (`internal/db/fsrs_review_log_repo.go`)
Because `fsrs_review_log` does not duplicate file hashes or page numbers, a multi-table `JOIN` query was added:

```sql
SELECT
    r.id,
    COALESCE(n.file_hash, '') AS file_hash,
    COALESCE(c.page_num, 0) AS page_num,
    r.activity_type,
    r.reference_id,
    r.reviewed_at,
    r.rating,
    r.scheduled_days,
    r.state_before_json,
    r.state_after_json
FROM fsrs_review_log r
LEFT JOIN fsrs_cards f ON f.id = r.reference_id AND r.activity_type = 'flashcard'
LEFT JOIN chunks c ON c.id = f.source_chunk_id
LEFT JOIN notebook_topics nt ON nt.topic_id = r.topic_id
LEFT JOIN notebooks n ON n.id = nt.notebook_id
WHERE r.reviewed_at > ?
ORDER BY r.reviewed_at ASC
```

### B. Notebook Selection Query (`internal/db/notebooks_repo.go`)
To support the alert flag, the model `Notebook` and its SQL queries (`GetNotebooks` & `GetNotebookByID`) were updated to query `file_hash` and subquery the `external_help_required` status:

```sql
SELECT 
    id, title, file_path, file_type, COALESCE(topic_id, ''), COALESCE(status, 'uploaded'), 
    COALESCE(indexing_status, 'PENDING'), page_count, chunk_count, COALESCE(priority, 5), 
    exam_deadline, uploaded_at, COALESCE(profile_id, ''), COALESCE(study_status, 'dormant'),
    COALESCE(file_hash, ''),
    COALESCE((
        SELECT t.external_help_required
        FROM topics t
        WHERE t.id = notebooks.topic_id
           OR t.id IN (SELECT topic_id FROM notebook_topics WHERE notebook_id = notebooks.id)
        LIMIT 1
    ), 0) AS external_help_required
FROM notebooks
```

---

## 5. Sync Payload Contract (`internal/study/sync.go`)

The updated structures mapped during sync:

```go
type NotebookSyncRecord struct {
	FileHash             string `json:"file_hash"`
	Filename             string `json:"filename"`
	Title                string `json:"title"`
	StudyStatus          string `json:"study_status"`
	ExternalHelpRequired bool   `json:"external_help_required"` // "Red Alert" signal
}

type SyncLogEntry struct {
	ID              string `json:"id"`
	FileHash        string `json:"file_hash"`
	PageNumber      int    `json:"page_number"`
	ActivityType    string `json:"activity_type"`
	ReferenceID     string `json:"reference_id"`
	ReviewedAt      int64  `json:"reviewed_at"`
	Rating          int    `json:"rating"`
	ScheduledDays   int    `json:"scheduled_days"`
	StateBeforeJSON string `json:"state_before_json"`
	StateAfterJSON  string `json:"state_after_json"`
}
```

---

## 6. Verification & Robustness
1. **Deduplication & Missing Data Safety:** Added `COALESCE` statements in database queries to handle edge cases where chunk data or file hashes might be missing/null.
2. **Backward Compatibility:** Keeps standard repo method signatures unchanged to avoid mass updates on 50+ local test configurations.
3. **Unit Tests:** Verified using `go test ./...` which passes successfully.
