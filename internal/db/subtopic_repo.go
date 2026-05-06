package db

import (
	"database/sql"
	"fmt"
	"strings"

	"ai-tutor/internal/models"
)

// CreateSubtopic inserts a new subtopic into the database.
func CreateSubtopic(subtopic models.Subtopic) error {
	subtopic.ID = strings.TrimSpace(subtopic.ID)
	subtopic.ParentTopicID = strings.TrimSpace(subtopic.ParentTopicID)
	subtopic.Title = strings.TrimSpace(subtopic.Title)

	if subtopic.ID == "" {
		return fmt.Errorf("subtopic id is required")
	}
	if subtopic.ParentTopicID == "" {
		return fmt.Errorf("parent topic id is required")
	}
	if subtopic.Title == "" {
		return fmt.Errorf("subtopic title is required")
	}
	if subtopic.StartPage < 0 {
		return fmt.Errorf("start page must be non-negative")
	}
	if subtopic.EndPage < subtopic.StartPage {
		return fmt.Errorf("end page must be >= start page")
	}

	_, err := conn.Exec(`
		INSERT INTO subtopics (id, parent_topic_id, title, start_page, end_page, search_snippet)
		VALUES (?, ?, ?, ?, ?, ?)
	`, subtopic.ID, subtopic.ParentTopicID, subtopic.Title, subtopic.StartPage, subtopic.EndPage, subtopic.SearchSnippet)

	return err
}

// GetSubtopicsByParentTopic returns all subtopics for a given parent topic, ordered by start_page.
func GetSubtopicsByParentTopic(parentTopicID string) ([]models.Subtopic, error) {
	parentTopicID = strings.TrimSpace(parentTopicID)
	if parentTopicID == "" {
		return nil, fmt.Errorf("parent topic id is required")
	}

	rows, err := conn.Query(`
		SELECT id, parent_topic_id, title, start_page, end_page, search_snippet, created_at, updated_at
		FROM subtopics
		WHERE parent_topic_id = ?
		ORDER BY start_page ASC
	`, parentTopicID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	subtopics := []models.Subtopic{}
	for rows.Next() {
		var s models.Subtopic
		if err := rows.Scan(&s.ID, &s.ParentTopicID, &s.Title, &s.StartPage, &s.EndPage, &s.SearchSnippet, &s.CreatedAt, &s.UpdatedAt); err != nil {
			return nil, err
		}
		subtopics = append(subtopics, s)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return subtopics, nil
}

// GetSubtopicByID returns a single subtopic by its ID.
func GetSubtopicByID(subtopicID string) (*models.Subtopic, error) {
	subtopicID = strings.TrimSpace(subtopicID)
	if subtopicID == "" {
		return nil, fmt.Errorf("subtopic id is required")
	}

	var s models.Subtopic
	err := conn.QueryRow(`
		SELECT id, parent_topic_id, title, start_page, end_page, search_snippet, created_at, updated_at
		FROM subtopics
		WHERE id = ?
	`, subtopicID).Scan(&s.ID, &s.ParentTopicID, &s.Title, &s.StartPage, &s.EndPage, &s.SearchSnippet, &s.CreatedAt, &s.UpdatedAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return &s, nil
}

// GetSubtopicByPageRange returns the subtopic that contains the given page within a parent topic.
func GetSubtopicByPageRange(parentTopicID string, page int) (*models.Subtopic, error) {
	parentTopicID = strings.TrimSpace(parentTopicID)
	if parentTopicID == "" {
		return nil, fmt.Errorf("parent topic id is required")
	}
	if page < 0 {
		return nil, fmt.Errorf("page must be non-negative")
	}

	var s models.Subtopic
	err := conn.QueryRow(`
		SELECT id, parent_topic_id, title, start_page, end_page, search_snippet, created_at, updated_at
		FROM subtopics
		WHERE parent_topic_id = ? AND start_page <= ? AND end_page >= ?
		ORDER BY start_page ASC
		LIMIT 1
	`, parentTopicID, page, page).Scan(&s.ID, &s.ParentTopicID, &s.Title, &s.StartPage, &s.EndPage, &s.SearchSnippet, &s.CreatedAt, &s.UpdatedAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return &s, nil
}

// UpdateSubtopic updates an existing subtopic.
func UpdateSubtopic(subtopic models.Subtopic) error {
	subtopic.ID = strings.TrimSpace(subtopic.ID)
	subtopic.ParentTopicID = strings.TrimSpace(subtopic.ParentTopicID)
	subtopic.Title = strings.TrimSpace(subtopic.Title)

	if subtopic.ID == "" {
		return fmt.Errorf("subtopic id is required")
	}
	if subtopic.ParentTopicID == "" {
		return fmt.Errorf("parent topic id is required")
	}
	if subtopic.Title == "" {
		return fmt.Errorf("subtopic title is required")
	}
	if subtopic.StartPage < 0 {
		return fmt.Errorf("start page must be non-negative")
	}
	if subtopic.EndPage < subtopic.StartPage {
		return fmt.Errorf("end page must be >= start page")
	}

	_, err := conn.Exec(`
		UPDATE subtopics
		SET title = ?, start_page = ?, end_page = ?, search_snippet = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ? AND parent_topic_id = ?
	`, subtopic.Title, subtopic.StartPage, subtopic.EndPage, subtopic.SearchSnippet, subtopic.ID, subtopic.ParentTopicID)

	return err
}

// DeleteSubtopic removes a subtopic by its ID.
func DeleteSubtopic(subtopicID string) error {
	subtopicID = strings.TrimSpace(subtopicID)
	if subtopicID == "" {
		return fmt.Errorf("subtopic id is required")
	}

	_, err := conn.Exec(`
		DELETE FROM subtopics WHERE id = ?
	`, subtopicID)

	return err
}

// DeleteSubtopicsByParentTopic removes all subtopics for a given parent topic.
func DeleteSubtopicsByParentTopic(parentTopicID string) error {
	parentTopicID = strings.TrimSpace(parentTopicID)
	if parentTopicID == "" {
		return fmt.Errorf("parent topic id is required")
	}

	_, err := conn.Exec(`
		DELETE FROM subtopics WHERE parent_topic_id = ?
	`, parentTopicID)

	return err
}

// GetNextSubtopic returns the next subtopic after the given page within a parent topic.
func GetNextSubtopic(parentTopicID string, currentPage int) (*models.Subtopic, error) {
	parentTopicID = strings.TrimSpace(parentTopicID)
	if parentTopicID == "" {
		return nil, fmt.Errorf("parent topic id is required")
	}
	if currentPage < 0 {
		return nil, fmt.Errorf("current page must be non-negative")
	}

	var s models.Subtopic
	err := conn.QueryRow(`
		SELECT id, parent_topic_id, title, start_page, end_page, search_snippet, created_at, updated_at
		FROM subtopics
		WHERE parent_topic_id = ? AND start_page > ?
		ORDER BY start_page ASC
		LIMIT 1
	`, parentTopicID, currentPage).Scan(&s.ID, &s.ParentTopicID, &s.Title, &s.StartPage, &s.EndPage, &s.SearchSnippet, &s.CreatedAt, &s.UpdatedAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return &s, nil
}

// CountSubtopicsByParentTopic returns the number of subtopics for a given parent topic.
func CountSubtopicsByParentTopic(parentTopicID string) (int, error) {
	parentTopicID = strings.TrimSpace(parentTopicID)
	if parentTopicID == "" {
		return 0, fmt.Errorf("parent topic id is required")
	}

	var count int
	err := conn.QueryRow(`
		SELECT COUNT(*)
		FROM subtopics
		WHERE parent_topic_id = ?
	`, parentTopicID).Scan(&count)

	return count, err
}
