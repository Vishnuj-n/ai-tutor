## Phase 1 Refactor Plan: Extract Study Brain Service

## Scope Confirmed

Move all quiz, flashcard, and written-assessment AI logic (prompt creation, RAG context assembly used by these flows, and LLM response parsing) out of `app.go`.

Keep exported Wails bridge method signatures unchanged in `app.go`.

Keep startup/provider wiring and AskAI readiness checks inside `App` for Phase 1.

Use direct `db` package calls inside the new service to preserve behavior and minimize churn.

---

## Build Goal

Create:

`internal/study/service.go`

Add a `StudyService` that owns study-generation orchestration currently embedded in `app.go`.

`app.go` becomes a thin bridge that delegates to:

`a.studyService`

No frontend-visible signatures or response payloads should change.

---

## Not in Scope

* No PDF / notebook upload refactor
* No UI API contract changes
* No prebuild queue migration
* No new frameworks / runtimes
* No major RAG redesign (`internal/rag/pipeline.go` remains unchanged)

---

## Chosen Strategy

Use a minimal vertical extraction:

Move only AI study logic + helpers into one service first.

Then wire delegation from `app.go`.

### Why this is preferred

* Lowest risk to Wails frontend bridge
* Preserves existing readiness behavior (`aiReady`, `aiInitError`)
* Fastest path to reduce `app.go` by ~400–600 LOC
* Easier rollback if needed

---

## Known Tradeoff

Direct `db` package coupling inside the service is not ideal for long-term testing/modularity.

### Mitigation

Keep all logic behind `StudyService` boundaries now so Phase 2 can introduce interfaces later without touching bridge signatures again.

---

## File-Level Change Plan

## 1. Add New File

`internal/study/service.go`

### Define `StudyService`

Dependencies:

* Fast LLM provider (same type used now)
* Optional heavy/RAG references where already needed
* Readiness/config fields needed for parity

---

## 2. Move Logic From `app.go`

### Quiz Flows

* existing get/create flow
* generation logic
* prompt builders
* parsing helpers

### Flashcards

* generation logic
* prompt builders
* parsing helpers

### Written Assessment

* question generation
* score parsing helpers

### Reader Completion Quiz

* prompt creation
* parsing path used by completion flow

### Preserve Existing Behavior Exactly

* retry counts
* tolerance checks
* count scaling
* JSON cleanup/parsing rules
* output shapes

---

## 3. Update `app.go`

### Add Field

```go
studyService *study.StudyService
```

### Convert Exported Methods to Thin Delegates

Examples:

```go
func (a *App) GenerateQuiz(topicID string) map[string]interface{} {
    return a.studyService.GenerateQuiz(topicID)
}
```

Keep:

* same signatures
* same map payloads
* same error text where possible

### Keep in App

* AskAI
* startup provider init
* readiness checks

---

## 4. Constructor / Startup Wiring

Inside `app.go` and verify bootstrap path in `main.go`.

Instantiate `StudyService` only after providers are ready.

Preserve nil safety so frontend never freezes.

---

## Runtime Flow After Refactor

```text
Vue Frontend
 -> Wails Bridge (App exported methods)
    -> StudyService
       -> db package
       -> FAST_LLM provider
       -> rag.Pipeline (only where already used)
 -> same map[string]interface{} responses
```

---

## Verification Required Before Phase 2

## Happy Paths

* `GenerateQuiz()` returns expected payload
* `GenerateFlashcards()` returns expected payload
* short-answer generate + score works
* `CompleteReadingSession()` still advances properly

## Error Paths

* FAST provider missing returns same error behavior
* malformed LLM JSON handled same way
* DB failures propagate same fields

## Edge Cases

* low-token topics still scale counts
* quiz retry/cardinality behavior unchanged
* empty topic content unchanged

---

## Rollback Plan

Single commit revert.

No schema or data migration involved.

---

## Existing Dependencies Only

Environment variables unchanged:

* `FAST_LLM_*`
* `HEAVY_LLM_*`

No new services, APIs, or tools required.

---

## Prior Decisions Preserved

Compatible with prior modularization direction:

* `refactor/db-modular (#25)`

Aligned with architecture docs already reviewed. 

---

## Final Decisions

* Preserve all exported bridge signatures
* Keep readiness ownership in `App`
* Extract by vertical slice (quiz / flashcard / written)
* No interfaces yet
* Stop after Phase 1 and validate UI flows before continuing

---

## Deferred for Phase 2

Whether `AskAI` should move into `StudyService`.

Deferred because it involves broader orchestration + readiness lifecycle changes.
