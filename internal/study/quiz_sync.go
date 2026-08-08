package study

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"ai-tutor/internal/db"
	"ai-tutor/internal/embeddings"
	"ai-tutor/internal/models"
	"ai-tutor/internal/utils"

	"github.com/google/uuid"
)

const maxAutomaticRereadAttempts = 1

// GenerateFlashcardsAfterQuiz generates flashcards after successful quiz completion.
// New cards are future-dated and intentionally excluded from immediate review materialization.
func (s *StudyService) GenerateFlashcardsAfterQuiz(notebookID, topicID string, startPage, endPage int) (int, error) {
	utils.Warnf("[FLASHCARD_PIPELINE] flashcard_generation_started source=quiz_completion notebookID=%s topicID=%s startPage=%d endPage=%d pipeline=fsrs_cards_direct", notebookID, topicID, startPage, endPage)

	// Validate inputs
	notebookID = strings.TrimSpace(notebookID)
	topicID = strings.TrimSpace(topicID)
	if notebookID == "" || topicID == "" {
		utils.Warnf("[FLASHCARD_PIPELINE] flashcard_generation_skipped reason=invalid_inputs notebookID=%s topicID=%s", notebookID, topicID)
		return 0, fmt.Errorf("notebook ID and topic ID are required")
	}
	if startPage <= 0 || endPage <= 0 || endPage < startPage {
		utils.Warnf("[FLASHCARD_PIPELINE] flashcard_generation_skipped reason=invalid_page_range notebookID=%s topicID=%s startPage=%d endPage=%d", notebookID, topicID, startPage, endPage)
		return 0, fmt.Errorf("invalid page range")
	}

	utils.Warnf("[FLASHCARD_PIPELINE] flashcard_auto_flow_confirmation using_page_bounded_context=true page_range=%d-%d no_topic_aggregation=true", startPage, endPage)

	// Generate FSRS flashcards for the topic (bypassing manual sandbox)
	cards, _, _, _, err := s.GenerateFSRSCardsForTopic(topicID, notebookID, startPage, endPage)
	if err != nil {
		utils.Warnf("[FLASHCARD_PIPELINE] flashcard_generation_failed notebookID=%s topicID=%s error=%v", notebookID, topicID, err)
		return 0, err
	}

	cardCount := len(cards)
	utils.Warnf("[FLASHCARD_PIPELINE] flashcard_generation_completed notebookID=%s topicID=%s cardCount=%d", notebookID, topicID, cardCount)
	utils.Warnf("[FLASHCARD_PIPELINE] review_session_skipped notebookID=%s topicID=%s reason=deferred_new_cards_excluded", notebookID, topicID)
	return cardCount, nil
}

// GenerateQuizForPageRange generates a quiz from a notebook's page range.
// This is the manual entry point for exploratory quiz generation.
func (s *StudyService) GenerateQuizForPageRange(notebookID string, startPage, endPage int) map[string]interface{} {
	notebookID = strings.TrimSpace(notebookID)
	if notebookID == "" {
		return map[string]interface{}{"error": "notebook ID is required"}
	}
	if startPage <= 0 || endPage <= 0 || endPage < startPage {
		return map[string]interface{}{"error": fmt.Sprintf("invalid page range: start=%d end=%d", startPage, endPage)}
	}

	contextChunks, tokenCount, err := s.buildPageBoundedContext(notebookID, startPage, endPage)
	if err != nil {
		return map[string]interface{}{"error": err.Error()}
	}
	if len(contextChunks) == 0 {
		return map[string]interface{}{"error": "no content found in page range"}
	}

	// Create synthetic topic for manual quiz
	syntheticTopicID := fmt.Sprintf("quiz-manual-%s-p%d-%d", notebookID, startPage, endPage)
	err = s.repo.EnsureTopicsBatch([]db.TopicBatchItem{{TopicID: syntheticTopicID, Title: fmt.Sprintf("Quiz %s p%d-%d", notebookID, startPage, endPage)}})
	if err != nil {
		return map[string]interface{}{"error": fmt.Sprintf("failed to create synthetic topic: %s", err.Error())}
	}

	// Extract chunk IDs and build chunk text map from context chunks
	chunkIDs := make([]string, 0, len(contextChunks))
	chunkTextByID := make(map[string]string, len(contextChunks))
	for _, chunk := range contextChunks {
		chunkIDs = append(chunkIDs, chunk.ChunkID)
		chunkTextByID[chunk.ChunkID] = strings.TrimSpace(chunk.Text)
	}

	// Use canonical GenerateQuizSync for actual generation
	payload, err := s.GenerateQuizSync(syntheticTopicID, chunkIDs, chunkTextByID)
	if err != nil {
		return map[string]interface{}{"error": fmt.Sprintf("quiz generation failed: %s", err.Error())}
	}

	return map[string]interface{}{
		"questions":     payload.Questions,
		"passing_score": payload.PassingScore,
		"topic_id":      syntheticTopicID,
		"notebook_id":   notebookID,
		"start_page":    startPage,
		"end_page":      endPage,
		"chunk_count":   len(chunkIDs),
		"token_count":   tokenCount,
	}
}

func (s *StudyService) resolveNotebookTitle(topicID string) string {
	notebookTitle := topicID
	if nbID, err := s.repo.GetNotebookIDByTopic(topicID); err == nil && nbID != "" {
		if nb, err := s.repo.GetNotebookByID(nbID); err == nil && nb != nil && nb.Title != "" {
			notebookTitle = nb.Title
		}
	}
	return notebookTitle
}

func normalizeChunkIDs(chunkIDs []string) ([]string, error) {
	normalizedChunkIDs := make([]string, 0, len(chunkIDs))
	seen := make(map[string]struct{}, len(chunkIDs))
	for _, id := range chunkIDs {
		trimmed := strings.TrimSpace(id)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		normalizedChunkIDs = append(normalizedChunkIDs, trimmed)
	}
	if len(normalizedChunkIDs) == 0 {
		return nil, fmt.Errorf("at least one chunk ID is required")
	}

	const maxChunks = 24
	if len(normalizedChunkIDs) > maxChunks {
		normalizedChunkIDs = normalizedChunkIDs[:maxChunks]
	}
	return normalizedChunkIDs, nil
}

func (s *StudyService) loadChunkTextFallback(topicID string) (map[string]string, error) {
	chunks, err := s.repo.GetChunksForTopic(topicID)
	if err != nil {
		return nil, fmt.Errorf("failed to load topic chunks: %w", err)
	}
	chunkTextByID := make(map[string]string, len(chunks))
	for _, chunk := range chunks {
		chunkTextByID[chunk.ID] = strings.TrimSpace(chunk.Text)
	}
	return chunkTextByID, nil
}

type quizContextResult struct {
	contextParts   []string
	totalWordCount int
	currentTokens  int
	truncatedCount int
}

// isFrontMatterChunk returns true if text consists predominantly of front matter or metadata
// such as copyright notices, ISBN numbers, publisher information, or table of contents listings.
func isFrontMatterChunk(text string) bool {
	lower := strings.ToLower(text)
	if strings.Contains(lower, "licensed to ") || strings.Contains(lower, "manning publications") || strings.Contains(lower, "all rights reserved") {
		return true
	}
	if strings.Contains(lower, "isbn:") || strings.Contains(lower, "development editor:") || strings.Contains(lower, "production editor:") {
		return true
	}
	if strings.Contains(lower, "contents") && strings.Count(lower, "chapter ") >= 2 {
		return true
	}
	return false
}

func buildQuizContext(
	normalizedChunkIDs []string,
	chunkTextByID map[string]string,
	availableBudget int,
) (quizContextResult, error) {
	totalWordCount := 0
	currentTokens := 0
	contextParts := make([]string, 0, len(normalizedChunkIDs))
	truncatedCount := 0

	// Separate substantive chunks from front matter chunks
	substantiveIDs := make([]string, 0, len(normalizedChunkIDs))
	frontMatterIDs := make([]string, 0, len(normalizedChunkIDs))

	for _, chunkID := range normalizedChunkIDs {
		text := strings.TrimSpace(chunkTextByID[chunkID])
		if text == "" {
			continue
		}
		if isFrontMatterChunk(text) {
			frontMatterIDs = append(frontMatterIDs, chunkID)
		} else {
			substantiveIDs = append(substantiveIDs, chunkID)
		}
	}

	targetIDs := substantiveIDs
	if len(targetIDs) == 0 {
		targetIDs = frontMatterIDs
	}

	for _, chunkID := range targetIDs {
		text := strings.TrimSpace(chunkTextByID[chunkID])
		if text == "" {
			continue
		}
		chunkLine := fmt.Sprintf("- chunk_id: %s | text: %s\n", chunkID, text)
		chunkTokens, err := embeddings.CountTokens(chunkLine)
		if err != nil {
			chunkTokens = len(strings.Fields(chunkLine))
		}

		if currentTokens+chunkTokens > availableBudget {
			truncatedCount++
			continue
		}

		totalWordCount += len(strings.Fields(text))
		contextParts = append(contextParts, fmt.Sprintf("- chunk_id: %s | text: %s", chunkID, text))
		currentTokens += chunkTokens
	}

	if len(contextParts) == 0 {
		return quizContextResult{}, fmt.Errorf("no chunk context found for quiz generation")
	}

	return quizContextResult{
		contextParts:   contextParts,
		totalWordCount: totalWordCount,
		currentTokens:  currentTokens,
		truncatedCount: truncatedCount,
	}, nil
}

func buildQuizPrompt(notebookTitle string, targetCount int, contextParts []string) string {
	return strings.Join([]string{
		"You are an expert academic tutor and quiz generator creating a quiz for spaced repetition study.",
		"Return STRICT JSON only.",
		fmt.Sprintf("Notebook: \"%s\"", notebookTitle),
		fmt.Sprintf("Generate exactly %d multiple-choice questions grounded strictly and exclusively in the provided text chunks.", targetCount),
		"",
		"=== GROUNDING & NO-HALLUCINATION RULES ===",
		"1. EVERY question must be answerable purely from the explicit facts and concepts present in the provided Chunks.",
		"2. NEVER ask meta-questions about the book, author, preface, target audience, difficulty level, prerequisites, table of contents, or overall structure (e.g. 'Which chapter covers X?', 'What is the lowest barrier to entry according to the author?', 'What does chapter N introduce?').",
		"3. Focus solely on testing the subject matter concept, theory, technique, or formula described in the text.",
		"",
		"=== ADAPTIVE CONTENT RULES ===",
		"Before generating questions, classify the text using the notebook title and provided content:",
		"",
		"- FACTUAL: Test specific facts, dates, names, formulas, definitions, and concrete data.",
		"- CONCEPTUAL: Test core ideas, frameworks, reasoning, comparisons, principles, and cause-effect relationships.",
		"- TECHNICAL: Prioritize definitions, terminology, algorithms, architectures, APIs, workflows, constraints, trade-offs, and practical application. Avoid theme- or opinion-based questions unless they describe a technical concept.",
		"",
		"=== QUESTION RULES ===",
		"Each question must have exactly 4 options.",
		"correct_answer must match one option exactly.",
		"AVOID yes/no questions. PREFER 'why', 'how', 'what is', 'explain' questions.",
		"",
		"JSON schema: {\"questions\":[{\"prompt\":string,\"options\":[string,string,string,string],\"correct_answer\":string}]}",
		"Chunks:",
		strings.Join(contextParts, "\n"),
	}, "\n")
}

func validateAndConvertQuestions(parsed *quizLLMResponse) []models.QuizTaskQuestion {
	if parsed == nil {
		return nil
	}
	questions := make([]models.QuizTaskQuestion, 0, len(parsed.Questions))
	for _, q := range parsed.Questions {
		if strings.TrimSpace(q.Prompt) == "" || len(q.Options) != 4 || strings.TrimSpace(q.CorrectAnswer) == "" {
			continue
		}
		matchedOption, ok := ResolveCorrectOption(q.CorrectAnswer, q.Options)
		if !ok {
			continue
		}
		questions = append(questions, models.QuizTaskQuestion{
			ID:            "q_" + uuid.NewString(),
			Prompt:        strings.TrimSpace(q.Prompt),
			Options:       q.Options,
			CorrectAnswer: matchedOption,
			SourceChunkID: strings.TrimSpace(q.SourceChunkID),
		})
	}
	return questions
}

func (s *StudyService) GenerateQuizSync(topicID string, chunkIDs []string, chunkTextByID map[string]string) (models.QuizTaskPayload, error) {
	topicID = strings.TrimSpace(topicID)
	if topicID == "" {
		return models.QuizTaskPayload{}, fmt.Errorf("topic ID is required")
	}
	if s.fastLLMProvider == nil {
		return models.QuizTaskPayload{}, fmt.Errorf("FAST_LLM provider not initialized")
	}

	notebookTitle := s.resolveNotebookTitle(topicID)

	normalizedChunkIDs, err := normalizeChunkIDs(chunkIDs)
	if err != nil {
		return models.QuizTaskPayload{}, err
	}

	// ponytail: fall back to DB lookup if chunkTextByID is nil or empty
	if len(chunkTextByID) == 0 {
		chunkTextByID, err = s.loadChunkTextFallback(topicID)
		if err != nil {
			return models.QuizTaskPayload{}, err
		}
	}

	// Get model-specific token limits
	llm := s.fastLLMProvider
	modelName := providerModelName(llm)
	limits := llm.GetLimits()
	maxInputTokens := limits.MaxInputTokens
	maxOutputTokens := limits.MaxOutputTokens
	utils.Warnf("[QUIZ_PIPELINE] model_limits model=%s max_input=%d max_output=%d", modelName, maxInputTokens, maxOutputTokens)

	// Estimate token limits and budget
	const baseOverheadTokens = 300
	const safetyMarginTokens = 500
	availableBudget := maxInputTokens - baseOverheadTokens - safetyMarginTokens
	if availableBudget < 1000 {
		availableBudget = 1000
	}

	ctxRes, err := buildQuizContext(normalizedChunkIDs, chunkTextByID, availableBudget)
	if err != nil {
		return models.QuizTaskPayload{}, err
	}

	if ctxRes.truncatedCount > 0 {
		utils.Warnf("[QUIZ_PIPELINE] chunk_trimming total_chunks=%d included=%d truncated=%d budget_used=%d available=%d",
			len(normalizedChunkIDs), len(ctxRes.contextParts), ctxRes.truncatedCount, ctxRes.currentTokens, availableBudget)
	}

	targetCount := scaledQuizQuestionCount(ctxRes.totalWordCount)

	prompt := buildQuizPrompt(notebookTitle, targetCount, ctxRes.contextParts)

	raw, err := s.fastLLMProvider.GenerateAnswer(prompt)
	if err != nil {
		return models.QuizTaskPayload{}, fmt.Errorf("quiz generation failed: %w", err)
	}
	parsed, err := parseQuizLLMResponse(raw)
	if err != nil {
		return models.QuizTaskPayload{}, fmt.Errorf("quiz parsing failed: %w", err)
	}

	questions := validateAndConvertQuestions(parsed)
	if len(questions) == 0 {
		return models.QuizTaskPayload{}, fmt.Errorf("no valid questions generated")
	}

	return models.QuizTaskPayload{Questions: questions, PassingScore: 70}, nil
}

func (s *StudyService) triggerSocraticRescueHandoffTx(
	tx *sql.Tx,
	task *models.StudyQueueTask,
	attempt *models.QuizAttemptRecord,
	failedQuestions []models.FailedQuestionDetail,
) (string, string, models.StudyTaskStatus, bool, models.StudyQueueTask, error) {
	feedback := "Concept rescue activated. Complete the Socratic session to retry."
	attempt.Feedback = feedback
	completionStatus := models.StudyTaskStatusCompleted
	manualReviewRecommended := true

	// Safety transaction: Delete FSRS cards to protect purity from rote clutter
	if err := s.repo.DeleteFSRSCardsByTopicIDTx(tx, task.TopicID); err != nil {
		return "", "", models.StudyTaskStatusCompleted, false, models.StudyQueueTask{}, fmt.Errorf("failed to delete FSRS cards: %w", err)
	}

	// Shift session into Socratic Rescue Lane by generating a SOCRATIC_REMEDIAL task
	socraticTaskID := uuid.NewString()
	socraticPayload, _ := json.Marshal(map[string]interface{}{
		"feedback":         feedback,
		"lane":             "socratic_rescue",
		"mode":             "external_prompt",
		"failed_questions": failedQuestions,
	})
	followUp := models.StudyQueueTask{
		ID:          socraticTaskID,
		NotebookID:  task.NotebookID,
		TopicID:     task.TopicID,
		TaskType:    models.StudyTaskTypeSocraticRemedial,
		Status:      models.StudyTaskStatusPending,
		Priority:    0,
		PayloadJSON: string(socraticPayload),
		StartPage:   task.StartPage,
		EndPage:     task.EndPage,
	}

	return socraticTaskID, feedback, completionStatus, manualReviewRecommended, followUp, nil
}

type quizScoringResult struct {
	correctCount    int
	totalCount      int
	score           int
	passed          bool
	failedQuestions []models.FailedQuestionDetail
}

func calculateQuizScore(questions []models.QuizTaskQuestion, answers []models.QuizAnswer, passingScore int) quizScoringResult {
	selectedByQuestionID := make(map[string]string, len(answers))
	for _, answer := range answers {
		questionID := strings.TrimSpace(answer.QuestionID)
		if questionID == "" {
			continue
		}
		selectedByQuestionID[questionID] = strings.TrimSpace(answer.Selected)
	}

	correctCount := 0
	var failedQuestions []models.FailedQuestionDetail
	for _, question := range questions {
		selected := strings.TrimSpace(selectedByQuestionID[question.ID])
		if strings.EqualFold(strings.TrimSpace(question.CorrectAnswer), selected) {
			correctCount++
		} else {
			failedQuestions = append(failedQuestions, models.FailedQuestionDetail{
				Prompt:        question.Prompt,
				Options:       question.Options,
				CorrectAnswer: question.CorrectAnswer,
				UserAnswer:    selected,
			})
		}
	}

	totalCount := len(questions)
	score := 0
	if totalCount > 0 {
		score = (correctCount * 100) / totalCount
	}
	passed := score >= passingScore

	return quizScoringResult{
		correctCount:    correctCount,
		totalCount:      totalCount,
		score:           score,
		passed:          passed,
		failedQuestions: failedQuestions,
	}
}

func buildQuizResultPayload(
	taskID string,
	scoreRes quizScoringResult,
	passingScore int,
	feedback string,
	manualReviewRecommended bool,
	rereadAttemptCount int,
	rereadTaskID string,
	attemptID string,
	flashcardsPending bool,
) models.QuizResult {
	return models.QuizResult{
		TaskID:                  taskID,
		Score:                   scoreRes.score,
		Passed:                  scoreRes.passed,
		CorrectCount:            scoreRes.correctCount,
		TotalCount:              scoreRes.totalCount,
		PassingScore:            passingScore,
		Feedback:                feedback,
		ManualReviewRecommended: manualReviewRecommended,
		RereadAttemptCount:      rereadAttemptCount,
		MaxRereadAttempts:       maxAutomaticRereadAttempts,
		RereadTaskID:            rereadTaskID,
		FlashcardTaskID:         "",
		AttemptRecord:           attemptID,
		FlashcardsPending:       flashcardsPending,
	}
}

func (s *StudyService) SubmitQuizAttempt(taskID string, answers []models.QuizAnswer) (models.QuizResult, error) {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return models.QuizResult{}, fmt.Errorf("task ID is required")
	}
	task, err := s.repo.GetTaskByID(taskID)
	if err != nil {
		return models.QuizResult{}, err
	}
	if task.TaskType != models.StudyTaskTypeQuiz && task.TaskType != models.StudyTaskTypeMilestoneExam {
		return models.QuizResult{}, fmt.Errorf("task is not a QUIZ or MILESTONE_EXAM task")
	}
	if task.TaskType == models.StudyTaskTypeMilestoneExam {
		quizPayload, err := CompileMilestonePayload(s.repo, task)
		if err != nil {
			return models.QuizResult{}, err
		}
		if len(quizPayload.Questions) > 0 {
			quizPayloadJSON, mErr := json.Marshal(quizPayload)
			if mErr != nil {
				return models.QuizResult{}, fmt.Errorf("failed to build milestone exam payload: %w", mErr)
			}
			task.PayloadJSON = string(quizPayloadJSON)
		}
	}
	if task.Status != models.StudyTaskStatusActive {
		return models.QuizResult{}, db.ErrTaskNotActive
	}
	if strings.TrimSpace(task.PayloadJSON) == "" {
		return models.QuizResult{}, fmt.Errorf("quiz payload missing")
	}

	var payload models.QuizTaskPayload
	if err := json.Unmarshal([]byte(task.PayloadJSON), &payload); err != nil {
		return models.QuizResult{}, fmt.Errorf("invalid quiz payload: %w", err)
	}
	if payload.PassingScore <= 0 {
		payload.PassingScore = 70
	}
	if len(payload.Questions) == 0 {
		return models.QuizResult{}, fmt.Errorf("quiz contains no questions")
	}

	scoreRes := calculateQuizScore(payload.Questions, answers, payload.PassingScore)
	feedback := "Review the missed concepts and retry the material."
	if scoreRes.passed {
		feedback = "Strong work. You can move forward."
	}

	answersJSONBytes, err := json.Marshal(answers)
	if err != nil {
		return models.QuizResult{}, fmt.Errorf("failed to encode answers: %w", err)
	}
	attemptID := uuid.NewString()
	followUps := make([]models.StudyQueueTask, 0, 1)
	rereadTaskID := ""
	socraticTaskID := ""
	rereadAttemptCount := 0
	manualReviewRecommended := false
	completionStatus := models.StudyTaskStatusCompleted

	strategy, _ := s.repo.GetRemedialStrategy()

	tx, err := s.repo.Begin()
	if err != nil {
		return models.QuizResult{}, fmt.Errorf("failed to begin quiz transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	isRescueRequiz := false
	if task.PayloadJSON != "" {
		var payloadMap map[string]interface{}
		if err := json.Unmarshal([]byte(task.PayloadJSON), &payloadMap); err == nil {
			if source, ok := payloadMap["source"].(string); ok && source == "socratic_rescue_requiz" {
				isRescueRequiz = true
			}
		}
	}

	attempt := models.QuizAttemptRecord{
		ID:          attemptID,
		TaskID:      task.ID,
		Score:       scoreRes.score,
		Passed:      scoreRes.passed,
		AnswersJSON: string(answersJSONBytes),
		Feedback:    feedback,
		CompletedAt: time.Now().Unix(),
	}
	if scoreRes.passed {
		if task.TopicID != "" {
			if err := s.repo.ResetRereadAttemptCountTx(tx, task.TopicID); err != nil {
				return models.QuizResult{}, fmt.Errorf("failed to reset reread attempts: %w", err)
			}
			if err := s.repo.MarkTopicCompletedTx(tx, task.TopicID); err != nil {
				return models.QuizResult{}, fmt.Errorf("failed to mark topic completed: %w", err)
			}
		}
	} else if task.TopicID != "" {
		if isRescueRequiz {
			// Student failed re-quiz — mark as EXTERNAL_HELP_REQUIRED, unblock queue
			completionStatus = models.StudyTaskStatusCompleted // Still mark as completed
			manualReviewRecommended = true
			feedback = "This concept requires external review. Your next reading task has been unlocked."
			attempt.Feedback = feedback

			// Mark topic as needing external help — abort transaction on failure to prevent infinite rescue loop
			if err := s.repo.MarkTopicExternalHelpRequiredTx(tx, task.TopicID); err != nil {
				return models.QuizResult{}, fmt.Errorf("failed to mark topic as requiring external help: %w", err)
			}
			utils.Warnf("[SOCRATIC_RESCUE] requiz_failed topicID=%s — external help required", task.TopicID)
		} else {
			if strategy == "FAST" {
				var followUp models.StudyQueueTask
				socraticTaskID, feedback, completionStatus, manualReviewRecommended, followUp, err = s.triggerSocraticRescueHandoffTx(tx, task, &attempt, scoreRes.failedQuestions)
				if err != nil {
					return models.QuizResult{}, err
				}
				followUps = append(followUps, followUp)
			} else {
				rereadAttemptCount, err = s.repo.IncrementRereadAttemptCountTx(tx, task.TopicID)
				if err != nil {
					return models.QuizResult{}, fmt.Errorf("failed to increment reread attempts: %w", err)
				}
				if rereadAttemptCount <= maxAutomaticRereadAttempts {
					rereadTaskID = uuid.NewString()
					feedbackPayload, _ := json.Marshal(map[string]string{"feedback": feedback})
					followUps = append(followUps, models.StudyQueueTask{
						ID:          rereadTaskID,
						NotebookID:  task.NotebookID,
						TopicID:     task.TopicID,
						TaskType:    models.StudyTaskTypeReread,
						Status:      models.StudyTaskStatusPending,
						Priority:    0,
						PayloadJSON: string(feedbackPayload),
						StartPage:   task.StartPage,
						EndPage:     task.EndPage,
					})
				} else {
					// Strike 3: SOCRATIC_REMEDIAL rescue
					var followUp models.StudyQueueTask
					socraticTaskID, feedback, completionStatus, manualReviewRecommended, followUp, err = s.triggerSocraticRescueHandoffTx(tx, task, &attempt, scoreRes.failedQuestions)
					if err != nil {
						return models.QuizResult{}, err
					}
					followUps = append(followUps, followUp)
				}
			}
		}
	}

	if err := s.repo.SaveQuizAttemptTx(tx, attempt); err != nil {
		return models.QuizResult{}, fmt.Errorf("failed to save quiz attempt: %w", err)
	}

	if err := s.repo.CompleteTaskTx(tx, task.ID, models.CompletionResult{
		Status:    completionStatus,
		Payload:   "", // ponytail: preserve original questions payload in study_queue for milestone exams
		FollowUps: followUps,
	}); err != nil {
		return models.QuizResult{}, err
	}
	// Flashcards are generated AFTER user clicks Continue (not during quiz submission)
	// This prevents blocking the UI on "Scoring..." during flashcard generation
	flashcardsPending := scoreRes.passed && task.TopicID != ""

	if err := tx.Commit(); err != nil {
		return models.QuizResult{}, fmt.Errorf("failed to commit quiz transaction: %w", err)
	}

	// Log quiz scoring completed immediately
	utils.Warnf("[QUIZ] quiz_scoring_completed taskID=%s score=%d passed=%t rereadTaskID=%s flashcardsPending=%t", task.ID, scoreRes.score, scoreRes.passed, rereadTaskID, flashcardsPending)

	// Log after successful commit to ensure consistency with persisted state
	if scoreRes.passed {
		utils.LogQuizResult(task.ID, scoreRes.score, true, "")
	} else if task.TopicID != "" {
		if isRescueRequiz {
			utils.LogQuizResult(task.ID, scoreRes.score, false, "")
			utils.Warnf("[QUIZ] quiz_failed_requiz_failed notebookID=%s topicID=%s — external help marked", task.NotebookID, task.TopicID)
		} else if socraticTaskID != "" {
			utils.LogQuizResult(task.ID, scoreRes.score, false, "")
			utils.Warnf("[QUIZ] quiz_failed_socratic_rescue_created notebookID=%s topicID=%s socraticTaskID=%s", task.NotebookID, task.TopicID, socraticTaskID)
		} else if rereadTaskID != "" {
			utils.LogRereadInsertion(rereadTaskID, task.TopicID, strconv.Itoa(rereadAttemptCount), strconv.Itoa(maxAutomaticRereadAttempts))
			utils.LogQuizResult(task.ID, scoreRes.score, false, rereadTaskID)
			utils.Warnf("[QUIZ] quiz_failed_reread_created notebookID=%s topicID=%s rereadTaskID=%s", task.NotebookID, task.TopicID, rereadTaskID)
		} else {
			utils.LogQuizResult(task.ID, scoreRes.score, false, "")
		}
	}

	return buildQuizResultPayload(
		task.ID,
		scoreRes,
		payload.PassingScore,
		feedback,
		manualReviewRecommended,
		rereadAttemptCount,
		rereadTaskID,
		attemptID,
		flashcardsPending,
	), nil
}

