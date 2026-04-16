package db

import (
	"database/sql"
	"fmt"

	"ai-tutor/internal/models"
)

func insertFSRSReviewLogRepo(reviewLog models.FSRSReviewLog) error {
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

	var validatedTopicID string
	if err = tx.QueryRow(`SELECT id FROM topics WHERE id = ?`, reviewLog.TopicID).Scan(&validatedTopicID); err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("topic not found for review log topic_id=%q", reviewLog.TopicID)
		}
		return err
	}

	if _, err = tx.Exec(`
		INSERT INTO fsrs_review_log (
			id, topic_id, activity_type, reference_id, reviewed_at, rating,
			scheduled_days, state_before_json, state_after_json
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, reviewLog.ID, validatedTopicID, reviewLog.ActivityType, reviewLog.ReferenceID,
		reviewLog.ReviewedAt, reviewLog.Rating, reviewLog.ScheduledDays,
		reviewLog.StateBeforeJSON, reviewLog.StateAfterJSON); err != nil {
		return err
	}

	if err = tx.Commit(); err != nil {
		return err
	}
	committed = true
	return nil
}
