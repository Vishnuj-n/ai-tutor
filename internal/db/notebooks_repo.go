package db

import (
	"database/sql"
	"fmt"
	"strings"

	"ai-tutor/internal/models"
)

// CreateNotebook saves a notebook record to the database
func CreateNotebook(id, title, filePath, fileType, topicID string, pageCount int) error {
	var topicValue interface{}
	if topicID != "" {
		validatedTopicID, err := validateID(topicID, "topic id")
		if err != nil {
			return err
		}
		topicValue = validatedTopicID
	}

	validatedID, err := validateID(id, "notebook id")
	if err != nil {
		return err
	}

	_, err = conn.Exec(`
		INSERT INTO notebooks (id, title, file_path, file_type, topic_id, status, page_count)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, validatedID, title, filePath, fileType, topicValue, "uploaded", pageCount)
	return err
}

// NotebookParentInput is a parent section row used by notebook ingestion transactions.
type NotebookParentInput struct {
	ID         string
	Heading    string
	Content    string
	OrderIndex int
}

// NotebookChunkInput is a chunk row used by notebook ingestion transactions.
type NotebookChunkInput struct {
	ID         string
	ParentID   string
	Text       string
	TokenCount int
	PageNum    int
}

// NotebookTopicIngestionGroup contains topic-scoped parent/chunk rows for one notebook ingestion run.
type NotebookTopicIngestionGroup struct {
	TopicID string
	Parents []NotebookParentInput
	Chunks  []NotebookChunkInput
}

type sqlExecer interface {
	Exec(query string, args ...interface{}) (sql.Result, error)
}

func insertParentRow(exec sqlExecer, topicID string, parent NotebookParentInput) error {
	_, err := exec.Exec(`
		INSERT INTO parents (id, topic_id, heading, order_index, content_text)
		VALUES (?, ?, ?, ?, ?)
	`, parent.ID, topicID, parent.Heading, parent.OrderIndex, parent.Content)
	return err
}

func insertChunkRow(exec sqlExecer, topicID string, chunk NotebookChunkInput) error {
	_, err := exec.Exec(`
		INSERT INTO chunks (id, topic_id, parent_id, chunk_text, page_num, token_count, importance_score, weakness_score)
		VALUES (?, ?, ?, ?, ?, ?, 0, 0)
	`, chunk.ID, topicID, chunk.ParentID, chunk.Text, chunk.PageNum, chunk.TokenCount)
	return err
}

// CreateParentSection inserts a parent section row.
func CreateParentSection(id, topicID, heading string, orderIndex int, content string) error {
	return insertParentRow(conn, topicID, NotebookParentInput{
		ID:         id,
		Heading:    heading,
		Content:    content,
		OrderIndex: orderIndex,
	})
}

// CreateChunk inserts a chunk row.
func CreateChunk(id, topicID, parentID, text string, tokenCount int, pageNum int) error {
	return insertChunkRow(conn, topicID, NotebookChunkInput{
		ID:         id,
		ParentID:   parentID,
		Text:       text,
		PageNum:    pageNum,
		TokenCount: tokenCount,
	})
}

// UpdateNotebookStatus updates the notebook ingestion status.
func UpdateNotebookStatus(notebookID string, status string) error {
	result, err := conn.Exec(`
		UPDATE notebooks
		SET status = ?
		WHERE id = ?
	`, status, notebookID)
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

	return nil
}

// UpdateNotebookTopic updates the notebook topic link used by UI-level notebook metadata.
func UpdateNotebookTopic(notebookID string, topicID string) error {
	validatedNotebookID, err := validateID(notebookID, "notebook id")
	if err != nil {
		return err
	}

	cleanedTopicID := strings.TrimSpace(topicID)
	if cleanedTopicID == "" {
		result, err := conn.Exec(`
			UPDATE notebooks
			SET topic_id = NULL
			WHERE id = ?
		`, validatedNotebookID)
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

		return nil
	}

	validatedTopicID, err := validateID(cleanedTopicID, "topic id")
	if err != nil {
		return err
	}

	result, err := conn.Exec(`
		UPDATE notebooks
		SET topic_id = ?
		WHERE id = ?
	`, validatedTopicID, validatedNotebookID)
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

	return nil
}

// UpdateNotebookTitle updates notebook display title.
func UpdateNotebookTitle(notebookID string, title string) error {
	notebookID = strings.TrimSpace(notebookID)
	title = strings.TrimSpace(title)
	if notebookID == "" {
		return fmt.Errorf("notebook id is required")
	}
	if title == "" {
		return fmt.Errorf("title is required")
	}

	result, err := conn.Exec(`
		UPDATE notebooks
		SET title = ?
		WHERE id = ?
	`, title, notebookID)
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

	return nil
}

// IngestNotebookContent performs a transactional relational commit for notebook sections/chunks.
func IngestNotebookContent(notebookID string, topicID string, parents []NotebookParentInput, chunks []NotebookChunkInput) error {
	notebookID = strings.TrimSpace(notebookID)
	topicID = strings.TrimSpace(topicID)
	if notebookID == "" {
		return fmt.Errorf("notebook id is required")
	}
	if topicID == "" {
		return fmt.Errorf("topic id is required")
	}

	group := NotebookTopicIngestionGroup{
		TopicID: topicID,
		Parents: parents,
		Chunks:  chunks,
	}

	// Route legacy single-topic ingestion through the multi-topic transaction path.
	return IngestNotebookContentByTopic(notebookID, []NotebookTopicIngestionGroup{group})
}

// IngestNotebookContentByTopic ingests notebook content into multiple topic buckets in one transaction.
func IngestNotebookContentByTopic(notebookID string, groups []NotebookTopicIngestionGroup) error {
	notebookID = strings.TrimSpace(notebookID)
	if notebookID == "" {
		return fmt.Errorf("notebook id is required")
	}
	if len(groups) == 0 {
		return fmt.Errorf("at least one ingestion group is required")
	}
	normalizedGroups := make([]NotebookTopicIngestionGroup, 0, len(groups))
	for _, group := range groups {
		group.TopicID = strings.TrimSpace(group.TopicID)
		if group.TopicID == "" {
			return fmt.Errorf("topic id is required for every ingestion group")
		}
		normalizedGroups = append(normalizedGroups, group)
	}
	return ingestNotebookContentByTopicRepo(notebookID, normalizedGroups)
}

// GetNotebooks retrieves all notebooks, optionally filtered by topic
func GetNotebooks(topicID string) ([]models.Notebook, error) {
	query := "SELECT id, title, file_path, file_type, COALESCE(topic_id, ''), COALESCE(status, 'uploaded'), page_count, chunk_count, uploaded_at FROM notebooks"
	args := []interface{}{}

	if topicID != "" {
		query += " WHERE topic_id = ?"
		args = append(args, topicID)
	}
	query += " ORDER BY uploaded_at DESC"

	rows, err := conn.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = rows.Close()
	}()

	var notebooks []models.Notebook
	for rows.Next() {
		var nb models.Notebook
		if err := rows.Scan(&nb.ID, &nb.Title, &nb.FilePath, &nb.FileType, &nb.TopicID, &nb.Status, &nb.PageCount, &nb.ChunkCount, &nb.UploadedAt); err != nil {
			return nil, err
		}
		notebooks = append(notebooks, nb)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return notebooks, nil
}

// GetNotebookByID retrieves a single notebook by ID
func GetNotebookByID(notebookID string) (*models.Notebook, error) {
	var nb models.Notebook
	err := conn.QueryRow(`
		SELECT id, title, file_path, file_type, COALESCE(topic_id, ''), COALESCE(status, 'uploaded'), page_count, chunk_count, uploaded_at
		FROM notebooks
		WHERE id = ?
	`, notebookID).Scan(&nb.ID, &nb.Title, &nb.FilePath, &nb.FileType, &nb.TopicID, &nb.Status, &nb.PageCount, &nb.ChunkCount, &nb.UploadedAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &nb, nil
}

// LinkChunksToNotebook associates chunks with a notebook
func LinkChunksToNotebook(notebookID string, chunkIDs []string) error {
	validatedNotebookID, err := validateID(notebookID, "notebook id")
	if err != nil {
		return err
	}

	tx, err := conn.Begin()
	if err != nil {
		return err
	}
	defer func() {
		_ = tx.Rollback()
	}()

	for _, chunkID := range chunkIDs {
		validatedChunkID, err := validateID(chunkID, "chunk id")
		if err != nil {
			return err
		}

		id := "nb-chunk-" + validatedNotebookID + "-" + validatedChunkID // simple composite ID
		_, err = tx.Exec(`
			INSERT INTO notebook_chunks (id, notebook_id, chunk_id)
			VALUES (?, ?, ?)
		`, id, validatedNotebookID, validatedChunkID)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

// UpdateNotebookChunkCount updates the chunk count for a notebook
func UpdateNotebookChunkCount(notebookID string, count int) error {
	result, err := conn.Exec(`
		UPDATE notebooks
		SET chunk_count = ?
		WHERE id = ?
	`, count, notebookID)
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

	return nil
}

// DeleteNotebook removes a notebook and its chunk links
func DeleteNotebook(notebookID string) error {
	notebookID = strings.TrimSpace(notebookID)
	if notebookID == "" {
		return fmt.Errorf("notebook id is required")
	}
	return deleteNotebookRepo(notebookID)
}
