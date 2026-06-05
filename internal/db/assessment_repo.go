package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"ai-tutor/internal/models"

	"github.com/google/uuid"
)

// AssessmentFSRSRecord represents the shared FSRS state for an assessment
type AssessmentFSRSRecord struct {
	TopicID        string
	SourceChunkID  string
	State          models.FlashcardState
	DueAt          int64
	LastReviewedAt int64
}

func (r *AssessmentFSRSRecord) GetTopicID() string {
	if r == nil {
		return ""
	}
	return r.TopicID
}

func (r *AssessmentFSRSRecord) GetState() models.FlashcardState {
	if r == nil {
		return models.FlashcardState{}
	}
	return r.State
}

func (r *AssessmentFSRSRecord) GetSourceChunkID() string {
	if r == nil {
		return ""
	}
	return r.SourceChunkID
}

func (r *AssessmentFSRSRecord) GetDueAt() int64 {
	if r == nil {
		return 0
	}
	return r.DueAt
}

func (r *AssessmentFSRSRecord) GetLastReviewedAt() int64 {
	if r == nil {
		return 0
	}
	return r.LastReviewedAt
}

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

// querier interface allows both *sql.DB and *sql.Tx to be used with the helper function
type querier interface {
	QueryRow(query string, args ...any) *sql.Row
}

// getAssessmentFSRSStateFromQuerier extracts FSRS state using any querier (DB or transaction)
func getAssessmentFSRSStateFromQuerier(q querier, activityType, referenceID, sourceChunkID string) (*AssessmentFSRSRecord, error) {
	var topicID string
	var storedSourceChunkID sql.NullString
	var stateJSON string
	var dueAt sql.NullInt64
	var lastReviewedAt sql.NullInt64
	err := q.QueryRow(`
		SELECT topic_id, source_chunk_id, state_json, due_at, last_reviewed_at
		FROM assessment_fsrs
		WHERE activity_type = ? AND reference_id = ? AND COALESCE(source_chunk_id, '') = ?
	`, activityType, referenceID, strings.TrimSpace(sourceChunkID)).Scan(&topicID, &storedSourceChunkID, &stateJSON, &dueAt, &lastReviewedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	var state models.FlashcardState
	if strings.TrimSpace(stateJSON) != "" {
		if err := json.Unmarshal([]byte(stateJSON), &state); err != nil {
			return nil, fmt.Errorf("decode assessment fsrs state: %w", err)
		}
	}

	record := &AssessmentFSRSRecord{
		TopicID:        topicID,
		SourceChunkID:  strings.TrimSpace(storedSourceChunkID.String),
		State:          state,
		DueAt:          dueAt.Int64,
		LastReviewedAt: lastReviewedAt.Int64,
	}
	return record, nil
}

func getAssessmentFSRSStateRepo(activityType, referenceID, sourceChunkID string) (*AssessmentFSRSRecord, error) {
	return getAssessmentFSRSStateFromQuerier(conn, activityType, referenceID, sourceChunkID)
}

func upsertAssessmentFSRSReviewRepo(activityType, referenceID, topicID, sourceChunkID string, state models.FlashcardState, dueAt, reviewedAt int64, reviewLog models.FSRSReviewLog) error {
	stateJSON, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("encode assessment fsrs state: %w", err)
	}

	return withTx(func(tx *sql.Tx) error {
		var existingID string
		if err = tx.QueryRow(`SELECT id FROM topics WHERE id = ?`, topicID).Scan(&existingID); err != nil {
			if err == sql.ErrNoRows {
				return fmt.Errorf("topic not found: %s", topicID)
			}
			return err
		}

		normalizedSourceChunkID := strings.TrimSpace(sourceChunkID)
		var sourceChunkIDValue interface{}
		if normalizedSourceChunkID == "" {
			sourceChunkIDValue = nil // Use NULL for empty strings
		} else {
			sourceChunkIDValue = normalizedSourceChunkID
		}

		if _, err = tx.Exec(`
			INSERT INTO assessment_fsrs (
				activity_type, reference_id, topic_id, source_chunk_id, state_json, due_at, last_reviewed_at, updated_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
			ON CONFLICT(activity_type, reference_id, source_chunk_id) DO UPDATE SET
				topic_id = excluded.topic_id,
				source_chunk_id = excluded.source_chunk_id,
				state_json = excluded.state_json,
				due_at = excluded.due_at,
				last_reviewed_at = excluded.last_reviewed_at,
				updated_at = CURRENT_TIMESTAMP
		`, activityType, referenceID, topicID, sourceChunkIDValue, string(stateJSON), dueAt, reviewedAt); err != nil {
			return err
		}

		if _, err = tx.Exec(`
			INSERT INTO fsrs_review_log (
				id, topic_id, activity_type, reference_id, reviewed_at, rating,
				scheduled_days, state_before_json, state_after_json
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, reviewLog.ID, topicID, reviewLog.ActivityType, reviewLog.ReferenceID, reviewLog.ReviewedAt,
			reviewLog.Rating, reviewLog.ScheduledDays, reviewLog.StateBeforeJSON, reviewLog.StateAfterJSON); err != nil {
			return err
		}

		return nil
	})
}

func getAssessmentFSRSStateRepoTx(tx *sql.Tx, activityType, referenceID, sourceChunkID string) (*AssessmentFSRSRecord, error) {
	if tx == nil {
		return nil, fmt.Errorf("transaction not initialized")
	}
	return getAssessmentFSRSStateFromQuerier(tx, activityType, referenceID, sourceChunkID)
}

func upsertAssessmentFSRSReviewRepoTx(tx *sql.Tx, activityType, referenceID, topicID, sourceChunkID string, state models.FlashcardState, dueAt, reviewedAt int64, reviewLog models.FSRSReviewLog) error {
	if tx == nil {
		return fmt.Errorf("transaction not initialized")
	}
	stateJSON, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("encode assessment fsrs state: %w", err)
	}

	var existingID string
	if err = tx.QueryRow(`SELECT id FROM topics WHERE id = ?`, topicID).Scan(&existingID); err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("topic not found: %s", topicID)
		}
		return err
	}

	normalizedSourceChunkID := strings.TrimSpace(sourceChunkID)
	var sourceChunkIDValue interface{}
	if normalizedSourceChunkID == "" {
		sourceChunkIDValue = nil // Use NULL for empty strings
	} else {
		sourceChunkIDValue = normalizedSourceChunkID
	}

	if _, err = tx.Exec(`
		INSERT INTO assessment_fsrs (
			activity_type, reference_id, topic_id, source_chunk_id, state_json, due_at, last_reviewed_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(activity_type, reference_id, source_chunk_id) DO UPDATE SET
			topic_id = excluded.topic_id,
			source_chunk_id = excluded.source_chunk_id,
			state_json = excluded.state_json,
			due_at = excluded.due_at,
			last_reviewed_at = excluded.last_reviewed_at,
			updated_at = CURRENT_TIMESTAMP
	`, activityType, referenceID, topicID, sourceChunkIDValue, string(stateJSON), dueAt, reviewedAt); err != nil {
		return err
	}

	if _, err = tx.Exec(`
		INSERT INTO fsrs_review_log (
			id, topic_id, activity_type, reference_id, reviewed_at, rating,
			scheduled_days, state_before_json, state_after_json
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, reviewLog.ID, topicID, reviewLog.ActivityType, reviewLog.ReferenceID, reviewLog.ReviewedAt,
		reviewLog.Rating, reviewLog.ScheduledDays, reviewLog.StateBeforeJSON, reviewLog.StateAfterJSON); err != nil {
		return err
	}

	return nil
}

func saveWrittenAnswerRepoTx(tx *sql.Tx, answer models.WrittenAnswer) error {
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

func saveWrittenAnswerRepo(answer models.WrittenAnswer) error {
	_, err := conn.Exec(`
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

// SaveWrittenAnswer stores a scored written response.
func SaveWrittenAnswer(answer models.WrittenAnswer) error {
	answer.QuestionID = strings.TrimSpace(answer.QuestionID)
	if answer.QuestionID == "" {
		return fmt.Errorf("question id is required")
	}
	// Validate UserAnswer without mutating original free-text input
	trimmedAnswer := strings.TrimSpace(answer.UserAnswer)
	if trimmedAnswer == "" {
		return fmt.Errorf("user answer is required")
	}
	return saveWrittenAnswerRepo(answer)
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
	return saveWrittenAnswerRepoTx(tx, answer)
}
