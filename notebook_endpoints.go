package main

import (
	"ai-tutor/internal/db"
)

// UploadNotebook handles file upload and creates notebook record
func (a *App) UploadNotebook(fileData []byte, fileName string, topicID string) map[string]interface{} {
	if a.notebookService == nil {
		return map[string]interface{}{
			"error": "notebook service not initialized",
		}
	}

	// Save file to disk
	uploadResult, err := a.notebookService.SaveUploadedFile(fileData, fileName)
	if err != nil {
		return map[string]interface{}{
			"error": err.Error(),
		}
	}

	// Extract metadata
	meta, err := a.notebookService.ExtractMetadata(uploadResult.FilePath, uploadResult.FileType)
	if err != nil {
		return map[string]interface{}{
			"error": err.Error(),
		}
	}

	// Create notebook record in database
	err = db.CreateNotebook(uploadResult.ID, fileName, uploadResult.FilePath, uploadResult.FileType, topicID, meta.PageCount)
	if err != nil {
		return map[string]interface{}{
			"error": err.Error(),
		}
	}

	return map[string]interface{}{
		"id":         uploadResult.ID,
		"file_name":  uploadResult.FileName,
		"file_type":  uploadResult.FileType,
		"size":       uploadResult.Size,
		"page_count": meta.PageCount,
	}
}

// GetNotebooks retrieves all notebooks, optionally filtered by topic
func (a *App) GetNotebooks(topicID string) []map[string]interface{} {
	notebooks, err := db.GetNotebooks(topicID)
	if err != nil {
		return []map[string]interface{}{
			{"error": err.Error()},
		}
	}

	var result []map[string]interface{}
	for _, nb := range notebooks {
		result = append(result, map[string]interface{}{
			"id":          nb.ID,
			"title":       nb.Title,
			"file_type":   nb.FileType,
			"topic_id":    nb.TopicID,
			"page_count":  nb.PageCount,
			"chunk_count": nb.ChunkCount,
			"uploaded_at": nb.UploadedAt,
		})
	}

	return result
}

// DeleteNotebook removes a notebook and its associated file
func (a *App) DeleteNotebook(notebookID string) map[string]interface{} {
	if a.notebookService == nil {
		return map[string]interface{}{
			"error": "notebook service not initialized",
		}
	}

	// Get notebook to retrieve file path
	nb, err := db.GetNotebookByID(notebookID)
	if err != nil {
		return map[string]interface{}{
			"error": err.Error(),
		}
	}

	if nb == nil {
		return map[string]interface{}{
			"error": "notebook not found",
		}
	}

	// Delete file from disk
	if err := a.notebookService.DeleteFile(nb.FilePath); err != nil {
		return map[string]interface{}{
			"error": err.Error(),
		}
	}

	// Delete database record
	if err := db.DeleteNotebook(notebookID); err != nil {
		return map[string]interface{}{
			"error": err.Error(),
		}
	}

	return map[string]interface{}{
		"success": true,
	}
}
