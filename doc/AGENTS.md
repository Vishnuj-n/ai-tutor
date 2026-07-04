# doc/ — Agent Instructions

Single source of truth for all project documentation.

## Document Reference

| File | Purpose | Read When |
|------|---------|-----------|
| `SPRINT.md` | Current sprint roadmap | Starting any work |
| `SPRINT_HISTORY.md` | Completed sprints | Understanding history |
| `ARCHITECTURE.md` | System architecture | Big picture |
| `AGENT_MAP.md` | Module responsibilities | Adding features |
| `SCHEMA.md` | Database schema | Writing DB queries |
| `DATA_API.md` | API contracts | Implementing endpoints |
| `APP_FLOW.md` | User flows | Building UI |
| `DESIGN.md` | UI/UX design | Frontend work |
| `RAG.md` | Retrieval system | RAG changes |

## Rules

**DO:** Update doc when code changes. Keep SPRINT.md current. Add decision records. Link related docs.

**DON'T:** Let docs drift. Document deprecated patterns. Duplicate info across files.

## Generated Assets

| Asset | Purpose | Status |
|-------|---------|--------|
| `tokenizer.json` | Tokenization vocabulary | Required runtime |
| `*.onnx` | Compiled embedding model | Required runtime |
| `wailsjs/` | Wails generated bindings | Build artifact |
| `frontend/dist/` | Compiled frontend | Build artifact |

Treat these as dependencies, not maintainability failures.

## Key Principles

1. **Queue-driven** — everything flows through `study_queue`
2. **Deterministic** — no hidden orchestration
3. **Explicit** — clear state transitions
4. **SQLite-backed** — single source of truth

*Last updated: 2026-05-08*
