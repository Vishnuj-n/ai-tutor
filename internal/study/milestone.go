package study

import (
	"encoding/json"
	"fmt"
	"strings"

	"ai-tutor/internal/db"
	"ai-tutor/internal/models"
)

// ComputeCorrectnessFlags compares submitted answers to quiz questions and returns 1/0 correctness flags.
func ComputeCorrectnessFlags(quizPayloadJSON, answersJSON string) ([]int, error) {
	quizPayloadJSON = strings.TrimSpace(quizPayloadJSON)
	answersJSON = strings.TrimSpace(answersJSON)
	if quizPayloadJSON == "" || answersJSON == "" {
		return nil, nil
	}

	var payload models.QuizTaskPayload
	if err := json.Unmarshal([]byte(quizPayloadJSON), &payload); err != nil {
		return nil, fmt.Errorf("failed to unmarshal quiz payload: %w", err)
	}

	var answers []models.QuizAnswer
	if err := json.Unmarshal([]byte(answersJSON), &answers); err != nil {
		return nil, fmt.Errorf("failed to unmarshal answers: %w", err)
	}

	selectedByQuestionID := make(map[string]string, len(answers))
	for _, answer := range answers {
		questionID := strings.TrimSpace(answer.QuestionID)
		if questionID == "" {
			continue
		}
		selectedByQuestionID[questionID] = strings.TrimSpace(answer.Selected)
	}

	flags := make([]int, 0, len(payload.Questions))
	for _, question := range payload.Questions {
		selected := strings.TrimSpace(selectedByQuestionID[question.ID])
		if strings.EqualFold(strings.TrimSpace(question.CorrectAnswer), selected) {
			flags = append(flags, 1)
			continue
		}
		flags = append(flags, 0)
	}

	return flags, nil
}

// CompileMilestonePayload compiles milestone exam questions into a QuizTaskPayload.
func CompileMilestonePayload(repo *db.Repository, task *models.StudyQueueTask) (models.QuizTaskPayload, error) {
	var milestonePayload models.MilestoneExamPayload
	if err := json.Unmarshal([]byte(task.PayloadJSON), &milestonePayload); err != nil {
		return models.QuizTaskPayload{}, fmt.Errorf("failed to parse milestone exam payload: %w", err)
	}
	if len(milestonePayload.Quizzes) == 0 {
		return models.QuizTaskPayload{}, nil
	}
	attemptIDs := make([]string, 0, len(milestonePayload.Quizzes))
	for id := range milestonePayload.Quizzes {
		attemptIDs = append(attemptIDs, id)
	}
	questions, qErr := repo.GetQuestionsForQuizAttempts(attemptIDs)
	if qErr != nil {
		return models.QuizTaskPayload{}, fmt.Errorf("failed to retrieve questions for milestone exam: %w", qErr)
	}
	quizPayload := models.QuizTaskPayload{
		Questions:    questions,
		PassingScore: milestonePayload.PassingScore,
	}
	if quizPayload.PassingScore <= 0 {
		quizPayload.PassingScore = 70
	}
	return quizPayload, nil
}
