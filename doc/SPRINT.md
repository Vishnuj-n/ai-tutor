# SPRINT.md — AI Tutor

**Status:** Active roadmap for Persistent Queue Architecture  
**Last Updated:** 2026-05-08  
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

### Sprint 1: Queue Foundation

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

### Sprint 2: Reading Flow & Page Locking

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

### Sprint 3: Synchronous Quiz Generation

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

### Sprint 4: Reread Remediation & Loop Protection

**Goal:** Implement reread tasks with retry limits to prevent infinite loops.

**Required Context:**

- **Documentation:** SCHEMA.md, DATA_API.md
- **Agent Instructions:** /AGENTS.md, /internal/AGENTS.md
- **Relevant Packages:** internal/db/
- **Important Constraints:** Max reread attempts must be enforced, queue progression must continue after max failures

**Reread Flow:**
1. Quiz score below threshold
2. REREAD task inserted for same content
3. User completes reread
4. New QUIZ task generated
5. If still failing after max attempts → stop automatic insertion

**Loop Protection:**
```sql
-- Track reread attempts per topic
CREATE TABLE reread_attempts (
    topic_id TEXT PRIMARY KEY,
    attempt_count INTEGER DEFAULT 0,
    last_attempt_at TIMESTAMP
);
```

**Config:**
- `max_reread_attempts = 3`

**After Max Failures:**
- Task marked COMPLETED (no further auto-remediation)
- Manual review recommended to user
- Queue progression continues with next task

**API Surface:**
- `InsertRereadTask(notebookID, topicID string, reason string) error`
- `CheckRereadLimit(topicID string) (attempts int, allowed bool)`

**Deliverables:**
- [ ] Reread task type and payload
- [ ] Reread attempt tracking table
- [ ] Max attempt enforcement
- [ ] Manual review recommendation UI
- [ ] Queue progression after max failures

---

### Sprint 5: Flashcard Review Tasks

**Goal:** Integrate FSRS with queue — due cards become review tasks.

**Required Context:**

- **Documentation:** SCHEMA.md, DATA_API.md, AGENT_MAP.md
- **Agent Instructions:** /AGENTS.md, /internal/AGENTS.md
- **Relevant Packages:** internal/db/, internal/study/
- **Important Constraints:** FSRS is scheduling algorithm only, not orchestrator; one task per review session not per card

**FSRS Role Clarification:**
- FSRS is ONLY: scheduling algorithm + interval calculator
- FSRS is NOT: orchestrator, mission engine, hidden scheduler

**Review Task Model:**
- One `FLASHCARD_REVIEW` task = one review session
- NOT one task per card (prevents queue explosion)
- Task payload contains list of due cards for the session

**Daily Flow:**
1. On dashboard load or explicit refresh: Query `fsrs_cards` for due cards
2. Group by notebook, create `FLASHCARD_REVIEW` tasks
3. Tasks enter queue at highest priority
4. User activates task → review session begins
5. Each card rating updates FSRS state
6. Session complete → mark task COMPLETED

**API Surface:**
- `GenerateReviewTasks(notebookID string) ([]StudyTask, error)` — Create tasks for due cards
- `GetReviewSession(taskID string) ReviewSession` — Get cards for this session
- `RecordCardReview(taskID, cardID string, rating int) error` — Update FSRS state
- `CompleteReviewSession(taskID string) error` — Mark task done

**Schema:**
```sql
-- Link review tasks to cards being reviewed
CREATE TABLE review_task_cards (
    task_id TEXT,
    card_id TEXT,
    status TEXT DEFAULT 'pending', -- pending, reviewed
    PRIMARY KEY (task_id, card_id)
);
```

**Rules:**
- One session task can contain 10-20 cards (configurable)
- Cards due together are batched into same session
- Queue priority ensures review happens before new reading

**Deliverables:**
- [ ] FLASHCARD_REVIEW task type
- [ ] Due card query and batching
- [ ] Review session payload structure
- [ ] FSRS rating integration (existing code)
- [ ] Session completion flow

---

### Sprint 6: Examiner Tasks & Mastery Triggers

**Goal:** Implement Examiner mode as queue-driven optional tasks.

**Required Context:**

- **Documentation:** SCHEMA.md, DATA_API.md, AGENT_MAP.md
- **Agent Instructions:** /AGENTS.md, /internal/AGENTS.md
- **Relevant Packages:** internal/db/, internal/assessment/
- **Important Constraints:** No hidden examiner orchestration, tasks are optional and queue-driven

**Examiner Tasks:**
- Inserted after mastery thresholds (e.g., 3 quizzes passed at 90%+)
- Appear naturally in queue at priority 5 (lowest)
- Optional — user can skip without penalty
- NOT interrupting, NOT autonomous

**Mastery Detection:**
```sql
-- Simple threshold-based trigger
SELECT topic_id, COUNT(*) as passed_count
FROM user_answers ua
JOIN questions q ON ua.question_id = q.id
WHERE ua.score >= 90
GROUP BY q.topic_id
HAVING passed_count >= 3;
```

**Examiner Task Payload:**
```json
{
  "written_question_ids": ["wq_...", "wq_..."],
  "triggered_by": "mastery_threshold",
  "optional": true
}
```

**API Surface:**
- `CheckMasteryTriggers(notebookID string) []MasteryTrigger` — Detect thresholds
- `InsertExaminerTask(notebookID, topicID string) error`
- `GetWrittenQuestions(taskID string) []WrittenQuestion`
- `SubmitWrittenAnswer(taskID, questionID, answer string) WrittenScore`

**Rules:**
- NO hidden examiner orchestration
- NO autonomous examiner flows
- Tasks are optional, queue-driven, user-initiated

**Deliverables:**
- [ ] Examiner task type and payload
- [ ] Mastery threshold detection
- [ ] Optional task handling (skip allowed)
- [ ] Written question integration
- [ ] Queue-driven examiner flow

---

### Sprint 7: Queue Balancing & Polish

**Goal:** Ensure fair queue distribution and recovery robustness.

**Required Context:**

- **Documentation:** SCHEMA.md, DATA_API.md
- **Agent Instructions:** /AGENTS.md, /internal/AGENTS.md
- **Relevant Packages:** internal/db/, internal/study/
- **Important Constraints:** Queue ordering must remain deterministic, no background queue mutation daemons

**Queue Balancing:**

1. **Starvation Prevention**
   - Lower priority notebooks get minimum quota
   - Config: `min_tasks_per_notebook_per_day = 2`

2. **Priority Decay (Query-Time Only)**
   - Old PENDING tasks get higher priority in ordering calculation
   - Implemented as SQL ORDER BY logic, NOT background mutation
   - Prevents infinite deferral while remaining deterministic

3. **Session Boundaries**
   - Configurable max tasks per session: `max_session_tasks = 10`
   - Soft limit — user can continue if desired

**Crash Recovery:**

1. **ACTIVE Task Handling**
   - On startup: Mark stale ACTIVE tasks back to PENDING
   - Stale threshold: `task_active_timeout = 24 hours`

2. **Reading Progress Recovery**
   - `reading_progress` table preserves cursor
   - User resumes at last page on restart

3. **Quiz Generation Idempotency**
   - Quiz generation keyed by (task_id, attempt_num)
   - Re-generation on crash produces identical questions

**API Surface:**
- `ApplyQueueOrderingRules(notebookID string) error` — Apply priority adjustments (query-time only)
- `RecoverStaleTasks() error` — Mark timed-out ACTIVE tasks
- `GetQueueStats() QueueStats` — Per-notebook pending counts

**Monitoring:**
```sql
-- Health check queries
SELECT notebook_id, task_type, status, COUNT(*) 
FROM study_queue 
GROUP BY notebook_id, task_type, status;
```

**Deliverables:**
- [ ] Starvation prevention logic
- [ ] Priority decay for old tasks
- [ ] Session task limits
- [ ] Stale task recovery
- [ ] Queue health monitoring
- [ ] Crash-resilient reading progress

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

## Current Status

- **Sprint 1:** Not started — Queue schema design complete
- **Sprint 2-7:** Planned, pending Sprint 1 completion

---

*For historical sprints (pre-queue architecture), see `doc/SPRINT_HISTORY.md`.*
