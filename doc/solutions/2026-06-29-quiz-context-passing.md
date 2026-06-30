# Solution: Quiz Context Passing & Corrective Flashcards

## Overview

To improve the remediation loop when a user answers quiz questions incorrectly, we implemented a minimal, database-backed context-passing flow:
1. **Quiz Failure (Below Threshold)**: Incorrect question details (prompt, choices, correct option, and user answer) are saved in the `SOCRATIC_REMEDIAL` task's payload.
   - **In-App Socratic Tutor**: Dynamically extracts these failed questions and injects them into the system prompt to guide the conversation.
   - **External Option**: Parses and renders the failed questions on the Socratic Rescue page and appends them to the copied clipboard prompt.
2. **Quiz Pass (Above Threshold with Errors)**: Retrieves the latest completed quiz attempt for the topic, extracts incorrect answers, and instructs the flashcard generator LLM to output targeted corrective flashcards in addition to the standard set.

---

## Architectural Details

### 1. Data Models
- **File**: [`internal/models/models.go`](file:///c:/Users/vishn/PROJECT/ai-tutor/internal/models/models.go)
- **Addition**: Added `FailedQuestionDetail` struct:
  ```go
  type FailedQuestionDetail struct {
  	Prompt        string   `json:"prompt"`
  	Options       []string `json:"options"`
  	CorrectAnswer string   `json:"correct_answer"`
  	UserAnswer    string   `json:"user_answer"`
  }
  ```

### 2. Database Layer (SQLite Queries)
- **File**: [`internal/db/study_queue_repo.go`](file:///c:/Users/vishn/PROJECT/ai-tutor/internal/db/study_queue_repo.go)
- **Methods**:
  - `GetLatestQuizAttemptDetailsByTopic(topicID string) (string, string, error)`: Retrieves the quiz payload and submitted answers for the latest attempt of a topic.
  - `GetActiveRemedialTaskPayloadByTopic(topicID string) (string, error)`: Retrieves the active `SOCRATIC_REMEDIAL` task payload for a topic to check for failed questions.

### 3. Quiz Completion & Handoff
- **File**: [`internal/study/quiz_sync.go`](file:///c:/Users/vishn/PROJECT/ai-tutor/internal/study/quiz_sync.go)
- **Scoring**: Scans submitted answers against the correct answers to identify failed questions during scoring.
- **Rescue Lane Handoff**: `triggerSocraticRescueHandoffTx` accepts `failedQuestions` and serializes them under the `"failed_questions"` key in the remedial task's `payload_json`.

### 4. Socratic Tutoring (removes extra API bridge params)
- **In-App Socratic Chat**:
  - **File**: [`internal/study/socratic.go`](file:///c:/Users/vishn/PROJECT/ai-tutor/internal/study/socratic.go)
  - **Details**: `AskSocratic` checks for an active remedial task payload, parses the failed questions list, and appends a concise textual representation to the LLM prompt.
- **External Socratic Prompt**:
  - **File**: [`frontend/src/pages/SocraticRescue.vue`](file:///c:/Users/vishn/PROJECT/ai-tutor/frontend/src/pages/SocraticRescue.vue)
  - **Details**: Extracts failed questions on mount, displays them visually with clean correct/incorrect styling, and appends them to the clipboard-copy prompt.

### 5. Corrective Flashcard Generation
- **File**: [`internal/study/flashcard.go`](file:///c:/Users/vishn/PROJECT/ai-tutor/internal/study/flashcard.go)
- **Details**:
  - `GenerateFSRSCardsForTopic` retrieves the latest quiz attempt, matches incorrect choices, and passes the failed questions list to the generator core.
  - `buildMarathonFlashcardPromptWithBudget` appends the failed questions context to the LLM generation prompt under a `=== TARGETED REVIEW: MISCONCEPTIONS ===` header and increments the target flashcard count by the number of failed questions.

---

## Testing Verification

### Automated Tests
Verified by running the database and study package tests:
```powershell
go test ./internal/db/... ./internal/study/...
```

**Output**:
```plaintext
ok      ai-tutor/internal/db    11.283s
ok      ai-tutor/internal/study 4.096s
```
