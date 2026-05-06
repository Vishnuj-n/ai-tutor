# Data API (Wails Bridge Contracts)

This defines the Go methods exposed through `app.go` and the exact data passed between Go and Vue.

---

## 1. Exposed Methods

### Topics & Content

```go
GetTopicContent(topicID string) map[string]interface{}
GetReaderTopicBundle(topicID string, notebookID string) map[string]interface{}
GetAvailableTopics() []map[string]string
```

---

### Ask AI & RAG

```go
AskAI(topicID string, question string) map[string]interface{}
ExplainReaderSection(sectionID string, question string) map[string]interface{}
GetEmbeddingDiagnostics(text string) map[string]interface{}
```

---

### Planning & Scheduling

```go
GetTodayPlan() map[string]interface{}
GetDailyAgenda() map[string]interface{}
GetDailyStudySettings() map[string]interface{}
UpdateDailyStudyMinutes(minutes int) map[string]interface{}
```

---

### Student Settings

```go
GetStudentSettings() map[string]interface{}
UpsertStudentSettings(studentID string, institutionalSync bool, dashboardEndpoint string, dailyStudyMinutes int) map[string]interface{}
UpdateTaskBoundary(taskID string, newEndPage int) map[string]interface{}
```

---

### Marathon Mode

```go
GenerateMarathonQuiz(notebookID string, startPage, endPage int) map[string]interface{}
GenerateMarathonFlashcards(notebookID string, startPage, endPage int) map[string]interface{}
GenerateComprehensiveExam(notebookID string, startPage, endPage int) map[string]interface{}
```

---

### Topic-Scoped Review

```go
GenerateTopicQuiz(topicId string, startPage, endPage int) map[string]interface{}
GenerateTopicFlashcards(topicId string, startPage, endPage int) map[string]interface{}
GenerateTopicWrittenAssessment(topicId string, startPage, endPage int) map[string]interface{}
GenerateFlashcards(topicID string) map[string]interface{}
```

---

### Reading & Quiz

```go
CompleteReadingSession(topicID string, startPage int, targetPage int) map[string]interface{}
ScoreAnswer(questionID, userAnswer string) map[string]interface{}
GenerateShortAnswerPrompt(topicID string) map[string]interface{}
ScoreShortAnswer(questionID, userAnswer string) map[string]interface{}
```

---

### Flashcards

```go
GetFlashcards(topicID string, dueOnly bool) map[string]interface{}
RecordFlashcardReview(cardID string, rating string) map[string]interface{}
```

---

### Review Logging

```go
LogReview(topicID, activityType, referenceID, sourceChunkID string, score int) map[string]interface{}
```

---

## 2. Response Format

All methods return `map[string]interface{}` with either:
- Success data with appropriate keys
- `"error": "error message"` on failure

Common response patterns:
- `GetTodayPlan`: returns plan with date, total_minutes, tasks, etc.
- `AskAI`: returns answer, cited_sections, chunks_retrieved
- `ScoreAnswer`: returns correct, score, feedback, hint, fsrs_rating
- `RecordFlashcardReview`: returns card, state, review_log_id

---

## Notes

- All methods are exposed via Wails bindings in `app.go`
- Business logic is delegated to service layers (study, scheduler, orchestrator, etc.)
- Database operations are centralized in `internal/db`
- Time formats use Unix timestamps or RFC3339 as appropriate
