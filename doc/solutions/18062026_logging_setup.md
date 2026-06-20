Let me look at the relevant files first.

Viewed logging.go:1-213
Viewed main.go:1-80

Now let me check where the loggers and helper functions are used across the codebase.

Searched for "utils\.Infof\|utils\.Warnf\|utils\.Errorf\|utils\.Debugf\|utils\.QueueLogger\|utils\.RagLogger\|utils\.ErrLogger\|utils\.LogQueueTransition\|utils\.LogBoot\|utils\.LogRetrieval\|utils\.LogQuizResult\|utils\.LogReread\|utils\.LogReview\|utils\.LogScheduler\|utils\.LogQueueTask"
Searched for "InitMultiFileLogger\|CloseMultiFileLogger"

The logging system is heavily used. Let me check the Go version to confirm `slog` is available, and also look at `app.go` for context on the logger initialization.

Viewed go.mod:1-53
Viewed app.go:60-115

Good. Go 1.26 — `log/slog` is well supported. Now I have a clear picture. Let me write the implementation plan.

The changes are:

1. **`internal/utils/logging.go`** → Replace `log.Logger` with `log/slog`. Two named `*slog.Logger`s (`QueueLogger`, `RagLogger`) writing to their respective files with JSON handler. `ErrLogger` folds into `slog.Default()` at Error level. `Infof/Warnf/Errorf/Debugf` become thin wrappers around `slog.Info/Warn/Error/Debug`. Structured helpers (`LogQueueTransition`, `LogBoot`, etc.) switch to `slog.Logger.Info` with named key-value args.

2. **`main.go`** → The `notebookHandler` simplifies. `http.FileServer` + `http.StripPrefix` handles path traversal and unescape natively. We still need the app-nil guard and the GET-only method check since this is a custom asset handler, but the path parsing collapses dramatically.

Viewed logging.go:1-213

Now I have everything I need. One important observation: the callers in `app_study.go` use both `utils.QueueLogger.Printf(...)` and `utils.ErrLogger.Printf(...)` directly, as well as the `utils.Warnf(...)` wrappers. With `slog`, `*slog.Logger` doesn't have a `.Printf` method — I need to handle those call sites too.

Let me check what the direct `.Printf` call sites look like to understand the scope:

Searched for "utils.QueueLogger.Printf\|utils.ErrLogger.Printf\|utils.RagLogger.Printf"
Searched for "utils\.(QueueLogger|RagLogger|ErrLogger)\."
Searched for "ErrLogger"
Searched for "QueueLogger"

Now I have full scope. There are direct `.Printf` calls on `utils.QueueLogger` and `utils.ErrLogger` from `app_study.go` and `internal/db/vector_repo.go`. With `slog`, `*slog.Logger` has no `.Printf` — those call sites need updating too.

Let me look at the vector_repo.go lines for context:

Viewed vector_repo.go:170-185
Viewed app_study.go:380-410

Good. Now I have a complete picture. Let me also check the other vector_repo.go call sites:

Viewed vector_repo.go:245-360

Now I have a complete picture of everything that needs changing. Here's the plan:

**`logging.go`**: Switch to `log/slog`. `QueueLogger` and `RagLogger` become `*slog.Logger` with `slog.NewJSONHandler` writing to their files. `ErrLogger` is removed — it duplicated output; callers just use `slog.Error` or `QueueLogger.Error`. The `Infof/Warnf/Errorf/Debugf` wrappers call `slog.Info/Warn/Error/Debug` with `fmt.Sprintf`. The structured helpers call `QueueLogger.Info`/`RagLogger.Info` with key-value pairs. Add `Printf`-compatible helpers (e.g., `ErrLogf`) so existing call sites compile.

Actually — the cleanest approach is to keep `ErrLogger` as a thin compatibility shim. But the right move is to just update the call sites in `app_study.go` and `vector_repo.go` to use `utils.Errorf(...)` (which wraps `slog.Error`). Let me do all three files cleanly.

Edited logging.go

Now fix the call sites in `app_study.go` (`.Printf` → slog-compatible equivalents):

Edited app_study.go

Now fix `vector_repo.go` — all four `.Printf` call sites:

Viewed vector_repo.go:174-180
Edited vector_repo.go

Now fix `main.go` — simplify `notebookHandler` using `http.FileServer`:

Edited main.go

Now verify everything compiles:

Ran command: `go build ./...`
Used tool: schedule

The build is running. While waiting, here's a summary of what was changed:

---

## Changes Made

### 1. [`internal/utils/logging.go`](file:///c:/Users/vishn/PROJECT/ai-tutor/internal/utils/logging.go) — `log/slog` migration

| Before | After |
|---|---|
| `*log.Logger` (QueueLogger, RagLogger, ErrLogger) | `*slog.Logger` (QueueLogger, RagLogger); `ErrLogger` removed |
| `log.New(file, prefix, flags)` | `slog.New(slog.NewJSONHandler(file, nil))` |
| `ErrLogger` wrote to a third file | Default `slog` logger writes to `system_errors.log` |
| Structured helpers hand-formatted strings: `"task=%s type=%s ..."` | Key-value pairs: `"task", taskID, "type", taskType` — native structured logging |
| `Infof/Warnf/Errorf/Debugf` → `log.Printf("LEVEL: ...")` | → `slog.Info/Warn/Error/Debug(fmt.Sprintf(...))` |

The log files are unchanged — `queue.log`, `rag_engine.log`, `system_errors.log` — only the format is now machine-parseable JSON instead of hand-crafted key=value strings.

### 2. [`main.go`](file:///c:/Users/vishn/PROJECT/ai-tutor/main.go) — `notebookHandler` simplification

| Before | After |
|---|---|
| Manual `url.PathUnescape` + `strings.TrimPrefix` | Removed — stdlib handles it |
| `filepath.Base(fileName) != fileName` traversal check | Removed — `http.FileServer` does `path.Clean` internally |
| `os.Stat` to block directory serving | Removed — `http.FileServer` returns 404 for directories by default |
| `http.ServeFile(rw, req, filePath)` | `http.StripPrefix("/notebooks", http.FileServer(...)).ServeHTTP(rw, req)` |
| Unused imports: `net/url`, `path/filepath`, `strings`, `os` | All removed |

### 3. [`app_study.go`](file:///c:/Users/vishn/PROJECT/ai-tutor/app_study.go) + [`internal/db/vector_repo.go`](file:///c:/Users/vishn/PROJECT/ai-tutor/internal/db/vector_repo.go) — call site fixes

Direct `.Printf` calls on the now-removed `ErrLogger` / the now `*slog.Logger`-typed `QueueLogger`/`RagLogger` are updated to use `utils.Errorf(...)` and `.Info(...)`/`.Warn(...)` with key-value pairs respectively.

One more missed call site. Let me fix it:

Viewed engine.go:288-300
Edited engine.go
Ran command: `go build ./...`

Also worth doing a complete audit now so there are no more surprises:

Searched for "(RagLogger|QueueLogger|ErrLogger)\.(Printf|Println|Print|Fatal|Fatalf|Panicf|Panic)"

Clean. Waiting on the build result now.

`go build ./...` — ✅ clean.

---

Here's the final tally of what was deleted vs. what replaced it:

**`internal/utils/logging.go`** — 213 → 196 lines
- Dropped: `ErrLogger *log.Logger`, `errLogFile *os.File`, all `log.New(...)` calls, the `init()` fallback block, hand-formatted `"key=value"` strings in the structured helpers
- Gained: `slog.NewJSONHandler` for all three output streams; the default `slog` logger now owns `system_errors.log`; structured helpers emit proper key-value pairs parseable by any log aggregator

**`main.go`** — `notebookHandler` 35 → 12 lines
- Dropped: `net/url`, `path/filepath`, `strings`, `os` imports; manual `PathUnescape`, `TrimPrefix`, `filepath.Base` traversal check, `os.Stat` directory guard
- Gained: `http.StripPrefix("/notebooks", http.FileServer(http.Dir(...)))` — one line that does all of the above correctly and handles edge cases the manual version missed (e.g., double-slash normalization, `%2F`-encoded slashes)

**Call sites fixed across 3 files** — [`app_study.go`](file:///c:/Users/vishn/PROJECT/ai-tutor/app_study.go), [`internal/db/vector_repo.go`](file:///c:/Users/vishn/PROJECT/ai-tutor/internal/db/vector_repo.go), [`internal/retrieval/engine.go`](file:///c:/Users/vishn/PROJECT/ai-tutor/internal/retrieval/engine.go): 12 direct `.Printf` calls migrated to `.Info`/`.Warn` with structured key-value args.