package db

import (
	"database/sql"
	"fmt"
	"strings"

	"ai-tutor/internal/models"
)

// InsertFSRSReviewLog inserts one generic FSRS review event.
func (r *Repository) InsertFSRSReviewLog(reviewLog models.FSRSReviewLog) error {
	reviewLog.ID = strings.TrimSpace(reviewLog.ID)
	reviewLog.TopicID = strings.TrimSpace(reviewLog.TopicID)
	reviewLog.ActivityType = strings.TrimSpace(reviewLog.ActivityType)
	reviewLog.ReferenceID = strings.TrimSpace(reviewLog.ReferenceID)
	reviewLog.StateBeforeJSON = strings.TrimSpace(reviewLog.StateBeforeJSON)
	reviewLog.StateAfterJSON = strings.TrimSpace(reviewLog.StateAfterJSON)

	if reviewLog.ID == "" {
		return fmt.Errorf("review log id is required")
	}
	if reviewLog.TopicID == "" {
		return fmt.Errorf("topic id is required")
	}
	if reviewLog.ActivityType == "" {
		return fmt.Errorf("activity type is required")
	}
	if reviewLog.ReferenceID == "" {
		return fmt.Errorf("reference id is required")
	}
	if reviewLog.ReviewedAt <= 0 {
		return fmt.Errorf("reviewed at is required")
	}
	if reviewLog.Rating < 1 || reviewLog.Rating > 4 {
		return fmt.Errorf("rating must be between 1 and 4")
	}
	if reviewLog.StateBeforeJSON == "" || reviewLog.StateAfterJSON == "" {
		return fmt.Errorf("review state json values are required")
	}
	if reviewLog.ScheduledDays < 0 {
		return fmt.Errorf("scheduled days must be non-negative")
	}

	return r.withTx(func(tx *sql.Tx) error {
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
func (r *Repository) GetRecentReviewLogs(limit int) ([]models.FSRSReviewLog, error) {
	rows, err := r.db.Query(`
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

// GetReviewLogsSince returns all review logs with reviewed_at > since (Unix seconds).
// Used for delta sync: only unsent events are included, eliminating duplicates and the
// arbitrary 100-row cap.
func (r *Repository) GetReviewLogsSince(since int64) ([]models.FSRSReviewLog, error) {
	rows, err := r.db.Query(`
		SELECT id, topic_id, activity_type, reference_id, reviewed_at, rating, scheduled_days, state_before_json, state_after_json
		FROM fsrs_review_log
		WHERE reviewed_at > ?
		ORDER BY reviewed_at ASC
	`, since)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

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
