# Plan Scope: Boundaries and Exclusions

## Purpose

Define explicit boundaries: what is IN scope and EXPLICITLY OUT of scope.

**Reference:** `ARCHITECTURE.md` for system design; `AGENT_MAP.md` for module responsibilities.

---

## IN Scope

### 1. Core Queue System

**IN:**
- `study_queue` table with 5 task types
- Status enum: `PENDING`, `ACTIVE`, `COMPLETED`, `SKIPPED`, `FAILED`
- Priority-based task ordering
- Task lifecycle semantics (crash recovery, timeout handling)
- SQLite as source of truth

**NOT:** Runtime-only queues, hidden state machines, in-memory task lists.

### 2. Ingestion Pipeline

**IN:**
- PDF upload and text extraction
- Chapter extraction and user pruning
- **Sliding window chunking**: 2500 words, 200-word overlap
- Automatic `READING` task insertion

**NOT:** Semantic chunking, AI-generated boundaries, autonomous orchestration.

### 3. Quiz System

**IN:**
- Synchronous quiz generation (LLM call)
- Quiz-taking interface
- Pass/fail threshold evaluation
- Remediation task insertion on fail
- Explicit generation states: `GENERATING`, `READY`, `FAILED`
- Failed quiz generation surfaces explicit error to user

**NOT:** Async generation, background jobs, forced loops, silent failures.

### 4. Flashcards & FSRS

**IN:**
- FSRS as scheduling algorithm
- Due date calculation
- `FLASHCARD_REVIEW` task insertion
- Card rating (Again/Hard/Good/Easy)

**NOT:** FSRS as queue router or session manager.

### 5. Remediation

**IN:**
- Lightweight `REREAD` task insertion
- AI-generated feedback on failed quizzes
- User can complete OR skip remediation
- Reread loop protection (max 3 attempts default)
- Auditable skip states

**NOT:** Forced loops, user traps, mandatory repetition.

### 6. Examiner Mode

**IN:**
- Written assessment tasks
- User-triggered after mastery thresholds
- Queue-driven appearance (tier 5 priority)
- Optional (user can skip)

**NOT:** Autonomous triggering, background generation, task starvation.

### 7. Queue Router

**IN:**
- Fetch next pending task (with deterministic ordering rules)
- Mount correct module
- Mark tasks complete/skipped/failed
- Insert follow-up tasks per explicit rules
- Task lifecycle management (ACTIVE → terminal states)
- Crash recovery (timeout stale ACTIVE tasks)

**NOT:** Proactive scheduling, event buses, workflow builders, background mutation.

### 8. Multi-Notebook Support

**IN:**
- Multiple notebooks with deterministic priority biasing
- Notebook `priority` field (1-10, default 5)
- Higher priority notebooks surface more frequently
- Lower priority notebooks still eventually appear
- Queue ordering with notebook bias

**NOT:** AI-driven scheduling, velocity orchestration, autonomous switching.

### 9. Dashboard Starvation Protection

**IN:**
- Deterministic balancing rule: after N reviews, allow 1 reading
- Default: after 5 review tasks, surface 1 READING task
- Query-time bias (NOT autonomous orchestration)
- Prevents review monopolization

**NOT:** Autonomous balancing, AI-driven pacing.

**Balancing rules are static SQL ordering constraints, not adaptive runtime systems.**

### 10. RAG / Ask AI

**IN:**
- Topic-scoped retrieval
- Single-turn stateless requests
- Sliding window chunk retrieval

**NOT:** Semantic retrieval, cross-topic search, conversation memory.

### 9. Synchronous Generation

**IN:**
- All LLM calls are synchronous
- Loading spinners during generation
- Immediate response with content

**NOT:** Background workers, async queues, proactive generation.

---

## EXPLICITLY OUT of Scope

### Architecture Patterns (DO NOT ADD)

| Pattern | Status | Reason |
|---------|--------|--------|
| Proactive orchestration | OUT | Use queue query instead |
| Hidden scheduling systems | OUT | SQLite queue is visible |
| Autonomous AI pipelines | OUT | Synchronous calls only |
| Dual timer engines | OUT | Single queue source |
| Event buses | OUT | Direct API calls |
| Workflow builders | OUT | Fixed queue types |
| Drag-drop orchestration | OUT | Static queue flow |
| Runtime-only state | OUT | Persistent SQLite |
| Async background jobs | OUT | Synchronous MVP |
| Multi-step agents | OUT | Stateless single-turn |
| LangChain | OUT | Explicit architecture |

### Features (DO NOT ADD)

| Feature | Status | Reason |
|---------|--------|--------|
| Semantic chunking | OUT | Sliding window is sufficient |
| AI chunk boundaries | OUT | Deterministic boundaries |
| Syllabus graphing | OUT | Overkill for MVP |
| Multi-device sync | OUT | Local-first MVP |
| Cloud backup | OUT | Phase 2 consideration |
| Social features | OUT | Single-user focus |
| Gamification | OUT | Queue simplicity |
| Advanced analytics | OUT | SQLite queries suffice |
| Plugin system | OUT | Fixed modules |
| Theme customization | OUT | Single design system |
| AI-driven scheduling | OUT | Deterministic bias only |
| Velocity orchestration | OUT | Query-driven only |
| Hidden balancing logic | OUT | Explicit rules only |
| Reading surveillance | OUT | No timers/tracking |
| Engagement tracking | OUT | Lightweight validation only |

---

## Scope Boundaries

### Queue as Source of Truth

All flows go through `study_queue`. See `ARCHITECTURE.md` Section 4 for data model.

### Stateless Modules

Modules render content for `block_id`; they do not route or schedule.

### SQLite as State Machine

State is queryable SQL, not in-memory code. See `ARCHITECTURE.md` Section 10 for state transition semantics.

---

## Decision Log

### Why Sliding Window?

**Decision:** Use sliding window chunking (2500 words, 200 overlap)

**Rationale:**
- Deterministic and inspectable
- No AI dependency for boundaries
- Sufficient for MVP
- Easy to debug

**Rejected:**
- Semantic chunking (too complex)
- AI boundaries (non-deterministic)
- Topic modeling (overkill)

### Why Synchronous Generation?

**Decision:** All LLM calls are synchronous

**Rationale:**
- Deterministic MVP > premature optimization
- No background worker complexity
- User sees immediate feedback
- Easier to debug

**Rejected:**
- Async job queues
- Background workers
- Event-driven architecture

### Why Persistent Queue?

**Decision:** SQLite `study_queue` drives all flows

**Rationale:**
- Resumable across app restarts
- Queryable and debuggable
- No runtime-only state
- Simple and explicit

**Rejected:**
- Runtime task lists
- Hidden queue routers
- In-memory queues
- Complex state machines

---

## Success Criteria

The architecture is correct if:

1. All user flows start from `study_queue` query
2. No runtime-only queues exist
3. All state transitions are explicit SQL updates
4. Modules have no orchestration logic
5. Quiz generation is synchronous with loading spinner
6. Remediation is optional (user can skip)
7. FSRS only schedules, does not orchestrate
8. Dashboard only shows pending tasks
9. No hidden state machines
10. SQLite is source of truth
