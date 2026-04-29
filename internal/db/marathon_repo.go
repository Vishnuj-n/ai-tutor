package db

import (
	"fmt"
	"strings"

	"ai-tutor/internal/models"
)

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
