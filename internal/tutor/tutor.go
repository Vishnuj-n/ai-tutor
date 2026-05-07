package tutor

import (
	"strings"

	"ai-tutor/internal/notebook"
	"ai-tutor/internal/parser"
)

func ExtractChapterTitles(llmProvider parser.LLMProvider, doc *notebook.ExtractedDocument) []string {
	if doc == nil || len(doc.Sections) == 0 {
		return []string{"General"}
	}

	deterministicCandidates := parser.ExtractDeterministicChapterCandidates(doc)
	if len(deterministicCandidates) > 0 {
		if llmProvider == nil {
			return parser.EnsureChapterFallback(deterministicCandidates, doc)
		}

		prompt := buildConstrainedChapterPrompt(deterministicCandidates)
		response, err := llmProvider.GenerateAnswer(prompt)
		if err != nil {
			return parser.EnsureChapterFallback(deterministicCandidates, doc)
		}

		chapters := parser.ParseChapterTitles(response)
		if len(chapters) == 0 {
			return parser.EnsureChapterFallback(deterministicCandidates, doc)
		}
		if parser.ShouldPreferDeterministicCandidates(chapters, deterministicCandidates) {
			return parser.EnsureChapterFallback(deterministicCandidates, doc)
		}
		return parser.EnsureChapterFallback(chapters, doc)
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

	if llmProvider == nil {
		return parser.EnsureChapterFallback(parser.FallbackChapterTitles(doc), doc)
	}

	prompt := "Extract 5 to 10 major chapter titles from this study text sample. Return strict JSON as {\"chapters\":[\"Title 1\",\"Title 2\"]}. No extra text.\\n\\n" + joined
	response, err := llmProvider.GenerateAnswer(prompt)
	if err != nil {
		return parser.EnsureChapterFallback(parser.FallbackChapterTitles(doc), doc)
	}

	chapters := parser.ParseChapterTitles(response)
	if len(chapters) == 0 {
		return parser.EnsureChapterFallback(parser.FallbackChapterTitles(doc), doc)
	}
	return parser.EnsureChapterFallback(chapters, doc)
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
