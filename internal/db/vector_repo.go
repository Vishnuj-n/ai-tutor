package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
)

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

func searchVectorsForTopicRepo(topicID string, queryVector []float32, k int) ([]string, error) {
	if len(queryVector) != int(embeddingDimension) {
		return nil, fmt.Errorf("query vector dimension mismatch: got %d, expected %d", len(queryVector), embeddingDimension)
	}

	queryVectorJSON, err := vectorToJSONRepo(queryVector)
	if err != nil {
		return nil, fmt.Errorf("failed to encode query vector: %w", err)
	}

	rows, err := conn.Query(`
		SELECT c.id
		FROM chunk_vectors cv
		JOIN chunks c ON c.rowid = cv.rowid
		WHERE c.topic_id = ?
		ORDER BY distance(cv.embedding, ?) ASC
		LIMIT ?
	`, topicID, queryVectorJSON, k)
	if err != nil {
		return nil, fmt.Errorf("vector search failed: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			log.Printf("warning: failed to close vector search rows: %v", closeErr)
		}
	}()

	var chunkIDs []string
	for rows.Next() {
		var chunkID string
		if err := rows.Scan(&chunkID); err != nil {
			return nil, err
		}
		chunkIDs = append(chunkIDs, chunkID)
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
