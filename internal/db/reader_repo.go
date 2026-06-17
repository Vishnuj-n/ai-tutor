package db

import (
	"fmt"
	"strings"

	"ai-tutor/internal/embeddings"
	"ai-tutor/internal/models"
)

// GetChunksForTopicPageRange retrieves chunks for a topic within a page range.
func GetChunksForTopicPageRange(topicID string, startPage, endPage int) ([]models.Chunk, error) {
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
		if startPage > endPage {
			startPage, endPage = endPage, startPage
		}
		query += " AND page_num >= ? AND page_num <= ?"
		args = append(args, startPage, endPage)
	}

	// Always include deterministic ordering
	query += " ORDER BY page_num ASC, id ASC"

	rows, err := conn.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = rows.Close()
	}()

	var chunks []models.Chunk
	for rows.Next() {
		var chunk models.Chunk
		if err := rows.Scan(
			&chunk.ID,
			&chunk.TopicID,
			&chunk.Text,
			&chunk.ImportanceScore,
			&chunk.WeaknessScore,
			&chunk.PageNum,
		); err != nil {
			return nil, err
		}
		chunks = append(chunks, chunk)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return chunks, nil
}

// GetChunksForTopic retrieves all chunks for a topic.
func GetChunksForTopic(topicID string) ([]models.Chunk, error) {
	rows, err := conn.Query(`
		SELECT id, topic_id, chunk_text, importance_score, weakness_score, page_num
		FROM chunks
		WHERE topic_id = ?
		ORDER BY id
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
		if err := rows.Scan(
			&chunk.ID,
			&chunk.TopicID,
			&chunk.Text,
			&chunk.ImportanceScore,
			&chunk.WeaknessScore,
			&chunk.PageNum,
		); err != nil {
			return nil, err
		}
		chunks = append(chunks, chunk)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return chunks, nil
}

// GetChunksForNotebook retrieves all chunks linked to one notebook.
func GetChunksForNotebook(notebookID string) ([]models.Chunk, error) {
	notebookID = strings.TrimSpace(notebookID)
	if notebookID == "" {
		return nil, fmt.Errorf("notebook id is required")
	}

	rows, err := conn.Query(`
		SELECT c.id, c.topic_id, c.chunk_text, c.importance_score, c.weakness_score, c.page_num
		FROM notebook_chunks nc
		JOIN chunks c ON c.id = nc.chunk_id
		WHERE nc.notebook_id = ?
		ORDER BY c.page_num ASC, c.id ASC
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
		if err := rows.Scan(
			&chunk.ID,
			&chunk.TopicID,
			&chunk.Text,
			&chunk.ImportanceScore,
			&chunk.WeaknessScore,
			&chunk.PageNum,
		); err != nil {
			return nil, err
		}
		chunks = append(chunks, chunk)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return chunks, nil
}

// GetChunksForTopics retrieves chunks for multiple topics in a single batch query
func GetChunksForTopics(topicIDs []string) (map[string][]models.Chunk, error) {
	if len(topicIDs) == 0 {
		return make(map[string][]models.Chunk), nil
	}

	// Build IN clause placeholders
	placeholders := make([]string, len(topicIDs))
	args := make([]interface{}, len(topicIDs))
	for i, topicID := range topicIDs {
		placeholders[i] = "?"
		args[i] = topicID
	}

	query := fmt.Sprintf(`
		SELECT id, topic_id, chunk_text, importance_score, weakness_score, page_num
		FROM chunks
		WHERE topic_id IN (%s)
	`, strings.Join(placeholders, ","))

	rows, err := conn.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = rows.Close()
	}()

	// Group chunks by topic_id
	result := make(map[string][]models.Chunk)
	for rows.Next() {
		var chunk models.Chunk
		if err := rows.Scan(
			&chunk.ID,
			&chunk.TopicID,
			&chunk.Text,
			&chunk.ImportanceScore,
			&chunk.WeaknessScore,
			&chunk.PageNum,
		); err != nil {
			return nil, err
		}
		result[chunk.TopicID] = append(result[chunk.TopicID], chunk)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return result, nil
}

// GetChunkSection retrieves a chunk section by ID
func GetChunkSection(chunkID string) (map[string]string, error) {
	var id, text string
	var pageNum int
	err := conn.QueryRow(`
		SELECT id, chunk_text, page_num
		FROM chunks
		WHERE id = ?
	`, chunkID).Scan(&id, &text, &pageNum)
	if err != nil {
		return nil, err
	}

	return map[string]string{
		"id":      id,
		"heading": fmt.Sprintf("Page %d", pageNum),
		"content": text,
	}, nil
}

// GetTopicIDBySectionID returns topic_id for a given chunk id.
func GetTopicIDBySectionID(chunkID string) (string, error) {
	chunkID = strings.TrimSpace(chunkID)
	if chunkID == "" {
		return "", fmt.Errorf("invalid empty chunkID")
	}
	var topicID string
	err := conn.QueryRow(`
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
func GetFirstNotebookIDByTopicID(topicID string) (string, error) {
	topicID = strings.TrimSpace(topicID)
	if topicID == "" {
		return "", fmt.Errorf("invalid empty topicID")
	}
	var notebookID string
	err := conn.QueryRow(`
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
func GetTotalChunkTokens(topicID string) (int, error) {
	return getTotalChunkTokens(topicID, 0, 0)
}

// GetTotalChunkTokensForPageRange returns estimated total tokens for one topic/page window.
// It prefers stored token_count values and falls back to len(chunk_text)/4 when token_count is zero or missing.
func GetTotalChunkTokensForPageRange(topicID string, startPage int, endPage int) (int, error) {
	return getTotalChunkTokens(topicID, startPage, endPage)
}

// GetTokensPerPageMap returns a map of page number to total tokens for that page within a page range.
// It prefers stored token_count values and falls back to len(chunk_text)/4 when token_count is zero or missing.
// This uses a single GROUP BY query to avoid N+1 query problems when scanning multiple pages.
func GetTokensPerPageMap(topicID string, startPage int, endPage int) (map[int]int, error) {
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

	rows, err := conn.Query(query, topicID, startPage, endPage)
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

func getTotalChunkTokens(topicID string, startPage int, endPage int) (int, error) {
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

	rows, err := conn.Query(query, args...)
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
func GetChunkTextsForTopicPageRange(topicID string, startPage int, endPage int) ([]string, error) {
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

	rows, err := conn.Query(`
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
func GetTopicHeadingPageRanges(topicID string) (map[string][2]int, error) {
	if conn == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	topicID = strings.TrimSpace(topicID)
	if topicID == "" {
		return nil, fmt.Errorf("topic id is required")
	}

	rows, err := conn.Query(`
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
func GetAllChunks() ([]models.Chunk, error) {
	rows, err := conn.Query(`
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
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return chunks, nil
}
