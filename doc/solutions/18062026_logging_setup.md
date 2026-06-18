# Implementation Plan - Structured Multi-File Logging Refactoring

Isolate the logging layer inside the `sandbox/combined-test` branch into a structured, domain-separated multi-file logging architecture. Ensure zero regressions in database structures, tests, or application endpoints.

## Proposed Changes

### 1. Component: `internal/utils`

#### [MODIFY] [logging.go](file:///c:/Users/vishn/PROJECT/ai-tutor/internal/utils/logging.go)
- Create global, thread-safe `*log.Logger` instances and `*os.File` pointers:
  - `QueueLogger` -> `logs/queue.log` (Prefix: `[QUEUE] `)
  - `RagLogger`   -> `logs/rag_engine.log` (Prefix: `[RAG] `)
  - `ErrLogger`   -> `logs/system_errors.log` (Prefix: `[CRITICAL_ERROR] `)
- Implement `InitMultiFileLogger(appDataDir string) error`:
  - Ensure the `logs` subdirectory is created using `os.MkdirAll`.
  - Open log files in append mode (`os.O_CREATE|os.O_WRONLY|os.O_APPEND`).
  - Initialize loggers under a thread-safe mutex.
- Implement `CloseMultiFileLogger()`:
  - Safely sync and close active log file handles.
  - Reset logger pointers to stdout/stderr defaults to prevent nil-pointer panics in test contexts.
- Simplify global helpers (`Debugf`, `Infof`, `Warnf`, `Errorf`) as thin wrappers to standard `log.Printf`.
- Update structured log functions (`LogQueueTransition`, `LogBoot`, etc.) to write to the domain-separated loggers without redundant prefixes or level-checks.

---

### 2. Component: `main` / `app`

#### [MODIFY] [app.go](file:///c:/Users/vishn/PROJECT/ai-tutor/app.go)
- In the `startup` method, resolve the application directory using `runtime.ResolveAppDir()` and call `utils.InitMultiFileLogger(appDir)` before loading database and other services.
- Add a `shutdown(ctx context.Context)` method calling `utils.CloseMultiFileLogger()`.
- Refactor the `InitializeReadingSession` method status check block:
  - If a task's status is `models.StudyTaskStatusActive`, write a clean idempotent resume message directly to `utils.QueueLogger.Printf` (omit fake warnings).
  - If task activation fails or task is `nil` (unexpected loading anomaly/database load error), write the error detail to `utils.ErrLogger.Printf` and trace a timeline notification to `utils.QueueLogger.Printf`.

#### [MODIFY] [main.go](file:///c:/Users/vishn/PROJECT/ai-tutor/main.go)
- Register `OnShutdown: app.shutdown,` in Wails' `options.App` initialization struct to guarantee proper handle release.

---

### 3. Component: `internal/retrieval`

#### [MODIFY] [engine.go](file:///c:/Users/vishn/PROJECT/ai-tutor/internal/retrieval/engine.go)
- Import `"ai-tutor/internal/utils"`.
- If `e.embedder == nil` during search operations, log an unallocated pointer notice directly to `utils.RagLogger.Printf`.

---

### 4. Component: `internal/db`

#### [MODIFY] [vector_repo.go](file:///c:/Users/vishn/PROJECT/ai-tutor/internal/db/vector_repo.go)
- Import `"ai-tutor/internal/utils"`.
- If a query skips due to `embeddingDimension <= 0` or fails due to `isVectorUnavailableError`, log the precise boundaries to both `utils.RagLogger` and `utils.ErrLogger`.

---

## Verification Plan

### Automated Tests
- Run `go test ./...` to ensure compilation and existing contract tests pass with zero regressions.
- Validate that mock tests inside [app_contract_test.go](file:///c:/Users/vishn/PROJECT/ai-tutor/app_contract_test.go) compile and execute successfully.

### Manual Verification
- Verify that `logs/` directory and log files are programmatically created upon running `go build`.
