package study

import (
	"encoding/json"
	"strings"

	"ai-tutor/internal/models"
)

// ComputeCorrectnessFlags compares submitted answers to quiz questions and returns 1/0 correctness flags.
func ComputeCorrectnessFlags(quizPayloadJSON, answersJSON string) []int {
	quizPayloadJSON = strings.TrimSpace(quizPayloadJSON)
	answersJSON = strings.TrimSpace(answersJSON)
	if quizPayloadJSON == "" || answersJSON == "" {
		return nil
	}

	var payload models.QuizTaskPayload
	if err := json.Unmarshal([]byte(quizPayloadJSON), &payload); err != nil {
		return nil
	}

	var answers []models.QuizAnswer
	if err := json.Unmarshal([]byte(answersJSON), &answers); err != nil {
		return nil
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

	return flags
}
