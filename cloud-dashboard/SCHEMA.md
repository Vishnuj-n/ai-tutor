# AI Tutor Cloud Dashboard Schema (Supabase PostgreSQL)

## Overview

This document defines the cloud database schema for the **AI Tutor Cloud Dashboard** hosted on Supabase (PostgreSQL). The cloud database receives periodic sync payloads from Desktop clients (SQLite) and serves dashboard data to teacher/student web users.

---

## Table Summary

| Table | Purpose | Primary Key |
|-------|---------|-------------|
| [`student_notebooks`](#student_notebooks) | Tracks student uploaded/assigned notebooks and rescue status (`external_help_required`) | `id` (UUID) |
| [`student_review_logs`](#student_review_logs) | Stores student study session review attempts and FSRS retention logs | `id` (TEXT) |
| [`teacher_assignments`](#teacher_assignments) | Stores assignments distributed by teachers to classroom codes | `id` (TEXT) |
| [`user_accounts`](#user_accounts) | User credentials and roles (`teacher`, `student`) | `username` (TEXT) |
| [`active_sessions`](#active_sessions) | Active authentication session tokens and expiration timestamps | `session_token` (UUID) |
| [`teacher_signup_invites`](#teacher_signup_invites) | One-time invite tokens/emails for teacher account registration | (`classroom_code`, `invite_email_or_username`) |
| [`anonymous_analytics_events`](#anonymous_analytics_events) | Telemetry and usage analytics events | `id` (UUID) |

---

## Table Specifications

### `student_notebooks`

Tracks all student notebooks per classroom for live dashboard monitoring.

| Column | Type | Nullable | Default | Description |
|--------|------|----------|---------|-------------|
| `id` | `UUID` | No | `uuid_generate_v4()` | Unique primary key |
| `student_token` | `TEXT` | No | — | Username or unique token of the student |
| `classroom_code` | `TEXT` | No | — | Classroom identifier (e.g. `BCD601`) |
| `file_hash` | `TEXT` | No | — | SHA-256 hash of the notebook file |
| `filename` | `TEXT` | No | — | Original file name |
| `title` | `TEXT` | No | — | Notebook title |
| `study_status` | `TEXT` | No | — | Status (`dormant`, `active`, `completed`) |
| `external_help_required` | `BOOLEAN` | No | `FALSE` | Socratic Red Alert flag (requires teacher assistance) |
| `updated_at` | `TIMESTAMPTZ` | No | `now()` | Last sync update timestamp |

* **Constraints:** `UNIQUE (student_token, file_hash)`
* **Indexes:** 
  - `idx_student_notebooks_classroom` ON `classroom_code`
  - `idx_student_notebooks_student` ON `student_token`

---

### `student_review_logs`

Stores flashcard and activity review outcomes for progress analytics.

| Column | Type | Nullable | Default | Description |
|--------|------|----------|---------|-------------|
| `id` | `TEXT` | No | — | Unique log entry ID (Primary Key) |
| `student_token` | `TEXT` | No | — | Student username or identifier |
| `classroom_code` | `TEXT` | No | — | Classroom code |
| `file_hash` | `TEXT` | No | — | File hash associated with the task |
| `page_number` | `INTEGER` | No | — | Page number of the review activity |
| `activity_type` | `TEXT` | No | — | Activity type (e.g. `QUIZ`, `FLASHCARD_REVIEW`) |
| `reference_id` | `TEXT` | No | — | Reference target ID (card or quiz ID) |
| `reviewed_at` | `BIGINT` | No | — | Epoch timestamp (milliseconds or seconds) |
| `rating` | `INTEGER` | No | — | Performance score / FSRS rating |
| `scheduled_days` | `INTEGER` | No | — | Next review interval scheduled |
| `state_before_json` | `TEXT` | Yes | `NULL` | FSRS JSON state before review |
| `state_after_json` | `TEXT` | Yes | `NULL` | FSRS JSON state after review |
| `created_at` | `TIMESTAMPTZ` | No | `now()` | Record creation timestamp |

* **Indexes:**
  - `idx_student_review_logs_classroom` ON `classroom_code`
  - `idx_student_review_logs_student` ON `student_token`
  - `idx_student_review_logs_file_hash` ON `file_hash`

---

### `teacher_assignments`

Created by teachers to push downloadable study materials to students.

| Column | Type | Nullable | Default | Description |
|--------|------|----------|---------|-------------|
| `id` | `TEXT` | No | — | Assignment ID (Primary Key) |
| `classroom_code` | `TEXT` | No | — | Target classroom code |
| `title` | `TEXT` | No | — | Assignment title |
| `download_url` | `TEXT` | No | — | HTTP/HTTPS URL for the assignment file |
| `start_page` | `INTEGER` | Yes | `NULL` | Optional starting page number for page-range ingestion |
| `end_page` | `INTEGER` | Yes | `NULL` | Optional ending page number for page-range ingestion |
| `created_at` | `TIMESTAMPTZ` | No | `now()` | Creation timestamp |

* **Constraints:** `CHECK (download_url ILIKE 'http://%' OR download_url ILIKE 'https://%')`
* **Indexes:** `idx_teacher_assignments_classroom` ON `classroom_code`

---

### `user_accounts`

Stores authentication credentials and account metadata for cloud users.

| Column | Type | Nullable | Default | Description |
|--------|------|----------|---------|-------------|
| `username` | `TEXT` | No | — | Account username (Primary Key) |
| `password_hash` | `TEXT` | No | — | Hashed password |
| `role` | `TEXT` | No | — | Role (`teacher` or `student`) |
| `classroom_code` | `TEXT` | No | — | Associated classroom code |
| `is_locked` | `BOOLEAN` | No | `FALSE` | Account lock status |
| `created_at` | `TIMESTAMPTZ` | No | `now()` | Account registration date |

---

### `active_sessions`

Manages session tokens generated during login.

| Column | Type | Nullable | Default | Description |
|--------|------|----------|---------|-------------|
| `session_token` | `UUID` | No | `gen_random_uuid()` | Active session token (Primary Key) |
| `entity_id` | `TEXT` | No | — | Username owning the session |
| `role` | `TEXT` | No | — | Role (`teacher` or `student`) |
| `expires_at` | `TIMESTAMPTZ` | No | — | Expiration timestamp |
| `created_at` | `TIMESTAMPTZ` | No | `now()` | Session creation time |

---

### `teacher_signup_invites`

Manages invitation codes for teacher registration.

| Column | Type | Nullable | Default | Description |
|--------|------|----------|---------|-------------|
| `classroom_code` | `TEXT` | No | — | Classroom code bound to invite |
| `invite_email_or_username` | `TEXT` | No | — | Allowed email or username |
| `is_used` | `BOOLEAN` | No | `FALSE` | Invitation redemption status |

* **Primary Key:** (`classroom_code`, `invite_email_or_username`)

---

### `anonymous_analytics_events`

Telemetry table for optional analytics collection.

| Column | Type | Nullable | Default | Description |
|--------|------|----------|---------|-------------|
| `id` | `UUID` | No | `gen_random_uuid()` | Event ID (Primary Key) |
| `event_type` | `TEXT` | No | — | Event type (`reading_complete`, `quiz_complete`) |
| `file_hash` | `TEXT` | Yes | `''` | File hash context |
| `page_number` | `INTEGER` | Yes | `0` | Related page number |
| `metadata` | `JSONB` | Yes | `NULL` | Event metadata (max 2KB) |
| `created_at` | `TIMESTAMPTZ` | No | `now()` | Event logged timestamp |

* **Constraints:** `CHECK (event_type IN ('reading_complete', 'quiz_complete'))`
* **Indexes:** `anonymous_analytics_events_created_at_idx` ON `created_at`

---

## Related Files

* Master Setup Script: [`cloud-dashboard/supabase/setup_all.sql`](file:///c:/Users/vishn/PROJECT/ai-tutor/cloud-dashboard/supabase/setup_all.sql)
* Table Definitions: [`cloud-dashboard/supabase/01_tables.sql`](file:///c:/Users/vishn/PROJECT/ai-tutor/cloud-dashboard/supabase/01_tables.sql)
* RLS Policies: [`cloud-dashboard/supabase/04_rls_and_storage.sql`](file:///c:/Users/vishn/PROJECT/ai-tutor/cloud-dashboard/supabase/04_rls_and_storage.sql)
