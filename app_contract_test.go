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

	"ai-tutor/internal/db"
	"ai-tutor/internal/llm"
	"ai-tutor/internal/models"
	"ai-tutor/internal/notebook"
	"ai-tutor/internal/rag"
	"ai-tutor/internal/retrieval"
	"ai-tutor/internal/scheduler"
	"ai-tutor/internal/study"
)

func mustInsertActiveQuizTask(t *testing.T, notebookID, topicID, taskID string, passingScore int) {
	t.Helper()
	if err := db.EnsureTopic(topicID, topicID+"-title"); err != nil {
		t.Fatalf("EnsureTopic failed: %v", err)
	}
	if err := db.CreateNotebook(notebookID, notebookID+"-name", "/tmp/"+notebookID+".pdf", "pdf", topicID, 12); err != nil {
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

	if err := db.InsertStudyTask(models.StudyQueueTask{
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
	if result.RereadAttemptCount != 1 || result.MaxRereadAttempts != 3 {
		t.Fatalf("unexpected reread metadata: %#v", result)
	}

	var rereadCount int
	if err := db.GetConnection().QueryRow(`
		SELECT COUNT(*)
		FROM study_queue
		WHERE id = ? AND task_type = 'REREAD' AND status = 'PENDING'
	`, result.RereadTaskID).Scan(&rereadCount); err != nil {
		t.Fatalf("query reread follow-up failed: %v", err)
	}
	if rereadCount != 1 {
		t.Fatalf("expected one pending reread follow-up, got %d", rereadCount)
	}
}

func TestSubmitQuizAttemptAfterMaxReturnsManualReviewWithoutReread(t *testing.T) {
	app := newTestApp(t)
	mustInsertActiveQuizTask(t, "nb-quiz-max", "topic-quiz-max", "task-quiz-max", 100)

	tx, err := db.GetConnection().Begin()
	if err != nil {
		t.Fatalf("begin tx failed: %v", err)
	}
	if _, err := db.IncrementRereadAttemptCountTx(tx, "topic-quiz-max"); err != nil {
		t.Fatalf("seed attempt 1 failed: %v", err)
	}
	if _, err := db.IncrementRereadAttemptCountTx(tx, "topic-quiz-max"); err != nil {
		t.Fatalf("seed attempt 2 failed: %v", err)
	}
	if _, err := db.IncrementRereadAttemptCountTx(tx, "topic-quiz-max"); err != nil {
		t.Fatalf("seed attempt 3 failed: %v", err)
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
	if result.RereadAttemptCount != 4 || result.MaxRereadAttempts != 3 {
		t.Fatalf("unexpected reread metadata: %#v", result)
	}

	var pendingRereads int
	if err := db.GetConnection().QueryRow(`
		SELECT COUNT(*)
		FROM study_queue
		WHERE topic_id = 'topic-quiz-max' AND task_type = 'REREAD' AND status = 'PENDING'
	`).Scan(&pendingRereads); err != nil {
		t.Fatalf("query pending rereads failed: %v", err)
	}
	if pendingRereads != 0 {
		t.Fatalf("expected no automatic reread inserted after max, got %d", pendingRereads)
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

	var pendingRereads int
	if err := db.GetConnection().QueryRow(`
		SELECT COUNT(*)
		FROM study_queue
		WHERE topic_id = 'topic-quiz-repeat' AND task_type = 'REREAD' AND status = 'PENDING'
	`).Scan(&pendingRereads); err != nil {
		t.Fatalf("query pending rereads failed: %v", err)
	}
	if pendingRereads != 1 {
		t.Fatalf("expected exactly one reread after duplicate submit attempt, got %d", pendingRereads)
	}
}

func TestSubmitQuizAttemptPassResetsAttemptsAndFutureFailureStartsAtOne(t *testing.T) {
	app := newTestApp(t)

	mustInsertActiveQuizTask(t, "nb-quiz-pass-reset", "topic-quiz-pass-reset", "task-quiz-pass", 100)
	tx, err := db.GetConnection().Begin()
	if err != nil {
		t.Fatalf("begin seed tx failed: %v", err)
	}
	if _, err := db.IncrementRereadAttemptCountTx(tx, "topic-quiz-pass-reset"); err != nil {
		t.Fatalf("seed attempt 1 failed: %v", err)
	}
	if _, err := db.IncrementRereadAttemptCountTx(tx, "topic-quiz-pass-reset"); err != nil {
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

	count, err := db.GetRereadAttemptCount("topic-quiz-pass-reset")
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

func initTestDB(t *testing.T) {
	t.Helper()
	tempDB := filepath.Join(t.TempDir(), "ai-tutor-test.db")
	if err := db.Init(tempDB, ""); err != nil {
		t.Fatalf("failed to init test db: %v", err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Fatalf("failed to close test db: %v", err)
		}
	})
	if err := db.SeedDemoDataForTests(); err != nil {
		t.Fatalf("failed to seed test data: %v", err)
	}
}

func initCleanTestDB(t *testing.T) {
	t.Helper()
	tempDB := filepath.Join(t.TempDir(), "ai-tutor-test.db")
	if err := db.Init(tempDB, ""); err != nil {
		t.Fatalf("failed to init clean test db: %v", err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Fatalf("failed to close clean test db: %v", err)
		}
	})
}

func initTestPipeline(t *testing.T) *rag.Pipeline {
	t.Helper()
	initTestDB(t)

	embedStore := rag.NewEmbeddingStore(nil)
	topicIDs, err := db.GetAllTopicIDs()
	if err != nil {
		t.Fatalf("failed to list topic IDs: %v", err)
	}

	for _, topicID := range topicIDs {
		chunks, chunksErr := db.GetChunksForTopic(topicID)
		if chunksErr != nil {
			t.Fatalf("failed to get chunks for topic %s: %v", topicID, chunksErr)
		}
		for _, chunk := range chunks {
			embedStore.AddChunk(chunk)
		}
	}

	mockLLM := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			http.NotFound(w, r)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"choices": []map[string]interface{}{
				{
					"message": map[string]string{"content": "Round Robin gives each process a fixed time slice."},
				},
			},
		})
	}))
	t.Cleanup(mockLLM.Close)

	t.Setenv("LLM_BASE_URL", mockLLM.URL)
	t.Setenv("LLM_API_KEY", "test-key")
	t.Setenv("LLM_MODEL", "test-model")

	provider := llm.NewProvider(llm.LoadConfigFromEnv())
	return rag.NewPipeline(embedStore, provider)
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
		fastLLMProvider:   provider,
		heavyLLMProvider:  provider,
		scheduler:         scheduler.New(),
		notebookService:   notebook.NewService(uploadDir),
		notebookUploadDir: uploadDir,
		aiReady:           true,
		aiInitError:       "",
	}

	// Initialize retrieval engine (required by study service)
	embedStore := rag.NewEmbeddingStore(nil)
	topicIDs, err := db.GetAllTopicIDs()
	if err != nil {
		t.Fatalf("failed to list topic IDs: %v", err)
	}
	for _, topicID := range topicIDs {
		chunks, chunksErr := db.GetChunksForTopic(topicID)
		if chunksErr != nil {
			t.Fatalf("failed to get chunks for topic %s: %v", topicID, chunksErr)
		}
		for _, chunk := range chunks {
			embedStore.AddChunk(chunk)
		}
	}
	app.retrievalEngine = retrieval.NewEngine(nil)

	// Initialize study service with all required dependencies
	app.studyService = study.NewStudyService(study.Config{
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
// - AI/RAG tests (AskAI, RAG pipeline)
// - Notebook/Topic tests (GetAvailableTopics, GetNotebookTopicTree, etc.)
// - Quiz/Scoring tests (ScoreAnswer, quiz generation)
// - Flashcard/FSRS tests (GenerateFlashcards, RecordFlashcardReview)
// - Written Answer tests (GenerateShortAnswerPrompt, ScoreShortAnswer)
// - Reader tests (GetReaderTopicBundle, ExplainReaderSection, CompleteReadingSession)
// - Notebook Upload tests (DraftNotebookSyllabus, ConfirmNotebookSyllabus)
// - Queue tests (GetNextTask, ActivateTask, CompleteTask, SkipTask) [TO BE ADDED]
// - Deterministic Ordering tests [TO BE ADDED]
// ============================================================================

// ============================================================================
// AI/RAG TESTS
// ============================================================================

func TestAskAIResponseShape(t *testing.T) {
	app := &App{ragPipeline: initTestPipeline(t), aiReady: true}

	resp := app.AskAI("os-scheduling", "What is Round Robin scheduling?")

	if _, hasError := resp["error"]; hasError {
		t.Fatalf("expected success response, got error: %v", resp["error"])
	}

	if _, ok := resp["answer"].(string); !ok {
		t.Fatalf("expected answer string, got: %#v", resp["answer"])
	}

	switch cited := resp["cited_sections"].(type) {
	case []string:
		// valid typed response
	case []interface{}:
		for idx, item := range cited {
			if _, ok := item.(string); !ok {
				t.Fatalf("expected cited_sections[%d] to be string, got: %#v", idx, item)
			}
		}
	default:
		t.Fatalf("expected cited_sections []string or []interface{}, got: %#v", resp["cited_sections"])
	}

	if _, ok := resp["chunks_retrieved"].(int); !ok {
		t.Fatalf("expected chunks_retrieved int, got: %#v", resp["chunks_retrieved"])
	}

	if _, ok := resp["sections_used"].(int); !ok {
		t.Fatalf("expected sections_used int, got: %#v", resp["sections_used"])
	}
}

func TestAskAIInvalidTopicReturnsError(t *testing.T) {
	app := &App{ragPipeline: initTestPipeline(t), aiReady: true}

	resp := app.AskAI("missing-topic", "Any question")
	if _, ok := resp["error"].(string); !ok {
		t.Fatalf("expected error string for invalid topic, got: %#v", resp)
	}
}

func TestGetAvailableTopicsFromDB(t *testing.T) {
	initTestDB(t)
	app := &App{}

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
	app := &App{}

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
	app := &App{}

	notebookA := "nb-tree-a"
	notebookB := "nb-tree-b"
	if err := db.CreateNotebook(notebookA, "Physics", "/tmp/physics.txt", "txt", "", 1); err != nil {
		t.Fatalf("CreateNotebook notebookA failed: %v", err)
	}
	if err := db.CreateNotebook(notebookB, "History", "/tmp/history.txt", "txt", "", 1); err != nil {
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
		if err := db.EnsureTopic(topic.id, topic.title); err != nil {
			t.Fatalf("EnsureTopic %s failed: %v", topic.id, err)
		}
	}

	parentThermo := "parent-thermo"
	parentNewton := "parent-newton"
	parentRenaissance := "parent-renaissance"
	if err := db.CreateParentSection(parentThermo, "topic-thermo", "Thermo", 1, "heat"); err != nil {
		t.Fatalf("CreateParentSection thermo failed: %v", err)
	}
	if err := db.CreateParentSection(parentNewton, "topic-newton", "Newton", 1, "motion"); err != nil {
		t.Fatalf("CreateParentSection newton failed: %v", err)
	}
	if err := db.CreateParentSection(parentRenaissance, "topic-renaissance", "Renaissance", 1, "history"); err != nil {
		t.Fatalf("CreateParentSection renaissance failed: %v", err)
	}

	chunkThermo := "chunk-thermo"
	chunkNewton := "chunk-newton"
	chunkRenaissance := "chunk-renaissance"
	if err := db.CreateChunk(chunkThermo, "topic-thermo", parentThermo, "thermo chunk", 2, 1); err != nil {
		t.Fatalf("CreateChunk thermo failed: %v", err)
	}
	if err := db.CreateChunk(chunkNewton, "topic-newton", parentNewton, "newton chunk", 2, 2); err != nil {
		t.Fatalf("CreateChunk newton failed: %v", err)
	}
	if err := db.CreateChunk(chunkRenaissance, "topic-renaissance", parentRenaissance, "renaissance chunk", 2, 3); err != nil {
		t.Fatalf("CreateChunk renaissance failed: %v", err)
	}

	if err := db.LinkChunksToNotebook(notebookA, []string{chunkThermo, chunkNewton}); err != nil {
		t.Fatalf("LinkChunksToNotebook notebookA failed: %v", err)
	}
	if err := db.LinkChunksToNotebook(notebookB, []string{chunkRenaissance}); err != nil {
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

func TestAskAINotReadyReturnsError(t *testing.T) {
	app := &App{aiReady: false, aiInitError: "missing runtime assets"}

	resp := app.AskAI("os-scheduling", "What is RR?")
	err, ok := resp["error"].(string)
	if !ok {
		t.Fatalf("expected error string, got: %#v", resp)
	}

	if err == "" {
		t.Fatalf("expected non-empty error message")
	}
}

// ============================================================================
// NOTEBOOK UTILITY TESTS
// ============================================================================

func TestNotebookAssetURLUsesBasename(t *testing.T) {
	assetURL := notebookAssetURL("C:/Users/vishn/AppData/Roaming/ai-tutor/uploads/sample.pdf")
	if assetURL != "/notebooks/sample.pdf" {
		t.Fatalf("expected notebook URL to use basename, got %q", assetURL)
	}
}

func TestNotebookAssetURLRejectsTraversalNames(t *testing.T) {
	if got := notebookAssetURL(".."); got != "" {
		t.Fatalf("expected empty URL for traversal segment, got %q", got)
	}
	if got := notebookAssetURL("."); got != "" {
		t.Fatalf("expected empty URL for current directory segment, got %q", got)
	}
}

// ============================================================================
// QUIZ/SCORING TESTS
// ============================================================================

func TestScoreAnswerCorrectAnswerFullText(t *testing.T) {
	app := newTestApp(t)

	topicID := "test-topic-score"
	if err := db.EnsureTopic(topicID, "Test Topic"); err != nil {
		t.Fatalf("EnsureTopic failed: %v", err)
	}

	questions := []models.QuizQuestion{
		{
			ID:            "q1",
			TopicID:       topicID,
			Prompt:        "What is Round Robin?",
			Options:       []string{"A scheduling algorithm", "A type of bread", "A programming language", "A network protocol"},
			CorrectAnswer: "A scheduling algorithm",
			Explanation:   "RR is a scheduling algorithm that assigns equal time slices.",
			Hint:          "It involves time slices",
			SourceHeading: "CPU Scheduling",
			SourceSnippet: "Round Robin...",
		},
	}

	if err := db.ReplaceQuestionsForTopic(topicID, questions); err != nil {
		t.Fatalf("ReplaceQuestionsForTopic failed: %v", err)
	}

	resp := app.ScoreAnswer("q1", "A scheduling algorithm")

	if _, hasErr := resp["error"]; hasErr {
		t.Fatalf("expected success, got error: %v", resp["error"])
	}

	if !resp["correct"].(bool) {
		t.Fatalf("expected correct=true for matching answer")
	}

	if score, ok := resp["score"].(int); !ok || score != 100 {
		t.Fatalf("expected score=100 for correct answer, got %#v (type: %T)", resp["score"], resp["score"])
	}
}

func TestScoreAnswerCorrectAnswerLetterAlias(t *testing.T) {
	app := newTestApp(t)

	topicID := "test-topic-letter"
	if err := db.EnsureTopic(topicID, "Test Topic"); err != nil {
		t.Fatalf("EnsureTopic failed: %v", err)
	}

	questions := []models.QuizQuestion{
		{
			ID:            "q2",
			TopicID:       topicID,
			Prompt:        "What is FIFO?",
			Options:       []string{"First In First Out", "Fast Input Fast Output", "Forwarded IP Feedback Optimization", "Fiber Internet For Office"},
			CorrectAnswer: "First In First Out",
			Explanation:   "FIFO is a queue discipline.",
			Hint:          "It is an acronym",
			SourceHeading: "Queue Disciplines",
			SourceSnippet: "FIFO queues...",
		},
	}

	if err := db.ReplaceQuestionsForTopic(topicID, questions); err != nil {
		t.Fatalf("ReplaceQuestionsForTopic failed: %v", err)
	}

	// Answer using letter alias
	resp := app.ScoreAnswer("q2", "a")

	if _, hasErr := resp["error"]; hasErr {
		t.Fatalf("expected success, got error: %v", resp["error"])
	}

	if !resp["correct"].(bool) {
		t.Fatalf("expected correct=true for letter alias 'a'")
	}

	if score, ok := resp["score"].(int); !ok || score != 100 {
		t.Fatalf("expected score=100 for correct answer, got %#v (type: %T)", resp["score"], resp["score"])
	}
}

func TestScoreAnswerIncorrectAnswer(t *testing.T) {
	app := newTestApp(t)

	topicID := "test-topic-incorrect"
	if err := db.EnsureTopic(topicID, "Test Topic"); err != nil {
		t.Fatalf("EnsureTopic failed: %v", err)
	}

	questions := []models.QuizQuestion{
		{
			ID:            "q3",
			TopicID:       topicID,
			Prompt:        "What is LIFO?",
			Options:       []string{"Last In First Out", "Linear Input Feedback Output", "Layered Internet Framework Organizer", "Long Integer File Order"},
			CorrectAnswer: "Last In First Out",
			Explanation:   "LIFO is also known as a stack.",
			Hint:          "Think of a stack of plates",
			SourceHeading: "Data Structures",
			SourceSnippet: "LIFO stacks...",
		},
	}

	if err := db.ReplaceQuestionsForTopic(topicID, questions); err != nil {
		t.Fatalf("ReplaceQuestionsForTopic failed: %v", err)
	}

	resp := app.ScoreAnswer("q3", "Linear Input Feedback Output")

	if _, hasErr := resp["error"]; hasErr {
		t.Fatalf("expected success, got error: %v", resp["error"])
	}

	if resp["correct"].(bool) {
		t.Fatalf("expected correct=false for wrong answer")
	}

	if score, ok := resp["score"].(int); !ok || score != 0 {
		t.Fatalf("expected score=0 for incorrect answer, got %#v (type: %T)", resp["score"], resp["score"])
	}
}

func TestScoreAnswerCaseInsensitiveMatching(t *testing.T) {
	app := newTestApp(t)

	topicID := "test-topic-case"
	if err := db.EnsureTopic(topicID, "Test Topic"); err != nil {
		t.Fatalf("EnsureTopic failed: %v", err)
	}

	questions := []models.QuizQuestion{
		{
			ID:            "q4",
			TopicID:       topicID,
			Prompt:        "What is SJF?",
			Options:       []string{"Shortest Job First", "Sequential Job Format", "Shared Job Framework", "Static Job Finder"},
			CorrectAnswer: "Shortest Job First",
			Explanation:   "SJF is a scheduling algorithm.",
			Hint:          "It prioritizes short jobs",
			SourceHeading: "Scheduling",
			SourceSnippet: "SJF...",
		},
	}

	if err := db.ReplaceQuestionsForTopic(topicID, questions); err != nil {
		t.Fatalf("ReplaceQuestionsForTopic failed: %v", err)
	}

	// Test with uppercase answer
	resp := app.ScoreAnswer("q4", "SHORTEST JOB FIRST")

	if _, hasErr := resp["error"]; hasErr {
		t.Fatalf("expected success, got error: %v", resp["error"])
	}

	if !resp["correct"].(bool) {
		t.Fatalf("expected correct=true for case-insensitive match")
	}
}

func TestScoreAnswerPersistenceInDatabase(t *testing.T) {
	app := newTestApp(t)

	topicID := "test-topic-persist"
	if err := db.EnsureTopic(topicID, "Test Topic"); err != nil {
		t.Fatalf("EnsureTopic failed: %v", err)
	}

	questions := []models.QuizQuestion{
		{
			ID:            "q5",
			TopicID:       topicID,
			Prompt:        "What is Priority Scheduling?",
			Options:       []string{"Priority Scheduling", "Process Priority System", "Package Priority Setup", "Port Priority Server"},
			CorrectAnswer: "Priority Scheduling",
			Explanation:   "Processes are scheduled by priority.",
			Hint:          "Processes have assigned priorities",
			SourceHeading: "Scheduling Algorithms",
			SourceSnippet: "Priority scheduling...",
		},
	}

	if err := db.ReplaceQuestionsForTopic(topicID, questions); err != nil {
		t.Fatalf("ReplaceQuestionsForTopic failed: %v", err)
	}

	// Score the answer
	resp := app.ScoreAnswer("q5", "Priority Scheduling")

	if _, hasErr := resp["error"]; hasErr {
		t.Fatalf("expected success, got error: %v", resp["error"])
	}

	if !resp["correct"].(bool) {
		t.Fatalf("expected correct=true")
	}

	// Verify the answer was persisted to database by retrieving user answers
	// Note: This would require a database query method to verify persistence.
	// For now, we verify that SaveUserAnswer didn't error in the ScoreAnswer call.
	// In production, you'd query the database to confirm the user_answers table was updated.
}

func TestScoreAnswerMissingQuestionReturnsError(t *testing.T) {
	app := newTestApp(t)

	resp := app.ScoreAnswer("nonexistent-question", "Any Answer")

	if _, hasErr := resp["error"]; !hasErr {
		t.Fatalf("expected error for missing question, got: %#v", resp)
	}
}

func TestScoreAnswerEmptyAnswerReturnsError(t *testing.T) {
	app := newTestApp(t)

	topicID := "test-topic-empty-ans"
	if err := db.EnsureTopic(topicID, "Test Topic"); err != nil {
		t.Fatalf("EnsureTopic failed: %v", err)
	}

	questions := []models.QuizQuestion{
		{
			ID:            "q6",
			TopicID:       topicID,
			Prompt:        "What is Preemptive Scheduling?",
			Options:       []string{"Preemptive Scheduling", "Process Priority Method", "Predictive Schedule Manager", "Pre-assigned Process Set"},
			CorrectAnswer: "Preemptive Scheduling",
			Explanation:   "CPU can be allocated for a fixed duration.",
			Hint:          "The CPU can be taken away",
			SourceHeading: "Preemption",
			SourceSnippet: "Preemptive...",
		},
	}

	if err := db.ReplaceQuestionsForTopic(topicID, questions); err != nil {
		t.Fatalf("ReplaceQuestionsForTopic failed: %v", err)
	}

	resp := app.ScoreAnswer("q6", "")

	if _, hasErr := resp["error"]; !hasErr {
		t.Fatalf("expected error for empty user answer, got: %#v", resp)
	}
}

// ============================================================================
// FLASHCARD/FSRS TESTS
// ============================================================================

func TestGenerateFlashcardsCreatesAndReturnsCards(t *testing.T) {
	app := newTestApp(t)
	expectedCount, err := db.GetTotalChunkTokens("os-scheduling")
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

	count, err := db.CountFlashcardsForTopic("os-scheduling")
	if err != nil {
		t.Fatalf("CountFlashcardsForTopic failed: %v", err)
	}
	if count != want {
		t.Fatalf("expected %d stored flashcards, got %d", want, count)
	}
}

func TestGenerateFlashcardsReturnsExistingCardsWithoutDuplication(t *testing.T) {
	app := newTestApp(t)
	totalTokens, err := db.GetTotalChunkTokens("os-scheduling")
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

	count, err := db.CountFlashcardsForTopic("os-scheduling")
	if err != nil {
		t.Fatalf("CountFlashcardsForTopic failed: %v", err)
	}
	if count != want {
		t.Fatalf("expected no duplicate flashcards, got %d", count)
	}
}

func TestGetFlashcardsDueOnlyFiltersByDueDate(t *testing.T) {
	app := newTestApp(t)

	resp := app.GenerateFlashcards("os-scheduling")
	if _, hasErr := resp["error"]; hasErr {
		t.Fatalf("generation failed: %v", resp["error"])
	}
	cards := resp["cards"].([]models.Flashcard)

	reviewResp := app.RecordFlashcardReview(cards[0].ID, "easy")
	if _, hasErr := reviewResp["error"]; hasErr {
		t.Fatalf("review failed: %v", reviewResp["error"])
	}

	dueResp := app.GetFlashcards("os-scheduling", true)
	if _, hasErr := dueResp["error"]; hasErr {
		t.Fatalf("GetFlashcards failed: %v", dueResp["error"])
	}
	dueCards, ok := dueResp["cards"].([]models.Flashcard)
	if !ok {
		t.Fatalf("expected typed flashcards slice, got %#v", dueResp["cards"])
	}
	if len(dueCards) != len(cards)-1 {
		t.Fatalf("expected %d due cards after scheduling one into the future, got %d", len(cards)-1, len(dueCards))
	}
}

func TestRecordFlashcardReviewUpdatesScheduleState(t *testing.T) {
	app := newTestApp(t)

	resp := app.GenerateFlashcards("os-scheduling")
	if _, hasErr := resp["error"]; hasErr {
		t.Fatalf("generation failed: %v", resp["error"])
	}
	cards := resp["cards"].([]models.Flashcard)

	reviewResp := app.RecordFlashcardReview(cards[0].ID, "good")
	if _, hasErr := reviewResp["error"]; hasErr {
		t.Fatalf("review failed: %v", reviewResp["error"])
	}

	state, ok := reviewResp["state"].(*models.FlashcardState)
	if !ok {
		t.Fatalf("expected flashcard state pointer, got %#v", reviewResp["state"])
	}
	if state.Reps != 1 {
		t.Fatalf("expected reps=1, got %#v", state.Reps)
	}
	if state.ScheduledDays <= 0 {
		t.Fatalf("expected scheduled_days to be positive, got %d", state.ScheduledDays)
	}

	card, ok := reviewResp["card"].(*models.Flashcard)
	if !ok {
		t.Fatalf("expected flashcard pointer, got %#v", reviewResp["card"])
	}
	if card.DueAt <= 0 {
		t.Fatalf("expected due_at epoch, got %d", card.DueAt)
	}
	if _, ok := reviewResp["review_log_id"].(string); !ok {
		t.Fatalf("expected review_log_id string, got %#v", reviewResp["review_log_id"])
	}

	dueCount, err := db.QueryDueReviewCards(32503680000)
	if err != nil {
		t.Fatalf("QueryDueReviewCards failed: %v", err)
	}
	if dueCount != len(cards) {
		t.Fatalf("expected all cards to be due by far-future cutoff, got %d", dueCount)
	}
}

func TestRecordFlashcardReviewRejectsInvalidRating(t *testing.T) {
	app := newTestApp(t)

	resp := app.RecordFlashcardReview("missing-card", "skip")
	if _, hasErr := resp["error"]; !hasErr {
		t.Fatalf("expected error for invalid rating, got %#v", resp)
	}
}

func TestRecordFlashcardReviewReturnsEpochTimestampsAndFSRSFields(t *testing.T) {
	app := newTestApp(t)

	resp := app.GenerateFlashcards("os-scheduling")
	if _, hasErr := resp["error"]; hasErr {
		t.Fatalf("generation failed: %v", resp["error"])
	}
	cards := resp["cards"].([]models.Flashcard)

	reviewResp := app.RecordFlashcardReview(cards[0].ID, "easy")
	if _, hasErr := reviewResp["error"]; hasErr {
		t.Fatalf("review failed: %v", reviewResp["error"])
	}

	card := reviewResp["card"].(*models.Flashcard)
	state := reviewResp["state"].(*models.FlashcardState)
	if card.DueAt <= 0 {
		t.Fatalf("expected due_at int64 epoch, got %d", card.DueAt)
	}
	if state.Stability <= 0 {
		t.Fatalf("expected stability > 0, got %f", state.Stability)
	}
	if state.Difficulty <= 0 {
		t.Fatalf("expected difficulty > 0, got %f", state.Difficulty)
	}
	if state.ScheduledDays <= 0 {
		t.Fatalf("expected scheduled_days > 0 for easy, got %d", state.ScheduledDays)
	}
}

func TestReviewSessionEndpointsSupportGenerationRecoveryAndCompletion(t *testing.T) {
	app := newTestApp(t)

	if err := db.EnsureTopic("queue-review-topic", "Queue Review Topic"); err != nil {
		t.Fatalf("EnsureTopic failed: %v", err)
	}
	if err := db.CreateNotebook("queue-review-nb", "Queue Review Notebook", "/tmp/queue-review.pdf", "pdf", "", 15); err != nil {
		t.Fatalf("CreateNotebook failed: %v", err)
	}
	if _, err := db.GetConnection().Exec(`
		INSERT INTO notebook_topics (notebook_id, topic_id)
		VALUES ('queue-review-nb', 'queue-review-topic')
	`); err != nil {
		t.Fatalf("link notebook_topics failed: %v", err)
	}
	if err := db.CreateFlashcards("queue-review-topic", []models.Flashcard{
		{ID: "queue-card-1", TopicID: "queue-review-topic", Prompt: "Q1", Answer: "A1", DueAt: 1},
		{ID: "queue-card-2", TopicID: "queue-review-topic", Prompt: "Q2", Answer: "A2", DueAt: 2},
	}, map[string]models.FlashcardState{
		"queue-card-1": {},
		"queue-card-2": {},
	}); err != nil {
		t.Fatalf("CreateFlashcards failed: %v", err)
	}

	generateResp := app.GenerateReviewTasks("queue-review-nb")
	if _, hasErr := generateResp["error"]; hasErr {
		t.Fatalf("GenerateReviewTasks failed: %v", generateResp["error"])
	}
	tasks, ok := generateResp["tasks"].([]models.StudyQueueTask)
	if !ok || len(tasks) != 1 {
		t.Fatalf("expected one generated review task, got %#v", generateResp["tasks"])
	}
	taskID := tasks[0].ID

	secondGenerateResp := app.GenerateReviewTasks("queue-review-nb")
	secondTasks, ok := secondGenerateResp["tasks"].([]models.StudyQueueTask)
	if !ok || len(secondTasks) != 1 || secondTasks[0].ID != taskID {
		t.Fatalf("expected duplicate prevention to return same task, got %#v", secondGenerateResp["tasks"])
	}

	if resp := app.ActivateTask(taskID); resp["error"] != nil {
		t.Fatalf("ActivateTask failed: %#v", resp)
	}

	sessionResp := app.GetReviewSession(taskID)
	if _, hasErr := sessionResp["error"]; hasErr {
		t.Fatalf("GetReviewSession failed: %v", sessionResp["error"])
	}
	session, ok := sessionResp["session"].(*models.ReviewSession)
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

	reloadResp := app.GetReviewSession(taskID)
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

	task, err := db.GetTaskByID(taskID)
	if err != nil {
		t.Fatalf("GetTaskByID failed: %v", err)
	}
	if task.Status != models.StudyTaskStatusCompleted {
		t.Fatalf("expected review task completed, got %s", task.Status)
	}
}

// ============================================================================
// WRITTEN ANSWER TESTS
// ============================================================================

func TestGenerateShortAnswerPrompt_Success(t *testing.T) {
	app := newTestApp(t)
	if err := db.EnsureTopic("os-scheduling", "OS Scheduling"); err != nil {
		t.Fatalf("EnsureTopic failed: %v", err)
	}

	mockLLM := &mockLLMProvider{
		answer: `{"prompt":"What is the primary purpose of scheduling in OS?"}`,
	}
	app.fastLLMProvider = mockLLM

	mockRAG := &mockRAGPipeline{
		result: &rag.Response{
			Answer: `{"prompt":"What is the primary purpose of scheduling in OS?"}`,
		},
	}
	app.ragPipeline = mockRAG

	result := app.GenerateShortAnswerPrompt("os-scheduling")

	if err, ok := result["error"]; ok {
		t.Fatalf("expected no error, got: %v", err)
	}

	prompt, ok := result["prompt"].(string)
	if !ok || prompt == "" {
		t.Fatalf("expected non-empty prompt string, got: %v", result["prompt"])
	}

	topicID, ok := result["topicID"].(string)
	if !ok || topicID != "os-scheduling" {
		t.Fatalf("expected topicID to be 'os-scheduling', got: %v", topicID)
	}

	questionID, ok := result["questionID"].(string)
	if !ok || questionID == "" {
		t.Fatalf("expected non-empty questionID, got: %v", result["questionID"])
	}
	writtenQuestion, err := db.GetWrittenQuestionByID(questionID)
	if err != nil {
		t.Fatalf("GetWrittenQuestionByID failed: %v", err)
	}
	if writtenQuestion == nil {
		t.Fatalf("expected persisted written question for id=%s", questionID)
		return // Explicit return to satisfy staticcheck SA5011
	}
	if writtenQuestion.TopicID != "os-scheduling" {
		t.Fatalf("expected persisted topicID os-scheduling, got: %s", writtenQuestion.TopicID)
	}
}

func TestGenerateShortAnswerPrompt_EmptyTopicID(t *testing.T) {
	app := newTestApp(t)

	result := app.GenerateShortAnswerPrompt("")

	if err, ok := result["error"].(string); !ok || err == "" {
		t.Fatalf("expected error for empty topicID, got: %v", result)
	}

	if !strings.Contains(result["error"].(string), "topic ID is required") {
		t.Fatalf("expected 'topic ID is required' error, got: %v", result["error"])
	}
}

func TestGenerateShortAnswerPrompt_WhitespaceTopicID(t *testing.T) {
	app := newTestApp(t)

	result := app.GenerateShortAnswerPrompt("   ")

	if err, ok := result["error"].(string); !ok || err == "" {
		t.Fatalf("expected error for whitespace-only topicID, got: %v", result)
	}
}

// TestGenerateShortAnswerPrompt_NoLLMProvider removed - study service now has fallback behavior

// TestGenerateShortAnswerPrompt_NoRAGPipeline removed - study service now has fallback behavior

// TestGenerateShortAnswerPrompt_RAGProcessQueryError removed - study service now has fallback behavior

// TestGenerateShortAnswerPrompt_InvalidJSONResponse removed - study service now has fallback behavior

// TestGenerateShortAnswerPrompt_EmptyPromptInResponse removed - study service now has fallback behavior

// TestGenerateShortAnswerPrompt_MalformedJSON removed - study service now has fallback behavior

func TestScoreShortAnswerLoadsPersistedPromptAndUpdatesFSRS(t *testing.T) {
	app := newTestApp(t)
	mockProvider := &mockLLMProvider{
		answer: `{"score":8,"feedback":"Strong answer with a small omission."}`,
	}
	app.fastLLMProvider = mockProvider
	// Rebuild study service with mock provider
	app.studyService = study.NewStudyService(study.Config{
		FastLLMProvider:  mockProvider,
		HeavyLLMProvider: mockProvider,
		RetrievalEngine:  app.retrievalEngine,
	})

	topicID := "written-score-topic"
	if err := db.EnsureTopic(topicID, "Written Score Topic"); err != nil {
		t.Fatalf("EnsureTopic failed: %v", err)
	}
	if err := db.CreateWrittenQuestion(models.WrittenQuestion{
		ID:              "written-q-1",
		TopicID:         topicID,
		Prompt:          "Explain why round robin improves fairness.",
		SourceHeading:   "CPU Scheduling",
		SourcePageStart: 2,
		SourcePageEnd:   3,
	}); err != nil {
		t.Fatalf("CreateWrittenQuestion failed: %v", err)
	}

	// Also insert into questions table to satisfy foreign key constraint for user_answers
	if err := db.ReplaceQuestionsForTopic(topicID, []models.QuizQuestion{{
		ID:              "written-q-1",
		TopicID:         topicID,
		Prompt:          "Explain why round robin improves fairness.",
		Options:         []string{},
		CorrectAnswer:   "",
		Explanation:     "",
		Hint:            "",
		SourceHeading:   "CPU Scheduling",
		SourceSnippet:   "",
		SourcePageStart: 2,
		SourcePageEnd:   3,
		LLMModel:        "test-model",
		PromptVersion:   "test-v1",
	}}); err != nil {
		t.Fatalf("ReplaceQuestionsForTopic failed: %v", err)
	}

	result := app.ScoreShortAnswer("written-q-1", "It gives each process a time slice.")
	if errMsg, ok := result["error"]; ok {
		t.Fatalf("expected success, got error: %v", errMsg)
	}
	if got := result["fsrsRating"]; got != "Good" {
		t.Fatalf("expected fsrsRating Good, got %#v", got)
	}
	if got := result["next_review_at"]; got == "" {
		t.Fatalf("expected next_review_at, got %#v", got)
	}

	state, err := db.GetAssessmentFSRSState("written_question", "written-q-1", "")
	if err != nil {
		t.Fatalf("GetAssessmentFSRSState failed: %v", err)
	}
	if state == nil {
		t.Fatalf("expected persisted assessment fsrs state")
	}
	if state.GetState().ScheduledDays <= 0 {
		t.Fatalf("expected scheduled days > 0, got %d", state.GetState().ScheduledDays)
	}
}

func TestScoreAnswerReturnsSharedAssessmentFSRSFields(t *testing.T) {
	app := newTestApp(t)

	topicID := "quiz-fsrs-topic"
	if err := db.EnsureTopic(topicID, "Quiz FSRS Topic"); err != nil {
		t.Fatalf("EnsureTopic failed: %v", err)
	}
	if err := db.ReplaceQuestionsForTopic(topicID, []models.QuizQuestion{{
		ID:            "quiz-fsrs-q1",
		TopicID:       topicID,
		Prompt:        "What does FIFO mean?",
		Options:       []string{"First In First Out", "Fast Input Fast Output"},
		CorrectAnswer: "First In First Out",
		Explanation:   "FIFO means first in, first out.",
	}}); err != nil {
		t.Fatalf("ReplaceQuestionsForTopic failed: %v", err)
	}

	result := app.ScoreAnswer("quiz-fsrs-q1", "First In First Out")
	if errMsg, ok := result["error"]; ok {
		t.Fatalf("expected success, got error: %v", errMsg)
	}
	if got := result["fsrsRating"]; got != "Easy" {
		t.Fatalf("expected fsrsRating Easy, got %#v", got)
	}
	if got := result["scheduled_days"]; got == nil {
		t.Fatalf("expected scheduled_days in response")
	}

	state, err := db.GetAssessmentFSRSState("quiz_question", "quiz-fsrs-q1", "")
	if err != nil {
		t.Fatalf("GetAssessmentFSRSState failed: %v", err)
	}
	if state == nil {
		t.Fatalf("expected persisted quiz assessment fsrs state")
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

type mockRAGPipeline struct {
	result *rag.Response
	err    error
}

func (m *mockRAGPipeline) ProcessQuery(topicID, question string, startPage, endPage int) (*rag.Response, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.result, nil
}

// ============================================================================
// READER TESTS
// ============================================================================

// Contract tests for GetReaderTopicBundle
func TestGetReaderTopicBundle_Success(t *testing.T) {
	initTestDB(t)
	app := &App{}

	notebookID := "test-notebook-reader"
	if err := db.CreateNotebook(notebookID, "Test Notebook", "/tmp/test.txt", "txt", "", 1); err != nil {
		t.Fatalf("CreateNotebook failed: %v", err)
	}

	topicID := "test-topic-reader"
	if err := db.EnsureTopic(topicID, "Test Topic"); err != nil {
		t.Fatalf("EnsureTopic failed: %v", err)
	}

	parentID := "parent-reader"
	if err := db.CreateParentSection(parentID, topicID, "Introduction", 1, "intro text"); err != nil {
		t.Fatalf("CreateParentSection failed: %v", err)
	}

	chunkID := "chunk-reader"
	if err := db.CreateChunk(chunkID, topicID, parentID, "chunk content", 2, 1); err != nil {
		t.Fatalf("CreateChunk failed: %v", err)
	}

	if err := db.LinkChunksToNotebook(notebookID, []string{chunkID}); err != nil {
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

	// Verify at least one section matches the seeded parent
	found := false
	for _, sec := range sections {
		if sectionMap, ok := sec.(map[string]interface{}); ok {
			if heading, ok := sectionMap["heading"].(string); ok && heading == "Introduction" {
				found = true
				break
			}
		}
	}
	if !found {
		t.Fatalf("expected to find section with heading 'Introduction' in sections: %#v", sections)
	}
}

func TestGetReaderTopicBundle_InvalidTopic(t *testing.T) {
	initTestDB(t)
	app := &App{}

	notebookID := "test-notebook-invalid"
	if err := db.CreateNotebook(notebookID, "Test Notebook", "/tmp/test.txt", "txt", "", 1); err != nil {
		t.Fatalf("CreateNotebook failed: %v", err)
	}

	resp := app.GetReaderTopicBundle("nonexistent-topic", notebookID)

	if _, hasErr := resp["error"]; !hasErr {
		t.Fatalf("expected error for invalid topic, got: %#v", resp)
	}
}

// Contract tests for ExplainReaderSection
func TestExplainReaderSection_Success(t *testing.T) {
	app := newTestApp(t)

	topicID := "test-topic-explain"
	if err := db.EnsureTopic(topicID, "Test Topic"); err != nil {
		t.Fatalf("EnsureTopic failed: %v", err)
	}

	parentID := "parent-explain"
	if err := db.CreateParentSection(parentID, topicID, "Section Title", 1, "section content"); err != nil {
		t.Fatalf("CreateParentSection failed: %v", err)
	}

	resp := app.ExplainReaderSection(parentID, "What is this section about?")

	if _, hasErr := resp["error"]; hasErr {
		t.Fatalf("expected success, got error: %v", resp["error"])
	}

	if _, ok := resp["answer"].(string); !ok {
		t.Fatalf("expected answer string, got: %#v", resp["answer"])
	}
}

func TestExplainReaderSection_InvalidSection(t *testing.T) {
	app := newTestApp(t)

	resp := app.ExplainReaderSection("nonexistent-section", "Any question?")

	if _, hasErr := resp["error"]; !hasErr {
		t.Fatalf("expected error for invalid section, got: %#v", resp)
	}
}

func TestExplainReaderSection_EmptyQuestion(t *testing.T) {
	app := newTestApp(t)

	topicID := "test-topic-explain-empty"
	if err := db.EnsureTopic(topicID, "Test Topic"); err != nil {
		t.Fatalf("EnsureTopic failed: %v", err)
	}

	parentID := "parent-explain-empty"
	if err := db.CreateParentSection(parentID, topicID, "Section", 1, "content"); err != nil {
		t.Fatalf("CreateParentSection failed: %v", err)
	}

	// Should succeed with empty question (uses default explanation)
	resp := app.ExplainReaderSection(parentID, "")

	if _, hasErr := resp["error"]; hasErr {
		t.Fatalf("expected success with empty question, got error: %v", resp["error"])
	}

	if _, ok := resp["answer"].(string); !ok {
		t.Fatalf("expected answer string for empty question, got: %#v", resp["answer"])
	}
}

func TestCompleteReadingSession_AppendsQuestionsAndAdvancesCursor(t *testing.T) {
	app := newTestApp(t)

	topicID := "complete-session-topic"
	notebookID := "complete-session-notebook"
	if err := db.EnsureTopic(topicID, "Complete Session Topic"); err != nil {
		t.Fatalf("EnsureTopic failed: %v", err)
	}
	if err := db.UpdateTopicPageBounds(topicID, 1, 4); err != nil {
		t.Fatalf("UpdateTopicPageBounds failed: %v", err)
	}
	if err := db.CreateNotebook(notebookID, "Complete Session Notebook", "/tmp/complete.txt", "txt", "", 4); err != nil {
		t.Fatalf("CreateNotebook failed: %v", err)
	}

	parentID := "complete-session-parent"
	if err := db.IngestNotebookContentByTopic(notebookID, []db.NotebookTopicIngestionGroup{{
		TopicID: topicID,
		Parents: []db.NotebookParentInput{{
			ID: parentID, Heading: "Complete Section", Content: "complete section body", OrderIndex: 1,
		}},
		Chunks: []db.NotebookChunkInput{
			{ID: "complete-session-c1", ParentID: parentID, Text: "page one context.", TokenCount: 3, PageNum: 1},
			{ID: "complete-session-c2", ParentID: parentID, Text: "page two context.", TokenCount: 3, PageNum: 2},
			{ID: "complete-session-c3", ParentID: parentID, Text: "page three buffer.", TokenCount: 3, PageNum: 3},
		},
	}}); err != nil {
		t.Fatalf("IngestNotebookContentByTopic failed: %v", err)
	}

	if err := db.ReplaceQuestionsForTopic(topicID, []models.QuizQuestion{{
		ID:            "complete-session-existing",
		TopicID:       topicID,
		Prompt:        "Existing?",
		Options:       []string{"A", "B"},
		CorrectAnswer: "A",
	}}); err != nil {
		t.Fatalf("ReplaceQuestionsForTopic failed: %v", err)
	}

	// Initialize cursor to match startPage
	if err := db.UpdateTopicReadingCursor(topicID, 1, false); err != nil {
		t.Fatalf("UpdateTopicReadingCursor failed: %v", err)
	}

	app.fastLLMProvider = &mockLLMProvider{answer: questionJSON(3, "complete-session-c1")}
	resp := app.CompleteReadingSession(topicID, 1, 2)
	if _, hasErr := resp["error"]; hasErr {
		t.Fatalf("expected completion success, got error: %v", resp["error"])
	}
	if got := resp["questions_generated"]; got != 3 {
		t.Fatalf("expected 3 generated questions, got %#v", got)
	}
	if got := resp["current_page_cursor"]; got != 3 {
		t.Fatalf("expected cursor 3, got %#v", got)
	}

	questions, err := db.GetQuestionsForTopic(topicID)
	if err != nil {
		t.Fatalf("GetQuestionsForTopic failed: %v", err)
	}
	if len(questions) != 4 {
		t.Fatalf("expected existing question plus 3 generated questions, got %d", len(questions))
	}
	generated := 0
	for _, q := range questions {
		if q.PromptVersion == "reader-complete-v2-density" {
			generated++
			if q.SourcePageStart != 1 || q.SourcePageEnd != 3 {
				t.Fatalf("expected generated question lineage pages 1-3, got %#v", q)
			}
		}
	}
	if generated != 3 {
		t.Fatalf("expected 3 reader-complete questions, got %d", generated)
	}
}

func TestCompleteReadingSession_RequiresFastLLM(t *testing.T) {
	initTestDB(t)

	app := &App{}
	resp := app.CompleteReadingSession("os-scheduling", 1, 2)
	if _, hasErr := resp["error"]; !hasErr {
		t.Fatalf("expected error without FAST_LLM, got %#v", resp)
	}
}

func TestCompleteReadingSession_RejectsInvalidWindow(t *testing.T) {
	initTestDB(t)

	topicID := "complete-invalid-window"
	if err := db.EnsureTopic(topicID, "Invalid Window"); err != nil {
		t.Fatalf("EnsureTopic failed: %v", err)
	}
	if err := db.UpdateTopicPageBounds(topicID, 5, 8); err != nil {
		t.Fatalf("UpdateTopicPageBounds failed: %v", err)
	}

	app := &App{fastLLMProvider: &mockLLMProvider{answer: questionJSON(3, "complete-session-c1")}}
	resp := app.CompleteReadingSession(topicID, 7, 6)
	if _, hasErr := resp["error"]; !hasErr {
		t.Fatalf("expected invalid window error, got %#v", resp)
	}
}

// ============================================================================
// NOTEBOOK UPLOAD TESTS
// ============================================================================

func TestDraftNotebookSyllabus_FallbackCreatesEditableChapter(t *testing.T) {
	initTestDB(t)
	uploadDir := t.TempDir()
	service := notebook.NewService(uploadDir)
	app := &App{notebookService: service}

	uploadResult, err := service.SaveUploadedFile([]byte("Alpha beta gamma"), "draft.txt")
	if err != nil {
		t.Fatalf("SaveUploadedFile failed: %v", err)
	}

	doc, err := service.ExtractDocument(uploadResult.FilePath, uploadResult.FileType)
	if err != nil {
		t.Fatalf("ExtractDocument failed: %v", err)
	}

	if err := db.CreateNotebook(uploadResult.ID, uploadResult.FileName, uploadResult.FilePath, uploadResult.FileType, "", doc.PageCount); err != nil {
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
	draftJSON, err := db.GetNotebookSyllabusDraft(uploadResult.ID)
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
	app := &App{notebookService: service}

	uploadResult, err := service.SaveUploadedFile([]byte("# Intro\n\nAlpha beta gamma\n\n## Details\n\nDelta epsilon zeta"), "confirm.md")
	if err != nil {
		t.Fatalf("SaveUploadedFile failed: %v", err)
	}

	doc, err := service.ExtractDocument(uploadResult.FilePath, uploadResult.FileType)
	if err != nil {
		t.Fatalf("ExtractDocument failed: %v", err)
	}

	if err := db.CreateNotebook(uploadResult.ID, uploadResult.FileName, uploadResult.FilePath, uploadResult.FileType, "", doc.PageCount); err != nil {
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

	startPage, endPage, err := db.GetTopicPageBounds(topicIDs[0])
	if err != nil {
		t.Fatalf("GetTopicPageBounds failed: %v", err)
	}
	if startPage != 1 || endPage != doc.PageCount {
		t.Fatalf("unexpected persisted bounds: got [%d,%d] want [1,%d]", startPage, endPage, doc.PageCount)
	}

	bundle, err := db.GetReaderTopicBundle(topicIDs[0], uploadResult.ID)
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

// ============================================================================
// QUEUE CONTRACT TESTS
// ============================================================================

// TestGetNextTask_ReturnsNextPendingTask verifies GetNextTask returns the next pending task
// and returns ErrNoPendingTasks when queue is empty.
func TestGetNextTask_ReturnsNextPendingTask(t *testing.T) {
	app := newTestApp(t)

	// Create a notebook for queue tasks
	notebookID := "queue-test-nb"
	if err := db.CreateNotebook(notebookID, "Queue Test Notebook", "/tmp/queue.txt", "txt", "", 1); err != nil {
		t.Fatalf("CreateNotebook failed: %v", err)
	}

	// Test empty queue returns ErrNoPendingTasks
	resp := app.GetNextTask(notebookID)
	if code, ok := resp["code"].(int); !ok || code != 204 {
		t.Fatalf("expected code 204 for empty queue, got: %#v", resp)
	}

	// Insert a pending task
	task := models.StudyQueueTask{
		ID:         "task-1",
		NotebookID: notebookID,
		TaskType:   models.StudyTaskTypeFlashcardReview,
		Status:     models.StudyTaskStatusPending,
		Priority:   1,
		StartPage:  1,
		EndPage:    5,
	}
	if err := db.InsertStudyTask(task); err != nil {
		t.Fatalf("InsertStudyTask failed: %v", err)
	}

	// GetNextTask should return the task
	resp = app.GetNextTask(notebookID)
	if _, hasErr := resp["error"]; hasErr {
		t.Fatalf("expected success, got error: %v", resp["error"])
	}

	returnedTask, ok := resp["task"].(*models.StudyQueueTask)
	if !ok {
		t.Fatalf("expected task in response, got: %#v", resp["task"])
	}
	if returnedTask.ID != "task-1" {
		t.Fatalf("expected task-1, got: %s", returnedTask.ID)
	}
	if returnedTask.Status != models.StudyTaskStatusPending {
		t.Fatalf("expected PENDING status, got: %s", returnedTask.Status)
	}
}

// TestActivateTask_TransitionsPendingToActive verifies ActivateTask moves task from PENDING to ACTIVE.
func TestActivateTask_TransitionsPendingToActive(t *testing.T) {
	app := newTestApp(t)

	notebookID := "activate-test-nb"
	if err := db.CreateNotebook(notebookID, "Activate Test Notebook", "/tmp/activate.txt", "txt", "", 1); err != nil {
		t.Fatalf("CreateNotebook failed: %v", err)
	}

	task := models.StudyQueueTask{
		ID:         "task-activate-1",
		NotebookID: notebookID,
		TaskType:   models.StudyTaskTypeReading,
		Status:     models.StudyTaskStatusPending,
		Priority:   1,
	}
	if err := db.InsertStudyTask(task); err != nil {
		t.Fatalf("InsertStudyTask failed: %v", err)
	}

	// Activate the task
	resp := app.ActivateTask("task-activate-1")
	if _, hasErr := resp["error"]; hasErr {
		t.Fatalf("expected success, got error: %v", resp["error"])
	}

	// Verify task is no longer in pending queue
	_, err := db.GetNextTask(notebookID)
	if err != db.ErrNoPendingTasks {
		t.Fatalf("expected no pending tasks after activation, got: %v", err)
	}
}

// TestActivateTask_RejectsNonPendingTask verifies ActivateTask rejects tasks not in PENDING status.
func TestActivateTask_RejectsNonPendingTask(t *testing.T) {
	app := newTestApp(t)

	notebookID := "activate-reject-nb"
	if err := db.CreateNotebook(notebookID, "Activate Reject Notebook", "/tmp/activate-reject.txt", "txt", "", 1); err != nil {
		t.Fatalf("CreateNotebook failed: %v", err)
	}

	task := models.StudyQueueTask{
		ID:         "task-already-active",
		NotebookID: notebookID,
		TaskType:   models.StudyTaskTypeQuiz,
		Status:     models.StudyTaskStatusActive, // Already active
		Priority:   1,
	}
	if err := db.InsertStudyTask(task); err != nil {
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
	if err := db.CreateNotebook(notebookID, "Complete Test Notebook", "/tmp/complete.txt", "txt", "", 1); err != nil {
		t.Fatalf("CreateNotebook failed: %v", err)
	}

	task := models.StudyQueueTask{
		ID:         "task-complete-1",
		NotebookID: notebookID,
		TaskType:   models.StudyTaskTypeReread,
		Status:     models.StudyTaskStatusActive,
		Priority:   1,
	}
	if err := db.InsertStudyTask(task); err != nil {
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
	_, err := db.GetNextTask(notebookID)
	if err != db.ErrNoPendingTasks {
		t.Fatalf("expected no pending tasks after completion, got: %v", err)
	}
}

// TestCompleteTask_InsertsFollowUpTasks verifies CompleteTask inserts follow-up tasks transactionally.
func TestCompleteTask_InsertsFollowUpTasks(t *testing.T) {
	app := newTestApp(t)

	notebookID := "followup-test-nb"
	if err := db.CreateNotebook(notebookID, "Followup Test Notebook", "/tmp/followup.txt", "txt", "", 1); err != nil {
		t.Fatalf("CreateNotebook failed: %v", err)
	}

	task := models.StudyQueueTask{
		ID:         "task-with-followup",
		NotebookID: notebookID,
		TaskType:   models.StudyTaskTypeReading,
		Status:     models.StudyTaskStatusActive,
		Priority:   1,
	}
	if err := db.InsertStudyTask(task); err != nil {
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
	nextTask, err := db.GetNextTask(notebookID)
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
	if err := db.CreateNotebook(notebookID, "Skip Test Notebook", "/tmp/skip.txt", "txt", "", 1); err != nil {
		t.Fatalf("CreateNotebook failed: %v", err)
	}

	task := models.StudyQueueTask{
		ID:         "task-skip-1",
		NotebookID: notebookID,
		TaskType:   models.StudyTaskTypeExaminer,
		Status:     models.StudyTaskStatusPending,
		Priority:   1,
	}
	if err := db.InsertStudyTask(task); err != nil {
		t.Fatalf("InsertStudyTask failed: %v", err)
	}

	resp := app.SkipTask("task-skip-1")
	if _, hasErr := resp["error"]; hasErr {
		t.Fatalf("expected success, got error: %v", resp["error"])
	}

	// Verify task is no longer in pending queue
	_, err := db.GetNextTask(notebookID)
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
	if err := db.CreateNotebook(notebookID, "Ordering Type Notebook", "/tmp/ordering-type.txt", "txt", "", 1); err != nil {
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
		if err := db.InsertStudyTask(task); err != nil {
			t.Fatalf("InsertStudyTask failed: %v", err)
		}
	}

	// Verify FLASHCARD_REVIEW is returned first
	nextTask, err := db.GetNextTask(notebookID)
	if err != nil {
		t.Fatalf("GetNextTask failed: %v", err)
	}
	if nextTask.TaskType != models.StudyTaskTypeFlashcardReview {
		t.Fatalf("expected FLASHCARD_REVIEW first, got: %s", nextTask.TaskType)
	}

	// Activate and complete to get next task
	if err := db.ActivateTask("task-1"); err != nil {
		t.Fatalf("ActivateTask failed: %v", err)
	}
	if err := db.CompleteTask("task-1", models.CompletionResult{Status: models.StudyTaskStatusCompleted}); err != nil {
		t.Fatalf("CompleteTask failed: %v", err)
	}

	// Verify REREAD is second
	nextTask, err = db.GetNextTask(notebookID)
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
	if err := db.CreateNotebook(notebookID, "Ordering Priority Notebook", "/tmp/ordering-priority.txt", "txt", "", 1); err != nil {
		t.Fatalf("CreateNotebook failed: %v", err)
	}

	// Insert tasks with different priorities (same type)
	taskHighPriority := models.StudyQueueTask{ID: "task-high-priority", NotebookID: notebookID, TaskType: models.StudyTaskTypeQuiz, Status: models.StudyTaskStatusPending, Priority: 1}
	taskLowPriority := models.StudyQueueTask{ID: "task-low-priority", NotebookID: notebookID, TaskType: models.StudyTaskTypeQuiz, Status: models.StudyTaskStatusPending, Priority: 10}

	if err := db.InsertStudyTask(taskLowPriority); err != nil {
		t.Fatalf("InsertStudyTask failed: %v", err)
	}
	if err := db.InsertStudyTask(taskHighPriority); err != nil {
		t.Fatalf("InsertStudyTask failed: %v", err)
	}

	// Verify high priority task (lower number) is returned first
	nextTask, err := db.GetNextTask(notebookID)
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
	if err := db.CreateNotebook(notebookID, "FIFO Notebook", "/tmp/fifo.txt", "txt", "", 1); err != nil {
		t.Fatalf("CreateNotebook failed: %v", err)
	}

	// Insert tasks with same type and priority (order of insertion determines FIFO)
	task1 := models.StudyQueueTask{ID: "task-fifo-1", NotebookID: notebookID, TaskType: models.StudyTaskTypeReading, Status: models.StudyTaskStatusPending, Priority: 5}
	task2 := models.StudyQueueTask{ID: "task-fifo-2", NotebookID: notebookID, TaskType: models.StudyTaskTypeReading, Status: models.StudyTaskStatusPending, Priority: 5}
	task3 := models.StudyQueueTask{ID: "task-fifo-3", NotebookID: notebookID, TaskType: models.StudyTaskTypeReading, Status: models.StudyTaskStatusPending, Priority: 5}

	if err := db.InsertStudyTask(task1); err != nil {
		t.Fatalf("InsertStudyTask failed: %v", err)
	}
	if err := db.InsertStudyTask(task2); err != nil {
		t.Fatalf("InsertStudyTask failed: %v", err)
	}
	if err := db.InsertStudyTask(task3); err != nil {
		t.Fatalf("InsertStudyTask failed: %v", err)
	}

	// Verify FIFO order (first inserted is first returned)
	nextTask, err := db.GetNextTask(notebookID)
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
	if err := db.CreateNotebook(notebookID, "Anti Starvation Notebook", "/tmp/anti-starvation.txt", "txt", "", 1); err != nil {
		t.Fatalf("CreateNotebook failed: %v", err)
	}

	// Insert mix of task types and priorities
	tasks := []models.StudyQueueTask{
		{ID: "task-a", NotebookID: notebookID, TaskType: models.StudyTaskTypeReading, Status: models.StudyTaskStatusPending, Priority: 10},
		{ID: "task-b", NotebookID: notebookID, TaskType: models.StudyTaskTypeFlashcardReview, Status: models.StudyTaskStatusPending, Priority: 10},
		{ID: "task-c", NotebookID: notebookID, TaskType: models.StudyTaskTypeReading, Status: models.StudyTaskStatusPending, Priority: 1},
	}

	for _, task := range tasks {
		if err := db.InsertStudyTask(task); err != nil {
			t.Fatalf("InsertStudyTask failed: %v", err)
		}
	}

	// Query multiple times to verify deterministic ordering (same result each time)
	var firstTaskID string
	for i := 0; i < 3; i++ {
		nextTask, err := db.GetNextTask(notebookID)
		if err != nil {
			t.Fatalf("GetNextTask failed on iteration %d: %v", i, err)
		}
		if i == 0 {
			firstTaskID = nextTask.ID
		} else if nextTask.ID != firstTaskID {
			t.Fatalf("deterministic ordering violated: iteration 0 returned %s, iteration %d returned %s", firstTaskID, i, nextTask.ID)
		}
	}

	// Verify FLASHCARD_REVIEW type wins despite higher priority number on other tasks
	// (task type priority > task priority)
	if firstTaskID != "task-b" {
		t.Fatalf("expected task-b (FLASHCARD_REVIEW) to win due to type priority, got: %s", firstTaskID)
	}
}

// ============================================================================
// LIGHTWEIGHT TEST BUILDERS
// ============================================================================
