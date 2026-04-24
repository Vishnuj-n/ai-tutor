package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"ai-tutor/internal/models"
)

type assessmentFSRSRecord struct {
	TopicID        string
	State          models.FlashcardState
	DueAt          int64
	LastReviewedAt int64
}

func createWrittenQuestionRepo(question models.WrittenQuestion) error {
	if conn == nil {
		return fmt.Errorf("database not initialized")
	}
	_, err := conn.Exec(`
		INSERT INTO written_questions (
			id, topic_id, prompt, source_heading, source_page_start, source_page_end,
			llm_model, prompt_version, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
	`, question.ID, question.TopicID, question.Prompt, question.SourceHeading, question.SourcePageStart,
		question.SourcePageEnd, question.LLMModel, question.PromptVersion)
	return err
}

func getWrittenQuestionByIDRepo(questionID string) (*models.WrittenQuestion, error) {
	if conn == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	var question models.WrittenQuestion
	err := conn.QueryRow(`
		SELECT id, topic_id, prompt, COALESCE(source_heading, ''), COALESCE(source_page_start, 0),
			COALESCE(source_page_end, 0), COALESCE(llm_model, ''), COALESCE(prompt_version, '')
		FROM written_questions
		WHERE id = ?
	`, questionID).Scan(&question.ID, &question.TopicID, &question.Prompt, &question.SourceHeading,
		&question.SourcePageStart, &question.SourcePageEnd, &question.LLMModel, &question.PromptVersion)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &question, nil
}

func getAssessmentFSRSStateRepo(activityType, referenceID string) (*assessmentFSRSRecord, error) {
	if conn == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	var topicID string
	var stateJSON string
	var dueAt sql.NullInt64
	var lastReviewedAt sql.NullInt64
	err := conn.QueryRow(`
		SELECT topic_id, state_json, due_at, last_reviewed_at
		FROM assessment_fsrs
		WHERE activity_type = ? AND reference_id = ?
	`, activityType, referenceID).Scan(&topicID, &stateJSON, &dueAt, &lastReviewedAt)
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

	record := &assessmentFSRSRecord{
		TopicID:        topicID,
		State:          state,
		DueAt:          dueAt.Int64,
		LastReviewedAt: lastReviewedAt.Int64,
	}
	return record, nil
}

func upsertAssessmentFSRSReviewRepo(activityType, referenceID, topicID string, state models.FlashcardState, dueAt, reviewedAt int64, reviewLog models.FSRSReviewLog) error {
	if conn == nil {
		return fmt.Errorf("database not initialized")
	}
	stateJSON, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("encode assessment fsrs state: %w", err)
	}

	tx, err := conn.Begin()
	if err != nil {
		return err
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	if err = tx.QueryRow(`SELECT id FROM topics WHERE id = ?`, topicID).Scan(&topicID); err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("topic not found: %s", topicID)
		}
		return err
	}

	if _, err = tx.Exec(`
		INSERT INTO assessment_fsrs (
			activity_type, reference_id, topic_id, state_json, due_at, last_reviewed_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(activity_type, reference_id) DO UPDATE SET
			topic_id = excluded.topic_id,
			state_json = excluded.state_json,
			due_at = excluded.due_at,
			last_reviewed_at = excluded.last_reviewed_at,
			updated_at = CURRENT_TIMESTAMP
	`, activityType, referenceID, topicID, string(stateJSON), dueAt, reviewedAt); err != nil {
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

	if err = tx.Commit(); err != nil {
		return err
	}
	committed = true
	return nil
}
