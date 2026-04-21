package main

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"ai-tutor/internal/db"
	"ai-tutor/internal/embeddings"
	"ai-tutor/internal/models"
	"ai-tutor/internal/notebook"
	"ai-tutor/internal/utils"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

const ingestionEventName = "ingestion-progress"
const ingestionBatchSize = 20
const topicExtractionMaxChars = 30000
const topicExtractionMaxSections = 30

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

	// Save file to disk
	uploadResult, err := a.notebookService.SaveUploadedFile(fileData, fileName)
	if err != nil {
		return map[string]interface{}{
			"error": err.Error(),
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

	// Create notebook record as unlinked; auto-analysis will create/link topics asynchronously.
	err = db.CreateNotebook(uploadResult.ID, fileName, uploadResult.FilePath, uploadResult.FileType, "", doc.PageCount)
	if err != nil {
		_ = a.notebookService.DeleteFile(uploadResult.FilePath)
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

	go a.processNotebookAutoIngestion(uploadResult.ID, doc)

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

func (a *App) processNotebookAutoIngestion(notebookID string, doc *notebook.ExtractedDocument) {
	chapters := a.extractChapterTitles(doc)
	if len(chapters) == 0 {
		chapters = []string{"General"}
	}

	topicIDs := make([]string, 0, len(chapters))
	topicTitles := make([]string, 0, len(chapters))
	for i, title := range chapters {
		normalized := strings.TrimSpace(title)
		if normalized == "" {
			continue
		}
		topicID := fmt.Sprintf("nb-%s-ch-%02d-%s", notebookID, i+1, slugify(normalized))
		if err := db.EnsureTopic(topicID, normalized); err != nil {
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
		topicTitles = append(topicTitles, normalized)
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

	groups, allChunks := buildTopicGroups(notebookID, doc, topicIDs, topicTitles)
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

	if err := a.notebookService.IngestNotebookContentByTopic(notebookID, groups); err != nil {
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

	if a.embedStore != nil {
		for _, chunk := range allChunks {
			a.embedStore.AddChunk(chunk)
		}
	}

	status := "chunked"
	chunkCount := len(allChunks)
	if a.embedder == nil {
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
		vector, embedErr := a.embedder.Embed(chunk.Text)
		if embedErr != nil {
			failedCount++
		} else if storeErr := db.UpsertChunkVector(chunk.ID, vector); storeErr != nil {
			failedCount++
		} else {
			indexedCount++
			hash := computeChunkHash(chunk.Text)
			_ = db.UpdateChunkEmbedding(chunk.ID, hash)
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

func (a *App) extractChapterTitles(doc *notebook.ExtractedDocument) []string {
	if doc == nil || len(doc.Sections) == 0 {
		return []string{"General"}
	}

	input := make([]string, 0, len(doc.Sections))
	for i, section := range doc.Sections {
		if i >= topicExtractionMaxSections {
			break
		}
		if strings.TrimSpace(section.Text) == "" {
			continue
		}
		input = append(input, section.Text)
		if len(strings.Join(input, "\n")) > topicExtractionMaxChars {
			break
		}
	}
	joined := strings.Join(input, "\n")
	if len(joined) > topicExtractionMaxChars {
		joined = joined[:topicExtractionMaxChars]
	}
	if strings.TrimSpace(joined) == "" {
		return []string{"General"}
	}

	if a.heavyLLMProvider == nil {
		return fallbackChapterTitles(doc)
	}

	prompt := "Extract all chapter or major topic headings from this PDF text sample in original order. Include every distinct chapter/topic you can infer from the material. Return strict JSON only as {\"chapters\":[\"Title 1\",\"Title 2\"]}. No markdown, no prose, no keys besides chapters.\\n\\n" + joined
	response, err := a.heavyLLMProvider.GenerateAnswer(prompt)
	if err != nil {
		return fallbackChapterTitles(doc)
	}

	chapters := parseChapterTitles(response)
	if len(chapters) == 0 {
		return fallbackChapterTitles(doc)
	}
	return chapters
}

func parseChapterTitles(raw string) []string {
	clean := strings.TrimSpace(raw)
	start := strings.Index(clean, "{")
	end := strings.LastIndex(clean, "}")
	if start >= 0 && end > start {
		clean = clean[start : end+1]
	}

	var payload struct {
		Chapters []string `json:"chapters"`
	}
	if err := json.Unmarshal([]byte(clean), &payload); err != nil {
		return nil
	}

	seen := map[string]struct{}{}
	result := make([]string, 0, len(payload.Chapters))
	for _, title := range payload.Chapters {
		t := strings.TrimSpace(title)
		if t == "" {
			continue
		}
		key := strings.ToLower(t)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, t)
		if len(result) == 50 {
			break
		}
	}
	return result
}

func fallbackChapterTitles(doc *notebook.ExtractedDocument) []string {
	if doc == nil {
		return []string{"General"}
	}
	seen := map[string]struct{}{}
	result := make([]string, 0, 6)
	for _, section := range doc.Sections {
		title := strings.TrimSpace(section.Heading)
		if title == "" || strings.HasPrefix(strings.ToLower(title), "page ") {
			continue
		}
		key := strings.ToLower(title)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, title)
		if len(result) >= 6 {
			break
		}
	}
	if len(result) == 0 {
		return []string{"General"}
	}
	return result
}

type topicGroupBuilder struct {
	topicID string
	parents []db.NotebookParentInput
	chunks  []db.NotebookChunkInput
	order   int
}

func buildTopicGroups(notebookID string, doc *notebook.ExtractedDocument, topicIDs, topicTitles []string) ([]db.NotebookTopicIngestionGroup, []models.Chunk) {
	if doc == nil || len(doc.Sections) == 0 || len(topicIDs) == 0 {
		return nil, nil
	}

	builders := make([]*topicGroupBuilder, len(topicIDs))
	for i := range topicIDs {
		builders[i] = &topicGroupBuilder{topicID: topicIDs[i]}
	}

	allChunks := make([]models.Chunk, 0)
	for sectionIndex, section := range doc.Sections {
		sectionText := strings.TrimSpace(section.Text)
		if sectionText == "" {
			continue
		}

		topicIdx := pickTopicForSection(section, topicTitles)
		if topicIdx < 0 || topicIdx >= len(builders) {
			topicIdx = 0
		}

		builder := builders[topicIdx]
		builder.order++
		parentID := fmt.Sprintf("nbp_%s_%02d_%04d", notebookID, topicIdx+1, builder.order)
		heading := strings.TrimSpace(section.Heading)
		if heading == "" {
			heading = fmt.Sprintf("Section %d", sectionIndex+1)
		}

		builder.parents = append(builder.parents, db.NotebookParentInput{
			ID:         parentID,
			Heading:    heading,
			Content:    sectionText,
			OrderIndex: builder.order,
		})

		chunkTexts := notebook.SplitIntoWordChunks(sectionText, notebook.ChunkWordWindow, notebook.ChunkWordOverlap)
		for chunkIndex, chunkText := range chunkTexts {
			chunkID := fmt.Sprintf("nbc_%s_%02d_%04d_%03d", notebookID, topicIdx+1, builder.order, chunkIndex+1)
			builder.chunks = append(builder.chunks, db.NotebookChunkInput{
				ID:         chunkID,
				ParentID:   parentID,
				Text:       chunkText,
				TokenCount: len(strings.Fields(chunkText)),
				PageNum:    section.PageNum,
			})
			allChunks = append(allChunks, models.Chunk{
				ID:              chunkID,
				TopicID:         builder.topicID,
				ParentID:        parentID,
				Text:            chunkText,
				ImportanceScore: 0,
				WeaknessScore:   0,
			})
		}
	}

	groups := make([]db.NotebookTopicIngestionGroup, 0, len(builders))
	for _, builder := range builders {
		if len(builder.chunks) == 0 {
			continue
		}
		groups = append(groups, db.NotebookTopicIngestionGroup{
			TopicID: builder.topicID,
			Parents: builder.parents,
			Chunks:  builder.chunks,
		})
	}

	return groups, allChunks
}

func pickTopicForSection(section notebook.ExtractedSection, topicTitles []string) int {
	if len(topicTitles) == 0 {
		return 0
	}
	text := strings.ToLower(section.Heading + " " + firstN(section.Text, 800))
	textTokens := embeddings.TokenizeSimple(text)
	if len(textTokens) == 0 {
		return 0
	}

	bestIdx := 0
	bestScore := -1
	for i, title := range topicTitles {
		tokens := embeddings.TokenizeSimple(strings.ToLower(title))
		score := 0
		for token := range tokens {
			if _, ok := textTokens[token]; ok {
				score++
			}
		}
		if score > bestScore {
			bestScore = score
			bestIdx = i
		}
	}
	return bestIdx
}

func firstN(text string, n int) string {
	if n <= 0 || len(text) <= n {
		return text
	}
	return text[:n]
}

var nonWord = regexp.MustCompile(`[^a-z0-9]+`)

func slugify(s string) string {
	lower := strings.ToLower(strings.TrimSpace(s))
	lower = nonWord.ReplaceAllString(lower, "-")
	lower = strings.Trim(lower, "-")
	if lower == "" {
		return "topic"
	}
	parts := strings.Split(lower, "-")
	uniq := make([]string, 0, len(parts))
	for _, p := range parts {
		if p == "" {
			continue
		}
		uniq = append(uniq, p)
		if len(uniq) == 8 {
			break
		}
	}
	if len(uniq) == 0 {
		return "topic"
	}
	return strings.Join(uniq, "-")
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
	return utils.MD5Hex(text)
}

// DeleteNotebook removes a notebook and its associated file
func (a *App) DeleteNotebook(notebookID string) map[string]interface{} {
	if a.notebookService == nil {
		return map[string]interface{}{
			"error": "notebook service not initialized",
		}
	}

	// Get notebook to retrieve file path
	nb, err := a.notebookService.GetNotebookByID(notebookID)
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
	if err := a.notebookService.DeleteNotebookRecords(notebookID); err != nil {
		return map[string]interface{}{
			"error": err.Error(),
		}
	}

	return map[string]interface{}{
		"success": true,
	}
}
