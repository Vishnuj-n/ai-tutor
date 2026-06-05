# AI Tutor Database Schema

## Overview

SQLite is the source of truth. The current schema is centered on the persistent `study_queue`, with content ingestion, quiz generation, FSRS retention, and user settings stored in a small set of explicit tables.

This document matches the tables created in `internal/db/schema.go`.

## Table Map

| Layer         | Tables                                                                                                       |
| ------------- | ------------------------------------------------------------------------------------------------------------ |
| Queue         | `study_queue`, `reading_progress`, `review_task_cards`                                                       |
| Content       | `notebooks`, `topics`, `parents`, `chunks`, `notebook_topics`, `notebook_chunks`, `topic_progress`           |
| Assessment    | `questions`, `user_answers`, `quiz_attempts`, `reread_attempts`, `written_questions`, `written_user_answers` |
| Retention     | `fsrs_cards`, `fsrs_review_log`, `assessment_fsrs`                                                           |
| Configuration | `user_settings`                                                                                              |

## Queue Tables

### `study_queue`

Central task table for the application.

| Field | Type | Description |
|---|---|---|
| `id` | TEXT PRIMARY KEY | Unique task identifier |
| `notebook_id` | TEXT NOT NULL | Parent notebook |
| `topic_id` | TEXT | Optional task context |
| `task_type` | TEXT NOT NULL | `READING`, `QUIZ`, `REREAD`, `FLASHCARD_REVIEW`, `EXAMINER` |
| `status` | TEXT NOT NULL | `PENDING`, `ACTIVE`, `COMPLETED`, `SKIPPED`, `FAILED` |
| `priority` | INTEGER DEFAULT 0 | Lower values sort first |
| `created_at` | TIMESTAMP DEFAULT CURRENT_TIMESTAMP | Creation time |
| `activated_at` | TIMESTAMP | When task became active |
| `completed_at` | TIMESTAMP | When task finished |
| `payload_json` | TEXT | Optional task payload |
| `start_page` | INTEGER | Reading start page |
| `end_page` | INTEGER | Reading end page |

**Indexes**

```sql
CREATE INDEX idx_study_queue_status_priority_created ON study_queue(status, priority, created_at);
CREATE INDEX idx_study_queue_notebook_status ON study_queue(notebook_id, status);
```

### `reading_progress`

Per-task reading cursor.

| Field | Type | Description |
|---|---|---|
| `task_id` | TEXT PRIMARY KEY | Reference to `study_queue(id)` |
| `current_page` | INTEGER DEFAULT 0 | Last visited page |
| `last_accessed_at` | TIMESTAMP DEFAULT CURRENT_TIMESTAMP | Last update time |

### `review_task_cards`

Links a flashcard review task to the cards selected for that session.

| Field | Type | Description |
|---|---|---|
| `task_id` | TEXT NOT NULL | Reference to `study_queue(id)` |
| `card_id` | TEXT NOT NULL | Reference to `fsrs_cards(id)` |
| `status` | TEXT NOT NULL DEFAULT 'pending' | Per-card session state |

Primary key: `(task_id, card_id)`.

## Content Tables

### `notebooks`

Top-level container for uploaded study material.

| Field | Type | Description |
|---|---|---|
| `id` | TEXT PRIMARY KEY | Notebook identifier |
| `title` | TEXT NOT NULL | Notebook title |
| `file_path` | TEXT NOT NULL | Local file path |
| `file_type` | TEXT DEFAULT 'pdf' | File type |
| `topic_id` | TEXT | Primary topic reference |
| `priority` | INTEGER DEFAULT 5 | Queue bias for this notebook |
| `status` | TEXT DEFAULT 'uploaded' | Notebook status |
| `indexing_status` | TEXT DEFAULT 'PENDING' | Ingestion state |
| `page_count` | INTEGER | Page count if known |
| `chunk_count` | INTEGER DEFAULT 0 | Number of chunks created |
| `syllabus_draft_json` | TEXT | Draft syllabus payload |
| `uploaded_at` | TIMESTAMP DEFAULT CURRENT_TIMESTAMP | Upload time |

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
| `created_at` | TIMESTAMP DEFAULT CURRENT_TIMESTAMP | Creation time |
| `updated_at` | TIMESTAMP DEFAULT CURRENT_TIMESTAMP | Update time |

### `parents`

Stores hierarchical headings for a topic.

| Field | Type | Description |
|---|---|---|
| `id` | TEXT PRIMARY KEY | Parent heading identifier |
| `topic_id` | TEXT NOT NULL | Reference to `topics(id)` |
| `heading` | TEXT | Heading text |
| `order_index` | INTEGER | Ordering within a topic |
| `content_text` | TEXT NOT NULL | Headline or section text |
| `created_at` | TIMESTAMP DEFAULT CURRENT_TIMESTAMP | Creation time |

### `chunks`

Granular content chunks produced from the source document.

| Field | Type | Description |
|---|---|---|
| `id` | TEXT PRIMARY KEY | Chunk identifier |
| `topic_id` | TEXT NOT NULL | Reference to `topics(id)` |
| `parent_id` | TEXT NOT NULL | Reference to `parents(id)` |
| `chunk_text` | TEXT NOT NULL | Chunk content |
| `page_num` | INTEGER DEFAULT 0 | Source page |
| `token_count` | INTEGER DEFAULT 0 | Token count |
| `importance_score` | REAL DEFAULT 0 | Relative importance |
| `weakness_score` | REAL DEFAULT 0 | Weakness signal |
| `embedding_ref` | TEXT | Reference used by retrieval code |
| `created_at` | TIMESTAMP DEFAULT CURRENT_TIMESTAMP | Creation time |

**Indexes**

```sql
CREATE INDEX idx_chunks_topic_page_num ON chunks(topic_id, page_num);
```

### `notebook_topics`

Many-to-many link between notebooks and topics.

| Field | Type | Description |
|---|---|---|
| `notebook_id` | TEXT NOT NULL | Reference to `notebooks(id)` |
| `topic_id` | TEXT NOT NULL | Reference to `topics(id)` |
| `created_at` | TIMESTAMP DEFAULT CURRENT_TIMESTAMP | Link creation time |

Primary key: `(notebook_id, topic_id)`.

### `notebook_chunks`

Many-to-many link between notebooks and chunks.

| Field | Type | Description |
|---|---|---|
| `id` | TEXT PRIMARY KEY | Link row identifier |
| `notebook_id` | TEXT NOT NULL | Reference to `notebooks(id)` |
| `chunk_id` | TEXT NOT NULL | Reference to `chunks(id)` |
| `page_num` | INTEGER DEFAULT 0 | Source page |
| `created_at` | TIMESTAMP DEFAULT CURRENT_TIMESTAMP | Link creation time |

Unique key: `(notebook_id, chunk_id)`.

### `topic_progress`

Topic-level learning metadata.

| Field | Type | Description |
|---|---|---|
| `topic_id` | TEXT PRIMARY KEY | Reference to `topics(id)` |
| `learned_at` | TIMESTAMP | When topic was marked learned |
| `last_read_at` | TIMESTAMP | Last read time |
| `mastery_score` | REAL DEFAULT 0 | Topic mastery score |
| `review_enabled` | INTEGER DEFAULT 0 | Whether review is enabled |

## Assessment Tables

### `questions`

Multiple-choice quiz questions.

| Field | Type | Description |
|---|---|---|
| `id` | TEXT PRIMARY KEY | Question identifier |
| `topic_id` | TEXT NOT NULL | Reference to `topics(id)` |
| `source_chunk_id` | TEXT | Optional source chunk |
| `prompt` | TEXT NOT NULL | Question prompt |
| `options_json` | TEXT NOT NULL | Answer options payload |
| `correct_answer` | TEXT NOT NULL | Correct option value |
| `explanation` | TEXT | Answer explanation |
| `hint` | TEXT | Hint text |
| `source_heading` | TEXT | Source heading |
| `source_snippet` | TEXT | Source excerpt |
| `source_page_start` | INTEGER DEFAULT 0 | Source start page |
| `source_page_end` | INTEGER DEFAULT 0 | Source end page |
| `llm_model` | TEXT | Model used for generation |
| `prompt_version` | TEXT | Prompt version |
| `created_at` | TIMESTAMP DEFAULT CURRENT_TIMESTAMP | Creation time |

### `user_answers`

Submitted answers for `questions`.

| Field | Type | Description |
|---|---|---|
| `id` | TEXT PRIMARY KEY | Answer identifier |
| `question_id` | TEXT NOT NULL | Reference to `questions(id)` |
| `user_answer` | TEXT NOT NULL | Selected answer |
| `is_correct` | INTEGER NOT NULL | Boolean flag |
| `score` | INTEGER NOT NULL | Per-answer score |
| `feedback` | TEXT | Feedback text |
| `hint` | TEXT | Hint shown or returned |
| `created_at` | TIMESTAMP DEFAULT CURRENT_TIMESTAMP | Submission time |

### `quiz_attempts`

Rollup of a completed quiz task.

| Field | Type | Description |
|---|---|---|
| `id` | TEXT PRIMARY KEY | Attempt identifier |
| `task_id` | TEXT NOT NULL | Reference to `study_queue(id)` |
| `score` | INTEGER NOT NULL | Final score |
| `passed` | INTEGER NOT NULL | Pass/fail flag |
| `answers_json` | TEXT NOT NULL | Serialized answers |
| `feedback` | TEXT | Attempt-level feedback |
| `completed_at` | INTEGER NOT NULL | Completion timestamp |
| `created_at` | TIMESTAMP DEFAULT CURRENT_TIMESTAMP | Record creation time |

### `reread_attempts`

Per-topic remediation counter.

| Field | Type | Description |
|---|---|---|
| `topic_id` | TEXT PRIMARY KEY | Reference to `topics(id)` |
| `attempt_count` | INTEGER NOT NULL DEFAULT 0 | Automatic reread count |
| `last_attempt_at` | TIMESTAMP DEFAULT CURRENT_TIMESTAMP | Last update time |

### `written_questions`

Written-response prompts.

| Field | Type | Description |
|---|---|---|
| `id` | TEXT PRIMARY KEY | Prompt identifier |
| `topic_id` | TEXT NOT NULL | Reference to `topics(id)` |
| `prompt` | TEXT NOT NULL | Written prompt |
| `source_chunk_id` | TEXT | Optional source chunk |
| `source_heading` | TEXT | Source heading |
| `source_page_start` | INTEGER DEFAULT 0 | Source start page |
| `source_page_end` | INTEGER DEFAULT 0 | Source end page |
| `llm_model` | TEXT | Model used for generation |
| `prompt_version` | TEXT | Prompt version |
| `created_at` | TIMESTAMP DEFAULT CURRENT_TIMESTAMP | Creation time |
| `updated_at` | TIMESTAMP DEFAULT CURRENT_TIMESTAMP | Update time |

### `written_user_answers`

Submitted written responses.

| Field | Type | Description |
|---|---|---|
| `id` | TEXT PRIMARY KEY | Answer identifier |
| `written_question_id` | TEXT NOT NULL | Reference to `written_questions(id)` |
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
| `topic_id` | TEXT NOT NULL | Reference to `topics(id)` |
| `source_chunk_id` | TEXT | Optional source chunk |
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
CREATE INDEX idx_fsrs_due ON fsrs_cards(due_at);
CREATE INDEX idx_fsrs_topic ON fsrs_cards(topic_id);
```

### `fsrs_review_log`

Immutable review log for flashcards.

| Field | Type | Description |
|---|---|---|
| `id` | TEXT PRIMARY KEY | Log identifier |
| `topic_id` | TEXT NOT NULL | Reference to `topics(id)` |
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

### `assessment_fsrs`

FSRS state for assessment activities.

| Field | Type | Description |
|---|---|---|
| `activity_type` | TEXT NOT NULL | Activity type |
| `reference_id` | TEXT NOT NULL | Assessment reference |
| `topic_id` | TEXT NOT NULL | Reference to `topics(id)` |
| `source_chunk_id` | TEXT | Optional source chunk |
| `state_json` | TEXT NOT NULL | FSRS state payload |
| `due_at` | INTEGER | Next due timestamp |
| `last_reviewed_at` | INTEGER | Last review timestamp |
| `created_at` | TIMESTAMP DEFAULT CURRENT_TIMESTAMP | Record creation time |
| `updated_at` | TIMESTAMP DEFAULT CURRENT_TIMESTAMP | Update time |

Primary key: `(activity_type, reference_id, source_chunk_id)`.

**Indexes**

```sql
CREATE INDEX idx_assessment_fsrs_topic_due_at ON assessment_fsrs(topic_id, due_at);
```

## Configuration Table

### `user_settings`

Singleton table for global preferences.

| Field | Type | Description |
|---|---|---|
| `id` | INTEGER PRIMARY KEY CHECK (id = 1) | Singleton row key |
| `daily_study_minutes` | INTEGER NOT NULL DEFAULT 90 | Daily study target |
| `updated_at` | TIMESTAMP DEFAULT CURRENT_TIMESTAMP | Last update time |

## Key Relationships

- `notebooks` can link to one or more `topics` through `notebook_topics`.
- `topics` own `parents`, `chunks`, `questions`, `written_questions`, `fsrs_cards`, `fsrs_review_log`, and `assessment_fsrs` rows.
- `questions` and `written_questions` can reference `chunks` for source context.
- `quiz_attempts` records the outcome of a `study_queue` quiz task.
- `review_task_cards` binds a `FLASHCARD_REVIEW` queue task to the exact cards reviewed in that session.

## Legacy Terms Removed From The Current Schema

The live schema no longer uses the legacy table names. Mapping (old → current):

- `blocks` → `parents` + `chunks` (section headings and granular content chunks)
- `quiz_sets` → `questions` (multiple-choice questions; see `questions.options_json`)
- `sources` → `notebooks` (source documents are stored in `notebooks` and linked via `notebook_chunks` / `notebook_topics`)
- `app_config` → `user_settings` (singleton configuration stored in `user_settings`)
- `block_vectors` → embeddings managed by the RAG embedding store; `chunks.embedding_ref` holds references to external/vector storage

These mappings are documentation-only: the code and live schema already use the current table names. Before removing any legacy migration scripts or external references, verify there are no external systems (backups, ETL jobs, CI scripts) that still depend on the legacy names.

## Data Flow Summary

1. Ingestion creates `notebooks`, `topics`, `parents`, and `chunks`.
2. Study work is queued through `study_queue`.
3. Quiz generation uses `questions` and `written_questions`; answers land in `user_answers` and `written_user_answers`.
4. Quiz completion is rolled up in `quiz_attempts`, with `reread_attempts` tracking repeated remediation.
5. Long-term retention is handled by `fsrs_cards`, `fsrs_review_log`, and `assessment_fsrs`.
6. Session-specific review mappings live in `review_task_cards`, and per-task reading cursors live in `reading_progress`.
