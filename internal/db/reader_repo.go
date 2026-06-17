package db

import (
	"database/sql"
	"fmt"
	"strings"

	"ai-tutor/internal/embeddings"
	"ai-tutor/internal/models"
)

// GetChunksForTopicPageRange retrieves chunks for a topic within a page range.
func (r *Repository) GetChunksForTopicPageRange(topicID string, startPage, endPage int) ([]models.Chunk, error) {
	query := `
		SELECT id, topic_id, chunk_text, importance_score, weakness_score, page_num
		FROM chunks
		WHERE topic_id = ?`

	var args []interface{}
	args = append(args, topicID)

	// Validate that either both bounds are provided or neither is
	if (startPage > 0) != (endPage > 0) {
		return nil, fmt.Errorf("both startPage and endPage must be provided together, or neither")
	}

	if startPage > 0 && endPage > 0 {
		query += " AND page_num BETWEEN ? AND ?"
		args = append(args, startPage, endPage)
	}

	query += " ORDER BY page_num ASC, id ASC"

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = rows.Close()
	}()

	var chunks []models.Chunk
	for rows.Next() {
		var chunk models.Chunk
		if err := rows.Scan(&chunk.ID, &chunk.TopicID, &chunk.Text, &chunk.ImportanceScore, &chunk.WeaknessScore, &chunk.PageNum); err != nil {
			return nil, err
		}
		chunks = append(chunks, chunk)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return chunks, nil
}

// GetChunksForTopic retrieves all chunks associated with a topic.
func (r *Repository) GetChunksForTopic(topicID string) ([]models.Chunk, error) {
	rows, err := r.db.Query(`
		SELECT id, topic_id, chunk_text, importance_score, weakness_score, page_num
		FROM chunks
		WHERE topic_id = ?
		ORDER BY page_num ASC, id ASC
	`, topicID)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = rows.Close()
	}()

	var chunks []models.Chunk
	for rows.Next() {
		var chunk models.Chunk
		if err := rows.Scan(&chunk.ID, &chunk.TopicID, &chunk.Text, &chunk.ImportanceScore, &chunk.WeaknessScore, &chunk.PageNum); err != nil {
			return nil, err
		}
		chunks = append(chunks, chunk)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return chunks, nil
}

// GetChunksForNotebook retrieves all chunks associated with a notebook.
func (r *Repository) GetChunksForNotebook(notebookID string) ([]models.Chunk, error) {
	rows, err := r.db.Query(`
		SELECT c.id, c.topic_id, c.chunk_text, c.importance_score, c.weakness_score, nc.page_num
		FROM chunks c
		JOIN notebook_chunks nc ON nc.chunk_id = c.id
		WHERE nc.notebook_id = ?
		ORDER BY nc.page_num ASC, c.id ASC
	`, notebookID)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = rows.Close()
	}()

	var chunks []models.Chunk
	for rows.Next() {
		var chunk models.Chunk
		if err := rows.Scan(&chunk.ID, &chunk.TopicID, &chunk.Text, &chunk.ImportanceScore, &chunk.WeaknessScore, &chunk.PageNum); err != nil {
			return nil, err
		}
		chunks = append(chunks, chunk)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return chunks, nil
}

// GetChunksForTopics batches chunk loading for multiple topics.
func (r *Repository) GetChunksForTopics(topicIDs []string) (map[string][]models.Chunk, error) {
	if len(topicIDs) == 0 {
		return make(map[string][]models.Chunk), nil
	}

	placeholders := make([]string, len(topicIDs))
	args := make([]interface{}, len(topicIDs))
	for i, id := range topicIDs {
		placeholders[i] = "?"
		args[i] = id
	}

	query := fmt.Sprintf(`
		SELECT id, topic_id, chunk_text, importance_score, weakness_score, page_num
		FROM chunks
		WHERE topic_id IN (%s)
		ORDER BY page_num ASC, id ASC
	`, strings.Join(placeholders, ","))

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = rows.Close()
	}()

	chunksByTopic := make(map[string][]models.Chunk)
	for rows.Next() {
		var chunk models.Chunk
		if err := rows.Scan(&chunk.ID, &chunk.TopicID, &chunk.Text, &chunk.ImportanceScore, &chunk.WeaknessScore, &chunk.PageNum); err != nil {
			return nil, err
		}
		chunksByTopic[chunk.TopicID] = append(chunksByTopic[chunk.TopicID], chunk)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return chunksByTopic, nil
}

// GetChunkSection returns notebook metadata plus ordered sections with resolved page numbers.
func (r *Repository) GetChunkSection(chunkID string) (map[string]string, error) {
	var notebookID sql.NullString
	var pageNum sql.NullInt64

	err := r.db.QueryRow(`
		SELECT nc.notebook_id, nc.page_num
		FROM notebook_chunks nc
		WHERE nc.chunk_id = ?
		LIMIT 1
	`, chunkID).Scan(&notebookID, &pageNum)
	if err != nil {
		return nil, err
	}

	res := map[string]string{}
	if notebookID.Valid {
		res["notebook_id"] = notebookID.String
	}
	if pageNum.Valid {
		res["page_num"] = fmt.Sprintf("%d", pageNum.Int64)
	}
	return res, nil
}

// GetTopicIDBySectionID returns the topic ID associated with a chunk ID.
func (r *Repository) GetTopicIDBySectionID(chunkID string) (string, error) {
	var topicID string
	err := r.db.QueryRow(`
		SELECT topic_id
		FROM chunks
		WHERE id = ?
	`, chunkID).Scan(&topicID)
	if err != nil {
		return "", err
	}
	return topicID, nil
}

// GetFirstNotebookIDByTopicID returns the earliest notebook_id linked to a topic.
// If no notebook is linked, returns sql.ErrNoRows.
func (r *Repository) GetFirstNotebookIDByTopicID(topicID string) (string, error) {
	topicID = strings.TrimSpace(topicID)
	if topicID == "" {
		return "", fmt.Errorf("invalid empty topicID")
	}
	var notebookID string
	err := r.db.QueryRow(`
		SELECT notebook_id
		FROM notebook_topics
		WHERE topic_id = ?
		ORDER BY created_at ASC, notebook_id ASC
		LIMIT 1
	`, topicID).Scan(&notebookID)
	if err != nil {
		return "", err
	}
	return notebookID, nil
}

// GetTotalChunkTokens returns estimated total tokens for one topic.
// It prefers stored token_count values and falls back to len(chunk_text)/4 when token_count is zero or missing.
func (r *Repository) GetTotalChunkTokens(topicID string) (int, error) {
	return r.getTotalChunkTokens(topicID, 0, 0)
}

// GetTotalChunkTokensForPageRange returns estimated total tokens for one topic/page window.
// It prefers stored token_count values and falls back to len(chunk_text)/4 when token_count is zero or missing.
func (r *Repository) GetTotalChunkTokensForPageRange(topicID string, startPage int, endPage int) (int, error) {
	return r.getTotalChunkTokens(topicID, startPage, endPage)
}

// GetTokensPerPageMap returns a map of page number to total tokens for that page within a page range.
// It prefers stored token_count values and falls back to len(chunk_text)/4 when token_count is zero or missing.
// This uses a single GROUP BY query to avoid N+1 query problems when scanning multiple pages.
func (r *Repository) GetTokensPerPageMap(topicID string, startPage int, endPage int) (map[int]int, error) {
	topicID = strings.TrimSpace(topicID)
	if topicID == "" {
		return nil, fmt.Errorf("topic id is required")
	}
	if startPage <= 0 || endPage <= 0 {
		return nil, fmt.Errorf("start page and end page must be positive")
	}
	if startPage > endPage {
		startPage, endPage = endPage, startPage
	}

	query := `
		SELECT page_num, COALESCE(token_count, 0), COALESCE(chunk_text, '')
		FROM chunks
		WHERE topic_id = ?
		  AND page_num BETWEEN ? AND ?
		ORDER BY page_num
	`

	rows, err := r.db.Query(query, topicID, startPage, endPage)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = rows.Close()
	}()

	result := make(map[int]int)
	for rows.Next() {
		var pageNum int
		var tokenCount int
		var chunkText string
		if err := rows.Scan(&pageNum, &tokenCount, &chunkText); err != nil {
			return nil, err
		}

		pageTotal := 0
		if tokenCount > 0 {
			pageTotal = tokenCount
		} else {
			count, err := embeddings.CountTokens(chunkText)
			if err != nil {
				// Fall back to estimation if tokenizer unavailable
				pageTotal = len(chunkText) / 4
				if pageTotal <= 0 && strings.TrimSpace(chunkText) != "" {
					pageTotal = 1
				}
			} else {
				pageTotal = count
			}
		}

		result[pageNum] += pageTotal
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return result, nil
}

func (r *Repository) getTotalChunkTokens(topicID string, startPage int, endPage int) (int, error) {
	topicID = strings.TrimSpace(topicID)
	if topicID == "" {
		return 0, fmt.Errorf("topic id is required")
	}

	// Validate page bounds
	var filterByPage bool
	if startPage == 0 && endPage == 0 {
		// No page filter - entire topic
		filterByPage = false
	} else if startPage <= 0 || endPage <= 0 {
		// Mixed positive/negative bounds are invalid
		return 0, fmt.Errorf("invalid page bounds: both startPage and endPage must be positive or both must be zero")
	} else {
		// Both bounds are positive - filter by page range
		filterByPage = true
		if startPage > endPage {
			startPage, endPage = endPage, startPage
		}
	}

	query := `
		SELECT COALESCE(token_count, 0), COALESCE(chunk_text, '')
		FROM chunks
		WHERE topic_id = ?
	`
	args := []interface{}{topicID}
	if filterByPage {
		query += ` AND page_num BETWEEN ? AND ?`
		args = append(args, startPage, endPage)
	}

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return 0, err
	}
	defer func() {
		_ = rows.Close()
	}()

	total := 0
	for rows.Next() {
		var tokenCount int
		var chunkText string
		if err := rows.Scan(&tokenCount, &chunkText); err != nil {
			return 0, err
		}

		if tokenCount > 0 {
			total += tokenCount
			continue
		}

		count, err := embeddings.CountTokens(chunkText)
		if err != nil {
			// Fall back to estimation if tokenizer unavailable
			fallback := len(chunkText) / 4
			if fallback <= 0 && strings.TrimSpace(chunkText) != "" {
				fallback = 1
			}
			total += fallback
		} else {
			total += count
		}
	}

	if err := rows.Err(); err != nil {
		return 0, err
	}

	return total, nil
}

// GetChunkTextsForTopicPageRange returns chunk_text rows ordered by chunk id for one topic/page window.
func (r *Repository) GetChunkTextsForTopicPageRange(topicID string, startPage int, endPage int) ([]string, error) {
	topicID = strings.TrimSpace(topicID)
	if topicID == "" {
		return nil, fmt.Errorf("topic id is required")
	}
	if startPage <= 0 || endPage <= 0 {
		return nil, fmt.Errorf("start page and end page must be positive")
	}
	if startPage > endPage {
		startPage, endPage = endPage, startPage
	}

	rows, err := r.db.Query(`
		SELECT chunk_text
		FROM chunks
		WHERE topic_id = ?
		  AND page_num BETWEEN ? AND ?
		ORDER BY page_num ASC, id ASC
	`, topicID, startPage, endPage)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = rows.Close()
	}()

	var chunkTexts []string
	for rows.Next() {
		var chunkText string
		if err := rows.Scan(&chunkText); err != nil {
			return nil, err
		}
		chunkTexts = append(chunkTexts, chunkText)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return chunkTexts, nil
}

// GetTopicHeadingPageRanges returns resolved page bounds per chunk ID for a topic.
func (r *Repository) GetTopicHeadingPageRanges(topicID string) (map[string][2]int, error) {
	if r.db == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	topicID = strings.TrimSpace(topicID)
	if topicID == "" {
		return nil, fmt.Errorf("topic id is required")
	}

	rows, err := r.db.Query(`
		SELECT id, COALESCE(page_num, 0)
		FROM chunks
		WHERE topic_id = ?
	`, topicID)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = rows.Close()
	}()

	result := make(map[string][2]int)
	for rows.Next() {
		var id string
		var pageNum int
		if err := rows.Scan(&id, &pageNum); err != nil {
			return nil, err
		}
		if id == "" {
			continue
		}
		result[id] = [2]int{pageNum, pageNum}
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return result, nil
}

// GetAllChunks retrieves all chunks in the system.
func (r *Repository) GetAllChunks() ([]models.Chunk, error) {
	rows, err := r.db.Query(`
		SELECT id, topic_id, chunk_text, importance_score, weakness_score, page_num
		FROM chunks
		ORDER BY id
	`)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = rows.Close()
	}()

	var chunks []models.Chunk
	for rows.Next() {
		var chunk models.Chunk
		if err := rows.Scan(&chunk.ID, &chunk.TopicID, &chunk.Text, &chunk.ImportanceScore, &chunk.WeaknessScore, &chunk.PageNum); err != nil {
			return nil, err
		}
		chunks = append(chunks, chunk)
	}
	return chunks, nil
}
