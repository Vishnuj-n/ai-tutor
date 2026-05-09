package db

import (
	"database/sql"
	"fmt"
	"log"
	"path/filepath"
	"strings"

	"ai-tutor/internal/models"
)

// GetReaderTopicBundle returns notebook metadata plus ordered sections with resolved page numbers.
// If notebookID is provided, section page mapping is scoped to that notebook.
func GetReaderTopicBundle(topicID string, notebookID string) (*models.ReaderTopicBundle, error) {
	topicID = strings.TrimSpace(topicID)
	selectedNotebookID := strings.TrimSpace(notebookID)
	log.Printf("[GetReaderTopicBundle] Called with topicID=%q notebookID=%q", topicID, selectedNotebookID)
	if topicID == "" {
		log.Printf("[GetReaderTopicBundle] ERROR: topic ID is required")
		return nil, fmt.Errorf("topic ID is required")
	}

	bundle := &models.ReaderTopicBundle{
		TopicID:  topicID,
		Sections: []models.ReaderSection{},
	}

	if err := conn.QueryRow(`SELECT title FROM topics WHERE id = ?`, topicID).Scan(&bundle.TopicTitle); err != nil {
		log.Printf("[GetReaderTopicBundle] ERROR: Failed to query topic title for topicID=%q: %v", topicID, err)
		return nil, err
	}
	log.Printf("[GetReaderTopicBundle] Found topic title: %q for topicID=%q", bundle.TopicTitle, topicID)

	var notebookIDRow sql.NullString
	var notebookTitle sql.NullString
	var filePath sql.NullString
	var fileType sql.NullString
	var pageCount sql.NullInt64

	var err error
	if selectedNotebookID != "" {
		err = conn.QueryRow(`
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
		err = conn.QueryRow(`
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
		log.Printf("[GetReaderTopicBundle] ERROR: Failed to query notebook: %v", err)
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
		bundle.NotebookURL = "/notebooks/" + filename
	}
	if fileType.Valid {
		bundle.FileType = fileType.String
	}
	if pageCount.Valid {
		bundle.PageCount = int(pageCount.Int64)
	}

	log.Printf("[GetReaderTopicBundle] Selected notebook: id=%q title=%q filePath=%q fileType=%q pageCount=%d",
		bundle.NotebookID, bundle.NotebookTitle, bundle.NotebookURL, bundle.FileType, bundle.PageCount)

	var rows *sql.Rows
	if bundle.NotebookID != "" {
		rows, err = conn.Query(`
			SELECT
				p.id,
				COALESCE(NULLIF(TRIM(p.heading), ''), 'Section ' || CAST(COALESCE(p.order_index, 0) AS TEXT)),
				p.content_text,
				COALESCE(p.order_index, 0),
				COALESCE(MIN(NULLIF(nc.page_num, 0)), 0) AS page_num
			FROM parents p
			LEFT JOIN chunks c ON c.parent_id = p.id AND c.topic_id = ?
			LEFT JOIN notebook_chunks nc ON nc.chunk_id = c.id AND nc.notebook_id = ?
			WHERE p.topic_id = ?
			GROUP BY p.id, p.heading, p.content_text, p.order_index
			ORDER BY p.order_index ASC, p.id ASC
		`, topicID, bundle.NotebookID, topicID)
	} else {
		rows, err = conn.Query(`
			SELECT
				p.id,
				COALESCE(NULLIF(TRIM(p.heading), ''), 'Section ' || CAST(COALESCE(p.order_index, 0) AS TEXT)),
				p.content_text,
				COALESCE(p.order_index, 0),
				COALESCE(MIN(NULLIF(nc.page_num, 0)), 0) AS page_num
			FROM parents p
			LEFT JOIN chunks c ON c.parent_id = p.id AND c.topic_id = ?
			LEFT JOIN notebook_chunks nc ON nc.chunk_id = c.id
			WHERE p.topic_id = ?
			GROUP BY p.id, p.heading, p.content_text, p.order_index
			ORDER BY p.order_index ASC, p.id ASC
		`, topicID, topicID)
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
		log.Printf("[GetReaderTopicBundle] ERROR: Failed to iterate sections: %v", err)
		return nil, err
	}

	log.Printf("[GetReaderTopicBundle] Returning bundle with %d sections", len(bundle.Sections))
	return bundle, nil
}
