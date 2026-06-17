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

### Sprint 11: 3-Strike Socratic Rescue Pipeline
**Goal:** Implement cognitive damage control via database clean slate, queue interleaving, and split-screen tutor layout.

- [ ] **Task 11.1: Database Intervention & Trigger**
  - Track consecutive quiz failures per chunk/topic.
  - On the 3rd strike, wipe or suspend all flashcards associated with that chunk to prevent FSRS ease hell.
  - Unblock the main reading timeline by marking the blocking reading task as `COMPLETED`.
- [ ] **Task 11.2: Queue Interleaving**
  - Generate a `SOCRATIC_REMEDIAL` task.
  - Place it in a specialized asynchronous bucket/lane on the dashboard instead of blocking the main linear progression.
- [ ] **Task 11.3: Dual-Lane Breakdown View (UI)**
  - Create a split-pane layout:
    - **Left Lane:** Local Socratic chat interface with raw text and local LLM acting as a strict tutor (leading questions only).
    - **Right Lane:** Fallback card that copies raw text and pre-engineered expert prompt template to the clipboard for external premium LLM use.
- [ ] **Task 11.4: Dev Mode Bypass Panel**
  - Add a floating developer panel enabled only in Vite Dev Mode (`import.meta.env.DEV`).
  - Implement a *"Force 3-Strike Rescue UI State"* trigger which mocks the backend state to test the UI instantly.
- [ ] **Task 11.5: Isolated Automated Tests**
  - Write isolated Go tests validating that the 3-strike trigger deletes flashcards, unblocks the reading task, and inserts the `SOCRATIC_REMEDIAL` task.

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

### SPRITN 13 User Asset Provisioning

- Detect missing RAG assets
- Download assets from GitHub Releases
- Show progress UI
- Verify hashes
- Resume failed downloads
- Allow manual asset location

Refer doc\future_plan\cross_platform_asset_delivery.md

- Added in onboarding and settings

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
1. `FLASHCARD_REVIEW` — Highest priority
2. `SOCRATIC_REMEDIAL` — Socratic Rescue Lane
3. `REREAD` — Remediation tasks
4. `QUIZ` — Assessment tasks  
5. `READING` — Content consumption
6. `EXAMINER` — Mastery verification

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
