package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"ai-tutor/internal/llm"
	"ai-tutor/internal/models"
	"ai-tutor/internal/study"
)

// ============================================================================
// QUIZ TESTS
// ============================================================================

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

	cardExists, err := testRepo.FlashcardExistsByID("dummy-card-1")
	if err != nil {
		t.Fatalf("query FSRS cards count failed: %v", err)
	}
	if cardExists {
		t.Fatalf("expected FSRS cards to be deleted on max reread failure, but found")
	}

	failedTask, err := testRepo.GetTaskByID("task-quiz-max")
	if err != nil {
		t.Fatalf("query task status failed: %v", err)
	}
	if failedTask.Status != models.StudyTaskStatusCompleted {
		t.Fatalf("expected quiz task status to be COMPLETED, got %q", failedTask.Status)
	}

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

// ============================================================================
// REVIEW SESSION TESTS
// ============================================================================

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

// ============================================================================
// READER TESTS
// ============================================================================

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

	if _, ok := resp["topic_start_page"].(int); !ok {
		t.Fatalf("expected topic_start_page int, got: %#v", resp["topic_start_page"])
	}
	if _, ok := resp["topic_end_page"].(int); !ok {
		t.Fatalf("expected topic_end_page int, got: %#v", resp["topic_end_page"])
	}

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
// SOCRATIC/REQUIZ TESTS
// ============================================================================

func TestCompleteSocraticRescueInsertsRequiz(t *testing.T) {
	app := newTestApp(t)
	if err := testRepo.EnsureTopic("topic-test", "Topic Test"); err != nil {
		t.Fatalf("EnsureTopic failed: %v", err)
	}
	if err := testRepo.CreateNotebook("nb-test", "NB Test", "/tmp/nb-test.pdf", "pdf", "topic-test", 12); err != nil {
		t.Fatalf("CreateNotebook failed: %v", err)
	}
	mustInsertMockChunk(t, "nb-test", "topic-test", "chunk-socratic-test-1", 1)

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

	socraticTask, err := testRepo.GetTaskByID("task-socratic-test")
	if err != nil {
		t.Fatalf("failed to get socratic task: %v", err)
	}
	if socraticTask.Status != models.StudyTaskStatusCompleted {
		t.Fatalf("expected socratic task status to be COMPLETED, got %q", socraticTask.Status)
	}

	quizCount, err := testRepo.CountTasksByTopicTypeAndStatus("topic-test", "QUIZ", "PENDING")
	if err != nil {
		t.Fatalf("failed to query quiz task: %v", err)
	}
	if quizCount != 1 {
		t.Fatalf("expected 1 PENDING QUIZ task, got %d", quizCount)
	}

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

	questions, ok := payloadMap["questions"].([]interface{})
	if !ok || len(questions) == 0 {
		t.Fatalf("expected generated questions in payload, got none")
	}
}

func TestRequizPassGeneratesFlashcards(t *testing.T) {
	app := newTestApp(t)
	if err := testRepo.EnsureTopic("topic-requiz-pass", "Requiz Pass Topic"); err != nil {
		t.Fatalf("EnsureTopic failed: %v", err)
	}
	if err := testRepo.CreateNotebook("nb-requiz-pass", "NB Requiz Pass", "/tmp/nb-requiz-pass.pdf", "pdf", "topic-requiz-pass", 12); err != nil {
		t.Fatalf("CreateNotebook failed: %v", err)
	}

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
	if err := testRepo.EnsureTopic("topic-requiz-fail", "Requiz Fail Topic"); err != nil {
		t.Fatalf("failed to ensure topic: %v", err)
	}
	if err := testRepo.CreateNotebook("nb-requiz-fail", "NB Requiz Fail", "/tmp/nb-requiz-fail.pdf", "pdf", "topic-requiz-fail", 12); err != nil {
		t.Fatalf("CreateNotebook failed: %v", err)
	}

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

	var required bool
	err := testRepo.QueryRowForTest("SELECT external_help_required FROM topics WHERE id = 'topic-requiz-fail'").Scan(&required)
	if err != nil {
		t.Fatalf("failed to query topic: %v", err)
	}
	if !required {
		t.Fatalf("expected external_help_required to be true, got false")
	}
}

// ============================================================================
// FSRS CALIBRATION TESTS
// ============================================================================

func TestFSRSCalibrationEasyAndDoubleGood(t *testing.T) {
	app := newTestApp(t)

	mustInsertActiveQuizTask(t, "nb-calibration-1", "topic-calibration-1", "task-quiz-ace", 100)
	mustInsertMockChunk(t, "nb-calibration-1", "topic-calibration-1", "chunk-fallback", 3)

	respAce := app.SubmitQuizAttempt("task-quiz-ace", []models.QuizAnswer{
		{QuestionID: "quiz-q1", Selected: "A"},
		{QuestionID: "quiz-q2", Selected: "B"},
	})
	if _, hasErr := respAce["error"]; hasErr {
		t.Fatalf("expected submit success, got error: %v", respAce["error"])
	}

	genResp1 := app.GenerateFlashcardsForQuizTask("task-quiz-ace")
	if _, hasErr := genResp1["error"]; hasErr {
		t.Fatalf("expected flashcard generation success, got error: %v", genResp1["error"])
	}

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

	mustInsertActiveQuizTask(t, "nb-calibration-2", "topic-calibration-2", "task-quiz-pass", 50)
	mustInsertMockChunk(t, "nb-calibration-2", "topic-calibration-2", "chunk-fallback", 3)

	respPass := app.SubmitQuizAttempt("task-quiz-pass", []models.QuizAnswer{
		{QuestionID: "quiz-q1", Selected: "A"},
		{QuestionID: "quiz-q2", Selected: "C"},
	})
	if _, hasErr := respPass["error"]; hasErr {
		t.Fatalf("expected submit success, got error: %v", respPass["error"])
	}

	genResp2 := app.GenerateFlashcardsForQuizTask("task-quiz-pass")
	if _, hasErr := genResp2["error"]; hasErr {
		t.Fatalf("expected flashcard generation success, got error: %v", genResp2["error"])
	}

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

// Mock used by ScoreShortAnswer contract tests.
type mockLLMProviderForQuiz struct {
	answer string
	err    error
}

func (m *mockLLMProviderForQuiz) GenerateAnswer(prompt string) (string, error) {
	if m.err != nil {
		return "", m.err
	}
	return m.answer, nil
}

func (m *mockLLMProviderForQuiz) ModelName() string {
	return "mock-model"
}

func (m *mockLLMProviderForQuiz) GetLimits() llm.ModelLimits {
	return llm.ModelLimits{MaxInputTokens: 30000, MaxOutputTokens: 3000}
}
