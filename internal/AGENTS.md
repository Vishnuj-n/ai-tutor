# internal/ — Agent Instructions

## Purpose

Go backend implementation. All business logic lives here. SQLite is the single source of truth.

---

## Directory Reference

| Directory | Responsibility | Key Files |
|-----------|----------------|-----------|
| `db/` | SQLite repositories | `store.go`, `schema.go`, `*_repo.go` |
| `models/` | Go structs | `models.go` — pure data, NO methods |
| `llm/` | LLM provider adapter | `provider.go` — OpenAI-compatible API |
| `rag/` | Retrieval pipeline (context only) | `indexer.go`, `retrieval.go` |
| `notebook/` | Upload + ingestion | `upload.go` — PDF processing |
| `study/` | Task execution logic only | Session management |

---

## Rules

### ✅ DO

- Repository pattern: all DB access through `db/*_repo.go`
- Transactions for multi-table operations
- Explicit error returns (no panics for expected errors)
- Pointers only when modifying data
- Keep `models.go` pure — no business logic in structs

### ❌ DON'T

- Access SQLite directly outside `db/` packages
- Build hidden state machines
- Add orchestration logic outside queue system
- Use RAG to control task progression (RAG retrieves context only)
- Use CGO in Windows builds (see `db/extension_nocgo.go`)
- Create autonomous flows (missions, agendas)

---

## Database Access Pattern

```go
// Good: Repository function
func (r *QueueRepo) GetNextPending(notebookID string) (*models.StudyTask, error) {
    row := r.db.QueryRow(`
        SELECT id, task_type, status, payload_json
        FROM study_queue
        WHERE notebook_id = ? AND status = 'PENDING'
        ORDER BY priority DESC, created_at ASC
        LIMIT 1
    `, notebookID)
    // ... scan and return
}
```

---

## Task State Management

All task transitions happen in `db/` repositories:

| From | To | Function |
|------|----|----------|
| PENDING | ACTIVE | `ActivateTask(taskID)` |
| ACTIVE | COMPLETED | `CompleteTask(taskID, result)` |
| ACTIVE | FAILED | `FailTask(taskID, reason)` |
| PENDING | SKIPPED | `SkipTask(taskID)` |

---

## Testing

- Unit tests: `*_test.go` alongside source
- Integration tests: `store_integration_test.go`
- Mock DB for unit tests, real SQLite for integration
- `go test ./...` must pass before commit

---

## Reference Docs

- Schema: `../doc/SCHEMA.md`
- API Contracts: `../doc/DATA_API.md`
- Module Map: `../doc/AGENT_MAP.md`

---

*Last updated: 2026-05-08*
