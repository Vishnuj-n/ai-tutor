package db

import (
	"log"

	"ai-tutor/internal/models"
)

// GetNotebookTopicTree returns notebooks with their discovered topics derived from linked chunks.
func GetNotebookTopicTree() ([]models.NotebookTopicTreeNode, error) {
	log.Printf("[GetNotebookTopicTree] Starting query")
	rows, err := conn.Query(`
		SELECT
			n.id,
			n.title,
			COALESCE(t.id, ''),
			COALESCE(t.title, '')
		FROM notebooks n
		LEFT JOIN notebook_chunks nc ON nc.notebook_id = n.id
		LEFT JOIN chunks c ON c.id = nc.chunk_id
		LEFT JOIN topics t ON t.id = c.topic_id
		ORDER BY n.uploaded_at DESC, t.title ASC, t.id ASC
	`)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = rows.Close()
	}()

	tree := make([]models.NotebookTopicTreeNode, 0)
	notebookIndex := make(map[string]int)
	seenTopics := make(map[string]map[string]struct{})

	for rows.Next() {
		var notebookID string
		var notebookTitle string
		var topicID string
		var topicTitle string

		if err := rows.Scan(&notebookID, &notebookTitle, &topicID, &topicTitle); err != nil {
			return nil, err
		}

		idx, exists := notebookIndex[notebookID]
		if !exists {
			tree = append(tree, models.NotebookTopicTreeNode{
				NotebookID: notebookID,
				Title:      notebookTitle,
				Topics:     []models.NotebookTopicTreeTopic{},
			})
			idx = len(tree) - 1
			notebookIndex[notebookID] = idx
			seenTopics[notebookID] = make(map[string]struct{})
		}

		if topicID == "" || topicTitle == "" {
			continue
		}

		if _, duplicate := seenTopics[notebookID][topicID]; duplicate {
			continue
		}

		tree[idx].Topics = append(tree[idx].Topics, models.NotebookTopicTreeTopic{
			TopicID: topicID,
			Title:   topicTitle,
		})
		seenTopics[notebookID][topicID] = struct{}{}
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	log.Printf("[GetNotebookTopicTree] Returning %d notebooks with topics", len(tree))
	for i, nb := range tree {
		log.Printf("[GetNotebookTopicTree] Notebook[%d]: id=%s title=%s topics=%d", i, nb.NotebookID, nb.Title, len(nb.Topics))
		for j, topic := range nb.Topics {
			log.Printf("[GetNotebookTopicTree]   Topic[%d]: id=%s title=%s", j, topic.TopicID, topic.Title)
		}
	}
	return tree, nil
}
