package notebook

import (
	"ai-tutor/internal/db"
	"ai-tutor/internal/models"
)

// GetNotebookByID returns notebook metadata by ID.
func (s *Service) GetNotebookByID(notebookID string) (*models.Notebook, error) {
	return db.GetNotebookByID(notebookID)
}

// IngestNotebookContentByTopic delegates topic-group ingestion to the DB transaction layer.
func (s *Service) IngestNotebookContentByTopic(notebookID string, groups []db.NotebookTopicIngestionGroup) error {
	return db.IngestNotebookContentByTopic(notebookID, groups)
}

// DeleteNotebookRecords removes notebook-linked relational data.
func (s *Service) DeleteNotebookRecords(notebookID string) error {
	return db.DeleteNotebook(notebookID)
}
