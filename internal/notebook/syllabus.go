package notebook

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"ai-tutor/internal/models"
)

// LLMProvider interface for LLM operations.
type LLMProvider interface {
	GenerateAnswer(prompt string) (string, error)
}

const topicExtractionMaxChars = 30000

// SyllabusDraftResult contains the result of syllabus drafting.
type SyllabusDraftResult struct {
	Chapters     []models.SyllabusChapterDraft
	PageCount    int
	FallbackUsed bool
}

// DraftSyllabusChapters creates editable chapter ranges for HITL verification.
func (s *Service) DraftSyllabusChapters(fileType, filePath string, doc *ExtractedDocument, llmProvider LLMProvider) (*SyllabusDraftResult, error) {
	if doc == nil || len(doc.Sections) == 0 {
		return &SyllabusDraftResult{Chapters: nil, PageCount: 0, FallbackUsed: false}, nil
	}

	bookmarkLikeDraft := []models.SyllabusChapterDraft{}
	if strings.EqualFold(strings.TrimSpace(fileType), "pdf") && strings.TrimSpace(filePath) != "" {
		bookmarkLikeDraft = extractPDFCPUBookmarkDraft(filePath, doc.PageCount, s.config.UploadDir)
	}
	sample := buildPageSample(doc, 30)

	if llmProvider != nil {
		bookmarkJSON, _ := json.Marshal(bookmarkLikeDraft)
		prompt := fmt.Sprintf("Create syllabus chapter ranges from this document sample. Return strict JSON only as {\"chapters\":[{\"title\":\"...\",\"start_page\":1,\"end_page\":10}]}. Keep absolute page numbers, preserve order, avoid overlaps, and cover as much content as possible.\n\nFile type: %s\nPage count: %d\nBookmark candidates (may be empty): %s\n\nText sample with absolute page markers:\n%s", strings.ToLower(fileType), doc.PageCount, string(bookmarkJSON), sample)
		raw, err := llmProvider.GenerateAnswer(prompt)
		if err == nil {
			parsed := parseSyllabusDraft(raw, doc.PageCount)
			if len(parsed) > 0 {
				return &SyllabusDraftResult{Chapters: parsed, PageCount: doc.PageCount, FallbackUsed: false}, nil
			}
		}
	}

	if len(bookmarkLikeDraft) > 0 {
		return &SyllabusDraftResult{
			Chapters:     NormalizeSyllabusChapters(bookmarkLikeDraft, doc.PageCount),
			PageCount:    doc.PageCount,
			FallbackUsed: true, // Bookmark-based chapters are a fallback
		}, nil
	}

	// No LLM response and no bookmarks - indicate fallback needed
	return &SyllabusDraftResult{Chapters: nil, PageCount: doc.PageCount, FallbackUsed: true}, nil
}

// parseSyllabusDraft parses LLM JSON response into chapter drafts.
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

	return NormalizeSyllabusChapters(payload.Chapters, pageCount)
}

// NormalizeSyllabusChapters normalizes and validates chapter page ranges.
func NormalizeSyllabusChapters(chapters []models.SyllabusChapterDraft, pageCount int) []models.SyllabusChapterDraft {
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

// buildPageSample builds a text sample from document sections for LLM prompting.
func buildPageSample(doc *ExtractedDocument, maxSections int) string {
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
		// Use rune-aware truncation to avoid splitting multi-byte UTF-8 characters
		runes := []rune(joined)
		if len(runes) > topicExtractionMaxChars {
			return string(runes[:topicExtractionMaxChars])
		}
	}
	return joined
}

// maxPage returns the valid maximum page count.
func maxPage(pageCount int) int {
	if pageCount <= 0 {
		return 1
	}
	return pageCount
}

// firstN returns the first N characters of a string.
func firstN(text string, n int) string {
	if n <= 0 || len(text) <= n {
		return text
	}
	return text[:n]
}
