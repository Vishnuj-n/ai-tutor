package db

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

func (r *Repository) GetRereadAttemptCount(topicID string) (int, error) {
	topicID = strings.TrimSpace(topicID)
	if topicID == "" {
		return 0, fmt.Errorf("topic id is required")
	}

	var count int
	err := r.db.QueryRow(`
		SELECT COALESCE(attempt_count, 0)
		FROM reread_attempts
		WHERE topic_id = ?
	`, topicID).Scan(&count)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	return count, nil
}

func (r *Repository) IncrementRereadAttemptCountTx(tx *sql.Tx, topicID string) (int, error) {
	topicID = strings.TrimSpace(topicID)
	if topicID == "" {
		return 0, fmt.Errorf("topic id is required")
	}

	var count int
	if err := tx.QueryRow(`
		INSERT INTO reread_attempts (topic_id, attempt_count, last_attempt_at)
		VALUES (?, 1, CURRENT_TIMESTAMP)
		ON CONFLICT(topic_id) DO UPDATE
		SET attempt_count = reread_attempts.attempt_count + 1,
		    last_attempt_at = CURRENT_TIMESTAMP
		RETURNING attempt_count
	`, topicID).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func (r *Repository) ResetRereadAttemptCountTx(tx *sql.Tx, topicID string) error {
	topicID = strings.TrimSpace(topicID)
	if topicID == "" {
		return fmt.Errorf("topic id is required")
	}

	_, err := tx.Exec(`
		INSERT INTO reread_attempts (topic_id, attempt_count, last_attempt_at)
		VALUES (?, 0, CURRENT_TIMESTAMP)
		ON CONFLICT(topic_id) DO UPDATE
		SET attempt_count = 0,
		    last_attempt_at = CURRENT_TIMESTAMP
	`, topicID)
	return err
}
