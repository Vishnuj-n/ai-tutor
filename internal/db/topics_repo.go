package db

import (
	"database/sql"
	"fmt"
	"log"
	"strings"

	"ai-tutor/internal/models"
	"ai-tutor/internal/utils"
)

// EnsureTopic inserts a topic if it does not already exist.
func (r *Repository) EnsureTopic(topicID, title string) error {
	topicID = strings.TrimSpace(topicID)
	if topicID == "" {
		return fmt.Errorf("topic id is required")
	}
	if title == "" {
		title = topicID
	}

	_, err := r.db.Exec(`
		INSERT INTO topics (id, title, status)
		VALUES (?, ?, 'reading')
		ON CONFLICT(id) DO UPDATE SET title = excluded.title
	`, topicID, title)
	return err
}

// TopicBatchItem represents a topic to be created/updated in batch
type TopicBatchItem struct {
	TopicID string
	Title   string
}

// EnsureTopicsBatch creates or updates multiple topics in a single transaction
func (r *Repository) EnsureTopicsBatch(items []TopicBatchItem) error {
	if len(items) == 0 {
		return nil
	}

	return r.withTx(func(tx *sql.Tx) error {
		return r.EnsureTopicsBatchTx(tx, items)
	})
}

// EnsureTopicsBatchTx creates or updates multiple topics within an existing transaction
func (r *Repository) EnsureTopicsBatchTx(tx *sql.Tx, items []TopicBatchItem) error {
	if len(items) == 0 {
		return nil
	}

	stmt, err := tx.Prepare(`
		INSERT INTO topics (id, title, status)
		VALUES (?, ?, 'reading')
		ON CONFLICT(id) DO UPDATE SET title = excluded.title
	`)
	if err != nil {
		return err
	}
	defer func() {
		_ = stmt.Close()
	}()

	for _, item := range items {
		id := strings.TrimSpace(item.TopicID)
		if id == "" {
			return fmt.Errorf("topic id is required for all batch items")
		}
		title := item.Title
		if title == "" {
			title = id
		}

		_, err = stmt.Exec(id, title)
		if err != nil {
			return err
		}
	}
	return nil
}

// UpdateTopicPageBounds stores deterministic chapter bounds for a topic.
// Initializes current_page_cursor to startPage if it is 0 (uninitialized).
func (r *Repository) UpdateTopicPageBounds(topicID string, startPage, endPage int) error {
	topicID = strings.TrimSpace(topicID)
	if topicID == "" {
		return fmt.Errorf("topic id is required")
	}
	if startPage < 0 {
		startPage = 0
	}
	if endPage < 0 {
		endPage = 0
	}
	if startPage > endPage {
		startPage, endPage = endPage, startPage
	}

	return r.withTx(func(tx *sql.Tx) error {
		// Determine if cursor needs initialization and detect shrinkage
		var previousStart int
		var previousEnd int
		var currentCursor int
		if err := tx.QueryRow(`
			SELECT COALESCE(start_page, 0), COALESCE(end_page, 0), COALESCE(current_page_cursor, 0)
			FROM topics WHERE id = ?
		`, topicID).Scan(&previousStart, &previousEnd, &currentCursor); err != nil && err != sql.ErrNoRows {
			return err
		}

		// Check if bounds shrunk (start moved forward OR end moved backward)
		shrunk := (previousStart > 0 && startPage > 0 && startPage > previousStart) ||
			(previousEnd > 0 && endPage > 0 && endPage < previousEnd)

		// Initialize cursor to startPage if uninitialized (0), otherwise clamp to new bounds
		var newCursor int
		if currentCursor == 0 {
			newCursor = startPage
			if newCursor < 0 {
				newCursor = 0
			}
		} else {
			// Clamp cursor to new bounds
			if currentCursor < startPage {
				newCursor = startPage
			} else if currentCursor > endPage {
				newCursor = endPage
			} else {
				newCursor = currentCursor
			}
		}

		// Update bounds and cursor
		result, err := tx.Exec(`
			UPDATE topics
			SET start_page = ?, end_page = ?, current_page_cursor = ?
			WHERE id = ?
		`, startPage, endPage, newCursor, topicID)
		if err != nil {
			return err
		}

		rowsAffected, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if rowsAffected == 0 {
			return sql.ErrNoRows
		}

		if shrunk {
			if err := r.deleteAssessmentDataOutsideBoundsTx(tx, topicID, startPage, endPage); err != nil {
				return err
			}
		}
		return nil
	})
}

// TopicPageBoundsBatchItem represents topic page bounds to be updated in batch
type TopicPageBoundsBatchItem struct {
	TopicID   string
	StartPage int
	EndPage   int
}

// UpdateTopicPageBoundsBatch updates page bounds for multiple topics in a single transaction
func (r *Repository) UpdateTopicPageBoundsBatch(items []TopicPageBoundsBatchItem) error {
	if len(items) == 0 {
		return nil
	}

	return r.withTx(func(tx *sql.Tx) error {
		for _, item := range items {
			topicID := strings.TrimSpace(item.TopicID)
			if topicID == "" {
				return fmt.Errorf("topic id is required for all batch items")
			}

			startPage := item.StartPage
			endPage := item.EndPage
			if startPage < 0 {
				startPage = 0
			}
			if endPage < 0 {
				endPage = 0
			}
			if startPage > endPage {
				startPage, endPage = endPage, startPage
			}

			// Check current cursor and detect shrinkage
			var previousStart int
			var previousEnd int
			var currentCursor int
			if cursorErr := tx.QueryRow(`
				SELECT COALESCE(start_page, 0), COALESCE(end_page, 0), COALESCE(current_page_cursor, 0)
				FROM topics WHERE id = ?
			`, topicID).Scan(&previousStart, &previousEnd, &currentCursor); cursorErr != nil && cursorErr != sql.ErrNoRows {
				return cursorErr
			}

			// Check if bounds shrunk (start moved forward OR end moved backward)
			shrunk := (previousStart > 0 && startPage > 0 && startPage > previousStart) ||
				(previousEnd > 0 && endPage > 0 && endPage < previousEnd)

			// Initialize cursor to startPage if uninitialized (0), otherwise clamp to new bounds
			var newCursor int
			if currentCursor == 0 {
				newCursor = startPage
				if newCursor < 0 {
					newCursor = 0
				}
			} else {
				// Clamp cursor to new bounds
				if currentCursor < startPage {
					newCursor = startPage
				} else if currentCursor > endPage {
					newCursor = endPage
				} else {
					newCursor = currentCursor
				}
			}

			// Update bounds and cursor
			res, err := tx.Exec(`
				UPDATE topics
				SET start_page = ?, end_page = ?, current_page_cursor = ?
				WHERE id = ?
			`, startPage, endPage, newCursor, topicID)
			if err != nil {
				return err
			}
			rowsAffected, err := res.RowsAffected()
			if err != nil {
				return err
			}
			if rowsAffected == 0 {
				return fmt.Errorf("no rows updated for topicID %s", topicID)
			}

			if shrunk {
				if err := r.deleteAssessmentDataOutsideBoundsTx(tx, topicID, startPage, endPage); err != nil {
					return err
				}
			}
		}
		return nil
	})
}

func (r *Repository) deleteAssessmentDataOutsideBoundsTx(tx *sql.Tx, topicID string, startPage int, endPage int) error {

	if _, err := tx.Exec(`
		DELETE FROM fsrs_review_log
		WHERE activity_type = 'written_question'
		  AND reference_id IN (
			SELECT id
			FROM written_questions
			WHERE topic_id = ?
			  AND (source_page_start IS NOT NULL AND source_page_start < ? OR source_page_end IS NOT NULL AND source_page_end > ?)
		)
	`, topicID, startPage, endPage); err != nil {
		return fmt.Errorf("delete out-of-range written review logs: %w", err)
	}

	if _, err := tx.Exec(`
		DELETE FROM written_questions
		WHERE topic_id = ?
		  AND (source_page_start IS NOT NULL AND source_page_start < ? OR source_page_end IS NOT NULL AND source_page_end > ?)
	`, topicID, startPage, endPage); err != nil {
		return fmt.Errorf("delete out-of-range written questions: %w", err)
	}

	return nil
}

// GetTopicPageBounds returns persisted chapter bounds for a topic.
func (r *Repository) GetTopicPageBounds(topicID string) (int, int, error) {
	topicID = strings.TrimSpace(topicID)
	if topicID == "" {
		return 0, 0, fmt.Errorf("topic id is required")
	}

	var startPage int
	var endPage int
	err := r.db.QueryRow(`
		SELECT COALESCE(start_page, 0), COALESCE(end_page, 0)
		FROM topics
		WHERE id = ?
	`, topicID).Scan(&startPage, &endPage)
	if err != nil {
		return 0, 0, err
	}

	return startPage, endPage, nil
}

// NotebookTopicInfo contains topic id, title and persisted page bounds for a notebook-scoped topic
type NotebookTopicInfo struct {
	TopicID   string
	Title     string
	StartPage int
	EndPage   int
}

// GetNotebookTopicsWithBounds returns topics linked to a notebook with their title and page bounds.
// Topics are ordered by topic id which for autogenerated notebook topics encodes chapter index.
func (r *Repository) GetNotebookTopicsWithBounds(notebookID string) ([]NotebookTopicInfo, error) {
	notebookID = strings.TrimSpace(notebookID)
	if notebookID == "" {
		return nil, fmt.Errorf("notebook id is required")
	}

	rows, err := r.db.Query(`
		SELECT t.id, COALESCE(t.title, ''), COALESCE(t.start_page, 0), COALESCE(t.end_page, 0)
		FROM notebook_topics nt
		JOIN topics t ON t.id = nt.topic_id
		WHERE nt.notebook_id = ?
		ORDER BY t.id
	`, notebookID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var result []NotebookTopicInfo
	for rows.Next() {
		var ti NotebookTopicInfo
		if err := rows.Scan(&ti.TopicID, &ti.Title, &ti.StartPage, &ti.EndPage); err != nil {
			return nil, err
		}
		result = append(result, ti)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return result, nil
}

// QueryNextReadingTopic returns the next reading topic with deterministic page bounds and cursor.
// Joins with notebooks table to ensure topics have a valid notebook source.
// Orders by notebook priority (higher first) to respect notebook priority biasing.
func (r *Repository) QueryNextReadingTopic() (models.ReadingTopicCursor, bool, error) {
	settings, err := r.GetUserSettings()
	if err != nil {
		return models.ReadingTopicCursor{}, false, fmt.Errorf("QueryNextReadingTopic: getting user settings: %w", err)
	}
	activeProfileStr := settings.ActiveProfileID

	var topic models.ReadingTopicCursor
	query := `
		SELECT
			t.id,
			t.title,
			COALESCE(t.start_page, 0),
			COALESCE(t.end_page, 0),
			COALESCE(t.current_page_cursor, 0),
			n.id
		FROM topics t
		LEFT JOIN notebook_topics nt ON nt.topic_id = t.id
		LEFT JOIN notebooks n ON (n.id = nt.notebook_id OR n.topic_id = t.id)
		WHERE t.status IN ('unseen', 'reading')
		  AND COALESCE(t.end_page, 0) > 0
		  AND COALESCE(t.current_page_cursor, 0) < COALESCE(t.end_page, 0)
		  AND (nt.notebook_id IS NOT NULL OR n.topic_id = t.id)
		  AND n.id IS NOT NULL
		  AND n.id != ''
		  AND n.study_status = 'active'
	`
	var args []interface{}
	if activeProfileStr != "" {
		query += ` AND (n.profile_id = ? OR n.profile_id IS NULL OR n.profile_id = '') `
		args = append(args, activeProfileStr)
	}
	query += ` ORDER BY COALESCE(n.priority, 5) DESC, t.updated_at ASC, t.created_at ASC LIMIT 1 `

	err = r.db.QueryRow(query, args...).Scan(&topic.ID, &topic.Title, &topic.StartPage, &topic.EndPage, &topic.CurrentPageCursor, &topic.NotebookID)
	if err == sql.ErrNoRows {
		return models.ReadingTopicCursor{}, false, nil
	}
	if err != nil {
		return models.ReadingTopicCursor{}, false, err
	}
	utils.Warnf("[SCHEDULER] QueryNextReadingTopic selected topicID=%s notebookID=%s (ordered by notebook priority DESC)", topic.ID, topic.NotebookID)
	return topic, true, nil
}

// GetAllTopicIDs returns all topic IDs currently in the database.
func (r *Repository) GetAllTopicIDs() ([]string, error) {
	rows, err := r.db.Query("SELECT id FROM topics ORDER BY id")
	if err != nil {
		return nil, err
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			log.Printf("warning: failed to close topic rows: %v", closeErr)
		}
	}()

	var topicIDs []string
	for rows.Next() {
		var topicID string
		if err := rows.Scan(&topicID); err != nil {
			return nil, err
		}
		topicIDs = append(topicIDs, topicID)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return topicIDs, nil
}

// GetAllTopics returns all topics as id/title pairs.
func (r *Repository) GetAllTopics() ([]map[string]string, error) {
	rows, err := r.db.Query("SELECT id, title FROM topics ORDER BY title")
	if err != nil {
		return nil, err
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			log.Printf("warning: failed to close topics rows: %v", closeErr)
		}
	}()

	topics := make([]map[string]string, 0)
	for rows.Next() {
		var id string
		var title string
		if err := rows.Scan(&id, &title); err != nil {
			return nil, err
		}
		topics = append(topics, map[string]string{
			"id":    id,
			"title": title,
		})
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return topics, nil
}

// UpdateTopicReadingCursor persists the current page cursor and optionally marks topic as learned.
func (r *Repository) UpdateTopicReadingCursor(topicID string, cursor int, markLearned bool) error {
	topicID = strings.TrimSpace(topicID)
	if topicID == "" {
		return fmt.Errorf("topic id is required")
	}
	if cursor < 0 {
		cursor = 0
	}

	status := "reading"
	if markLearned {
		status = "learned"
	}

	result, err := r.db.Exec(`
		UPDATE topics
		SET current_page_cursor = ?, status = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, cursor, status, topicID)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return fmt.Errorf("topic not found: %s", topicID)
	}

	return nil
}

// DeleteTopic removes a topic and all associated data
func (r *Repository) DeleteTopic(topicID string) error {
	topicID = strings.TrimSpace(topicID)
	if topicID == "" {
		return fmt.Errorf("topic id is required")
	}

	return r.withTx(func(tx *sql.Tx) error {
		// Delete dependent tables in order to respect foreign key constraints

		// Delete notebook_chunks (via chunks)
		if _, err := tx.Exec("DELETE FROM notebook_chunks WHERE chunk_id IN (SELECT id FROM chunks WHERE topic_id = ?)", topicID); err != nil {
			return fmt.Errorf("failed to delete notebook_chunks: %w", err)
		}

		// Delete fsrs_review_log
		if _, err := tx.Exec("DELETE FROM fsrs_review_log WHERE topic_id = ?", topicID); err != nil {
			return fmt.Errorf("failed to delete fsrs_review_log: %w", err)
		}

		// Delete fsrs_cards
		if _, err := tx.Exec("DELETE FROM fsrs_cards WHERE topic_id = ?", topicID); err != nil {
			return fmt.Errorf("failed to delete fsrs_cards: %w", err)
		}

		// Delete topic_progress
		if _, err := tx.Exec("DELETE FROM topic_progress WHERE topic_id = ?", topicID); err != nil {
			return fmt.Errorf("failed to delete topic_progress: %w", err)
		}

		// Delete chunks
		if _, err := tx.Exec("DELETE FROM chunks WHERE topic_id = ?", topicID); err != nil {
			return fmt.Errorf("failed to delete chunks: %w", err)
		}

		// Update notebooks that reference this topic to null
		if _, err := tx.Exec("UPDATE notebooks SET topic_id = NULL WHERE topic_id = ?", topicID); err != nil {
			return fmt.Errorf("failed to update notebooks: %w", err)
		}

		// Finally delete the topic
		if _, err := tx.Exec("DELETE FROM topics WHERE id = ?", topicID); err != nil {
			return fmt.Errorf("failed to delete topic: %w", err)
		}
		return nil
	})
}

// DeleteFSRSCardsByTopicIDTx deletes FSRS cards for a given topic in a transaction.
func (r *Repository) DeleteFSRSCardsByTopicIDTx(tx *sql.Tx, topicID string) error {
	_, err := tx.Exec("DELETE FROM fsrs_cards WHERE topic_id = ?", topicID)
	return err
}

// MarkTopicExternalHelpRequiredTx sets external_help_required = 1 for the specified topic within a transaction.
func (r *Repository) MarkTopicExternalHelpRequiredTx(tx *sql.Tx, topicID string) error {
	topicID = strings.TrimSpace(topicID)
	if topicID == "" {
		return fmt.Errorf("topic id is required")
	}
	result, err := tx.Exec(`
		UPDATE topics
		SET external_help_required = 1, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, topicID)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return fmt.Errorf("topic %s not found", topicID)
	}
	return nil
}

