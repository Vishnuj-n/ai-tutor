# Walkthrough - Pruning Dead Code

All 21 functions identified as "Truly Dead Code" (having 0 callers in both production and test paths) have been successfully removed.

## Changes Made

We modified 10 source files to remove unused functions, unused package variables, and unused package imports:

### Assessment Repository
- Removed `saveWrittenAnswerRepo` and `SaveWrittenAnswer` in [assessment_repo.go](file:///c:/Users/vishn/PROJECT/ai-tutor/internal/db/assessment_repo.go).

### Notebooks Repository
- Removed `GetChunkTextByNotebookPageRange`, `GetNotebookPageCount`, and `AutoSwapCompletedNotebook` in [notebooks_repo.go](file:///c:/Users/vishn/PROJECT/ai-tutor/internal/db/notebooks_repo.go).

### Reader Repository
- Removed `GetTopicContent` in [reader_repo.go](file:///c:/Users/vishn/PROJECT/ai-tutor/internal/db/reader_repo.go).

### Store & Review Sessions
- Removed `GetExistingReviewTaskForNotebook`, `GetDueReviewCardCountsByNotebook`, and `GetChunkEmbeddingRef` in [store.go](file:///c:/Users/vishn/PROJECT/ai-tutor/internal/db/store.go).
- Removed `getDueReviewCardCountsByNotebookRepo` in [review_session_repo.go](file:///c:/Users/vishn/PROJECT/ai-tutor/internal/db/review_session_repo.go).

### Study Queue Repository
- Removed `saveQuizAttemptRepo` and `SaveQuizAttempt` in [study_queue_repo.go](file:///c:/Users/vishn/PROJECT/ai-tutor/internal/db/study_queue_repo.go).

### Topics Repository
- Removed `GetTopicCurrentPageCursor`, `QueryActiveTopics`, `QueryLearningTopics`, `QueryUpcomingReadingTopics`, `CountLearnedTopics`, and `AppendQuestionsAndAdvanceCursor` in [topics_repo.go](file:///c:/Users/vishn/PROJECT/ai-tutor/internal/db/topics_repo.go).

### Embeddings
- Removed `TokenizeSimple` and its helper regexp variable `nonWord` in [text.go](file:///c:/Users/vishn/PROJECT/ai-tutor/internal/embeddings/text.go).
- Pruned the unused `"regexp"` package import.

### Models
- Removed `ReadingSessionResponse.Validate` in [models.go](file:///c:/Users/vishn/PROJECT/ai-tutor/internal/models/models.go).
- Pruned the unused `"fmt"` package import.

### Notebook Upload Options
- Removed Option arguments `WithReadFileFunc` and `WithOpenPDFFunc` in [upload.go](file:///c:/Users/vishn/PROJECT/ai-tutor/internal/notebook/upload.go).

### Utils Logging
- Removed `Debugf`, `LogReviewSessionResume`, and `LogQueueOrdering` in [logging.go](file:///c:/Users/vishn/PROJECT/ai-tutor/internal/utils/logging.go).

---

## Verification Results

### Automated Verification
We ran the entire backend test suite using `go test ./...`. All packages compile and all tests pass:

```
ok  	ai-tutor	5.908s
ok  	ai-tutor/internal/db	3.714s
ok  	ai-tutor/internal/embeddings	(cached)
ok  	ai-tutor/internal/llm	(cached)
ok  	ai-tutor/internal/notebook	1.746s
ok  	ai-tutor/internal/scheduler	2.009s

```
# NOT DEAD CODE:
2. Test-Only Helpers (Called ONLY in *_test.go Files)
These are called by integration tests, unit tests, or test utilities to set up the DB state or assert test conditions. While they are "unreachable" in the actual compiled application binary (app.go), removing them would break the test suite.

createFlashcardsRepo & CreateFlashcards: Tests use CreateFlashcards to seed deck states. The main application uses GetOrCreateFlashcardsForTopic instead.
countFlashcardsForTopicRepo & CountFlashcardsForTopic: Only called inside app_contract_test.go to count cards.
insertFSRSReviewLogRepo & InsertFSRSReviewLog: Only called in store_integration_test.go to simulate FSRS history.
insertChunkRow & CreateChunk: Test seed helpers.
LinkChunksToNotebook: Test seed helper.
UpdateNotebookChunkCount: Test utility.
GetTotalChunkTokens & getTotalChunkTokens: Test verification utility.
GetTotalChunkTokensForPageRange: Test verification utility.
GetChunkTextsForTopicPageRange: Test verification utility.
GetRereadAttemptCount: Test verification utility.
Close (
internal/db/store.go
): Used by tests during cleanup to reset the database. The desktop app does not close the database connection manually during execution.
GetDueReviewCardsForNotebook: Test verification utility.
GetNextTask (
internal/db/study_queue_repo.go
): Used in tests to retrieve and check the next pending task. The production application queries active/pending tasks via GetAllActiveTasks and GetAllPendingTasks on dashboard load.
PersistReadingProgress: Test verification utility.
assertCountEquals, contains, equalStringSlices, sanitizeWhitespace: Generic assertions inside internal/db/test_utils.go.
SeedDemoDataForTests: Test seed helper.
EnsureTopic: Test seed helper.
UpdateTopicPageBounds: Test seed helper.
UpdateTopicReadingCursor: Test utility.
WithExtractPDFFunc: Functional option used to mock PDF parser in upload_test.go.
WithQueryDueReviewCards, WithQueryNextDueReviewNotebook, WithQueryDailyStudyMinutes, WithQueryNextReadingTopic, WithQueryTokensPerPageMap: Options used to inject database query mocks into the scheduler service during unit tests.