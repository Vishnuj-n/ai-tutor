# Wails IPC Wiring

This document maps the communication boundaries between the Vue frontend and Go backend via Wails. Many of these methods appear "isolated" in static analysis because they are called dynamically over the bridge.

## App Methods (Wails Bindings)

| Go Method | File | Purpose |
|-----------|------|---------|
| `Greet` | `app.go` | **DEPRECATED** - Leftover template code. |
| `GetTopicContent` | `app.go` | Fetches raw topic content. |
| `GetReaderTopicBundle` | `app.go` | Fetches unified bundle for the Reader view. |
| `GetAvailableTopics` | `app.go` | Lists all topics (indexing is optional). |
| `AskAI` | `app.go` | Main RAG Q&A entry point. |
| `ExplainReaderSection` | `app.go` | Contextual AI explanation for a specific section. |
| `GetEmbeddingDiagnostics` | `app.go` | Dev tool for checking ONNX embedding health. |
| `GetTodayPlan` | `app.go` | Returns the scheduler's daily study plan. |
| `GetDailyAgenda` | `app.go` | Returns the orchestrator's task list. |
| `GetDailyStudySettings` | `app.go` | Fetches user's study budget. |
| `UpdateDailyStudyMinutes` | `app.go` | Updates user's study budget. |
| `GetStudentSettings` | `app.go` | Institutional sync and student metadata. |
| `UpsertStudentSettings` | `app.go` | Updates institutional sync settings. |
| `UpdateTaskBoundary` | `app.go` | Updates reading mission progress. |
| `GenerateMarathonQuiz` | `app.go` | Phase 1 Marathon Mode quiz generation. |
| `GenerateMarathonFlashcards` | `app.go` | Phase 1 Marathon Mode flashcard generation. |
| `GenerateComprehensiveExam` | `app.go` | Phase 1 Marathon Mode exam generation. |
| `GenerateTopicQuiz` | `app.go` | Topic-scoped quiz (Phase 3). |
| `GenerateTopicFlashcards` | `app.go` | Topic-scoped flashcard (Phase 3). |
| `GenerateTopicWrittenAssessment` | `app.go` | Topic-scoped written assessment (Phase 3). |
| `GenerateFlashcards` | `app.go` | General flashcard generation. |
| `CompleteReadingSession` | `app.go` | Marks a reading task as complete. |
| `GetFlashcards` | `app.go` | Fetches flashcards for review. |
| `RecordFlashcardReview` | `app.go` | Saves FSRS review log for a flashcard. |
| `ScoreAnswer` | `app.go` | Scores a quiz question and updates FSRS. |
| `LogReview` | `app.go` | Generic FSRS review logger. |
| `GenerateShortAnswerPrompt` | `app.go` | Socratic mode prompt generation. |
| `ScoreShortAnswer` | `app.go` | Socratic mode scoring. |
| `UploadNotebook` | `notebook_endpoints.go` | Handles raw byte uploads. |
| `UploadNotebookFromPath` | `notebook_endpoints.go` | Handles desktop-local file ingestion. |
| `DraftNotebookSyllabus` | `notebook_endpoints.go` | AI-assisted chapter drafting. |
| `ConfirmNotebookSyllabus` | `notebook_endpoints.go` | Finalizes notebook ingestion. |
| `GetNotebooks` | `notebook_endpoints.go` | Lists all notebooks. |
| `GetNotebookTopicTree` | `notebook_endpoints.go` | Hierarchical topic selector data. |
| `UpdateNotebookTitle` | `notebook_endpoints.go` | Metadata update. |
| `DeleteNotebook` | `notebook_endpoints.go` | Removes notebook and linked data. |

## Event Payloads (IPC Events)

| Event Name | Go Struct | Direction | Purpose |
|------------|-----------|-----------|---------|
| `ingestion-progress` | `ingestionProgressPayload` | Go -> JS | Real-time feedback during PDF processing. |

## Frontend Bridge (`frontend/src/services/appApi.js`)

Most frontend calls go through this abstraction layer. If a Go method is removed, this file MUST be updated.

## Isolated Nodes Analysis (Graphify Audit)

The Graphify report identified 53 isolated nodes (nodes with ≤1 edge). After analysis, **none are dead code**. They appear isolated due to static analysis limitations:

### Wails DTOs/Interfaces (Dynamic IPC - Not Detected)
- `llmProviderInterface` (app.go) - Field type for LLM providers
- `ragPipelineInterface` (app.go) - Field type for RAG pipeline
- `ingestionProgressPayload` (notebook_endpoints.go) - Event payload for Wails events
- `OpenAIRequest`, `OpenAIMessage`, `OpenAIResponse` (internal/llm/provider.go) - LLM API types

### RAG/AI Engine Components (Optional AI Suite - Preserved)
- `VectorEntry`, `RetrievalResult`, `RetrievalContext` (internal/rag/embeddings.go) - RAG retrieval types
- `IndexerConfig` (internal/rag/indexer.go) - Vector indexer configuration
- `Response` (internal/rag/pipeline.go) - RAG pipeline response
- `SearchResult` (internal/retrieval/engine.go) - Semantic search results

### Study/Scheduler Types (Active Code - Interface Patterns)
- `Option` types (internal/scheduler/service.go, internal/notebook/upload.go) - Functional options pattern
- `queryDueReviewCardsFn`, `queryDailyStudyMinutesFn`, `queryNextReadingTopicFn` - Dependency injection
- `LLMProvider` (internal/study/service.go, internal/notebook/syllabus.go) - Interface for LLM operations
- `Config` (internal/study/service.go) - Study service configuration
- LLM response types (`quizLLM*`, `flashcardLLM*`, `shortAnswer*`) - JSON parsing helpers
- `getEnvInt` (internal/study/service.go) - Environment variable helper

### Documentation Artifacts (Kept)
- `god_files_report.md` - Code quality audit documentation
- `phase_1_plan.md` - Refactor plan documentation
- Architecture/design docs in `doc/` directory

### Conclusion
The "isolated nodes" are a false positive from static analysis. The codebase is properly structured with:
- Clear IPC boundaries via Wails
- Optional AI suite (RAG/ONNX/vec0) for Lite Mode
- Interface-based dependency injection patterns
- Comprehensive documentation

No deletions recommended.
