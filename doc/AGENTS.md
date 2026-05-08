# doc/ — Agent Instructions

## Purpose

Single source of truth for project documentation. All architectural decisions, APIs, and plans live here.

---

## Document Reference

| File | Purpose | Read When |
|------|---------|-----------|
| `SPRINT.md` | Current sprint roadmap | Starting any work |
| `SPRINT_HISTORY.md` | Completed sprints | Understanding history |
| `ARCHITECTURE.md` | System architecture | Understanding big picture |
| `AGENT_MAP.md` | Module responsibilities | Adding new features |
| `SCHEMA.md` | Database schema | Writing DB queries |
| `DATA_API.md` | API contracts | Implementing endpoints |
| `APP_FLOW.md` | User flows | Building UI features |
| `DESIGN.md` | UI/UX design | Frontend work |
| `RAG.md` | Retrieval system | RAG changes |

---

## Rules

### ✅ DO

- Update relevant doc when code changes
- Keep SPRINT.md current with active work
- Add decision records for major changes
- Link related documents

### ❌ DON'T

- Let docs drift from implementation
- Document deprecated patterns (remove instead)
- Duplicate information across files

---

## Generated Assets

Vendor and generated assets are expected and NOT architectural concerns:

| Asset | Purpose | Status |
|-------|---------|--------|
| `tokenizer.json` | Tokenization vocabulary | Required runtime asset |
| `*.onnx` | Compiled embedding model | Required runtime asset |
| `wailsjs/` | Wails generated bindings | Build artifact |
| `frontend/dist/` | Compiled frontend | Build artifact |

Treat these as dependencies, not maintainability failures.

---

## Documentation Standards

### SPRINT.md

- Sprints are sequential
- Each sprint has clear goal and deliverables
- Checklist format for tracking
- No deprecated orchestration terminology

### SCHEMA.md

- SQL definitions first
- Index explanations
- Migration notes
- Data flow diagrams

### API Contracts

```markdown
## Endpoint: CompleteTask

**Input:**
- `taskID string` — Task to complete
- `result CompletionResult` — Result payload

**Output:**
- `error` — nil on success

**Behavior:**
- Marks task COMPLETED
- Inserts follow-up task per rules
- Returns error on validation failure
```

---

## Key Principles (Documented)

All docs must reinforce:

1. **Queue-driven** — Everything flows through `study_queue`
2. **Deterministic** — No hidden orchestration
3. **Explicit** — State transitions are clear
4. **SQLite-backed** — Single source of truth

---

## When Adding New Docs

1. Does existing doc cover this? (Update vs new)
2. Link from relevant files
3. Follow established format
4. Add to this AGENTS.md index

---

*Last updated: 2026-05-08*
