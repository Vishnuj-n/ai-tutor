package study

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"ai-tutor/internal/db"
	"ai-tutor/internal/models"

	"github.com/google/uuid"
)

const maxAutomaticRereadAttempts = 3

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

	contextChunks, tokenCount, err := buildPageBoundedContext(notebookID, startPage, endPage)
	if err != nil {
		return map[string]interface{}{"error": err.Error()}
	}
	if len(contextChunks) == 0 {
		return map[string]interface{}{"error": "no content found in page range"}
	}

	// Create synthetic topic for manual quiz
	syntheticTopicID := fmt.Sprintf("quiz-manual-%s-p%d-%d", notebookID, startPage, endPage)
	err = db.EnsureTopicsBatch([]db.TopicBatchItem{{TopicID: syntheticTopicID, Title: fmt.Sprintf("Quiz %s p%d-%d", notebookID, startPage, endPage)}})
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

func (s *StudyService) GenerateQuizSync(topicID string, chunkIDs []string, chunkTextByID map[string]string) (models.QuizTaskPayload, error) {
	topicID = strings.TrimSpace(topicID)
	if topicID == "" {
		return models.QuizTaskPayload{}, fmt.Errorf("topic ID is required")
	}
	if s.fastLLMProvider == nil {
		return models.QuizTaskPayload{}, fmt.Errorf("FAST_LLM provider not initialized")
	}

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
		return models.QuizTaskPayload{}, fmt.Errorf("at least one chunk ID is required")
	}

	const maxChunks = 24
	if len(normalizedChunkIDs) > maxChunks {
		normalizedChunkIDs = normalizedChunkIDs[:maxChunks]
	}

	// If chunkTextByID is not provided, fall back to database lookup
	if chunkTextByID == nil {
		chunks, err := db.GetChunksForTopic(topicID)
		if err != nil {
			return models.QuizTaskPayload{}, fmt.Errorf("failed to load topic chunks: %w", err)
		}
		chunkTextByID = make(map[string]string, len(chunks))
		for _, chunk := range chunks {
			chunkTextByID[chunk.ID] = strings.TrimSpace(chunk.Text)
		}
	}

	contextParts := make([]string, 0, len(normalizedChunkIDs))
	for _, chunkID := range normalizedChunkIDs {
		text := strings.TrimSpace(chunkTextByID[chunkID])
		if text == "" {
			continue
		}
		contextParts = append(contextParts, fmt.Sprintf("- chunk_id: %s | text: %s", chunkID, text))
	}
	if len(contextParts) == 0 {
		return models.QuizTaskPayload{}, fmt.Errorf("no chunk context found for quiz generation")
	}

	prompt := strings.Join([]string{
		"You are an AI tutor quiz generator.",
		"Return STRICT JSON only.",
		"Generate exactly 5 multiple-choice questions from the provided chunks.",
		"Each question must have exactly 4 options.",
		"correct_answer must match one option exactly.",
		"JSON schema: {\"questions\":[{\"source_chunk_id\":string,\"prompt\":string,\"options\":[string,string,string,string],\"correct_answer\":string}]}",
		"Chunks:",
		strings.Join(contextParts, "\n"),
	}, "\n")

	raw, err := s.fastLLMProvider.GenerateAnswer(prompt)
	if err != nil {
		return models.QuizTaskPayload{}, fmt.Errorf("quiz generation failed: %w", err)
	}
	parsed, err := parseQuizLLMResponse(raw)
	if err != nil {
		return models.QuizTaskPayload{}, fmt.Errorf("quiz parsing failed: %w", err)
	}

	questions := make([]models.QuizTaskQuestion, 0, len(parsed.Questions))
	for _, q := range parsed.Questions {
		if strings.TrimSpace(q.Prompt) == "" || len(q.Options) != 4 || strings.TrimSpace(q.CorrectAnswer) == "" {
			continue
		}
		matchedOption, ok := resolveCorrectOption(q.CorrectAnswer, q.Options)
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
	if len(questions) == 0 {
		return models.QuizTaskPayload{}, fmt.Errorf("no valid questions generated")
	}

	return models.QuizTaskPayload{Questions: questions, PassingScore: 70}, nil
}

func (s *StudyService) SubmitQuizAttempt(taskID string, answers []models.QuizAnswer) (models.QuizResult, error) {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return models.QuizResult{}, fmt.Errorf("task ID is required")
	}
	task, err := db.GetTaskByID(taskID)
	if err != nil {
		return models.QuizResult{}, err
	}
	if task.TaskType != models.StudyTaskTypeQuiz {
		return models.QuizResult{}, fmt.Errorf("task is not a QUIZ task")
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

	selectedByQuestionID := make(map[string]string, len(answers))
	for _, answer := range answers {
		questionID := strings.TrimSpace(answer.QuestionID)
		if questionID == "" {
			continue
		}
		selectedByQuestionID[questionID] = strings.TrimSpace(answer.Selected)
	}

	correctCount := 0
	for _, question := range payload.Questions {
		selected := strings.TrimSpace(selectedByQuestionID[question.ID])
		if selected == "" {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(question.CorrectAnswer), selected) {
			correctCount++
		}
	}

	totalCount := len(payload.Questions)
	score := 0
	if totalCount > 0 {
		score = int(float64(correctCount) / float64(totalCount) * 100)
	}
	passed := score >= payload.PassingScore
	feedback := "Review the missed concepts and retry the material."
	if passed {
		feedback = "Strong work. You can move forward."
	}

	answersJSONBytes, err := json.Marshal(answers)
	if err != nil {
		return models.QuizResult{}, fmt.Errorf("failed to encode answers: %w", err)
	}
	attemptID := uuid.NewString()
	followUps := make([]models.StudyQueueTask, 0, 1)
	rereadTaskID := ""
	rereadAttemptCount := 0
	manualReviewRecommended := false
	var resultPayload []byte

	conn := db.GetConnection()
	if conn == nil {
		return models.QuizResult{}, fmt.Errorf("database not initialized")
	}
	tx, err := conn.Begin()
	if err != nil {
		return models.QuizResult{}, fmt.Errorf("failed to begin quiz transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	attempt := models.QuizAttemptRecord{
		ID:          attemptID,
		TaskID:      task.ID,
		Score:       score,
		Passed:      passed,
		AnswersJSON: string(answersJSONBytes),
		Feedback:    feedback,
		CompletedAt: time.Now().Unix(),
	}
	if passed {
		if task.TopicID != "" {
			if err := db.ResetRereadAttemptCountTx(tx, task.TopicID); err != nil {
				return models.QuizResult{}, fmt.Errorf("failed to reset reread attempts: %w", err)
			}
		}
	} else if task.TopicID != "" {
		rereadAttemptCount, err = db.IncrementRereadAttemptCountTx(tx, task.TopicID)
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
			manualReviewRecommended = true
			feedback = "Automatic reread limit reached. Review this topic manually, then return when ready to retry."
			attempt.Feedback = feedback
		}
	}

	if err := db.SaveQuizAttemptTx(tx, attempt); err != nil {
		return models.QuizResult{}, fmt.Errorf("failed to save quiz attempt: %w", err)
	}

	resultPayload, _ = json.Marshal(map[string]interface{}{
		"score":                     score,
		"passed":                    passed,
		"correct_count":             correctCount,
		"total_count":               totalCount,
		"manual_review_recommended": manualReviewRecommended,
		"reread_attempt_count":      rereadAttemptCount,
		"max_reread_attempts":       maxAutomaticRereadAttempts,
	})

	if err := db.CompleteTaskTx(tx, task.ID, models.CompletionResult{
		Status:    models.StudyTaskStatusCompleted,
		Payload:   string(resultPayload),
		FollowUps: followUps,
	}); err != nil {
		return models.QuizResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return models.QuizResult{}, fmt.Errorf("failed to commit quiz transaction: %w", err)
	}

	return models.QuizResult{
		TaskID:                  task.ID,
		Score:                   score,
		Passed:                  passed,
		CorrectCount:            correctCount,
		TotalCount:              totalCount,
		PassingScore:            payload.PassingScore,
		Feedback:                feedback,
		ManualReviewRecommended: manualReviewRecommended,
		RereadAttemptCount:      rereadAttemptCount,
		MaxRereadAttempts:       maxAutomaticRereadAttempts,
		RereadTaskID:            rereadTaskID,
		AttemptRecord:           attemptID,
	}, nil
}
