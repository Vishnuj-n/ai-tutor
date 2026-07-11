# Milestone Exam — 10-Session Mastery Checkpoint

## Overview

Auto-insert a `MILESTONE_EXAM` task into `study_queue` every 10th completed quiz for a notebook. Stores a compact correctness map in `payload_json` — no duplication of question data. Gives the user a consolidated view of their last 10 quizzes for research paper analysis.

---

## Flow

```
User completes 10th QUIZ
  → Flashcards generated (existing flow)
  → count % 10 == 0 check
  → Fetch last 10 quiz attempts
  → Compute correctness flags [1,0,1,0]
  → Insert MILESTONE_EXAM into study_queue
  → User sees "10-Session Mastery Exam" in queue
```

Insertion point: `GenerateFlashcardsForQuizTask` (app_study.go:861), after flashcard generation completes. This guarantees flashcards are intact before the exam is queued.

---

## Compact Payload Format

```go
type MilestoneExamPayload struct {
    Quizzes      map[string][]int `json:"quizzes"`       // attemptID → [1,0,1,0]
    PassingScore int              `json:"passing_score"`  // from quiz payload
    QuizCount    int              `json:"quiz_count"`     // always 10 (or less for final)
}
```

Example `payload_json`:
```json
{
  "quizzes": {
    "142": [1, 0, 1, 0],
    "138": [1, 1, 0, 1, 1],
    "133": [0, 1, 0]
  },
  "passing_score": 70,
  "quiz_count": 3
}
```

Key: correctness computed at insert time, not display time. Quiz questions don't change, so no reason to defer.

---

## Queue Ordering Priority

```
FLASHCARD_GENERATE(7) > SOCRATIC_REMEDIAL(6) > FLASHCARD_REVIEW(5) > REREAD(4) > QUIZ(3) > MILESTONE_EXAM(2) > READING(1) > EXAMINER(0)
```

MILESTONE_EXAM sits near the end — it doesn't block regular study flow.

---

## Implementation Steps

### Step 1: Add task type constant

**File:** `internal/models/models.go`

```go
StudyTaskTypeMilestoneExam StudyTaskType = "MILESTONE_EXAM"
```

### Step 2: Add payload model

**File:** `internal/models/models.go`

```go
type MilestoneExamPayload struct {
    Quizzes      map[string][]int `json:"quizzes"`
    PassingScore int              `json:"passing_score"`
    QuizCount    int              `json:"quiz_count"`
}
```

### Step 3: Add queue ordering priority

**File:** `internal/db/study_queue_repo.go`

Update the CASE expression (~line 132-141):

```sql
WHEN 'MILESTONE_EXAM' THEN 2
WHEN 'READING' THEN 3
```

### Step 4: Add repository methods

**File:** `internal/db/study_queue_repo.go`

```go
// CountCompletedQuizzesByNotebook returns count of completed QUIZ tasks for a notebook.
func (r *Repository) CountCompletedQuizzesByNotebook(notebookID string) (int, error)

// QuizAttemptWithPayload pairs a quiz attempt with its original quiz payload.
type QuizAttemptWithPayload struct {
    ID           string
    Score        int
    Passed       bool
    AnswersJSON  string
    CompletedAt  int64
    QuizPayload  string // from study_queue.payload_json
    PassingScore int
}

// GetLastNQuizAttemptsWithCorrectness returns last N quiz attempts for a notebook.
func (r *Repository) GetLastNQuizAttemptsWithCorrectness(notebookID string, n int) ([]QuizAttemptWithPayload, error)

// InsertMilestoneExamTask inserts a MILESTONE_EXAM task into the queue.
func (r *Repository) InsertMilestoneExamTask(notebookID string, payload MilestoneExamPayload) error
```

Fetch query:

```sql
SELECT qa.id, qa.score, qa.passed, qa.answers_json, qa.completed_at,
       sq.payload_json
FROM quiz_attempts qa
JOIN study_queue sq ON qa.task_id = sq.id
WHERE sq.notebook_id = ? AND sq.task_type = 'QUIZ'
ORDER BY qa.completed_at DESC
LIMIT ?;
```

### Step 5: Correctness computation

**File:** `internal/study/milestone.go` (new)

Pure function, no side effects:

```go
func ComputeCorrectnessFlags(quizPayloadJSON, answersJSON string) []int
```

Parses both JSONs, compares each `QuizAnswer.Selected` against `QuizTaskQuestion.CorrectAnswer`, returns `[1,0,1,0]`.

### Step 6: Integration hook

**File:** `app_study.go`, inside `GenerateFlashcardsForQuizTask` (~line 893)

After flashcard generation completes:

```go
count, err := repo.CountCompletedQuizzesByNotebook(task.NotebookID)
if err == nil && count > 0 && count%10 == 0 {
    attempts, err := repo.GetLastNQuizAttemptsWithCorrectness(task.NotebookID, 10)
    if err == nil {
        quizzes := make(map[string][]int)
        var passingScore int
        for i, a := range attempts {
            flags := study.ComputeCorrectnessFlags(a.QuizPayload, a.AnswersJSON)
            quizzes[a.ID] = flags
            if i == 0 {
                passingScore = a.PassingScore
            }
        }
        payload := models.MilestoneExamPayload{
            Quizzes:      quizzes,
            PassingScore: passingScore,
            QuizCount:    len(attempts),
        }
        _ = repo.InsertMilestoneExamTask(task.NotebookID, payload)
    }
}
```

### Step 7: Display label

**File:** `app_study.go`

```go
case models.StudyTaskTypeMilestoneExam:
    displayType = "Milestone Exam"
```

### Step 8: Update docs

**File:** `doc/SCHEMA.md`

Add `MILESTONE_EXAM` to the `task_type` list in `study_queue` table.

---

## Files Touched

| File | Change |
|------|--------|
| `internal/models/models.go` | Add constant + payload struct |
| `internal/db/study_queue_repo.go` | Add 3 repo methods + update priority CASE |
| `internal/study/milestone.go` | New file — correctness computation |
| `app_study.go` | Hook into GenerateFlashcardsForQuizTask + display label |
| `doc/SCHEMA.md` | Update task_type docs |

5 files. No new tables, no migrations, no schema changes.

---

## Test Paths

| Scenario | Expected |
|----------|----------|
| Complete 10 quizzes for a notebook | MILESTONE_EXAM appears in queue |
| Complete 9 quizzes | No milestone exam |
| Quiz fails | Not counted toward milestone |
| Notebook with < 10 quizzes | No milestone exam until 10th |
| Correctness flags match actual answers | Verify [1,0,1,0] against quiz data |
| Payload roundtrip | Marshal → unmarshal → verify integrity |

---

## Not Building (Phase 2)

- LLM generation of new exam questions
- New UI page for taking the milestone exam
- Changes to existing EXAMINER task type
- Per-topic milestone exams (notebook-level only)

---

## Premise Collapse Risk

Assumes `GenerateFlashcardsForQuizTask` is the only code path that completes quizzes with post-processing. If another path completes quizzes without this function, the milestone exam won't insert. **Mitigation:** grep for all `CompleteTaskTx` calls with QUIZ type to verify.
