# AI Tutor Database Schema

## Overview

SQLite is the source of truth. The `study_queue` table drives all user flows. All tables support the persistent queue architecture.

---

## Core Queue Table

### `study_queue`

The central queue that drives all application flow.

| Field | Type | Description |
|-------|------|-------------|
| `id` | TEXT PRIMARY KEY | Unique task identifier (UUID) |
| `task_type` | TEXT NOT NULL | `READING`, `QUIZ`, `REREAD`, `FLASHCARD_REVIEW`, `EXAMINER` |
| `block_id` | TEXT | Reference to content block (chunk, quiz_set, card_set) |
| `related_id` | TEXT | Optional related entity (topic_id for grouping) |
| `notebook_id` | TEXT | Reference to notebooks for priority biasing |
| `status` | TEXT NOT NULL | `PENDING`, `ACTIVE`, `COMPLETED`, `SKIPPED`, `FAILED` |
| `priority` | INTEGER NOT NULL | Lower = higher priority (1 = urgent) |
| `created_at` | INTEGER NOT NULL | Unix timestamp |
| `activated_at` | INTEGER | When task became ACTIVE (NULL if never active) |
| `completed_at` | INTEGER | Unix timestamp (NULL if pending) |
| `reread_attempt` | INTEGER DEFAULT 0 | Count of reread cycles for this material |
| `generation_status` | TEXT | `GENERATING`, `READY`, `FAILED` (for QUIZ tasks) |

**Indexes:**
```sql
CREATE INDEX idx_queue_status_priority ON study_queue(status, priority, created_at);
CREATE INDEX idx_queue_related ON study_queue(related_id, status);
CREATE INDEX idx_queue_notebook ON study_queue(notebook_id, status);
CREATE INDEX idx_queue_active_timeout ON study_queue(status, activated_at);
```

**Task Types:**

| Type | Purpose | Created By |
|------|---------|------------|
| `READING` | Read a content block | Ingestion pipeline |
| `QUIZ` | Take a generated quiz | Reading completion |
| `REREAD` | Revisit material (remediation) | Failed quiz |
| `FLASHCARD_REVIEW` | Review due flashcards (block-level) | FSRS scheduler |
| `EXAMINER` | Written assessment | Mastery threshold |

**Task Status Values:**

| Status | Meaning | Transition |
|--------|---------|------------|
| `PENDING` | Waiting in queue | → ACTIVE (on open) |
| `ACTIVE` | Currently being worked | → COMPLETED/SKIPPED/FAILED |
| `COMPLETED` | Successfully finished | Terminal |
| `SKIPPED` | User bypassed task | Terminal, auditable |
| `FAILED` | Quiz generation failed or error | Terminal, can retry |

**Generation Status (QUIZ tasks only):**

| Status | Meaning |
|--------|---------|
| `GENERATING` | LLM call in progress |
| `READY` | Quiz ready for user |
| `FAILED` | Generation error, user-visible |

**Reread Protection:**

- `reread_attempt` tracks how many times material has been assigned for reread
- Default max: 3 attempts per block
- After max reached: stop auto-inserting reread tasks, show manual retry option

---

## Content Tables

### `notebooks`

Top-level container for study materials (multi-notebook support).

| Field | Type | Description |
|-------|------|-------------|
| `id` | TEXT PRIMARY KEY | Unique notebook identifier |
| `title` | TEXT NOT NULL | Notebook name |
| `priority` | INTEGER DEFAULT 5 | 1-10 (higher = more frequent in queue) |
| `created_at` | INTEGER | Unix timestamp |
| `updated_at` | INTEGER | Unix timestamp |

### `topics`

Organizational unit for study material within a notebook.

| Field | Type | Description |
|-------|------|-------------|
| `id` | TEXT PRIMARY KEY | Unique topic identifier |
| `notebook_id` | TEXT NOT NULL | Parent notebook reference |
| `title` | TEXT NOT NULL | Topic name |
| `status` | TEXT | `unseen`, `reading`, `learned` |
| `start_page` | INTEGER | First page in source |
| `end_page` | INTEGER | Last page in source |
| `current_page_cursor` | INTEGER | Last read position |
| `created_at` | INTEGER | Unix timestamp |
| `updated_at` | INTEGER | Unix timestamp |

### `blocks`

Content blocks created by sliding window chunking.

| Field | Type | Description |
|-------|------|-------------|
| `id` | TEXT PRIMARY KEY | Unique block identifier |
| `topic_id` | TEXT NOT NULL | Parent topic reference |
| `block_type` | TEXT NOT NULL | `CHUNK`, `QUIZ`, `FLASHCARD` |
| `content` | TEXT | Text content or JSON payload |
| `word_count` | INTEGER | For progress tracking |
| `order_index` | INTEGER | Sequence within topic |
| `start_page` | INTEGER | Source page start |
| `end_page` | INTEGER | Source page end |
| `reread_count` | INTEGER | Number of reread cycles completed |
| `created_at` | INTEGER | Unix timestamp |

**Block Storage:**

| Field | Purpose |
|-------|---------|
| `id` | Unique block identifier |
| `topic_id` | Parent topic reference |
| `block_type` | `CHUNK`, `QUIZ`, `FLASHCARD` |
| `content` | Text content or JSON payload |
| `word_count` | For progress tracking |
| `order_index` | Sequence within topic |
| `start_page`, `end_page` | Page provenance |
| `reread_count` | Number of reread cycles completed |

**Indexes:**
```sql
CREATE INDEX idx_blocks_topic ON blocks(topic_id, order_index);
CREATE INDEX idx_blocks_type ON blocks(block_type, topic_id);
```

### `block_vectors`

Embedding storage via sqlite-vec virtual table.

| Field | Type | Description |
|-------|------|-------------|
| `block_id` | TEXT | Reference to blocks table |
| `embedding` | JSON | Float32 vector as JSON string |

---

## Quiz Tables

### `quiz_sets`

Generated quiz content.

| Field | Type | Description |
|-------|------|-------------|
| `id` | TEXT PRIMARY KEY | Quiz set identifier |
| `topic_id` | TEXT NOT NULL | Parent topic |
| `block_id` | TEXT | Source block reference |
| `payload_json` | TEXT NOT NULL | Quiz questions/answers JSON |
| `created_at` | INTEGER | Unix timestamp |
| `score_threshold` | INTEGER | Pass threshold (default 70) |

### `quiz_attempts`

User quiz submissions.

| Field | Type | Description |
|-------|------|-------------|
| `id` | TEXT PRIMARY KEY | Attempt identifier |
| `quiz_set_id` | TEXT NOT NULL | Reference to quiz_sets |
| `score` | INTEGER | Percentage score (0-100) |
| `passed` | BOOLEAN | Score >= threshold |
| `answers_json` | TEXT | User answers |
| `completed_at` | INTEGER | Unix timestamp |

---

## Flashcard Tables

### `fsrs_cards`

Individual flashcards with FSRS state.

| Field | Type | Description |
|-------|------|-------------|
| `id` | TEXT PRIMARY KEY | Card identifier |
| `topic_id` | TEXT NOT NULL | Parent topic |
| `block_id` | TEXT | Source content block |
| `prompt` | TEXT NOT NULL | Front of card |
| `answer` | TEXT NOT NULL | Back of card |
| `state_json` | TEXT | FSRS scheduling state |
| `due_at` | INTEGER | Next review timestamp |
| `created_at` | INTEGER | Unix timestamp |

**Indexes:**
```sql
CREATE INDEX idx_fsrs_due ON fsrs_cards(due_at);
CREATE INDEX idx_fsrs_topic ON fsrs_cards(topic_id);
```

### `fsrs_review_log`

Audit trail of all reviews.

| Field | Type | Description |
|-------|------|-------------|
| `id` | INTEGER PRIMARY KEY | Auto-increment |
| `card_id` | TEXT NOT NULL | Reference to fsrs_cards |
| `rating` | INTEGER | 1=Again, 2=Hard, 3=Good, 4=Easy |
| `before_state` | TEXT | FSRS state before review |
| `after_state` | TEXT | FSRS state after review |
| `reviewed_at` | INTEGER | Unix timestamp |

---

## Source Tables

### `sources`

Original uploaded files.

| Field | Type | Description |
|-------|------|-------------|
| `id` | TEXT PRIMARY KEY | Source file identifier |
| `filename` | TEXT NOT NULL | Original filename |
| `file_path` | TEXT | Local storage path |
| `file_type` | TEXT | `pdf`, `txt`, `md` |
| `page_count` | INTEGER | Total pages |
| `created_at` | INTEGER | Unix timestamp |

---

## Configuration Tables

### `app_config`

User settings and preferences.

| Field | Type | Description |
|-------|------|-------------|
| `key` | TEXT PRIMARY KEY | Config key |
| `value` | TEXT | Config value |

---

## Schema Design Principles

### 1. Queue-Centric

All user flows originate from `study_queue`. The dashboard queries:

```sql
SELECT * FROM study_queue 
WHERE status = 'PENDING' 
ORDER BY priority ASC, created_at ASC 
LIMIT 1;
```

### 2. Deterministic Task Types

Task types are explicit enums, not dynamic strings:
- `READING` - Content consumption
- `QUIZ` - Knowledge assessment
- `REREAD` - Remediation
- `FLASHCARD_REVIEW` - Spaced repetition
- `EXAMINER` - Written assessment

### 3. Block-Based Content

All content stored in `blocks` table with uniform schema:
- `CHUNK` blocks for reading
- `QUIZ` blocks for assessments
- `FLASHCARD` blocks for review

### 4. FSRS Integration

FSRS is data-only:
- Calculates due dates
- Updates `state_json` on cards
- Creates `FLASHCARD_REVIEW` tasks when `due_at <= now`
- Does NOT orchestrate flow

### 5. Audit Trail

Key tables have review logs:
- `fsrs_review_log` - All card reviews
- `quiz_attempts` - All quiz submissions
- `app_events` (optional) - System events

### 6. Multi-Notebook Priority Biasing

Notebooks have priority (1-10, default 5). Higher priority notebooks surface more frequently in the queue.

Queue ordering applies this priority hierarchy FIRST, then notebook priority as bias:

| Order | Task Type | Rationale |
|-------|-----------|-----------|
| 1 | `FLASHCARD_REVIEW` (due reviews) | Spaced repetition is time-sensitive |
| 2 | `REREAD` | Remediation should be timely |
| 3 | `QUIZ` | Assessment follows reading |
| 4 | `READING` | New material after obligations |
| 5 | `EXAMINER` | Optional advanced assessment |

Within each tier, notebook priority biases ordering:

```sql
-- Priority hierarchy with notebook bias
SELECT * FROM study_queue sq
LEFT JOIN notebooks n ON sq.notebook_id = n.id
WHERE sq.status = 'PENDING'
ORDER BY 
  CASE sq.task_type
    WHEN 'FLASHCARD_REVIEW' THEN 1
    WHEN 'REREAD' THEN 2
    WHEN 'QUIZ' THEN 3
    WHEN 'READING' THEN 4
    WHEN 'EXAMINER' THEN 5
  END,
  n.priority DESC,
  sq.priority ASC,
  sq.created_at ASC;
```

### 7. Task Lifecycle Semantics

Explicit state transitions:

```
PENDING → ACTIVE (when user opens task)
ACTIVE → COMPLETED (on success)
ACTIVE → SKIPPED (on user skip)
ACTIVE → FAILED (on error/generation failure)
```

**Crash Recovery:**
- ACTIVE tasks older than timeout threshold (e.g., 30 minutes) revert to PENDING on startup
- This ensures restart-safe queue recovery
- `activated_at` timestamp tracks when task became active

```sql
-- Crash recovery: reset stale ACTIVE tasks
UPDATE study_queue 
SET status = 'PENDING', activated_at = NULL
WHERE status = 'ACTIVE' 
  AND activated_at < (strftime('%s', 'now') - 1800); -- 30 min timeout
```

### 8. Dashboard Starvation Protection

To prevent review monopolization (e.g., 500 flashcards blocking all reading):

**Deterministic Balancing Rule:**
After N review tasks (`FLASHCARD_REVIEW` or `REREAD`), allow one `READING` task.

Recommended: N = 5 (after 5 reviews, surface 1 reading)

This is a lightweight query-time bias, NOT autonomous orchestration.

**Balancing rules are static SQL ordering constraints, not adaptive runtime systems.**

### 9. Reading Validation

Minimal validation: user must reach final assigned page before Complete button activates.

- `current_page_cursor` tracked during reading
- Complete button disabled until `current_page_cursor >= end_page`
- No surveillance logic, timers, or engagement tracking

### 10. Flashcard Review Granularity

**One `FLASHCARD_REVIEW` task = one review session for a block/chunk.**

- Do NOT create one queue task per flashcard
- A single task represents "review all due cards in this block"
- Prevents queue explosion with many cards

---

## What This Replaces

| Old Approach | New Approach |
|--------------|--------------|
| Runtime-only queues | `study_queue` table |
| Hidden orchestrators | Explicit orchestrator service |
| In-memory session engines | Persistent SQLite state |
| Proactive scheduling | Query-driven task fetching |
| Complex state machines | Status enum transitions |

---

## Query Examples

### Get Dashboard Tasks
```sql
SELECT 
  sq.id,
  sq.task_type,
  sq.priority,
  t.title as topic_title,
  b.word_count
FROM study_queue sq
LEFT JOIN topics t ON sq.related_id = t.id
LEFT JOIN blocks b ON sq.block_id = b.id
WHERE sq.status = 'PENDING'
ORDER BY sq.priority ASC, sq.created_at ASC;
```

### Get Reading Progress
```sql
SELECT 
  COUNT(CASE WHEN status = 'COMPLETED' THEN 1 END) as completed,
  COUNT(*) as total
FROM study_queue
WHERE task_type = 'READING' AND related_id = ?;
```

### Get Due Flashcards (Create Tasks)
```sql
SELECT * FROM fsrs_cards 
WHERE due_at <= strftime('%s', 'now');
```

### Mark Task Complete
```sql
UPDATE study_queue 
SET status = 'COMPLETED', completed_at = strftime('%s', 'now')
WHERE id = ?;
```
