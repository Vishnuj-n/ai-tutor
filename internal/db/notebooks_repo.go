package db

import (
	"database/sql"
	"fmt"
	"strings"

	"ai-tutor/internal/models"
)

// CreateNotebook saves a notebook record to the database
func (r *Repository) CreateNotebook(id, title, filePath, fileType, topicID string, pageCount int) error {
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

	_, err = r.db.Exec(`
		INSERT INTO notebooks (id, title, file_path, file_type, topic_id, status, indexing_status, page_count)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, validatedID, title, filePath, fileType, topicValue, "uploaded", "PENDING", pageCount)
	return err
}

// NotebookChunkInput is a chunk row used by notebook ingestion transactions.
type NotebookChunkInput struct {
	ID         string
	Text       string
	TokenCount int
	PageNum    int
}

// NotebookTopicIngestionGroup contains topic-scoped chunk rows for one notebook ingestion run.
type NotebookTopicIngestionGroup struct {
	TopicID string
	Chunks  []NotebookChunkInput
}

type sqlExecer interface {
	Exec(query string, args ...interface{}) (sql.Result, error)
}

func insertChunkRow(exec sqlExecer, topicID string, chunk NotebookChunkInput) error {
	_, err := exec.Exec(`
		INSERT INTO chunks (id, topic_id, chunk_text, page_num, token_count, importance_score, weakness_score)
		VALUES (?, ?, ?, ?, ?, 0, 0)
	`, chunk.ID, topicID, chunk.Text, chunk.PageNum, chunk.TokenCount)
	return err
}

// CreateChunk inserts a chunk row.
func (r *Repository) CreateChunk(id, topicID, text string, tokenCount int, pageNum int) error {
	return insertChunkRow(r.db, topicID, NotebookChunkInput{
		ID:         id,
		Text:       text,
		PageNum:    pageNum,
		TokenCount: tokenCount,
	})
}

// UpdateNotebookStatus updates the notebook ingestion status.
func (r *Repository) UpdateNotebookStatus(notebookID string, status string) error {
	result, err := r.db.Exec(`
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
func (r *Repository) UpdateNotebookIndexingStatus(notebookID string, status string) error {
	result, err := r.db.Exec(`
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
func (r *Repository) UpdateNotebookTopic(notebookID string, topicID string) error {
	validatedNotebookID, err := validateID(notebookID, "notebook id")
	if err != nil {
		return err
	}

	cleanedTopicID := strings.TrimSpace(topicID)
	if cleanedTopicID == "" {
		result, err := r.db.Exec(`
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

	result, err := r.db.Exec(`
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
func (r *Repository) UpdateNotebookTitle(notebookID string, title string) error {
	notebookID = strings.TrimSpace(notebookID)
	title = strings.TrimSpace(title)
	if notebookID == "" {
		return fmt.Errorf("notebook id is required")
	}
	if title == "" {
		return fmt.Errorf("title is required")
	}

	result, err := r.db.Exec(`
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
func (r *Repository) EnsureNotebookTopic(notebookID, topicID string) error {
	notebookID = strings.TrimSpace(notebookID)
	topicID = strings.TrimSpace(topicID)
	if notebookID == "" || topicID == "" {
		return fmt.Errorf("notebook id and topic id are required")
	}
	_, err := r.db.Exec(`
		INSERT OR IGNORE INTO notebook_topics (notebook_id, topic_id)
		VALUES (?, ?)
	`, notebookID, topicID)
	return err
}

// LinkNotebookTopics associates a set of topics with a notebook by replacing any existing links.
func (r *Repository) LinkNotebookTopics(notebookID string, topicIDs []string) error {
	notebookID = strings.TrimSpace(notebookID)
	if notebookID == "" {
		return fmt.Errorf("notebook id is required")
	}

	return r.withTx(func(tx *sql.Tx) error {
		// Delete existing links for this notebook
		if _, err := tx.Exec("DELETE FROM notebook_topics WHERE notebook_id = ?", notebookID); err != nil {
			return err
		}

		// Insert new links
		for _, topicID := range topicIDs {
			topicID = strings.TrimSpace(topicID)
			if topicID == "" {
				return fmt.Errorf("topic id cannot be empty")
			}
			if _, err := tx.Exec(`
				INSERT INTO notebook_topics (notebook_id, topic_id)
				VALUES (?, ?)
			`, notebookID, topicID); err != nil {
				return err
			}
		}
		return nil
	})
}

// IngestNotebookContentByTopic ingests notebook content into multiple topic buckets in one transaction.
func (r *Repository) IngestNotebookContentByTopic(notebookID string, groups []NotebookTopicIngestionGroup) error {
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

	return r.withTx(func(tx *sql.Tx) error {
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

		chunkPrefix := fmt.Sprintf("nbc_%s_%%", notebookID)

		if _, err := tx.Exec("DELETE FROM chunks WHERE id LIKE ?", chunkPrefix); err != nil {
			return err
		}

		totalChunks := 0
		for _, group := range normalizedGroups {
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
		`, totalChunks, "chunked", normalizedGroups[0].TopicID, notebookID); err != nil {
			return err
		}
		return nil
	})
}

// GetNotebooks retrieves all notebooks, optionally filtered by topic and profile.
// When profileID is empty, returns all notebooks (backward compatible).
// When profileID is set, returns only notebooks belonging to that profile or unassigned notebooks.
func (r *Repository) GetNotebooks(topicID, profileID string) ([]models.Notebook, error) {
	query := "SELECT id, title, file_path, file_type, COALESCE(topic_id, ''), COALESCE(status, 'uploaded'), COALESCE(indexing_status, 'PENDING'), page_count, chunk_count, COALESCE(priority, 5), exam_deadline, uploaded_at, COALESCE(profile_id, ''), COALESCE(study_status, 'dormant') FROM notebooks"
	args := []interface{}{}
	whereClause := ""

	if topicID != "" {
		if whereClause != "" {
			whereClause += " AND "
		}
		whereClause += `(topic_id = ? OR EXISTS (SELECT 1 FROM notebook_topics nt WHERE nt.notebook_id = notebooks.id AND nt.topic_id = ?))`
		args = append(args, topicID, topicID)
	}

	// Profile isolation: filter by profile if set
	if profileID != "" {
		if whereClause != "" {
			whereClause += " AND "
		}
		// Include notebooks belonging to this profile OR unassigned notebooks (NULL profile_id)
		whereClause += `(profile_id = ? OR profile_id IS NULL OR profile_id = '')`
		args = append(args, profileID)
	}

	if whereClause != "" {
		query += " WHERE " + whereClause
	}
	query += " ORDER BY uploaded_at DESC"

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = rows.Close()
	}()

	var notebooks []models.Notebook
	for rows.Next() {
		var nb models.Notebook
		if err := rows.Scan(&nb.ID, &nb.Title, &nb.FilePath, &nb.FileType, &nb.TopicID, &nb.Status, &nb.IndexingStatus, &nb.PageCount, &nb.ChunkCount, &nb.Priority, &nb.ExamDeadline, &nb.UploadedAt, &nb.ProfileID, &nb.StudyStatus); err != nil {
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
func (r *Repository) GetNotebookByID(notebookID string) (*models.Notebook, error) {
	var nb models.Notebook
	err := r.db.QueryRow(`
		SELECT id, title, file_path, file_type, COALESCE(topic_id, ''), COALESCE(status, 'uploaded'), COALESCE(indexing_status, 'PENDING'), page_count, chunk_count, COALESCE(priority, 5), exam_deadline, uploaded_at, COALESCE(profile_id, ''), COALESCE(study_status, 'dormant')
		FROM notebooks
		WHERE id = ?
	`, notebookID).Scan(&nb.ID, &nb.Title, &nb.FilePath, &nb.FileType, &nb.TopicID, &nb.Status, &nb.IndexingStatus, &nb.PageCount, &nb.ChunkCount, &nb.Priority, &nb.ExamDeadline, &nb.UploadedAt, &nb.ProfileID, &nb.StudyStatus)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &nb, nil
}

// LinkChunksToNotebook associates chunks with a notebook
func (r *Repository) LinkChunksToNotebook(notebookID string, chunkIDs []string) error {
	validatedNotebookID, err := validateID(notebookID, "notebook id")
	if err != nil {
		return err
	}

	return r.withTx(func(tx *sql.Tx) error {
		for _, chunkID := range chunkIDs {
			validatedChunkID, err := validateID(chunkID, "chunk id")
			if err != nil {
				return err
			}

			id := "nb-chunk-" + validatedNotebookID + "-" + validatedChunkID // simple composite ID
			_, err = tx.Exec(`
				INSERT INTO notebook_chunks (id, notebook_id, chunk_id, page_num)
				SELECT ?, ?, ?, COALESCE(page_num, 0) FROM chunks WHERE id = ?
			`, id, validatedNotebookID, validatedChunkID, validatedChunkID)
			if err != nil {
				return err
			}
		}
		return nil
	})
}

// UpdateNotebookChunkCount updates the chunk count for a notebook
func (r *Repository) UpdateNotebookChunkCount(notebookID string, count int) error {
	result, err := r.db.Exec(`
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
func (r *Repository) UpdateNotebookPriority(notebookID string, priority int) error {
	result, err := r.db.Exec(`
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
func (r *Repository) GetNotebookSyllabusDraft(notebookID string) (string, error) {
	notebookID = strings.TrimSpace(notebookID)
	if notebookID == "" {
		return "", fmt.Errorf("notebook id is required")
	}

	var draftJSON sql.NullString
	err := r.db.QueryRow(`
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
func (r *Repository) UpdateNotebookSyllabusDraft(notebookID, draftJSON string) error {
	notebookID = strings.TrimSpace(notebookID)
	if notebookID == "" {
		return fmt.Errorf("notebook id is required")
	}

	result, err := r.db.Exec(`
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
func (r *Repository) DeleteNotebook(notebookID string) error {
	notebookID = strings.TrimSpace(notebookID)
	if notebookID == "" {
		return fmt.Errorf("notebook id is required")
	}
	return r.withTx(func(tx *sql.Tx) error {
		// Delete reading_progress for tasks associated with this notebook first
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

		// Delete study_queue entries for this notebook
		if _, err := tx.Exec(`
			DELETE FROM study_queue WHERE notebook_id = ?
		`, notebookID); err != nil {
			return err
		}

		chunkIDs := make([]string, 0)
		chunkRows, err := tx.Query(`
			SELECT DISTINCT c.id
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

		for chunkRows.Next() {
			var chunkID string
			if scanErr := chunkRows.Scan(&chunkID); scanErr != nil {
				_ = chunkRows.Close()
				return scanErr
			}
			chunkIDs = append(chunkIDs, chunkID)
		}
		if rowsErr := chunkRows.Err(); rowsErr != nil {
			_ = chunkRows.Close()
			return rowsErr
		}
		_ = chunkRows.Close()

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

		// Delete notebook_chunks entries
		if _, err := tx.Exec(`
			DELETE FROM notebook_chunks WHERE notebook_id = ?
		`, notebookID); err != nil {
			return err
		}

		// Bulk delete chunks
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
			var chunkCount int
			if chunkCountErr := tx.QueryRow(`
				SELECT COUNT(*) FROM chunks WHERE topic_id = ?
			`, topicID).Scan(&chunkCount); chunkCountErr != nil {
				return chunkCountErr
			}

			if chunkCount == 0 {
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

// GetNotebookTopicTree returns notebooks with their discovered topics derived from linked chunks, optionally filtered by profile.
func (r *Repository) GetNotebookTopicTree(profileID string) ([]models.NotebookTopicTreeNode, error) {
	query := `
		SELECT
			n.id,
			n.title,
			COALESCE(t.id, ''),
			COALESCE(t.title, '')
		FROM notebooks n
		LEFT JOIN notebook_chunks nc ON nc.notebook_id = n.id
		LEFT JOIN chunks c ON c.id = nc.chunk_id
		LEFT JOIN topics t ON t.id = c.topic_id
	`
	var args []interface{}
	if profileID != "" {
		query += ` WHERE (n.profile_id = ? OR n.profile_id IS NULL OR n.profile_id = '') `
		args = append(args, profileID)
	}
	query += ` ORDER BY n.uploaded_at DESC, t.title ASC, t.id ASC `

	rows, err := r.db.Query(query, args...)
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

func insertChunkRowRepo(exec sqlExecer, topicID string, chunk NotebookChunkInput) error {
	_, err := exec.Exec(`
		INSERT INTO chunks (id, topic_id, chunk_text, page_num, token_count, importance_score, weakness_score)
		VALUES (?, ?, ?, ?, ?, 0, 0)
	`, chunk.ID, topicID, chunk.Text, chunk.PageNum, chunk.TokenCount)
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
		SELECT count(1)
		FROM sqlite_master
		WHERE type = 'table' AND name = ?
	`, tableName).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// GetChunksWithContextByNotebookPageRange returns structured chunks for a notebook page range.
func (r *Repository) GetChunksWithContextByNotebookPageRange(notebookID string, startPage, endPage int) ([]models.ChunkWithContext, error) {
	notebookID = strings.TrimSpace(notebookID)
	if notebookID == "" {
		return nil, fmt.Errorf("notebook id is required")
	}
	if startPage <= 0 || endPage <= 0 || endPage < startPage {
		return nil, fmt.Errorf("invalid page range: start=%d end=%d", startPage, endPage)
	}

	rows, err := r.db.Query(`
		SELECT c.id, nc.page_num, c.chunk_text
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
		if err := rows.Scan(&chunk.ChunkID, &chunk.PageNum, &chunk.Text); err != nil {
			return nil, fmt.Errorf("scan structured chunk: %w", err)
		}
		chunks = append(chunks, chunk)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("row iteration error: %w", err)
	}
	return chunks, nil
}

// AssignNotebookToProfile sets the profile ID for a notebook.
func (r *Repository) AssignNotebookToProfile(notebookID string, profileID string) error {
	notebookID = strings.TrimSpace(notebookID)
	if notebookID == "" {
		return fmt.Errorf("notebook id is required")
	}
	var pID interface{} = nil
	if profileID != "" {
		pID = profileID
	}
	_, err := r.db.Exec(`
		UPDATE notebooks
		SET profile_id = ?
		WHERE id = ?
	`, pID, notebookID)
	return err
}

// UpdateNotebookStudyStatus updates the study status of a notebook ('active', 'dormant', 'completed').
func (r *Repository) UpdateNotebookStudyStatus(notebookID string, studyStatus string) error {
	notebookID = strings.TrimSpace(notebookID)
	if notebookID == "" {
		return fmt.Errorf("notebook id is required")
	}
	if studyStatus != "active" && studyStatus != "dormant" && studyStatus != "completed" {
		return fmt.Errorf("invalid study status: %s", studyStatus)
	}

	// If activating, we enforce the hard limit of 3-4 active notebooks per profile.
	if studyStatus == "active" {
		var profileID sql.NullString
		err := r.db.QueryRow(`SELECT profile_id FROM notebooks WHERE id = ?`, notebookID).Scan(&profileID)
		if err != nil {
			return err
		}
		if profileID.Valid && profileID.String != "" {
			var activeCount int
			err = r.db.QueryRow(`
				SELECT COUNT(*) FROM notebooks
				WHERE profile_id = ? AND study_status = 'active'
			`, profileID.String).Scan(&activeCount)
			if err != nil {
				return err
			}
			if activeCount >= 4 {
				return fmt.Errorf("profile already has the maximum limit of 4 active notebooks")
			}
		}
	}

	_, err := r.db.Exec(`
		UPDATE notebooks
		SET study_status = ?
		WHERE id = ?
	`, studyStatus, notebookID)
	return err
}

// GetProfileRemainingWords calculates the remaining unread words across all notebooks in a profile.
func (r *Repository) GetProfileRemainingWords(profileID string) (int, error) {
	var total int
	err := r.db.QueryRow(`
		SELECT COALESCE(SUM(
			CASE
				WHEN COALESCE(t.current_page_cursor, 0) > 0 THEN
					CASE WHEN nc.page_num > t.current_page_cursor
						THEN LENGTH(c.chunk_text) - LENGTH(REPLACE(c.chunk_text, ' ', '')) + 1
						ELSE 0 END
				ELSE
					CASE WHEN nc.page_num >= COALESCE(t.start_page, 0)
						THEN LENGTH(c.chunk_text) - LENGTH(REPLACE(c.chunk_text, ' ', '')) + 1
						ELSE 0 END
			END
		), 0)
		FROM notebooks n
		JOIN notebook_chunks nc ON nc.notebook_id = n.id
		JOIN chunks c ON c.id = nc.chunk_id
		LEFT JOIN topics t ON t.id = c.topic_id
		WHERE (n.profile_id = ? OR n.profile_id IS NULL)
	`, profileID).Scan(&total)
	if err != nil {
		return 0, err
	}
	return total, nil
}
