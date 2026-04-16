package db

import (
	"fmt"
	"sort"
	"strings"

	"ai-tutor/internal/models"
)

// GetNotebookTopicTree returns notebooks with their discovered topics derived from linked chunks.
func GetNotebookTopicTree() ([]models.NotebookTopicTreeNode, error) {
	notebookRows, err := conn.Query(`
		SELECT id, title
		FROM notebooks
		ORDER BY uploaded_at DESC, id ASC
	`)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = notebookRows.Close()
	}()

	tree := make([]models.NotebookTopicTreeNode, 0)
	for notebookRows.Next() {
		var notebookID string
		var notebookTitle string

		if err := notebookRows.Scan(&notebookID, &notebookTitle); err != nil {
			return nil, err
		}

		topics, topicsErr := getNotebookTopics(notebookID)
		if topicsErr != nil {
			return nil, topicsErr
		}

		tree = append(tree, models.NotebookTopicTreeNode{
			NotebookID: notebookID,
			Title:      notebookTitle,
			Topics:     topics,
		})
	}

	if err := notebookRows.Err(); err != nil {
		return nil, err
	}

	return tree, nil
}

func getNotebookTopics(notebookID string) ([]models.NotebookTopicTreeTopic, error) {
	seen := map[string]struct{}{}
	ordered := make([]models.NotebookTopicTreeTopic, 0, 16)

	// First, prefer notebook-scoped chapter topics created during ingestion.
	prefix := fmt.Sprintf("nb-%s-ch-", notebookID)
	canonicalRows, err := conn.Query(`
		SELECT id, title
		FROM topics
		WHERE id LIKE ?
		ORDER BY id ASC
	`, prefix+"%")
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = canonicalRows.Close()
	}()

	for canonicalRows.Next() {
		var topicID string
		var title string
		if err := canonicalRows.Scan(&topicID, &title); err != nil {
			return nil, err
		}
		addTopic(&ordered, seen, topicID, title)
	}
	if err := canonicalRows.Err(); err != nil {
		return nil, err
	}

	// Also include linked chunk topics for legacy notebooks or manual linkage.
	chunkRows, err := conn.Query(`
		SELECT DISTINCT COALESCE(t.id, ''), COALESCE(t.title, '')
		FROM notebook_chunks nc
		LEFT JOIN chunks c ON c.id = nc.chunk_id
		LEFT JOIN topics t ON t.id = c.topic_id
		WHERE nc.notebook_id = ?
	`, notebookID)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = chunkRows.Close()
	}()

	chunkTopics := make([]models.NotebookTopicTreeTopic, 0, 8)
	for chunkRows.Next() {
		var topicID string
		var title string
		if err := chunkRows.Scan(&topicID, &title); err != nil {
			return nil, err
		}
		if strings.TrimSpace(topicID) == "" || strings.TrimSpace(title) == "" {
			continue
		}
		if _, ok := seen[topicID]; ok {
			continue
		}
		chunkTopics = append(chunkTopics, models.NotebookTopicTreeTopic{
			TopicID: topicID,
			Title:   title,
		})
	}
	if err := chunkRows.Err(); err != nil {
		return nil, err
	}

	sort.Slice(chunkTopics, func(i, j int) bool {
		if chunkTopics[i].Title == chunkTopics[j].Title {
			return chunkTopics[i].TopicID < chunkTopics[j].TopicID
		}
		return chunkTopics[i].Title < chunkTopics[j].Title
	})
	for _, topic := range chunkTopics {
		addTopic(&ordered, seen, topic.TopicID, topic.Title)
	}

	return ordered, nil
}

func addTopic(target *[]models.NotebookTopicTreeTopic, seen map[string]struct{}, topicID, title string) {
	id := strings.TrimSpace(topicID)
	name := strings.TrimSpace(title)
	if id == "" || name == "" {
		return
	}
	if _, ok := seen[id]; ok {
		return
	}
	seen[id] = struct{}{}
	*target = append(*target, models.NotebookTopicTreeTopic{
		TopicID: id,
		Title:   name,
	})
}
