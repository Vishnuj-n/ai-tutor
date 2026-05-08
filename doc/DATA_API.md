# Data API Contracts

## Overview

API contracts between frontend, queue router, and modules. All communication is synchronous and explicit.

---

## Queue Router API

### GetNextTask

Returns the next pending task from the queue.

**Endpoint:** `GetNextTask() → Task`

**Request:** None

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

**Errors:**
- `ErrNoPendingTasks` - Queue is empty

---

### CompleteTask

Marks a task complete and triggers follow-up logic.

**Endpoint:** `CompleteTask(taskID string, result TaskResult) → error`

**Request:**
```json
{
  "task_id": "task-uuid",
  "result": {
    "type": "quiz_result",
    "score": 75,
    "passed": true
  }
}
```

**Result Types:**

| Type | Use Case | Data Fields |
|------|----------|-------------|
| `quiz_result` | Quiz completion | `score`, `passed` |
| `read_complete` | Reading completion | `pages_read`, `reached_end` |
| `flashcard_review` | Flashcard session | `cards_reviewed`, `ratings` |
| `skip` | User skips task | `reason` (optional) |

**Response:** Success or error

**Side Effects:**
- Updates task status to `COMPLETED`, `SKIPPED`, or `FAILED`
- May insert follow-up tasks based on result
- Skipped tasks preserve audit trail and can resurface

### SkipTask

Explicitly marks a task as skipped (auditable bypass).

**Endpoint:** `SkipTask(taskID string, reason string) → error`

**Request:**
```json
{
  "task_id": "task-uuid",
  "reason": "User chose to skip remediation"
}
```

**Response:** Success or error

**Side Effects:**
- Updates task status to `SKIPPED`
- Task remains auditable in database
- Can be resurfaced via manual retry if needed
- No follow-up tasks inserted

---

### GetTaskContext

Returns full context for a task.

**Endpoint:** `GetTaskContext(taskID string) → TaskContext`

**Response:**
```json
{
  "task": {
    "id": "task-uuid",
    "task_type": "READING",
    "block_id": "block-uuid"
  },
  "block": {
    "id": "block-uuid",
    "content": "...",
    "word_count": 2500,
    "start_page": 10,
    "end_page": 15
  },
  "topic": {
    "id": "topic-uuid",
    "title": "Neural Networks"
  }
}
```

---

## Reader Module API

### GetBlockContent

Returns content for a reading block.

**Endpoint:** `GetBlockContent(blockID string) → BlockContent`

**Response:**
```json
{
  "id": "block-uuid",
  "content": "Full text content...",
  "word_count": 2500,
  "start_page": 10,
  "end_page": 15,
  "order_index": 3,
  "topic_id": "topic-uuid"
}
```

---

### MarkBlockRead

Records reading progress.

**Endpoint:** `MarkBlockRead(blockID string, progress int) → error`

**Request:**
```json
{
  "block_id": "block-uuid",
  "progress": 100
}
```

---

## Quiz Module API

### GetQuizSet

Returns quiz questions for a block.

**Endpoint:** `GetQuizSet(blockID string) → QuizSet`

**Response:**
```json
{
  "id": "quiz-set-uuid",
  "block_id": "block-uuid",
  "topic_id": "topic-uuid",
  "questions": [
    {
      "id": "q-1",
      "question": "What is backpropagation?",
      "options": ["A", "B", "C", "D"],
      "correct_answer": 0
    }
  ],
  "threshold": 70
}
```

---

### SubmitQuiz

Submits answers and returns score.

**Endpoint:** `SubmitQuiz(blockID string, answers []Answer) → QuizResult`

**Request:**
```json
{
  "block_id": "quiz-set-uuid",
  "answers": [
    {"question_id": "q-1", "selected": 0},
    {"question_id": "q-2", "selected": 2}
  ]
}
```

**Response:**
```json
{
  "score": 75,
  "passed": true,
  "correct_count": 3,
  "total_count": 4,
  "feedback": "Good understanding of concepts..."
}
```

---

## Flashcard Module API

### GetDueCards

Returns cards due for review.

**Endpoint:** `GetDueCards(blockID string) → []Card`

**Response:**
```json
{
  "cards": [
    {
      "id": "card-uuid",
      "prompt": "What is gradient descent?",
      "answer": "An optimization algorithm...",
      "due_at": 1234567890
    }
  ]
}
```

---

### RateCard

Records user rating and updates FSRS state.

**Endpoint:** `RateCard(cardID string, rating Rating) → error`

**Request:**
```json
{
  "card_id": "card-uuid",
  "rating": 3
}
```

**Rating Values:**
- 1 = Again
- 2 = Hard
- 3 = Good
- 4 = Easy

---

## FSRS Service API

### CalculateNextReview

Pure function for FSRS scheduling.

**Endpoint:** `CalculateNextReview(state FSRSState, rating int) → FSRSResult`

**Request:**
```json
{
  "state": {
    "stability": 1.5,
    "difficulty": 5.0,
    "elapsed_days": 1
  },
  "rating": 3
}
```

**Response:**
```json
{
  "next_interval_days": 3,
  "new_state": {
    "stability": 2.8,
    "difficulty": 4.8
  }
}
```

---

### GetDueCards

Returns all cards due for a topic.

**Endpoint:** `GetDueCards(topicID string) → []Card`

---

## RAG / Ask AI API

### AskQuestion

Answers a question using topic-scoped retrieval.

**Endpoint:** `AskQuestion(topicID string, question string) → Answer`

**Request:**
```json
{
  "topic_id": "topic-uuid",
  "question": "Explain backpropagation"
}
```

**Response:**
```json
{
  "answer": "Backpropagation is...",
  "context_blocks": ["block-uuid-1", "block-uuid-2"],
  "confidence": 0.95
}
```

---

## Ingestion API

### ProcessPDF

Extracts text and creates chunks.

**Endpoint:** `ProcessPDF(filePath string) → ProcessingResult`

**Response:**
```json
{
  "topic_id": "topic-uuid",
  "title": "Neural Networks",
  "blocks_created": 12,
  "tasks_inserted": 12
}
```

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

**Status Semantics:**

| Status | Meaning | Terminal |
|--------|---------|----------|
| `PENDING` | Waiting in queue | No |
| `ACTIVE` | Currently being worked | No |
| `COMPLETED` | Successfully finished | Yes |
| `SKIPPED` | User bypassed task | Yes (auditable) |
| `FAILED` | Generation error | Yes (can retry) |

### Generation Status (Quiz Tasks)

```go
type GenerationStatus string

const (
  StatusGenerating GenerationStatus = "GENERATING"
  StatusReady      GenerationStatus = "READY"
  StatusFailedGen  GenerationStatus = "FAILED"
)
```

### Task Result Types

```go
type TaskResult struct {
  Type   string      // "quiz_result", "read_complete", "flashcard_review"
  Data   interface{} // Type-specific data
}

type QuizResult struct {
  Score   int  // 0-100
  Passed  bool
}

type FlashcardReviewResult struct {
  CardsReviewed int
  Ratings       []int
}
```

---

## Error Handling

### Standard Errors

| Error | Code | Description |
|-------|------|-------------|
| ErrNotFound | 404 | Resource not found |
| ErrNoPendingTasks | 204 | Queue is empty |
| ErrInvalidInput | 400 | Invalid request |
| ErrLLMUnavailable | 503 | LLM service down |
| ErrQuizGenerationFailed | 500 | Quiz generation error |
| ErrMaxRereadsReached | 409 | Max reread attempts exceeded |
| ErrReadingIncomplete | 400 | User has not reached final page |
| ErrTaskNotActive | 409 | Task is not in ACTIVE status |

### Error Response Format

```json
{
  "error": "ErrNoPendingTasks",
  "message": "No pending tasks in queue",
  "code": 204
}
```

---

## API Call Patterns

### Standard Flow

```
1. Dashboard calls GetNextTask()
2. User clicks task
3. Router mounts module with task.context
4. Module calls its API (GetBlockContent, GetQuizSet, etc.)
5. User completes task
6. Module calls CompleteTask(taskID, result)
7. Queue router marks complete, inserts follow-ups
8. Dashboard refreshes, shows next task
```

### No Async Patterns

- No callbacks
- No event listeners
- No webhooks
- No background job status polling

All calls are:
- Synchronous request/response
- Immediate result
- Loading state shown in UI

---

## Authentication / Security

Local-only app - no authentication required.

All APIs:
- Run on localhost
- Bound to Wails bridge
- No CORS needed
- No tokens needed
