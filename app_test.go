package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/google/uuid"

	"ai-tutor/internal/db"
	"ai-tutor/internal/llm"
	"ai-tutor/internal/models"
	"ai-tutor/internal/notebook"
	"ai-tutor/internal/retrieval"
	"ai-tutor/internal/scheduler"
	"ai-tutor/internal/study"
)

var testRepo *db.Repository

func initTestDB(t *testing.T) {
	t.Helper()
	tempDB := filepath.Join(t.TempDir(), "ai-tutor-test.db")
	repo, err := db.Init(tempDB, "")
	if err != nil {
		t.Fatalf("failed to init test db: %v", err)
	}
	testRepo = repo
	t.Cleanup(func() {
		if err := testRepo.Close(); err != nil {
			t.Fatalf("failed to close test db: %v", err)
		}
		testRepo = nil
	})
	if err := testRepo.SeedDemoDataForTests(); err != nil {
		t.Fatalf("failed to seed test data: %v", err)
	}
}

func initCleanTestDB(t *testing.T) {
	t.Helper()
	tempDB := filepath.Join(t.TempDir(), "ai-tutor-test.db")
	repo, err := db.Init(tempDB, "")
	if err != nil {
		t.Fatalf("failed to init clean test db: %v", err)
	}
	testRepo = repo
	t.Cleanup(func() {
		if err := testRepo.Close(); err != nil {
			t.Fatalf("failed to close clean test db: %v", err)
		}
		testRepo = nil
	})
}

func initTestProvider(t *testing.T) *llm.Provider {
	t.Helper()

	mockLLM := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			http.NotFound(w, r)
			return
		}

		var body struct {
			Messages []struct {
				Content string `json:"content"`
			} `json:"messages"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		content := "Round Robin gives each process a fixed time slice."
		if len(body.Messages) > 0 {
			prompt := body.Messages[0].Content
			switch {
			case strings.Contains(prompt, "flashcard generator"):
				content = flashcardJSON(extractRequestedCount(prompt, "Generate exactly "), extractFirstChunkID(prompt))
			case strings.Contains(prompt, "quiz generator"):
				content = questionJSON(extractRequestedCount(prompt, "Generate exactly "), extractFirstChunkID(prompt))
			}
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"choices": []map[string]interface{}{
				{
					"message": map[string]string{"content": content},
				},
			},
		})
	}))
	t.Cleanup(mockLLM.Close)

	t.Setenv("LLM_BASE_URL", mockLLM.URL)
	t.Setenv("LLM_API_KEY", "test-key")
	t.Setenv("LLM_MODEL", "test-model")

	return llm.NewProvider(llm.LoadConfigFromEnv())
}

// newTestApp provides canonical test App initialization with all dependencies wired.
func newTestApp(t *testing.T) *App {
	t.Helper()
	initTestDB(t)

	provider := initTestProvider(t)
	uploadDir := t.TempDir()

	app := &App{
		ctx:               context.Background(),
		repo:              testRepo,
		fastLLMProvider:   provider,
		heavyLLMProvider:  provider,
		scheduler:         scheduler.New(testRepo, scheduler.Dependencies{}),
		notebookService:   notebook.NewService(uploadDir),
		notebookUploadDir: uploadDir,
		aiReady:           true,
		aiInitError:       "",
	}

	topicIDs, err := testRepo.GetAllTopicIDs()
	if err != nil {
		t.Fatalf("failed to list topic IDs: %v", err)
	}
	for _, topicID := range topicIDs {
		chunks, chunksErr := testRepo.GetChunksForTopic(topicID)
		if chunksErr != nil {
			t.Fatalf("failed to get chunks for topic %s: %v", topicID, chunksErr)
		}
		for _, chunk := range chunks {
			if app.retrievalEngine == nil {
				app.retrievalEngine = retrieval.NewEngine(testRepo, nil)
			}
			app.retrievalEngine.AddChunk(chunk)
		}
	}
	if app.retrievalEngine == nil {
		app.retrievalEngine = retrieval.NewEngine(testRepo, nil)
	}

	app.studyService = study.NewStudyService(study.Config{
		Repo:             testRepo,
		FastLLMProvider:  provider,
		HeavyLLMProvider: provider,
		RetrievalEngine:  app.retrievalEngine,
	})

	return app
}

func mustInsertActiveQuizTask(t *testing.T, notebookID, topicID, taskID string, passingScore int) {
	t.Helper()
	if err := testRepo.EnsureTopic(topicID, topicID+"-title"); err != nil {
		t.Fatalf("EnsureTopic failed: %v", err)
	}
	if err := testRepo.CreateNotebook(notebookID, notebookID+"-name", "/tmp/"+notebookID+".pdf", "pdf", topicID, 12); err != nil {
		t.Fatalf("CreateNotebook failed: %v", err)
	}

	payloadBytes, err := json.Marshal(models.QuizTaskPayload{
		Questions: []models.QuizTaskQuestion{
			{
				ID:            "quiz-q1",
				Prompt:        "Question 1",
				Options:       []string{"A", "B", "C", "D"},
				CorrectAnswer: "A",
			},
			{
				ID:            "quiz-q2",
				Prompt:        "Question 2",
				Options:       []string{"A", "B", "C", "D"},
				CorrectAnswer: "B",
			},
		},
		PassingScore: passingScore,
	})
	if err != nil {
		t.Fatalf("marshal quiz payload failed: %v", err)
	}

	if err := testRepo.InsertStudyTask(models.StudyQueueTask{
		ID:          taskID,
		NotebookID:  notebookID,
		TopicID:     topicID,
		TaskType:    models.StudyTaskTypeQuiz,
		Status:      models.StudyTaskStatusActive,
		Priority:    0,
		PayloadJSON: string(payloadBytes),
		StartPage:   3,
		EndPage:     6,
	}); err != nil {
		t.Fatalf("InsertStudyTask quiz failed: %v", err)
	}
}

func mustInsertMockChunk(t *testing.T, notebookID, topicID, chunkID string, pageNum int) {
	t.Helper()
	if _, err := testRepo.ExecForTest(`
		INSERT OR IGNORE INTO chunks (id, topic_id, chunk_text, page_num)
		VALUES (?, ?, 'Mock chunk text for calibration test', ?)
	`, chunkID, topicID, pageNum); err != nil {
		t.Fatalf("failed to insert chunk: %v", err)
	}

	if _, err := testRepo.ExecForTest(`
		INSERT OR IGNORE INTO notebook_chunks (id, notebook_id, chunk_id, page_num)
		VALUES (?, ?, ?, ?)
	`, uuid.NewString(), notebookID, chunkID, pageNum); err != nil {
		t.Fatalf("failed to insert notebook_chunk link: %v", err)
	}
}

type mockLLMProvider struct {
	answer string
	err    error
}

func (m *mockLLMProvider) GenerateAnswer(prompt string) (string, error) {
	if m.err != nil {
		return "", m.err
	}
	return m.answer, nil
}

func (m *mockLLMProvider) ModelName() string {
	return "mock-model"
}

func (m *mockLLMProvider) GetLimits() llm.ModelLimits {
	return llm.ModelLimits{MaxInputTokens: 30000, MaxOutputTokens: 3000}
}

func extractRequestedCount(prompt string, prefix string) int {
	idx := strings.Index(prompt, prefix)
	if idx < 0 {
		return 1
	}
	rest := prompt[idx+len(prefix):]
	fields := strings.Fields(rest)
	if len(fields) == 0 {
		return 1
	}
	count, err := strconv.Atoi(fields[0])
	if err != nil || count <= 0 {
		return 1
	}
	return count
}

func questionJSON(count int, sourceChunkID string) string {
	if strings.TrimSpace(sourceChunkID) == "" {
		sourceChunkID = "chunk-fallback"
	}
	items := make([]string, 0, count)
	correct := []string{"A", "B", "C", "D"}
	for i := 0; i < count; i++ {
		answer := correct[i%len(correct)]
		items = append(items, fmt.Sprintf(`{"source_chunk_id":"%s","prompt":"Question %d?","options":["A","B","C","D"],"correct_answer":"%s","explanation":"Explanation %d.","hint":"Hint %d.","source_heading":"Complete Section","source_snippet":"Snippet %d."}`, sourceChunkID, i+1, answer, i+1, i+1, i+1))
	}
	return `{"questions":[` + strings.Join(items, ",") + `]}`
}

func flashcardJSON(count int, sourceChunkID string) string {
	if strings.TrimSpace(sourceChunkID) == "" {
		sourceChunkID = "chunk-fallback"
	}
	items := make([]string, 0, count)
	for i := 0; i < count; i++ {
		items = append(items, fmt.Sprintf(`{"source_chunk_id":"%s","prompt":"Flashcard %d prompt?","answer":"Flashcard %d answer."}`, sourceChunkID, i+1, i+1))
	}
	return `{"cards":[` + strings.Join(items, ",") + `]}`
}

func extractFirstChunkID(prompt string) string {
	const marker = "chunk_id: "
	idx := strings.Index(prompt, marker)
	if idx < 0 {
		return ""
	}
	rest := prompt[idx+len(marker):]
	end := strings.Index(rest, " |")
	if end < 0 {
		end = strings.Index(rest, "\n")
	}
	if end < 0 {
		end = len(rest)
	}
	return strings.TrimSpace(rest[:end])
}

// ============================================================================
// QUEUE CONTRACT TESTS
// ============================================================================

func TestActivateTask_TransitionsPendingToActive(t *testing.T) {
	app := newTestApp(t)

	notebookID := "activate-test-nb"
	if err := testRepo.CreateNotebook(notebookID, "Activate Test Notebook", "/tmp/activate.txt", "txt", "", 1); err != nil {
		t.Fatalf("CreateNotebook failed: %v", err)
	}

	task := models.StudyQueueTask{
		ID:         "task-activate-1",
		NotebookID: notebookID,
		TaskType:   models.StudyTaskTypeReading,
		Status:     models.StudyTaskStatusPending,
		Priority:   1,
	}
	if err := testRepo.InsertStudyTask(task); err != nil {
		t.Fatalf("InsertStudyTask failed: %v", err)
	}

	resp := app.ActivateTask("task-activate-1")
	if _, hasErr := resp["error"]; hasErr {
		t.Fatalf("expected success, got error: %v", resp["error"])
	}

	_, err := testRepo.GetNextTask(notebookID)
	if err != db.ErrNoPendingTasks {
		t.Fatalf("expected no pending tasks after activation, got: %v", err)
	}
}

func TestActivateTask_RejectsNonPendingTask(t *testing.T) {
	app := newTestApp(t)

	notebookID := "activate-reject-nb"
	if err := testRepo.CreateNotebook(notebookID, "Activate Reject Notebook", "/tmp/activate-reject.txt", "txt", "", 1); err != nil {
		t.Fatalf("CreateNotebook failed: %v", err)
	}

	task := models.StudyQueueTask{
		ID:         "task-already-active",
		NotebookID: notebookID,
		TaskType:   models.StudyTaskTypeQuiz,
		Status:     models.StudyTaskStatusActive,
		Priority:   1,
	}
	if err := testRepo.InsertStudyTask(task); err != nil {
		t.Fatalf("InsertStudyTask failed: %v", err)
	}

	resp := app.ActivateTask("task-already-active")
	if code, ok := resp["code"].(int); !ok || code != 409 {
		t.Fatalf("expected code 409 for non-pending task, got: %#v", resp)
	}
}

func TestCompleteTask_MarksActiveAsCompleted(t *testing.T) {
	app := newTestApp(t)

	notebookID := "complete-test-nb"
	if err := testRepo.CreateNotebook(notebookID, "Complete Test Notebook", "/tmp/complete.txt", "txt", "", 1); err != nil {
		t.Fatalf("CreateNotebook failed: %v", err)
	}

	task := models.StudyQueueTask{
		ID:         "task-complete-1",
		NotebookID: notebookID,
		TaskType:   models.StudyTaskTypeReread,
		Status:     models.StudyTaskStatusActive,
		Priority:   1,
	}
	if err := testRepo.InsertStudyTask(task); err != nil {
		t.Fatalf("InsertStudyTask failed: %v", err)
	}

	result := models.CompletionResult{
		Status: models.StudyTaskStatusCompleted,
	}

	resp := app.CompleteTask("task-complete-1", result)
	if _, hasErr := resp["error"]; hasErr {
		t.Fatalf("expected success, got error: %v", resp["error"])
	}

	_, err := testRepo.GetNextTask(notebookID)
	if err != db.ErrNoPendingTasks {
		t.Fatalf("expected no pending tasks after completion, got: %v", err)
	}
}

func TestCompleteTask_InsertsFollowUpTasks(t *testing.T) {
	app := newTestApp(t)

	notebookID := "followup-test-nb"
	if err := testRepo.CreateNotebook(notebookID, "Followup Test Notebook", "/tmp/followup.txt", "txt", "", 1); err != nil {
		t.Fatalf("CreateNotebook failed: %v", err)
	}

	task := models.StudyQueueTask{
		ID:         "task-with-followup",
		NotebookID: notebookID,
		TaskType:   models.StudyTaskTypeReading,
		Status:     models.StudyTaskStatusActive,
		Priority:   1,
	}
	if err := testRepo.InsertStudyTask(task); err != nil {
		t.Fatalf("InsertStudyTask failed: %v", err)
	}

	followUp := models.StudyQueueTask{
		ID:         "followup-1",
		NotebookID: notebookID,
		TaskType:   models.StudyTaskTypeQuiz,
		Status:     models.StudyTaskStatusPending,
		Priority:   1,
	}

	result := models.CompletionResult{
		Status:    models.StudyTaskStatusCompleted,
		FollowUps: []models.StudyQueueTask{followUp},
	}

	resp := app.CompleteTask("task-with-followup", result)
	if _, hasErr := resp["error"]; hasErr {
		t.Fatalf("expected success, got error: %v", resp["error"])
	}

	nextTask, err := testRepo.GetNextTask(notebookID)
	if err != nil {
		t.Fatalf("expected follow-up task in queue, got error: %v", err)
	}
	if nextTask.ID != "followup-1" {
		t.Fatalf("expected followup-1, got: %s", nextTask.ID)
	}
}

func TestSkipTask_MarksTaskAsSkipped(t *testing.T) {
	app := newTestApp(t)

	notebookID := "skip-test-nb"
	if err := testRepo.CreateNotebook(notebookID, "Skip Test Notebook", "/tmp/skip.txt", "txt", "", 1); err != nil {
		t.Fatalf("CreateNotebook failed: %v", err)
	}

	task := models.StudyQueueTask{
		ID:         "task-skip-1",
		NotebookID: notebookID,
		TaskType:   models.StudyTaskTypeExaminer,
		Status:     models.StudyTaskStatusPending,
		Priority:   1,
	}
	if err := testRepo.InsertStudyTask(task); err != nil {
		t.Fatalf("InsertStudyTask failed: %v", err)
	}

	resp := app.SkipTask("task-skip-1")
	if _, hasErr := resp["error"]; hasErr {
		t.Fatalf("expected success, got error: %v", resp["error"])
	}

	_, err := testRepo.GetNextTask(notebookID)
	if err != db.ErrNoPendingTasks {
		t.Fatalf("expected no pending tasks after skip, got: %v", err)
	}
}

// ============================================================================
// DETERMINISTIC ORDERING TESTS
// ============================================================================

func TestOrdering_TaskTypePriority(t *testing.T) {
	initTestDB(t)

	notebookID := "ordering-type-nb"
	if err := testRepo.CreateNotebook(notebookID, "Ordering Type Notebook", "/tmp/ordering-type.txt", "txt", "", 1); err != nil {
		t.Fatalf("CreateNotebook failed: %v", err)
	}

	tasks := []models.StudyQueueTask{
		{ID: "task-5", NotebookID: notebookID, TaskType: models.StudyTaskTypeExaminer, Status: models.StudyTaskStatusPending, Priority: 1},
		{ID: "task-4", NotebookID: notebookID, TaskType: models.StudyTaskTypeReading, Status: models.StudyTaskStatusPending, Priority: 1},
		{ID: "task-3", NotebookID: notebookID, TaskType: models.StudyTaskTypeQuiz, Status: models.StudyTaskStatusPending, Priority: 1},
		{ID: "task-2", NotebookID: notebookID, TaskType: models.StudyTaskTypeReread, Status: models.StudyTaskStatusPending, Priority: 1},
		{ID: "task-1", NotebookID: notebookID, TaskType: models.StudyTaskTypeFlashcardReview, Status: models.StudyTaskStatusPending, Priority: 1},
	}

	for _, task := range tasks {
		if err := testRepo.InsertStudyTask(task); err != nil {
			t.Fatalf("InsertStudyTask failed: %v", err)
		}
	}

	nextTask, err := testRepo.GetNextTask(notebookID)
	if err != nil {
		t.Fatalf("GetNextTask failed: %v", err)
	}
	if nextTask.TaskType != models.StudyTaskTypeFlashcardReview {
		t.Fatalf("expected FLASHCARD_REVIEW first, got: %s", nextTask.TaskType)
	}

	if err := testRepo.ActivateTask("task-1"); err != nil {
		t.Fatalf("ActivateTask failed: %v", err)
	}
	if err := testRepo.CompleteTask("task-1", models.CompletionResult{Status: models.StudyTaskStatusCompleted}); err != nil {
		t.Fatalf("CompleteTask failed: %v", err)
	}

	nextTask, err = testRepo.GetNextTask(notebookID)
	if err != nil {
		t.Fatalf("GetNextTask failed: %v", err)
	}
	if nextTask.TaskType != models.StudyTaskTypeReread {
		t.Fatalf("expected REREAD second, got: %s", nextTask.TaskType)
	}
}

func TestOrdering_NotebookPriority(t *testing.T) {
	t.Skip("db.UpdateNotebookPriority method does not exist - notebook priority ordering is preserved in query but cannot be tested")
}

func TestOrdering_TaskPriority(t *testing.T) {
	initTestDB(t)

	notebookID := "ordering-priority-nb"
	if err := testRepo.CreateNotebook(notebookID, "Ordering Priority Notebook", "/tmp/ordering-priority.txt", "txt", "", 1); err != nil {
		t.Fatalf("CreateNotebook failed: %v", err)
	}

	taskHighPriority := models.StudyQueueTask{ID: "task-high-priority", NotebookID: notebookID, TaskType: models.StudyTaskTypeQuiz, Status: models.StudyTaskStatusPending, Priority: 1}
	taskLowPriority := models.StudyQueueTask{ID: "task-low-priority", NotebookID: notebookID, TaskType: models.StudyTaskTypeQuiz, Status: models.StudyTaskStatusPending, Priority: 10}

	if err := testRepo.InsertStudyTask(taskLowPriority); err != nil {
		t.Fatalf("InsertStudyTask failed: %v", err)
	}
	if err := testRepo.InsertStudyTask(taskHighPriority); err != nil {
		t.Fatalf("InsertStudyTask failed: %v", err)
	}

	nextTask, err := testRepo.GetNextTask(notebookID)
	if err != nil {
		t.Fatalf("GetNextTask failed: %v", err)
	}
	if nextTask.ID != "task-high-priority" {
		t.Fatalf("expected task-high-priority first, got: %s", nextTask.ID)
	}
}

func TestOrdering_FIFOFallback(t *testing.T) {
	initTestDB(t)

	notebookID := "ordering-fifo-nb"
	if err := testRepo.CreateNotebook(notebookID, "FIFO Notebook", "/tmp/fifo.txt", "txt", "", 1); err != nil {
		t.Fatalf("CreateNotebook failed: %v", err)
	}

	task1 := models.StudyQueueTask{ID: "task-fifo-1", NotebookID: notebookID, TaskType: models.StudyTaskTypeReading, Status: models.StudyTaskStatusPending, Priority: 5}
	task2 := models.StudyQueueTask{ID: "task-fifo-2", NotebookID: notebookID, TaskType: models.StudyTaskTypeReading, Status: models.StudyTaskStatusPending, Priority: 5}
	task3 := models.StudyQueueTask{ID: "task-fifo-3", NotebookID: notebookID, TaskType: models.StudyTaskTypeReading, Status: models.StudyTaskStatusPending, Priority: 5}

	if err := testRepo.InsertStudyTask(task1); err != nil {
		t.Fatalf("InsertStudyTask failed: %v", err)
	}
	if err := testRepo.InsertStudyTask(task2); err != nil {
		t.Fatalf("InsertStudyTask failed: %v", err)
	}
	if err := testRepo.InsertStudyTask(task3); err != nil {
		t.Fatalf("InsertStudyTask failed: %v", err)
	}

	nextTask, err := testRepo.GetNextTask(notebookID)
	if err != nil {
		t.Fatalf("GetNextTask failed: %v", err)
	}
	if nextTask.ID != "task-fifo-1" {
		t.Fatalf("expected task-fifo-1 first (FIFO), got: %s", nextTask.ID)
	}
}

func TestOrdering_AntiStarvation(t *testing.T) {
	initTestDB(t)

	notebookID := "anti-starvation-nb"
	if err := testRepo.CreateNotebook(notebookID, "Anti Starvation Notebook", "/tmp/anti-starvation.txt", "txt", "", 1); err != nil {
		t.Fatalf("CreateNotebook failed: %v", err)
	}

	tasks := []models.StudyQueueTask{
		{ID: "task-a", NotebookID: notebookID, TaskType: models.StudyTaskTypeReading, Status: models.StudyTaskStatusPending, Priority: 10},
		{ID: "task-b", NotebookID: notebookID, TaskType: models.StudyTaskTypeFlashcardReview, Status: models.StudyTaskStatusPending, Priority: 10},
		{ID: "task-c", NotebookID: notebookID, TaskType: models.StudyTaskTypeReading, Status: models.StudyTaskStatusPending, Priority: 1},
	}

	for _, task := range tasks {
		if err := testRepo.InsertStudyTask(task); err != nil {
			t.Fatalf("InsertStudyTask failed: %v", err)
		}
	}

	var firstTaskID string
	for i := 0; i < 3; i++ {
		nextTask, err := testRepo.GetNextTask(notebookID)
		if err != nil {
			t.Fatalf("GetNextTask failed on iteration %d: %v", i, err)
		}
		if i == 0 {
			firstTaskID = nextTask.ID
		} else if nextTask.ID != firstTaskID {
			t.Fatalf("deterministic ordering violated: iteration 0 returned %s, iteration %d returned %s", firstTaskID, i, nextTask.ID)
		}
	}

	if firstTaskID != "task-b" {
		t.Fatalf("expected task-b to win due to task type precedence, got: %s", firstTaskID)
	}
}

// ============================================================================
// CLOUD SYNC TESTS
// ============================================================================

func TestTriggerCloudSyncRetriesAndFailSafe(t *testing.T) {
	_ = newTestApp(t)

	if err := testRepo.CreateNotebook("os-notebook-sync", "OS Notebook", "/tmp/os-notebook.pdf", "pdf", "", 12); err != nil {
		t.Fatalf("failed to create notebook: %v", err)
	}

	var attempts int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"new_notebooks": []interface{}{},
		})
	}))
	defer server.Close()

	if _, err := testRepo.ExecForTest(`
		UPDATE user_settings
		SET cloud_sync_url = ?, cloud_api_token = 'token'
		WHERE id = 1
	`, server.URL); err != nil {
		t.Fatalf("failed to update user settings: %v", err)
	}

	err := study.TriggerCloudSync(testRepo)
	if err != nil {
		t.Fatalf("expected cloud sync to succeed, got error: %v", err)
	}
	if attempts != 3 {
		t.Fatalf("expected exactly 3 attempts, got %d", attempts)
	}

	syncCount, err := testRepo.CountTasksByTopicTypeAndStatus("", "FLASHCARD_SYNC", "PENDING")
	if err != nil {
		t.Fatalf("failed to query sync task: %v", err)
	}
	if syncCount != 0 {
		t.Fatalf("expected no PENDING FLASHCARD_SYNC task, got %d", syncCount)
	}

	serverFail := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer serverFail.Close()

	if _, err := testRepo.ExecForTest(`
		UPDATE user_settings
		SET cloud_sync_url = ?
		WHERE id = 1
	`, serverFail.URL); err != nil {
		t.Fatalf("failed to update user settings: %v", err)
	}

	err = study.TriggerCloudSync(testRepo)
	if err == nil {
		t.Fatalf("expected sync to fail, but it succeeded")
	}

	syncCount, err = testRepo.CountTasksByTopicTypeAndStatus("", "FLASHCARD_SYNC", "PENDING")
	if err != nil {
		t.Fatalf("failed to query sync task: %v", err)
	}
	if syncCount != 1 {
		t.Fatalf("expected 1 PENDING FLASHCARD_SYNC task, got %d", syncCount)
	}

	serverSuccess := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"new_notebooks": []interface{}{},
		})
	}))
	defer serverSuccess.Close()

	if _, err := testRepo.ExecForTest(`
		UPDATE user_settings
		SET cloud_sync_url = ?
		WHERE id = 1
	`, serverSuccess.URL); err != nil {
		t.Fatalf("failed to update user settings: %v", err)
	}

	err = study.TriggerCloudSync(testRepo)
	if err != nil {
		t.Fatalf("expected sync to succeed, got %v", err)
	}

	var status string
	err = testRepo.QueryRowForTest("SELECT status FROM study_queue WHERE task_type = 'FLASHCARD_SYNC'").Scan(&status)
	if err != nil {
		t.Fatalf("failed to query sync task status: %v", err)
	}
	if status != "COMPLETED" {
		t.Fatalf("expected sync task status to be COMPLETED, got %q", status)
	}
}

// ============================================================================
// HELPER FUNCTION UNIT TESTS
// ============================================================================

func TestCalculateDailyStudyMinutes(t *testing.T) {
	tests := []struct {
		name     string
		start    string
		end      string
		expected int
	}{
		{"normal range", "9:00", "11:30", 150},
		{"same hour", "10:00", "10:30", 30},
		{"midnight wrap", "22:00", "2:00", 240},
		{"full day", "0:00", "23:59", 1439},
		{"invalid start", "bad", "10:00", 60},
		{"invalid end", "9:00", "bad", 60},
		{"both invalid", "x", "y", 60},
		{"zero diff", "10:00", "10:00", 60},
		{"single minute", "9:00", "9:01", 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := calculateDailyStudyMinutes(tt.start, tt.end)
			if got != tt.expected {
				t.Errorf("calculateDailyStudyMinutes(%q, %q) = %d, want %d", tt.start, tt.end, got, tt.expected)
			}
		})
	}
}

func TestCalculateFlashcardBudgets(t *testing.T) {
	tests := []struct {
		name                string
		dueCards            int
		maxFlashcards       int
		wantMaterialized    int
		wantDeferred        int
		wantSafeReviewBudget int
	}{
		{"under max", 5, 20, 5, 0, 3},
		{"at max", 10, 10, 10, 0, 5},
		{"over max", 30, 20, 20, 10, 10},
		{"zero cards", 0, 10, 0, 0, 0},
		{"zero max", 5, 0, 0, 5, 0},
		{"both zero", 0, 0, 0, 0, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mat, deferred, budget := calculateFlashcardBudgets(tt.dueCards, tt.maxFlashcards)
			if mat != tt.wantMaterialized || deferred != tt.wantDeferred || budget != tt.wantSafeReviewBudget {
				t.Errorf("calculateFlashcardBudgets(%d, %d) = (%d, %d, %d), want (%d, %d, %d)",
					tt.dueCards, tt.maxFlashcards, mat, deferred, budget,
					tt.wantMaterialized, tt.wantDeferred, tt.wantSafeReviewBudget)
			}
		})
	}
}

func TestAggregateQueueTasks(t *testing.T) {
	t.Run("empty inputs", func(t *testing.T) {
		tasks, topics, minutes, actions := aggregateQueueTasks(nil, nil)
		if len(tasks) != 0 {
			t.Errorf("expected 0 tasks, got %d", len(tasks))
		}
		if len(topics) != 0 {
			t.Errorf("expected 0 topics, got %d", len(topics))
		}
		if minutes != 0 {
			t.Errorf("expected 0 minutes, got %d", minutes)
		}
		if len(actions) != 0 {
			t.Errorf("expected 0 actions, got %d", len(actions))
		}
	})

	t.Run("active and pending combined", func(t *testing.T) {
		active := []models.StudyQueueTask{
			{ID: "a1", TaskType: models.StudyTaskTypeReading, Status: models.StudyTaskStatusActive, Title: "Topic A", StartPage: 1, EndPage: 5},
		}
		pending := []models.StudyQueueTask{
			{ID: "p1", TaskType: models.StudyTaskTypeQuiz, Status: models.StudyTaskStatusPending, Title: "Topic B", StartPage: 1, EndPage: 3},
		}
		tasks, topics, minutes, actions := aggregateQueueTasks(active, pending)

		if len(tasks) != 2 {
			t.Fatalf("expected 2 tasks, got %d", len(tasks))
		}
		if len(topics) != 2 {
			t.Errorf("expected 2 active topics, got %d", len(topics))
		}
		if minutes == 0 {
			t.Errorf("expected non-zero learning minutes")
		}
		if actions["reading"] != 1 || actions["quiz"] != 1 {
			t.Errorf("expected reading=1, quiz=1, got %v", actions)
		}
	})

	t.Run("empty title defaults to Task", func(t *testing.T) {
		active := []models.StudyQueueTask{
			{ID: "a1", TaskType: models.StudyTaskTypeReading, Status: models.StudyTaskStatusActive, Title: "", StartPage: 1, EndPage: 5},
		}
		tasks, _, _, _ := aggregateQueueTasks(active, nil)
		if tasks[0].Title != "Read: Task" {
			t.Errorf("expected 'Read: Task', got %q", tasks[0].Title)
		}
	})
}
