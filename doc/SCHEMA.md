# AI Tutor Database Schema

## Overview

SQLite is the source of truth. The current schema is centered on the persistent `study_queue`, with content ingestion, quiz generation, FSRS retention, profile management, and user/LLM settings stored in a set of explicit tables.

This document is generated from and must remain synchronized with `internal/db/schema.go`. Every `CREATE TABLE IF NOT EXISTS` and `CREATE INDEX IF NOT EXISTS` statement in `InitSchema()` must have a corresponding documented entry below.

## Table Map

| Layer         | Tables                                                                                  |
| ------------- | --------------------------------------------------------------------------------------- |
| Queue         | `study_queue`, `reading_progress`, `review_task_cards`                                  |
| Content       | `notebooks`, `topics`, `chunks`, `notebook_topics`, `notebook_chunks`, `topic_progress` |
| Assessment    | `quiz_attempts`, `reread_attempts`, `written_questions`, `written_user_answers`         |
| Retention     | `fsrs_cards`, `fsrs_review_log`, `manual_flashcards`                                    |
| Configuration | `user_settings`, `llm_settings`, `study_profiles`                                       |

## Queue Tables

### `study_queue`

Central task table for the application.

| Field          | Type                                | Description                                                 |
| -------------- | ----------------------------------- | ----------------------------------------------------------- |
| `id`           | TEXT PRIMARY KEY                    | Unique task identifier                                      |
| `notebook_id`  | TEXT NOT NULL                       | Parent notebook. FK → `notebooks(id)`                       |
| `topic_id`     | TEXT                                | Optional task context. FK → `topics(id)`                    |
| `task_type`    | TEXT NOT NULL                       | `READING`, `QUIZ`, `REREAD`, `FLASHCARD_REVIEW`, `EXAMINER`, `SOCRATIC_REMEDIAL`, `FLASHCARD_SYNC` |
| `status`       | TEXT NOT NULL                       | `PENDING`, `ACTIVE`, `COMPLETED`, `SKIPPED`, `FAILED`       |
| `priority`     | INTEGER DEFAULT 0                   | Lower values sort first                                     |
| `created_at`   | TIMESTAMP DEFAULT CURRENT_TIMESTAMP | Creation time                                               |
| `activated_at` | TIMESTAMP                           | When task became active                                     |
| `completed_at` | TIMESTAMP                           | When task finished                                          |
| `payload_json` | TEXT                                | Optional task payload                                       |
| `start_page`   | INTEGER                             | Reading start page                                          |
| `end_page`     | INTEGER                             | Reading end page                                            |

**Foreign keys:** `notebook_id` → `notebooks(id)`, `topic_id` → `topics(id)`.

**Indexes**

```sql
CREATE INDEX idx_study_queue_status_priority_created ON study_queue(status, priority, created_at);
CREATE INDEX idx_study_queue_notebook_status ON study_queue(notebook_id, status);
```

### `reading_progress`

Per-task reading cursor.

| Field | Type | Description |
|---|---|---|
| `task_id` | TEXT PRIMARY KEY | FK → `study_queue(id)` |
| `current_page` | INTEGER DEFAULT 0 | Last visited page |
| `last_accessed_at` | TIMESTAMP DEFAULT CURRENT_TIMESTAMP | Last update time |

**Foreign keys:** `task_id` → `study_queue(id)`.

### `review_task_cards`

Links a flashcard review task to the cards selected for that session.

| Field | Type | Description |
|---|---|---|
| `task_id` | TEXT NOT NULL | FK → `study_queue(id)` ON DELETE CASCADE |
| `card_id` | TEXT NOT NULL | FK → `fsrs_cards(id)` ON DELETE CASCADE |
| `status` | TEXT NOT NULL DEFAULT 'pending' | Per-card session state |

Primary key: `(task_id, card_id)`.

**Indexes**

```sql
CREATE INDEX idx_review_task_cards_task_status ON review_task_cards(task_id, status);
```

## Content Tables

### `notebooks`

Top-level container for uploaded study material.

| Field | Type | Description |
|---|---|---|
| `id` | TEXT PRIMARY KEY | Notebook identifier |
| `title` | TEXT NOT NULL | Notebook title |
| `file_path` | TEXT NOT NULL | Local file path |
| `file_type` | TEXT DEFAULT 'pdf' | File type |
| `topic_id` | TEXT | Primary topic reference. FK → `topics(id)` |
| `priority` | INTEGER DEFAULT 5 | Queue bias for this notebook |
| `status` | TEXT DEFAULT 'uploaded' | Notebook status |
| `indexing_status` | TEXT DEFAULT 'PENDING' | Ingestion state |
| `page_count` | INTEGER | Page count if known |
| `chunk_count` | INTEGER DEFAULT 0 | Number of chunks created |
| `syllabus_draft_json` | TEXT | Draft syllabus payload |
| `exam_deadline` | TEXT | Exam date string for deadline tracking |
| `profile_id` | TEXT | Owning study profile. FK → `study_profiles(id)` ON DELETE SET NULL |
| `study_status` | TEXT DEFAULT 'dormant' | Lifecycle state (`dormant`, `active`, `completed`) |
| `uploaded_at` | TIMESTAMP DEFAULT CURRENT_TIMESTAMP | Upload time |

**Foreign keys:** `topic_id` → `topics(id)`, `profile_id` → `study_profiles(id)` ON DELETE SET NULL.

### `topics`

Topic or section container.

| Field | Type | Description |
|---|---|---|
| `id` | TEXT PRIMARY KEY | Topic identifier |
| `title` | TEXT NOT NULL | Topic title |
| `status` | TEXT DEFAULT 'reading' | Topic status |
| `start_page` | INTEGER DEFAULT 0 | Start page |
| `end_page` | INTEGER DEFAULT 0 | End page |
| `current_page_cursor` | INTEGER DEFAULT 0 | Latest reading cursor |
| `external_help_required` | BOOLEAN DEFAULT 0 | Whether topic requires external review after failed rescue |
| `created_at` | TIMESTAMP DEFAULT CURRENT_TIMESTAMP | Creation time |
| `updated_at` | TIMESTAMP DEFAULT CURRENT_TIMESTAMP | Update time |

**Indexes**

```sql
CREATE INDEX idx_topics_status_updated_at ON topics(status, updated_at DESC);
CREATE INDEX idx_topics_status_created_at ON topics(status, created_at DESC);
```

### `chunks`

Granular content chunks produced from the source document.

| Field | Type | Description |
|---|---|---|
| `id` | TEXT PRIMARY KEY | Chunk identifier |
| `topic_id` | TEXT NOT NULL | FK → `topics(id)` |
| `chunk_text` | TEXT NOT NULL | Chunk content |
| `page_num` | INTEGER DEFAULT 0 | Source page |
| `token_count` | INTEGER DEFAULT 0 | Token count |
| `importance_score` | REAL DEFAULT 0 | Relative importance |
| `weakness_score` | REAL DEFAULT 0 | Weakness signal |
| `embedding_ref` | TEXT | Reference used by retrieval code |
| `created_at` | TIMESTAMP DEFAULT CURRENT_TIMESTAMP | Creation time |

**Foreign keys:** `topic_id` → `topics(id)`.

**Indexes**

```sql
CREATE INDEX idx_chunks_topic_page_num ON chunks(topic_id, page_num);
```

### `notebook_topics`

Many-to-many link between notebooks and topics.

| Field | Type | Description |
|---|---|---|
| `notebook_id` | TEXT NOT NULL | FK → `notebooks(id)` ON DELETE CASCADE |
| `topic_id` | TEXT NOT NULL | FK → `topics(id)` ON DELETE CASCADE |
| `created_at` | TIMESTAMP DEFAULT CURRENT_TIMESTAMP | Link creation time |

Primary key: `(notebook_id, topic_id)`.

### `notebook_chunks`

Many-to-many link between notebooks and chunks.

| Field | Type | Description |
|---|---|---|
| `id` | TEXT PRIMARY KEY | Link row identifier |
| `notebook_id` | TEXT NOT NULL | FK → `notebooks(id)` |
| `chunk_id` | TEXT NOT NULL | FK → `chunks(id)` |
| `page_num` | INTEGER DEFAULT 0 | Source page |
| `created_at` | TIMESTAMP DEFAULT CURRENT_TIMESTAMP | Link creation time |

Unique constraint on `(notebook_id, chunk_id)` enforced by `idx_notebook_chunk_unique`.

**Indexes**

```sql
CREATE UNIQUE INDEX idx_notebook_chunk_unique ON notebook_chunks(notebook_id, chunk_id);
```

### `topic_progress`

Topic-level learning metadata.

| Field | Type | Description |
|---|---|---|
| `topic_id` | TEXT PRIMARY KEY | FK → `topics(id)` |
| `learned_at` | TIMESTAMP | When topic was marked learned |
| `last_read_at` | TIMESTAMP | Last read time |
| `mastery_score` | REAL DEFAULT 0 | Topic mastery score |
| `review_enabled` | INTEGER DEFAULT 0 | Whether review is enabled |
| `status` | TEXT DEFAULT 'active' | Topic progress lifecycle state |

## Assessment Tables

### `quiz_attempts`

Rollup of a completed quiz task.

| Field | Type | Description |
|---|---|---|
| `id` | TEXT PRIMARY KEY | Attempt identifier |
| `task_id` | TEXT NOT NULL | FK → `study_queue(id)` ON DELETE CASCADE |
| `score` | INTEGER NOT NULL | Final score |
| `passed` | INTEGER NOT NULL | Pass/fail flag |
| `answers_json` | TEXT NOT NULL | Serialized answers |
| `feedback` | TEXT | Attempt-level feedback |
| `completed_at` | INTEGER NOT NULL | Completion timestamp |
| `created_at` | TIMESTAMP DEFAULT CURRENT_TIMESTAMP | Record creation time |

**Indexes**

```sql
CREATE INDEX idx_quiz_attempts_task_completed_at ON quiz_attempts(task_id, completed_at DESC);
```

### `reread_attempts`

Per-topic remediation counter.

| Field | Type | Description |
|---|---|---|
| `topic_id` | TEXT PRIMARY KEY | FK → `topics(id)` ON DELETE CASCADE |
| `attempt_count` | INTEGER NOT NULL DEFAULT 0 | Automatic reread count |
| `last_attempt_at` | TIMESTAMP DEFAULT CURRENT_TIMESTAMP | Last update time |

**Indexes**

```sql
CREATE INDEX idx_reread_attempts_last_attempt_at ON reread_attempts(last_attempt_at DESC);
```

### `written_questions`

Written-response prompts.

| Field | Type | Description |
|---|---|---|
| `id` | TEXT PRIMARY KEY | Prompt identifier |
| `topic_id` | TEXT NOT NULL | FK → `topics(id)` ON DELETE CASCADE |
| `prompt` | TEXT NOT NULL | Written prompt |
| `source_chunk_id` | TEXT | FK → `chunks(id)` ON DELETE SET NULL |
| `source_heading` | TEXT | Source heading |
| `source_page_start` | INTEGER DEFAULT 0 | Source start page |
| `source_page_end` | INTEGER DEFAULT 0 | Source end page |
| `llm_model` | TEXT | Model used for generation |
| `prompt_version` | TEXT | Prompt version |
| `created_at` | TIMESTAMP DEFAULT CURRENT_TIMESTAMP | Creation time |
| `updated_at` | TIMESTAMP DEFAULT CURRENT_TIMESTAMP | Update time |

**Indexes**

```sql
CREATE INDEX idx_written_questions_topic_created_at ON written_questions(topic_id, created_at DESC);
```

### `written_user_answers`

Submitted written responses.

| Field | Type | Description |
|---|---|---|
| `id` | TEXT PRIMARY KEY | Answer identifier |
| `written_question_id` | TEXT NOT NULL | FK → `written_questions(id)` ON DELETE CASCADE |
| `user_answer` | TEXT NOT NULL | Answer text |
| `score` | INTEGER NOT NULL | Evaluation score |
| `feedback` | TEXT | Feedback text |
| `source_heading` | TEXT | Source heading |
| `created_at` | TIMESTAMP DEFAULT CURRENT_TIMESTAMP | Submission time |

## Retention Tables

### `fsrs_cards`

Flashcards with FSRS state.

| Field | Type | Description |
|---|---|---|
| `id` | TEXT PRIMARY KEY | Card identifier |
| `topic_id` | TEXT NOT NULL | FK → `topics(id)` ON DELETE CASCADE |
| `source_chunk_id` | TEXT | FK → `chunks(id)` ON DELETE SET NULL |
| `prompt` | TEXT NOT NULL | Card front |
| `answer` | TEXT NOT NULL | Card back |
| `state_json` | TEXT | FSRS state payload |
| `due_at` | INTEGER | Next due timestamp |
| `suspended` | BOOLEAN DEFAULT 0 | Whether the card is suspended |
| `created_at` | TIMESTAMP DEFAULT CURRENT_TIMESTAMP | Creation time |
| `updated_at` | TIMESTAMP DEFAULT CURRENT_TIMESTAMP | Update time |

**Indexes**

```sql
CREATE UNIQUE INDEX idx_fsrs_cards_topic_prompt ON fsrs_cards(topic_id, prompt);
CREATE INDEX idx_fsrs_cards_suspended_due_at ON fsrs_cards(suspended, due_at);
```

### `fsrs_review_log`

Immutable review log for flashcards.

| Field | Type | Description |
|---|---|---|
| `id` | TEXT PRIMARY KEY | Log identifier |
| `topic_id` | TEXT NOT NULL | FK → `topics(id)` ON DELETE CASCADE |
| `activity_type` | TEXT NOT NULL | Review activity type |
| `reference_id` | TEXT NOT NULL | Activity reference |
| `reviewed_at` | INTEGER NOT NULL | Review timestamp |
| `rating` | INTEGER NOT NULL | Review rating |
| `scheduled_days` | INTEGER NOT NULL | Scheduled interval |
| `state_before_json` | TEXT NOT NULL | State before review |
| `state_after_json` | TEXT NOT NULL | State after review |
| `created_at` | TIMESTAMP DEFAULT CURRENT_TIMESTAMP | Record creation time |

**Indexes**

```sql
CREATE INDEX idx_fsrs_review_log_activity_ref_reviewed_at ON fsrs_review_log(activity_type, reference_id, reviewed_at DESC);
CREATE INDEX idx_fsrs_review_log_topic_reviewed_at ON fsrs_review_log(topic_id, reviewed_at DESC);
```

### `manual_flashcards`

User-created manual flashcards.

| Field | Type | Description |
|---|---|---|
| `id` | TEXT PRIMARY KEY | Card identifier |
| `notebook_id` | TEXT NOT NULL | FK → `notebooks(id)` ON DELETE CASCADE |
| `prompt` | TEXT NOT NULL | Card front |
| `answer` | TEXT NOT NULL | Card back |
| `created_at` | TIMESTAMP DEFAULT CURRENT_TIMESTAMP | Creation time |

**Indexes**

```sql
CREATE INDEX idx_manual_flashcards_notebook_id ON manual_flashcards(notebook_id);
```

## Configuration Tables

### `user_settings`

Singleton table for global preferences.

| Field | Type | Description |
|---|---|---|
| `id` | INTEGER PRIMARY KEY CHECK (id = 1) | Singleton row key |
| `max_flashcards_per_session` | INTEGER NOT NULL DEFAULT 30 | Max flashcards per session |
| `study_start_time` | TEXT DEFAULT '17:00' | Study window start time (HH:MM format) |
| `study_end_time` | TEXT DEFAULT '18:00' | Study window end time (HH:MM format) |
| `reminders_enabled` | BOOLEAN DEFAULT 1 | Whether study reminders are enabled |
| `active_profile_id` | TEXT | Active study profile. FK → `study_profiles(id)` ON DELETE SET NULL |
| `skip_to_reading_active` | BOOLEAN DEFAULT 0 | Skip dashboard to active reading |
| `cloud_sync_url` | TEXT DEFAULT '' | Remote sync endpoint URL |
| `cloud_api_token` | TEXT DEFAULT '' | Remote sync auth token |
| `theme` | TEXT DEFAULT 'light-classic' | UI theme selector |
| `rag_enabled` | BOOLEAN DEFAULT 0 | Master RAG toggle |
| `rag_notebook_chapter` | BOOLEAN DEFAULT 1 | RAG over notebook chapters |
| `rag_entire_notebook` | BOOLEAN DEFAULT 1 | RAG over entire notebook |
| `rag_queue_study` | BOOLEAN DEFAULT 1 | RAG over queued study content |
| `default_remedial_strategy` | TEXT DEFAULT 'CLASSIC' | User preference for quiz failure handling (`CLASSIC` or `FAST`) |
| `updated_at` | TIMESTAMP DEFAULT CURRENT_TIMESTAMP | Last update time |

**Foreign keys:** `active_profile_id` → `study_profiles(id)` ON DELETE SET NULL.

**Bootstrap:** A single row with default settings `(id=1, max_flashcards_per_session=30, study_start_time='17:00', study_end_time='18:00', reminders_enabled=1)` is inserted on initial schema creation.

### `llm_settings`

LLM provider configuration per performance tier.

| Field | Type | Description |
|---|---|---|
| `tier` | TEXT PRIMARY KEY CHECK (tier IN ('fast', 'heavy')) | Performance tier identifier |
| `provider` | TEXT NOT NULL DEFAULT 'groq' | Provider name (e.g. `groq`, `openai`) |
| `base_url` | TEXT NOT NULL DEFAULT '' | API base URL |
| `model` | TEXT NOT NULL DEFAULT '' | Model identifier string |
| `timeout_ms` | INTEGER NOT NULL DEFAULT 30000 | Request timeout in milliseconds |
| `api_key_source` | TEXT NOT NULL DEFAULT 'keyring' | Key storage backend |
| `has_api_key` | BOOLEAN DEFAULT 0 | Whether an API key has been configured |
| `updated_at` | TIMESTAMP DEFAULT CURRENT_TIMESTAMP | Last update time |

**Bootstrap:** Two rows are inserted on initial schema creation:

```
('fast',  'groq', 'https://api.groq.com/openai', 'openai/gpt-oss-120b', 60000, 'keyring', 0)
('heavy', 'groq', 'https://api.groq.com/openai', 'openai/gpt-oss-120b', 90000, 'keyring', 0)
```

### `study_profiles`

Named study profiles with deadline tracking. Referenced by `user_settings.active_profile_id` and `notebooks.profile_id`.

| Field | Type | Description |
|---|---|---|
| `id` | TEXT PRIMARY KEY | Profile identifier |
| `name` | TEXT NOT NULL | Human-readable profile name |
| `deadline_at` | INTEGER NOT NULL | Unix timestamp of target deadline |
| `created_at` | TIMESTAMP DEFAULT CURRENT_TIMESTAMP | Creation time |

**Referenced by:** `user_settings.active_profile_id` (FK → `id` ON DELETE SET NULL), `notebooks.profile_id` (FK → `id` ON DELETE SET NULL).

## Key Relationships

### Foreign Key Graph

| Source Table | Source Column(s) | Target Table | Cascade Behavior |
|---|---|---|---|
| `study_queue` | `notebook_id` | `notebooks(id)` | None |
| `study_queue` | `topic_id` | `topics(id)` | None |
| `reading_progress` | `task_id` | `study_queue(id)` | None |
| `review_task_cards` | `task_id` | `study_queue(id)` | ON DELETE CASCADE |
| `review_task_cards` | `card_id` | `fsrs_cards(id)` | ON DELETE CASCADE |
| `chunks` | `topic_id` | `topics(id)` | None |
| `topic_progress` | `topic_id` | `topics(id)` | None (implicit) |
| `quiz_attempts` | `task_id` | `study_queue(id)` | ON DELETE CASCADE |
| `reread_attempts` | `topic_id` | `topics(id)` | ON DELETE CASCADE |
| `written_questions` | `topic_id` | `topics(id)` | ON DELETE CASCADE |
| `written_questions` | `source_chunk_id` | `chunks(id)` | ON DELETE SET NULL |
| `written_user_answers` | `written_question_id` | `written_questions(id)` | ON DELETE CASCADE |
| `notebooks` | `topic_id` | `topics(id)` | None |
| `notebooks` | `profile_id` | `study_profiles(id)` | ON DELETE SET NULL |
| `notebook_topics` | `notebook_id` | `notebooks(id)` | ON DELETE CASCADE |
| `notebook_topics` | `topic_id` | `topics(id)` | ON DELETE CASCADE |
| `notebook_chunks` | `notebook_id` | `notebooks(id)` | None |
| `notebook_chunks` | `chunk_id` | `chunks(id)` | None |
| `fsrs_cards` | `topic_id` | `topics(id)` | ON DELETE CASCADE |
| `fsrs_cards` | `source_chunk_id` | `chunks(id)` | ON DELETE SET NULL |
| `fsrs_review_log` | `topic_id` | `topics(id)` | ON DELETE CASCADE |
| `manual_flashcards` | `notebook_id` | `notebooks(id)` | ON DELETE CASCADE |
| `user_settings` | `active_profile_id` | `study_profiles(id)` | ON DELETE SET NULL |

### Semantic Relationships

- `notebooks` link to `topics` through `notebook_topics` (M:N).
- `notebooks` link to `chunks` through `notebook_chunks` (M:N).
- `topics` own `chunks`, `written_questions`, `fsrs_cards`, and `fsrs_review_log` rows.
- `written_questions` and `fsrs_cards` can optionally reference a specific `chunk` for source context.
- `quiz_attempts` records the outcome of a `study_queue` QUIZ task.
- `review_task_cards` binds a `FLASHCARD_REVIEW` queue task to the exact cards reviewed in that session.
- `topic_progress` stores per-topic learning metadata (mastery, last read, review toggle).
- `study_profiles` own `notebooks` (via `profile_id`) and are selected as active profile in `user_settings`.
- `llm_settings` is referenced by application code at runtime to configure model providers per tier.

## Legacy Terms Removed From The Current Schema

The live schema no longer uses the legacy table names. Mapping (old → current):

- `blocks` → `chunks` (granular content chunks)
- `quiz_sets` → dynamically generated questions (stored in task payload JSON)
- `sources` → `notebooks` (source documents are stored in `notebooks` and linked via `notebook_chunks` / `notebook_topics`)
- `app_config` → `user_settings` (singleton configuration stored in `user_settings`)
- `block_vectors` → embeddings managed by the RAG embedding store; `chunks.embedding_ref` holds references to external/vector storage

These mappings are documentation-only: the code and live schema already use the current table names. Before removing any legacy migration scripts or external references, verify there are no external systems (backups, ETL jobs, CI scripts) that still depend on the legacy names.

## Data Flow Summary

1. Ingestion creates `notebooks`, `topics`, and `chunks`.
2. Study work is queued through `study_queue`.
3. Quiz generation uses `written_questions` and inline payload quiz questions; answers/attempts land in `quiz_attempts` and `written_user_answers`.
4. Quiz completion is rolled up in `quiz_attempts`, with `reread_attempts` tracking repeated remediation. After 1 failed reread, `SOCRATIC_REMEDIAL` rescue task is inserted.
5. Socratic rescue uses `external_help_required` flag on `topics` to prevent infinite rescue cycles.
6. Long-term retention is handled by `fsrs_cards`, `fsrs_review_log`, and `manual_flashcards`.
7. Cloud sync failures insert `FLASHCARD_SYNC` tasks at highest queue priority.
6. Session-specific review mappings live in `review_task_cards`, and per-task reading cursors live in `reading_progress`.
