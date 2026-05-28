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
		INSERT INTO notebooks (id, title, file_path, file_type, topic_id, status, indexing_status, page_count)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, validatedID, title, filePath, fileType, topicValue, "uploaded", "PENDING", pageCount)
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

// UpdateNotebookIndexingStatus updates the notebook semantic indexing status.
func UpdateNotebookIndexingStatus(notebookID string, status string) error {
	result, err := conn.Exec(`
		UPDATE notebooks
		SET indexing_status = ?
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

// EnsureNotebookTopic links a topic to a notebook if not already linked.
func EnsureNotebookTopic(notebookID, topicID string) error {
	notebookID = strings.TrimSpace(notebookID)
	topicID = strings.TrimSpace(topicID)
	if notebookID == "" || topicID == "" {
		return fmt.Errorf("notebook id and topic id are required")
	}
	_, err := conn.Exec(`
		INSERT OR IGNORE INTO notebook_topics (notebook_id, topic_id)
		VALUES (?, ?)
	`, notebookID, topicID)
	return err
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
	query := "SELECT id, title, file_path, file_type, COALESCE(topic_id, ''), COALESCE(status, 'uploaded'), COALESCE(indexing_status, 'PENDING'), page_count, chunk_count, COALESCE(priority, 5), uploaded_at FROM notebooks"
	args := []interface{}{}

	if topicID != "" {
		query += `
			WHERE topic_id = ?
			   OR EXISTS (
				SELECT 1
				FROM notebook_topics nt
				WHERE nt.notebook_id = notebooks.id
				  AND nt.topic_id = ?
			)
		`
		args = append(args, topicID, topicID)
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
		if err := rows.Scan(&nb.ID, &nb.Title, &nb.FilePath, &nb.FileType, &nb.TopicID, &nb.Status, &nb.IndexingStatus, &nb.PageCount, &nb.ChunkCount, &nb.Priority, &nb.UploadedAt); err != nil {
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
		SELECT id, title, file_path, file_type, COALESCE(topic_id, ''), COALESCE(status, 'uploaded'), COALESCE(indexing_status, 'PENDING'), page_count, chunk_count, COALESCE(priority, 5), uploaded_at
		FROM notebooks
		WHERE id = ?
	`, notebookID).Scan(&nb.ID, &nb.Title, &nb.FilePath, &nb.FileType, &nb.TopicID, &nb.Status, &nb.IndexingStatus, &nb.PageCount, &nb.ChunkCount, &nb.Priority, &nb.UploadedAt)

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

	return withTx(func(tx *sql.Tx) error {
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
		return nil
	})
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

// UpdateNotebookPriority updates the notebook priority
func UpdateNotebookPriority(notebookID string, priority int) error {
	result, err := conn.Exec(`
		UPDATE notebooks
		SET priority = ?
		WHERE id = ?
	`, priority, notebookID)
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

// GetNotebookSyllabusDraft retrieves the persisted syllabus draft JSON for a notebook
func GetNotebookSyllabusDraft(notebookID string) (string, error) {
	notebookID = strings.TrimSpace(notebookID)
	if notebookID == "" {
		return "", fmt.Errorf("notebook id is required")
	}

	var draftJSON sql.NullString
	err := conn.QueryRow(`
		SELECT syllabus_draft_json
		FROM notebooks
		WHERE id = ?
	`, notebookID).Scan(&draftJSON)

	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", err
	}

	if !draftJSON.Valid {
		return "", nil
	}

	return draftJSON.String, nil
}

// UpdateNotebookSyllabusDraft persists the syllabus draft JSON for a notebook
func UpdateNotebookSyllabusDraft(notebookID, draftJSON string) error {
	notebookID = strings.TrimSpace(notebookID)
	if notebookID == "" {
		return fmt.Errorf("notebook id is required")
	}

	result, err := conn.Exec(`
		UPDATE notebooks
		SET syllabus_draft_json = ?
		WHERE id = ?
	`, draftJSON, notebookID)
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

// GetNotebookTopicTree returns notebooks with their discovered topics derived from linked chunks.
func GetNotebookTopicTree() ([]models.NotebookTopicTreeNode, error) {
	rows, err := conn.Query(`
		SELECT
			n.id,
			n.title,
			COALESCE(t.id, ''),
			COALESCE(t.title, '')
		FROM notebooks n
		LEFT JOIN notebook_chunks nc ON nc.notebook_id = n.id
		LEFT JOIN chunks c ON c.id = nc.chunk_id
		LEFT JOIN topics t ON t.id = c.topic_id
		ORDER BY n.uploaded_at DESC, t.title ASC, t.id ASC
	`)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = rows.Close()
	}()

	tree := make([]models.NotebookTopicTreeNode, 0)
	notebookIndex := make(map[string]int)
	seenTopics := make(map[string]map[string]struct{})

	for rows.Next() {
		var notebookID string
		var notebookTitle string
		var topicID string
		var topicTitle string

		if err := rows.Scan(&notebookID, &notebookTitle, &topicID, &topicTitle); err != nil {
			return nil, err
		}

		idx, exists := notebookIndex[notebookID]
		if !exists {
			tree = append(tree, models.NotebookTopicTreeNode{
				NotebookID: notebookID,
				Title:      notebookTitle,
				Topics:     []models.NotebookTopicTreeTopic{},
			})
			idx = len(tree) - 1
			notebookIndex[notebookID] = idx
			seenTopics[notebookID] = make(map[string]struct{})
		}

		if topicID == "" || topicTitle == "" {
			continue
		}

		if _, duplicate := seenTopics[notebookID][topicID]; duplicate {
			continue
		}

		tree[idx].Topics = append(tree[idx].Topics, models.NotebookTopicTreeTopic{
			TopicID: topicID,
			Title:   topicTitle,
		})
		seenTopics[notebookID][topicID] = struct{}{}
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return tree, nil
}

func ingestNotebookContentByTopicRepo(notebookID string, groups []NotebookTopicIngestionGroup) error {
	if notebookID == "" {
		return fmt.Errorf("notebook id is required")
	}
	if len(groups) == 0 {
		return fmt.Errorf("at least one topic group is required")
	}

	return withTx(func(tx *sql.Tx) error {
		if _, err := tx.Exec(`
			UPDATE notebooks
			SET status = ?, chunk_count = 0
			WHERE id = ?
		`, "processing", notebookID); err != nil {
			return err
		}

		if _, err := tx.Exec("DELETE FROM notebook_chunks WHERE notebook_id = ?", notebookID); err != nil {
			return err
		}

		parentPrefix := fmt.Sprintf("nbp_%s_%%", notebookID)
		chunkPrefix := fmt.Sprintf("nbc_%s_%%", notebookID)

		if _, err := tx.Exec("DELETE FROM chunks WHERE id LIKE ?", chunkPrefix); err != nil {
			return err
		}
		if _, err := tx.Exec("DELETE FROM parents WHERE id LIKE ?", parentPrefix); err != nil {
			return err
		}

		totalChunks := 0
		for _, group := range groups {
			if group.TopicID == "" {
				return fmt.Errorf("topic id is required for each ingestion group")
			}

			for _, parent := range group.Parents {
				if err := insertParentRowRepo(tx, group.TopicID, parent); err != nil {
					return err
				}
			}

			for _, chunk := range group.Chunks {
				if err := insertChunkRowRepo(tx, group.TopicID, chunk); err != nil {
					return err
				}

				if err := linkNotebookChunkRowRepo(tx, notebookID, chunk); err != nil {
					return err
				}

				totalChunks++
			}
		}

		if _, err := tx.Exec(`
			UPDATE notebooks
			SET chunk_count = ?, status = ?, topic_id = ?
			WHERE id = ?
		`, totalChunks, "chunked", groups[0].TopicID, notebookID); err != nil {
			return err
		}
		return nil
	})
}

func deleteNotebookRepo(notebookID string) error {
	return withTx(func(tx *sql.Tx) error {
		// Delete reading_progress for tasks associated with this notebook first
		// (reading_progress references study_queue, which references notebooks)
		if _, err := tx.Exec(`
			DELETE FROM reading_progress
			WHERE task_id IN (
				SELECT id FROM study_queue WHERE notebook_id = ?
			)
		`, notebookID); err != nil {
			return err
		}

		// Delete quiz_attempts for tasks associated with this notebook
		if _, err := tx.Exec(`
			DELETE FROM quiz_attempts
			WHERE task_id IN (
				SELECT id FROM study_queue WHERE notebook_id = ?
			)
		`, notebookID); err != nil {
			return err
		}

		// Delete study_queue entries for this notebook (foreign key to notebooks)
		if _, err := tx.Exec(`
			DELETE FROM study_queue WHERE notebook_id = ?
		`, notebookID); err != nil {
			return err
		}

		parentIDs := make(map[string]struct{})
		chunkIDs := make([]string, 0)
		parentRows, err := tx.Query(`
			SELECT DISTINCT c.parent_id, c.id
			FROM chunks c
			WHERE c.topic_id IN (
				SELECT topic_id FROM notebook_topics WHERE notebook_id = ?
				UNION
				SELECT id FROM topics WHERE id LIKE ?
			)
		`, notebookID, "nb-"+notebookID+"-%")
		if err != nil {
			return err
		}

		for parentRows.Next() {
			var parentID string
			var chunkID string
			if scanErr := parentRows.Scan(&parentID, &chunkID); scanErr != nil {
				_ = parentRows.Close()
				return scanErr
			}
			parentIDs[parentID] = struct{}{}
			chunkIDs = append(chunkIDs, chunkID)
		}
		if rowsErr := parentRows.Err(); rowsErr != nil {
			_ = parentRows.Close()
			return rowsErr
		}
		_ = parentRows.Close()

		hasChunkVectors := false
		if exists, tableErr := doesTableExistTxRepo(tx, "chunk_vectors"); tableErr != nil {
			return tableErr
		} else {
			hasChunkVectors = exists
		}

		if hasChunkVectors {
			if _, delVecErr := tx.Exec(`
				DELETE FROM chunk_vectors
				WHERE rowid IN (
					SELECT c.rowid
					FROM chunks c
					JOIN notebook_chunks nc ON nc.chunk_id = c.id
					WHERE nc.notebook_id = ?
				)
			`, notebookID); delVecErr != nil {
				return delVecErr
			}
		}

		// Delete notebook_chunks entries (foreign key to notebooks)
		if _, err := tx.Exec(`
			DELETE FROM notebook_chunks WHERE notebook_id = ?
		`, notebookID); err != nil {
			return err
		}

		// Bulk delete chunks using IN clause for better performance
		if len(chunkIDs) > 0 {
			placeholders := make([]string, len(chunkIDs))
			args := make([]interface{}, len(chunkIDs))
			for i, chunkID := range chunkIDs {
				placeholders[i] = "?"
				args[i] = chunkID
			}

			query := fmt.Sprintf(`DELETE FROM chunks WHERE id IN (%s)`, strings.Join(placeholders, ","))
			if _, delChunkErr := tx.Exec(query, args...); delChunkErr != nil {
				return delChunkErr
			}
		}

		_, err = tx.Exec("DELETE FROM notebooks WHERE id = ?", notebookID)
		if err != nil {
			return err
		}

		for parentID := range parentIDs {
			var count int
			if countErr := tx.QueryRow(`
				SELECT COUNT(*) FROM chunks WHERE parent_id = ?
			`, parentID).Scan(&count); countErr != nil {
				return countErr
			}
			if count == 0 {
				if _, delParentErr := tx.Exec(`
					DELETE FROM parents WHERE id = ?
				`, parentID); delParentErr != nil {
					return delParentErr
				}
			}
		}

		topicRows, err := tx.Query(`
			SELECT id
			FROM topics
			WHERE id LIKE ?
		`, "nb-"+notebookID+"-%")
		if err != nil {
			return err
		}

		autoTopicIDs := make([]string, 0)
		for topicRows.Next() {
			var topicID string
			if scanErr := topicRows.Scan(&topicID); scanErr != nil {
				_ = topicRows.Close()
				return scanErr
			}
			autoTopicIDs = append(autoTopicIDs, topicID)
		}
		if rowsErr := topicRows.Err(); rowsErr != nil {
			_ = topicRows.Close()
			return rowsErr
		}
		_ = topicRows.Close()

		for _, topicID := range autoTopicIDs {
			var parentCount int
			if parentCountErr := tx.QueryRow(`
				SELECT COUNT(*) FROM parents WHERE topic_id = ?
			`, topicID).Scan(&parentCount); parentCountErr != nil {
				return parentCountErr
			}

			var chunkCount int
			if chunkCountErr := tx.QueryRow(`
				SELECT COUNT(*) FROM chunks WHERE topic_id = ?
			`, topicID).Scan(&chunkCount); chunkCountErr != nil {
				return chunkCountErr
			}

			if parentCount == 0 && chunkCount == 0 {
				if _, delProgressErr := tx.Exec(`
					DELETE FROM topic_progress WHERE topic_id = ?
				`, topicID); delProgressErr != nil {
					return delProgressErr
				}
				if _, delTopicErr := tx.Exec(`
					DELETE FROM topics WHERE id = ?
				`, topicID); delTopicErr != nil {
					return delTopicErr
				}
			}
		}
		return nil
	})
}

func insertParentRowRepo(exec sqlExecer, topicID string, parent NotebookParentInput) error {
	_, err := exec.Exec(`
		INSERT INTO parents (id, topic_id, heading, order_index, content_text)
		VALUES (?, ?, ?, ?, ?)
	`, parent.ID, topicID, parent.Heading, parent.OrderIndex, parent.Content)
	return err
}

func insertChunkRowRepo(exec sqlExecer, topicID string, chunk NotebookChunkInput) error {
	_, err := exec.Exec(`
		INSERT INTO chunks (id, topic_id, parent_id, chunk_text, page_num, token_count, importance_score, weakness_score)
		VALUES (?, ?, ?, ?, ?, ?, 0, 0)
	`, chunk.ID, topicID, chunk.ParentID, chunk.Text, chunk.PageNum, chunk.TokenCount)
	return err
}

func linkNotebookChunkRowRepo(exec sqlExecer, notebookID string, chunk NotebookChunkInput) error {
	linkID := "nb-chunk-" + notebookID + "-" + chunk.ID
	_, err := exec.Exec(`
		INSERT INTO notebook_chunks (id, notebook_id, chunk_id, page_num)
		VALUES (?, ?, ?, ?)
	`, linkID, notebookID, chunk.ID, chunk.PageNum)
	return err
}

func doesTableExistTxRepo(tx *sql.Tx, tableName string) (bool, error) {
	var count int
	err := tx.QueryRow(`
		SELECT COUNT(1)
		FROM sqlite_master
		WHERE type = 'table' AND name = ?
	`, tableName).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// GetChunkTextByNotebookPageRange returns the concatenated chunk_text for all chunks
// belonging to the given notebook_id with page_num BETWEEN startPage AND endPage.
// The join goes through notebook_chunks (notebook_id, chunk_id, page_num) → chunks (chunk_text).
func GetChunkTextByNotebookPageRange(notebookID string, startPage, endPage int) (string, error) {
	notebookID = strings.TrimSpace(notebookID)
	if notebookID == "" {
		return "", fmt.Errorf("notebook id is required")
	}
	if startPage <= 0 || endPage <= 0 || endPage < startPage {
		return "", fmt.Errorf("invalid page range: start=%d end=%d", startPage, endPage)
	}

	rows, err := conn.Query(`
		SELECT c.chunk_text
		FROM notebook_chunks nc
		JOIN chunks c ON c.id = nc.chunk_id
		WHERE nc.notebook_id = ?
		  AND nc.page_num BETWEEN ? AND ?
		ORDER BY nc.page_num ASC, nc.chunk_id ASC
	`, notebookID, startPage, endPage)
	if err != nil {
		return "", fmt.Errorf("page-range query failed: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var b strings.Builder
	for rows.Next() {
		var text string
		if err := rows.Scan(&text); err != nil {
			return "", fmt.Errorf("scan chunk_text: %w", err)
		}
		b.WriteString(text)
		b.WriteByte('\n')
	}
	if err := rows.Err(); err != nil {
		return "", fmt.Errorf("row iteration error: %w", err)
	}
	return strings.TrimSpace(b.String()), nil
}

// GetChunksWithContextByNotebookPageRange returns structured chunks for a notebook page range.
func GetChunksWithContextByNotebookPageRange(notebookID string, startPage, endPage int) ([]models.ChunkWithContext, error) {
	notebookID = strings.TrimSpace(notebookID)
	if notebookID == "" {
		return nil, fmt.Errorf("notebook id is required")
	}
	if startPage <= 0 || endPage <= 0 || endPage < startPage {
		return nil, fmt.Errorf("invalid page range: start=%d end=%d", startPage, endPage)
	}

	rows, err := conn.Query(`
		SELECT c.id, c.parent_id, nc.page_num, c.chunk_text
		FROM notebook_chunks nc
		JOIN chunks c ON c.id = nc.chunk_id
		WHERE nc.notebook_id = ?
		  AND nc.page_num BETWEEN ? AND ?
		ORDER BY nc.page_num ASC, nc.chunk_id ASC
	`, notebookID, startPage, endPage)
	if err != nil {
		return nil, fmt.Errorf("page-range structured query failed: %w", err)
	}
	defer func() { _ = rows.Close() }()

	chunks := make([]models.ChunkWithContext, 0)
	for rows.Next() {
		var chunk models.ChunkWithContext
		if err := rows.Scan(&chunk.ChunkID, &chunk.ParentID, &chunk.PageNum, &chunk.Text); err != nil {
			return nil, fmt.Errorf("scan structured chunk: %w", err)
		}
		chunks = append(chunks, chunk)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("row iteration error: %w", err)
	}
	return chunks, nil
}

// GetNotebookPageCount returns the maximum page_num stored in notebook_chunks for a notebook.
func GetNotebookPageCount(notebookID string) (int, error) {
	notebookID = strings.TrimSpace(notebookID)
	if notebookID == "" {
		return 0, fmt.Errorf("notebook id is required")
	}
	var maxPage int
	err := conn.QueryRow(`
		SELECT COALESCE(MAX(page_num), 0)
		FROM notebook_chunks
		WHERE notebook_id = ?
	`, notebookID).Scan(&maxPage)
	return maxPage, err
}
