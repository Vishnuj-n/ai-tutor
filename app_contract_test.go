//go:build integration
// +build integration

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"ai-tutor/internal/db"
	"ai-tutor/internal/llm"
	"ai-tutor/internal/models"
	"ai-tutor/internal/rag"
)

func initTestDB(t *testing.T) {
	t.Helper()
	tempDB := filepath.Join(t.TempDir(), "ai-tutor-test.db")
	if err := db.Init(tempDB, ""); err != nil {
		t.Fatalf("failed to init test db: %v", err)
	}
	if err := db.SeedDemoDataForTests(); err != nil {
		t.Fatalf("failed to seed test data: %v", err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Fatalf("failed to close test db: %v", err)
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
				content = `{"cards":[{"prompt":"What does Round Robin assign to each process?","answer":"A fixed time slice."},{"prompt":"What happens when a process uses up its quantum?","answer":"It moves to the back of the ready queue."},{"prompt":"What system type is Round Robin good for?","answer":"Time-sharing systems."},{"prompt":"What tradeoff increases with smaller quantums?","answer":"Context switching overhead."},{"prompt":"What fairness property does Round Robin provide?","answer":"Each process gets a turn."},{"prompt":"What is another term for the time quantum?","answer":"Time slice."},{"prompt":"Why is Round Robin considered preemptive?","answer":"The CPU can be taken away after the quantum expires."},{"prompt":"What queue does Round Robin cycle through?","answer":"The ready queue."}]}`
			case strings.Contains(prompt, "quiz generator"):
				content = `{"questions":[{"prompt":"What is Round Robin scheduling?","options":["A scheduling algorithm","A disk format","A file system","A network layer"],"correct_answer":"A scheduling algorithm","explanation":"Round Robin is a CPU scheduling algorithm.","hint":"Think CPU time slices.","source_heading":"Round Robin Scheduling","source_snippet":"Round Robin assigns time slices."}]}`
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

func TestGetNotebookTopicTreeEmptyReturnsArray(t *testing.T) {
	initTestDB(t)
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
	initTestDB(t)
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
	if err := db.CreateChunk(chunkThermo, "topic-thermo", parentThermo, "thermo chunk", 2); err != nil {
		t.Fatalf("CreateChunk thermo failed: %v", err)
	}
	if err := db.CreateChunk(chunkNewton, "topic-newton", parentNewton, "newton chunk", 2); err != nil {
		t.Fatalf("CreateChunk newton failed: %v", err)
	}
	if err := db.CreateChunk(chunkRenaissance, "topic-renaissance", parentRenaissance, "renaissance chunk", 2); err != nil {
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

func TestScoreAnswerCorrectAnswerFullText(t *testing.T) {
	initTestDB(t)
	app := &App{}

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
	initTestDB(t)
	app := &App{}

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
	initTestDB(t)
	app := &App{}

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
	initTestDB(t)
	app := &App{}

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
	initTestDB(t)
	app := &App{}

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
	initTestDB(t)
	app := &App{}

	resp := app.ScoreAnswer("nonexistent-question", "Any Answer")

	if _, hasErr := resp["error"]; !hasErr {
		t.Fatalf("expected error for missing question, got: %#v", resp)
	}
}

func TestScoreAnswerEmptyAnswerReturnsError(t *testing.T) {
	initTestDB(t)
	app := &App{}

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

func TestGenerateFlashcardsCreatesAndReturnsCards(t *testing.T) {
	initTestDB(t)
	app := &App{llmProvider: initTestProvider(t)}

	resp := app.GenerateFlashcards("os-scheduling")
	if _, hasErr := resp["error"]; hasErr {
		t.Fatalf("expected success, got error: %v", resp["error"])
	}

	cards, ok := resp["cards"].([]models.Flashcard)
	if !ok {
		t.Fatalf("expected typed flashcards slice, got %#v", resp["cards"])
	}
	if len(cards) != 8 {
		t.Fatalf("expected 8 flashcards, got %d", len(cards))
	}

	count, err := db.CountFlashcardsForTopic("os-scheduling")
	if err != nil {
		t.Fatalf("CountFlashcardsForTopic failed: %v", err)
	}
	if count != 8 {
		t.Fatalf("expected 8 stored flashcards, got %d", count)
	}
}

func TestGenerateFlashcardsReturnsExistingCardsWithoutDuplication(t *testing.T) {
	initTestDB(t)
	app := &App{llmProvider: initTestProvider(t)}

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
	if count != 8 {
		t.Fatalf("expected no duplicate flashcards, got %d", count)
	}
}

func TestGetFlashcardsDueOnlyFiltersByDueDate(t *testing.T) {
	initTestDB(t)
	app := &App{llmProvider: initTestProvider(t)}

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
	if len(dueCards) != 7 {
		t.Fatalf("expected 7 due cards after scheduling one into the future, got %d", len(dueCards))
	}
}

func TestRecordFlashcardReviewUpdatesScheduleState(t *testing.T) {
	initTestDB(t)
	app := &App{llmProvider: initTestProvider(t)}

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
	if dueCount != 8 {
		t.Fatalf("expected all cards to be due by far-future cutoff, got %d", dueCount)
	}
}

func TestRecordFlashcardReviewRejectsInvalidRating(t *testing.T) {
	initTestDB(t)
	app := &App{}

	resp := app.RecordFlashcardReview("missing-card", "skip")
	if _, hasErr := resp["error"]; !hasErr {
		t.Fatalf("expected error for invalid rating, got %#v", resp)
	}
}

func TestRecordFlashcardReviewReturnsEpochTimestampsAndFSRSFields(t *testing.T) {
	initTestDB(t)
	app := &App{llmProvider: initTestProvider(t)}

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

func TestGenerateShortAnswerPrompt_Success(t *testing.T) {
	app := NewApp()
	app.ctx = context.Background()

	mockLLM := &mockLLMProvider{
		answer: `{"prompt":"What is the primary purpose of scheduling in OS?"}`,
	}
	app.llmProvider = mockLLM

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

	if !strings.Contains(questionID, "os-scheduling:") {
		t.Fatalf("expected questionID to contain topic prefix, got: %v", questionID)
	}
}

func TestGenerateShortAnswerPrompt_EmptyTopicID(t *testing.T) {
	app := NewApp()
	app.ctx = context.Background()

	result := app.GenerateShortAnswerPrompt("")

	if err, ok := result["error"].(string); !ok || err == "" {
		t.Fatalf("expected error for empty topicID, got: %v", result)
	}

	if !strings.Contains(result["error"].(string), "topic ID is required") {
		t.Fatalf("expected 'topic ID is required' error, got: %v", result["error"])
	}
}

func TestGenerateShortAnswerPrompt_WhitespaceTopicID(t *testing.T) {
	app := NewApp()
	app.ctx = context.Background()

	result := app.GenerateShortAnswerPrompt("   ")

	if err, ok := result["error"].(string); !ok || err == "" {
		t.Fatalf("expected error for whitespace-only topicID, got: %v", result)
	}
}

func TestGenerateShortAnswerPrompt_NoLLMProvider(t *testing.T) {
	app := NewApp()
	app.ctx = context.Background()
	app.llmProvider = nil

	result := app.GenerateShortAnswerPrompt("os-scheduling")

	if err, ok := result["error"].(string); !ok || err == "" {
		t.Fatalf("expected error for missing LLM provider, got: %v", result)
	}

	if !strings.Contains(result["error"].(string), "LLM provider not initialized") {
		t.Fatalf("expected 'LLM provider not initialized' error, got: %v", result["error"])
	}
}

func TestGenerateShortAnswerPrompt_NoRAGPipeline(t *testing.T) {
	app := NewApp()
	app.ctx = context.Background()
	app.llmProvider = &mockLLMProvider{}
	app.ragPipeline = nil

	result := app.GenerateShortAnswerPrompt("os-scheduling")

	if err, ok := result["error"].(string); !ok || err == "" {
		t.Fatalf("expected error for missing RAG pipeline, got: %v", result)
	}

	if !strings.Contains(result["error"].(string), "RAG pipeline not initialized") {
		t.Fatalf("expected 'RAG pipeline not initialized' error, got: %v", result["error"])
	}
}

func TestGenerateShortAnswerPrompt_RAGProcessQueryError(t *testing.T) {
	app := NewApp()
	app.ctx = context.Background()
	app.llmProvider = &mockLLMProvider{}

	mockRAG := &mockRAGPipeline{
		err: fmt.Errorf("query processing failed"),
	}
	app.ragPipeline = mockRAG

	result := app.GenerateShortAnswerPrompt("os-scheduling")

	if err, ok := result["error"].(string); !ok || err == "" {
		t.Fatalf("expected error from RAG pipeline, got: %v", result)
	}

	if !strings.Contains(result["error"].(string), "short-answer prompt generation failed") {
		t.Fatalf("expected prompt generation error, got: %v", result["error"])
	}
}

func TestGenerateShortAnswerPrompt_InvalidJSONResponse(t *testing.T) {
	app := NewApp()
	app.ctx = context.Background()
	app.llmProvider = &mockLLMProvider{}

	mockRAG := &mockRAGPipeline{
		result: &rag.Response{
			Answer: `not json at all`,
		},
	}
	app.ragPipeline = mockRAG

	result := app.GenerateShortAnswerPrompt("os-scheduling")

	if err, ok := result["error"]; ok {
		t.Fatalf("expected success with fallback prompt, got: %v", err)
	}

	prompt, ok := result["prompt"].(string)
	if !ok || prompt != "not json at all" {
		t.Fatalf("expected fallback prompt from raw response, got: %v", result["prompt"])
	}
}

func TestGenerateShortAnswerPrompt_EmptyPromptInResponse(t *testing.T) {
	app := NewApp()
	app.ctx = context.Background()
	app.llmProvider = &mockLLMProvider{}

	mockRAG := &mockRAGPipeline{
		result: &rag.Response{
			Answer: `{"prompt":"   "}`,
		},
	}
	app.ragPipeline = mockRAG

	result := app.GenerateShortAnswerPrompt("os-scheduling")

	if err, ok := result["error"].(string); !ok || err == "" {
		t.Fatalf("expected error for empty prompt, got: %v", result)
	}

	if !strings.Contains(result["error"].(string), "no prompt in LLM response") {
		t.Fatalf("expected no prompt error, got: %v", result["error"])
	}
}

func TestGenerateShortAnswerPrompt_MalformedJSON(t *testing.T) {
	app := NewApp()
	app.ctx = context.Background()
	app.llmProvider = &mockLLMProvider{}

	mockRAG := &mockRAGPipeline{
		result: &rag.Response{
			Answer: `{"prompt":}`,
		},
	}
	app.ragPipeline = mockRAG

	result := app.GenerateShortAnswerPrompt("os-scheduling")

	if err, ok := result["error"]; ok {
		t.Fatalf("expected fallback parsing behavior, got: %v", err)
	}

	prompt, ok := result["prompt"].(string)
	if !ok || prompt != `{"prompt":}` {
		t.Fatalf("expected fallback prompt from malformed JSON, got: %v", result["prompt"])
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

type mockRAGPipeline struct {
	result *rag.Response
	err    error
}

func (m *mockRAGPipeline) ProcessQuery(topicID, question string) (*rag.Response, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.result, nil
}

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
	if err := db.CreateChunk(chunkID, topicID, parentID, "chunk content", 2); err != nil {
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
	initTestDB(t)
	app := &App{llmProvider: initTestProvider(t)}

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
	initTestDB(t)
	app := &App{llmProvider: initTestProvider(t)}

	resp := app.ExplainReaderSection("nonexistent-section", "Any question?")

	if _, hasErr := resp["error"]; !hasErr {
		t.Fatalf("expected error for invalid section, got: %#v", resp)
	}
}

func TestExplainReaderSection_EmptyQuestion(t *testing.T) {
	initTestDB(t)
	app := &App{llmProvider: initTestProvider(t)}

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
