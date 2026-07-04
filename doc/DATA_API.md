# Data API Contracts

All communication synchronous and explicit.

---

## Queue Router API

### GetNextTask
Returns next pending task.

**Response:**
```json
{
  "id": "task-uuid",
  "task_type": "READING",
  "block_id": "block-uuid",
  "related_id": "topic-uuid",
  "status": "PENDING",
  "priority": 1,
  "created_at": 1234567890,
  "context": {
    "topic_title": "Neural Networks",
    "word_count": 2500,
    "progress": 0
  }
}
```

**Errors:** `ErrNoPendingTasks` — queue empty.

### CompleteTask
Marks task complete + triggers follow-ups.

**Result Types:**

| Type | Use Case | Data |
|------|----------|------|
| `quiz_result` | Quiz completion | `score`, `passed`. No FSRS update. |
| `read_complete` | Reading completion | `pages_read` (informational) |
| `flashcard_review` | Flashcard session | `cards_reviewed`, `ratings`. Updates FSRS. |
| `skip` | User skips task | `reason` (optional) |

**Side effects:** Updates status, may insert follow-ups, skipped tasks preserve audit trail.

### SkipTask
Marks task as skipped (auditable). No follow-ups inserted.

### GetTaskContext
Returns full context: task, block content, topic info.

---

## Reader Module API

### GetBlockContent
Returns content for a reading block: id, content, word_count, start_page, end_page, order_index, topic_id.

### MarkBlockRead
Records reading progress (block_id, progress percentage).

---

## Quiz Module API

### GetQuizSet
Returns quiz questions for a block: questions array, threshold.

### SubmitQuiz
Submits answers, returns score/passed/feedback. Backend evaluates for follow-up behavior.

---

## Flashcard Module API

### GetDueCards
Returns cards due for review (id, prompt, answer, due_at).

### RateCard
Records rating (1=Again, 2=Hard, 3=Good, 4=Easy). Updates FSRS state.

### GenerateFlashcardsForQuizTask
Generates FSRS flashcards after passed quiz. Clean Review state (StateCode: 2), day-based `due_at` offsets:
- Ace (100%): 3-day offset
- Pass (<100%): 1-day offset
- Default: tomorrow

---

## FSRS Service API

### CalculateNextReview
Pure function: FSRSState + rating → next interval + new state.

### GetDueCards
Returns all due cards for a topic.

---

## RAG / Ask AI API

### AskQuestion
Topic-scoped retrieval. Input: topic_id + question. Output: answer, context_blocks, confidence.

---

## SuspendFlashcard
Suspends a card (removed from future reviews). Returns remaining pending card count.

---

## GetTopicSectionsContent
Returns joined text of all sections in a topic (used by SocraticRescue).

---

## SocraticRescue API

### CompleteSocraticRescue
Completes rescue session, inserts fresh QUIZ task with `"source": "socratic_rescue_requiz"`. Queue unblocks.

### DevForceSocraticRescue
Dev-only: forces topic into SOCRATIC_REMEDIAL state. Requires `APP_ENV=dev`.

---

## Settings API

### GetRemedialStrategy / SetRemedialStrategy
Get/set user's quiz failure preference: `CLASSIC` (reread first) or `FAST` (direct rescue).

---

## Ingestion API

### ProcessPDF
Extracts text, creates chunks. Returns topic_id, title, chunks_created, tasks_inserted.

---

## Type Definitions

### Task Types
```go
type TaskType string
const (
  TaskTypeReading         TaskType = "READING"
  TaskTypeQuiz            TaskType = "QUIZ"
  TaskTypeReread          TaskType = "REREAD"
  TaskTypeFlashcardReview TaskType = "FLASHCARD_REVIEW"
  TaskTypeExaminer        TaskType = "EXAMINER"
  TaskTypeSocraticRemedial TaskType = "SOCRATIC_REMEDIAL"
  TaskTypeFlashcardSync   TaskType = "FLASHCARD_SYNC"
)
```

### Task Status
```go
type TaskStatus string
const (
  StatusPending   TaskStatus = "PENDING"
  StatusActive    TaskStatus = "ACTIVE"
  StatusCompleted TaskStatus = "COMPLETED"
  StatusSkipped   TaskStatus = "SKIPPED"
  StatusFailed    TaskStatus = "FAILED"
)
```

| Status | Terminal |
|--------|----------|
| `PENDING` | No |
| `ACTIVE` | No |
| `COMPLETED` | Yes |
| `SKIPPED` | Yes (auditable, can resurface) |
| `FAILED` | Yes (can retry) |

### Generation Status (Quiz Tasks)
`GENERATING` → `READY` or `FAILED`

---

## Dashboard Streak API

### GetStreakState
Returns current_streak, longest_streak, active_dates[] for calendar widget. Timezone-aware.

---

## Cloud Sync Payload

```json
{
  "classroom_code": "CLS101",
  "logs": [
    {
      "filename": "textbook_chapter3.pdf",
      "page_number": 15,
      "activity_type": "review",
      "reference_id": "card-uuid",
      "reviewed_at": 1234567890,
      "rating": 3,
      "scheduled_days": 5,
      "state_before_json": "{}",
      "state_after_json": "{}"
    }
  ]
}
```

Data chain: `reference_id → flashcards.id → chunks.id → notebook_topics.topic_id → notebooks.file_path → filepath.Base()`

---

## Error Handling

| Error | Code | Description |
|-------|------|-------------|
| ErrNotFound | 404 | Resource not found |
| ErrNoPendingTasks | 204 | Queue empty |
| ErrInvalidInput | 400 | Invalid request |
| ErrLLMUnavailable | 503 | LLM service down |
| ErrQuizGenerationFailed | 500 | Quiz generation error |
| ErrMaxRereadsReached | 409 | Max reread attempts exceeded |

---

## Standard Flow

```
Dashboard → GetNextTask() → User clicks → Router mounts module
→ Module calls API (GetBlockContent, GetQuizSet, etc.)
→ User completes → CompleteTask(taskID, result)
→ Queue router marks complete + inserts follow-ups
→ Dashboard refreshes, shows next task
```

All calls synchronous. No callbacks, event listeners, webhooks, or background polling.

## Authentication

Local-only app. All APIs run on localhost via Wails bridge. No auth, no CORS, no tokens.
