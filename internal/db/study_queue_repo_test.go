package db

import (
	"ai-tutor/internal/models"
	"errors"
	"fmt"
	"testing"
	"time"
)

func TestSchemaIncludesRereadAttemptsTable(t *testing.T) {
	initDBForTest(t, false, 0)

	var name string
	if err := testRepo.db.QueryRow(`
		SELECT name
		FROM sqlite_master
		WHERE type = 'table' AND name = 'reread_attempts'
	`).Scan(&name); err != nil {
		t.Fatalf("expected reread_attempts table to exist: %v", err)
	}
	if name != "reread_attempts" {
		t.Fatalf("expected reread_attempts table, got %q", name)
	}
}

func TestSchemaIncludesReviewTaskCardsTableAndIndex(t *testing.T) {
	initDBForTest(t, false, 0)

	var tableName string
	if err := testRepo.db.QueryRow(`
		SELECT name FROM sqlite_master
		WHERE type = 'table' AND name = 'review_task_cards'
	`).Scan(&tableName); err != nil {
		t.Fatalf("expected review_task_cards table to exist: %v", err)
	}
	if tableName != "review_task_cards" {
		t.Fatalf("expected review_task_cards table, got %q", tableName)
	}

	var indexName string
	if err := testRepo.db.QueryRow(`
		SELECT name FROM sqlite_master
		WHERE type = 'index' AND name = 'idx_review_task_cards_task_status'
	`).Scan(&indexName); err != nil {
		t.Fatalf("expected idx_review_task_cards_task_status index to exist: %v", err)
	}
}

func TestRereadAttemptCountHelpers(t *testing.T) {
	initDBForTest(t, false, 0)

	if err := testRepo.EnsureTopic("topic-attempts", "Topic Attempts"); err != nil {
		t.Fatalf("EnsureTopic failed: %v", err)
	}

	count, err := testRepo.GetRereadAttemptCount("topic-attempts")
	if err != nil {
		t.Fatalf("GetRereadAttemptCount initial failed: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected initial reread attempt count 0, got %d", count)
	}

	tx, err := testRepo.db.Begin()
	if err != nil {
		t.Fatalf("begin tx failed: %v", err)
	}
	count, err = testRepo.IncrementRereadAttemptCountTx(tx, "topic-attempts")
	if err != nil {
		t.Fatalf("IncrementRereadAttemptCountTx first failed: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected first increment to return 1, got %d", count)
	}
	count, err = testRepo.IncrementRereadAttemptCountTx(tx, "topic-attempts")
	if err != nil {
		t.Fatalf("IncrementRereadAttemptCountTx second failed: %v", err)
	}
	if count != 2 {
		t.Fatalf("expected second increment to return 2, got %d", count)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit increment tx failed: %v", err)
	}

	count, err = testRepo.GetRereadAttemptCount("topic-attempts")
	if err != nil {
		t.Fatalf("GetRereadAttemptCount after increment failed: %v", err)
	}
	if count != 2 {
		t.Fatalf("expected persisted reread attempt count 2, got %d", count)
	}

	tx, err = testRepo.db.Begin()
	if err != nil {
		t.Fatalf("begin reset tx failed: %v", err)
	}
	if err := testRepo.ResetRereadAttemptCountTx(tx, "topic-attempts"); err != nil {
		t.Fatalf("ResetRereadAttemptCountTx failed: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit reset tx failed: %v", err)
	}

	count, err = testRepo.GetRereadAttemptCount("topic-attempts")
	if err != nil {
		t.Fatalf("GetRereadAttemptCount after reset failed: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected reread attempt count reset to 0, got %d", count)
	}
}

func TestStudyQueueLifecycleAndState(t *testing.T) {
	initDBForTest(t, false, 0)

	if err := testRepo.EnsureTopic("topic-1", "Topic 1"); err != nil {
		t.Fatalf("EnsureTopic failed: %v", err)
	}
	if err := testRepo.CreateNotebook("nb-1", "NB 1", "/tmp/nb1.pdf", "pdf", "topic-1", 10); err != nil {
		t.Fatalf("CreateNotebook nb-1 failed: %v", err)
	}
	if err := testRepo.UpdateNotebookPriority("nb-1", 9); err != nil {
		t.Fatalf("UpdateNotebookPriority failed: %v", err)
	}

	if err := testRepo.InsertStudyTask(models.StudyQueueTask{
		ID:         "task-read",
		NotebookID: "nb-1",
		TopicID:    "topic-1",
		TaskType:   models.StudyTaskTypeReading,
		Status:     models.StudyTaskStatusPending,
		Priority:   1,
	}); err != nil {
		t.Fatalf("InsertStudyTask reading failed: %v", err)
	}
	if err := testRepo.InsertStudyTask(models.StudyQueueTask{
		ID:         "task-review",
		NotebookID: "nb-1",
		TopicID:    "topic-1",
		TaskType:   models.StudyTaskTypeFlashcardReview,
		Status:     models.StudyTaskStatusPending,
		Priority:   10,
	}); err != nil {
		t.Fatalf("InsertStudyTask review failed: %v", err)
	}

	next, err := testRepo.GetNextTask("nb-1")
	if err != nil {
		t.Fatalf("GetNextTask failed: %v", err)
	}
	if next.ID != "task-review" {
		t.Fatalf("expected FLASHCARD_REVIEW first, got %s", next.ID)
	}

	if err := testRepo.ActivateTask(next.ID); err != nil {
		t.Fatalf("ActivateTask failed: %v", err)
	}

	if err := testRepo.CompleteTask(next.ID, models.CompletionResult{
		Status: models.StudyTaskStatusCompleted,
		FollowUps: []models.StudyQueueTask{
			{
				ID:         "task-follow-up",
				NotebookID: "nb-1",
				TopicID:    "topic-1",
				TaskType:   models.StudyTaskTypeQuiz,
				Status:     models.StudyTaskStatusPending,
				Priority:   0,
			},
		},
	}); err != nil {
		t.Fatalf("CompleteTask failed: %v", err)
	}

	state, err := testRepo.GetQueueState("nb-1")
	if err != nil {
		t.Fatalf("GetQueueState failed: %v", err)
	}
	if state.Pending["READING"] != 1 || state.Pending["QUIZ"] != 1 || state.Total != 2 {
		t.Fatalf("unexpected queue state: %#v", state)
	}
}

func TestStudyQueueErrors(t *testing.T) {
	initDBForTest(t, false, 0)

	if _, err := testRepo.GetNextTask(""); !errors.Is(err, ErrNoPendingTasks) {
		t.Fatalf("expected ErrNoPendingTasks, got %v", err)
	}
	if err := testRepo.ActivateTask("missing"); !errors.Is(err, ErrTaskNotFound) {
		t.Fatalf("expected ErrTaskNotFound, got %v", err)
	}
	if err := testRepo.SkipTask("missing"); !errors.Is(err, ErrTaskNotFound) {
		t.Fatalf("expected ErrTaskNotFound from skip, got %v", err)
	}
}

func TestStudyQueueDeterministicOrdering(t *testing.T) {
	initDBForTest(t, false, 0)

	if err := testRepo.EnsureTopic("topic-a", "Topic A"); err != nil {
		t.Fatalf("EnsureTopic topic-a failed: %v", err)
	}
	if err := testRepo.EnsureTopic("topic-b", "Topic B"); err != nil {
		t.Fatalf("EnsureTopic topic-b failed: %v", err)
	}
	if err := testRepo.CreateNotebook("nb-a", "NB A", "/tmp/a.pdf", "pdf", "topic-a", 10); err != nil {
		t.Fatalf("CreateNotebook nb-a failed: %v", err)
	}
	if err := testRepo.CreateNotebook("nb-b", "NB B", "/tmp/b.pdf", "pdf", "topic-b", 10); err != nil {
		t.Fatalf("CreateNotebook nb-b failed: %v", err)
	}
	if _, err := testRepo.db.Exec(`UPDATE notebooks SET priority = 10 WHERE id = 'nb-a'`); err != nil {
		t.Fatalf("set nb-a priority failed: %v", err)
	}
	if _, err := testRepo.db.Exec(`UPDATE notebooks SET priority = 1 WHERE id = 'nb-b'`); err != nil {
		t.Fatalf("set nb-b priority failed: %v", err)
	}

	if err := testRepo.InsertStudyTask(models.StudyQueueTask{
		ID:         "t-low-notebook",
		NotebookID: "nb-b",
		TopicID:    "topic-b",
		TaskType:   models.StudyTaskTypeQuiz,
		Status:     models.StudyTaskStatusPending,
		Priority:   0,
	}); err != nil {
		t.Fatalf("Insert t-low-notebook failed: %v", err)
	}
	if err := testRepo.InsertStudyTask(models.StudyQueueTask{
		ID:         "t-high-notebook",
		NotebookID: "nb-a",
		TopicID:    "topic-a",
		TaskType:   models.StudyTaskTypeQuiz,
		Status:     models.StudyTaskStatusPending,
		Priority:   0,
	}); err != nil {
		t.Fatalf("Insert t-high-notebook failed: %v", err)
	}

	next, err := testRepo.GetNextTask("")
	if err != nil {
		t.Fatalf("GetNextTask failed: %v", err)
	}
	if next.ID != "t-high-notebook" {
		t.Fatalf("expected higher notebook priority task first, got %s", next.ID)
	}
}

func TestStudyQueueTaskQueriesPreservePayloadAndExposeTitle(t *testing.T) {
	initDBForTest(t, false, 0)

	if err := testRepo.EnsureTopic("topic-title", "Display Topic"); err != nil {
		t.Fatalf("EnsureTopic failed: %v", err)
	}
	if err := testRepo.CreateNotebook("nb-title", "Title Notebook", "/tmp/title.pdf", "pdf", "topic-title", 10); err != nil {
		t.Fatalf("CreateNotebook failed: %v", err)
	}

	pendingPayload := `{"kind":"pending"}`
	activePayload := `{"kind":"active"}`
	if err := testRepo.InsertStudyTask(models.StudyQueueTask{
		ID:          "task-pending",
		NotebookID:  "nb-title",
		TopicID:     "topic-title",
		TaskType:    models.StudyTaskTypeQuiz,
		Status:      models.StudyTaskStatusPending,
		Priority:    1,
		PayloadJSON: pendingPayload,
	}); err != nil {
		t.Fatalf("Insert pending task failed: %v", err)
	}
	if err := testRepo.InsertStudyTask(models.StudyQueueTask{
		ID:          "task-active",
		NotebookID:  "nb-title",
		TopicID:     "topic-title",
		TaskType:    models.StudyTaskTypeQuiz,
		Status:      models.StudyTaskStatusPending,
		Priority:    2,
		PayloadJSON: activePayload,
	}); err != nil {
		t.Fatalf("Insert active task failed: %v", err)
	}
	if err := testRepo.ActivateTask("task-active"); err != nil {
		t.Fatalf("ActivateTask failed: %v", err)
	}

	pendingTasks, err := testRepo.GetAllPendingTasks()
	if err != nil {
		t.Fatalf("GetAllPendingTasks failed: %v", err)
	}
	var pendingTask *models.StudyQueueTask
	for i := range pendingTasks {
		if pendingTasks[i].ID == "task-pending" {
			pendingTask = &pendingTasks[i]
			break
		}
	}
	if pendingTask == nil {
		t.Fatalf("pending task not found in GetAllPendingTasks result: %#v", pendingTasks)
	}
	if pendingTask.PayloadJSON != pendingPayload {
		t.Fatalf("expected pending payload to remain intact, got %q", pendingTask.PayloadJSON)
	}
	if pendingTask.Title != "Display Topic" {
		t.Fatalf("expected pending task title to use topic title, got %q", pendingTask.Title)
	}

	activeTasks, err := testRepo.GetAllActiveTasks()
	if err != nil {
		t.Fatalf("GetAllActiveTasks failed: %v", err)
	}
	var activeTask *models.StudyQueueTask
	for i := range activeTasks {
		if activeTasks[i].ID == "task-active" {
			activeTask = &activeTasks[i]
			break
		}
	}
	if activeTask == nil {
		t.Fatalf("active task not found in GetAllActiveTasks result: %#v", activeTasks)
	}
	if activeTask.PayloadJSON != activePayload {
		t.Fatalf("expected active payload to remain intact, got %q", activeTask.PayloadJSON)
	}
	if activeTask.Title != "Display Topic" {
		t.Fatalf("expected active task title to use topic title, got %q", activeTask.Title)
	}
}

func TestReadingTaskProgressValidationAndCompletion(t *testing.T) {
	initDBForTest(t, false, 0)

	if err := testRepo.EnsureTopic("topic-r", "Topic R"); err != nil {
		t.Fatalf("EnsureTopic failed: %v", err)
	}
	if err := testRepo.CreateNotebook("nb-r", "NB R", "/tmp/r.pdf", "pdf", "topic-r", 12); err != nil {
		t.Fatalf("CreateNotebook failed: %v", err)
	}
	if err := testRepo.InsertStudyTask(models.StudyQueueTask{
		ID:         "task-reading",
		NotebookID: "nb-r",
		TopicID:    "topic-r",
		TaskType:   models.StudyTaskTypeReading,
		Status:     models.StudyTaskStatusPending,
		Priority:   1,
		StartPage:  5,
		EndPage:    8,
	}); err != nil {
		t.Fatalf("InsertStudyTask reading failed: %v", err)
	}

	task, err := testRepo.GetReadingTask("task-reading")
	if err != nil {
		t.Fatalf("GetReadingTask failed: %v", err)
	}
	if task.CurrentPage != 5 {
		t.Fatalf("expected current page to initialize at start page, got %d", task.CurrentPage)
	}

	ok, err := testRepo.PersistReadingProgress("task-reading", 7)
	if err != nil {
		t.Fatalf("PersistReadingProgress failed: %v", err)
	}
	if ok {
		t.Fatalf("expected PersistReadingProgress to return false before end page")
	}

	task, err = testRepo.GetReadingTask("task-reading")
	if err != nil {
		t.Fatalf("GetReadingTask after progress failed: %v", err)
	}
	if task.CurrentPage != 7 {
		t.Fatalf("expected persisted current page 7, got %d", task.CurrentPage)
	}

	ok, err = testRepo.PersistReadingProgress("task-reading", 8)
	if err != nil {
		t.Fatalf("PersistReadingProgress at end page failed: %v", err)
	}
	if !ok {
		t.Fatalf("expected PersistReadingProgress to return true at end page")
	}
}

func TestCompleteReadingWithGeneratedQuizAdvancesTopicCursorToTaskEnd(t *testing.T) {
	initDBForTest(t, false, 0)

	if err := testRepo.EnsureTopic("topic-cursor", "Topic Cursor"); err != nil {
		t.Fatalf("EnsureTopic failed: %v", err)
	}
	if err := testRepo.CreateNotebook("nb-cursor", "NB Cursor", "/tmp/cursor.pdf", "pdf", "topic-cursor", 60); err != nil {
		t.Fatalf("CreateNotebook failed: %v", err)
	}
	if err := testRepo.UpdateTopicPageBounds("topic-cursor", 1, 60); err != nil {
		t.Fatalf("UpdateTopicPageBounds failed: %v", err)
	}
	if err := testRepo.InsertStudyTask(models.StudyQueueTask{
		ID:         "task-cursor",
		NotebookID: "nb-cursor",
		TopicID:    "topic-cursor",
		TaskType:   models.StudyTaskTypeReading,
		Status:     models.StudyTaskStatusPending,
		Priority:   1,
		StartPage:  21,
		EndPage:    49,
	}); err != nil {
		t.Fatalf("InsertStudyTask reading failed: %v", err)
	}
	if err := testRepo.ActivateTask("task-cursor"); err != nil {
		t.Fatalf("ActivateTask failed: %v", err)
	}

	// Persist partial progress to simulate trust-based completion without explicit final-page sync.
	if _, err := testRepo.PersistReadingProgress("task-cursor", 21); err != nil {
		t.Fatalf("PersistReadingProgress failed: %v", err)
	}

	quizTaskID, err := testRepo.CompleteReadingWithGeneratedQuiz("task-cursor", models.QuizTaskPayload{
		Questions: []models.QuizTaskQuestion{
			{
				ID:            "q1",
				Prompt:        "Prompt",
				Options:       []string{"A", "B"},
				CorrectAnswer: "A",
			},
		},
		PassingScore: 70,
	})
	if err != nil {
		t.Fatalf("CompleteReadingWithGeneratedQuiz failed: %v", err)
	}
	if quizTaskID == "" {
		t.Fatalf("expected quiz task id to be returned")
	}

	var cursor int
	if err := testRepo.db.QueryRow(`SELECT COALESCE(current_page_cursor, 0) FROM topics WHERE id = ?`, "topic-cursor").Scan(&cursor); err != nil {
		t.Fatalf("query topic cursor failed: %v", err)
	}
	if cursor != 49 {
		t.Fatalf("expected cursor advanced to task end page 49, got %d", cursor)
	}
}

func TestRereadTaskCanBeLoadedAndCompletedThroughReaderHelpers(t *testing.T) {
	initDBForTest(t, false, 0)

	if err := testRepo.EnsureTopic("topic-reread", "Topic Reread"); err != nil {
		t.Fatalf("EnsureTopic failed: %v", err)
	}
	if err := testRepo.UpdateTopicPageBounds("topic-reread", 10, 14); err != nil {
		t.Fatalf("UpdateTopicPageBounds failed: %v", err)
	}
	if err := testRepo.CreateNotebook("nb-reread", "NB Reread", "/tmp/reread.pdf", "pdf", "topic-reread", 20); err != nil {
		t.Fatalf("CreateNotebook failed: %v", err)
	}
	if err := testRepo.InsertStudyTask(models.StudyQueueTask{
		ID:         "task-reread-reader",
		NotebookID: "nb-reread",
		TopicID:    "topic-reread",
		TaskType:   models.StudyTaskTypeReread,
		Status:     models.StudyTaskStatusPending,
		Priority:   1,
		StartPage:  10,
		EndPage:    14,
	}); err != nil {
		t.Fatalf("InsertStudyTask reread failed: %v", err)
	}
	if err := testRepo.ActivateTask("task-reread-reader"); err != nil {
		t.Fatalf("ActivateTask failed: %v", err)
	}

	task, err := testRepo.GetReadingTask("task-reread-reader")
	if err != nil {
		t.Fatalf("GetReadingTask reread failed: %v", err)
	}
	if task.StartPage != 10 || task.EndPage != 14 {
		t.Fatalf("unexpected reread task bounds: %#v", task)
	}

	quizTaskID, err := testRepo.CompleteReadingWithGeneratedQuiz("task-reread-reader", models.QuizTaskPayload{
		PassingScore: 70,
		Questions: []models.QuizTaskQuestion{{
			ID:            "reread-q1",
			Prompt:        "Q1",
			Options:       []string{"A", "B"},
			CorrectAnswer: "A",
		}},
	})
	if err != nil {
		t.Fatalf("CompleteReadingWithGeneratedQuiz reread failed: %v", err)
	}

	var status string
	if err := testRepo.db.QueryRow(`SELECT status FROM study_queue WHERE id = ?`, "task-reread-reader").Scan(&status); err != nil {
		t.Fatalf("query reread task status failed: %v", err)
	}
	if status != "COMPLETED" {
		t.Fatalf("expected reread task status COMPLETED, got %s", status)
	}

	var quizCount int
	if err := testRepo.db.QueryRow(`
		SELECT COUNT(*)
		FROM study_queue
		WHERE topic_id = ? AND task_type = 'QUIZ' AND status = 'PENDING'
	`, "topic-reread").Scan(&quizCount); err != nil {
		t.Fatalf("query reread follow-up quiz failed: %v", err)
	}
	if quizCount != 1 {
		t.Fatalf("expected one follow-up QUIZ after reread completion, got %d", quizCount)
	}
	if quizTaskID == "" {
		t.Fatalf("expected quiz task ID to be set")
	}
	fetchedTask, err := testRepo.GetTaskByID(quizTaskID)
	if err != nil {
		t.Fatalf("failed to fetch task by ID: %v", err)
	}
	if fetchedTask == nil {
		t.Fatalf("expected task to be found, got nil")
	}
	if fetchedTask.TaskType != models.StudyTaskTypeQuiz {
		t.Fatalf("expected task type %s, got %s", models.StudyTaskTypeQuiz, fetchedTask.TaskType)
	}
	if fetchedTask.ID != quizTaskID {
		t.Fatalf("expected task ID %s, got %s", quizTaskID, fetchedTask.ID)
	}
}

func TestCreateReviewSessionDueCardBatchingAndDuplicatePrevention(t *testing.T) {
	initDBForTest(t, false, 0)

	if err := testRepo.EnsureTopic("topic-review-a", "Review Topic A"); err != nil {
		t.Fatalf("EnsureTopic A failed: %v", err)
	}
	if err := testRepo.EnsureTopic("topic-review-b", "Review Topic B"); err != nil {
		t.Fatalf("EnsureTopic B failed: %v", err)
	}
	if err := testRepo.CreateNotebook("nb-review", "NB Review", "/tmp/review.pdf", "pdf", "", 30); err != nil {
		t.Fatalf("CreateNotebook failed: %v", err)
	}
	if _, err := testRepo.db.Exec(`INSERT INTO notebook_topics (notebook_id, topic_id) VALUES ('nb-review', 'topic-review-a')`); err != nil {
		t.Fatalf("link topic-review-a failed: %v", err)
	}

	cards := make([]models.Flashcard, 0, 24)
	states := make(map[string]models.FlashcardState)
	for i := 0; i < 22; i++ {
		id := "due-card-" + string(rune('a'+i))
		cards = append(cards, models.Flashcard{
			ID:        id,
			TopicID:   "topic-review-a",
			Prompt:    id,
			Answer:    "answer",
			DueAt:     int64(100 + i),
			Suspended: false,
		})
		states[id] = models.FlashcardState{}
	}
	cards = append(cards,
		models.Flashcard{ID: "future-card", TopicID: "topic-review-a", Prompt: "future", Answer: "future", DueAt: 5000, Suspended: false},
		models.Flashcard{ID: "suspended-card", TopicID: "topic-review-a", Prompt: "suspended", Answer: "suspended", DueAt: 50, Suspended: true},
		models.Flashcard{ID: "other-notebook-card", TopicID: "topic-review-b", Prompt: "other", Answer: "other", DueAt: 10, Suspended: false},
	)
	states["future-card"] = models.FlashcardState{}
	states["suspended-card"] = models.FlashcardState{}
	states["other-notebook-card"] = models.FlashcardState{}
	if err := testRepo.CreateFlashcards("topic-review-a", cards[:24], states); err != nil {
		t.Fatalf("CreateFlashcards topic-review-a failed: %v", err)
	}
	if err := testRepo.CreateFlashcards("topic-review-b", []models.Flashcard{cards[24]}, states); err != nil {
		t.Fatalf("CreateFlashcards topic-review-b failed: %v", err)
	}

	dueCards, err := testRepo.GetDueReviewCardsForNotebook("nb-review", 1000, 20)
	if err != nil {
		t.Fatalf("GetDueReviewCardsForNotebook failed: %v", err)
	}
	if len(dueCards) != 20 {
		t.Fatalf("expected due-card batch capped at 20, got %d", len(dueCards))
	}
	if dueCards[0].ID != "due-card-a" || dueCards[19].ID != "due-card-t" {
		t.Fatalf("unexpected deterministic due-card ordering: first=%s last=%s", dueCards[0].ID, dueCards[19].ID)
	}

	task, existing, err := testRepo.CreateReviewSession("nb-review")
	if err != nil {
		t.Fatalf("CreateReviewSession failed: %v", err)
	}
	if existing {
		t.Fatalf("expected first CreateReviewSession to create a new task")
	}
	if task == nil {
		t.Fatalf("expected review task to be created")
	}

	var linkedCount int
	if err := testRepo.db.QueryRow(`SELECT COUNT(*) FROM review_task_cards WHERE task_id = ?`, task.ID).Scan(&linkedCount); err != nil {
		t.Fatalf("count review_task_cards failed: %v", err)
	}
	if linkedCount != 23 {
		t.Fatalf("expected 23 linked review cards, got %d", linkedCount)
	}

	task2, existing2, err := testRepo.CreateReviewSession("nb-review")
	if err != nil {
		t.Fatalf("second CreateReviewSession failed: %v", err)
	}
	if !existing2 {
		t.Fatalf("expected second CreateReviewSession to return existing task")
	}
	if task2 == nil || task2.ID != task.ID {
		t.Fatalf("expected duplicate prevention to return task %s, got %#v", task.ID, task2)
	}
	assertCountEquals(t, `SELECT COUNT(*) FROM study_queue WHERE notebook_id = ? AND task_type = 'FLASHCARD_REVIEW'`, "nb-review", 1)
}

func TestReviewSessionRecoveryOrderingAndCompletion(t *testing.T) {
	initDBForTest(t, false, 0)

	if err := testRepo.EnsureTopic("topic-session", "Review Session Topic"); err != nil {
		t.Fatalf("EnsureTopic failed: %v", err)
	}
	if err := testRepo.CreateNotebook("nb-session", "NB Session", "/tmp/session.pdf", "pdf", "", 20); err != nil {
		t.Fatalf("CreateNotebook failed: %v", err)
	}
	if _, err := testRepo.db.Exec(`INSERT INTO notebook_topics (notebook_id, topic_id) VALUES ('nb-session', 'topic-session')`); err != nil {
		t.Fatalf("link topic failed: %v", err)
	}
	if err := testRepo.CreateFlashcards("topic-session", []models.Flashcard{
		{ID: "card-1", TopicID: "topic-session", Prompt: "Q1", Answer: "A1", DueAt: 10},
		{ID: "card-2", TopicID: "topic-session", Prompt: "Q2", Answer: "A2", DueAt: 20},
		{ID: "card-3", TopicID: "topic-session", Prompt: "Q3", Answer: "A3", DueAt: 30},
	}, map[string]models.FlashcardState{
		"card-1": {},
		"card-2": {},
		"card-3": {},
	}); err != nil {
		t.Fatalf("CreateFlashcards failed: %v", err)
	}

	task, _, err := testRepo.CreateReviewSession("nb-session")
	if err != nil {
		t.Fatalf("CreateReviewSession failed: %v", err)
	}
	if err := testRepo.ActivateTask(task.ID); err != nil {
		t.Fatalf("ActivateTask failed: %v", err)
	}

	if _, err := testRepo.db.Exec(`
		UPDATE review_task_cards SET status = 'reviewed'
		WHERE task_id = ? AND card_id = 'card-1'
	`, task.ID); err != nil {
		t.Fatalf("seed reviewed link failed: %v", err)
	}

	session, err := testRepo.GetReviewSession(task.ID)
	if err != nil {
		t.Fatalf("GetReviewSession failed: %v", err)
	}
	if session.Remaining != 2 || session.ReviewedCount != 1 {
		t.Fatalf("unexpected session counts: %#v", session)
	}
	if session.NextPendingIdx != 0 || session.CurrentCard == nil || session.CurrentCard.CardID != "card-2" {
		t.Fatalf("expected next pending card-2 first, got %#v", session.CurrentCard)
	}
	if session.Cards[2].CardID != "card-1" || session.Cards[2].Status != models.ReviewTaskCardStatusReviewed {
		t.Fatalf("expected reviewed card moved after pending cards, got %#v", session.Cards)
	}

	if err := testRepo.CompleteReviewSession(task.ID); !errors.Is(err, ErrReviewSessionOpen) {
		t.Fatalf("expected ErrReviewSessionOpen before all cards reviewed, got %v", err)
	}

	if _, err := testRepo.db.Exec(`UPDATE review_task_cards SET status = 'reviewed' WHERE task_id = ?`, task.ID); err != nil {
		t.Fatalf("mark all reviewed failed: %v", err)
	}
	if err := testRepo.CompleteReviewSession(task.ID); err != nil {
		t.Fatalf("CompleteReviewSession failed: %v", err)
	}

	var status string
	if err := testRepo.db.QueryRow(`SELECT status FROM study_queue WHERE id = ?`, task.ID).Scan(&status); err != nil {
		t.Fatalf("query task status failed: %v", err)
	}
	if status != "COMPLETED" {
		t.Fatalf("expected COMPLETED task, got %s", status)
	}
}

func TestCreateReviewSessionResolvesLegacyNotebookTopicContext(t *testing.T) {
	initDBForTest(t, false, 0)

	if err := testRepo.EnsureTopic("topic-legacy-review", "Legacy Review Topic"); err != nil {
		t.Fatalf("EnsureTopic failed: %v", err)
	}
	if err := testRepo.CreateNotebook("nb-legacy-review", "Legacy NB", "/tmp/legacy.pdf", "pdf", "topic-legacy-review", 12); err != nil {
		t.Fatalf("CreateNotebook failed: %v", err)
	}
	if err := testRepo.CreateFlashcards("topic-legacy-review", []models.Flashcard{
		{ID: "legacy-card-1", TopicID: "topic-legacy-review", Prompt: "Q1", Answer: "A1", DueAt: 10},
		{ID: "legacy-card-2", TopicID: "topic-legacy-review", Prompt: "Q2", Answer: "A2", DueAt: 20},
	}, map[string]models.FlashcardState{
		"legacy-card-1": {},
		"legacy-card-2": {},
	}); err != nil {
		t.Fatalf("CreateFlashcards failed: %v", err)
	}

	task, existing, err := testRepo.CreateReviewSession("nb-legacy-review")
	if err != nil {
		t.Fatalf("CreateReviewSession failed: %v", err)
	}
	if existing {
		t.Fatalf("expected new session for legacy-linked notebook")
	}
	if task == nil || task.NotebookID != "nb-legacy-review" {
		t.Fatalf("expected task for notebook nb-legacy-review, got %#v", task)
	}

	var linkedCount int
	if err := testRepo.db.QueryRow(`SELECT COUNT(*) FROM review_task_cards WHERE task_id = ?`, task.ID).Scan(&linkedCount); err != nil {
		t.Fatalf("count review_task_cards failed: %v", err)
	}
	if linkedCount != 2 {
		t.Fatalf("expected 2 linked review cards, got %d", linkedCount)
	}
}

func TestStudyQueueNewPriorityLevels(t *testing.T) {
	initDBForTest(t, false, 0)

	topicID := "topic-priority"
	notebookID := "nb-priority"

	if err := testRepo.EnsureTopic(topicID, "Priority Topic"); err != nil {
		t.Fatalf("EnsureTopic failed: %v", err)
	}
	if err := testRepo.CreateNotebook(notebookID, "Priority Notebook", "/tmp/priority.pdf", "pdf", topicID, 5); err != nil {
		t.Fatalf("CreateNotebook failed: %v", err)
	}

	taskTypes := []models.StudyTaskType{
		models.StudyTaskTypeExaminer,
		models.StudyTaskTypeSocraticRemedial,
		models.StudyTaskTypeReading,
		models.StudyTaskTypeQuiz,
		models.StudyTaskTypeReread,
		models.StudyTaskTypeFlashcardReview,
		models.StudyTaskTypeFlashcardSync,
	}

	// Insert all task types in reverse-priority or arbitrary order to test queue sorting
	for i, taskType := range taskTypes {
		taskID := fmt.Sprintf("task-%d", i)
		if err := testRepo.InsertStudyTask(models.StudyQueueTask{
			ID:         taskID,
			NotebookID: notebookID,
			TopicID:    topicID,
			TaskType:   taskType,
			Status:     models.StudyTaskStatusPending,
			Priority:   1, // keep same priority to test task type precedence
		}); err != nil {
			t.Fatalf("InsertStudyTask %s failed: %v", taskType, err)
		}
	}

	// Expected order (highest priority first)
	expectedOrder := []models.StudyTaskType{
		models.StudyTaskTypeFlashcardSync,
		models.StudyTaskTypeSocraticRemedial,
		models.StudyTaskTypeFlashcardReview,
		models.StudyTaskTypeReread,
		models.StudyTaskTypeQuiz,
		models.StudyTaskTypeReading,
		models.StudyTaskTypeExaminer,
	}

	for _, expectedType := range expectedOrder {
		next, err := testRepo.GetNextTask(notebookID)
		if err != nil {
			t.Fatalf("GetNextTask failed: %v", err)
		}
		if next.TaskType != expectedType {
			t.Fatalf("expected next task type to be %s, got %s", expectedType, next.TaskType)
		}
		// Activate task first
		if err := testRepo.ActivateTask(next.ID); err != nil {
			t.Fatalf("ActivateTask failed: %v", err)
		}
		// Complete task to get the next one in queue
		if err := testRepo.CompleteTask(next.ID, models.CompletionResult{Status: models.StudyTaskStatusCompleted}); err != nil {
			t.Fatalf("CompleteTask failed: %v", err)
		}
	}
}

func TestGetCompletedTaskTimes(t *testing.T) {
	initDBForTest(t, false, 0)

	notebookID := "nb-streak-test"
	topicID := "topic-streak-test"
	if err := testRepo.EnsureTopic(topicID, "Test Topic"); err != nil {
		t.Fatalf("EnsureTopic failed: %v", err)
	}
	if err := testRepo.CreateNotebook(notebookID, "Test Notebook", "/tmp/streak.pdf", "pdf", topicID, 5); err != nil {
		t.Fatalf("CreateNotebook failed: %v", err)
	}

	// Insert active, pending and completed tasks
	task1 := models.StudyQueueTask{
		ID:         "task-streak-1",
		NotebookID: notebookID,
		TopicID:    topicID,
		TaskType:   models.StudyTaskTypeReading,
		Status:     models.StudyTaskStatusPending,
	}
	task2 := models.StudyQueueTask{
		ID:         "task-streak-2",
		NotebookID: notebookID,
		TopicID:    topicID,
		TaskType:   models.StudyTaskTypeQuiz,
		Status:     models.StudyTaskStatusPending,
	}

	if err := testRepo.InsertStudyTask(task1); err != nil {
		t.Fatalf("InsertStudyTask 1 failed: %v", err)
	}
	if err := testRepo.InsertStudyTask(task2); err != nil {
		t.Fatalf("InsertStudyTask 2 failed: %v", err)
	}

	// Fetch initial completions (should be 0)
	completions, err := testRepo.GetCompletedTaskTimes()
	if err != nil {
		t.Fatalf("GetCompletedTaskTimes initial failed: %v", err)
	}
	if len(completions) != 0 {
		t.Fatalf("expected 0 completions, got %d", len(completions))
	}

	// Activate and complete task 1
	if err := testRepo.ActivateTask(task1.ID); err != nil {
		t.Fatalf("ActivateTask failed: %v", err)
	}
	if err := testRepo.CompleteTask(task1.ID, models.CompletionResult{Status: models.StudyTaskStatusCompleted}); err != nil {
		t.Fatalf("CompleteTask failed: %v", err)
	}

	// Fetch completions (should be 1)
	completions, err = testRepo.GetCompletedTaskTimes()
	if err != nil {
		t.Fatalf("GetCompletedTaskTimes after complete failed: %v", err)
	}
	if len(completions) != 1 {
		t.Fatalf("expected 1 completion, got %d", len(completions))
	}

	// Verify that the timestamp is close to now
	timeDiff := time.Since(completions[0])
	if timeDiff < 0 {
		timeDiff = -timeDiff
	}
	if timeDiff > 1*time.Minute {
		t.Fatalf("expected completed time to be close to now, but diff is %v (completed time: %v, now: %v)", timeDiff, completions[0], time.Now().UTC())
	}
}

