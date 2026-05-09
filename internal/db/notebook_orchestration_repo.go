package db

import (
	"database/sql"
	"fmt"
	"strings"
)

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

	// Delete reading_progress for tasks associated with this notebook first
	// (reading_progress references study_queue, which references notebooks)
	if _, err := tx.Exec(`
		DELETE FROM reading_progress
		WHERE task_id IN (
			SELECT id FROM study_queue WHERE notebook_id = ?
		)
	`, notebookID); err != nil {
		return err
	}

	// Delete quiz_attempts for tasks associated with this notebook
	if _, err := tx.Exec(`
		DELETE FROM quiz_attempts
		WHERE task_id IN (
			SELECT id FROM study_queue WHERE notebook_id = ?
		)
	`, notebookID); err != nil {
		return err
	}

	// Delete study_queue entries for this notebook (foreign key to notebooks)
	if _, err := tx.Exec(`
		DELETE FROM study_queue WHERE notebook_id = ?
	`, notebookID); err != nil {
		return err
	}

	// Delete notebook_chunks entries (foreign key to notebooks)
	if _, err := tx.Exec(`
		DELETE FROM notebook_chunks WHERE notebook_id = ?
	`, notebookID); err != nil {
		return err
	}

	parentIDs := make(map[string]struct{})
	chunkIDs := make([]string, 0)
	parentRows, err := tx.Query(`
		SELECT DISTINCT c.parent_id, c.id
		FROM chunks c
		WHERE c.topic_id IN (
			SELECT topic_id FROM notebook_topics WHERE notebook_id = ?
			UNION
			SELECT id FROM topics WHERE id LIKE ?
		)
	`, notebookID, "nb-"+notebookID+"-%")
	if err != nil {
		return err
	}

	for parentRows.Next() {
		var parentID string
		var chunkID string
		if scanErr := parentRows.Scan(&parentID, &chunkID); scanErr != nil {
			_ = parentRows.Close()
			return scanErr
		}
		parentIDs[parentID] = struct{}{}
		chunkIDs = append(chunkIDs, chunkID)
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

	_, err = tx.Exec("DELETE FROM notebook_chunks WHERE notebook_id = ?", notebookID)
	if err != nil {
		return err
	}

	// Bulk delete chunks using IN clause for better performance
	if len(chunkIDs) > 0 {
		placeholders := make([]string, len(chunkIDs))
		args := make([]interface{}, len(chunkIDs))
		for i, chunkID := range chunkIDs {
			placeholders[i] = "?"
			args[i] = chunkID
		}

		query := fmt.Sprintf(`DELETE FROM chunks WHERE id IN (%s)`, strings.Join(placeholders, ","))
		if _, delChunkErr := tx.Exec(query, args...); delChunkErr != nil {
			return delChunkErr
		}
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
		INSERT INTO chunks (id, topic_id, parent_id, chunk_text, page_num, token_count, importance_score, weakness_score)
		VALUES (?, ?, ?, ?, ?, ?, 0, 0)
	`, chunk.ID, topicID, chunk.ParentID, chunk.Text, chunk.PageNum, chunk.TokenCount)
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
