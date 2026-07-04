# Plan Scope: Boundaries and Exclusions

**Reference:** `ARCHITECTURE.md` for system design; `AGENT_MAP.md` for module responsibilities.

---

## IN Scope

### 1. Core Queue System
- `study_queue` table with 5+ task types
- Status: `PENDING`, `ACTIVE`, `COMPLETED`, `SKIPPED`, `FAILED`
- Priority-based ordering, crash recovery, timeout handling
- SQLite = source of truth
- **NOT:** runtime-only queues, hidden state machines

### 2. Ingestion Pipeline
- PDF upload, text extraction, chapter extraction + pruning
- **Sliding window chunking**: 2500 words, 200 overlap
- Auto `READING` task insertion
- **NOT:** semantic chunking, AI boundaries, autonomous orchestration

### 3. Quiz System
- Sync quiz generation, quiz-taking, pass/fail evaluation
- Remediation on fail, generation states: `GENERATING`, `READY`, `FAILED`
- Explicit errors surfaced to user
- **NOT:** async generation, background jobs, forced loops

### 4. Flashcards & FSRS
- FSRS as scheduling algorithm, due date calc
- `FLASHCARD_REVIEW` task insertion, card ratings (Again/Hard/Good/Easy)
- **NOT:** FSRS as queue router or session manager

### 5. Remediation
- `REREAD` task insertion, AI feedback on failures
- User can complete or skip; max reread attempts enforced
- Auditable skip states
- **NOT:** forced loops, user traps

### 6. Examiner Mode
- Written assessments, user-triggered after mastery
- Queue-driven (tier 5 priority), optional
- **NOT:** autonomous triggering, task starvation

### 7. Queue Router
- Fetch next task (deterministic ordering), mount module
- Mark complete/skip/fail, insert follow-ups
- Crash recovery (timeout stale ACTIVE tasks)
- **NOT:** proactive scheduling, event buses, background mutation

### 8. Multi-Notebook Support
- Priority biasing (1-10, default 5)
- Higher priority = more frequent, lower = still appears
- **NOT:** AI-driven scheduling, velocity orchestration

### 9. Dashboard Starvation Protection
- After 5 reviews → surface 1 reading (query-time bias)
- **NOT:** autonomous balancing

### 10. RAG / Ask AI
- Topic-scoped retrieval, single-turn stateless, sliding window chunks
- **NOT:** semantic retrieval, cross-topic search, conversation memory

### 11. Synchronous Generation
- All LLM calls synchronous, loading spinners, immediate response
- **NOT:** background workers, async queues

---

## EXPLICITLY OUT of Scope

### Architecture Patterns

| Pattern | Status | Reason |
|---------|--------|--------|
| Proactive orchestration | OUT | Use queue query |
| Hidden scheduling | OUT | SQLite queue is visible |
| Autonomous AI pipelines | OUT | Sync calls only |
| Dual timer engines | OUT | Single queue source |
| Event buses | OUT | Direct API calls |
| Async background jobs | OUT | Sync MVP |
| Multi-step agents | OUT | Stateless single-turn |
| LangChain | OUT | Explicit architecture |

### Features

| Feature | Status | Reason |
|---------|--------|--------|
| Semantic chunking | OUT | Sliding window sufficient |
| AI chunk boundaries | OUT | Deterministic |
| Syllabus graphing | OUT | Overkill for MVP |
| Multi-device sync | OUT | Local-first |
| Social features | OUT | Single-user |
| Gamification | OUT | Queue simplicity |
| Plugin system | OUT | Fixed modules |
| AI scheduling | OUT | Deterministic bias only |
| Reading surveillance | OUT | No timers/tracking |

---

## Decision Log

**Why Sliding Window?** Deterministic, inspectable, no AI dependency, sufficient for MVP.

**Why Synchronous?** Deterministic MVP > premature optimization. No background complexity, immediate feedback, easy debugging.

**Why Persistent Queue?** Resumable across restarts, queryable, no runtime-only state.

---

## Success Criteria

1. All flows start from `study_queue` query
2. No runtime-only queues
3. All state transitions = explicit SQL updates
4. No orchestration logic in modules
5. Quiz generation synchronous with spinner
6. Remediation optional (user can skip)
7. FSRS only schedules, no orchestration
8. Dashboard only shows pending tasks
9. No hidden state machines
10. SQLite = source of truth
