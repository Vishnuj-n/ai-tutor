package db

import (
	"fmt"
	"sort"
	"strings"

	"ai-tutor/internal/embeddings"
	"ai-tutor/internal/models"
)

// GetTopicContent retrieves all parent sections for a topic
func GetTopicContent(topicID string) (map[string]interface{}, error) {
	var title string
	err := conn.QueryRow("SELECT title FROM topics WHERE id = ?", topicID).Scan(&title)
	if err != nil {
		return nil, err
	}

	rows, err := conn.Query(`
		SELECT id, heading, content_text, order_index
		FROM parents
		WHERE topic_id = ?
		ORDER BY order_index
	`, topicID)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = rows.Close()
	}()

	var sections []map[string]interface{}
	for rows.Next() {
		var id, heading, content string
		var order int
		if err := rows.Scan(&id, &heading, &content, &order); err != nil {
			return nil, err
		}
		sections = append(sections, map[string]interface{}{
			"id":      id,
			"heading": heading,
			"content": content,
			"order":   order,
		})
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"title":    title,
		"sections": sections,
	}, nil
}

// GetChunksForTopicPageRange retrieves chunks for a topic within a page range.
func GetChunksForTopicPageRange(topicID string, startPage, endPage int) ([]models.Chunk, error) {
	query := `
		SELECT id, topic_id, parent_id, chunk_text, importance_score, weakness_score, page_num
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
			&chunk.ParentID,
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
		SELECT id, topic_id, parent_id, chunk_text, importance_score, weakness_score, page_num
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
			&chunk.ParentID,
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
		SELECT c.id, c.topic_id, c.parent_id, c.chunk_text, c.importance_score, c.weakness_score, c.page_num
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
			&chunk.ParentID,
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
		SELECT id, topic_id, parent_id, chunk_text, importance_score, weakness_score, page_num
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
			&chunk.ParentID,
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

// GetParentSection retrieves a parent section by ID
func GetParentSection(parentID string) (map[string]string, error) {
	var id, heading, content string
	err := conn.QueryRow(`
		SELECT id, heading, content_text
		FROM parents
		WHERE id = ?
	`, parentID).Scan(&id, &heading, &content)
	if err != nil {
		return nil, err
	}

	return map[string]string{
		"id":      id,
		"heading": heading,
		"content": content,
	}, nil
}

// GetTopicIDBySectionID returns topic_id for a given parent/section id.
func GetTopicIDBySectionID(sectionID string) (string, error) {
	sectionID = strings.TrimSpace(sectionID)
	if sectionID == "" {
		return "", fmt.Errorf("invalid empty sectionID")
	}
	var topicID string
	err := conn.QueryRow(`
		SELECT topic_id
		FROM parents
		WHERE id = ?
	`, sectionID).Scan(&topicID)
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

// GetParentPassagesForTopicPageRange retrieves chunks with their parent passage context for a topic page range
func GetParentPassagesForTopicPageRange(topicID string, startPage int, endPage int) ([]string, error) {
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
		SELECT c.chunk_text, COALESCE(p.heading, ''), COALESCE(p.content_text, '')
		FROM chunks c
		LEFT JOIN parents p ON c.parent_id = p.id AND p.topic_id = c.topic_id
		WHERE c.topic_id = ?
		  AND c.page_num BETWEEN ? AND ?
		ORDER BY c.page_num ASC, c.id ASC
	`, topicID, startPage, endPage)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = rows.Close()
	}()

	var parentPassages []string
	for rows.Next() {
		var chunkText, parentHeading, parentContent string
		if err := rows.Scan(&chunkText, &parentHeading, &parentContent); err != nil {
			return nil, err
		}

		// Build parent passage context
		var passage strings.Builder
		if parentHeading != "" {
			passage.WriteString("Section: ")
			passage.WriteString(parentHeading)
			passage.WriteString("\n")
		}
		if parentContent != "" {
			passage.WriteString("Context: ")
			passage.WriteString(parentContent)
			passage.WriteString("\n")
		}
		passage.WriteString("Content: ")
		passage.WriteString(chunkText)

		parentPassages = append(parentPassages, passage.String())
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return parentPassages, nil
}

// GetTopicHeadingPageRanges returns resolved page bounds per heading for a topic.
// Key format is parent ID (from parents table).
func GetTopicHeadingPageRanges(topicID string) (map[string][2]int, error) {
	if conn == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	topicID = strings.TrimSpace(topicID)
	if topicID == "" {
		return nil, fmt.Errorf("topic id is required")
	}

	rows, err := conn.Query(`
		SELECT
			p.id,
			COALESCE(MIN(NULLIF(c.page_num, 0)), 0) AS start_page,
			COALESCE(MAX(NULLIF(c.page_num, 0)), 0) AS end_page
		FROM parents p
		LEFT JOIN chunks c ON c.parent_id = p.id AND c.topic_id = p.topic_id
		WHERE p.topic_id = ?
		GROUP BY p.id
	`, topicID)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = rows.Close()
	}()

	ranges := make(map[string][][2]int)
	for rows.Next() {
		var parentID string
		var startPage int
		var endPage int
		if err := rows.Scan(&parentID, &startPage, &endPage); err != nil {
			return nil, err
		}

		if parentID == "" {
			continue
		}

		if startPage > 0 && endPage <= 0 {
			endPage = startPage
		}
		if endPage > 0 && startPage <= 0 {
			startPage = endPage
		}
		if startPage <= 0 || endPage <= 0 {
			continue
		}
		if startPage > endPage {
			startPage, endPage = endPage, startPage
		}

		newSpan := [2]int{startPage, endPage}
		existingSpans, ok := ranges[parentID]
		if !ok {
			ranges[parentID] = [][2]int{newSpan}
			continue
		}

		// Collect all spans and sort by start position for proper merging
		allSpans := append(existingSpans, newSpan)
		// Sort by start position
		sort.Slice(allSpans, func(i, j int) bool {
			return allSpans[i][0] < allSpans[j][0]
		})

		// Single linear coalescing pass
		coalesced := [][2]int{allSpans[0]}
		for i := 1; i < len(allSpans); i++ {
			current := allSpans[i]
			last := coalesced[len(coalesced)-1]
			// Check if overlapping or adjacent (end of last >= start of current - 1)
			if last[1] >= current[0]-1 {
				// Merge: extend end if needed
				if current[1] > last[1] {
					coalesced[len(coalesced)-1][1] = current[1]
				}
			} else {
				// No overlap/adjacency, add as new span
				coalesced = append(coalesced, current)
			}
		}
		ranges[parentID] = coalesced
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Convert slice of spans back to single [2]int by taking overall min/max
	// for backward compatibility with existing callers
	result := make(map[string][2]int)
	for parentID, spans := range ranges {
		if len(spans) == 0 {
			continue
		}
		if len(spans) > 1 {
			return nil, fmt.Errorf("disjoint page ranges found for parent %q: %v", parentID, spans)
		}
		result[parentID] = spans[0]
	}

	return result, nil
}
