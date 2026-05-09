package db

import (
	"ai-tutor/internal/models"
	"errors"
	"testing"
)

func TestStudyQueueLifecycleAndState(t *testing.T) {
	initDBForTest(t, false, 0)

	if err := EnsureTopic("topic-1", "Topic 1"); err != nil {
		t.Fatalf("EnsureTopic failed: %v", err)
	}
	if err := CreateNotebook("nb-1", "NB 1", "/tmp/nb1.pdf", "pdf", "topic-1", 10); err != nil {
		t.Fatalf("CreateNotebook nb-1 failed: %v", err)
	}
	if err := UpdateNotebookPriority("nb-1", 9); err != nil {
		t.Fatalf("UpdateNotebookPriority failed: %v", err)
	}

	if err := InsertStudyTask(models.StudyQueueTask{
		ID:         "task-read",
		NotebookID: "nb-1",
		TopicID:    "topic-1",
		TaskType:   models.StudyTaskTypeReading,
		Status:     models.StudyTaskStatusPending,
		Priority:   1,
	}); err != nil {
		t.Fatalf("InsertStudyTask reading failed: %v", err)
	}
	if err := InsertStudyTask(models.StudyQueueTask{
		ID:         "task-review",
		NotebookID: "nb-1",
		TopicID:    "topic-1",
		TaskType:   models.StudyTaskTypeFlashcardReview,
		Status:     models.StudyTaskStatusPending,
		Priority:   10,
	}); err != nil {
		t.Fatalf("InsertStudyTask review failed: %v", err)
	}

	next, err := GetNextTask("nb-1")
	if err != nil {
		t.Fatalf("GetNextTask failed: %v", err)
	}
	if next.ID != "task-review" {
		t.Fatalf("expected FLASHCARD_REVIEW first, got %s", next.ID)
	}

	if err := ActivateTask(next.ID); err != nil {
		t.Fatalf("ActivateTask failed: %v", err)
	}

	if err := CompleteTask(next.ID, models.CompletionResult{
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

	state, err := GetQueueState("nb-1")
	if err != nil {
		t.Fatalf("GetQueueState failed: %v", err)
	}
	if state.Pending["READING"] != 1 || state.Pending["QUIZ"] != 1 || state.Total != 2 {
		t.Fatalf("unexpected queue state: %#v", state)
	}
}

func TestStudyQueueErrors(t *testing.T) {
	initDBForTest(t, false, 0)

	if _, err := GetNextTask(""); !errors.Is(err, ErrNoPendingTasks) {
		t.Fatalf("expected ErrNoPendingTasks, got %v", err)
	}
	if err := ActivateTask("missing"); !errors.Is(err, ErrTaskNotFound) {
		t.Fatalf("expected ErrTaskNotFound, got %v", err)
	}
	if err := SkipTask("missing"); !errors.Is(err, ErrTaskNotFound) {
		t.Fatalf("expected ErrTaskNotFound from skip, got %v", err)
	}
}

func TestStudyQueueDeterministicOrdering(t *testing.T) {
	initDBForTest(t, false, 0)

	if err := EnsureTopic("topic-a", "Topic A"); err != nil {
		t.Fatalf("EnsureTopic topic-a failed: %v", err)
	}
	if err := EnsureTopic("topic-b", "Topic B"); err != nil {
		t.Fatalf("EnsureTopic topic-b failed: %v", err)
	}
	if err := CreateNotebook("nb-a", "NB A", "/tmp/a.pdf", "pdf", "topic-a", 10); err != nil {
		t.Fatalf("CreateNotebook nb-a failed: %v", err)
	}
	if err := CreateNotebook("nb-b", "NB B", "/tmp/b.pdf", "pdf", "topic-b", 10); err != nil {
		t.Fatalf("CreateNotebook nb-b failed: %v", err)
	}
	if _, err := conn.Exec(`UPDATE notebooks SET priority = 10 WHERE id = 'nb-a'`); err != nil {
		t.Fatalf("set nb-a priority failed: %v", err)
	}
	if _, err := conn.Exec(`UPDATE notebooks SET priority = 1 WHERE id = 'nb-b'`); err != nil {
		t.Fatalf("set nb-b priority failed: %v", err)
	}

	if err := InsertStudyTask(models.StudyQueueTask{
		ID:         "t-low-notebook",
		NotebookID: "nb-b",
		TopicID:    "topic-b",
		TaskType:   models.StudyTaskTypeQuiz,
		Status:     models.StudyTaskStatusPending,
		Priority:   0,
	}); err != nil {
		t.Fatalf("Insert t-low-notebook failed: %v", err)
	}
	if err := InsertStudyTask(models.StudyQueueTask{
		ID:         "t-high-notebook",
		NotebookID: "nb-a",
		TopicID:    "topic-a",
		TaskType:   models.StudyTaskTypeQuiz,
		Status:     models.StudyTaskStatusPending,
		Priority:   0,
	}); err != nil {
		t.Fatalf("Insert t-high-notebook failed: %v", err)
	}

	next, err := GetNextTask("")
	if err != nil {
		t.Fatalf("GetNextTask failed: %v", err)
	}
	if next.ID != "t-high-notebook" {
		t.Fatalf("expected higher notebook priority task first, got %s", next.ID)
	}
}

func TestStudyQueueTaskQueriesPreservePayloadAndExposeTitle(t *testing.T) {
	initDBForTest(t, false, 0)

	if err := EnsureTopic("topic-title", "Display Topic"); err != nil {
		t.Fatalf("EnsureTopic failed: %v", err)
	}
	if err := CreateNotebook("nb-title", "Title Notebook", "/tmp/title.pdf", "pdf", "topic-title", 10); err != nil {
		t.Fatalf("CreateNotebook failed: %v", err)
	}

	pendingPayload := `{"kind":"pending"}`
	activePayload := `{"kind":"active"}`
	if err := InsertStudyTask(models.StudyQueueTask{
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
	if err := InsertStudyTask(models.StudyQueueTask{
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
	if err := ActivateTask("task-active"); err != nil {
		t.Fatalf("ActivateTask failed: %v", err)
	}

	pendingTasks, err := GetAllPendingTasks()
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

	activeTasks, err := GetAllActiveTasks()
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

	if err := EnsureTopic("topic-r", "Topic R"); err != nil {
		t.Fatalf("EnsureTopic failed: %v", err)
	}
	if err := CreateNotebook("nb-r", "NB R", "/tmp/r.pdf", "pdf", "topic-r", 12); err != nil {
		t.Fatalf("CreateNotebook failed: %v", err)
	}
	if err := InsertStudyTask(models.StudyQueueTask{
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

	task, err := GetReadingTask("task-reading")
	if err != nil {
		t.Fatalf("GetReadingTask failed: %v", err)
	}
	if task.CurrentPage != 5 {
		t.Fatalf("expected current page to initialize at start page, got %d", task.CurrentPage)
	}

	ok, err := ValidateReadingCompletion("task-reading", 7)
	if err != nil {
		t.Fatalf("ValidateReadingCompletion failed: %v", err)
	}
	if ok {
		t.Fatalf("expected ValidateReadingCompletion to return false before end page")
	}

	task, err = GetReadingTask("task-reading")
	if err != nil {
		t.Fatalf("GetReadingTask after progress failed: %v", err)
	}
	if task.CurrentPage != 7 {
		t.Fatalf("expected persisted current page 7, got %d", task.CurrentPage)
	}

	ok, err = ValidateReadingCompletion("task-reading", 8)
	if err != nil {
		t.Fatalf("ValidateReadingCompletion at end page failed: %v", err)
	}
	if !ok {
		t.Fatalf("expected ValidateReadingCompletion to return true at end page")
	}

	if err := ActivateTask("task-reading"); err != nil {
		t.Fatalf("ActivateTask failed: %v", err)
	}
	// Manual completion now allowed even before end page
	if err := CompleteReading("task-reading"); err != nil {
		t.Fatalf("CompleteReading expected to succeed for manual completion, got: %v", err)
	}

	var status string
	if err := conn.QueryRow(`SELECT status FROM study_queue WHERE id = ?`, "task-reading").Scan(&status); err != nil {
		t.Fatalf("query reading task status failed: %v", err)
	}
	if status != "COMPLETED" {
		t.Fatalf("expected reading task status COMPLETED, got %s", status)
	}

	var quizCount int
	if err := conn.QueryRow(`SELECT COUNT(*) FROM study_queue WHERE topic_id = ? AND task_type = 'QUIZ' AND status = 'PENDING'`, "topic-r").Scan(&quizCount); err != nil {
		t.Fatalf("query quiz follow-up failed: %v", err)
	}
	if quizCount != 1 {
		t.Fatalf("expected one pending QUIZ follow-up, got %d", quizCount)
	}
}
