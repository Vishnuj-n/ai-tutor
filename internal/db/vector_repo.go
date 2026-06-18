package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"ai-tutor/internal/utils"
)

// ChunkVectorBatchItem contains one vector persistence request.
type ChunkVectorBatchItem struct {
	ChunkID      string
	Vector       []float32
	EmbeddingRef string
}

// UpsertChunkVector stores or updates a chunk embedding vector.
func (r *Repository) UpsertChunkVector(chunkID string, vector []float32) error {
	chunkID = strings.TrimSpace(chunkID)
	if chunkID == "" {
		return fmt.Errorf("chunk id is required")
	}
	if len(vector) == 0 {
		return fmt.Errorf("vector is required")
	}

	if len(vector) != int(r.embeddingDimension) {
		return fmt.Errorf("vector dimension mismatch: got %d, expected %d", len(vector), r.embeddingDimension)
	}

	vectorJSON, err := r.vectorToJSON(vector)
	if err != nil {
		return fmt.Errorf("failed to encode vector: %w", err)
	}

	rowID, err := r.lookupChunkRowID(chunkID)
	if err != nil {
		return fmt.Errorf("failed to resolve chunk rowid for %s: %w", chunkID, err)
	}

	var exists int
	err = r.db.QueryRow(`
		SELECT COUNT(*) FROM chunk_vectors WHERE rowid = ?
	`, rowID).Scan(&exists)
	if err != nil && err != sql.ErrNoRows {
		return err
	}

	if exists > 0 {
		_, err = r.db.Exec(`
			UPDATE chunk_vectors SET embedding = ? WHERE rowid = ?
		`, vectorJSON, rowID)
		return err
	}

	_, err = r.db.Exec(`
		INSERT INTO chunk_vectors (rowid, embedding) VALUES (?, ?)
	`, rowID, vectorJSON)
	return err
}

// UpsertChunkVectorsBatch stores vectors and embedding refs in a single transaction.
func (r *Repository) UpsertChunkVectorsBatch(items []ChunkVectorBatchItem) error {
	if len(items) == 0 {
		return nil
	}

	return r.withTx(func(tx *sql.Tx) error {
		// Prepare statements to prevent re-compilation in the loop
		stmtGetRowID, err := tx.Prepare(`SELECT rowid FROM chunks WHERE id = ?`)
		if err != nil {
			return fmt.Errorf("failed to prepare stmtGetRowID: %w", err)
		}
		defer func() {
			_ = stmtGetRowID.Close()
		}()

		stmtCheckExists, err := tx.Prepare(`SELECT COUNT(*) FROM chunk_vectors WHERE rowid = ?`)
		if err != nil {
			return fmt.Errorf("failed to prepare stmtCheckExists: %w", err)
		}
		defer func() {
			_ = stmtCheckExists.Close()
		}()

		stmtUpdateVector, err := tx.Prepare(`UPDATE chunk_vectors SET embedding = ? WHERE rowid = ?`)
		if err != nil {
			return fmt.Errorf("failed to prepare stmtUpdateVector: %w", err)
		}
		defer func() {
			_ = stmtUpdateVector.Close()
		}()

		stmtInsertVector, err := tx.Prepare(`INSERT INTO chunk_vectors (rowid, embedding) VALUES (?, ?)`)
		if err != nil {
			return fmt.Errorf("failed to prepare stmtInsertVector: %w", err)
		}
		defer func() {
			_ = stmtInsertVector.Close()
		}()

		stmtUpdateRef, err := tx.Prepare(`UPDATE chunks SET embedding_ref = ? WHERE id = ?`)
		if err != nil {
			return fmt.Errorf("failed to prepare stmtUpdateRef: %w", err)
		}
		defer func() {
			_ = stmtUpdateRef.Close()
		}()

		for _, item := range items {
			item.ChunkID = strings.TrimSpace(item.ChunkID)
			if item.ChunkID == "" {
				return fmt.Errorf("chunk id is required for each batch item")
			}
			if len(item.Vector) == 0 {
				return fmt.Errorf("vector is required for each batch item")
			}

			if len(item.Vector) != int(r.embeddingDimension) {
				return fmt.Errorf("vector dimension mismatch for chunk %s: got %d, expected %d", item.ChunkID, len(item.Vector), r.embeddingDimension)
			}

			vectorJSON, encodeErr := r.vectorToJSON(item.Vector)
			if encodeErr != nil {
				return fmt.Errorf("failed to encode vector for chunk %s: %w", item.ChunkID, encodeErr)
			}

			var rowID int64
			if scanErr := stmtGetRowID.QueryRow(item.ChunkID).Scan(&rowID); scanErr != nil {
				return fmt.Errorf("failed to resolve chunk rowid for %s: %w", item.ChunkID, scanErr)
			}

			var exists int
			countErr := stmtCheckExists.QueryRow(rowID).Scan(&exists)
			if countErr != nil && countErr != sql.ErrNoRows {
				return countErr
			}

			if exists > 0 {
				if _, execErr := stmtUpdateVector.Exec(vectorJSON, rowID); execErr != nil {
					return execErr
				}
			} else {
				if _, execErr := stmtInsertVector.Exec(rowID, vectorJSON); execErr != nil {
					return execErr
				}
			}

			if item.EmbeddingRef != "" {
				if _, execErr := stmtUpdateRef.Exec(item.EmbeddingRef, item.ChunkID); execErr != nil {
					return execErr
				}
			}
		}
		return nil
	})
}

// SearchVectorsForTopic finds the top-k most similar vectors for a topic-scoped query.
// When startPage and endPage are positive, search is context-locked to that page window.
func (r *Repository) SearchVectorsForTopic(topicID string, queryVector []float32, k int, startPage int, endPage int) ([]string, error) {
	topicID = strings.TrimSpace(topicID)
	if topicID == "" {
		return nil, fmt.Errorf("topic id is required")
	}
	if len(queryVector) == 0 {
		return nil, fmt.Errorf("query vector is required")
	}
	if k <= 0 || k > maxRetrievalK {
		return nil, fmt.Errorf("k must be between 1 and %d", maxRetrievalK)
	}

	if r.embeddingDimension <= 0 {
		msg := fmt.Sprintf("warning: vector search skipped for topic %s because embedding dimension is not initialized", topicID)
		utils.RagLogger.Printf("%s", msg)
		utils.ErrLogger.Printf("%s", msg)
		return []string{}, nil
	}

	if len(queryVector) != int(r.embeddingDimension) {
		return nil, fmt.Errorf("query vector dimension mismatch: got %d, expected %d", len(queryVector), r.embeddingDimension)
	}

	queryVectorJSON, err := r.vectorToJSON(queryVector)
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

	rowRows, err := r.db.Query(rowidQuery, rowidArgs...)
	if err != nil {
		return nil, fmt.Errorf("chunk prefilter failed: %w", err)
	}
	defer func() {
		_ = rowRows.Close()
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
	allowedRowIDsJSON, err := json.Marshal(allowedRowIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to encode allowed row ids: %w", err)
	}

	vectorArgs := []interface{}{string(allowedRowIDsJSON), queryVectorJSON, k}

	vectorSQL := `
		SELECT rowid
		FROM chunk_vectors
		WHERE rowid IN (SELECT CAST(value AS INTEGER) FROM json_each(?))
		ORDER BY distance(embedding, ?) ASC
		LIMIT ?
	`

	rows, err := r.db.Query(vectorSQL, vectorArgs...)
	if err != nil {
		if isVectorUnavailableError(err) {
			msg := fmt.Sprintf("warning: vector search unavailable for topic %s, using lexical fallback: %v", topicID, err)
			utils.RagLogger.Printf("%s", msg)
			utils.ErrLogger.Printf("%s", msg)
			return []string{}, nil
		}
		return nil, fmt.Errorf("vector search failed: %w", err)
	}
	defer func() {
		_ = rows.Close()
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

// SearchVectorsForNotebook finds the top-k most similar vectors for a notebook-scoped query.
func (r *Repository) SearchVectorsForNotebook(notebookID string, queryVector []float32, k int) ([]string, error) {
	notebookID = strings.TrimSpace(notebookID)
	if notebookID == "" {
		return nil, fmt.Errorf("notebook id is required")
	}
	if len(queryVector) == 0 {
		return nil, fmt.Errorf("query vector is required")
	}
	if k <= 0 || k > maxRetrievalK {
		return nil, fmt.Errorf("k must be between 1 and %d", maxRetrievalK)
	}

	if r.embeddingDimension <= 0 {
		msg := fmt.Sprintf("warning: vector search skipped for notebook %s because embedding dimension is not initialized", notebookID)
		utils.RagLogger.Printf("%s", msg)
		utils.ErrLogger.Printf("%s", msg)
		return []string{}, nil
	}

	if len(queryVector) != int(r.embeddingDimension) {
		return nil, fmt.Errorf("query vector dimension mismatch: got %d, expected %d", len(queryVector), r.embeddingDimension)
	}

	queryVectorJSON, err := r.vectorToJSON(queryVector)
	if err != nil {
		return nil, fmt.Errorf("failed to encode query vector: %w", err)
	}

	rowRows, err := r.db.Query(`
		SELECT DISTINCT c.rowid, c.id
		FROM notebook_chunks nc
		JOIN chunks c ON c.id = nc.chunk_id
		WHERE nc.notebook_id = ?
	`, notebookID)
	if err != nil {
		return nil, fmt.Errorf("chunk prefilter failed: %w", err)
	}
	defer func() {
		_ = rowRows.Close()
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

	allowedRowIDsJSON, err := json.Marshal(allowedRowIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to encode allowed row ids: %w", err)
	}

	rows, err := r.db.Query(`
		SELECT rowid
		FROM chunk_vectors
		WHERE rowid IN (SELECT CAST(value AS INTEGER) FROM json_each(?))
		ORDER BY distance(embedding, ?) ASC
		LIMIT ?
	`, string(allowedRowIDsJSON), queryVectorJSON, k)
	if err != nil {
		if isVectorUnavailableError(err) {
			msg := fmt.Sprintf("warning: vector search unavailable for notebook %s, using lexical fallback: %v", notebookID, err)
			utils.RagLogger.Printf("%s", msg)
			utils.ErrLogger.Printf("%s", msg)
			return []string{}, nil
		}
		return nil, fmt.Errorf("vector search failed: %w", err)
	}
	defer func() {
		_ = rows.Close()
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

func (r *Repository) vectorToJSON(vector []float32) (string, error) {
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

func (r *Repository) lookupChunkRowID(chunkID string) (int64, error) {
	var rowID int64
	if err := r.db.QueryRow(`
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
