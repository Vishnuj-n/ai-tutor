package db

import (
	"database/sql"
	"encoding/json"
	"fmt"

	"ai-tutor/internal/models"

	"github.com/google/uuid"
)

func replaceQuestionsForTopicRepo(topicID string, questions []models.QuizQuestion) error {
	tx, err := conn.Begin()
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	// Delete dependent user answers first to maintain foreign key integrity
	if _, err = tx.Exec(`
		DELETE FROM user_answers
		WHERE question_id IN (SELECT id FROM questions WHERE topic_id = ?)
	`, topicID); err != nil {
		return err
	}

	// Now safe to delete the questions
	if _, err = tx.Exec(`DELETE FROM questions WHERE topic_id = ?`, topicID); err != nil {
		return err
	}

	for _, q := range questions {
		optionsJSON, marshalErr := json.Marshal(q.Options)
		if marshalErr != nil {
			err = fmt.Errorf("failed to encode options for question %s: %w", q.ID, marshalErr)
			return err
		}

		if _, err = tx.Exec(`
			INSERT INTO questions (
				id, topic_id, prompt, options_json, correct_answer, explanation, hint, source_heading, source_snippet,
				source_page_start, source_page_end, llm_model, prompt_version
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, q.ID, topicID, q.Prompt, string(optionsJSON), q.CorrectAnswer, q.Explanation, q.Hint, q.SourceHeading, q.SourceSnippet,
			q.SourcePageStart, q.SourcePageEnd, q.LLMModel, q.PromptVersion); err != nil {
			return err
		}
	}

	err = tx.Commit()
	return err
}

func getQuestionsForTopicRepo(topicID string) ([]models.QuizQuestion, error) {
	rows, err := conn.Query(`
		SELECT id, topic_id, prompt, options_json, correct_answer, explanation, hint, source_heading, source_snippet,
			source_page_start, source_page_end, llm_model, prompt_version
		FROM questions
		WHERE topic_id = ?
		ORDER BY created_at, id
	`, topicID)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = rows.Close()
	}()

	questions := make([]models.QuizQuestion, 0)
	for rows.Next() {
		var q models.QuizQuestion
		var optionsJSON string
		if err := rows.Scan(
			&q.ID,
			&q.TopicID,
			&q.Prompt,
			&optionsJSON,
			&q.CorrectAnswer,
			&q.Explanation,
			&q.Hint,
			&q.SourceHeading,
			&q.SourceSnippet,
			&q.SourcePageStart,
			&q.SourcePageEnd,
			&q.LLMModel,
			&q.PromptVersion,
		); err != nil {
			return nil, err
		}

		if unmarshalErr := json.Unmarshal([]byte(optionsJSON), &q.Options); unmarshalErr != nil {
			return nil, fmt.Errorf("failed to decode question options for %s: %w", q.ID, unmarshalErr)
		}

		questions = append(questions, q)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return questions, nil
}

func appendQuestionsForTopicRepo(topicID string, questions []models.QuizQuestion) error {
	tx, err := conn.Begin()
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	for _, q := range questions {
		optionsJSON, marshalErr := json.Marshal(q.Options)
		if marshalErr != nil {
			err = fmt.Errorf("failed to encode options for question %s: %w", q.ID, marshalErr)
			return err
		}

		if _, err = tx.Exec(`
			INSERT INTO questions (
				id, topic_id, prompt, options_json, correct_answer, explanation, hint, source_heading, source_snippet,
				source_page_start, source_page_end, llm_model, prompt_version
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, q.ID, topicID, q.Prompt, string(optionsJSON), q.CorrectAnswer, q.Explanation, q.Hint, q.SourceHeading, q.SourceSnippet,
			q.SourcePageStart, q.SourcePageEnd, q.LLMModel, q.PromptVersion); err != nil {
			return err
		}
	}

	err = tx.Commit()
	return err
}

func getQuestionByIDRepo(questionID string) (*models.QuizQuestion, error) {
	var q models.QuizQuestion
	var optionsJSON string
	err := conn.QueryRow(`
		SELECT id, topic_id, prompt, options_json, correct_answer, explanation, hint, source_heading, source_snippet,
			source_page_start, source_page_end, llm_model, prompt_version
		FROM questions
		WHERE id = ?
	`, questionID).Scan(
		&q.ID,
		&q.TopicID,
		&q.Prompt,
		&optionsJSON,
		&q.CorrectAnswer,
		&q.Explanation,
		&q.Hint,
		&q.SourceHeading,
		&q.SourceSnippet,
		&q.SourcePageStart,
		&q.SourcePageEnd,
		&q.LLMModel,
		&q.PromptVersion,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	if unmarshalErr := json.Unmarshal([]byte(optionsJSON), &q.Options); unmarshalErr != nil {
		return nil, fmt.Errorf("failed to decode question options for %s: %w", q.ID, unmarshalErr)
	}

	return &q, nil
}

func saveUserAnswerRepo(score models.QuizScore) error {
	_, err := conn.Exec(`
		INSERT INTO user_answers (id, question_id, user_answer, is_correct, score, feedback, hint)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`,
		uuid.NewString(),
		score.QuestionID,
		score.UserAnswer,
		boolToInt(score.Correct),
		score.Score,
		score.Feedback,
		score.Hint,
	)
	return err
}

func boolToInt(v bool) int {
	if v {
		return 1
	}
	return 0
}
