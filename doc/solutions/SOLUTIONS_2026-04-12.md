# 🥷 Solutions Log - 2026-04-12

This file records the completed solutions delivered in this development cycle.

## 1) Input Validation Normalization Across Ingestion Functions
- **Problem:** IDs passed to ingestion functions (notebook, topic, chunk, question) could contain leading/trailing whitespace, causing inconsistent storage and retrieval.
- **Root Cause:** Inline `strings.TrimSpace()` calls were scattered across validation checks, sometimes applied inconsistently.
- **Solution:** Standardized pattern: trim inputs at function entry point before validation. All ID parameters now normalized upfront in `IngestNotebookContent()`, `IngestNotebookContentByTopic()`, `UpsertChunkVector()`, `SearchVectorsForTopic()`, `ReplaceQuestionsForTopic()`, `GetQuestionsForTopic()`, `GetQuestionByID()`, and `SaveUserAnswer()`.
- **Impact:** IDs are consistently normalized across the system; reduces bugs from whitespace differences and improves data integrity.
- **Code Changes:** `internal/db/store.go` - Refactored all ID validation functions to normalize inputs early.

## 2) Ingestion Function Decoupling and Enhanced Input Validation
- **Problem:** Ingestion functions had tight coupling with repository layer and lacked comprehensive input validation.
- **Root Cause:** Public API functions delegated validation directly to repo functions without enforcing business rules at the service layer.
- **Solution:** Decoupled public ingestion functions (`IngestNotebookContent`, `IngestNotebookContentByTopic`) from repository implementations. Added comprehensive validation at the service layer before delegation.
- **Impact:** Clear separation between public API validation and repository implementation; easier to test and maintain business rules independently.
- **Code Changes:** `internal/db/store.go` and `internal/db/notebook_orchestration_repo.go` - Relocated and enhanced validation logic.

## 3) Vector Search Retrieval Limit Enforcement
- **Problem:** Vector search could be called with arbitrarily large k values (e.g., k=1000000), potentially causing performance degradation or resource exhaustion.
- **Root Cause:** No upper bound on k parameter in `SearchVectorsForTopic()`.
- **Solution:** Added `maxRetrievalK = 100` constant and updated validation to check `1 <= k <= maxRetrievalK`. Error message updated to reflect valid range.
- **Impact:** Vector search queries now capped at 100 results, preventing accidental or malicious resource exhaustion attacks. Aligns with typical information retrieval best practices.
- **Code Changes:** `internal/db/store.go` - Added constant and validation check in `SearchVectorsForTopic()`.

## 4) User Answer Validation Without Mutation
- **Problem:** `SaveUserAnswer()` was trimming whitespace directly on the input struct field, potentially mutating caller's data unexpectedly.
- **Root Cause:** Input validation modified the `score.UserAnswer` field in place before persisting.
- **Solution:** Changed to validate against a local `trimmedAnswer` variable without mutating the original `score` struct.
- **Impact:** API now follows immutability principle; callers' data is not modified as a side effect of validation. Clearer intent and fewer unexpected bugs.
- **Code Changes:** `internal/db/store.go` - Modified `SaveUserAnswer()` validation logic.

## 5) Cross-Topic Question Validation
- **Problem:** Questions could be ingested into topics with topic IDs that don't match the question's embedded topic ID, causing data inconsistency and orphaned questions.
- **Root Cause:** `ReplaceQuestionsForTopic()` only filled in missing topic IDs but did not validate mismatches.
- **Solution:** Enhanced validation in `ReplaceQuestionsForTopic()`: if a question has a non-empty topic ID that differs from the target topic ID, reject the operation with an error. Only auto-fill topic IDs when blank.
- **Impact:** Prevents accidental cross-topic ingestion and ensures referential integrity between questions and topics. Surfaces inconsistencies at ingest time rather than at query time.
- **Code Changes:** `internal/db/store.go` - Added explicit mismatch check in question normalization loop.

## 6) Test Enhancements: Topic ID Mismatch and Rollback Preservation
- **Problem:** Tests did not verify that topic ID mismatches were properly rejected or that rollback preserved seeded questions correctly.
- **Root Cause:** Test coverage gaps for edge cases around data consistency and transaction safety.
- **Solution:** Added comprehensive test cases in `store_integration_test.go`:
  - Validation test for topic ID mismatch rejection
  - Rollback preservation test to verify seeded questions remain intact after failed ingestion
  - Cross-topic side effects prevention test
  - Whitespace-only ID rejection test
- **Impact:** Higher confidence in data consistency behavior, especially during transaction rollback scenarios. Prevents regressions in critical ingestion paths.
- **Code Changes:** `internal/db/store_integration_test.go` - Added 4 new test functions with detailed assertions.

## Build & Runtime Notes
- Build command: `wails build -tags sqlite_extension` or `wails dev -tags sqlite_extension` (CGO_ENABLED=1 required)
- Vector search now limited to max k=100 results per query
- All ID parameters are normalized (trimmed) at ingestion entry points
- Questions must have topic ID either blank (auto-filled) or matching the target topic ID
- All tests pass; cross-topic and rollback scenarios are now validated

## Notes
- This sprint focused on data consistency and input validation hardening
- All changes are backward compatible; no database migrations required
- Input normalization pattern should be applied to future ingestion functions
