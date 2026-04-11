package db

import (
	"database/sql"
	"fmt"
)

func ingestNotebookContentRepo(notebookID string, topicID string, parents []NotebookParentInput, chunks []NotebookChunkInput) error {
	if topicID == "" {
		return fmt.Errorf("topic id is required for ingestion")
	}
	group := NotebookTopicIngestionGroup{
		TopicID: topicID,
		Parents: parents,
		Chunks:  chunks,
	}
	return ingestNotebookContentByTopicRepo(notebookID, []NotebookTopicIngestionGroup{group})
}

func ingestNotebookContentByTopicRepo(notebookID string, groups []NotebookTopicIngestionGroup) error {
	if notebookID == "" {
		return fmt.Errorf("notebook id is required")
	}
	if len(groups) == 0 {
		return fmt.Errorf("at least one topic group is required")
	}

	tx, err := conn.Begin()
	if err != nil {
		return err
	}
	defer func() {
		_ = tx.Rollback()
	}()

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

	parentPrefix := fmt.Sprintf("nbp_%s_%%", notebookID)
	chunkPrefix := fmt.Sprintf("nbc_%s_%%", notebookID)

	if _, err := tx.Exec("DELETE FROM chunks WHERE id LIKE ?", chunkPrefix); err != nil {
		return err
	}
	if _, err := tx.Exec("DELETE FROM parents WHERE id LIKE ?", parentPrefix); err != nil {
		return err
	}

	totalChunks := 0
	for _, group := range groups {
		if group.TopicID == "" {
			return fmt.Errorf("topic id is required for each ingestion group")
		}

		for _, parent := range group.Parents {
			if err := insertParentRowRepo(tx, group.TopicID, parent); err != nil {
				return err
			}
		}

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
	`, totalChunks, "chunked", groups[0].TopicID, notebookID); err != nil {
		return err
	}

	return tx.Commit()
}

func deleteNotebookRepo(notebookID string) error {
	tx, err := conn.Begin()
	if err != nil {
		return err
	}
	defer func() {
		_ = tx.Rollback()
	}()

	parentIDs := make(map[string]struct{})
	parentRows, err := tx.Query(`
		SELECT DISTINCT c.parent_id
		FROM chunks c
		JOIN notebook_chunks nc ON nc.chunk_id = c.id
		WHERE nc.notebook_id = ?
	`, notebookID)
	if err != nil {
		return err
	}

	for parentRows.Next() {
		var parentID string
		if scanErr := parentRows.Scan(&parentID); scanErr != nil {
			_ = parentRows.Close()
			return scanErr
		}
		parentIDs[parentID] = struct{}{}
	}
	if rowsErr := parentRows.Err(); rowsErr != nil {
		_ = parentRows.Close()
		return rowsErr
	}
	_ = parentRows.Close()

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

	if _, delChunkErr := tx.Exec(`
		DELETE FROM chunks
		WHERE id IN (
			SELECT chunk_id
			FROM notebook_chunks
			WHERE notebook_id = ?
		)
	`, notebookID); delChunkErr != nil {
		return delChunkErr
	}

	_, err = tx.Exec("DELETE FROM notebook_chunks WHERE notebook_id = ?", notebookID)
	if err != nil {
		return err
	}

	_, err = tx.Exec("DELETE FROM notebooks WHERE id = ?", notebookID)
	if err != nil {
		return err
	}

	for parentID := range parentIDs {
		var count int
		if countErr := tx.QueryRow(`
			SELECT COUNT(*) FROM chunks WHERE parent_id = ?
		`, parentID).Scan(&count); countErr != nil {
			return countErr
		}
		if count == 0 {
			if _, delParentErr := tx.Exec(`
				DELETE FROM parents WHERE id = ?
			`, parentID); delParentErr != nil {
				return delParentErr
			}
		}
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
		var parentCount int
		if parentCountErr := tx.QueryRow(`
			SELECT COUNT(*) FROM parents WHERE topic_id = ?
		`, topicID).Scan(&parentCount); parentCountErr != nil {
			return parentCountErr
		}

		var chunkCount int
		if chunkCountErr := tx.QueryRow(`
			SELECT COUNT(*) FROM chunks WHERE topic_id = ?
		`, topicID).Scan(&chunkCount); chunkCountErr != nil {
			return chunkCountErr
		}

		if parentCount == 0 && chunkCount == 0 {
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

	return tx.Commit()
}

func insertParentRowRepo(exec sqlExecer, topicID string, parent NotebookParentInput) error {
	_, err := exec.Exec(`
		INSERT INTO parents (id, topic_id, heading, order_index, content_text)
		VALUES (?, ?, ?, ?, ?)
	`, parent.ID, topicID, parent.Heading, parent.OrderIndex, parent.Content)
	return err
}

func insertChunkRowRepo(exec sqlExecer, topicID string, chunk NotebookChunkInput) error {
	_, err := exec.Exec(`
		INSERT INTO chunks (id, topic_id, parent_id, chunk_text, token_count, importance_score, weakness_score)
		VALUES (?, ?, ?, ?, ?, 0, 0)
	`, chunk.ID, topicID, chunk.ParentID, chunk.Text, chunk.TokenCount)
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
		SELECT COUNT(1)
		FROM sqlite_master
		WHERE type = 'table' AND name = ?
	`, tableName).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}
