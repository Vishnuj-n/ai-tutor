package api

import (
	"fmt"
	"context"

	"ai-tutor/internal/db"
	"ai-tutor/internal/models"
	"ai-tutor/internal/notebook"
	"ai-tutor/internal/parser"
	"ai-tutor/internal/tutor"

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

type AppInterface interface {
	GetNotebookService() *notebook.Service
	GetLLMProvider() parser.LLMProvider
	GetEmbedStore() EmbedStoreInterface
	GetEmbedder() EmbedderInterface
	GetContext() context.Context
}

type EmbedStoreInterface interface {
	AddChunk(chunk models.Chunk)
}

type EmbedderInterface interface {
	Embed(text string) ([]float32, error)
}

// UploadNotebook handles file upload and creates notebook record
func UploadNotebook(a AppInterface, fileData []byte, fileName string) map[string]interface{} {
	if a.GetNotebookService() == nil {
		return map[string]interface{}{
			"error": "notebook service not initialized",
		}
	}

	// Save file to disk
	uploadResult, err := a.GetNotebookService().SaveUploadedFile(fileData, fileName)
	if err != nil {
		return map[string]interface{}{
			"error": err.Error(),
		}
	}

	// Extract normalized document content for metadata and downstream auto-analysis.
	doc, err := a.GetNotebookService().ExtractDocument(uploadResult.FilePath, uploadResult.FileType)
	if err != nil {
		_ = a.GetNotebookService().DeleteFile(uploadResult.FilePath)
		return map[string]interface{}{
			"error": err.Error(),
		}
	}

	// Create notebook record as unlinked; auto-analysis will create/link topics asynchronously.
	err = db.CreateNotebook(uploadResult.ID, fileName, uploadResult.FilePath, uploadResult.FileType, "", doc.PageCount)
	if err != nil {
		_ = a.GetNotebookService().DeleteFile(uploadResult.FilePath)
		return map[string]interface{}{
			"error": err.Error(),
		}
	}

	status := "analyzing"
	_ = db.UpdateNotebookStatus(uploadResult.ID, status)

	emitIngestionProgress(a, ingestionProgressPayload{
		NotebookID: uploadResult.ID,
		Status:     status,
		Message:    "Analyzing notebook structure",
		Phase:      "analysis",
		Processed:  0,
		Total:      0,
		Percent:    0,
	})

	go processNotebookAutoIngestion(a, uploadResult.ID, doc)

	return map[string]interface{}{
		"id":            uploadResult.ID,
		"file_name":     uploadResult.FileName,
		"file_type":     uploadResult.FileType,
		"size":          uploadResult.Size,
		"page_count":    doc.PageCount,
		"word_count":    doc.WordCount,
		"chunk_count":   0,
		"indexed_count": 0,
		"failed_count":  0,
		"status":        status,
	}
}

func processNotebookAutoIngestion(a AppInterface, notebookID string, doc *notebook.ExtractedDocument) {
	chapters := tutor.ExtractChapterTitles(a.GetLLMProvider(), doc)
	if len(chapters) == 0 {
		chapters = []string{"General"}
	}

	topicIDs := make([]string, 0, len(chapters))
	topicTitles := make([]string, 0, len(chapters))
	for i, title := range chapters {
		normalized := parser.Slugify(title)
		if normalized == "" {
			continue
		}
		topicID := fmt.Sprintf("nb-%s-ch-%02d-%s", notebookID, i+1, normalized)
		if err := db.EnsureTopic(topicID, title); err != nil {
			_ = db.UpdateNotebookStatus(notebookID, "failed")
			emitIngestionProgress(a, ingestionProgressPayload{
				NotebookID: notebookID,
				Status:     "failed",
				Message:    "Failed to create topics for notebook",
				Phase:      "analysis",
				Percent:    100,
			})
			return
		}
		topicIDs = append(topicIDs, topicID)
		topicTitles = append(topicTitles, title)
	}
	if len(topicIDs) == 0 {
		topicID := fmt.Sprintf("nb-%s-general", notebookID)
		_ = db.EnsureTopic(topicID, "General")
		topicIDs = []string{topicID}
		topicTitles = []string{"General"}
	}

	_ = db.UpdateNotebookTopic(notebookID, topicIDs[0])

	emitIngestionProgress(a, ingestionProgressPayload{
		NotebookID: notebookID,
		Status:     "analyzing",
		Message:    fmt.Sprintf("Detected %d chapter topics", len(topicIDs)),
		Phase:      "analysis",
		Percent:    20,
	})

	groups, allChunks := parser.BuildTopicGroups(notebookID, doc, topicIDs, topicTitles)
	if len(groups) == 0 || len(allChunks) == 0 {
		_ = db.UpdateNotebookStatus(notebookID, "failed")
		emitIngestionProgress(a, ingestionProgressPayload{
			NotebookID: notebookID,
			Status:     "failed",
			Message:    "Document produced no chunks",
			Phase:      "chunking",
			Percent:    100,
		})
		return
	}

	if err := a.GetNotebookService().IngestNotebookContentByTopic(notebookID, groups); err != nil {
		_ = db.UpdateNotebookStatus(notebookID, "failed")
		emitIngestionProgress(a, ingestionProgressPayload{
			NotebookID: notebookID,
			Status:     "failed",
			Message:    "Chunk ingestion failed",
			Phase:      "chunking",
			Percent:    100,
		})
		return
	}

	if a.GetEmbedStore() != nil {
		for _, chunk := range allChunks {
			a.GetEmbedStore().AddChunk(chunk)
		}
	}

	status := "chunked"
	chunkCount := len(allChunks)
	if a.GetEmbedder() == nil {
		_ = db.UpdateNotebookStatus(notebookID, status)
		emitIngestionProgress(a, ingestionProgressPayload{
			NotebookID: notebookID,
			Status:     status,
			Message:    "Chunking complete; vector indexing skipped because embedder is unavailable",
			Phase:      "indexing",
			Processed:  chunkCount,
			Total:      chunkCount,
			Percent:    100,
		})
		return
	}

	status = "indexing"
	_ = db.UpdateNotebookStatus(notebookID, status)
	emitIngestionProgress(a, ingestionProgressPayload{
		NotebookID: notebookID,
		Status:     status,
		Message:    "Starting vector indexing",
		Phase:      "indexing",
		Processed:  0,
		Total:      chunkCount,
		Percent:    30,
	})

	indexedCount := 0
	failedCount := 0
	for i, chunk := range allChunks {
		vector, embedErr := a.GetEmbedder().Embed(chunk.Text)
		if embedErr != nil {
			failedCount++
		} else if storeErr := db.UpsertChunkVector(chunk.ID, vector); storeErr != nil {
			failedCount++
		} else {
			indexedCount++
		}

		processed := i + 1
		if processed%ingestionBatchSize == 0 || processed == chunkCount {
			emitIngestionProgress(a, ingestionProgressPayload{
				NotebookID:   notebookID,
				Status:       status,
				Message:      fmt.Sprintf("Indexing chunk %d/%d", processed, chunkCount),
				Phase:        "indexing",
				Processed:    processed,
				Total:        chunkCount,
				IndexedCount: indexedCount,
				FailedCount:  failedCount,
				Percent:      calculatePercent(processed, chunkCount),
			})
		}
	}

	if failedCount > 0 {
		status = "partial_indexed"
		emitIngestionProgress(a, ingestionProgressPayload{
			NotebookID:   notebookID,
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
			NotebookID:   notebookID,
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

	_ = db.UpdateNotebookStatus(notebookID, status)
}

// GetNotebooks retrieves all notebooks, optionally filtered by topic
func GetNotebooks(topicID string) []map[string]interface{} {
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

// GetNotebookTopicTree returns notebook-scoped topic options for hierarchical selectors.
func GetNotebookTopicTree() ([]models.NotebookTopicTreeNode, error) {
	tree, err := db.GetNotebookTopicTree()
	if err != nil {
		return nil, err
	}

	return tree, nil
}

// DeleteNotebook removes a notebook and its associated file
func DeleteNotebook(a AppInterface, notebookID string) map[string]interface{} {
	if a.GetNotebookService() == nil {
		return map[string]interface{}{
			"error": "notebook service not initialized",
		}
	}

	// Get notebook to retrieve file path
	nb, err := a.GetNotebookService().GetNotebookByID(notebookID)
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
	if err := a.GetNotebookService().DeleteFile(nb.FilePath); err != nil {
		return map[string]interface{}{
			"error": err.Error(),
		}
	}

	// Delete database record
	if err := a.GetNotebookService().DeleteNotebookRecords(notebookID); err != nil {
		return map[string]interface{}{
			"error": err.Error(),
		}
	}

	return map[string]interface{}{
		"success": true,
	}
}

func emitIngestionProgress(a AppInterface, payload ingestionProgressPayload) {
	if a == nil || a.GetContext() == nil {
		return
	}
	wailsruntime.EventsEmit(a.GetContext(), ingestionEventName, payload)
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
