package db

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"ai-tutor/internal/models"

	"github.com/google/uuid"
)


func createWrittenQuestionRepo(question models.WrittenQuestion) error {
	var sourceChunkID interface{}
	if strings.TrimSpace(question.SourceChunkID) == "" {
		sourceChunkID = nil
	} else {
		sourceChunkID = strings.TrimSpace(question.SourceChunkID)
	}
	_, err := conn.Exec(`
		INSERT INTO written_questions (
			id, topic_id, prompt, source_chunk_id, source_heading, source_page_start, source_page_end,
			llm_model, prompt_version, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
	`, question.ID, question.TopicID, question.Prompt, sourceChunkID, question.SourceHeading, question.SourcePageStart,
		question.SourcePageEnd, question.LLMModel, question.PromptVersion)
	return err
}

func getWrittenQuestionByIDRepo(questionID string) (*models.WrittenQuestion, error) {
	var question models.WrittenQuestion
	err := conn.QueryRow(`
		SELECT id, topic_id, prompt, COALESCE(source_chunk_id, ''), COALESCE(source_heading, ''), COALESCE(source_page_start, 0),
			COALESCE(source_page_end, 0), COALESCE(llm_model, ''), COALESCE(prompt_version, '')
		FROM written_questions
		WHERE id = ?
	`, questionID).Scan(&question.ID, &question.TopicID, &question.Prompt, &question.SourceChunkID, &question.SourceHeading,
		&question.SourcePageStart, &question.SourcePageEnd, &question.LLMModel, &question.PromptVersion)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &question, nil
}

func saveWrittenAnswerRepoTx(tx *sql.Tx, answer models.WrittenAnswer) error {
	if tx == nil {
		return errors.New("transaction not initialized")
	}
	_, err := tx.Exec(`
		INSERT INTO written_user_answers (id, written_question_id, user_answer, score, feedback, source_heading)
		VALUES (?, ?, ?, ?, ?, ?)
	`,
		uuid.NewString(),
		answer.QuestionID,
		answer.UserAnswer,
		answer.Score,
		answer.Feedback,
		answer.SourceHeading,
	)
	return err
}

// SaveWrittenAnswerTx stores a scored written response within a transaction.
func SaveWrittenAnswerTx(tx *sql.Tx, answer models.WrittenAnswer) error {
	answer.QuestionID = strings.TrimSpace(answer.QuestionID)
	if answer.QuestionID == "" {
		return fmt.Errorf("question id is required")
	}
	// Validate UserAnswer without mutating original free-text input
	trimmedAnswer := strings.TrimSpace(answer.UserAnswer)
	if trimmedAnswer == "" {
		return fmt.Errorf("user answer is required")
	}
	if tx == nil {
		return errors.New("transaction not initialized")
	}
	return saveWrittenAnswerRepoTx(tx, answer)
}
