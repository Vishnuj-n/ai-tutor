# SPRINT.md — AI Tutor Mission Engine (Task + FSRS Priority)

## The Immutable Architecture Rules (Apply to all Sprints)
1. **Fresh Schema:** No migration scripts. Delete `ai-tutor.db` and let `store.go` rebuild it.
2. **One Page, One Chunk:** Text chunks strictly map to a single `page_num`. 
3. **Question Lineage:** Every generated question stores `source_page_start`, `source_page_end`, `llm_model`, and `prompt_version`.
4. **Hard Deletion:** If a user shrinks a chapter boundary, execute an immediate SQL `DELETE` for questions orphaned by the new boundaries.
5. **Two-Step Fast Retrieval:** Vector search must pre-filter `rowid` by `topic_id` and `page_num` *before* executing the distance calculation.

---

## Phase 1: The Unified FSRS Brain (Highest Priority)

### Goal
Create a single, unified endpoint that handles ALL review scoring across the application. Whether a user completes a Flashcard, a Quiz, or an AI Examiner session, the final action MUST pass through this brain.

### Backend (`internal/study/fsrs.go`)

**Create the Core Endpoint:**
```go
func LogReview(notebookID string, pageRange [2]int, score int) error
```

**Score Mapping (Strict):**
- 1 = Again (low recall)
- 2 = Hard (partial recall)  
- 3 = Good (expected recall)
- 4 = Easy (strong recall)

**Logic Flow:**
1. Identify the `activity_type` (flashcard, quiz, written) and `reference_id`
2. Load current FSRS state from `assessment_fsrs` table
3. Apply `scheduler.NextFSRSState(currentState, score)` 
4. Calculate `next_review = now + (scheduledDays * 24h)`
5. Update `assessment_fsrs` with new state and due timestamp
6. Log the review in `fsrs_review_log` for analytics

**Database Operations:**
- Use `assessment_fsrs` table: PRIMARY KEY (activity_type, reference_id)
- Update `state_json`, `due_at`, `last_reviewed_at`
- Insert into `fsrs_review_log` with before/after state snapshots

### Integration Points
- **Flashcard flow:** Existing `GradeFlashcard()` calls `LogReview(cardID, [page, page], score)`
- **Quiz flow:** Quiz completion aggregates per-question scores into single `LogReview(quizID, [startPage, endPage], averageScore)`
- **Written assessment flow:** Examiner completion calls `LogReview(writtenID, [startPage, endPage], score)`

---

## Phase 2: The Task Orchestrator (`internal/orchestrator/service.go`)

### Goal
Build the engine that generates the daily agenda by querying the FSRS brain and reading progress.

### Core Function
```go
func GetDailyAgenda() []models.ScheduledTask
```

### Priority Algorithm (Strict Order):

**Priority 1 (Retention): Review Missions**
```sql
SELECT activity_type, reference_id, topic_id 
FROM assessment_fsrs 
WHERE due_at <= strftime('%s', 'now')
ORDER BY due_at ASC
LIMIT 10
```
- Convert each due item to `ScheduledTask` with `ActionType = "Review"`
- Include `TopicID`, `StartPage`, `EndPage` from source lineage
- Estimate: 2 minutes per flashcard, 5 minutes per quiz, 8 minutes per written

**Priority 2 (Continuity): Reading Missions**
```sql
SELECT id, title, current_page_cursor, end_page 
FROM notebooks 
WHERE status = 'active' 
ORDER BY updated_at DESC
LIMIT 3
```
- Calculate reading target based on daily study minutes setting
- Create `ScheduledTask` with `ActionType = "Read"`
- Set `StartPage = current_page_cursor`, `EndPage = target_page`

### Output Format
Return strict array of 5-10 tasks max:
```json
{
  "tasks": [
    {
      "id": "review-1",
      "action_type": "Review", 
      "title": "Flashcard Review: Neural Networks",
      "topic_id": "topic-123",
      "start_page": 45,
      "end_page": 45,
      "estimate_minutes": 10,
      "priority": 1,
      "meta": "flashcard"
    },
    {
      "id": "read-1",
      "action_type": "Read",
      "title": "Continue Reading: Deep Learning Fundamentals", 
      "topic_id": "topic-123",
      "start_page": 67,
      "end_page": 85,
      "estimate_minutes": 45,
      "priority": 2,
      "meta": "reading"
    }
  ]
}
```

---

## Phase 3: The "Context-Locked" UI Execution

### Goal
Transform the Dashboard from a metrics display into a pure task execution launcher.

### Frontend (`frontend/src/pages/Dashboard.vue`)

**Delete All Generic Metrics:**
- Remove progress charts, summary statistics, completion percentages
- Keep ONLY the task list returned by `GetDailyAgenda()`

**Task Rendering:**
```vue
<template>
  <div class="dashboard">
    <h1>Today's Mission</h1>
    <div v-for="task in tasks" :key="task.id" class="task-card" @click="executeTask(task)">
      <h3>{{ task.title }}</h3>
      <p>{{ task.action_type }} • {{ task.estimate_minutes }} minutes</p>
      <div class="task-meta">{{ task.meta }}</div>
    </div>
  </div>
</template>
```

### Router Integration (`frontend/src/router/index.js`)

**Context-Locked Routing:**
```javascript
executeTask(task) {
  const route = {
    name: task.action_type === 'Read' ? 'Reader' : 
          task.action_type === 'Review' && task.meta === 'flashcard' ? 'Flashcards' :
          task.action_type === 'Review' && task.meta === 'quiz' ? 'Quiz' : 'WrittenAssessment',
    params: {
      notebookId: task.topic_id,
      startPage: task.start_page,
      endPage: task.end_page
    }
  }
  this.$router.push(route)
}
```

### Component State Locking

**Reader.vue:**
- Accept `startPage` and `endPage` as route params
- Lock PDF viewer to this page range
- Show "Complete Session" button that advances `current_page_cursor` and calls `orchestrator.GetDailyAgenda()`

**Quiz/Flashcard/WrittenAssessment.vue:**
- Load only assessments for the specified `topic_id` and page range
- Show "Complete Session" button that calls `LogReview()` with aggregated score
- On completion, immediately refresh the daily agenda

### Completion Flow
1. User completes task (finishes reading, finishes review)
2. Component calls appropriate backend endpoint (`LogReview` or cursor update)
3. Backend updates database state
4. Component emits `task-completed` event
5. Dashboard automatically refreshes `GetDailyAgenda()`
6. Next task appears or shows "Mission Complete!" victory state

---

## Strict Success Criteria

### Phase 1 Success
- `LogReview()` endpoint accepts any (notebookID, pageRange, score) and correctly updates `assessment_fsrs.due_at`
- All three assessment types (flashcard, quiz, written) funnel through this single endpoint
- FSRS math produces valid `next_review` timestamps

### Phase 2 Success  
- `GetDailyAgenda()` returns 5-10 prioritized tasks
- Priority 1 always returns due reviews first
- Priority 2 returns reading continuations with calculated page targets
- Task estimates are realistic (2-8 minutes per review, 2.5 minutes per reading page)

### Phase 3 Success
- Dashboard renders ONLY the task list, no metrics
- Clicking any task routes to exact component with locked context
- "Complete Session" buttons update state and refresh agenda immediately
- User can complete entire daily queue without leaving the task flow

---

## Explicit Out of Scope (Deferred)

- Background ingestion queues
- Soft page boundaries and semantic chunking improvements  
- Acronym/Mindmap generator tools
- Documentation rewrites
- Advanced analytics or progress tracking
- Sync functionality
- Multi-device support

---

## Implementation Order

1. **Phase 1:** Build `LogReview()` endpoint first - this is the brain
2. **Phase 2:** Build `GetDailyAgenda()` - this creates the mission  
3. **Phase 3:** Rewrite Dashboard and routing - this executes the mission

**Stop after each phase for validation.** The brain must work before building the mission system. The mission system must work before building the UI.

