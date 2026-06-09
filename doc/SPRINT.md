# SPRINT.md — AI Tutor

**Status:** Active roadmap for Persistent Queue Architecture  
**Last Updated:** 2026-06-05  
**Architecture:** SQLite-backed deterministic queue (NOT autonomous orchestration)

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

## Sprint Implementation Rule

Each sprint implementation must:

1. Read only the directly relevant documentation
2. Respect AGENTS.md hierarchy
3. Avoid introducing new architecture patterns
4. Preserve deterministic queue behavior
5. Prefer explicit state transitions over hidden automation

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
2. `REREAD` — Remediation tasks
3. `QUIZ` — Assessment tasks  
4. `READING` — Content consumption
5. `EXAMINER` — Mastery verification

**Queue Ordering Rules:**
1. Task type priority (as above)
2. Notebook priority (higher = more frequent)
3. Task priority (explicit override)
4. Creation time (FIFO within tier)

**Note:** Ordering is evaluated deterministically at query time, not via background mutation.

**Notebook Priority Biasing:**
- Higher priority notebooks appear more frequently in queue ordering
- Lower priority notebooks still surface (starvation prevention)
- Priority is deterministic ordering bias, NOT autonomous scheduling

---

## Sprint Roadmap

---

### Sprint 1: Queue Foundation [DONE]

**Goal:** Establish the `study_queue` schema and core task lifecycle.

**Required Context:**

- **Documentation:** SCHEMA.md, DATA_API.md, ARCHITECTURE.md
- **Agent Instructions:** /AGENTS.md
- **Relevant Packages:** internal/db/
- **Important Constraints:** No hidden queue mutation, queue ordering must remain deterministic

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
    -- Task-specific payload (JSON)
    payload_json TEXT,
    -- For reading tasks: page bounds
    start_page INTEGER,
    end_page INTEGER,
    -- Foreign keys
    FOREIGN KEY (notebook_id) REFERENCES notebooks(id),
    FOREIGN KEY (topic_id) REFERENCES topics(id)
);

CREATE INDEX idx_study_queue_status_priority_created 
    ON study_queue(status, priority, created_at);
CREATE INDEX idx_study_queue_notebook_status 
    ON study_queue(notebook_id, status);
```

**API Surface:**
- `GetNextTask(notebookID string) StudyTask` — Fetch next pending task by ordering rules
- `ActivateTask(taskID string) error` — Move PENDING → ACTIVE
- `CompleteTask(taskID string, result CompletionResult) error` — Move ACTIVE → COMPLETED, trigger follow-up insertion
- `SkipTask(taskID string) error` — Move to SKIPPED (user-initiated)
- `GetQueueState(notebookID string) QueueState` — Pending count by task type

**Deliverables:**
- [ ] `study_queue` table and indexes
- [ ] Queue repository with CRUD operations
- [ ] Task lifecycle state machine
- [ ] Basic Wails bindings for task operations
- [ ] Queue state query for Dashboard

---

### Sprint 2: Reading Flow & Page Locking [DONE]

**Goal:** Implement deterministic reading tasks with page-range locking.

**Required Context:**

- **Documentation:** SCHEMA.md, APP_FLOW.md
- **Agent Instructions:** /AGENTS.md, /internal/AGENTS.md
- **Relevant Packages:** internal/db/, frontend/src/pages/
- **Important Constraints:** No engagement surveillance, reading completion only requires reaching final page

**Reading Task Flow:**
1. User opens reading task from queue
2. PDF viewer locked to assigned page range (`start_page` to `end_page`)
3. User navigates within bounds
4. On reaching `end_page`, completion button activates
5. User clicks Complete → QUIZ task inserted

**API Surface:**
- `GetReadingTask(taskID string) ReadingTask` — Get task with page bounds
- `ValidateReadingCompletion(taskID string, finalPage int) bool` — Verify user reached end page
- `CompleteReading(taskID string) error` — Complete task, trigger quiz insertion

**Schema Additions:**
```sql
-- reading_progress tracks per-task progress (restart-safe)
CREATE TABLE reading_progress (
    task_id TEXT PRIMARY KEY,
    current_page INTEGER DEFAULT 0,
    last_accessed_at TIMESTAMP,
    FOREIGN KEY (task_id) REFERENCES study_queue(id)
);
```

**Rules:**
- NO engagement surveillance (no timers, no scroll tracking)
- Completion requires reaching `end_page` — nothing else
- PDF locked to assigned range — user cannot read ahead

**Deliverables:**
- [ ] Reading task payload with page bounds
- [ ] PDF viewer page locking (frontend)
- [ ] Reading progress persistence
- [ ] Completion validation (reach final page)
- [ ] Quiz task auto-insertion on completion

---

### Sprint 3: Synchronous Quiz Generation [DONE]

**Goal:** Implement quiz generation as synchronous, queue-triggered flow.

**Required Context:**

- **Documentation:** SCHEMA.md, DATA_API.md, AGENT_MAP.md
- **Agent Instructions:** /AGENTS.md, /internal/AGENTS.md
- **Relevant Packages:** internal/db/, internal/llm/, frontend/src/pages/
- **Important Constraints:** Synchronous generation only, no background quiz generation

**Quiz Flow:**
1. User completes reading task
2. Frontend shows loading spinner
3. Backend synchronously calls LLM for question generation
4. QUIZ task created with generated questions in payload
5. User proceeds to quiz UI
6. User submits answers → scored immediately

**API Surface:**
- `GenerateQuizSync(topicID string, chunkIDs []string) (QuizTask, error)` — Synchronous generation
- `SubmitQuizAttempt(taskID string, answers []Answer) QuizResult` — Score and record

**Quiz Task Payload:**
```json
{
  "questions": [
    {
      "id": "q_...",
      "prompt": "What is...",
      "options": ["A", "B", "C", "D"],
      "correct_answer": "B",
      "source_chunk_id": "chk_..."
    }
  ],
  "passing_score": 70
}
```

**Scoring Outcomes:**
- Score >= threshold → Mark COMPLETED, optionally insert FLASHCARD_REVIEW
- Score < threshold → Insert REREAD task, generate lightweight AI feedback

**Rules:**
- Synchronous generation — queue waits, user sees spinner
- NO background/async quiz generation
- Questions stored in task payload (ephemeral, not persisted to questions table until scored)

**Deliverables:**
- [ ] Synchronous quiz generation endpoint
- [ ] Quiz task payload structure
- [ ] Quiz UI with loading state
- [ ] Immediate scoring and feedback
- [ ] Conditional reread insertion on failure

---

### Sprint 4: Reread Remediation & Loop Protection [DONE]

**Goal:** Implement reread tasks with retry limits inside the existing quiz completion transaction to prevent infinite loops.

**Required Context:**

- **Documentation:** SCHEMA.md, DATA_API.md
- **Agent Instructions:** /AGENTS.md, /internal/AGENTS.md
- **Relevant Packages:** internal/db/
- **Important Constraints:** Max reread attempts must be enforced, queue progression must continue after max failures

**Reread Flow:**
1. Active quiz score below threshold
2. Increment `reread_attempts` for the quiz `topic_id` in the open quiz transaction
3. If the resulting count is `1..3`, insert exactly one `REREAD` task for the same content
4. User completes reread through the existing `/reader` flow
5. Reader completion inserts a new `QUIZ` follow-up through the existing queue path
6. If the resulting count is `4+`, stop automatic insertion and return a manual-review recommendation

**Loop Protection:**
```sql
CREATE TABLE reread_attempts (
    topic_id TEXT PRIMARY KEY,
    attempt_count INTEGER NOT NULL DEFAULT 0,
    last_attempt_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
```

**Exact Semantics:**
- Maximum automatic reread insertions = `3`
- Attempts are tracked per `topic_id`
- Successful quiz completion resets the attempt count to `0`
- Duplicate or stale quiz submissions do not create extra rereads because only an `ACTIVE` quiz task can complete

**After Max Automatic Insertions Are Exhausted:**
- Quiz task is marked `COMPLETED` with no further auto-remediation
- Manual review is recommended in the quiz result only
- Queue progression continues with the next pending task

**Repository Surface:**
- Read reread attempt count by `topic_id`
- Increment attempt count transactionally
- Reset attempt count transactionally on quiz pass

**Deliverables:**
- [ ] `reread_attempts` table
- [ ] Transactional reread attempt helpers
- [ ] Max automatic reread insertion enforcement in `SubmitQuizAttempt`
- [ ] Reader reuse for both `READING` and `REREAD`
- [ ] Manual review recommendation UI

---

## Sprint 5: Core Foundation, Bootstrap Isolation & Settings [DONE]

**Goal:** Lock down native database queue sorting, isolate bootstrap logic, and build system configuration inputs.

**Tasks:**
- **Task 5.1**: Implement Native SQL Desktop Queue Routing (`internal/db/study_queue_repo.go`). Write the deterministic `GetAllPendingTasks()` query using Notebook Priority biasing to handle macro-interleaving.
- **Task 5.2**: Deconstruct `app.go` God-File Setup Block. Move initialization, asset validation, and path resolvers to `internal/runtime/boot.go`.
- **Task 5.3**: Expose System Configuration Endpoints. Create a settings persistence layer in SQLite to track `daily_study_minutes` and a user-configured `exam_target_date` timestamp.
- **Task 5.4**: Collapse Middle-Tier Schema. Permanently delete the `parents` table and update `chunks` to reference `topic_id` directly for streamlined local RAG joins.

**Deliverables:**
- [x] SQL sorting routing implementation
- [x] Bootstrap package `boot.go`
- [x] Updated lightweight `app.go` bridge
- [x] Settings persistence table and Wails read/write bindings
- [x] Flattened single-join database schema definitions

---

## Sprint 6: Reading, Quiz Pipelines & Deadline Pacing (Priority: Medium-High)

**Goal:** Build bounded reading logic, content-density quiz scaling, and expose daily study velocity.

**Tasks:**
- **Task 6.1**: Enforce Backend Context Locking (`internal/study/service.go`). Pull text chunks strictly by assigned page bounds during quiz generation, removing frontend view restriction dependencies.
- **Task 6.2**: Implement Density-Scaled Quiz Quantities (`internal/study/reader.go`). Globalize token capacities and calculate target question volume dynamically using a words-per-question density script.
- **Task 6.3**: Wire the Deadline Velocity UI. Create a backend utility to run the target formula (`Remaining Words / Days to Exam Target`) and render the resulting required daily pace metric on the main dashboard workspace.

**Deliverables:**
- [ ] Backend-only page range validation safety
- [ ] Scalable context-locked quiz generation
- [ ] Interactive configuration screen for setting exam deadlines
- [ ] Front-facing dashboard target telemetry widget

---

## Sprint 7: Memory Engine & Dashboard Synchronization (Priority: Medium)

**Goal:** Integrate type‑safe FSRS tracking and align dashboard view.

**Tasks:**
- **Task 7.1**: Strict Typecasting Mapping for `go-fsrs/v4` (`internal/study/review_session.go`). Convert `ElapsedDays` to `ElapsedTime (uint64)`.
- **Task 7.2**: Resolve Dual‑Path Split‑Brain in `GetTodayPlan` (`app.go`). Use `study_queue` as single source of truth; scheduler only aggregates review metrics.

**Deliverables:**
- [ ] FSRS type‑safe integration
- [ ] Consistent `GetTodayPlan` logic

---

## Sprint 8: Socratic Tutor Routing & Milestone Examiner Gate (Priority: Low / Good‑to‑Have)

**Goal:** Secure prompt handling and add the 10‑session milestone gate.

**Tasks:**
- **Task 8.1**: Move Socratic Prompt Engineering to Backend (`app.go` & `Socratic.vue`). Add `AskSocraticAI` endpoint.
- **Task 8.2**: Implement Milestone Examiner Gate (`internal/study/service.go`). Use SQLite count query; on multiples of 10 create an `EXAMINER` task.

**Deliverables:**
- [ ] Backend Socratic endpoint
- [ ] Milestone gate implementation

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

### Dashboard Role (Revised)

The Dashboard is now:
- A deterministic task launcher

It is NOT:
- A mission planner
- A scheduling engine
- An AI agenda system

Dashboard simply:
1. Fetches next queue task
2. Displays queue state (counts by type)
3. Launches task modules on user action

### Ingestion Pipeline (Retained)

Current pipeline remains:
- PDF upload → chapter extraction → chapter pruning

Chunking strategy:
- 2500-word chunks
- 200-word overlap

**Explicitly removed:**
- Semantic topic chunking
- AI-generated chunk boundaries
- Autonomous chunk planning

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

---

*For historical sprints (pre-queue architecture), see `doc/SPRINT_HISTORY.md`.*
