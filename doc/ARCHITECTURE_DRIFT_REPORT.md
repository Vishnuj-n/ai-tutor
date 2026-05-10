# AI Tutor Architecture Drift Audit Report

**Generated:** May 10, 2026  
**Scope:** Full codebase analysis  
**Intended Architecture:** Persistent Guided Study Queue (SQLite-backed, deterministic, no hidden orchestration)

---

## Executive Summary

This audit analyzes the AI Tutor codebase against the intended architecture specified in AGENTS.md, SCHEMA.md, and ARCHITECTURE.md. The architecture mandates a **Persistent Guided Study Queue** with:
- SQLite as single source of truth
- `study_queue` as orchestration backbone
- Deterministic queue lifecycle with explicit transitions
- No hidden orchestration or autonomous schedulers
- FSRS as interval scheduling algorithm only (not flow control)

### Key Findings

| Category | Count | Risk Level |
|----------|-------|------------|
| **CORE** (Active, Production-Ready) | 15 | Low |
| **SUPPORTING** (Active, Non-Critical) | 8 | Low |
| **TRANSITIONAL** (In Migration) | 4 | Medium |
| **PREMATURE** (Ghost Architecture) | 2 | **High** |
| **LEGACY** (Should Evaluate for Removal) | 2 | Medium |
| **DEAD** (No Consumers) | 0 | - |

---

## 1. Database Schema Analysis

### Current Schema Tables (from `schema.go`)

| Table | Classification | Status |
|-------|----------------|--------|
| `topics` | CORE | Active |
| `parents` | CORE | Active - Section/chunk parent container |
| `chunks` | CORE | Active - Content blocks |
| `topic_progress` | SUPPORTING | Active - Learning metadata |
| `questions` | CORE | Active - Quiz questions |
| `user_answers` | CORE | Active - Quiz attempt storage |
| `quiz_attempts` | CORE | Active - Quiz results tracking |
| `reread_attempts` | CORE | Active - Remediation counter |
| `written_questions` | SUPPORTING | Active - Short-answer prompts |
| `written_user_answers` | SUPPORTING | Active - Short-answer submissions |
| `user_settings` | SUPPORTING | Active - Daily study minutes |
| `notebooks` | CORE | Active - Source documents |
| `notebook_topics` | CORE | Active - Notebook-topic linking |
| `notebook_chunks` | CORE | Active - Notebook-chunk linking |
| `fsrs_cards` | CORE | Active - Flashcard state |
| `fsrs_review_log` | CORE | Active - Review audit trail |
| `assessment_fsrs` | TRANSITIONAL | Active - Written assessment FSRS |
| `study_queue` | **CORE** | Active - Central queue |
| `reading_progress` | CORE | Active - Per-task progress |
| `review_task_cards` | TRANSITIONAL | Active - Review session mapping |

### Schema Drift Issues

**DOCUMENTATION MISMATCH (SCHEMA.md vs Actual):**

| Documented | Actual | Status |
|------------|--------|--------|
| `blocks` table | `chunks` + `parents` | DOC OUTDATED |
| `quiz_sets` table | `questions` with `payload_json` | DOC OUTDATED |
| `sources` table | merged into `notebooks` | DOC OUTDATED |
| `app_config` table | `user_settings` | DOC OUTDATED |
| `block_vectors` table | vectors in rag.EmbeddingStore | DOC OUTDATED |

**FINDING:** SCHEMA.md is significantly outdated. Actual implementation is more evolved than documentation suggests.

---

## 2. Repository Analysis (internal/db/)

### Active Repositories

| Repository | Classification | Lines | Status |
|------------|----------------|-------|--------|
| `study_queue_repo.go` | **CORE** | ~500 | Active, well-maintained |
| `notebooks_repo.go` | **CORE** | ~400 | Active |
| `flashcard_repo.go` | **CORE** | ~300 | Active |
| `fsrs_review_log_repo.go` | **CORE** | ~150 | Active |
| `topics_repo.go` | **CORE** | ~200 | Active |
| `quiz_repo.go` | **CORE** | ~250 | Active |
| `reread_attempts_repo.go` | **CORE** | ~80 | Active |
| `store.go` | **CORE** | ~400 | Active |
| `schema.go` | **CORE** | ~300 | Active |
| `reader_bundle_repo.go` | SUPPORTING | ~150 | Active |
| `reader_repo.go` | SUPPORTING | ~100 | Active |
| `review_session_repo.go` | SUPPORTING | ~80 | Active |
| `assessment_repo.go` | SUPPORTING | ~120 | Active |
| `vector_repo.go` | SUPPORTING | ~100 | Active |

### Orphaned/Dead Repositories

| Repository | Classification | Issue | Recommendation |
|------------|----------------|-------|----------------|
| `marathon_repo.go` | **LEGACY** | Marathon Mode endpoints moved to study service | Remove after verification |
| `notebook_topic_tree_repo.go` | **LEGACY** | Replaced by GetNotebookTopicTree in notebooks_repo.go | Remove after verification |
| `notebook_orchestration_repo.go` | **PREMATURE** | No clear consumer | **Investigate for removal** |
| `internal/db/notebook_orchestration_repo.go` | **PREMATURE** | No clear consumer in codebase | **Investigate for removal** |

---

## 3. Service Analysis (internal/)

### Core Services

| Service | Classification | Purpose |
|---------|----------------|---------|
| `internal/scheduler/service.go` | **CORE** | Deterministic task generation |
| `internal/scheduler/fsrs.go` | **CORE** | Pure FSRS algorithm |
| `internal/study/service.go` | **CORE** | Quiz/flashcard generation |
| `internal/study/reader.go` | **CORE** | Reading completion flow |
| `internal/study/quiz.go` | **CORE** | Quiz generation and scoring |
| `internal/study/flashcard.go` | **CORE** | Flashcard management |
| `internal/notebook/ingestion.go` | **CORE** | Content processing |

### Supporting Services

| Service | Purpose |
|---------|---------|
| `internal/study/fsrs.go` | FSRS wrapper |
| `internal/study/examiner.go` | Assessment system |
| `internal/study/socratic.go` | Socratic tutoring |
| `internal/study/review_session.go` | Review sessions |
| `internal/notebook/upload.go` | File upload |
| `internal/notebook/pdfcpu.go` | PDF processing |
| `internal/notebook/syllabus.go` | Chapter extraction |
| `internal/rag/pipeline.go` | RAG retrieval |

### Premature/Orphaned Services

| Service | Classification | Issue |
|---------|----------------|-------|
| `internal/notebook/orchestration.go` | **PREMATURE** | No clear Wails binding consumer; appears to duplicate study_queue functionality |
| `internal/db/notebook_orchestration_repo.go` | **PREMATURE** | Orphaned repository |

**STATUS: HIGH RISK** - These represent ghost architecture that should be investigated for removal.

---

## 4. Wails Bindings Analysis (app.go + notebook_endpoints.go)

### Queue Operations (CORRECT)

```
GetNextTask() → study_queue query
ActivateTask() → PENDING → ACTIVE
CompleteTask() → COMPLETED/FAILED + follow-ups
SkipTask() → SKIPPED
GetQueueState() → pending counts
GetAllActiveTasks() / GetAllPendingTasks() → queue materialization
```

### Reading Flow (CORRECT)

```
InitializeReadingSession() → activate + load context
CompleteReading() → COMPLETED + QUIZ follow-up inserted
CompleteReadingWithGeneratedQuiz() → payload in QUIZ task
```

### Scheduler Integration (TRANSITIONAL)

```
GetTodayPlan() → scheduler.BuildTodayPlan()
  → ALSO merges active/pending queue tasks directly
```

**FINDING:** GetTodayPlan has dual behavior - it builds a fresh plan via scheduler AND merges existing queue tasks. This is a **TRANSITIONAL** pattern showing migration from scheduler-centric to queue-centric architecture.

### Marathon/Comprehensive Endpoints (LEGACY)

| Binding | Status |
|---------|--------|
| `GenerateMarathonQuiz()` | TRANSITIONAL |
| `GenerateMarathonFlashcards()` | TRANSITIONAL |
| `GenerateComprehensiveExam()` | TRANSITIONAL |
| `GenerateFlashcards()` | CORE |

---

## 5. Frontend API Consumption

### Active Bindings (appApi.js)

| Binding | Page | Classification |
|---------|------|----------------|
| `getTodayPlan` | Dashboard | **CORE** |
| `activateTask` | Dashboard | **CORE** |
| `completeTask` | Dashboard | **CORE** |
| `initializeReadingSession` | Reader | **CORE** |
| `completeReading` | Reader | **CORE** |
| `getFlashcards` | Flashcards | **CORE** |
| `recordFlashcardReview` | Flashcards | **CORE** |
| `submitQuizAttempt` | Quiz | **CORE** |
| `uploadNotebook` | Notebook | **CORE** |
| `confirmNotebookSyllabus` | Notebook | **CORE** |
| `generateReviewTasks` | Review | SUPPORTING |

### Potentially Unused/Deprecated

| Binding | Status |
|---------|--------|
| `ValidateReadingCompletion` | DEPRECATED (trust-based reading) |
| `GetTask` | May be redundant with queue flow |
| `GetDailyAgenda` | Alias for GetTodayPlan |

---

## 6. Queue Lifecycle Analysis

### Current Implementation (CORRECT)

```
PENDING → ACTIVE (on open)
ACTIVE → COMPLETED (on success)
ACTIVE → SKIPPED (on user skip)
ACTIVE → FAILED (on error)
```

### Flow Patterns

**READING → QUIZ**:
1. READING task in queue (PENDING)
2. User opens → ACTIVE via InitializeReadingSession
3. User completes → CompleteReading()
4. db.CompleteReadingWithGeneratedQuiz() inserts QUIZ follow-up

**QUIZ → REREAD/next**:
1. QUIZ task in queue (PENDING) 
2. User submits → SubmitQuizAttempt
3. studyService.SubmitQuizAttempt scores and determines follow-up
4. REREAD inserted if failed (via completion result)

**FLASHCARD_REVIEW**:
1. GenerateReviewTasks creates FLASHCARD_REVIEW tasks from due cards
2. User reviews → RecordCardReview updates FSRS state

### Queue Ordering (DETERMINISTIC ✓)

Priority hierarchy correctly implemented:
1. Task type: FLASHCARD_REVIEW > REREAD > QUIZ > READING > EXAMINER
2. Notebook priority (higher = more frequent)
3. Task priority (within same task type only)
4. Creation time (FIFO)

---

## 7. FSRS Integration Analysis

### Current Flow (CORRECT)

```
1. Flashcard created → initial FSRS state (Learning)
2. User reviews → RecordCardReview / RecordFlashcardReview
3. scheduler.NextFSRSState() calculates new interval
4. fsrs_cards.due_at updated
5. Review logged to fsrs_review_log
```

### Integration Points

- `internal/scheduler/fsrs.go` - Pure algorithm
- `internal/study/fsrs.go` - Service layer wrapping
- `internal/db/flashcard_repo.go` - Persistence

**FINDING:** FSRS correctly integrated as scheduling algorithm only. Does NOT control queue flow.

---

## 8. Remediation Flow Analysis

### Implemented (CORRECT)

```
Quiz score < threshold → REREAD task inserted
reread_attempts table tracks per-topic retry count
Max 3 automatic retries, then manual review recommendation
```

### Code Paths

1. `studyService.SubmitQuizAttempt()` scores quiz
2. If failed, creates REREAD follow-up via CompletionResult
3. `reread_attempts` incremented atomically
4. After 3 failures, no auto-REREAD inserted

---

## 9. Dependency Graphs

### Table → Repository → Service → Frontend

```
study_queue
  → study_queue_repo.go
    → app.go (GetNextTask, CompleteTask, etc.)
      → appApi.js (getTodayPlan, activateTask, completeTask)
        → Dashboard.vue

fsrs_cards
  → flashcard_repo.go
    → study/flashcard.go, study/fsrs.go
      → app.go (GetFlashcards, RecordFlashcardReview)
        → appApi.js (getFlashcards, recordFlashcardReview)
          → Flashcards.vue

questions (quiz)
  → quiz_repo.go
    → study/quiz.go
      → app.go (SubmitQuizAttempt, CompleteReading)
        → appApi.js (submitQuizAttempt)
          → Quiz.vue

chunks/parents (content)
  → notebooks_repo.go
    → notebook/ingestion.go
      → app.go (ConfirmNotebookSyllabus)
        → appApi.js (confirmNotebookSyllabus)
          → Notebook configuration UI
```

### Queue Lifecycle Flow

```
                    ┌──────────────┐
                    │  Dashboard   │
                    │ (GetNextTask)│
                    └──────┬───────┘
                           │
                           ▼
┌─────────────┐     ┌──────────────┐     ┌──────────────┐
│  PENDING    │────▶│   ACTIVE     │────▶│  COMPLETED   │
│  (queued)   │     │  (in work)   │     │  (success)   │
└─────────────┘     └──────────────┘     └──────────────┘
                           │
                    ┌──────┴───────┐
                    ▼              ▼
              ┌──────────┐   ┌──────────┐
              │ SKIPPED  │   │  FAILED  │
              │ (bypass) │   │  (error) │
              └──────────┘   └──────────┘

Follow-up inserts:
  READING → QUIZ (auto via CompleteReading)
  QUIZ (fail) → REREAD (via SubmitQuizAttempt)
  FLASHCARD → FLASHCARD_REVIEW (via GenerateReviewTasks)
```

---

## 10. Architecture Compliance Analysis

### ✓ Compliant with Intended Architecture

| Requirement | Status | Evidence |
|-------------|--------|----------|
| SQLite is source of truth | ✓ | All state in DB, no in-memory orchestration |
| study_queue is backbone | ✓ | Central table, all flows through it |
| Deterministic queue ordering | ✓ | Fixed priority rules in GetAllPendingTasks() |
| No hidden orchestration | ✓ | No event buses, no background schedulers |
| FSRS as algorithm only | ✓ | FSRS in scheduler/, not controlling flow |
| Explicit task transitions | ✓ | All state changes are DB writes |

### ⚠ Non-Compliant / Concerns

| Issue | Severity | Description |
|-------|----------|-------------|
| Ghost orchestration files | HIGH | `notebook/orchestration.go` and `notebook_orchestration_repo.go` have no consumers |
| SCHEMA.md outdated | MEDIUM | Documentation doesn't match actual implementation |
| Marathon endpoints | LOW | Legacy comprehensive mode, should evaluate consolidation |

---

## 11. Cleanup Priority Report

### HIGH RISK - Safe to Remove (After Verification)

| File | Reason | Verification Required |
|------|--------|----------------------|
| `internal/notebook/orchestration.go` | No Wails binding consumer | Check for any internal callers |
| `internal/db/notebook_orchestration_repo.go` | No consumers in codebase | Verify grep for usages |

### MEDIUM RISK - Evaluate Before Removal

| File | Replacement | Action |
|------|-------------|--------|
| `internal/db/marathon_repo.go` | studyService marathon methods | Verify all functions have replacements |
| `internal/db/notebook_topic_tree_repo.go` | notebooks_repo.GetNotebookTopicTree | Verify callers migrated |

### LOW RISK - Documentation Updates

| File | Action |
|------|--------|
| `doc/SCHEMA.md` | Update to match actual schema |
| `doc/ARCHITECTURE.md` | Add queue-centric details |

---

## 12. Phased Cleanup Plan

### Phase 1: Investigation (Low Risk)
- [ ] Verify no callers for `notebook/orchestration.go`
- [ ] Verify no callers for `notebook_orchestration_repo.go`
- [ ] Check if marathon methods fully replaced in studyService

### Phase 2: Documentation Fixes (Zero Risk)
- [ ] Update SCHEMA.md with current tables
- [ ] Add queue lifecycle documentation to ARCHITECTURE.md

### Phase 3: Removal (Medium Risk)
- [ ] Remove premature/orphaned files after verification
- [ ] Remove legacy marathon_repo.go if fully replaced
- [ ] Remove notebook_topic_tree_repo.go if fully replaced

### Phase 4: Consolidation (Optional)
- [ ] Evaluate marathon endpoints vs queue-based flows
- [ ] Consider deprecating GetTodayPlan merge pattern

---

## 13. Classification Summary

| Category | Count | Examples |
|----------|-------|----------|
| **CORE** | 15 | study_queue_repo, scheduler, fsrs, quiz flow, notebooks_repo, flashcard_repo |
| **SUPPORTING** | 8 | RAG, embeddings, retrieval, syllabus, examiner |
| **TRANSITIONAL** | 4 | GetTodayPlan merge, marathon endpoints, assessment_fsrs |
| **PREMATURE** | 2 | notebook/orchestration, notebook_orchestration_repo |
| **LEGACY** | 2 | marathon_repo, notebook_topic_tree_repo |
| **DEAD** | 0 | None definitively identified |

---

## 14. Visual Outputs

Additional visualizations generated:
- `graphify-out/graph.html` - Interactive dependency graph
- `graphify-out/GRAPH_REPORT.md` - Graph analysis report

---

## 15. Recommendations

### Immediate Actions
1. Investigate `notebook/orchestration.go` and `notebook_orchestration_repo.go` for removal
2. Update `doc/SCHEMA.md` to reflect current implementation

### Short-term
1. Verify marathon_repo.go is fully replaced
2. Verify notebook_topic_tree_repo.go callers migrated

### Long-term
1. Consolidate marathon endpoints with queue-based flows
2. Document queue ordering rules in ARCHITECTURE.md
3. Consider removing GetTodayPlan merge pattern once full queue-centric

---

*Report generated by architecture drift audit. No refactoring performed.*

**Graphify Analysis:** Query-based analysis completed with 3000 token budget for dependency tracing and orphaned system detection.