# SPRINT.md — AI Tutor

**Status:** Active roadmap for Persistent Queue Architecture  
**Last Updated:** 2026-06-13  
**Architecture:** SQLite-backed deterministic queue (NOT autonomous orchestration)

---

## Active Roadmap (Production Proofing, RAG, & Rescue Pipeline)

### Sprint 10: RAG Setup, Asset Management & Environment Verification [DONE]
**Goal:** Make RAG and assets production-proof, verifying architecture requirements dynamically.

- [ ] **Task 10.1: Dynamic CGO & Vector Extension Verification**
  - Implement a startup check to verify if CGO/vec0 SQLite extension is present when RAG is requested/enabled.
  - If RAG is required by the user but DLLs or CGO dependencies are missing, show a clean error/fallback status instead of crashing.
- [ ] **Task 10.2: Asset Downloader Script**
  - Write a reliable Go/shell/Powershell asset manager command/script to download raw embedding models/onnx DLLs on-demand if missing in `%LOCALAPPDATA%/ai-tutor/assets/`.

---

### Sprint 11: 2-Strike Socratic Rescue Pipeline [DONE]
**Goal:** Implement cognitive damage control via database clean slate, queue interleaving, and dual-lane rescue UI.

**Implementation:** 2-strike (`maxAutomaticRereadAttempts = 1`), not 3-strike as originally planned.

- [x] **Task 11.1: Database Intervention & Trigger**
  - Track consecutive quiz failures per topic via `reread_attempts` table.
  - On the 2nd quiz failure (after 1 reread), delete FSRS cards for the topic and insert `SOCRATIC_REMEDIAL` task.
- [x] **Task 11.2: Queue Interleaving**
  - `SOCRATIC_REMEDIAL` task type inserted at priority tier 6 (blocks queue until completed).
  - Quiz marked COMPLETED on rescue insertion to unblock main timeline.
- [x] **Task 11.3: Dual-Lane Breakdown View (UI)**
  - Split-pane layout in `SocraticRescue.vue`:
    - **Option A:** In-App Socratic Tutor (interactive chat with context-grounded leading questions).
    - **Option B:** External AI Prompt (source text preview + copy-to-clipboard for external LLM use).
- [x] **Task 11.4: Dev Mode Bypass Panel**
  - `DevForceSocraticRescue` endpoint for testing (requires `APP_ENV=dev`).
- [x] **Task 11.5: Automated Tests**
  - Tests validating rescue trigger, flashcard deletion, and `SOCRATIC_REMEDIAL` task insertion.

---

### Sprint 12: Cloud Dashboard Handover
**Goal:** Initiate the official cloud dashboard bridge and prepare the SQLite database for cloud sync/handover.

- [ ] **Task 12.1: Schema Audit & Sync Prep**
  - Verify every table has a globally unique UUID key instead of auto-incrementing integer IDs to prevent merge conflicts during cloud sync.
- [ ] **Task 12.2: Sync Status Metadata**
  - Introduce dirty flags (`needs_sync`) and modification timestamps (`updated_at`) on all core tables (`study_queue`, `notebooks`, `profiles`, `flashcards`).
- [ ] **Task 12.3: Handover Payload Endpoint**
  - Create backend endpoints to export and import user profile states as clean JSON payloads for cloud dashboard integration.

---

### SPRINT 13 User Asset Provisioning [DONE]

- Detect missing RAG assets
- Download assets from GitHub Releases
- Show progress UI
- Verify hashes
- Resume failed downloads
- Allow manual asset location

Refer doc/future_plan/cross_platform_asset_delivery.md

- Added in onboarding and settings

---

### Sprint 14: User-Configurable Remediation Strategy [DONE]
**Goal:** Allow users to choose between a standard reread-first remediation and a direct Socratic rescue intervention.

**Implementation:** Added `default_remedial_strategy` to `user_settings` with `CLASSIC` (default) and `FAST` options.

- [x] **Task 14.1: Database & Settings**
  - Added `default_remedial_strategy` column to `user_settings`.
  - Created `GetRemedialStrategy` and `SetRemedialStrategy` DB helpers.
  - Added corresponding Wails bindings.
- [x] **Task 14.2: Quiz Logic Branching**
  - Updated `SubmitQuizAttempt` to check the strategy before starting the transaction.
  - Implemented "Fast Track" to skip reread and insert `SOCRATIC_REMEDIAL` directly on first failure.
- [x] **Task 14.3: Frontend Settings UI**
  - Added "Quiz Failure Rescue" toggle (Classic/Fast Track) in General Settings.
  - Integrated with existing profile change logic to prevent settings resets.
- [x] **Task 14.4: Testing**
  - Added `TestFastTrackSkipsReread`, `TestClassicTrackInsertsReread`, and `TestDefaultIsClassic` tests.

---

### Sprint 15: Simplified FSRS Calibration & Enhanced Features [DONE]
**Goal:** Simplify FSRS calibration with clean initial states, add cloud sync, streak tracking, and UI enhancements.

- [x] **Task 15.1: Simplified FSRS Calibration**
  - Removed `scheduler.NextFSRSState` review simulation
  - Initialize all flashcards with `StateCode: 2` (Review state) to bypass FSRS intraday learning phase
  - Set initial `due_at` based on quiz score:
    - Ace (100%): 3-day offset
    - Pass (<100%): 1-day offset
  - Updated `TestFSRSCalibrationEasyAndDoubleGood` to assert clean Review state and day-based offsets

- [x] **Task 15.2: Cloud Sync with Stable Identifiers**
  - Implemented cloud sync functionality with stable identifiers
  - Added `file_hash` to notebooks for cross-install identification
  - Added `page_number` to sync logs for stable referencing
  - Implemented external help alerts for failed Socratic rescues
  - Added `FLASHCARD_SYNC` task type for cloud sync recovery

- [x] **Task 15.3: Streak Tracking Feature**
  - Implemented streak tracking with calendar widget
  - Added streak API integration for tracking consecutive study days
  - Enhanced dashboard with streak visualization

- [x] **Task 15.4: UI Enhancements**
  - Added flip-back button to flashcards
  - Improved task title assignment logic in study queue
  - Enhanced sidebar menu animations
  - Added scroll progress bar to reader page
  - Optimized file search using glob for improved performance

- [x] **Task 15.5: Delta Sync & Settings**
  - Implemented delta sync for review logs
  - Added `last_synced_at` timestamp to user settings
  - Added notebook request handling and logging
  - Implemented fallback for notebook routes

## Archive / Historical Completed Sprints

<details>
<summary><b>Click to expand completed sprints (Sprint 1 - 9)</b></summary>

### Sprint 1: Queue Foundation [DONE]
**Goal:** Establish the `study_queue` schema and core task lifecycle.

**Schema Requirements:**
```sql
CREATE TABLE study_queue (
    id TEXT PRIMARY KEY,
    notebook_id TEXT NOT NULL,
    topic_id TEXT,
    task_type TEXT NOT NULL,  -- FLASHCARD_REVIEW, REREAD, QUIZ, READING, EXAMINER
    status TEXT NOT NULL,     -- PENDING, ACTIVE, COMPLETED, FAILED, SKIPPED
    priority INTEGER DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    activated_at TIMESTAMP,
    completed_at TIMESTAMP,
    payload_json TEXT,
    start_page INTEGER,
    end_page INTEGER,
    FOREIGN KEY (notebook_id) REFERENCES notebooks(id),
    FOREIGN KEY (topic_id) REFERENCES topics(id)
);
```

**Deliverables:**
- [x] `study_queue` table and indexes
- [x] Queue repository with CRUD operations
- [x] Task lifecycle state machine
- [x] Basic Wails bindings for task operations
- [x] Queue state query for Dashboard

---


### Sprint 2: Reading Flow & Page Locking [DONE]
**Goal:** Implement deterministic reading tasks with page-range locking.

**Schema Additions:**
```sql
CREATE TABLE reading_progress (
    task_id TEXT PRIMARY KEY,
    current_page INTEGER DEFAULT 0,
    last_accessed_at TIMESTAMP,
    FOREIGN KEY (task_id) REFERENCES study_queue(id)
);
```

**Deliverables:**
- [x] Reading task payload with page bounds
- [x] PDF viewer page locking (frontend)
- [x] Reading progress persistence
- [x] Completion validation (reach final page)
- [x] Quiz task auto-insertion on completion

---

### Sprint 3: Synchronous Quiz Generation [DONE]
**Goal:** Implement quiz generation as synchronous, queue-triggered flow.

**Deliverables:**
- [x] Synchronous quiz generation endpoint
- [x] Quiz task payload structure
- [x] Quiz UI with loading state
- [x] Immediate scoring and feedback
- [x] Conditional reread insertion on failure

---

### Sprint 4: Reread Remediation & Loop Protection [DONE]
**Goal:** Implement reread tasks with retry limits inside the existing quiz completion transaction to prevent infinite loops.

**Loop Protection:**
```sql
CREATE TABLE reread_attempts (
    topic_id TEXT PRIMARY KEY,
    attempt_count INTEGER NOT NULL DEFAULT 0,
    last_attempt_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
```

**Deliverables:**
- [x] `reread_attempts` table
- [x] Transactional reread attempt helpers
- [x] Max automatic reread insertion enforcement in `SubmitQuizAttempt`
- [x] Reader reuse for both `READING` and `REREAD`
- [x] Manual review recommendation UI

---

### Sprint 5: Core Foundation, Bootstrap Isolation & Settings [DONE]
**Goal:** Lock down native database queue sorting, isolate bootstrap logic, and build system configuration inputs.

**Deliverables:**
- [x] SQL sorting routing implementation
- [x] Bootstrap package `boot.go`
- [x] Updated lightweight `app.go` bridge
- [x] Settings persistence table and Wails read/write bindings
- [x] Flattened single-join database schema definitions

---

### Sprint 6: Reading, Quiz Pipelines & Deadline Pacing [DONE]
**Goal:** Build bounded reading logic, content-density quiz scaling, and expose daily study velocity.

**Deliverables:**
- [x] Backend-only page range validation safety
- [x] Scalable context-locked quiz generation
- [x] Interactive configuration screen for setting exam deadlines
- [x] Front-facing dashboard target telemetry widget

---

### Sprint 7: Memory Engine & Dashboard Synchronization [DONE]
**Goal:** Integrate type‑safe FSRS tracking and align dashboard view.

**Deliverables:**
- [x] FSRS type‑safe integration
- [x] Consistent `GetTodayPlan` logic

---

### Sprint 8: Constraint-Based Study Groups [DONE]
**Goal:** Implement multi-notebook deadline grouping and feasibility verification without autonomous AI scheduling.

**Deliverables:**
- [x] `study_groups` schema and database migrations
- [x] Feasibility verification backend logic
- [x] Updated Active Lane SQL priority multiplier
- [x] Frontend capacity monitor and warning UI

---

### Sprint 9: Socratic Tutor Routing & Milestone Examiner Gate [DONE]
**Goal:** Secure prompt handling and add the 10‑session milestone gate.

**Deliverables:**
- [x] Backend Socratic endpoint
- [x] Milestone gate implementation

</details>

---

## Architecture Foundation

This application is: **A Persistent Guided Study Queue**

NOT:
- An autonomous AI tutor
- A mission engine  
- A hidden orchestrator
- A proactive scheduler

**Core Principle:** Advanced learning systems are **Data, not Engines**.

- Quizzes create queue tasks
- FSRS creates review tasks
- Remediation creates reread tasks  
- Examiner creates assessment tasks

**None of these systems own orchestration.** SQLite is the single source of truth.

---

## Queue Model

All progression flows through: `study_queue`

**Task Lifecycle:**
```
PENDING → ACTIVE → COMPLETED
           ↓
        FAILED / SKIPPED
```

**Task Types (Priority Order):**
1. `FLASHCARD_SYNC` — Cloud sync recovery (highest)
2. `FLASHCARD_REVIEW` — Spaced repetition reviews
3. `SOCRATIC_REMEDIAL` — Socratic Rescue Lane
4. `REREAD` — Remediation tasks
5. `QUIZ` — Assessment tasks  
6. `READING` — Content consumption
7. `EXAMINER` — Mastery verification (lowest)

**Queue Ordering Rules:**
1. Task type priority (as above)
2. Notebook priority (higher = more frequent)
3. Task priority (explicit override)
4. Creation time (FIFO within tier)

---

## Technical Implementation Notes

### Queue Router

The queue router ONLY:
1. Fetches next pending task via ordering rules
2. Mounts correct module based on `task_type`
3. Marks tasks complete when module signals completion
4. Inserts follow-up tasks per completion rules

It does NOT:
- Dynamically generate agendas
- Proactively schedule sessions
- Own remediation systems
- Run hidden orchestration logic

### Dashboard Role

The Dashboard is now a deterministic task launcher. It does NOT plan missions or schedule.

---

## Terminology Guide

| Use This | NOT This |
|----------|----------|
| `study_queue` | DailyAgenda |
| Task type | Mission type |
| Queue ordering | Scheduling engine |
| Task lifecycle | Orchestration flow |
| Priority bias | Autonomous prioritization |
| Deterministic | AI-driven |
| Insert task | Generate mission |
| Activate task | Launch session |
| Complete task | Finish mission |
| FSRS algorithm | FSRS orchestrator |
| Reading task | Encoding phase |
| Quiz task | Assessment mission |
| Notebook priority | Study plan weight |

---

## Definition of Done (All Sprints)

Each sprint is complete when:

1. Schema migrations applied (if any)
2. Repository layer implemented with tests
3. Wails bindings exposed
4. Frontend UI wired (if applicable)
5. `go test ./...` passes
6. `wails dev` smoke test passes
7. No deprecated orchestration terminology in code/comments
