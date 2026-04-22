package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"strings"
)

type chunkVectorBatchItemRepo struct {
	ChunkID      string
	Vector       []float32
	EmbeddingRef string
}

func upsertChunkVectorRepo(chunkID string, vector []float32) error {
	if len(vector) != int(embeddingDimension) {
		return fmt.Errorf("vector dimension mismatch: got %d, expected %d", len(vector), embeddingDimension)
	}

	vectorJSON, err := vectorToJSONRepo(vector)
	if err != nil {
		return fmt.Errorf("failed to encode vector: %w", err)
	}

	rowID, err := lookupChunkRowIDRepo(chunkID)
	if err != nil {
		return fmt.Errorf("failed to resolve chunk rowid for %s: %w", chunkID, err)
	}

	var exists int
	err = conn.QueryRow(`
		SELECT COUNT(*) FROM chunk_vectors WHERE rowid = ?
	`, rowID).Scan(&exists)
	if err != nil && err != sql.ErrNoRows {
		return err
	}

	if exists > 0 {
		_, err = conn.Exec(`
			UPDATE chunk_vectors SET embedding = ? WHERE rowid = ?
		`, vectorJSON, rowID)
		return err
	}

	_, err = conn.Exec(`
		INSERT INTO chunk_vectors (rowid, embedding) VALUES (?, ?)
	`, rowID, vectorJSON)
	return err
}

func upsertChunkVectorsBatchRepo(items []chunkVectorBatchItemRepo) (err error) {
	if len(items) == 0 {
		return nil
	}

	tx, err := conn.Begin()
	if err != nil {
		return err
	}

	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	for _, item := range items {
		if len(item.Vector) != int(embeddingDimension) {
			return fmt.Errorf("vector dimension mismatch for chunk %s: got %d, expected %d", item.ChunkID, len(item.Vector), embeddingDimension)
		}

		vectorJSON, encodeErr := vectorToJSONRepo(item.Vector)
		if encodeErr != nil {
			return fmt.Errorf("failed to encode vector for chunk %s: %w", item.ChunkID, encodeErr)
		}

		var rowID int64
		if scanErr := tx.QueryRow(`
			SELECT rowid FROM chunks WHERE id = ?
		`, item.ChunkID).Scan(&rowID); scanErr != nil {
			return fmt.Errorf("failed to resolve chunk rowid for %s: %w", item.ChunkID, scanErr)
		}

		var exists int
		countErr := tx.QueryRow(`
			SELECT COUNT(*) FROM chunk_vectors WHERE rowid = ?
		`, rowID).Scan(&exists)
		if countErr != nil && countErr != sql.ErrNoRows {
			return countErr
		}

		if exists > 0 {
			if _, execErr := tx.Exec(`
				UPDATE chunk_vectors SET embedding = ? WHERE rowid = ?
			`, vectorJSON, rowID); execErr != nil {
				return execErr
			}
		} else {
			if _, execErr := tx.Exec(`
				INSERT INTO chunk_vectors (rowid, embedding) VALUES (?, ?)
			`, rowID, vectorJSON); execErr != nil {
				return execErr
			}
		}

		if item.EmbeddingRef != "" {
			if _, execErr := tx.Exec(`
				UPDATE chunks SET embedding_ref = ? WHERE id = ?
			`, item.EmbeddingRef, item.ChunkID); execErr != nil {
				return execErr
			}
		}
	}

	err = tx.Commit()
	return err
}

func searchVectorsForTopicRepo(topicID string, queryVector []float32, k int, startPage int, endPage int) ([]string, error) {
	if embeddingDimension <= 0 {
		log.Printf("warning: vector search skipped for topic %s because embedding dimension is not initialized", topicID)
		return []string{}, nil
	}

	if len(queryVector) != int(embeddingDimension) {
		return nil, fmt.Errorf("query vector dimension mismatch: got %d, expected %d", len(queryVector), embeddingDimension)
	}

	queryVectorJSON, err := vectorToJSONRepo(queryVector)
	if err != nil {
		return nil, fmt.Errorf("failed to encode query vector: %w", err)
	}

	filterByPage := startPage > 0 && endPage > 0
	if filterByPage && startPage > endPage {
		startPage, endPage = endPage, startPage
	}

	rowidQuery := `
		SELECT rowid, id
		FROM chunks
		WHERE topic_id = ?
	`
	rowidArgs := []interface{}{topicID}
	if filterByPage {
		rowidQuery += " AND page_num BETWEEN ? AND ?"
		rowidArgs = append(rowidArgs, startPage, endPage)
	}

	rowRows, err := conn.Query(rowidQuery, rowidArgs...)
	if err != nil {
		return nil, fmt.Errorf("chunk prefilter failed: %w", err)
	}
	defer func() {
		if closeErr := rowRows.Close(); closeErr != nil {
			log.Printf("warning: failed to close chunk prefilter rows: %v", closeErr)
		}
	}()

	allowedChunkByRowID := make(map[int64]string)
	allowedRowIDs := make([]int64, 0)
	for rowRows.Next() {
		var rowID int64
		var chunkID string
		if scanErr := rowRows.Scan(&rowID, &chunkID); scanErr != nil {
			return nil, scanErr
		}
		allowedChunkByRowID[rowID] = chunkID
		allowedRowIDs = append(allowedRowIDs, rowID)
	}
	if err := rowRows.Err(); err != nil {
		return nil, err
	}
	if len(allowedRowIDs) == 0 {
		return []string{}, nil
	}

	placeholders := make([]string, 0, len(allowedRowIDs))
	vectorArgs := make([]interface{}, 0, len(allowedRowIDs)+2)
	for _, rowID := range allowedRowIDs {
		placeholders = append(placeholders, "?")
		vectorArgs = append(vectorArgs, rowID)
	}
	vectorArgs = append(vectorArgs, queryVectorJSON, k)

	vectorSQL := fmt.Sprintf(`
		SELECT rowid
		FROM chunk_vectors
		WHERE rowid IN (%s)
		ORDER BY distance(embedding, ?) ASC
		LIMIT ?
	`, strings.Join(placeholders, ","))

	rows, err := conn.Query(vectorSQL, vectorArgs...)
	if err != nil {
		if isVectorUnavailableError(err) {
			log.Printf("warning: vector search unavailable for topic %s, using lexical fallback: %v", topicID, err)
			return []string{}, nil
		}
		return nil, fmt.Errorf("vector search failed: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			log.Printf("warning: failed to close vector search rows: %v", closeErr)
		}
	}()

	chunkIDs := make([]string, 0, k)
	for rows.Next() {
		var rowID int64
		if err := rows.Scan(&rowID); err != nil {
			return nil, err
		}
		if chunkID, ok := allowedChunkByRowID[rowID]; ok {
			chunkIDs = append(chunkIDs, chunkID)
		}
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return chunkIDs, nil
}

func vectorToJSONRepo(vector []float32) (string, error) {
	if len(vector) == 0 {
		return "[]", nil
	}

	values := make([]float64, len(vector))
	for i, value := range vector {
		values[i] = float64(value)
	}

	encoded, err := json.Marshal(values)
	if err != nil {
		return "", err
	}

	return string(encoded), nil
}

func lookupChunkRowIDRepo(chunkID string) (int64, error) {
	var rowID int64
	if err := conn.QueryRow(`
		SELECT rowid FROM chunks WHERE id = ?
	`, chunkID).Scan(&rowID); err != nil {
		return 0, err
	}

	return rowID, nil
}

func isVectorUnavailableError(err error) bool {
	if err == nil {
		return false
	}

	errText := strings.ToLower(err.Error())
	switch {
	case strings.Contains(errText, "no such module: vec0"):
		return true
	case strings.Contains(errText, "no such table: chunk_vectors"):
		return true
	case strings.Contains(errText, "no such function: distance"):
		return true
	default:
		return false
	}
}
