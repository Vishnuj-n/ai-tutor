# AI Tutor — Agent Instructions

## Project Overview

A **Persistent Guided Study Queue** application built with Go (Wails backend) and Vue 3 (frontend).

**NOT:** An autonomous AI tutor, mission engine, hidden orchestrator, or proactive scheduler.

---

## Quick Reference

| Task | Reference |
|------|-----------|
| Architecture | `doc/ARCHITECTURE.md` |
| Current Sprint | `doc/SPRINT.md` |
| Database Schema | `doc/SCHEMA.md` |
| API Contracts | `doc/DATA_API.md` |
| Module Responsibilities | `doc/AGENT_MAP.md` |
| App Flow | `doc/APP_FLOW.md` |

---

## Core Architecture Principles

### 1. Queue-Guided Progression

The queue drives deterministic progression. Manual and exploratory study entry points are valid, but they must reuse the same canonical initialization, retrieval, and notebook/topic ownership semantics.

Task lifecycle:
```
PENDING → ACTIVE → COMPLETED
           ↓
        FAILED / SKIPPED
```

### 2. Data, Not Engines

Learning systems create tasks — they don't orchestrate:
- Quizzes create QUIZ tasks
- FSRS creates FLASHCARD_REVIEW tasks  
- Remediation creates REREAD tasks
- Examiner creates ASSESSMENT tasks

### 3. SQLite is Source of Truth

No hidden state machines. No in-memory orchestration. All state in database.

### 4. Deterministic Ordering

Priority hierarchy (in order):
1. Task type: FLASHCARD_REVIEW > REREAD > QUIZ > READING > EXAMINER
2. Notebook priority (higher = more frequent)
3. Task priority (within same task type only)
4. Creation time (FIFO)

---

## Architecture Invariants

These must NEVER be violated:

1. **Queue controls progression** — No side channels, no direct module-to-module signaling for lifecycle flow
2. **SQLite is source of truth** — No in-memory state machines, no hidden orchestration
3. **Queue mutations are explicit** — Every state change is a database write with clear audit trail
4. **No hidden orchestration state** — No event buses, no background schedulers, no autonomous flows
5. **Frontend does not own business logic** — UI is thin; all decisions happen in Go backend
6. **FSRS creates tasks, not flow control** — FSRS is scheduling algorithm only; it inserts FLASHCARD_REVIEW tasks
7. **RAG retrieves context only** — RAG does not control task progression or make decisions
8. **Queue ordering is deterministic** — Same inputs always produce same task order; no AI-driven prioritization
9. **No background queue mutation** — Queue changes only happen via explicit user action or task completion trigger (no daemons, no auto-inserters, no startup rebalance jobs)

---

## Terminology (Use This → NOT This)

| Correct | Deprecated |
|---------|------------|
| `study_queue` | DailyAgenda |
| Task type | Mission type |
| Queue ordering | Scheduling engine |
| Queue controller | Orchestrator |
| Task lifecycle | Orchestration flow |
| Priority bias | Autonomous prioritization |
| Deterministic | AI-driven |
| Insert task | Generate mission |
| Activate task | Launch session |
| Complete task | Finish mission |
| FSRS algorithm | FSRS orchestrator |
| Reading task | Encoding phase |

---

## Rules — What NOT To Do

### ❌ Never Add

- Hidden state machines
- Proactive scheduling logic
- Autonomous mission generation
- Context-locked routing
- Dual timers (reading + idle tracking)
- Event buses for orchestration
- AI-generated chunk boundaries
- Semantic topic chunking
- Engagement surveillance (timers, scroll tracking)
- Behavioral tracking beyond completion validation
- Background queue mutation (auto-inserters, daemon sync loops, startup rebalance jobs)

### ❌ Never Say

- "Orchestrator schedules..."
- "Mission engine generates..."
- "Daily agenda creates..."
- "Autonomous flow manages..."
- "Hidden scheduler triggers..."

### ✅ Always Do

- Query `study_queue` for next task
- Keep manual and queue entry points on the same canonical init path
- Mark tasks with explicit status transitions
- Persist all state to SQLite immediately
- Validate reading completion at final page only
- Use synchronous LLM calls with loading states
- Return explicit errors (no silent failures)

---

## File Organization

```
├── internal/
│   ├── db/          # SQLite repositories (query + persistence)
│   ├── models/      # Go structs (NO business logic)
│   ├── llm/         # LLM provider adapter
│   ├── rag/         # Retrieval pipeline
│   ├── notebook/    # Upload + ingestion
│   └── study/       # Study session logic
├── frontend/
│   ├── src/pages/   # Vue pages (thin, state in backend)
│   └── src/services/# API bridge
└── doc/             # All documentation
```

---

## Code Style

### Go

- Repository pattern for all DB access
- Pointers only when modifying data
- Avoid unnecessary interfaces
- `go test ./...` must pass
- No CGO in Windows builds (use `extension_nocgo.go`)

### Vue

- Pages are thin — state lives in backend
- Pinia only for ephemeral UI state
- Wails bindings in `src/services/`
- No direct DB access from frontend

---

## Testing

- Unit tests for repositories
- Integration tests for DB operations
- Contract tests for Wails bindings
- Smoke test: `wails dev` loads without errors

---

## When in Doubt

1. Check `doc/SPRINT.md` for current sprint scope
2. Check `doc/AGENT_MAP.md` for module boundaries
3. Ask: "Is this deterministic queue behavior or hidden orchestration?"
4. Prefer explicit state over implicit flows

---

*Last updated: 2026-05-08*
