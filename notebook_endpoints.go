package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"ai-tutor/internal/db"
	"ai-tutor/internal/models"
	"ai-tutor/internal/notebook"
	"ai-tutor/internal/utils"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

const ingestionEventName = "ingestion-progress"

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
func (a *App) UploadNotebook(fileData []byte, fileName string) map[string]interface{} {
	if a.notebookService == nil {
		return map[string]interface{}{
			"error": "notebook service not initialized",
		}
	}

	uploadResult, err := a.notebookService.SaveUploadedFile(fileData, fileName)
	if err != nil {
		return map[string]interface{}{
			"error": err.Error(),
		}
	}

	return a.finalizeNotebookUpload(uploadResult)
}

// UploadNotebookFromPath stores a local file selected from desktop without bridge byte-array transfer.
func (a *App) UploadNotebookFromPath(filePath string) map[string]interface{} {
	if a.notebookService == nil {
		return map[string]interface{}{
			"error": "notebook service not initialized",
		}
	}

	uploadResult, err := a.notebookService.SaveUploadedFileFromPath(filePath)
	if err != nil {
		return map[string]interface{}{
			"error": err.Error(),
		}
	}

	return a.finalizeNotebookUpload(uploadResult)
}

func (a *App) finalizeNotebookUpload(uploadResult *notebook.UploadResult) map[string]interface{} {
	if uploadResult == nil {
		return map[string]interface{}{
			"error": "upload failed",
		}
	}

	// Extract normalized document content for metadata and downstream auto-analysis.
	doc, err := a.notebookService.ExtractDocument(uploadResult.FilePath, uploadResult.FileType)
	if err != nil {
		_ = a.notebookService.DeleteFile(uploadResult.FilePath)
		return map[string]interface{}{
			"error": err.Error(),
		}
	}

	// Create notebook record as unlinked; Sprint 11 uses a draft/confirm ingestion flow.
	err = db.CreateNotebook(uploadResult.ID, uploadResult.FileName, uploadResult.FilePath, uploadResult.FileType, "", doc.PageCount)
	if err != nil {
		_ = a.notebookService.DeleteFile(uploadResult.FilePath)
		return map[string]interface{}{
			"error": err.Error(),
		}
	}

	status := "uploaded"
	_ = db.UpdateNotebookStatus(uploadResult.ID, status)

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

// DraftNotebookSyllabus creates editable chapter ranges for HITL verification.
// If regenerate=false and a draft exists in DB, returns the persisted draft without re-running extraction/LLM.
// If regenerate=true or no draft exists, runs extraction/LLM and persists the result.
func (a *App) DraftNotebookSyllabus(notebookID string, regenerate bool) map[string]interface{} {
	notebookID = strings.TrimSpace(notebookID)
	if notebookID == "" {
		return map[string]interface{}{"error": "notebook id is required"}
	}
	if a.notebookService == nil {
		return map[string]interface{}{"error": "notebook service not initialized"}
	}

	nb, err := db.GetNotebookByID(notebookID)
	if err != nil {
		return map[string]interface{}{"error": err.Error()}
	}
	if nb == nil {
		return map[string]interface{}{"error": "notebook not found"}
	}

	// Try to load persisted draft if not regenerating
	if !regenerate {
		draftJSON, err := db.GetNotebookSyllabusDraft(notebookID)
		if err != nil {
			return map[string]interface{}{"error": err.Error()}
		}
		if draftJSON != "" {
			// Parse and return persisted draft
			var persistedDraft models.SyllabusDraft
			if err := json.Unmarshal([]byte(draftJSON), &persistedDraft); err == nil {
				return map[string]interface{}{
					"notebook_id":   notebookID,
					"page_count":    persistedDraft.PageCount,
					"chapters":      persistedDraft.Chapters,
					"status":        "draft_ready",
					"fallback_used": false,
				}
			}
		}
	}

	// No persisted draft or regenerate=true: run extraction and LLM
	// Use lightweight sample extraction for faster response time
	// Only extract first 30 pages for LLM context instead of full document
	doc, err := a.notebookService.ExtractDocumentSample(nb.FilePath, nb.FileType, 30)
	if err != nil {
		return map[string]interface{}{"error": err.Error()}
	}

	_ = db.UpdateNotebookStatus(notebookID, "analyzing")
	result, err := a.notebookService.DraftSyllabusChapters(nb.FileType, nb.FilePath, doc, a.heavyLLMProvider)
	if err != nil {
		_ = db.UpdateNotebookStatus(notebookID, "failed")
		return map[string]interface{}{"error": err.Error()}
	}

	chapters := result.Chapters
	fallbackUsed := result.FallbackUsed
	if len(chapters) == 0 {
		endPage := doc.PageCount
		if endPage <= 0 {
			endPage = 1
		}
		chapters = []models.SyllabusChapterDraft{{
			Title:     "General",
			StartPage: 1,
			EndPage:   endPage,
		}}
		fallbackUsed = true
	}

	// Persist the draft for future use
	draftToPersist := models.SyllabusDraft{
		PageCount: doc.PageCount,
		Chapters:  chapters,
	}
	draftJSON, err := json.Marshal(draftToPersist)
	if err == nil {
		_ = db.UpdateNotebookSyllabusDraft(notebookID, string(draftJSON))
	}

	_ = db.UpdateNotebookStatus(notebookID, "draft_ready")
	return map[string]interface{}{
		"notebook_id":   notebookID,
		"page_count":    doc.PageCount,
		"chapters":      chapters,
		"status":        "draft_ready",
		"fallback_used": fallbackUsed,
	}
}

// ConfirmNotebookSyllabus commits notebook ingestion from user-confirmed chapter bounds.
func (a *App) ConfirmNotebookSyllabus(notebookID string, chapters []models.SyllabusChapterDraft) map[string]interface{} {
	notebookID = strings.TrimSpace(notebookID)
	if notebookID == "" {
		return map[string]interface{}{"error": "notebook id is required"}
	}
	if a.notebookService == nil {
		return map[string]interface{}{"error": "notebook service not initialized"}
	}

	nb, err := db.GetNotebookByID(notebookID)
	if err != nil {
		return map[string]interface{}{"error": err.Error()}
	}
	if nb == nil {
		return map[string]interface{}{"error": "notebook not found"}
	}

	// Extract document only when a full re-ingest is necessary. We'll try to detect
	// whether a metadata-only or topic-metadata-only update is sufficient.
	normalized := notebook.NormalizeSyllabusChapters(chapters, nb.PageCount)
	if len(normalized) == 0 {
		return map[string]interface{}{"error": "at least one valid chapter is required"}
	}

	// Attempt to fetch existing topics/bounds for this notebook to decide path
	existingTopics, etErr := db.GetNotebookTopicsWithBounds(notebookID)
	if etErr != nil {
		// Log but continue with conservative full re-ingest flow
		utils.Warnf("ConfirmNotebookSyllabus: unable to load existing topics for %s: %v", notebookID, etErr)
	}

	// If notebook already chunked and we have existing topic info, compare bounds/titles
	if nb.Status == "chunked" && len(existingTopics) > 0 {
		boundsChanged := false
		titlesChanged := false

		if len(existingTopics) != len(normalized) {
			boundsChanged = true
		} else {
			for i := range normalized {
				if existingTopics[i].StartPage != normalized[i].StartPage || existingTopics[i].EndPage != normalized[i].EndPage {
					boundsChanged = true
					break
				}
				if strings.TrimSpace(existingTopics[i].Title) != strings.TrimSpace(normalized[i].Title) {
					titlesChanged = true
				}
			}
		}

		if !boundsChanged && !titlesChanged {
			// Nothing changed (no chapter or title changes) — treat as metadata_only/no-op
			utils.Infof("ConfirmNotebookSyllabus: metadata_only (no chapter/title changes) for %s", notebookID)
			return map[string]interface{}{
				"success":     true,
				"status":      nb.Status,
				"notebook_id": notebookID,
				"mode":        "metadata_only",
			}
		}

		if !boundsChanged && titlesChanged {
			// Only titles changed — update topic titles in-place and preserve chunks/vectors
			utils.Infof("ConfirmNotebookSyllabus: topic_metadata_only for %s — updating topic titles only", notebookID)

			topicItems := make([]db.TopicBatchItem, 0, len(existingTopics))
			topicIDs := make([]string, 0, len(existingTopics))
			for i, et := range existingTopics {
				topicItems = append(topicItems, db.TopicBatchItem{TopicID: et.TopicID, Title: normalized[i].Title})
				topicIDs = append(topicIDs, et.TopicID)
			}

			if err := db.EnsureTopicsBatch(topicItems); err != nil {
				_ = db.UpdateNotebookStatus(notebookID, "failed")
				return map[string]interface{}{"error": "failed to update topics: " + err.Error()}
			}

			if len(topicIDs) > 0 {
				_ = db.UpdateNotebookTopic(notebookID, topicIDs[0])
			}

			// Return without running extraction/ingestion or embedding updates
			return map[string]interface{}{
				"success":     true,
				"status":      nb.Status,
				"notebook_id": notebookID,
				"mode":        "topic_metadata_only",
				"topic_ids":   topicIDs,
			}
		}
		// If boundsChanged==true fall through to full re-ingest
	}

	// Full re-ingest path (extract document and rebuild chunks)
	doc, err := a.notebookService.ExtractDocument(nb.FilePath, nb.FileType)
	if err != nil {
		return map[string]interface{}{"error": err.Error()}
	}

	// Re-normalize with real page count from document
	normalized = notebook.NormalizeSyllabusChapters(chapters, doc.PageCount)
	if len(normalized) == 0 {
		return map[string]interface{}{"error": "at least one valid chapter is required"}
	}

	// Collect all topics and bounds for batch processing
	topicItems := make([]db.TopicBatchItem, 0, len(normalized))
	boundsItems := make([]db.TopicPageBoundsBatchItem, 0, len(normalized))
	topicIDs := make([]string, 0, len(normalized))

	for i, ch := range normalized {
		// Sanitize topic ID: lowercase, replace non-alphanumerics with hyphens, collapse duplicates
		sanitized := strings.ToLower(strings.TrimSpace(ch.Title))
		// Replace any character not in [a-z0-9] with hyphen
		var result []rune
		for _, r := range sanitized {
			if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
				result = append(result, r)
			} else {
				result = append(result, '-')
			}
		}
		sanitized = string(result)
		// Collapse duplicate hyphens
		for strings.Contains(sanitized, "--") {
			sanitized = strings.ReplaceAll(sanitized, "--", "-")
		}
		// Trim leading/trailing hyphens
		sanitized = strings.Trim(sanitized, "-")
		// Fallback if empty
		if sanitized == "" {
			sanitized = "topic"
		}
		// Limit length
		if len(sanitized) > 20 {
			sanitized = sanitized[:20]
		}
		topicID := fmt.Sprintf("nb-%s-ch-%02d-%s", notebookID, i+1, sanitized)
		topicIDs = append(topicIDs, topicID)

		topicItems = append(topicItems, db.TopicBatchItem{
			TopicID: topicID,
			Title:   ch.Title,
		})

		boundsItems = append(boundsItems, db.TopicPageBoundsBatchItem{
			TopicID:   topicID,
			StartPage: ch.StartPage,
			EndPage:   ch.EndPage,
		})
	}

	// Batch create/update topics
	if err := db.EnsureTopicsBatch(topicItems); err != nil {
		_ = db.UpdateNotebookStatus(notebookID, "failed")
		return map[string]interface{}{"error": "failed to create topics: " + err.Error()}
	}

	// Batch update page bounds
	if err := db.UpdateTopicPageBoundsBatch(boundsItems); err != nil {
		_ = db.UpdateNotebookStatus(notebookID, "failed")
		// Cleanup: delete created topic rows to avoid orphaned records
		for _, item := range topicItems {
			_ = db.DeleteTopic(item.TopicID)
		}
		return map[string]interface{}{"error": "failed to persist topic bounds: " + err.Error()}
	}

	if len(topicIDs) > 0 {
		_ = db.UpdateNotebookTopic(notebookID, topicIDs[0])
	}

	// Track which topic IDs were newly created for cleanup
	newlyCreatedTopicIDs := make(map[string]bool)
	for _, item := range topicItems {
		newlyCreatedTopicIDs[item.TopicID] = true
	}

	groups, allChunks := notebook.BuildTopicGroupsFromChapters(notebookID, doc, topicIDs, normalized)
	if len(groups) == 0 || len(allChunks) == 0 {
		_ = db.UpdateNotebookStatus(notebookID, "failed")
		// Cleanup: delete only newly created topic rows to avoid orphaned records
		for topicID := range newlyCreatedTopicIDs {
			_ = db.DeleteTopic(topicID)
		}
		return map[string]interface{}{"error": "confirmed chapters produced no chunks"}
	}

	utils.Infof("ConfirmNotebookSyllabus: full_reingest for %s — creating %d chunks", notebookID, len(allChunks))

	emitIngestionProgress(a, ingestionProgressPayload{
		NotebookID: notebookID,
		Status:     "chunking",
		Message:    fmt.Sprintf("Creating %d chunks for confirmed chapters", len(allChunks)),
		Phase:      "chunking",
		Processed:  0,
		Total:      len(allChunks),
		Percent:    20,
	})

	if err := db.IngestNotebookContentByTopic(notebookID, groups); err != nil {
		_ = db.UpdateNotebookStatus(notebookID, "failed")
		// Cleanup: delete only newly created topic rows to avoid orphaned records
		for topicID := range newlyCreatedTopicIDs {
			_ = db.DeleteTopic(topicID)
		}
		emitIngestionProgress(a, ingestionProgressPayload{
			NotebookID: notebookID,
			Status:     "failed",
			Message:    "Chunk ingestion failed",
			Phase:      "chunking",
			Processed:  0,
			Total:      len(allChunks),
			Percent:    100,
		})
		return map[string]interface{}{"error": "chunk ingestion failed: " + err.Error()}
	}

	if a.embedStore != nil {
		for _, chunk := range allChunks {
			a.embedStore.AddChunk(chunk)
		}
	}

	status := "chunked"
	emitIngestionProgress(a, ingestionProgressPayload{
		NotebookID: notebookID,
		Status:     status,
		Message:    "Chunking complete",
		Phase:      "complete",
		Processed:  len(allChunks),
		Total:      len(allChunks),
		Percent:    100,
	})

	_ = db.UpdateNotebookStatus(notebookID, status)
	return map[string]interface{}{
		"success":     true,
		"status":      status,
		"notebook_id": notebookID,
		"topic_ids":   topicIDs,
		"chunk_count": len(allChunks),
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
			"id":              nb.ID,
			"title":           nb.Title,
			"file_type":       nb.FileType,
			"topic_id":        nb.TopicID,
			"status":          nb.Status,
			"indexing_status": nb.IndexingStatus,
			"page_count":      nb.PageCount,
			"chunk_count":     nb.ChunkCount,
			"priority":        nb.Priority,
			"uploaded_at":     nb.UploadedAt,
		})
	}

	return result
}

// GetNotebookTopicTree returns notebook-scoped topic options for hierarchical selectors.
func (a *App) GetNotebookTopicTree() ([]models.NotebookTopicTreeNode, error) {
	tree, err := db.GetNotebookTopicTree()
	if err != nil {
		return nil, err
	}

	return tree, nil
}

func emitIngestionProgress(a *App, payload ingestionProgressPayload) {
	if a == nil || a.ctx == nil {
		return
	}
	wailsruntime.EventsEmit(a.ctx, ingestionEventName, payload)
}

// UpdateNotebookTitle updates notebook metadata for user edits before re-ingestion.
func (a *App) UpdateNotebookTitle(notebookID string, title string) map[string]interface{} {
	notebookID = strings.TrimSpace(notebookID)
	title = strings.TrimSpace(title)
	if notebookID == "" {
		return map[string]interface{}{"error": "notebook id is required"}
	}
	if title == "" {
		return map[string]interface{}{"error": "title is required"}
	}

	if err := db.UpdateNotebookTitle(notebookID, title); err != nil {
		return map[string]interface{}{"error": err.Error()}
	}

	return map[string]interface{}{"success": true}
}

// UpdateNotebookPriority updates the notebook priority level (1-10).
func (a *App) UpdateNotebookPriority(notebookID string, priority int) map[string]interface{} {
	notebookID = strings.TrimSpace(notebookID)
	if notebookID == "" {
		return map[string]interface{}{"error": "notebook id is required"}
	}
	// Clamp priority to valid range 1-10
	if priority < 1 {
		priority = 1
	}
	if priority > 10 {
		priority = 10
	}

	if err := db.UpdateNotebookPriority(notebookID, priority); err != nil {
		return map[string]interface{}{"error": err.Error()}
	}

	return map[string]interface{}{"success": true}
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
