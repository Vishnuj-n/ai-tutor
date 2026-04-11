package main

import (
	"crypto/md5"
	"fmt"

	"ai-tutor/internal/db"
	"ai-tutor/internal/models"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

const ingestionEventName = "ingestion-progress"
const ingestionBatchSize = 20

type ingestionProgressPayload struct {
	NotebookID   string `json:"notebook_id"`
	TopicID      string `json:"topic_id"`
	Status       string `json:"status"`
	Message      string `json:"message"`
	Phase        string `json:"phase"`
	Processed    int    `json:"processed"`
	Total        int    `json:"total"`
	IndexedCount int    `json:"indexed_count"`
	FailedCount  int    `json:"failed_count"`
	Percent      int    `json:"percent"`
}

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

	// Extract normalized document content for metadata + deterministic chunking.
	doc, err := a.notebookService.ExtractDocument(uploadResult.FilePath, uploadResult.FileType)
	if err != nil {
		_ = a.notebookService.DeleteFile(uploadResult.FilePath)
		return map[string]interface{}{
			"error": err.Error(),
		}
	}

	// Create notebook record in database
	err = db.CreateNotebook(uploadResult.ID, fileName, uploadResult.FilePath, uploadResult.FileType, topicID, doc.PageCount)
	if err != nil {
		_ = a.notebookService.DeleteFile(uploadResult.FilePath)
		return map[string]interface{}{
			"error": err.Error(),
		}
	}

	status := "uploaded"
	chunkCount := 0
	indexedCount := 0
	failedCount := 0

	if topicID == "" {
		status = "uploaded_unlinked"
		_ = db.UpdateNotebookStatus(uploadResult.ID, status)
	} else {
		ingestionData, buildErr := a.notebookService.BuildIngestionData(uploadResult.ID, doc)
		if buildErr != nil {
			_ = db.UpdateNotebookStatus(uploadResult.ID, "failed")
			return map[string]interface{}{
				"error": buildErr.Error(),
			}
		}

		parents := make([]db.NotebookParentInput, 0, len(ingestionData.Parents))
		for _, parent := range ingestionData.Parents {
			parents = append(parents, db.NotebookParentInput{
				ID:         parent.ID,
				Heading:    parent.Heading,
				Content:    parent.Content,
				OrderIndex: parent.OrderIndex,
			})
		}

		chunks := make([]db.NotebookChunkInput, 0, len(ingestionData.Chunks))
		for _, chunk := range ingestionData.Chunks {
			chunks = append(chunks, db.NotebookChunkInput{
				ID:         chunk.ID,
				ParentID:   chunk.ParentID,
				Text:       chunk.Text,
				TokenCount: chunk.TokenCount,
				PageNum:    chunk.PageNum,
			})
		}

		ingestErr := db.IngestNotebookContent(uploadResult.ID, topicID, parents, chunks)
		if ingestErr != nil {
			_ = db.UpdateNotebookStatus(uploadResult.ID, "failed")
			return map[string]interface{}{
				"error": ingestErr.Error(),
			}
		}

		status = "chunked"
		chunkCount = len(chunks)

		if a.embedStore != nil {
			for _, chunk := range chunks {
				a.embedStore.AddChunk(models.Chunk{
					ID:              chunk.ID,
					TopicID:         topicID,
					ParentID:        chunk.ParentID,
					Text:            chunk.Text,
					ImportanceScore: 0,
					WeaknessScore:   0,
				})
			}
		}

		if a.embedder == nil {
			emitIngestionProgress(a, ingestionProgressPayload{
				NotebookID: uploadResult.ID,
				TopicID:    topicID,
				Status:     status,
				Message:    "Chunking complete; vector indexing skipped because embedder is unavailable",
				Phase:      "indexing",
				Processed:  chunkCount,
				Total:      chunkCount,
				Percent:    100,
			})
		} else {
			status = "indexing"
			_ = db.UpdateNotebookStatus(uploadResult.ID, status)

			emitIngestionProgress(a, ingestionProgressPayload{
				NotebookID: uploadResult.ID,
				TopicID:    topicID,
				Status:     status,
				Message:    "Starting vector indexing",
				Phase:      "indexing",
				Processed:  0,
				Total:      chunkCount,
				Percent:    0,
			})

			cancelled := false
			for i, chunk := range chunks {
				if a.ctx != nil {
					select {
					case <-a.ctx.Done():
						cancelled = true
						break
					default:
					}
				}
				if cancelled {
					break
				}

				vector, embedErr := a.embedder.Embed(chunk.Text)
				if embedErr != nil {
					failedCount++
				} else {
					storeErr := db.UpsertChunkVector(chunk.ID, vector)
					if storeErr != nil {
						failedCount++
					} else {
						indexedCount++
						hash := computeChunkHash(chunk.Text)
						_ = db.UpdateChunkEmbedding(chunk.ID, hash)
					}
				}

				processed := i + 1
				if processed%ingestionBatchSize == 0 || processed == chunkCount {
					percent := calculatePercent(processed, chunkCount)
					emitIngestionProgress(a, ingestionProgressPayload{
						NotebookID:   uploadResult.ID,
						TopicID:      topicID,
						Status:       status,
						Message:      fmt.Sprintf("Indexing chunk %d/%d", processed, chunkCount),
						Phase:        "indexing",
						Processed:    processed,
						Total:        chunkCount,
						IndexedCount: indexedCount,
						FailedCount:  failedCount,
						Percent:      percent,
					})
				}
			}

			if cancelled {
				status = "partial_indexed"
				emitIngestionProgress(a, ingestionProgressPayload{
					NotebookID:   uploadResult.ID,
					TopicID:      topicID,
					Status:       status,
					Message:      "Indexing cancelled",
					Phase:        "indexing",
					Processed:    indexedCount + failedCount,
					Total:        chunkCount,
					IndexedCount: indexedCount,
					FailedCount:  failedCount,
					Percent:      calculatePercent(indexedCount+failedCount, chunkCount),
				})
			} else if failedCount > 0 {
				status = "partial_indexed"
				emitIngestionProgress(a, ingestionProgressPayload{
					NotebookID:   uploadResult.ID,
					TopicID:      topicID,
					Status:       status,
					Message:      "Indexing completed with partial failures",
					Phase:        "indexing",
					Processed:    chunkCount,
					Total:        chunkCount,
					IndexedCount: indexedCount,
					FailedCount:  failedCount,
					Percent:      100,
				})
			} else {
				status = "indexed"
				emitIngestionProgress(a, ingestionProgressPayload{
					NotebookID:   uploadResult.ID,
					TopicID:      topicID,
					Status:       status,
					Message:      "Vector indexing complete",
					Phase:        "indexing",
					Processed:    chunkCount,
					Total:        chunkCount,
					IndexedCount: indexedCount,
					FailedCount:  0,
					Percent:      100,
				})
			}

			_ = db.UpdateNotebookStatus(uploadResult.ID, status)
		}
	}

	return map[string]interface{}{
		"id":            uploadResult.ID,
		"file_name":     uploadResult.FileName,
		"file_type":     uploadResult.FileType,
		"size":          uploadResult.Size,
		"page_count":    doc.PageCount,
		"word_count":    doc.WordCount,
		"chunk_count":   chunkCount,
		"indexed_count": indexedCount,
		"failed_count":  failedCount,
		"status":        status,
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
			"status":      nb.Status,
			"page_count":  nb.PageCount,
			"chunk_count": nb.ChunkCount,
			"uploaded_at": nb.UploadedAt,
		})
	}

	return result
}

func emitIngestionProgress(a *App, payload ingestionProgressPayload) {
	if a == nil || a.ctx == nil {
		return
	}
	wailsruntime.EventsEmit(a.ctx, ingestionEventName, payload)
}

func calculatePercent(processed, total int) int {
	if total <= 0 {
		return 0
	}
	if processed >= total {
		return 100
	}
	if processed <= 0 {
		return 0
	}
	return int(float64(processed) / float64(total) * 100)
}

func computeChunkHash(text string) string {
	hash := md5.Sum([]byte(text))
	return fmt.Sprintf("%x", hash)
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
