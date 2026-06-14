package db

import (
	"database/sql"
	"fmt"

	"ai-tutor/internal/models"
)

func insertFSRSReviewLogRepo(reviewLog models.FSRSReviewLog) error {
	return withTx(func(tx *sql.Tx) error {
		var validatedTopicID string
		if err := tx.QueryRow(`SELECT id FROM topics WHERE id = ?`, reviewLog.TopicID).Scan(&validatedTopicID); err != nil {
			if err == sql.ErrNoRows {
				return fmt.Errorf("topic not found for review log topic_id=%q", reviewLog.TopicID)
			}
			return err
		}

		if _, err := tx.Exec(`
			INSERT INTO fsrs_review_log (
				id, topic_id, activity_type, reference_id, reviewed_at, rating,
				scheduled_days, state_before_json, state_after_json
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, reviewLog.ID, validatedTopicID, reviewLog.ActivityType, reviewLog.ReferenceID,
			reviewLog.ReviewedAt, reviewLog.Rating, reviewLog.ScheduledDays,
			reviewLog.StateBeforeJSON, reviewLog.StateAfterJSON); err != nil {
			return err
		}
		return nil
	})
}

// GetRecentReviewLogs retrieves recent FSRS review logs.
func GetRecentReviewLogs(limit int) ([]models.FSRSReviewLog, error) {
	rows, err := conn.Query(`
		SELECT id, topic_id, activity_type, reference_id, reviewed_at, rating, scheduled_days, state_before_json, state_after_json
		FROM fsrs_review_log
		ORDER BY reviewed_at DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = rows.Close()
	}()

	var logs []models.FSRSReviewLog
	for rows.Next() {
		var log models.FSRSReviewLog
		if err := rows.Scan(
			&log.ID, &log.TopicID, &log.ActivityType, &log.ReferenceID,
			&log.ReviewedAt, &log.Rating, &log.ScheduledDays,
			&log.StateBeforeJSON, &log.StateAfterJSON,
		); err != nil {
			return nil, err
		}
		logs = append(logs, log)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return logs, nil
}
