package db

import (
	"database/sql"
	"fmt"
	"net/url"
	"path/filepath"
	"strings"

	"ai-tutor/internal/models"
)

// GetReaderTopicBundle returns notebook metadata plus ordered sections with resolved page numbers.
// If notebookID is provided, section page mapping is scoped to that notebook.
func (r *Repository) GetReaderTopicBundle(topicID string, notebookID string) (*models.ReaderTopicBundle, error) {
	topicID = strings.TrimSpace(topicID)
	selectedNotebookID := strings.TrimSpace(notebookID)
	if topicID == "" {
		return nil, fmt.Errorf("topic ID is required")
	}

	bundle := &models.ReaderTopicBundle{
		TopicID:  topicID,
		Sections: []models.ReaderSection{},
	}

	var startPage int
	var endPage int
	if err := r.db.QueryRow(`
		SELECT title, COALESCE(start_page, 0), COALESCE(end_page, 0)
		FROM topics WHERE id = ?
	`, topicID).Scan(&bundle.TopicTitle, &startPage, &endPage); err != nil {
		return nil, err
	}
	bundle.TopicStartPage = startPage
	bundle.TopicEndPage = endPage

	var notebookIDRow sql.NullString
	var notebookTitle sql.NullString
	var filePath sql.NullString
	var fileType sql.NullString
	var pageCount sql.NullInt64

	var err error
	if selectedNotebookID != "" {
		err = r.db.QueryRow(`
			SELECT id, title, file_path, file_type, COALESCE(page_count, 0)
			FROM notebooks n
			WHERE n.id = ?
			  AND (
				n.topic_id = ?
				OR EXISTS (
					SELECT 1
					FROM notebook_chunks nc
					JOIN chunks c ON c.id = nc.chunk_id
					WHERE nc.notebook_id = n.id AND c.topic_id = ?
				)
			  )
			LIMIT 1
		`, selectedNotebookID, topicID, topicID).Scan(&notebookIDRow, &notebookTitle, &filePath, &fileType, &pageCount)
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("selected notebook does not contain this topic")
		}
	} else {
		err = r.db.QueryRow(`
			SELECT id, title, file_path, file_type, page_count
			FROM (
				SELECT
					n.id,
					n.title,
					n.file_path,
					n.file_type,
					COALESCE(n.page_count, 0) AS page_count,
					n.uploaded_at,
					0 AS rank
				FROM notebooks n
				WHERE n.topic_id = ?

				UNION

				SELECT
					n.id,
					n.title,
					n.file_path,
					n.file_type,
					COALESCE(n.page_count, 0) AS page_count,
					n.uploaded_at,
					1 AS rank
				FROM notebooks n
				JOIN notebook_chunks nc ON nc.notebook_id = n.id
				JOIN chunks c ON c.id = nc.chunk_id
				WHERE c.topic_id = ?
			)
			ORDER BY rank ASC, uploaded_at DESC, id ASC
			LIMIT 1
		`, topicID, topicID).Scan(&notebookIDRow, &notebookTitle, &filePath, &fileType, &pageCount)
	}
	if err != nil && err != sql.ErrNoRows {
		return nil, err
	}

	if notebookIDRow.Valid {
		bundle.NotebookID = notebookIDRow.String
	}
	if notebookTitle.Valid {
		bundle.NotebookTitle = notebookTitle.String
	}
	if filePath.Valid {
		// Convert filesystem path to URL path for the file server
		// The notebookHandler in main.go serves files at /notebooks/<filename>
		filename := filepath.Base(filePath.String)
		bundle.NotebookURL = "/notebooks/" + url.PathEscape(filename)
	}
	if fileType.Valid {
		bundle.FileType = fileType.String
	}
	if pageCount.Valid {
		bundle.PageCount = int(pageCount.Int64)
	}

	var rows *sql.Rows
	if bundle.NotebookID != "" {
		rows, err = r.db.Query(`
			SELECT
				c.id,
				'Page ' || CAST(COALESCE(nc.page_num, 0) AS TEXT),
				c.chunk_text,
				COALESCE(nc.page_num, 0),
				COALESCE(nc.page_num, 0) AS page_num
			FROM chunks c
			JOIN notebook_chunks nc ON nc.chunk_id = c.id AND nc.notebook_id = ?
			WHERE c.topic_id = ?
			ORDER BY nc.page_num ASC, c.id ASC
		`, bundle.NotebookID, topicID)
	} else {
		rows, err = r.db.Query(`
			SELECT
				c.id,
				'Page ' || CAST(COALESCE(c.page_num, 0) AS TEXT),
				c.chunk_text,
				COALESCE(c.page_num, 0),
				COALESCE(c.page_num, 0) AS page_num
			FROM chunks c
			WHERE c.topic_id = ?
			ORDER BY c.page_num ASC, c.id ASC
		`, topicID)
	}
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = rows.Close()
	}()

	for rows.Next() {
		var section models.ReaderSection
		if err := rows.Scan(
			&section.ID,
			&section.Heading,
			&section.Content,
			&section.Order,
			&section.PageNum,
		); err != nil {
			return nil, err
		}
		bundle.Sections = append(bundle.Sections, section)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return bundle, nil
}
