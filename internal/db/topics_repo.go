package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"ai-tutor/internal/models"
	"ai-tutor/internal/utils"
)

// EnsureTopic inserts a topic if it does not already exist.
func EnsureTopic(topicID, title string) error {
	topicID = strings.TrimSpace(topicID)
	if topicID == "" {
		return fmt.Errorf("topic id is required")
	}
	if title == "" {
		title = topicID
	}

	_, err := conn.Exec(`
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
func EnsureTopicsBatch(items []TopicBatchItem) error {
	if len(items) == 0 {
		return nil
	}

	tx, err := conn.Begin()
	if err != nil {
		return err
	}
	defer func() {
		_ = tx.Rollback()
	}()

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
			err = fmt.Errorf("topic id is required for all batch items")
			return err
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

	return tx.Commit()
}

// UpdateTopicPageBounds stores deterministic chapter bounds for a topic.
// Initializes current_page_cursor to startPage if it is 0 (uninitialized).
func UpdateTopicPageBounds(topicID string, startPage, endPage int) error {
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

	tx, err := conn.Begin()
	if err != nil {
		return err
	}
	defer func() {
		_ = tx.Rollback()
	}()

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
		if err := deleteAssessmentDataOutsideBoundsTx(tx, topicID, startPage, endPage); err != nil {
			return err
		}
	}

	return tx.Commit()
}

// TopicPageBoundsBatchItem represents topic page bounds to be updated in batch
type TopicPageBoundsBatchItem struct {
	TopicID   string
	StartPage int
	EndPage   int
}

// UpdateTopicPageBoundsBatch updates page bounds for multiple topics in a single transaction
func UpdateTopicPageBoundsBatch(items []TopicPageBoundsBatchItem) error {
	if len(items) == 0 {
		return nil
	}

	tx, err := conn.Begin()
	if err != nil {
		return err
	}
	defer func() {
		_ = tx.Rollback()
	}()

	for _, item := range items {
		topicID := strings.TrimSpace(item.TopicID)
		if topicID == "" {
			err = fmt.Errorf("topic id is required for all batch items")
			return err
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
			if err := deleteAssessmentDataOutsideBoundsTx(tx, topicID, startPage, endPage); err != nil {
				return err
			}
		}
	}

	return tx.Commit()
}

func deleteAssessmentDataOutsideBoundsTx(tx *sql.Tx, topicID string, startPage int, endPage int) error {
	if _, err := tx.Exec(`
		DELETE FROM user_answers
		WHERE question_id IN (
			SELECT id
			FROM questions
			WHERE topic_id = ?
			  AND (source_page_start IS NOT NULL AND source_page_start < ? OR source_page_end IS NOT NULL AND source_page_end > ?)
		)
	`, topicID, startPage, endPage); err != nil {
		return fmt.Errorf("delete out-of-range user answers: %w", err)
	}

	if _, err := tx.Exec(`
		DELETE FROM fsrs_review_log
		WHERE activity_type = 'quiz_question'
		  AND reference_id IN (
			SELECT id
			FROM questions
			WHERE topic_id = ?
			  AND (source_page_start IS NOT NULL AND source_page_start < ? OR source_page_end IS NOT NULL AND source_page_end > ?)
		)
	`, topicID, startPage, endPage); err != nil {
		return fmt.Errorf("delete out-of-range quiz review logs: %w", err)
	}

	if _, err := tx.Exec(`
		DELETE FROM assessment_fsrs
		WHERE activity_type = 'quiz_question'
		  AND reference_id IN (
			SELECT id
			FROM questions
			WHERE topic_id = ?
			  AND (source_page_start IS NOT NULL AND source_page_start < ? OR source_page_end IS NOT NULL AND source_page_end > ?)
		)
	`, topicID, startPage, endPage); err != nil {
		return fmt.Errorf("delete out-of-range quiz fsrs state: %w", err)
	}

	if _, err := tx.Exec(`
		DELETE FROM questions
		WHERE topic_id = ?
		  AND (source_page_start IS NOT NULL AND source_page_start < ? OR source_page_end IS NOT NULL AND source_page_end > ?)
	`, topicID, startPage, endPage); err != nil {
		return fmt.Errorf("delete out-of-range questions: %w", err)
	}

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
		DELETE FROM assessment_fsrs
		WHERE activity_type = 'written_question'
		  AND reference_id IN (
			SELECT id
			FROM written_questions
			WHERE topic_id = ?
			  AND (source_page_start IS NOT NULL AND source_page_start < ? OR source_page_end IS NOT NULL AND source_page_end > ?)
		)
	`, topicID, startPage, endPage); err != nil {
		return fmt.Errorf("delete out-of-range written fsrs state: %w", err)
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
func GetTopicPageBounds(topicID string) (int, int, error) {
	topicID = strings.TrimSpace(topicID)
	if topicID == "" {
		return 0, 0, fmt.Errorf("topic id is required")
	}

	var startPage int
	var endPage int
	err := conn.QueryRow(`
		SELECT COALESCE(start_page, 0), COALESCE(end_page, 0)
		FROM topics
		WHERE id = ?
	`, topicID).Scan(&startPage, &endPage)
	if err != nil {
		return 0, 0, err
	}

	return startPage, endPage, nil
}

// GetTopicCurrentPageCursor returns the current page cursor for a topic.
func GetTopicCurrentPageCursor(topicID string) (int, error) {
	topicID = strings.TrimSpace(topicID)
	if topicID == "" {
		return 0, fmt.Errorf("topic id is required")
	}

	var cursor int
	err := conn.QueryRow(`
		SELECT COALESCE(current_page_cursor, 0)
		FROM topics
		WHERE id = ?
	`, topicID).Scan(&cursor)
	if err != nil {
		return 0, err
	}

	return cursor, nil
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
func GetNotebookTopicsWithBounds(notebookID string) ([]NotebookTopicInfo, error) {
	notebookID = strings.TrimSpace(notebookID)
	if notebookID == "" {
		return nil, fmt.Errorf("notebook id is required")
	}

	rows, err := conn.Query(`
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
func QueryNextReadingTopic() (models.ReadingTopicCursor, bool, error) {
	var topic models.ReadingTopicCursor
	err := conn.QueryRow(`
		SELECT
			t.id,
			t.title,
			COALESCE(t.start_page, 0),
			COALESCE(t.end_page, 0),
			COALESCE(t.current_page_cursor, 0),
			n.id
		FROM topics t
		INNER JOIN notebooks n ON n.topic_id = t.id
		WHERE t.status IN ('unseen', 'reading')
		  AND COALESCE(t.end_page, 0) > 0
		  AND COALESCE(t.current_page_cursor, 0) < COALESCE(t.end_page, 0)
		  AND n.id IS NOT NULL
		  AND n.id != ''
		ORDER BY COALESCE(n.priority, 5) DESC, t.updated_at ASC, t.created_at ASC
		LIMIT 1
	`).Scan(&topic.ID, &topic.Title, &topic.StartPage, &topic.EndPage, &topic.CurrentPageCursor, &topic.NotebookID)
	if err == sql.ErrNoRows {
		return models.ReadingTopicCursor{}, false, nil
	}
	if err != nil {
		return models.ReadingTopicCursor{}, false, err
	}
	utils.Warnf("[SCHEDULER] QueryNextReadingTopic selected topicID=%s notebookID=%s (ordered by notebook priority DESC)", topic.ID, topic.NotebookID)
	return topic, true, nil
}

// QueryActiveTopics returns top N active topic titles
func QueryActiveTopics(limit int) ([]string, error) {
	rows, err := conn.Query(`
		SELECT title
		FROM topics
		WHERE status IN ('reading', 'learned')
		ORDER BY updated_at DESC, created_at DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = rows.Close()
	}()

	var active []string
	for rows.Next() {
		var title string
		if err := rows.Scan(&title); err != nil {
			return nil, err
		}
		active = append(active, title)
	}
	return active, nil
}

// QueryLearningTopics returns topics ready for learning
func QueryLearningTopics(limit int) ([]models.TopicSummary, error) {
	rows, err := conn.Query(`
		SELECT id, title, status
		FROM topics
		WHERE status IN ('unseen', 'reading')
		ORDER BY updated_at ASC, created_at ASC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = rows.Close()
	}()

	var topics []models.TopicSummary
	for rows.Next() {
		var topic models.TopicSummary
		if err := rows.Scan(&topic.ID, &topic.Title, &topic.Status); err != nil {
			return nil, err
		}
		topics = append(topics, topic)
	}
	return topics, nil
}

// QueryUpcomingReadingTopics returns ordered unread/in-progress topics with configured bounds.
func QueryUpcomingReadingTopics(limit int) ([]models.ReadingTopicCursor, error) {
	if limit <= 0 {
		return []models.ReadingTopicCursor{}, nil
	}

	rows, err := conn.Query(`
		SELECT
			id,
			title,
			COALESCE(start_page, 0),
			COALESCE(end_page, 0),
			COALESCE(current_page_cursor, 0)
		FROM topics
		WHERE status IN ('unseen', 'reading')
		  AND COALESCE(end_page, 0) > 0
		  AND COALESCE(current_page_cursor, 0) < COALESCE(end_page, 0)
		ORDER BY updated_at ASC, created_at ASC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = rows.Close()
	}()

	topics := make([]models.ReadingTopicCursor, 0, limit)
	for rows.Next() {
		var topic models.ReadingTopicCursor
		if err := rows.Scan(&topic.ID, &topic.Title, &topic.StartPage, &topic.EndPage, &topic.CurrentPageCursor); err != nil {
			return nil, err
		}
		topics = append(topics, topic)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return topics, nil
}

// CountLearnedTopics returns the count of fully learned topics
func CountLearnedTopics() (int, error) {
	var count int
	err := conn.QueryRow(`
		SELECT COUNT(*)
		FROM topics
		WHERE status = 'learned'
	`).Scan(&count)
	return count, err
}

// GetAllTopicIDs returns all topic IDs currently in the database.
func GetAllTopicIDs() ([]string, error) {
	rows, err := conn.Query("SELECT id FROM topics ORDER BY id")
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
func GetAllTopics() ([]map[string]string, error) {
	rows, err := conn.Query("SELECT id, title FROM topics ORDER BY title")
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
func UpdateTopicReadingCursor(topicID string, cursor int, markLearned bool) error {
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

	result, err := conn.Exec(`
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

// AppendQuestionsAndAdvanceCursor atomically appends questions and updates the reading cursor in a single transaction
func AppendQuestionsAndAdvanceCursor(topicID string, questions []models.QuizQuestion, nextCursor int, markLearned bool) error {
	if len(questions) == 0 {
		return fmt.Errorf("at least one question is required")
	}

	topicID = strings.TrimSpace(topicID)
	if topicID == "" {
		return fmt.Errorf("topic id is required")
	}
	if nextCursor < 0 {
		nextCursor = 0
	}

	tx, err := conn.Begin()
	if err != nil {
		return err
	}
	defer func() {
		_ = tx.Rollback()
	}()

	// Append questions first
	for _, q := range questions {
		if q.TopicID != topicID {
			err = fmt.Errorf("question topic_id %s does not match target topic %s", q.TopicID, topicID)
			return err
		}
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

	// Update cursor
	status := "reading"
	if markLearned {
		status = "learned"
	}

	result, err := tx.Exec(`
		UPDATE topics
		SET current_page_cursor = ?, status = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, nextCursor, status, topicID)
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

	return tx.Commit()
}

// DeleteTopic removes a topic and all associated data
func DeleteTopic(topicID string) error {
	topicID = strings.TrimSpace(topicID)
	if topicID == "" {
		return fmt.Errorf("topic id is required")
	}

	// Begin transaction for atomic deletion
	tx, err := conn.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	// Delete dependent tables in order to respect foreign key constraints

	// Delete user_answers (via questions)
	if _, err = tx.Exec("DELETE FROM user_answers WHERE question_id IN (SELECT id FROM questions WHERE topic_id = ?)", topicID); err != nil {
		return fmt.Errorf("failed to delete user_answers: %w", err)
	}

	// Delete notebook_chunks (via chunks)
	if _, err = tx.Exec("DELETE FROM notebook_chunks WHERE chunk_id IN (SELECT id FROM chunks WHERE topic_id = ?)", topicID); err != nil {
		return fmt.Errorf("failed to delete notebook_chunks: %w", err)
	}

	// Delete fsrs_review_log
	if _, err = tx.Exec("DELETE FROM fsrs_review_log WHERE topic_id = ?", topicID); err != nil {
		return fmt.Errorf("failed to delete fsrs_review_log: %w", err)
	}

	// Delete fsrs_cards
	if _, err = tx.Exec("DELETE FROM fsrs_cards WHERE topic_id = ?", topicID); err != nil {
		return fmt.Errorf("failed to delete fsrs_cards: %w", err)
	}

	// Delete questions
	if _, err = tx.Exec("DELETE FROM questions WHERE topic_id = ?", topicID); err != nil {
		return fmt.Errorf("failed to delete questions: %w", err)
	}

	// Delete topic_progress
	if _, err = tx.Exec("DELETE FROM topic_progress WHERE topic_id = ?", topicID); err != nil {
		return fmt.Errorf("failed to delete topic_progress: %w", err)
	}

	// Delete chunks
	if _, err = tx.Exec("DELETE FROM chunks WHERE topic_id = ?", topicID); err != nil {
		return fmt.Errorf("failed to delete chunks: %w", err)
	}

	// Delete parents
	if _, err = tx.Exec("DELETE FROM parents WHERE topic_id = ?", topicID); err != nil {
		return fmt.Errorf("failed to delete parents: %w", err)
	}

	// Update notebooks that reference this topic to null
	if _, err = tx.Exec("UPDATE notebooks SET topic_id = NULL WHERE topic_id = ?", topicID); err != nil {
		return fmt.Errorf("failed to update notebooks: %w", err)
	}

	// Finally delete the topic
	if _, err = tx.Exec("DELETE FROM topics WHERE id = ?", topicID); err != nil {
		return fmt.Errorf("failed to delete topic: %w", err)
	}

	// Commit the transaction
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}
