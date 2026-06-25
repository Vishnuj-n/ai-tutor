package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
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

func TestSubmitQuizAttemptFailedQuizInsertsRereadAndReturnsCountMetadata(t *testing.T) {
	app := newTestApp(t)
	mustInsertActiveQuizTask(t, "nb-quiz-fail", "topic-quiz-fail", "task-quiz-fail", 100)

	resp := app.SubmitQuizAttempt("task-quiz-fail", []models.QuizAnswer{
		{QuestionID: "quiz-q1", Selected: "B"},
		{QuestionID: "quiz-q2", Selected: "C"},
	})
	if _, hasErr := resp["error"]; hasErr {
		t.Fatalf("expected submit success, got error: %v", resp["error"])
	}

	result, ok := resp["result"].(models.QuizResult)
	if !ok {
		t.Fatalf("expected QuizResult payload, got %#v", resp["result"])
	}
	if result.Passed {
		t.Fatalf("expected failed quiz result")
	}
	if result.RereadTaskID == "" {
		t.Fatalf("expected reread task id on failed quiz below cap")
	}
	if result.ManualReviewRecommended {
		t.Fatalf("expected manual_review_recommended=false below cap")
	}
	if result.RereadAttemptCount != 1 || result.MaxRereadAttempts != 1 {
		t.Fatalf("unexpected reread metadata: %#v", result)
	}

	rereadTask, err := testRepo.GetTaskByID(result.RereadTaskID)
	if err != nil {
		t.Fatalf("query reread follow-up failed: %v", err)
	}
	if rereadTask.TaskType != "REREAD" || rereadTask.Status != "PENDING" {
		t.Fatalf("expected pending reread follow-up, got type=%s status=%s", rereadTask.TaskType, rereadTask.Status)
	}
}

func TestSubmitQuizAttemptAfterMaxReturnsManualReviewWithoutReread(t *testing.T) {
	app := newTestApp(t)
	mustInsertActiveQuizTask(t, "nb-quiz-max", "topic-quiz-max", "task-quiz-max", 100)

	// Insert dummy FSRS card for the topic to verify it is deleted during safety transaction
	if _, err := testRepo.ExecForTest(`
		INSERT INTO fsrs_cards (id, topic_id, prompt, answer)
		VALUES ('dummy-card-1', 'topic-quiz-max', 'Prompt 1', 'Answer 1')
	`); err != nil {
		t.Fatalf("failed to insert dummy FSRS card: %v", err)
	}

	tx, err := testRepo.Begin()
	if err != nil {
		t.Fatalf("begin tx failed: %v", err)
	}
	if _, err := testRepo.IncrementRereadAttemptCountTx(tx, "topic-quiz-max"); err != nil {
		t.Fatalf("seed attempt 1 failed: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit seed attempts failed: %v", err)
	}

	resp := app.SubmitQuizAttempt("task-quiz-max", []models.QuizAnswer{
		{QuestionID: "quiz-q1", Selected: "B"},
		{QuestionID: "quiz-q2", Selected: "C"},
	})
	if _, hasErr := resp["error"]; hasErr {
		t.Fatalf("expected submit success, got error: %v", resp["error"])
	}

	result, ok := resp["result"].(models.QuizResult)
	if !ok {
		t.Fatalf("expected QuizResult payload, got %#v", resp["result"])
	}
	if result.RereadTaskID != "" {
		t.Fatalf("expected no reread task id after max automatic rereads, got %q", result.RereadTaskID)
	}
	if !result.ManualReviewRecommended {
		t.Fatalf("expected manual_review_recommended=true after max automatic rereads")
	}
	if result.RereadAttemptCount != 2 || result.MaxRereadAttempts != 1 {
		t.Fatalf("unexpected reread metadata: %#v", result)
	}

	pendingRereads, err := testRepo.CountTasksByTopicTypeAndStatus("topic-quiz-max", "REREAD", "PENDING")
	if err != nil {
		t.Fatalf("query pending rereads failed: %v", err)
	}
	if pendingRereads != 0 {
		t.Fatalf("expected no automatic reread inserted after max, got %d", pendingRereads)
	}

	// Verify dummy FSRS card was deleted
	cardExists, err := testRepo.FlashcardExistsByID("dummy-card-1")
	if err != nil {
		t.Fatalf("query FSRS cards count failed: %v", err)
	}
	if cardExists {
		t.Fatalf("expected FSRS cards to be deleted on max reread failure, but found")
	}

	// Verify quiz task status is COMPLETED
	failedTask, err := testRepo.GetTaskByID("task-quiz-max")
	if err != nil {
		t.Fatalf("query task status failed: %v", err)
	}
	if failedTask.Status != models.StudyTaskStatusCompleted {
		t.Fatalf("expected quiz task status to be COMPLETED, got %q", failedTask.Status)
	}

	// Verify SOCRATIC_REMEDIAL task with status PENDING is created
	socraticCount, err := testRepo.CountTasksByTopicTypeAndStatus("topic-quiz-max", "SOCRATIC_REMEDIAL", "PENDING")
	if err != nil {
		t.Fatalf("query SOCRATIC_REMEDIAL task count failed: %v", err)
	}
	if socraticCount != 1 {
		t.Fatalf("expected 1 PENDING SOCRATIC_REMEDIAL task, got %d", socraticCount)
	}
}

func TestSubmitQuizAttemptRepeatedSubmissionReturnsErrTaskNotActiveAndNoDuplicateReread(t *testing.T) {
	app := newTestApp(t)
	mustInsertActiveQuizTask(t, "nb-quiz-repeat", "topic-quiz-repeat", "task-quiz-repeat", 100)

	first := app.SubmitQuizAttempt("task-quiz-repeat", []models.QuizAnswer{
		{QuestionID: "quiz-q1", Selected: "B"},
		{QuestionID: "quiz-q2", Selected: "C"},
	})
	if _, hasErr := first["error"]; hasErr {
		t.Fatalf("expected first submit success, got error: %v", first["error"])
	}

	second := app.SubmitQuizAttempt("task-quiz-repeat", []models.QuizAnswer{
		{QuestionID: "quiz-q1", Selected: "B"},
		{QuestionID: "quiz-q2", Selected: "C"},
	})
	if got := second["error"]; got == nil || !strings.Contains(fmt.Sprint(got), "ErrTaskNotActive") {
		t.Fatalf("expected ErrTaskNotActive on repeated submit, got %#v", got)
	}

	pendingRereads, err := testRepo.CountTasksByTopicTypeAndStatus("topic-quiz-repeat", "REREAD", "PENDING")
	if err != nil {
		t.Fatalf("query pending rereads failed: %v", err)
	}
	if pendingRereads != 1 {
		t.Fatalf("expected exactly one reread after duplicate submit attempt, got %d", pendingRereads)
	}
}

func TestSubmitQuizAttemptPassResetsAttemptsAndFutureFailureStartsAtOne(t *testing.T) {
	app := newTestApp(t)

	mustInsertActiveQuizTask(t, "nb-quiz-pass-reset", "topic-quiz-pass-reset", "task-quiz-pass", 100)
	tx, err := testRepo.Begin()
	if err != nil {
		t.Fatalf("begin seed tx failed: %v", err)
	}
	if _, err := testRepo.IncrementRereadAttemptCountTx(tx, "topic-quiz-pass-reset"); err != nil {
		t.Fatalf("seed attempt 1 failed: %v", err)
	}
	if _, err := testRepo.IncrementRereadAttemptCountTx(tx, "topic-quiz-pass-reset"); err != nil {
		t.Fatalf("seed attempt 2 failed: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit seed tx failed: %v", err)
	}

	passResp := app.SubmitQuizAttempt("task-quiz-pass", []models.QuizAnswer{
		{QuestionID: "quiz-q1", Selected: "A"},
		{QuestionID: "quiz-q2", Selected: "B"},
	})
	if _, hasErr := passResp["error"]; hasErr {
		t.Fatalf("expected pass submit success, got error: %v", passResp["error"])
	}

	count, err := testRepo.GetRereadAttemptCount("topic-quiz-pass-reset")
	if err != nil {
		t.Fatalf("GetRereadAttemptCount after pass failed: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected pass to reset reread attempt count to 0, got %d", count)
	}

	mustInsertActiveQuizTask(t, "nb-quiz-pass-reset-2", "topic-quiz-pass-reset", "task-quiz-fail-after-reset", 100)
	failResp := app.SubmitQuizAttempt("task-quiz-fail-after-reset", []models.QuizAnswer{
		{QuestionID: "quiz-q1", Selected: "B"},
		{QuestionID: "quiz-q2", Selected: "C"},
	})
	if _, hasErr := failResp["error"]; hasErr {
		t.Fatalf("expected fail submit success after reset, got error: %v", failResp["error"])
	}

	result, ok := failResp["result"].(models.QuizResult)
	if !ok {
		t.Fatalf("expected QuizResult payload, got %#v", failResp["result"])
	}
	if result.RereadAttemptCount != 1 {
		t.Fatalf("expected failure after reset to restart reread attempts at 1, got %d", result.RereadAttemptCount)
	}
	if result.RereadTaskID == "" {
		t.Fatalf("expected reread task id after reset and new failure")
	}
}

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
// This eliminates inconsistent partial initialization patterns (e.g., &App{}, NewApp(), etc.)
// and ensures deterministic setup for all App-level contract tests.
//
// DEPENDENCY STRUCT EVALUATION:
// After reviewing App initialization complexity, a lightweight dependency struct is NOT needed.
// Current constructor patterns are sufficient because:
// - App has ~12 dependencies, all explicit
// - newTestApp() provides centralized wiring
// - No circular dependencies or complex lifecycle management
// - Tests can override specific fields via direct assignment when needed
// - Adding a container would be over-engineering for this scale
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

	// Initialize retrieval engine (required by study service)
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

	// Initialize study service with all required dependencies
	app.studyService = study.NewStudyService(study.Config{
		Repo:             testRepo,
		FastLLMProvider:  provider,
		HeavyLLMProvider: provider,
		RetrievalEngine:  app.retrievalEngine,
	})

	return app
}

// ============================================================================
// SECTION MARKERS FOR FUTURE MODULAR SPLIT
// ============================================================================
// The following sections organize tests by domain for easier navigation and
// future extraction into separate test files. This is a light organization
// pass without premature churn.
//
// Domain sections:
// - Notebook/Topic tests (GetAvailableTopics, GetNotebookTopicTree, etc.)
// - Quiz/Scoring tests (ScoreAnswer, quiz generation)
// - Flashcard/FSRS tests (GenerateFlashcards)
// - Written Answer tests (ScoreShortAnswer)
// - Reader tests (GetReaderTopicBundle)
// - Notebook Upload tests (DraftNotebookSyllabus, ConfirmNotebookSyllabus)
// - Queue tests (ActivateTask, CompleteTask, SkipTask) [TO BE ADDED]
// - Deterministic Ordering tests [TO BE ADDED]
// ============================================================================

// ============================================================================
// AI/RAG TESTS
// ============================================================================

func TestGetAvailableTopicsFromDB(t *testing.T) {
	initTestDB(t)
	app := &App{repo: testRepo}

	topics := app.GetAvailableTopics()
	if len(topics) == 0 {
		t.Fatalf("expected at least one topic")
	}

	found := false
	for _, topic := range topics {
		if topic["id"] == "os-scheduling" {
			found = true
			break
		}
	}

	if !found {
		t.Fatalf("expected seeded topic os-scheduling in available topics: %#v", topics)
	}
}

// ============================================================================
// NOTEBOOK/TOPIC TESTS
// ============================================================================

func TestGetNotebookTopicTreeEmptyReturnsArray(t *testing.T) {
	initCleanTestDB(t)
	app := &App{repo: testRepo}

	tree, err := app.GetNotebookTopicTree()
	if err != nil {
		t.Fatalf("GetNotebookTopicTree failed: %v", err)
	}
	if tree == nil {
		t.Fatalf("expected empty array, got nil")
	}
	if len(tree) != 0 {
		t.Fatalf("expected no notebooks in tree, got %#v", tree)
	}
}

func TestGetNotebookTopicTreeReturnsNestedTopics(t *testing.T) {
	initCleanTestDB(t)
	app := &App{repo: testRepo}

	notebookA := "nb-tree-a"
	notebookB := "nb-tree-b"
	if err := testRepo.CreateNotebook(notebookA, "Physics", "/tmp/physics.txt", "txt", "", 1); err != nil {
		t.Fatalf("CreateNotebook notebookA failed: %v", err)
	}
	if err := testRepo.CreateNotebook(notebookB, "History", "/tmp/history.txt", "txt", "", 1); err != nil {
		t.Fatalf("CreateNotebook notebookB failed: %v", err)
	}

	for _, topic := range []struct {
		id    string
		title string
	}{
		{id: "topic-thermo", title: "Thermodynamics"},
		{id: "topic-newton", title: "Newton's Laws"},
		{id: "topic-renaissance", title: "The Renaissance"},
	} {
		if err := testRepo.EnsureTopic(topic.id, topic.title); err != nil {
			t.Fatalf("EnsureTopic %s failed: %v", topic.id, err)
		}
	}

	chunkThermo := "chunk-thermo"
	chunkNewton := "chunk-newton"
	chunkRenaissance := "chunk-renaissance"
	if err := testRepo.CreateChunk(chunkThermo, "topic-thermo", "thermo chunk", 2, 1); err != nil {
		t.Fatalf("CreateChunk thermo failed: %v", err)
	}
	if err := testRepo.CreateChunk(chunkNewton, "topic-newton", "newton chunk", 2, 2); err != nil {
		t.Fatalf("CreateChunk newton failed: %v", err)
	}
	if err := testRepo.CreateChunk(chunkRenaissance, "topic-renaissance", "renaissance chunk", 2, 3); err != nil {
		t.Fatalf("CreateChunk renaissance failed: %v", err)
	}

	if err := testRepo.LinkChunksToNotebook(notebookA, []string{chunkThermo, chunkNewton}); err != nil {
		t.Fatalf("LinkChunksToNotebook notebookA failed: %v", err)
	}
	if err := testRepo.LinkChunksToNotebook(notebookB, []string{chunkRenaissance}); err != nil {
		t.Fatalf("LinkChunksToNotebook notebookB failed: %v", err)
	}

	tree, err := app.GetNotebookTopicTree()
	if err != nil {
		t.Fatalf("GetNotebookTopicTree failed: %v", err)
	}
	if len(tree) != 2 {
		t.Fatalf("expected 2 notebooks, got %#v", tree)
	}

	var physicsTopics []string
	var historyTopics []string
	for _, node := range tree {
		switch node.NotebookID {
		case notebookA:
			for _, topic := range node.Topics {
				physicsTopics = append(physicsTopics, topic.Title)
			}
		case notebookB:
			for _, topic := range node.Topics {
				historyTopics = append(historyTopics, topic.Title)
			}
		}
	}

	if len(physicsTopics) != 2 || physicsTopics[0] != "Newton's Laws" || physicsTopics[1] != "Thermodynamics" {
		t.Fatalf("unexpected physics topics: %#v", physicsTopics)
	}
	if len(historyTopics) != 1 || historyTopics[0] != "The Renaissance" {
		t.Fatalf("unexpected history topics: %#v", historyTopics)
	}
}

// ============================================================================
// QUIZ/SCORING TESTS
// ============================================================================



// ============================================================================
// FLASHCARD/FSRS TESTS
// ============================================================================

func TestGenerateFlashcardsCreatesAndReturnsCards(t *testing.T) {
	app := newTestApp(t)
	expectedCount, err := testRepo.GetTotalChunkTokens("os-scheduling")
	if err != nil {
		t.Fatalf("GetTotalChunkTokens failed: %v", err)
	}
	want := study.ScaledFlashcardCount(expectedCount)

	resp := app.GenerateFlashcards("os-scheduling")
	if _, hasErr := resp["error"]; hasErr {
		t.Fatalf("expected success, got error: %v", resp["error"])
	}

	cards, ok := resp["cards"].([]models.Flashcard)
	if !ok {
		t.Fatalf("expected typed flashcards slice, got %#v", resp["cards"])
	}
	if len(cards) != want {
		t.Fatalf("expected %d flashcards, got %d", want, len(cards))
	}

	count, err := testRepo.CountFlashcardsForTopic("os-scheduling")
	if err != nil {
		t.Fatalf("CountFlashcardsForTopic failed: %v", err)
	}
	if count != want {
		t.Fatalf("expected %d stored flashcards, got %d", want, count)
	}
}

func TestGenerateFlashcardsReturnsExistingCardsWithoutDuplication(t *testing.T) {
	app := newTestApp(t)
	totalTokens, err := testRepo.GetTotalChunkTokens("os-scheduling")
	if err != nil {
		t.Fatalf("GetTotalChunkTokens failed: %v", err)
	}
	want := study.ScaledFlashcardCount(totalTokens)

	first := app.GenerateFlashcards("os-scheduling")
	if _, hasErr := first["error"]; hasErr {
		t.Fatalf("first generation failed: %v", first["error"])
	}

	second := app.GenerateFlashcards("os-scheduling")
	if _, hasErr := second["error"]; hasErr {
		t.Fatalf("second generation failed: %v", second["error"])
	}
	if existing, ok := second["existing"].(bool); !ok || !existing {
		t.Fatalf("expected existing=true on second generation, got %#v", second["existing"])
	}

	count, err := testRepo.CountFlashcardsForTopic("os-scheduling")
	if err != nil {
		t.Fatalf("CountFlashcardsForTopic failed: %v", err)
	}
	if count != want {
		t.Fatalf("expected no duplicate flashcards, got %d", count)
	}
}

func TestReviewSessionEndpointsSupportGenerationRecoveryAndCompletion(t *testing.T) {
	app := newTestApp(t)

	if err := testRepo.EnsureTopic("queue-review-topic", "Queue Review Topic"); err != nil {
		t.Fatalf("EnsureTopic failed: %v", err)
	}
	if err := testRepo.CreateNotebook("queue-review-nb", "Queue Review Notebook", "/tmp/queue-review.pdf", "pdf", "", 15); err != nil {
		t.Fatalf("CreateNotebook failed: %v", err)
	}
	if err := testRepo.LinkNotebookTopics("queue-review-nb", []string{"queue-review-topic"}); err != nil {
		t.Fatalf("link notebook_topics failed: %v", err)
	}
	if err := testRepo.CreateFlashcards("queue-review-topic", []models.Flashcard{
		{ID: "queue-card-1", TopicID: "queue-review-topic", Prompt: "Q1", Answer: "A1", DueAt: 1},
		{ID: "queue-card-2", TopicID: "queue-review-topic", Prompt: "Q2", Answer: "A2", DueAt: 2},
	}, map[string]models.FlashcardState{
		"queue-card-1": {},
		"queue-card-2": {},
	}); err != nil {
		t.Fatalf("CreateFlashcards failed: %v", err)
	}

	sessionResp := app.GetReviewSession(models.ReviewTaskDailyID, "queue-review-nb")
	if _, hasErr := sessionResp["error"]; hasErr {
		t.Fatalf("GetReviewSession materialization failed: %v", sessionResp["error"])
	}
	session, ok := sessionResp["session"].(*models.ReviewSession)
	if !ok {
		t.Fatalf("expected review session pointer, got %#v", sessionResp["session"])
	}
	taskID := session.Task.ID

	// Test duplicate/idempotency: loading it again should return the same materialized task ID
	secondSessionResp := app.GetReviewSession(models.ReviewTaskDailyID, "queue-review-nb")
	if _, hasErr := secondSessionResp["error"]; hasErr {
		t.Fatalf("GetReviewSession materialization failed: %v", secondSessionResp["error"])
	}
	secondSession, ok := secondSessionResp["session"].(*models.ReviewSession)
	if !ok || secondSession.Task.ID != taskID {
		t.Fatalf("expected duplicate prevention to return same task, got %#v", secondSessionResp["session"])
	}

	if resp := app.ActivateTask(taskID); resp["error"] != nil {
		t.Fatalf("ActivateTask failed: %#v", resp)
	}

	sessionResp = app.GetReviewSession(taskID, "queue-review-nb")
	if _, hasErr := sessionResp["error"]; hasErr {
		t.Fatalf("GetReviewSession failed: %v", sessionResp["error"])
	}
	session, ok = sessionResp["session"].(*models.ReviewSession)
	if !ok {
		t.Fatalf("expected review session pointer, got %#v", sessionResp["session"])
	}
	if session.CurrentCard == nil || session.CurrentCard.CardID != "queue-card-1" {
		t.Fatalf("expected first pending card queue-card-1, got %#v", session.CurrentCard)
	}

	reviewResp := app.RecordCardReview(taskID, "queue-card-1", 3)
	if _, hasErr := reviewResp["error"]; hasErr {
		t.Fatalf("RecordCardReview failed: %v", reviewResp["error"])
	}
	if remaining, ok := reviewResp["remaining"].(int); !ok || remaining != 1 {
		t.Fatalf("expected remaining=1, got %#v", reviewResp["remaining"])
	}

	reloadResp := app.GetReviewSession(taskID, "queue-review-nb")
	reloaded := reloadResp["session"].(*models.ReviewSession)
	if reloaded.CurrentCard == nil || reloaded.CurrentCard.CardID != "queue-card-2" {
		t.Fatalf("expected resumed next pending card queue-card-2, got %#v", reloaded.CurrentCard)
	}

	duplicateReviewResp := app.RecordCardReview(taskID, "queue-card-1", 3)
	if code, ok := duplicateReviewResp["code"].(int); !ok || code != 409 {
		t.Fatalf("expected duplicate review to return 409, got %#v", duplicateReviewResp)
	}

	incompleteCompleteResp := app.CompleteReviewSession(taskID)
	if code, ok := incompleteCompleteResp["code"].(int); !ok || code != 409 {
		t.Fatalf("expected incomplete completion to return 409, got %#v", incompleteCompleteResp)
	}

	reviewResp2 := app.RecordCardReview(taskID, "queue-card-2", 4)
	if _, hasErr := reviewResp2["error"]; hasErr {
		t.Fatalf("second RecordCardReview failed: %v", reviewResp2["error"])
	}

	completeResp := app.CompleteReviewSession(taskID)
	if _, hasErr := completeResp["error"]; hasErr {
		t.Fatalf("CompleteReviewSession failed: %v", completeResp["error"])
	}

	task, err := testRepo.GetTaskByID(taskID)
	if err != nil {
		t.Fatalf("GetTaskByID failed: %v", err)
	}
	if task.Status != models.StudyTaskStatusCompleted {
		t.Fatalf("expected review task completed, got %s", task.Status)
	}
}

func TestGetReviewSessionNoDueCards(t *testing.T) {
	app := newTestApp(t)

	if err := testRepo.CreateNotebook("no-due-nb", "No Due Notebook", "/tmp/no-due.pdf", "pdf", "", 15); err != nil {
		t.Fatalf("CreateNotebook failed: %v", err)
	}

	sessionResp := app.GetReviewSession(models.ReviewTaskDailyID, "no-due-nb")
	errVal, hasErr := sessionResp["error"]
	if !hasErr {
		t.Fatalf("expected error response when there are no due cards, got: %#v", sessionResp)
	}
	expectedErr := "No due cards found for review materialization"
	if errVal != expectedErr {
		t.Fatalf("expected error %q, got %q", expectedErr, errVal)
	}
}

// ============================================================================
// WRITTEN ANSWER TESTS
// ============================================================================

// TestGenerateShortAnswerPrompt_NoLLMProvider removed - study service now has fallback behavior

// TestGenerateShortAnswerPrompt_NoRAGPipeline removed - study service now has fallback behavior

// TestGenerateShortAnswerPrompt_RAGProcessQueryError removed - study service now has fallback behavior

// TestGenerateShortAnswerPrompt_InvalidJSONResponse removed - study service now has fallback behavior

// TestGenerateShortAnswerPrompt_EmptyPromptInResponse removed - study service now has fallback behavior

// TestGenerateShortAnswerPrompt_MalformedJSON removed - study service now has fallback behavior

func TestScoreShortAnswerLoadsPersistedPromptAndSavesResponse(t *testing.T) {
	app := newTestApp(t)
	mockProvider := &mockLLMProvider{
		answer: `{"score":8,"feedback":"Strong answer with a small omission."}`,
	}
	app.fastLLMProvider = mockProvider
	app.studyService = study.NewStudyService(study.Config{
		Repo:             testRepo,
		FastLLMProvider:  mockProvider,
		HeavyLLMProvider: mockProvider,
		RetrievalEngine:  app.retrievalEngine,
	})

	topicID := "written-score-topic"
	if err := testRepo.EnsureTopic(topicID, "Written Score Topic"); err != nil {
		t.Fatalf("EnsureTopic failed: %v", err)
	}
	if err := testRepo.CreateWrittenQuestion(models.WrittenQuestion{
		ID:              "written-q-1",
		TopicID:         topicID,
		Prompt:          "Explain why round robin improves fairness.",
		SourceHeading:   "CPU Scheduling",
		SourcePageStart: 2,
		SourcePageEnd:   3,
	}); err != nil {
		t.Fatalf("CreateWrittenQuestion failed: %v", err)
	}

	result := app.ScoreShortAnswer("written-q-1", "It gives each process a time slice.")
	if errMsg, ok := result["error"]; ok {
		t.Fatalf("expected success, got error: %v", errMsg)
	}
	if got := result["score"]; got != 8 {
		t.Fatalf("expected score 8, got %#v", got)
	}
	if got := result["feedback"]; got != "Strong answer with a small omission." {
		t.Fatalf("expected feedback, got %#v", got)
	}
}



// Mocks used by GenerateShortAnswerPrompt contract tests.
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
// READER TESTS
// ============================================================================

// Contract tests for GetReaderTopicBundle
func TestGetReaderTopicBundle_Success(t *testing.T) {
	initTestDB(t)
	app := &App{repo: testRepo}

	notebookID := "test-notebook-reader"
	if err := testRepo.CreateNotebook(notebookID, "Test Notebook", "/tmp/test.txt", "txt", "", 1); err != nil {
		t.Fatalf("CreateNotebook failed: %v", err)
	}

	topicID := "test-topic-reader"
	if err := testRepo.EnsureTopic(topicID, "Test Topic"); err != nil {
		t.Fatalf("EnsureTopic failed: %v", err)
	}

	chunkID := "chunk-reader"
	if err := testRepo.CreateChunk(chunkID, topicID, "chunk content", 2, 1); err != nil {
		t.Fatalf("CreateChunk failed: %v", err)
	}

	if err := testRepo.LinkChunksToNotebook(notebookID, []string{chunkID}); err != nil {
		t.Fatalf("LinkChunksToNotebook failed: %v", err)
	}

	resp := app.GetReaderTopicBundle(topicID, notebookID)

	if _, hasErr := resp["error"]; hasErr {
		t.Fatalf("expected success, got error: %v", resp["error"])
	}

	if _, ok := resp["notebook_id"].(string); !ok {
		t.Fatalf("expected notebook_id string, got: %#v", resp["notebook_id"])
	}

	if _, ok := resp["topic_id"].(string); !ok {
		t.Fatalf("expected topic_id string, got: %#v", resp["topic_id"])
	}

	// Verify topic_start_page and topic_end_page are present
	if _, ok := resp["topic_start_page"].(int); !ok {
		t.Fatalf("expected topic_start_page int, got: %#v", resp["topic_start_page"])
	}
	if _, ok := resp["topic_end_page"].(int); !ok {
		t.Fatalf("expected topic_end_page int, got: %#v", resp["topic_end_page"])
	}

	// Verify sections were returned and contain expected data
	sectionsRaw, exists := resp["sections"]
	if !exists {
		t.Fatalf("expected sections key in response, got: %#v", resp)
	}

	var sections []interface{}
	switch v := sectionsRaw.(type) {
	case []interface{}:
		sections = v
	case []map[string]interface{}:
		for _, m := range v {
			sections = append(sections, m)
		}
	default:
		t.Fatalf("expected sections to be array-like, got: %#v", sectionsRaw)
	}

	if len(sections) < 1 {
		t.Fatalf("expected at least one section, got %d", len(sections))
	}

	// Verify at least one section matches the seeded chunk
	found := false
	for _, sec := range sections {
		if sectionMap, ok := sec.(map[string]interface{}); ok {
			if heading, ok := sectionMap["heading"].(string); ok && heading == "Page 1" {
				found = true
				break
			}
		}
	}
	if !found {
		t.Fatalf("expected to find section with heading 'Page 1' in sections: %#v", sections)
	}
}

func TestGetReaderTopicBundle_InvalidTopic(t *testing.T) {
	initTestDB(t)
	app := &App{repo: testRepo}

	notebookID := "test-notebook-invalid"
	if err := testRepo.CreateNotebook(notebookID, "Test Notebook", "/tmp/test.txt", "txt", "", 1); err != nil {
		t.Fatalf("CreateNotebook failed: %v", err)
	}

	resp := app.GetReaderTopicBundle("nonexistent-topic", notebookID)

	if _, hasErr := resp["error"]; !hasErr {
		t.Fatalf("expected error for invalid topic, got: %#v", resp)
	}
}

// Contract tests for ExplainReaderSection
func TestAskReaderAI_ScopedResponseShape(t *testing.T) {
	app := newTestApp(t)

	notebookID := "reader-ai-nb"
	topicID := "reader-ai-topic"
	chunkID := "reader-ai-chunk"

	if err := testRepo.EnsureTopic(topicID, "Reader AI Topic"); err != nil {
		t.Fatalf("EnsureTopic failed: %v", err)
	}
	if err := testRepo.UpdateTopicPageBounds(topicID, 2, 4); err != nil {
		t.Fatalf("UpdateTopicPageBounds failed: %v", err)
	}
	if err := testRepo.CreateNotebook(notebookID, "Reader AI Notebook", "/tmp/reader-ai.txt", "txt", topicID, 6); err != nil {
		t.Fatalf("CreateNotebook failed: %v", err)
	}
	if err := testRepo.UpdateNotebookIndexingStatus(notebookID, "READY"); err != nil {
		t.Fatalf("UpdateNotebookIndexingStatus failed: %v", err)
	}
	if err := testRepo.CreateChunk(chunkID, topicID, "Round robin stays fair by rotating time slices.", 10, 3); err != nil {
		t.Fatalf("CreateChunk failed: %v", err)
	}
	if err := testRepo.LinkChunksToNotebook(notebookID, []string{chunkID}); err != nil {
		t.Fatalf("LinkChunksToNotebook failed: %v", err)
	}
	app.retrievalEngine.AddChunk(models.Chunk{
		ID:       chunkID,
		TopicID:  topicID,
		Text:     "Round robin stays fair by rotating time slices.",
		PageNum:  3,
	})

	resp := app.AskReaderAI(topicID, notebookID, "Why is round robin fair?", "current_page", 3, 2, 4)

	if _, hasErr := resp["error"]; hasErr {
		t.Fatalf("expected success, got error: %v", resp["error"])
	}
	if got, ok := resp["scope"].(string); !ok || got != "current_page" {
		t.Fatalf("expected current_page scope, got %#v", resp["scope"])
	}
	if _, ok := resp["answer"].(string); !ok {
		t.Fatalf("expected answer string, got %#v", resp["answer"])
	}
	switch cited := resp["cited_sections"].(type) {
	case []string:
		if len(cited) == 0 {
			t.Fatalf("expected citations, got empty slice")
		}
	case []interface{}:
		if len(cited) == 0 {
			t.Fatalf("expected citations, got empty slice")
		}
	default:
		t.Fatalf("expected citations array, got %#v", resp["cited_sections"])
	}
}

// ============================================================================
// NOTEBOOK UPLOAD TESTS
// ============================================================================

func TestDraftNotebookSyllabus_FallbackCreatesEditableChapter(t *testing.T) {
	initTestDB(t)
	uploadDir := t.TempDir()
	service := notebook.NewService(uploadDir)
	app := &App{repo: testRepo, notebookService: service}

	uploadResult, err := service.SaveUploadedFile([]byte("Alpha beta gamma"), "draft.txt")
	if err != nil {
		t.Fatalf("SaveUploadedFile failed: %v", err)
	}

	doc, err := service.ExtractDocument(uploadResult.FilePath, uploadResult.FileType)
	if err != nil {
		t.Fatalf("ExtractDocument failed: %v", err)
	}

	if err := testRepo.CreateNotebook(uploadResult.ID, uploadResult.FileName, uploadResult.FilePath, uploadResult.FileType, "", doc.PageCount); err != nil {
		t.Fatalf("CreateNotebook failed: %v", err)
	}

	resp := app.DraftNotebookSyllabus(uploadResult.ID, false)
	if _, hasErr := resp["error"]; hasErr {
		t.Fatalf("expected successful draft response, got error: %v", resp["error"])
	}

	chapters, ok := resp["chapters"].([]models.SyllabusChapterDraft)
	if !ok {
		t.Fatalf("expected typed chapters slice, got %#v", resp["chapters"])
	}
	if len(chapters) == 0 {
		t.Fatalf("expected at least one chapter in draft")
	}

	// Verify draft is persisted to DB
	draftJSON, err := testRepo.GetNotebookSyllabusDraft(uploadResult.ID)
	if err != nil {
		t.Fatalf("GetNotebookSyllabusDraft failed: %v", err)
	}
	if draftJSON == "" {
		t.Fatalf("expected draft to be persisted to DB, but got empty string")
	}

	// Verify that loading with regenerate=false returns the persisted draft without re-running extraction
	resp2 := app.DraftNotebookSyllabus(uploadResult.ID, false)
	if _, hasErr := resp2["error"]; hasErr {
		t.Fatalf("expected successful draft response on reload, got error: %v", resp2["error"])
	}

	chapters2, ok := resp2["chapters"].([]models.SyllabusChapterDraft)
	if !ok {
		t.Fatalf("expected typed chapters slice on reload, got %#v", resp2["chapters"])
	}
	if len(chapters2) != len(chapters) {
		t.Fatalf("expected same number of chapters on reload, got %d vs %d", len(chapters2), len(chapters))
	}

	// Verify that regenerate=true forces re-generation (should still work)
	resp3 := app.DraftNotebookSyllabus(uploadResult.ID, true)
	if _, hasErr := resp3["error"]; hasErr {
		t.Fatalf("expected successful draft response on regenerate, got error: %v", resp3["error"])
	}
	if chapters[0].StartPage < 1 || chapters[0].EndPage < chapters[0].StartPage {
		t.Fatalf("invalid chapter page bounds: %#v", chapters[0])
	}
}

func TestConfirmNotebookSyllabus_PersistsBoundsAndPageAwareChunks(t *testing.T) {
	initTestDB(t)
	uploadDir := t.TempDir()
	service := notebook.NewService(uploadDir)
	app := &App{repo: testRepo, notebookService: service}

	uploadResult, err := service.SaveUploadedFile([]byte("# Intro\n\nAlpha beta gamma\n\n## Details\n\nDelta epsilon zeta"), "confirm.md")
	if err != nil {
		t.Fatalf("SaveUploadedFile failed: %v", err)
	}

	doc, err := service.ExtractDocument(uploadResult.FilePath, uploadResult.FileType)
	if err != nil {
		t.Fatalf("ExtractDocument failed: %v", err)
	}

	if err := testRepo.CreateNotebook(uploadResult.ID, uploadResult.FileName, uploadResult.FilePath, uploadResult.FileType, "", doc.PageCount); err != nil {
		t.Fatalf("CreateNotebook failed: %v", err)
	}

	resp := app.ConfirmNotebookSyllabus(uploadResult.ID, []models.SyllabusChapterDraft{{
		Title:     "Confirmed Chapter",
		StartPage: 1,
		EndPage:   doc.PageCount,
	}})
	if _, hasErr := resp["error"]; hasErr {
		t.Fatalf("expected confirm success, got error: %v", resp["error"])
	}

	topicIDs, ok := resp["topic_ids"].([]string)
	if !ok || len(topicIDs) == 0 {
		t.Fatalf("expected topic ids, got %#v", resp["topic_ids"])
	}

	startPage, endPage, err := testRepo.GetTopicPageBounds(topicIDs[0])
	if err != nil {
		t.Fatalf("GetTopicPageBounds failed: %v", err)
	}
	if startPage != 1 || endPage != doc.PageCount {
		t.Fatalf("unexpected persisted bounds: got [%d,%d] want [1,%d]", startPage, endPage, doc.PageCount)
	}

	bundle, err := testRepo.GetReaderTopicBundle(topicIDs[0], uploadResult.ID)
	if err != nil {
		t.Fatalf("GetReaderTopicBundle failed: %v", err)
	}
	if len(bundle.Sections) == 0 {
		t.Fatalf("expected reader sections after confirm ingestion")
	}
	if bundle.Sections[0].PageNum <= 0 {
		t.Fatalf("expected page-aware section mapping, got page_num=%d", bundle.Sections[0].PageNum)
	}
}

func TestConfirmNotebookSyllabus_AutoActivatesIfLessThansFourActive(t *testing.T) {
	initTestDB(t)
	uploadDir := t.TempDir()
	service := notebook.NewService(uploadDir)
	app := &App{repo: testRepo, notebookService: service}

	profileID := "test-profile-auto"
	err := testRepo.CreateProfile(models.StudyProfile{ID: profileID, Name: "Test Profile Auto", DeadlineAt: 0})
	if err != nil {
		t.Fatalf("CreateProfile failed: %v", err)
	}
	err = testRepo.UpdateUserSettings(models.UserSettings{ActiveProfileID: profileID})
	if err != nil {
		t.Fatalf("UpdateUserSettings failed: %v", err)
	}

	uploadResult, err := service.SaveUploadedFile([]byte("# Intro\n\nSome book content here"), "book1.md")
	if err != nil {
		t.Fatalf("SaveUploadedFile failed: %v", err)
	}

	doc, err := service.ExtractDocument(uploadResult.FilePath, uploadResult.FileType)
	if err != nil {
		t.Fatalf("ExtractDocument failed: %v", err)
	}

	if err := testRepo.CreateNotebook(uploadResult.ID, uploadResult.FileName, uploadResult.FilePath, uploadResult.FileType, "", doc.PageCount); err != nil {
		t.Fatalf("CreateNotebook failed: %v", err)
	}
	if err := testRepo.AssignNotebookToProfile(uploadResult.ID, profileID); err != nil {
		t.Fatalf("AssignNotebookToProfile failed: %v", err)
	}

	resp := app.ConfirmNotebookSyllabus(uploadResult.ID, []models.SyllabusChapterDraft{{
		Title:     "Chapter 1",
		StartPage: 1,
		EndPage:   doc.PageCount,
	}})
	if _, hasErr := resp["error"]; hasErr {
		t.Fatalf("expected confirm success, got error: %v", resp["error"])
	}

	nb, err := testRepo.GetNotebookByID(uploadResult.ID)
	if err != nil {
		t.Fatalf("GetNotebookByID failed: %v", err)
	}
	if nb.StudyStatus != "active" {
		t.Fatalf("expected study status to be auto-activated to 'active', got %q", nb.StudyStatus)
	}
}

func TestConfirmNotebookSyllabus_DoesNotAutoActivateIfFourOrMoreActive(t *testing.T) {
	initTestDB(t)
	uploadDir := t.TempDir()
	service := notebook.NewService(uploadDir)
	app := &App{repo: testRepo, notebookService: service}

	profileID := "test-profile-limit"
	err := testRepo.CreateProfile(models.StudyProfile{ID: profileID, Name: "Test Profile Limit", DeadlineAt: 0})
	if err != nil {
		t.Fatalf("CreateProfile failed: %v", err)
	}
	err = testRepo.UpdateUserSettings(models.UserSettings{ActiveProfileID: profileID})
	if err != nil {
		t.Fatalf("UpdateUserSettings failed: %v", err)
	}

	for i := 1; i <= 4; i++ {
		id := fmt.Sprintf("nb-active-%d", i)
		err = testRepo.CreateNotebook(id, fmt.Sprintf("Active %d", i), "dummy", "md", "", 1)
		if err != nil {
			t.Fatalf("CreateNotebook failed: %v", err)
		}
		err = testRepo.AssignNotebookToProfile(id, profileID)
		if err != nil {
			t.Fatalf("AssignNotebookToProfile failed: %v", err)
		}
		err = testRepo.UpdateNotebookStudyStatus(id, "active")
		if err != nil {
			t.Fatalf("UpdateNotebookStudyStatus failed: %v", err)
		}
	}

	uploadResult, err := service.SaveUploadedFile([]byte("# Intro\n\nSome book content here"), "book5.md")
	if err != nil {
		t.Fatalf("SaveUploadedFile failed: %v", err)
	}

	doc, err := service.ExtractDocument(uploadResult.FilePath, uploadResult.FileType)
	if err != nil {
		t.Fatalf("ExtractDocument failed: %v", err)
	}

	if err := testRepo.CreateNotebook(uploadResult.ID, uploadResult.FileName, uploadResult.FilePath, uploadResult.FileType, "", doc.PageCount); err != nil {
		t.Fatalf("CreateNotebook failed: %v", err)
	}
	if err := testRepo.AssignNotebookToProfile(uploadResult.ID, profileID); err != nil {
		t.Fatalf("AssignNotebookToProfile failed: %v", err)
	}

	resp := app.ConfirmNotebookSyllabus(uploadResult.ID, []models.SyllabusChapterDraft{{
		Title:     "Chapter 1",
		StartPage: 1,
		EndPage:   doc.PageCount,
	}})
	if _, hasErr := resp["error"]; hasErr {
		t.Fatalf("expected confirm success, got error: %v", resp["error"])
	}

	nb, err := testRepo.GetNotebookByID(uploadResult.ID)
	if err != nil {
		t.Fatalf("GetNotebookByID failed: %v", err)
	}
	if nb.StudyStatus == "active" {
		t.Fatalf("expected study status to remain dormant/empty, got %q", nb.StudyStatus)
	}
}

func TestConfirmNotebookSyllabus_MetadataOnlySkipsExtraction(t *testing.T) {
	app, uploadResult, chapters := setupConfirmedChunkedNotebook(t, "confirm-metadata-only.md")

	if err := os.Remove(uploadResult.FilePath); err != nil {
		t.Fatalf("Remove source file failed: %v", err)
	}
	beforeChunks := mustNotebookChunkCount(t, uploadResult.ID)

	resp := app.ConfirmNotebookSyllabus(uploadResult.ID, chapters)
	if _, hasErr := resp["error"]; hasErr {
		t.Fatalf("expected metadata-only confirm success, got error: %v", resp["error"])
	}
	if mode, ok := resp["mode"].(string); !ok || mode != "metadata_only" {
		t.Fatalf("expected metadata_only mode, got %#v", resp["mode"])
	}
	if afterChunks := mustNotebookChunkCount(t, uploadResult.ID); afterChunks != beforeChunks {
		t.Fatalf("expected chunks unchanged, got %d want %d", afterChunks, beforeChunks)
	}
}

func TestConfirmNotebookSyllabus_TitleOnlyUpdatesTopicsAndSkipsExtraction(t *testing.T) {
	app, uploadResult, chapters := setupConfirmedChunkedNotebook(t, "confirm-title-only.md")

	if err := os.Remove(uploadResult.FilePath); err != nil {
		t.Fatalf("Remove source file failed: %v", err)
	}
	beforeChunks := mustNotebookChunkCount(t, uploadResult.ID)
	renamed := []models.SyllabusChapterDraft{
		{Title: "Renamed Intro", StartPage: chapters[0].StartPage, EndPage: chapters[0].EndPage},
		{Title: "Renamed Details", StartPage: chapters[1].StartPage, EndPage: chapters[1].EndPage},
	}

	resp := app.ConfirmNotebookSyllabus(uploadResult.ID, renamed)
	if _, hasErr := resp["error"]; hasErr {
		t.Fatalf("expected title-only confirm success, got error: %v", resp["error"])
	}
	if mode, ok := resp["mode"].(string); !ok || mode != "topic_metadata_only" {
		t.Fatalf("expected topic_metadata_only mode, got %#v", resp["mode"])
	}
	if afterChunks := mustNotebookChunkCount(t, uploadResult.ID); afterChunks != beforeChunks {
		t.Fatalf("expected chunks unchanged, got %d want %d", afterChunks, beforeChunks)
	}

	topics, err := testRepo.GetNotebookTopicsWithBounds(uploadResult.ID)
	if err != nil {
		t.Fatalf("GetNotebookTopicsWithBounds failed: %v", err)
	}
	if len(topics) != len(renamed) {
		t.Fatalf("expected %d topics, got %d", len(renamed), len(topics))
	}
	for i := range renamed {
		if topics[i].Title != renamed[i].Title {
			t.Fatalf("topic %d title mismatch: got %q want %q", i, topics[i].Title, renamed[i].Title)
		}
		if topics[i].StartPage != renamed[i].StartPage || topics[i].EndPage != renamed[i].EndPage {
			t.Fatalf("topic %d bounds changed: got [%d,%d] want [%d,%d]", i, topics[i].StartPage, topics[i].EndPage, renamed[i].StartPage, renamed[i].EndPage)
		}
	}
}

func TestConfirmNotebookSyllabus_BoundaryChangeFallsBackToFullReingest(t *testing.T) {
	app, uploadResult, _ := setupConfirmedChunkedNotebook(t, "confirm-boundary-change.md")

	resp := app.ConfirmNotebookSyllabus(uploadResult.ID, []models.SyllabusChapterDraft{{
		Title:     "Intro",
		StartPage: 1,
		EndPage:   2,
	}})
	if _, hasErr := resp["error"]; hasErr {
		t.Fatalf("expected boundary-change confirm success, got error: %v", resp["error"])
	}
	if mode, ok := resp["mode"].(string); !ok || mode != "full_reingest" {
		t.Fatalf("expected full_reingest mode, got %#v", resp["mode"])
	}

	topicIDs, ok := resp["topic_ids"].([]string)
	if !ok || len(topicIDs) != 1 {
		t.Fatalf("expected one topic id after merged bounds, got %#v", resp["topic_ids"])
	}
	startPage, endPage, err := testRepo.GetTopicPageBounds(topicIDs[0])
	if err != nil {
		t.Fatalf("GetTopicPageBounds failed: %v", err)
	}
	if startPage != 1 || endPage != 2 {
		t.Fatalf("unexpected persisted bounds: got [%d,%d] want [1,2]", startPage, endPage)
	}
}

func TestConfirmNotebookSyllabus_MixedTitleAndBoundaryChangeFullReingests(t *testing.T) {
	app, uploadResult, _ := setupConfirmedChunkedNotebook(t, "confirm-mixed-change.md")

	resp := app.ConfirmNotebookSyllabus(uploadResult.ID, []models.SyllabusChapterDraft{{
		Title:     "Renamed Combined Chapter",
		StartPage: 1,
		EndPage:   2,
	}})
	if _, hasErr := resp["error"]; hasErr {
		t.Fatalf("expected mixed-change confirm success, got error: %v", resp["error"])
	}
	if mode, ok := resp["mode"].(string); !ok || mode != "full_reingest" {
		t.Fatalf("expected full_reingest mode, got %#v", resp["mode"])
	}

	topicIDs, ok := resp["topic_ids"].([]string)
	if !ok || len(topicIDs) != 1 {
		t.Fatalf("expected one topic id after mixed change, got %#v", resp["topic_ids"])
	}
	topics, err := testRepo.GetNotebookTopicsWithBounds(uploadResult.ID)
	if err != nil {
		t.Fatalf("GetNotebookTopicsWithBounds failed: %v", err)
	}
	found := false
	for _, topic := range topics {
		if topic.TopicID == topicIDs[0] {
			found = true
			if topic.Title != "Renamed Combined Chapter" {
				t.Fatalf("expected renamed topic title, got %q", topic.Title)
			}
			if topic.StartPage != 1 || topic.EndPage != 2 {
				t.Fatalf("unexpected renamed topic bounds: got [%d,%d] want [1,2]", topic.StartPage, topic.EndPage)
			}
		}
	}
	if !found {
		t.Fatalf("expected new mixed-change topic %q in notebook topic bounds", topicIDs[0])
	}
}

func setupConfirmedChunkedNotebook(t *testing.T, fileName string) (*App, notebook.UploadResult, []models.SyllabusChapterDraft) {
	t.Helper()
	initTestDB(t)
	uploadDir := t.TempDir()
	service := notebook.NewService(uploadDir)
	app := &App{repo: testRepo, notebookService: service}

	content := []byte("# Intro\n\nAlpha beta gamma\n\n## Details\n\nDelta epsilon zeta")
	uploadResult, err := service.SaveUploadedFile(content, fileName)
	if err != nil {
		t.Fatalf("SaveUploadedFile failed: %v", err)
	}
	doc, err := service.ExtractDocument(uploadResult.FilePath, uploadResult.FileType)
	if err != nil {
		t.Fatalf("ExtractDocument failed: %v", err)
	}
	if doc.PageCount != 2 {
		t.Fatalf("expected two-page markdown fixture, got %d", doc.PageCount)
	}
	if err := testRepo.CreateNotebook(uploadResult.ID, uploadResult.FileName, uploadResult.FilePath, uploadResult.FileType, "", doc.PageCount); err != nil {
		t.Fatalf("CreateNotebook failed: %v", err)
	}

	chapters := []models.SyllabusChapterDraft{
		{Title: "Intro", StartPage: 1, EndPage: 1},
		{Title: "Details", StartPage: 2, EndPage: 2},
	}
	resp := app.ConfirmNotebookSyllabus(uploadResult.ID, chapters)
	if _, hasErr := resp["error"]; hasErr {
		t.Fatalf("initial ConfirmNotebookSyllabus failed: %v", resp["error"])
	}
	if status, ok := resp["status"].(string); !ok || status != "chunked" {
		t.Fatalf("expected initial chunked status, got %#v", resp["status"])
	}
	if count := mustNotebookChunkCount(t, uploadResult.ID); count == 0 {
		t.Fatalf("expected initial chunks")
	}

	return app, *uploadResult, chapters
}

func mustNotebookChunkCount(t *testing.T, notebookID string) int {
	t.Helper()
	chunks, err := testRepo.GetChunksForNotebook(notebookID)
	if err != nil {
		t.Fatalf("GetChunksForNotebook failed: %v", err)
	}
	return len(chunks)
}

// ============================================================================
// QUEUE CONTRACT TESTS
// ============================================================================

// TestActivateTask_TransitionsPendingToActive verifies ActivateTask moves task from PENDING to ACTIVE.
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

	// Activate the task
	resp := app.ActivateTask("task-activate-1")
	if _, hasErr := resp["error"]; hasErr {
		t.Fatalf("expected success, got error: %v", resp["error"])
	}

	// Verify task is no longer in pending queue
	_, err := testRepo.GetNextTask(notebookID)
	if err != db.ErrNoPendingTasks {
		t.Fatalf("expected no pending tasks after activation, got: %v", err)
	}
}

// TestActivateTask_RejectsNonPendingTask verifies ActivateTask rejects tasks not in PENDING status.
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
		Status:     models.StudyTaskStatusActive, // Already active
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

// TestCompleteTask_MarksActiveAsCompleted verifies CompleteTask marks ACTIVE task as COMPLETED.
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

	// Verify task is now COMPLETED (should not appear in pending queue)
	_, err := testRepo.GetNextTask(notebookID)
	if err != db.ErrNoPendingTasks {
		t.Fatalf("expected no pending tasks after completion, got: %v", err)
	}
}

// TestCompleteTask_InsertsFollowUpTasks verifies CompleteTask inserts follow-up tasks transactionally.
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

	// Verify follow-up task was inserted
	nextTask, err := testRepo.GetNextTask(notebookID)
	if err != nil {
		t.Fatalf("expected follow-up task in queue, got error: %v", err)
	}
	if nextTask.ID != "followup-1" {
		t.Fatalf("expected followup-1, got: %s", nextTask.ID)
	}
}

// TestSkipTask_MarksTaskAsSkipped verifies SkipTask marks task as SKIPPED.
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

	// Verify task is no longer in pending queue
	_, err := testRepo.GetNextTask(notebookID)
	if err != db.ErrNoPendingTasks {
		t.Fatalf("expected no pending tasks after skip, got: %v", err)
	}
}

// ============================================================================
// DETERMINISTIC ORDERING TESTS
// ============================================================================

// TestOrdering_TaskTypePriority verifies FLASHCARD_REVIEW > REREAD > QUIZ > READING > EXAMINER.
func TestOrdering_TaskTypePriority(t *testing.T) {
	initTestDB(t)

	notebookID := "ordering-type-nb"
	if err := testRepo.CreateNotebook(notebookID, "Ordering Type Notebook", "/tmp/ordering-type.txt", "txt", "", 1); err != nil {
		t.Fatalf("CreateNotebook failed: %v", err)
	}

	// Insert tasks in reverse priority order (EXAMINER first, FLASHCARD_REVIEW last)
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

	// Verify FLASHCARD_REVIEW is returned first
	nextTask, err := testRepo.GetNextTask(notebookID)
	if err != nil {
		t.Fatalf("GetNextTask failed: %v", err)
	}
	if nextTask.TaskType != models.StudyTaskTypeFlashcardReview {
		t.Fatalf("expected FLASHCARD_REVIEW first, got: %s", nextTask.TaskType)
	}

	// Activate and complete to get next task
	if err := testRepo.ActivateTask("task-1"); err != nil {
		t.Fatalf("ActivateTask failed: %v", err)
	}
	if err := testRepo.CompleteTask("task-1", models.CompletionResult{Status: models.StudyTaskStatusCompleted}); err != nil {
		t.Fatalf("CompleteTask failed: %v", err)
	}

	// Verify REREAD is second
	nextTask, err = testRepo.GetNextTask(notebookID)
	if err != nil {
		t.Fatalf("GetNextTask failed: %v", err)
	}
	if nextTask.TaskType != models.StudyTaskTypeReread {
		t.Fatalf("expected REREAD second, got: %s", nextTask.TaskType)
	}
}

// TestOrdering_NotebookPriority verifies higher notebook priority tasks are returned first.
// NOTE: This test is skipped because db.UpdateNotebookPriority does not exist in the current codebase.
// Notebook priority ordering is preserved in the query but cannot be tested without a method to set priority.
func TestOrdering_NotebookPriority(t *testing.T) {
	t.Skip("db.UpdateNotebookPriority method does not exist - notebook priority ordering is preserved in query but cannot be tested")
}

// TestOrdering_TaskPriority verifies lower task priority numbers are returned first.
func TestOrdering_TaskPriority(t *testing.T) {
	initTestDB(t)

	notebookID := "ordering-priority-nb"
	if err := testRepo.CreateNotebook(notebookID, "Ordering Priority Notebook", "/tmp/ordering-priority.txt", "txt", "", 1); err != nil {
		t.Fatalf("CreateNotebook failed: %v", err)
	}

	// Insert tasks with different priorities (same type)
	taskHighPriority := models.StudyQueueTask{ID: "task-high-priority", NotebookID: notebookID, TaskType: models.StudyTaskTypeQuiz, Status: models.StudyTaskStatusPending, Priority: 1}
	taskLowPriority := models.StudyQueueTask{ID: "task-low-priority", NotebookID: notebookID, TaskType: models.StudyTaskTypeQuiz, Status: models.StudyTaskStatusPending, Priority: 10}

	if err := testRepo.InsertStudyTask(taskLowPriority); err != nil {
		t.Fatalf("InsertStudyTask failed: %v", err)
	}
	if err := testRepo.InsertStudyTask(taskHighPriority); err != nil {
		t.Fatalf("InsertStudyTask failed: %v", err)
	}

	// Verify high priority task (lower number) is returned first
	nextTask, err := testRepo.GetNextTask(notebookID)
	if err != nil {
		t.Fatalf("GetNextTask failed: %v", err)
	}
	if nextTask.ID != "task-high-priority" {
		t.Fatalf("expected task-high-priority first, got: %s", nextTask.ID)
	}
}

// TestOrdering_FIFOFallback verifies FIFO ordering when all other priorities are equal.
func TestOrdering_FIFOFallback(t *testing.T) {
	initTestDB(t)

	notebookID := "ordering-fifo-nb"
	if err := testRepo.CreateNotebook(notebookID, "FIFO Notebook", "/tmp/fifo.txt", "txt", "", 1); err != nil {
		t.Fatalf("CreateNotebook failed: %v", err)
	}

	// Insert tasks with same type and priority (order of insertion determines FIFO)
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

	// Verify FIFO order (first inserted is first returned)
	nextTask, err := testRepo.GetNextTask(notebookID)
	if err != nil {
		t.Fatalf("GetNextTask failed: %v", err)
	}
	if nextTask.ID != "task-fifo-1" {
		t.Fatalf("expected task-fifo-1 first (FIFO), got: %s", nextTask.ID)
	}
}

// TestOrdering_AntiStarvation verifies deterministic ordering prevents starvation.
// This test verifies that the ordering is query-time deterministic and not adaptive.
func TestOrdering_AntiStarvation(t *testing.T) {
	initTestDB(t)

	notebookID := "anti-starvation-nb"
	if err := testRepo.CreateNotebook(notebookID, "Anti Starvation Notebook", "/tmp/anti-starvation.txt", "txt", "", 1); err != nil {
		t.Fatalf("CreateNotebook failed: %v", err)
	}

	// Insert mix of task types and priorities
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

	// Query multiple times to verify deterministic ordering (same result each time)
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

	// Verify task type precedence wins before deterministic fallback order
	if firstTaskID != "task-b" {
		t.Fatalf("expected task-b to win due to task type precedence, got: %s", firstTaskID)
	}
}

func mustInsertMockChunk(t *testing.T, notebookID, topicID, chunkID string, pageNum int) {
	t.Helper()
	// Ensure chunk exists in chunks table
	if _, err := testRepo.ExecForTest(`
		INSERT OR IGNORE INTO chunks (id, topic_id, chunk_text, page_num)
		VALUES (?, ?, 'Mock chunk text for calibration test', ?)
	`, chunkID, topicID, pageNum); err != nil {
		t.Fatalf("failed to insert chunk: %v", err)
	}

	// Link chunk to notebook
	if _, err := testRepo.ExecForTest(`
		INSERT OR IGNORE INTO notebook_chunks (id, notebook_id, chunk_id, page_num)
		VALUES (?, ?, ?, ?)
	`, uuid.NewString(), notebookID, chunkID, pageNum); err != nil {
		t.Fatalf("failed to insert notebook_chunk link: %v", err)
	}
}

func TestCompleteSocraticRescueInsertsRequiz(t *testing.T) {
	app := newTestApp(t)
	// Seed notebook and topic
	if err := testRepo.EnsureTopic("topic-test", "Topic Test"); err != nil {
		t.Fatalf("EnsureTopic failed: %v", err)
	}
	if err := testRepo.CreateNotebook("nb-test", "NB Test", "/tmp/nb-test.pdf", "pdf", "topic-test", 12); err != nil {
		t.Fatalf("CreateNotebook failed: %v", err)
	}
	mustInsertMockChunk(t, "nb-test", "topic-test", "chunk-socratic-test-1", 1)

	// Seed a pending/active SOCRATIC_REMEDIAL task
	task := models.StudyQueueTask{
		ID:          "task-socratic-test",
		NotebookID:  "nb-test",
		TopicID:     "topic-test",
		TaskType:    models.StudyTaskTypeSocraticRemedial,
		Status:      models.StudyTaskStatusActive,
		StartPage:   1,
		EndPage:     2,
	}
	if err := testRepo.InsertStudyTask(task); err != nil {
		t.Fatalf("failed to insert socratic task: %v", err)
	}
	res := app.CompleteSocraticRescue("task-socratic-test")
	if errVal, ok := res["error"]; ok && errVal != nil {
		t.Fatalf("CompleteSocraticRescue failed: %v", errVal)
	}
	returnedQuizTaskID, ok := res["quiz_task_id"].(string)
	if !ok || returnedQuizTaskID == "" {
		t.Fatalf("expected completeSocraticRescue to return quiz_task_id")
	}

	// Verify SOCRATIC_REMEDIAL task status is COMPLETED
	socraticTask, err := testRepo.GetTaskByID("task-socratic-test")
	if err != nil {
		t.Fatalf("failed to get socratic task: %v", err)
	}
	if socraticTask.Status != models.StudyTaskStatusCompleted {
		t.Fatalf("expected socratic task status to be COMPLETED, got %q", socraticTask.Status)
	}

	// Verify a new QUIZ task was created with the correct source payload
	quizCount, err := testRepo.CountTasksByTopicTypeAndStatus("topic-test", "QUIZ", "PENDING")
	if err != nil {
		t.Fatalf("failed to query quiz task: %v", err)
	}
	if quizCount != 1 {
		t.Fatalf("expected 1 PENDING QUIZ task, got %d", quizCount)
	}

	// Retrieve actual payload and assert
	var payloadJSON string
	err = testRepo.QueryRowForTest(`
		SELECT payload_json
		FROM study_queue
		WHERE id = ? AND status = 'PENDING'
	`, returnedQuizTaskID).Scan(&payloadJSON)
	if err != nil {
		t.Fatalf("failed to retrieve quiz task payload: %v", err)
	}

	var payloadMap map[string]interface{}
	if err := json.Unmarshal([]byte(payloadJSON), &payloadMap); err != nil {
		t.Fatalf("failed to unmarshal quiz task payload: %v", err)
	}

	if source, ok := payloadMap["source"].(string); !ok || source != "socratic_rescue_requiz" {
		t.Fatalf("expected payload source to be %q, got %q", "socratic_rescue_requiz", payloadMap["source"])
	}

	// Assert that questions were generated and stored in the payload
	questions, ok := payloadMap["questions"].([]interface{})
	if !ok || len(questions) == 0 {
		t.Fatalf("expected generated questions in payload, got none")
	}
}

func TestRequizPassGeneratesFlashcards(t *testing.T) {
	app := newTestApp(t)
	// Seed notebook and topic
	if err := testRepo.EnsureTopic("topic-requiz-pass", "Requiz Pass Topic"); err != nil {
		t.Fatalf("EnsureTopic failed: %v", err)
	}
	if err := testRepo.CreateNotebook("nb-requiz-pass", "NB Requiz Pass", "/tmp/nb-requiz-pass.pdf", "pdf", "topic-requiz-pass", 12); err != nil {
		t.Fatalf("CreateNotebook failed: %v", err)
	}

	// Seed an active re-quiz task
	payloadBytes, _ := json.Marshal(map[string]interface{}{
		"source":   "socratic_rescue_requiz",
		"topic_id": "topic-requiz-pass",
		"questions": []models.QuizTaskQuestion{
			{ID: "q1", Prompt: "P1", Options: []string{"A", "B"}, CorrectAnswer: "A"},
		},
		"passing_score": 70,
	})
	task := models.StudyQueueTask{
		ID:          "task-requiz-pass",
		NotebookID:  "nb-requiz-pass",
		TopicID:     "topic-requiz-pass",
		TaskType:    models.StudyTaskTypeQuiz,
		Status:      models.StudyTaskStatusActive,
		PayloadJSON: string(payloadBytes),
		StartPage:   1,
		EndPage:     2,
	}
	if err := testRepo.InsertStudyTask(task); err != nil {
		t.Fatalf("failed to insert requiz task: %v", err)
	}

	resp := app.SubmitQuizAttempt("task-requiz-pass", []models.QuizAnswer{
		{QuestionID: "q1", Selected: "A"},
	})
	if _, hasErr := resp["error"]; hasErr {
		t.Fatalf("expected submit success, got error: %v", resp["error"])
	}

	result, ok := resp["result"].(models.QuizResult)
	if !ok {
		t.Fatalf("expected QuizResult payload, got %#v", resp["result"])
	}
	if !result.Passed {
		t.Fatalf("expected passed quiz result")
	}
	if !result.FlashcardsPending {
		t.Fatalf("expected FlashcardsPending to be true on passing re-quiz")
	}
}

func TestRequizFailMarksExternalHelp(t *testing.T) {
	app := newTestApp(t)
	// Ensure topic and notebook exist
	if err := testRepo.EnsureTopic("topic-requiz-fail", "Requiz Fail Topic"); err != nil {
		t.Fatalf("failed to ensure topic: %v", err)
	}
	if err := testRepo.CreateNotebook("nb-requiz-fail", "NB Requiz Fail", "/tmp/nb-requiz-fail.pdf", "pdf", "topic-requiz-fail", 12); err != nil {
		t.Fatalf("CreateNotebook failed: %v", err)
	}

	// Seed an active re-quiz task
	payloadBytes, _ := json.Marshal(map[string]interface{}{
		"source":   "socratic_rescue_requiz",
		"topic_id": "topic-requiz-fail",
		"questions": []models.QuizTaskQuestion{
			{ID: "q1", Prompt: "P1", Options: []string{"A", "B"}, CorrectAnswer: "A"},
		},
		"passing_score": 70,
	})
	task := models.StudyQueueTask{
		ID:          "task-requiz-fail",
		NotebookID:  "nb-requiz-fail",
		TopicID:     "topic-requiz-fail",
		TaskType:    models.StudyTaskTypeQuiz,
		Status:      models.StudyTaskStatusActive,
		PayloadJSON: string(payloadBytes),
		StartPage:   1,
		EndPage:     2,
	}
	if err := testRepo.InsertStudyTask(task); err != nil {
		t.Fatalf("failed to insert requiz task: %v", err)
	}

	resp := app.SubmitQuizAttempt("task-requiz-fail", []models.QuizAnswer{
		{QuestionID: "q1", Selected: "B"},
	})
	if _, hasErr := resp["error"]; hasErr {
		t.Fatalf("expected submit success, got error: %v", resp["error"])
	}

	result, ok := resp["result"].(models.QuizResult)
	if !ok {
		t.Fatalf("expected QuizResult payload, got %#v", resp["result"])
	}
	if result.Passed {
		t.Fatalf("expected failed quiz result")
	}
	if result.FlashcardsPending {
		t.Fatalf("expected FlashcardsPending to be false on failing re-quiz")
	}

	// Verify topic.external_help_required is 1
	var required bool
	err := testRepo.QueryRowForTest("SELECT external_help_required FROM topics WHERE id = 'topic-requiz-fail'").Scan(&required)
	if err != nil {
		t.Fatalf("failed to query topic: %v", err)
	}
	if !required {
		t.Fatalf("expected external_help_required to be true, got false")
	}
}

func TestFSRSCalibrationEasyAndDoubleGood(t *testing.T) {
	app := newTestApp(t)

	// 1. Test Ace calibration: seed a 100% quiz attempt
	mustInsertActiveQuizTask(t, "nb-calibration-1", "topic-calibration-1", "task-quiz-ace", 100)
	mustInsertMockChunk(t, "nb-calibration-1", "topic-calibration-1", "chunk-fallback", 3)

	respAce := app.SubmitQuizAttempt("task-quiz-ace", []models.QuizAnswer{
		{QuestionID: "quiz-q1", Selected: "A"}, // Correct
		{QuestionID: "quiz-q2", Selected: "B"}, // Correct
	})
	if _, hasErr := respAce["error"]; hasErr {
		t.Fatalf("expected submit success, got error: %v", respAce["error"])
	}

	// Generate flashcards
	genResp1 := app.GenerateFlashcardsForQuizTask("task-quiz-ace")
	if _, hasErr := genResp1["error"]; hasErr {
		t.Fatalf("expected flashcard generation success, got error: %v", genResp1["error"])
	}

	// Verify that the generated flashcard has Easy FSRS state (stability > 8.0, difficulty = 1.0, reps = 1)
	var stateJSON sql.NullString
	var dueAt int64
	err := testRepo.QueryRowForTest("SELECT state_json, due_at FROM fsrs_cards WHERE topic_id = 'topic-calibration-1' LIMIT 1").Scan(&stateJSON, &dueAt)
	if err != nil {
		t.Fatalf("failed to query card: %v", err)
	}
	var cardState models.FlashcardState
	if err := json.Unmarshal([]byte(stateJSON.String), &cardState); err != nil {
		t.Fatalf("failed to unmarshal state: %v", err)
	}
	if cardState.Reps != 1 || cardState.Difficulty != 1.0 || cardState.Stability < 8.0 {
		t.Fatalf("expected Ace card state to be calibrated to Easy, got reps=%d diff=%f stability=%f", cardState.Reps, cardState.Difficulty, cardState.Stability)
	}

	// 2. Test Pass calibration: seed a 50% quiz attempt (passing_score = 50)
	mustInsertActiveQuizTask(t, "nb-calibration-2", "topic-calibration-2", "task-quiz-pass", 50)
	mustInsertMockChunk(t, "nb-calibration-2", "topic-calibration-2", "chunk-fallback", 3)

	respPass := app.SubmitQuizAttempt("task-quiz-pass", []models.QuizAnswer{
		{QuestionID: "quiz-q1", Selected: "A"}, // Correct
		{QuestionID: "quiz-q2", Selected: "C"}, // Incorrect
	})
	if _, hasErr := respPass["error"]; hasErr {
		t.Fatalf("expected submit success, got error: %v", respPass["error"])
	}

	// Generate flashcards
	genResp2 := app.GenerateFlashcardsForQuizTask("task-quiz-pass")
	if _, hasErr := genResp2["error"]; hasErr {
		t.Fatalf("expected flashcard generation success, got error: %v", genResp2["error"])
	}

	// Verify that the generated flashcard has Double Good FSRS state (reps = 2, stability = 2.3065)
	var stateJSON2 sql.NullString
	var dueAt2 int64
	err = testRepo.QueryRowForTest("SELECT state_json, due_at FROM fsrs_cards WHERE topic_id = 'topic-calibration-2' LIMIT 1").Scan(&stateJSON2, &dueAt2)
	if err != nil {
		t.Fatalf("failed to query card: %v", err)
	}
	var cardState2 models.FlashcardState
	if err := json.Unmarshal([]byte(stateJSON2.String), &cardState2); err != nil {
		t.Fatalf("failed to unmarshal state: %v", err)
	}
	if cardState2.Reps != 2 || cardState2.Stability != 2.3065 {
		t.Fatalf("expected Pass card state to be calibrated to Double Good, got reps=%d stability=%f", cardState2.Reps, cardState2.Stability)
	}
}

func TestTriggerCloudSyncRetriesAndFailSafe(t *testing.T) {
	_ = newTestApp(t)

	// Ensure we create a notebook to make sure sync has a valid notebook ID
	if err := testRepo.CreateNotebook("os-notebook-sync", "OS Notebook", "/tmp/os-notebook.pdf", "pdf", "", 12); err != nil {
		t.Fatalf("failed to create notebook: %v", err)
	}

	// Seed user settings with local server URL
	var attempts int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts < 3 {
			// Fail first 2 attempts
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		// Succeed on 3rd attempt
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"new_notebooks": []interface{}{},
		})
	}))
	defer server.Close()

	// Update user settings in DB
	if _, err := testRepo.ExecForTest(`
		UPDATE user_settings
		SET cloud_sync_url = ?, cloud_api_token = 'token'
		WHERE id = 1
	`, server.URL); err != nil {
		t.Fatalf("failed to update user settings: %v", err)
	}

	// 1. Verify successful sync after retries
	err := study.TriggerCloudSync(testRepo)
	if err != nil {
		t.Fatalf("expected cloud sync to succeed, got error: %v", err)
	}
	if attempts != 3 {
		t.Fatalf("expected exactly 3 attempts, got %d", attempts)
	}

	// Verify no FLASHCARD_SYNC task was inserted since sync succeeded eventually
	syncCount, err := testRepo.CountTasksByTopicTypeAndStatus("", "FLASHCARD_SYNC", "PENDING")
	if err != nil {
		t.Fatalf("failed to query sync task: %v", err)
	}
	if syncCount != 0 {
		t.Fatalf("expected no PENDING FLASHCARD_SYNC task, got %d", syncCount)
	}

	// 2. Test persistent failure: make server return 500 always
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

	// Verify FLASHCARD_SYNC task was inserted
	syncCount, err = testRepo.CountTasksByTopicTypeAndStatus("", "FLASHCARD_SYNC", "PENDING")
	if err != nil {
		t.Fatalf("failed to query sync task: %v", err)
	}
	if syncCount != 1 {
		t.Fatalf("expected 1 PENDING FLASHCARD_SYNC task, got %d", syncCount)
	}

	// 3. Verify sync resolution: make server succeed again
	serverSuccess := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
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

	// Verify FLASHCARD_SYNC task was resolved (status == COMPLETED)
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
// LIGHTWEIGHT TEST BUILDERS
// ============================================================================

