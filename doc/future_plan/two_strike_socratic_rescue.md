# 2-Strike Socratic Rescue Pipeline — Implementation Plan

## Overview

When a student fails a quiz twice on the same topic, the system intervenes:
- **Strike 1**: REREAD task inserted (already works)
- **Strike 2**: SOCRATIC_REMEDIAL task inserted, QUIZ marked COMPLETED, queue blocked
- **After Socratic session**: Re-quiz the topic
- **Re-quiz pass**: Flashcards generated, topic mastered
- **Re-quiz fail**: Mark as EXTERNAL_HELP_REQUIRED, unblock queue, show notice

No flashcards are ever generated for failed concepts. No complex counters needed — just one rescue cycle.

---

## Flow Diagram

```
[Quiz Fail #1] → REREAD task → Student re-reads → Quiz again
                                                    ↓
                                            [Quiz Fail #2]
                                                    ↓
                                    SOCRATIC_REMEDIAL task (blocks queue)
                                                    ↓
                                    Student completes external prompt
                                                    ↓
                                        Re-quiz (one shot)
                                       ↙                ↘
                              [Pass]                    [Fail]
                               ↓                          ↓
                        Flashcards generated        EXTERNAL_HELP_REQUIRED
                        Topic mastered              Queue unblocks
                                                   Notice shown
```

---

## File Changes

### 1. `internal/models/models.go` — Add SOCRATIC_REMEDIAL task type

**Line 38** — Add new constant after `StudyTaskTypeExaminer`:

```go
StudyTaskTypeSocraticRemedial StudyTaskType = "SOCRATIC_REMEDIAL"
```

**No new status needed.** EXTERNAL_HELP_REQUIRED is a topic-level flag, not a task status. The SOCRATIC_REMEDIAL task itself gets COMPLETED when the student finishes the session.

---

### 2. `internal/study/quiz_sync.go` — Lower threshold + new rescue logic

**Line 18** — Change threshold:

```go
const maxAutomaticRereadAttempts = 2
```

**Lines 346-388** — Replace the `else` block (current rescue logic) with:

```go
} else {
    // Strike 2: SOCRATIC_REMEDIAL rescue
    manualReviewRecommended = true
    feedback = "Concept rescue activated. Complete the Socratic session to retry."
    attempt.Feedback = feedback
    completionStatus = models.StudyTaskStatusCompleted // Mark QUIZ as COMPLETED, not FAILED

    socraticTaskID := uuid.NewString()
    socraticPayload, _ := json.Marshal(map[string]string{
        "feedback": feedback,
        "lane":     "socratic_rescue",
        "mode":     "external_prompt",
    })
    followUps = append(followUps, models.StudyQueueTask{
        ID:          socraticTaskID,
        NotebookID:  task.NotebookID,
        TopicID:     task.TopicID,
        TaskType:    models.StudyTaskTypeSocraticRemedial,
        Status:      models.StudyTaskStatusPending,
        Priority:    0,
        PayloadJSON: string(socraticPayload),
        StartPage:   task.StartPage,
        EndPage:     task.EndPage,
    })
}
```

**Key differences from current code:**
- `completionStatus = COMPLETED` (not FAILED)
- No `DeleteFSRSCardsByTopicIDTx` call (no flashcards to delete — they were never generated)
- Task type is `SOCRATIC_REMEDIAL` (not EXAMINER)
- Payload includes `"mode": "external_prompt"` to signal the UI approach

---

### 3. `internal/db/study_queue_repo.go` — Add priority ordering

**Lines 123-129** — Add SOCRATIC_REMEDIAL to the CASE statement:

```sql
CASE sq.task_type
    WHEN 'FLASHCARD_REVIEW' THEN 6
    WHEN 'REREAD' THEN 5
    WHEN 'QUIZ' THEN 4
    WHEN 'READING' THEN 3
    WHEN 'SOCRATIC_REMEDIAL' THEN 2
    WHEN 'EXAMINER' THEN 1
    ELSE 0
END DESC
```

SOCRATIC_REMEDIAL sits between READING and EXAMINER — it blocks the queue but doesn't override flashcard reviews or rereads.

---

### 4. `internal/study/socratic_rescue.go` — New file: rescue completion handler

Create a new file for the SOCRATIC_REMEDIAL completion flow.

**Purpose:** When the student completes the SOCRATIC_REMEDIAL session, insert a fresh QUIZ task for the same topic.

```go
package study

import (
    "encoding/json"
    "fmt"

    "ai-tutor/internal/models"
    "ai-tutor/internal/utils"

    "github.com/google/uuid"
)

// CompleteSocraticRescue handles the completion of a SOCRATIC_REMEDIAL task.
// It inserts a fresh QUIZ task for the same topic so the student can prove mastery.
func (s *StudyService) CompleteSocraticRescue(taskID string) error {
    // Load the SOCRATIC_REMEDIAL task
    task, err := s.repo.GetTaskByID(taskID)
    if err != nil {
        return fmt.Errorf("failed to load socratic task: %w", err)
    }
    if task.TaskType != models.StudyTaskTypeSocraticRemedial {
        return fmt.Errorf("task %s is not SOCRATIC_REMEDIAL", taskID)
    }

    // Start a transaction
    tx, err := s.repo.BeginTx()
    if err != nil {
        return fmt.Errorf("failed to begin transaction: %w", err)
    }
    defer tx.Rollback()

    // Complete the SOCRATIC_REMEDIAL task
    if err := s.repo.CompleteTaskTx(tx, taskID, models.CompletionResult{
        Status: models.StudyTaskStatusCompleted,
    }); err != nil {
        return fmt.Errorf("failed to complete socratic task: %w", err)
    }

    // Generate a fresh QUIZ for the same topic
    quizTaskID := uuid.NewString()
    quizPayload, _ := json.Marshal(map[string]interface{}{
        "source": "socratic_rescue_requiz",
        "topic_id": task.TopicID,
    })

    followUps := []models.StudyQueueTask{
        {
            ID:          quizTaskID,
            NotebookID:  task.NotebookID,
            TopicID:     task.TopicID,
            TaskType:    models.StudyTaskTypeQuiz,
            Status:      models.StudyTaskStatusPending,
            Priority:    0,
            PayloadJSON: string(quizPayload),
            StartPage:   task.StartPage,
            EndPage:     task.EndPage,
        },
    }

    // Insert follow-up tasks
    if err := s.repo.InsertFollowUpTasksTx(tx, taskID, followUps); err != nil {
        return fmt.Errorf("failed to insert re-quiz task: %w", err)
    }

    if err := tx.Commit(); err != nil {
        return fmt.Errorf("failed to commit socratic rescue completion: %w", err)
    }

    utils.Warnf("[SOCRATIC_RESCUE] rescue_completed taskID=%s topicID=%s requizTaskID=%s", taskID, task.TopicID, quizTaskID)
    return nil
}
```

---

### 5. `internal/study/quiz_sync.go` — Handle re-quiz after rescue

The re-quiz after SOCRATIC_REMEDIAL needs special handling. When the student passes the re-quiz, generate flashcards. When they fail, mark the topic as EXTERNAL_HELP_REQUIRED.

**In `SubmitQuizAttempt`**, after the quiz result is determined, check if this is a rescue re-quiz:

```go
// After line 414 (flashcardsPending assignment)
// Check if this is a rescue re-quiz
isRescueRequiz := false
if task.PayloadJSON != "" {
    var payloadMap map[string]interface{}
    if err := json.Unmarshal([]byte(task.PayloadJSON), &payloadMap); err == nil {
        if source, ok := payloadMap["source"].(string); ok && source == "socratic_rescue_requiz" {
            isRescueRequiz = true
        }
    }
}

if isRescueRequiz {
    if passed {
        // Student passed re-quiz — generate flashcards, topic mastered
        flashcardsPending = true
        utils.Warnf("[SOCRATIC_RESCUE] requiz_passed topicID=%s — flashcards will be generated", task.TopicID)
    } else {
        // Student failed re-quiz — mark as EXTERNAL_HELP_REQUIRED, unblock queue
        completionStatus = models.StudyTaskStatusCompleted // Still mark as completed
        manualReviewRecommended = true
        feedback = "This concept requires external review. Your next reading task has been unlocked."

        // Mark topic as needing external help
        if err := s.repo.MarkTopicExternalHelpRequiredTx(tx, task.TopicID); err != nil {
            utils.Warnf("[SOCRATIC_RESCUE] failed to mark external help: %v", err)
        }

        flashcardsPending = false // No flashcards on failure
        utils.Warnf("[SOCRATIC_RESCUE] requiz_failed topicID=%s — external help required", task.TopicID)
    }
}
```

---

### 6. `internal/db/schema.go` — Add external_help_required column to topics

**Inside the `alterStatements` array (around line 336)**, add:

```go
{"topics", "external_help_required", "ALTER TABLE topics ADD COLUMN external_help_required BOOLEAN DEFAULT 0"},
```

This is a simple boolean flag. No new table needed.

---

### 7. `internal/db/topic_repo.go` — Add helper function

Add a function to mark a topic as needing external help:

```go
// MarkTopicExternalHelpRequiredTx marks a topic as needing external review.
func (r *Store) MarkTopicExternalHelpRequiredTx(tx *sql.Tx, topicID string) error {
    _, err := tx.Exec(`
        UPDATE topics SET external_help_required = 1, updated_at = CURRENT_TIMESTAMP
        WHERE id = ?
    `, topicID)
    return err
}

// IsTopicExternalHelpRequired checks if a topic needs external review.
func (r *Store) IsTopicExternalHelpRequired(topicID string) (bool, error) {
    var required bool
    err := r.db.QueryRow(`
        SELECT COALESCE(external_help_required, 0) FROM topics WHERE id = ?
    `, topicID).Scan(&required)
    if err != nil {
        return false, err
    }
    return required, nil
}
```

---

### 8. `app_study.go` — Expose rescue completion to frontend

Add a Wails binding for completing the SOCRATIC_REMEDIAL task:

```go
// CompleteSocraticRescue completes the socratic rescue session and inserts a re-quiz.
func (a *App) CompleteSocraticRescue(taskID string) error {
    return a.studyService.CompleteSocraticRescue(taskID)
}
```

---

### 9. `frontend/src/services/appApi.js` — Add API binding

```js
export function completeSocraticRescue(taskID) {
    return window.go.main.App.CompleteSocraticRescue(taskID)
}
```

---

### 10. `frontend/src/pages/Dashboard.vue` — SOCRATIC_REMEDIAL task card

Add a new task card type for SOCRATIC_REMEDIAL in the dashboard task list.

**In the task type rendering logic**, add a case for `SOCRATIC_REMEDIAL`:

- **Label**: "Concept Rescue"
- **Icon**: Shield or life-buoy icon
- **Color**: Orange/amber to distinguish from normal tasks
- **Description**: "Complete the Socratic session to retry this concept"
- **Action**: Opens the rescue modal

---

### 11. `frontend/src/pages/SocraticRescue.vue` — New page/modal

Create the rescue UI. This is a modal or full-page view with two sections:

**Left Section (Source Text Preview):**
- Shows the raw text content for the topic's page range
- Loaded from the topic's chunks
- Read-only display

**Right Section (External Prompt):**
- Pre-engineered Socratic prompt template:
  ```
  I'm studying the following text for UPSC preparation. I've failed to understand
  it twice. Please act as a Socratic tutor — don't give me summaries or answers.
  Instead, ask me leading questions that guide me to discover the key concepts
  myself. Start with the most fundamental question.

  [TEXT CONTENT HERE]
  ```
- "Copy to Clipboard" button that copies the full prompt + text
- "I've Completed the Session" button that calls `completeSocraticRescue(taskID)`

**On "I've Completed" click:**
- Call `completeSocraticRescue(taskID)`
- Redirect to dashboard
- Fresh QUIZ task will appear in queue

---

### 12. `frontend/src/pages/Quiz.vue` — Handle EXTERNAL_HELP_REQUIRED

After a re-quiz failure, show a distinct notice:

```
This concept requires external review.
Your next reading task has been unlocked so you don't fall behind.
Please consult your notes or instructor for this page range.
```

Route back to dashboard. The topic is flagged and won't trigger further rescue cycles.

---

## Testing Plan

### Unit Tests (`internal/study/quiz_sync_test.go`)

1. **Test2StrikeTriggersSocraticRescue**
   - Seed: reading task → quiz task → fail quiz (strike 1) → REREAD inserted
   - Execute: fail quiz again (strike 2)
   - Assert: SOCRATIC_REMEDIAL task inserted, QUIZ marked COMPLETED, no flashcards deleted

2. **TestSocraticRescueCompletionInsertsRequiz**
   - Seed: SOCRATIC_REMEDIAL task in queue
   - Execute: CompleteSocraticRescue(taskID)
   - Assert: SOCRATIC_REMEDIAL marked COMPLETED, fresh QUIZ task inserted

3. **TestRequizPassGeneratesFlashcards**
   - Seed: QUIZ task with `source: socratic_rescue_requiz` in payload
   - Execute: Submit quiz with passing score
   - Assert: flashcardsPending = true

4. **TestRequizFailMarksExternalHelp**
   - Seed: QUIZ task with `source: socratic_rescue_requiz` in payload
   - Execute: Submit quiz with failing score
   - Assert: flashcardsPending = false, topic.external_help_required = 1

5. **TestNoFlashcardsOnInitialFailure**
   - Seed: reading task → quiz task
   - Execute: fail quiz (strike 1)
   - Assert: flashcardsPending = false, no flashcards generated

### Integration Tests

6. **TestFullRescueFlow**
   - Complete flow: read → quiz fail → reread → quiz fail → socratic → re-quiz pass → flashcards
   - Verify queue ordering at each step
   - Verify EXTERNAL_HELP_REQUIRED flag on re-quiz fail

### Frontend Tests

7. **TestSocraticRescueModalRenders**
   - Mount dashboard with SOCRATIC_REMEDIAL task
   - Verify rescue card appears with correct label
   - Click card → modal opens with source text + copy button

8. **TestClipboardCopy**
   - Click "Copy to Clipboard" → verify clipboard contains prompt + text
   - Verify "I've Completed" button is enabled after copy

---

## Queue Behavior

SOCRATIC_REMEDIAL **blocks the queue**. It sits at priority level 2 (between READING at 3 and EXAMINER at 1). Since it's PENDING and higher priority than READING, it will be the next task activated.

The student cannot skip it — they must either complete the rescue session or the queue stays blocked. This is intentional: the student has failed twice, they need intervention before moving on.

---

## What Gets NOT Built

- No local LLM integration (external clipboard only)
- No flashcard deletion on failure (flashcards were never generated)
- No complex failure counter (reread_attempts table already tracks this)
- No parallel rescue lane (rescue blocks queue)
- No infinite rescue loop (max 1 rescue cycle)

---

## Implementation Order

1. `internal/models/models.go` — Add SOCRATIC_REMEDIAL enum (5 min)
2. `internal/db/schema.go` — Add external_help_required column (5 min)
3. `internal/db/topic_repo.go` — Add helper functions (10 min)
4. `internal/study/quiz_sync.go` — Lower threshold + new rescue logic (30 min)
5. `internal/study/socratic_rescue.go` — New file: rescue completion handler (30 min)
6. `internal/db/study_queue_repo.go` — Update priority ordering (5 min)
7. `app_study.go` — Expose Wails binding (5 min)
8. `frontend/src/services/appApi.js` — Add API binding (5 min)
9. `frontend/src/pages/SocraticRescue.vue` — New rescue UI (45 min)
10. `frontend/src/pages/Dashboard.vue` — Add rescue task card (20 min)
11. `frontend/src/pages/Quiz.vue` — Handle EXTERNAL_HELP_REQUIRED notice (15 min)
12. Unit tests (30 min)
13. Integration test: full flow (20 min)

**Estimated total: ~3.5 hours**

---

## Success Criteria

- [ ] Quiz fail #1 inserts REREAD task
- [ ] Quiz fail #2 inserts SOCRATIC_REMEDIAL task, marks QUIZ COMPLETED
- [ ] SOCRATIC_REMEDIAL blocks queue (no other tasks activate)
- [ ] Rescue UI shows source text + copy-to-clipboard prompt
- [ ] "I've Completed" inserts fresh QUIZ task
- [ ] Re-quiz pass → flashcards generated
- [ ] Re-quiz fail → EXTERNAL_HELP_REQUIRED flag set, queue unblocks, notice shown
- [ ] No flashcards generated for failed concepts at any point
- [ ] `go test ./...` passes
- [ ] `wails dev` loads without errors
