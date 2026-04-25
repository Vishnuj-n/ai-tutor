package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"ai-tutor/internal/db"
	"ai-tutor/internal/models"
	"ai-tutor/internal/notebook"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

const ingestionEventName = "ingestion-progress"
const topicExtractionMaxChars = 30000

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
func (a *App) DraftNotebookSyllabus(notebookID string) map[string]interface{} {
	notebookID = strings.TrimSpace(notebookID)
	if notebookID == "" {
		return map[string]interface{}{"error": "notebook id is required"}
	}
	if a.notebookService == nil {
		return map[string]interface{}{"error": "notebook service not initialized"}
	}

	nb, err := a.notebookService.GetNotebookByID(notebookID)
	if err != nil {
		return map[string]interface{}{"error": err.Error()}
	}
	if nb == nil {
		return map[string]interface{}{"error": "notebook not found"}
	}

	doc, err := a.notebookService.ExtractDocument(nb.FilePath, nb.FileType)
	if err != nil {
		return map[string]interface{}{"error": err.Error()}
	}

	_ = db.UpdateNotebookStatus(notebookID, "analyzing")
	chapters, fallbackUsed := a.draftSyllabusChapters(nb.FileType, nb.FilePath, doc)
	if len(chapters) == 0 {
		chapters = []models.SyllabusChapterDraft{{
			Title:     "General",
			StartPage: 1,
			EndPage:   maxPage(doc.PageCount),
		}}
		fallbackUsed = true
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

	nb, err := a.notebookService.GetNotebookByID(notebookID)
	if err != nil {
		return map[string]interface{}{"error": err.Error()}
	}
	if nb == nil {
		return map[string]interface{}{"error": "notebook not found"}
	}

	doc, err := a.notebookService.ExtractDocument(nb.FilePath, nb.FileType)
	if err != nil {
		return map[string]interface{}{"error": err.Error()}
	}

	normalized := normalizeSyllabusChapters(chapters, doc.PageCount)
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

	groups, allChunks := buildTopicGroupsFromChapters(notebookID, doc, topicIDs, normalized)
	if len(groups) == 0 || len(allChunks) == 0 {
		_ = db.UpdateNotebookStatus(notebookID, "failed")
		// Cleanup: delete created topic rows to avoid orphaned records
		for _, item := range topicItems {
			_ = db.DeleteTopic(item.TopicID)
		}
		return map[string]interface{}{"error": "confirmed chapters produced no chunks"}
	}

	emitIngestionProgress(a, ingestionProgressPayload{
		NotebookID: notebookID,
		Status:     "chunking",
		Message:    fmt.Sprintf("Creating %d chunks for confirmed chapters", len(allChunks)),
		Phase:      "chunking",
		Processed:  0,
		Total:      len(allChunks),
		Percent:    20,
	})

	if err := a.notebookService.IngestNotebookContentByTopic(notebookID, groups); err != nil {
		_ = db.UpdateNotebookStatus(notebookID, "failed")
		// Cleanup: delete created topic rows to avoid orphaned records
		for _, item := range topicItems {
			_ = db.DeleteTopic(item.TopicID)
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
	a.requestPrebuildRefresh()
	return map[string]interface{}{
		"success":     true,
		"status":      status,
		"notebook_id": notebookID,
		"topic_ids":   topicIDs,
		"chunk_count": len(allChunks),
	}
}

func (a *App) draftSyllabusChapters(fileType, filePath string, doc *notebook.ExtractedDocument) ([]models.SyllabusChapterDraft, bool) {
	if doc == nil || len(doc.Sections) == 0 {
		return nil, false
	}

	bookmarkLikeDraft := []models.SyllabusChapterDraft{}
	fallbackUsed := false
	if strings.EqualFold(strings.TrimSpace(fileType), "pdf") && strings.TrimSpace(filePath) != "" {
		bookmarkLikeDraft = extractPDFCPUBookmarkDraft(filePath, doc.PageCount)
	}
	sample := buildPageSample(doc, 30)

	if a.heavyLLMProvider != nil {
		bookmarkJSON, _ := json.Marshal(bookmarkLikeDraft)
		prompt := fmt.Sprintf("Create syllabus chapter ranges from this document sample. Return strict JSON only as {\"chapters\":[{\"title\":\"...\",\"start_page\":1,\"end_page\":10}]}. Keep absolute page numbers, preserve order, avoid overlaps, and cover as much content as possible.\\n\\nFile type: %s\\nPage count: %d\\nBookmark candidates (may be empty): %s\\n\\nText sample with absolute page markers:\\n%s", strings.ToLower(fileType), doc.PageCount, string(bookmarkJSON), sample)
		raw, err := a.heavyLLMProvider.GenerateAnswer(prompt)
		if err == nil {
			parsed := parseSyllabusDraft(raw, doc.PageCount)
			if len(parsed) > 0 {
				return parsed, false
			}
		}
	}

	if len(bookmarkLikeDraft) > 0 {
		return normalizeSyllabusChapters(bookmarkLikeDraft, doc.PageCount), fallbackUsed
	}

	// No LLM response and no bookmarks - indicate fallback needed
	return nil, true
}

func parseSyllabusDraft(raw string, pageCount int) []models.SyllabusChapterDraft {
	clean := strings.TrimSpace(raw)
	start := strings.Index(clean, "{")
	end := strings.LastIndex(clean, "}")
	if start >= 0 && end > start {
		clean = clean[start : end+1]
	}

	var payload struct {
		Chapters []models.SyllabusChapterDraft `json:"chapters"`
	}
	if err := json.Unmarshal([]byte(clean), &payload); err != nil {
		return nil
	}

	return normalizeSyllabusChapters(payload.Chapters, pageCount)
}

func normalizeSyllabusChapters(chapters []models.SyllabusChapterDraft, pageCount int) []models.SyllabusChapterDraft {
	if len(chapters) == 0 {
		return nil
	}
	max := maxPage(pageCount)
	normalized := make([]models.SyllabusChapterDraft, 0, len(chapters))
	for _, ch := range chapters {
		title := strings.TrimSpace(ch.Title)
		if title == "" {
			continue
		}
		start := ch.StartPage
		end := ch.EndPage
		if start <= 0 {
			start = 1
		}
		if start > max {
			start = max
		}
		if end < start {
			end = start
		}
		if end > max {
			end = max
		}
		normalized = append(normalized, models.SyllabusChapterDraft{Title: title, StartPage: start, EndPage: end})
	}

	if len(normalized) == 0 {
		return nil
	}

	sort.SliceStable(normalized, func(i, j int) bool {
		if normalized[i].StartPage == normalized[j].StartPage {
			return normalized[i].EndPage < normalized[j].EndPage
		}
		return normalized[i].StartPage < normalized[j].StartPage
	})

	resolved := make([]models.SyllabusChapterDraft, 0, len(normalized))
	nextPage := 1
	for i, ch := range normalized {
		start := ch.StartPage
		if start > nextPage && len(resolved) > 0 {
			// Assign gap pages to the previous chapter so no pages are dropped during ingestion.
			resolved[len(resolved)-1].EndPage = start - 1
			nextPage = start
		}
		if start < nextPage {
			start = nextPage
		}
		if start > max {
			break
		}
		end := ch.EndPage
		if i < len(normalized)-1 {
			nextStart := normalized[i+1].StartPage
			if nextStart > start && end <= start {
				end = nextStart - 1
			}
		}
		if end < start {
			end = start
		}
		if end > max {
			end = max
		}
		resolved = append(resolved, models.SyllabusChapterDraft{Title: ch.Title, StartPage: start, EndPage: end})
		nextPage = end + 1
	}

	if len(resolved) == 0 {
		return nil
	}
	resolved[len(resolved)-1].EndPage = max
	return resolved
}

func extractPDFCPUBookmarkDraft(filePath string, pageCount int) []models.SyllabusChapterDraft {
	jsonOutput, err := runPDFCPUBookmarksExport(filePath)
	if err != nil || strings.TrimSpace(string(jsonOutput)) == "" {
		return nil
	}

	return parsePDFCPUBookmarkDraftFromJSON(jsonOutput, pageCount)
}

func runPDFCPUBookmarksExport(filePath string) ([]byte, error) {
	absFilePath, err := validatePDFCPUInputFilePath(filePath)
	if err != nil {
		return nil, err
	}

	pdfcpuPath, err := findPDFCPUExecutable()
	if err != nil {
		return nil, err
	}

	tmpFile, err := os.CreateTemp("", "pdfcpu-bookmarks-*.json")
	if err != nil {
		return nil, err
	}
	tmpPath := tmpFile.Name()
	_ = tmpFile.Close()
	defer func() {
		_ = os.Remove(tmpPath)
	}()

	cmd := exec.Command(pdfcpuPath, "bookmarks", "export", absFilePath, tmpPath)
	if _, runErr := cmd.Output(); runErr != nil {
		return nil, runErr
	}

	content, readErr := os.ReadFile(tmpPath)
	if readErr != nil {
		return nil, readErr
	}
	return content, nil
}

func validatePDFCPUInputFilePath(filePath string) (string, error) {
	trimmed := strings.TrimSpace(filePath)
	if trimmed == "" {
		return "", fmt.Errorf("file path is required")
	}
	if strings.Contains(trimmed, "\x00") {
		return "", fmt.Errorf("invalid file path")
	}
	if strings.Contains(trimmed, "..\\") || strings.Contains(trimmed, "../") {
		return "", fmt.Errorf("file path traversal is not allowed")
	}

	cleaned := filepath.Clean(trimmed)
	absPath, err := filepath.Abs(cleaned)
	if err != nil {
		return "", fmt.Errorf("invalid file path: %w", err)
	}
	uploadDir, err := resolveNotebookDir()
	if err != nil {
		return "", fmt.Errorf("failed to resolve upload directory: %w", err)
	}
	uploadRoot, err := filepath.Abs(uploadDir)
	if err != nil {
		return "", fmt.Errorf("failed to resolve upload directory: %w", err)
	}
	relToUploadRoot, err := filepath.Rel(uploadRoot, absPath)
	if err != nil {
		return "", fmt.Errorf("invalid file path relation: %w", err)
	}
	if relToUploadRoot == ".." || strings.HasPrefix(relToUploadRoot, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("file path is outside allowed upload directory")
	}

	info, err := os.Stat(absPath)
	if err != nil {
		return "", fmt.Errorf("failed to stat file path: %w", err)
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("file path must point to a regular file")
	}
	return absPath, nil
}

func findPDFCPUExecutable() (string, error) {
	pdfcpuPath, err := exec.LookPath("pdfcpu")
	if err == nil {
		return pdfcpuPath, nil
	}

	binary := "pdfcpu"
	if runtime.GOOS == "windows" {
		binary = "pdfcpu.exe"
	}

	candidateDirs := make([]string, 0, 8)
	if gobin := strings.TrimSpace(os.Getenv("GOBIN")); gobin != "" {
		candidateDirs = append(candidateDirs, gobin)
	}
	if gopath := strings.TrimSpace(os.Getenv("GOPATH")); gopath != "" {
		candidateDirs = append(candidateDirs, filepath.Join(gopath, "bin"))
	} else if home, homeErr := os.UserHomeDir(); homeErr == nil && home != "" {
		candidateDirs = append(candidateDirs, filepath.Join(home, "go", "bin"))
	}

	switch runtime.GOOS {
	case "windows":
		candidateDirs = append(candidateDirs, `C:\Program Files\pdfcpu`, `C:\Program Files (x86)\pdfcpu`)
	case "darwin":
		candidateDirs = append(candidateDirs, "/usr/local/bin", "/opt/homebrew/bin")
	default:
		candidateDirs = append(candidateDirs, "/usr/local/bin", "/usr/bin")
	}

	for _, dir := range candidateDirs {
		if strings.TrimSpace(dir) == "" {
			continue
		}
		candidate := filepath.Join(dir, binary)
		info, statErr := os.Stat(candidate)
		if statErr == nil && !info.IsDir() {
			return candidate, nil
		}
	}

	return "", fmt.Errorf("pdfcpu binary not found; install pdfcpu and ensure it is available on PATH, GOBIN, or GOPATH/bin")
}

func parsePDFCPUBookmarkDraftFromJSON(raw []byte, pageCount int) []models.SyllabusChapterDraft {
	var payload interface{}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil
	}

	type bookmarkNode struct {
		title string
		page  int
	}

	collected := make([]bookmarkNode, 0)
	var walk func(node interface{})
	walk = func(node interface{}) {
		switch typed := node.(type) {
		case map[string]interface{}:
			title := strings.TrimSpace(firstString(typed, "title", "Title", "name", "Name"))
			page := firstInt(typed, "page", "Page", "pageNr", "PageNr", "p", "PageFrom", "from")
			if title != "" && page > 0 {
				collected = append(collected, bookmarkNode{title: title, page: page})
			}
			for _, key := range []string{"children", "Children", "bookmarks", "Bookmarks", "items", "Items", "nodes", "Nodes", "sub", "Sub"} {
				if child, ok := typed[key]; ok {
					walk(child)
				}
			}
		case []interface{}:
			for _, child := range typed {
				walk(child)
			}
		}
	}

	walk(payload)
	if len(collected) == 0 {
		return nil
	}

	draft := make([]models.SyllabusChapterDraft, 0, len(collected))
	for _, item := range collected {
		draft = append(draft, models.SyllabusChapterDraft{Title: item.title, StartPage: item.page, EndPage: item.page})
	}

	return normalizeSyllabusChapters(draft, pageCount)
}

func firstString(node map[string]interface{}, keys ...string) string {
	for _, key := range keys {
		if value, ok := node[key]; ok {
			switch typed := value.(type) {
			case string:
				if strings.TrimSpace(typed) != "" {
					return typed
				}
			}
		}
	}
	return ""
}

func firstInt(node map[string]interface{}, keys ...string) int {
	for _, key := range keys {
		if value, ok := node[key]; ok {
			switch typed := value.(type) {
			case float64:
				return int(typed)
			case int:
				return typed
			case string:
				var parsed int
				if _, err := fmt.Sscanf(strings.TrimSpace(typed), "%d", &parsed); err == nil {
					return parsed
				}
			}
		}
	}
	return 0
}

func buildPageSample(doc *notebook.ExtractedDocument, maxSections int) string {
	if doc == nil || len(doc.Sections) == 0 || maxSections <= 0 {
		return ""
	}
	parts := make([]string, 0, maxSections)
	for i, section := range doc.Sections {
		if i >= maxSections {
			break
		}
		text := strings.TrimSpace(section.Text)
		if text == "" {
			continue
		}
		parts = append(parts, fmt.Sprintf("[Page %d] %s", section.PageNum, firstN(text, 2000)))
	}
	joined := strings.Join(parts, "\n\n")
	if len(joined) > topicExtractionMaxChars {
		return joined[:topicExtractionMaxChars]
	}
	return joined
}

func maxPage(pageCount int) int {
	if pageCount <= 0 {
		return 1
	}
	return pageCount
}

func buildTopicGroupsFromChapters(notebookID string, doc *notebook.ExtractedDocument, topicIDs []string, chapters []models.SyllabusChapterDraft) ([]db.NotebookTopicIngestionGroup, []models.Chunk) {
	if doc == nil || len(doc.Sections) == 0 || len(topicIDs) == 0 || len(chapters) == 0 || len(topicIDs) != len(chapters) {
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
		page := section.PageNum
		if page <= 0 {
			page = 1
		}

		topicIdx := chapterIndexForPage(page, chapters)
		if topicIdx < 0 {
			continue
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

		chunkTexts := notebook.SplitPageIntoSemanticChunks(sectionText, notebook.DefaultSemanticChunkTargetWords)
		for chunkIndex, chunkText := range chunkTexts {
			chunkID := fmt.Sprintf("nbc_%s_%02d_%04d_%03d", notebookID, topicIdx+1, builder.order, chunkIndex+1)
			builder.chunks = append(builder.chunks, db.NotebookChunkInput{
				ID:         chunkID,
				ParentID:   parentID,
				Text:       chunkText,
				TokenCount: len(strings.Fields(chunkText)),
				PageNum:    page,
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

func chapterIndexForPage(page int, chapters []models.SyllabusChapterDraft) int {
	for i, ch := range chapters {
		if page >= ch.StartPage && page <= ch.EndPage {
			return i
		}
	}
	if len(chapters) == 0 {
		return -1
	}
	if page < chapters[0].StartPage {
		return 0
	}
	last := chapters[len(chapters)-1]
	if page > last.EndPage {
		return len(chapters) - 1
	}
	return -1
}

type topicGroupBuilder struct {
	topicID string
	parents []db.NotebookParentInput
	chunks  []db.NotebookChunkInput
	order   int
}

func firstN(text string, n int) string {
	if n <= 0 || len(text) <= n {
		return text
	}
	return text[:n]
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
