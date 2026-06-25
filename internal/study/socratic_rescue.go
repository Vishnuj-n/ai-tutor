package study

import (
	"encoding/json"
	"fmt"

	"ai-tutor/internal/models"
	"ai-tutor/internal/utils"

	"github.com/google/uuid"
)

// CompleteSocraticRescue handles the completion of a SOCRATIC_REMEDIAL task.
// It generates and inserts a fresh QUIZ task for the same topic so the student can prove mastery.
func (s *StudyService) CompleteSocraticRescue(taskID string) (string, error) {
	// Load the SOCRATIC_REMEDIAL task
	task, err := s.repo.GetTaskByID(taskID)
	if err != nil {
		return "", fmt.Errorf("failed to load socratic task: %w", err)
	}
	if task.TaskType != models.StudyTaskTypeSocraticRemedial {
		return "", fmt.Errorf("task %s is not SOCRATIC_REMEDIAL", taskID)
	}
	if task.Status != models.StudyTaskStatusActive {
		return "", fmt.Errorf("task %s is not ACTIVE (status=%s)", taskID, task.Status)
	}

	// Load chunks for the topic page range of this task
	chunks, err := s.repo.GetChunksForTopicPageRange(task.TopicID, task.StartPage, task.EndPage)
	if err != nil {
		return "", fmt.Errorf("failed to load chunks for socratic task: %w", err)
	}

	chunkIDs := make([]string, 0, len(chunks))
	chunkTextByID := make(map[string]string, len(chunks))
	for _, chunk := range chunks {
		chunkIDs = append(chunkIDs, chunk.ID)
		chunkTextByID[chunk.ID] = chunk.Text
	}

	// Generate the quiz questions synchronously
	generatedQuiz, err := s.GenerateQuizSync(task.TopicID, chunkIDs, chunkTextByID)
	if err != nil {
		return "", fmt.Errorf("failed to generate quiz: %w", err)
	}

	// Start a transaction
	tx, err := s.repo.Begin()
	if err != nil {
		return "", fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	// Generate a fresh QUIZ for the same topic with the generated questions
	quizTaskID := uuid.NewString()
	quizPayload, _ := json.Marshal(map[string]interface{}{
		"source":        "socratic_rescue_requiz",
		"topic_id":      task.TopicID,
		"questions":     generatedQuiz.Questions,
		"passing_score": generatedQuiz.PassingScore,
	})

	followUps := []models.StudyQueueTask{
		{
			ID:          quizTaskID,
			NotebookID:  task.NotebookID,
			TopicID:     task.TopicID,
			TaskType:    models.StudyTaskTypeQuiz,
			Status:      models.StudyTaskStatusPending,
			Priority:    0,
			PayloadJSON: string(quizPayload),
			StartPage:   task.StartPage,
			EndPage:     task.EndPage,
		},
	}

	// Complete the SOCRATIC_REMEDIAL task and insert the follow-up re-quiz
	if err := s.repo.CompleteTaskTx(tx, taskID, models.CompletionResult{
		Status:    models.StudyTaskStatusCompleted,
		FollowUps: followUps,
	}); err != nil {
		return "", fmt.Errorf("failed to complete socratic task: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return "", fmt.Errorf("failed to commit socratic rescue completion: %w", err)
	}

	utils.Warnf("[SOCRATIC_RESCUE] rescue_completed taskID=%s topicID=%s requizTaskID=%s", taskID, task.TopicID, quizTaskID)
	return quizTaskID, nil
}
