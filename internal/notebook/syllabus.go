package notebook

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"ai-tutor/internal/embeddings"
	"ai-tutor/internal/llm"
	"ai-tutor/internal/models"
)

// LLMProvider interface for LLM operations.
type LLMProvider interface {
	GenerateAnswer(prompt string) (string, error)
	GetLimits() llm.ModelLimits
}

const topicExtractionMaxChars = 30000

// SyllabusDraftResult contains the result of syllabus drafting.
type SyllabusDraftResult struct {
	Chapters     []models.SyllabusChapterDraft
	PageCount    int
	FallbackUsed bool
}


// DraftSyllabusChapters creates editable chapter ranges for HITL verification.
// Uses LLM with bookmark context when llmProvider is non-nil.
func (s *Service) DraftSyllabusChapters(fileType, filePath string, doc *ExtractedDocument, llmProvider LLMProvider) (*SyllabusDraftResult, error) {
	if doc == nil || len(doc.Sections) == 0 {
		return &SyllabusDraftResult{Chapters: nil, PageCount: 0, FallbackUsed: false}, nil
	}

	bookmarkLikeDraft := []models.SyllabusChapterDraft{}
	var rawBookmarkJSON []byte // full nested tree from pdfcpu, used for LLM context
	if strings.EqualFold(strings.TrimSpace(fileType), "pdf") && strings.TrimSpace(filePath) != "" {
		bookmarkLikeDraft = extractPDFCPUBookmarkDraft(filePath, doc.PageCount, s.config.UploadDir)
		if raw, err := runPDFCPUBookmarksExport(filePath, s.config.UploadDir); err == nil {
			rawBookmarkJSON = raw
		}
	}
	sample := buildPageSample(doc, 30)

	if llmProvider != nil {
		bookName := strings.TrimSuffix(filepath.Base(strings.TrimSpace(filePath)), filepath.Ext(filePath))
		if bookName == "" {
			bookName = "(unknown)"
		}

		var prompt string
		if len(rawBookmarkJSON) > 0 || len(bookmarkLikeDraft) > 0 {
			var bookmarkContext string
			if len(rawBookmarkJSON) > 0 {
				fullNodes := ExtractFullPDFCPUBookmarkNodes(rawBookmarkJSON)
				if len(fullNodes) > 0 {
					if b, err := json.Marshal(fullNodes); err == nil {
						bookmarkContext = string(b)
					}
				} else {
					bookmarkContext = string(rawBookmarkJSON)
				}
			} else if len(bookmarkLikeDraft) > 0 {
				if b, err := json.Marshal(bookmarkLikeDraft); err == nil {
					bookmarkContext = string(b)
				}
			}
			if bookmarkContext == "" {
				bookmarkContext = "(none)"
			}

			prompt = fmt.Sprintf(`You are extracting main study chapters for a book syllabus.

Document: %s
Total pages: %d

Normalized Bookmark Tree:
%s

Task: Identify only the main chapters (excluding sub-chapters, sections, sub-topics, or minor headings).
Rules:
- Output strict JSON only: {"chapters":[{"title":"...","start_page":1,"end_page":10}]}
- Select only top-level main chapters. Exclude sub-chapters, sections, and sub-topics.
- Clean up chapter titles (e.g. remove "Chapter 1." prefix or noise).
- Preserve accurate page numbers from the provided bookmarks. No gaps. No overlaps.`,
				bookName, doc.PageCount, bookmarkContext)
		} else {
			limits := llmProvider.GetLimits()
			maxInputTokens := limits.MaxInputTokens
			const baseOverheadTokens = 500
			const safetyMarginTokens = 500
			availableBudget := maxInputTokens - baseOverheadTokens - safetyMarginTokens
			if availableBudget < 1000 {
				availableBudget = 1000
			}

			sampleChars := len(sample)
			maxChars := availableBudget * 4
			if sampleChars > maxChars && maxChars > 0 {
				sample = truncateToCharBoundary(sample, maxChars)
			}

			prompt = fmt.Sprintf(`You are extracting a study syllabus from a document.

Document: %s
File type: %s
Total pages: %d

Text sample with absolute page markers (first 30 sections):
%s

Task: Return a flat list of study-ready chapters with accurate page ranges.
Rules:
- Output strict JSON only: {"chapters":[{"title":"...","start_page":1,"end_page":10}]}
- Use absolute page numbers. Preserve order. No gaps. No overlaps.
- Prefer main chapters.`,
				bookName, strings.ToLower(fileType), doc.PageCount, sample)
		}

		// Token budgeting check
		limits := llmProvider.GetLimits()
		maxInputTokens := limits.MaxInputTokens
		promptTokens, err := embeddings.CountTokens(prompt)
		if err == nil && promptTokens > maxInputTokens {
			targetTokens := maxInputTokens - 200
			if targetTokens < 1000 {
				targetTokens = 1000
			}
			if truncated, err := embeddings.TruncateToTokens(prompt, targetTokens); err == nil && len(truncated) > 0 {
				prompt = truncated
			}
		}

		raw, err := llmProvider.GenerateAnswer(prompt)
		if err != nil {
			return nil, fmt.Errorf("AI generation failed: %w", err)
		}
		parsed := parseSyllabusDraft(raw, doc.PageCount)
		if len(parsed) == 0 {
			return nil, fmt.Errorf("AI returned an invalid or empty chapter draft response")
		}
		return &SyllabusDraftResult{Chapters: parsed, PageCount: doc.PageCount, FallbackUsed: false}, nil
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

// truncateToCharBoundary truncates text to maxChars, preferring a newline boundary.
func truncateToCharBoundary(text string, maxChars int) string {
	if len(text) <= maxChars {
		return text
	}
	truncated := text[:maxChars]
	// Prefer breaking at a newline for cleaner context.
	if idx := strings.LastIndex(truncated, "\n"); idx > maxChars/2 {
		return truncated[:idx]
	}
	return truncated
}
