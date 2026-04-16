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
const maxChapterTitles = 25

var frontMatterDenylist = []string{
	"foreword",
	"preface",
	"acknowledgments",
	"acknowledgements",
	"about the author",
	"copyright",
	"dedication",
	"contents",
	"table of contents",
	"index",
	"bibliography",
	"references",
}

var (
	numericChapterPattern    = regexp.MustCompile(`(?i)^(chapter\s+\d{1,3}\b(?:\s*[:.\-]\s*.*|\s+.*)?|\d{1,3}(?:\.\d{1,2}){0,2}\.?(?:\s*[:)\-]\s*.*|\s+.*)?)$`)
	romanChapterPattern      = regexp.MustCompile(`(?i)^(?:chapter\s+)?[ivxlcdm]{1,7}\.?(?:\s*[:)\-]\s*.*|\s+.*)$`)
	partChapterPattern       = regexp.MustCompile(`(?i)^part\s+(?:[ivxlcdm]+|\d{1,3})\b(?:\s*[:.\-]\s*.*)?$`)
	introOrPostscriptPattern = regexp.MustCompile(`(?i)^(introduction|postscript)\b(?:\s*[:.\-]\s*.*|\s+.*)?$`)
	chapterNumberPattern     = regexp.MustCompile(`(?i)\bchapter\s+(\d{1,3})\b`)
	chapterMarkerPattern     = regexp.MustCompile(`(?i)\bchapter\s+\d{1,3}\b`)
	numericTOCMarkerPattern  = regexp.MustCompile(`(?i)\b\d{1,3}\.\s+`)
	leadingBulletPattern     = regexp.MustCompile(`^[\s\-]+`)
	trailingDotsPagePattern  = regexp.MustCompile(`\s*[._·•-]{2,}\s*\d+\s*$`)
	onlyPunctOrDigits        = regexp.MustCompile(`^[\d\W_]+$`)
)

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

	deterministicCandidates := extractDeterministicChapterCandidates(doc)
	if len(deterministicCandidates) > 0 {
		if a.llmProvider == nil {
			return ensureChapterFallback(deterministicCandidates, doc)
		}

		prompt := buildConstrainedChapterPrompt(deterministicCandidates)
		response, err := a.llmProvider.GenerateAnswer(prompt)
		if err != nil {
			return ensureChapterFallback(deterministicCandidates, doc)
		}

		chapters := parseChapterTitles(response)
		if len(chapters) == 0 {
			return ensureChapterFallback(deterministicCandidates, doc)
		}
		if shouldPreferDeterministicCandidates(chapters, deterministicCandidates) {
			return ensureChapterFallback(deterministicCandidates, doc)
		}
		return ensureChapterFallback(chapters, doc)
	}

	input := make([]string, 0, len(doc.Sections))
	for _, section := range doc.Sections {
		if strings.TrimSpace(section.Text) == "" {
			continue
		}
		input = append(input, section.Text)
		if len(strings.Join(input, "\n")) > 10000 {
			break
		}
	}
	joined := strings.Join(input, "\n")
	if len(joined) > 10000 {
		joined = joined[:10000]
	}
	if strings.TrimSpace(joined) == "" {
		return []string{"General"}
	}

	if a.llmProvider == nil {
		return ensureChapterFallback(fallbackChapterTitles(doc), doc)
	}

	prompt := "Extract 5 to 10 major chapter titles from this study text sample. Return strict JSON as {\"chapters\":[\"Title 1\",\"Title 2\"]}. No extra text.\\n\\n" + joined
	response, err := a.llmProvider.GenerateAnswer(prompt)
	if err != nil {
		return ensureChapterFallback(fallbackChapterTitles(doc), doc)
	}

	chapters := parseChapterTitles(response)
	if len(chapters) == 0 {
		return ensureChapterFallback(fallbackChapterTitles(doc), doc)
	}
	return ensureChapterFallback(chapters, doc)
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

	return sanitizeChapterTitles(payload.Chapters)
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
	return sanitizeChapterTitles(result)
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
	lastMatchedTopicIdx := -1
	for sectionIndex, section := range doc.Sections {
		sectionText := strings.TrimSpace(section.Text)
		if sectionText == "" {
			continue
		}

		topicIdx, isConfidentMatch := pickTopicForSection(section, topicTitles, lastMatchedTopicIdx)
		if boundaryIdx, ok := detectChapterBoundaryTopic(section, topicTitles, lastMatchedTopicIdx); ok {
			topicIdx = boundaryIdx
			isConfidentMatch = true
		}
		if topicIdx < 0 || topicIdx >= len(builders) {
			topicIdx = 0
		}
		if isConfidentMatch {
			lastMatchedTopicIdx = topicIdx
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

func pickTopicForSection(section notebook.ExtractedSection, topicTitles []string, priorMatchedIdx int) (int, bool) {
	if len(topicTitles) == 0 {
		return 0, false
	}

	headingText := strings.ToLower(strings.TrimSpace(section.Heading))
	bodyText := strings.ToLower(firstN(section.Text, 1200))
	headingTokens := embeddings.TokenizeSimple(headingText)
	bodyTokens := embeddings.TokenizeSimple(bodyText)

	if len(headingTokens) == 0 && len(bodyTokens) == 0 {
		if priorMatchedIdx >= 0 && priorMatchedIdx < len(topicTitles) {
			return priorMatchedIdx, false
		}
		return 0, false
	}

	bestIdx := 0
	bestScore := 0
	for i, title := range topicTitles {
		titleText := strings.ToLower(strings.TrimSpace(title))
		tokens := embeddings.TokenizeSimple(titleText)
		score := overlapCount(tokens, headingTokens)*4 + overlapCount(tokens, bodyTokens)
		if titleText != "" && strings.Contains(headingText, titleText) {
			score += 3
		} else if titleText != "" && strings.Contains(bodyText, titleText) {
			score += 1
		}

		if score > bestScore {
			bestScore = score
			bestIdx = i
		}
	}

	if bestScore <= 0 {
		if priorMatchedIdx >= 0 && priorMatchedIdx < len(topicTitles) {
			return priorMatchedIdx, false
		}
		return 0, false
	}

	return bestIdx, true
}

func detectChapterBoundaryTopic(section notebook.ExtractedSection, topicTitles []string, priorMatchedIdx int) (int, bool) {
	if len(topicTitles) == 0 {
		return 0, false
	}

	headingText := strings.ToLower(strings.TrimSpace(section.Heading))
	bodyText := strings.ToLower(firstN(section.Text, 500))
	boundaryText := headingText + " " + bodyText

	// Strong boundary signal: explicit "Chapter N" markers in heading/page lead text.
	if matches := chapterNumberPattern.FindStringSubmatch(boundaryText); len(matches) == 2 {
		numText := strings.TrimSpace(matches[1])
		if numText != "" {
			n := 0
			for _, r := range numText {
				if r < '0' || r > '9' {
					n = 0
					break
				}
				n = n*10 + int(r-'0')
			}
			idx := n - 1
			if idx >= 0 && idx < len(topicTitles) {
				return idx, true
			}
		}
	}

	// Sequential progression hint: if content starts mentioning the next topic title, move forward.
	nextIdx := priorMatchedIdx + 1
	if nextIdx >= 0 && nextIdx < len(topicTitles) {
		nextTitle := strings.ToLower(strings.TrimSpace(topicTitles[nextIdx]))
		if nextTitle != "" && (strings.Contains(headingText, nextTitle) || strings.Contains(boundaryText, nextTitle)) {
			return nextIdx, true
		}
	}

	return 0, false
}

func firstN(text string, n int) string {
	if n <= 0 || len(text) <= n {
		return text
	}
	return text[:n]
}

func extractDeterministicChapterCandidates(doc *notebook.ExtractedDocument) []string {
	if doc == nil || len(doc.Sections) == 0 {
		return nil
	}

	seen := map[string]struct{}{}
	result := make([]string, 0, maxChapterTitles)

	sectionLimit := 24
	if len(doc.Sections) < sectionLimit {
		sectionLimit = len(doc.Sections)
	}

	for i := 0; i < sectionLimit; i++ {
		section := doc.Sections[i]
		candidateLines := make([]string, 0, 40)

		if strings.TrimSpace(section.Heading) != "" {
			candidateLines = append(candidateLines, section.Heading)
		}

		lines := strings.Split(firstN(section.Text, 4000), "\n")
		lineLimit := 40
		if len(lines) < lineLimit {
			lineLimit = len(lines)
		}
		candidateLines = append(candidateLines, lines[:lineLimit]...)

		for _, raw := range candidateLines {
			normalized := normalizeHeadingLine(raw)
			if normalized == "" || isFrontMatterTitle(normalized) || isNoisyHeading(normalized) {
				continue
			}
			if !looksLikeTopLevelChapter(normalized) {
				continue
			}

			key := strings.ToLower(normalized)
			if _, exists := seen[key]; exists {
				continue
			}
			seen[key] = struct{}{}
			result = append(result, normalized)
			if len(result) >= maxChapterTitles {
				return result
			}
		}

		// PDF page text is normalized and often loses line breaks, so detect inline Chapter markers too.
		inlineCandidates := extractInlineChapterCandidates(section.Text)
		for _, candidate := range inlineCandidates {
			normalized := normalizeHeadingLine(candidate)
			if normalized == "" || isFrontMatterTitle(normalized) || isNoisyHeading(normalized) {
				continue
			}

			key := strings.ToLower(normalized)
			if _, exists := seen[key]; exists {
				continue
			}
			seen[key] = struct{}{}
			result = append(result, normalized)
			if len(result) >= maxChapterTitles {
				return result
			}
		}

		// Some books encode TOC as flattened text: "1. ... 2. ..." with intro/postscript entries.
		numericTOC := extractInlineNumericTOCCandidates(section.Text)
		for _, candidate := range numericTOC {
			normalized := normalizeHeadingLine(candidate)
			if normalized == "" || isFrontMatterTitle(normalized) || isNoisyHeading(normalized) {
				continue
			}

			key := strings.ToLower(normalized)
			if _, exists := seen[key]; exists {
				continue
			}
			seen[key] = struct{}{}
			result = append(result, normalized)
			if len(result) >= maxChapterTitles {
				return result
			}
		}
	}

	return result
}

func buildConstrainedChapterPrompt(candidates []string) string {
	lines := make([]string, 0, len(candidates))
	for _, c := range candidates {
		lines = append(lines, "- "+c)
	}

	return strings.Join([]string{
		"You normalize study chapter candidates into top-level chapter titles.",
		"Return strict JSON object only: {\"chapters\":[\"Title 1\",\"Title 2\"]}.",
		"Rules:",
		"1) Keep only main study chapters.",
		"2) Remove front matter like preface, acknowledgments, references, index, and contents.",
		"3) Collapse sub-headings into parent chapter when obvious.",
		"4) Do not invent new chapters; output must come from the candidate list.",
		"5) Keep distinct numbered chapters separate, even with similar stem titles (e.g., Part I vs Part II).",
		"6) Maximum 25 chapters.",
		"",
		"Candidates:",
		strings.Join(lines, "\n"),
	}, "\n")
}

func ensureChapterFallback(primary []string, doc *notebook.ExtractedDocument) []string {
	sanitizedPrimary := sanitizeChapterTitles(primary)
	if len(sanitizedPrimary) > 0 {
		return sanitizedPrimary
	}

	sanitizedFallback := sanitizeChapterTitles(fallbackChapterTitles(doc))
	if len(sanitizedFallback) > 0 {
		return sanitizedFallback
	}

	return []string{"General"}
}

func sanitizeChapterTitles(chapters []string) []string {
	seen := map[string]struct{}{}
	result := make([]string, 0, len(chapters))

	for _, raw := range chapters {
		title := normalizeHeadingLine(raw)
		if title == "" || isFrontMatterTitle(title) || isNoisyHeading(title) {
			continue
		}

		key := strings.ToLower(title)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, title)
		if len(result) >= maxChapterTitles {
			break
		}
	}

	return result
}

func normalizeHeadingLine(input string) string {
	line := strings.TrimSpace(input)
	if line == "" {
		return ""
	}

	line = embeddings.NormalizeWhitespace(line)
	line = leadingBulletPattern.ReplaceAllString(line, "")
	line = trailingDotsPagePattern.ReplaceAllString(line, "")
	line = strings.Trim(line, " \t-_:;,.|/")
	return embeddings.NormalizeWhitespace(line)
}

func isFrontMatterTitle(title string) bool {
	if title == "" {
		return true
	}

	lower := strings.ToLower(embeddings.NormalizeWhitespace(title))
	for _, term := range frontMatterDenylist {
		if strings.Contains(lower, term) {
			return true
		}
	}
	return false
}

func looksLikeTopLevelChapter(line string) bool {
	if line == "" {
		return false
	}
	return numericChapterPattern.MatchString(line) || romanChapterPattern.MatchString(line) || partChapterPattern.MatchString(line) || introOrPostscriptPattern.MatchString(line)
}

func isNoisyHeading(title string) bool {
	if title == "" {
		return true
	}

	normalized := strings.ToLower(strings.TrimSpace(title))
	if strings.HasPrefix(normalized, "page ") {
		return true
	}

	if onlyPunctOrDigits.MatchString(normalized) {
		return true
	}

	wordCount := len(strings.Fields(normalized))
	if wordCount == 0 {
		return true
	}

	if len([]rune(normalized)) <= 2 && wordCount == 1 {
		return true
	}

	return false
}

func overlapCount(a, b map[string]struct{}) int {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}

	score := 0
	for token := range a {
		if _, ok := b[token]; ok {
			score++
		}
	}
	return score
}

func extractInlineChapterCandidates(text string) []string {
	raw := embeddings.NormalizeWhitespace(text)
	if raw == "" {
		return nil
	}

	matches := chapterMarkerPattern.FindAllStringIndex(raw, -1)
	if len(matches) == 0 {
		return nil
	}

	result := make([]string, 0, len(matches))
	for i, match := range matches {
		start := match[0]
		end := len(raw)
		if i+1 < len(matches) {
			end = matches[i+1][0]
		}
		segment := strings.TrimSpace(raw[start:end])
		if segment == "" {
			continue
		}

		segment = firstN(segment, 180)
		result = append(result, segment)
	}

	return result
}

func extractInlineNumericTOCCandidates(text string) []string {
	raw := embeddings.NormalizeWhitespace(text)
	if raw == "" {
		return nil
	}

	result := make([]string, 0, 16)
	lower := strings.ToLower(raw)

	numericMatches := numericTOCMarkerPattern.FindAllStringIndex(raw, -1)
	firstNumeric := len(raw)
	if len(numericMatches) > 0 {
		firstNumeric = numericMatches[0][0]
	}

	introIdx := strings.Index(lower, "introduction")
	if introIdx >= 0 {
		introStart := introIdx
		introEnd := firstNumeric
		if introEnd <= introStart || introEnd > len(raw) {
			introEnd = len(raw)
		}
		intro := strings.TrimSpace(raw[introStart:introEnd])
		if intro != "" {
			result = append(result, intro)
		}
	}

	postscriptIdx := strings.Index(lower, "postscript")

	for i, match := range numericMatches {
		start := match[0]
		end := len(raw)
		if i+1 < len(numericMatches) {
			end = numericMatches[i+1][0]
		} else if postscriptIdx > start {
			end = postscriptIdx
		}

		segment := strings.TrimSpace(raw[start:end])
		if segment == "" {
			continue
		}

		dotIdx := strings.Index(segment, ".")
		if dotIdx <= 0 {
			continue
		}
		num := strings.TrimSpace(segment[:dotIdx])
		title := strings.TrimSpace(segment[dotIdx+1:])
		if num == "" || title == "" {
			continue
		}

		result = append(result, fmt.Sprintf("%s. %s", num, firstN(title, 120)))
	}

	if postscriptIdx >= 0 {
		postscript := strings.TrimSpace(raw[postscriptIdx:])
		if postscript != "" {
			result = append(result, postscript)
		}
	}

	return result
}

func shouldPreferDeterministicCandidates(llmChapters, deterministic []string) bool {
	if len(deterministic) == 0 || len(llmChapters) == 0 {
		return false
	}

	// If LLM returns a reduced list, trust deterministic candidates.
	// Keep at least 80% of deterministic chapter candidates to avoid dropping real chapters.
	minExpected := (len(deterministic)*8 + 9) / 10
	if minExpected < 3 {
		minExpected = 3
	}
	return len(llmChapters) < minExpected
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
