package main

import (
	"ai-tutor/internal/parser"
	"ai-tutor/internal/tutor"
	"testing"

	"ai-tutor/internal/notebook"
)

type chapterTestLLM struct {
	answer string
	err    error
}

func (m *chapterTestLLM) GenerateAnswer(prompt string) (string, error) {
	if m.err != nil {
		return "", m.err
	}
	return m.answer, nil
}

func TestExtractDeterministicChapterCandidatesKeepsNumberedChapters(t *testing.T) {
	doc := &notebook.ExtractedDocument{
		Sections: []notebook.ExtractedSection{
			{Text: "Chapter 1: Basics\n1.1 Overview\nChapter 2: Advanced Concepts"},
		},
	}

	got := parser.ExtractDeterministicChapterCandidates(doc)
	if len(got) < 2 {
		t.Fatalf("expected at least two chapter candidates, got %#v", got)
	}
	if got[0] != "Chapter 1: Basics" {
		t.Fatalf("expected first chapter to be kept, got %#v", got)
	}
	if got[1] != "1.1 Overview" && got[1] != "Chapter 2: Advanced Concepts" {
		t.Fatalf("expected numbered chapter candidate, got %#v", got)
	}
}

func TestExtractDeterministicChapterCandidatesRemovesFrontMatter(t *testing.T) {
	doc := &notebook.ExtractedDocument{
		Sections: []notebook.ExtractedSection{
			{Text: "Preface\nAcknowledgments\nTable of Contents\nChapter 1 Introduction"},
		},
	}

	got := parser.ExtractDeterministicChapterCandidates(doc)
	if len(got) == 0 {
		t.Fatalf("expected at least one candidate, got %#v", got)
	}
	for _, title := range got {
		if parser.IsFrontMatterTitle(title) {
			t.Fatalf("front matter should be removed, got %#v", got)
		}
	}
}

func TestExtractDeterministicChapterCandidatesDedupesAndCaps(t *testing.T) {
	lines := "Chapter 1 Intro\nchapter 1 intro\nChapter 2\nChapter 3\nChapter 4\nChapter 5\nChapter 6\nChapter 7\nChapter 8\nChapter 9\nChapter 10\nChapter 11\nChapter 12\nChapter 13\nChapter 14\nChapter 15\nChapter 16\nChapter 17\nChapter 18\nChapter 19\nChapter 20\nChapter 21\nChapter 22\nChapter 23\nChapter 24\nChapter 25\nChapter 26\nChapter 27\nChapter 28"
	doc := &notebook.ExtractedDocument{
		Sections: []notebook.ExtractedSection{
			{Text: lines},
		},
	}

	got := parser.ExtractDeterministicChapterCandidates(doc)
	if len(got) != 25 {
		t.Fatalf("expected candidate cap of %d, got %d (%#v)", 25, len(got), got)
	}
	if got[0] != "Chapter 1 Intro" {
		t.Fatalf("expected normalized deduped first chapter, got %#v", got)
	}
}

func TestExtractChapterTitlesMalformedLLMOutputFallsBackToDeterministic(t *testing.T) {
	app := &App{
		llmProvider: &chapterTestLLM{
			answer: "not-json-at-all",
		},
	}
	doc := &notebook.ExtractedDocument{
		Sections: []notebook.ExtractedSection{
			{Text: "Chapter 1: Intro\nChapter 2: Methods"},
		},
	}

	got := tutor.ExtractChapterTitles(app.llmProvider, doc)
	if len(got) < 2 {
		t.Fatalf("expected deterministic fallback chapters, got %#v", got)
	}
	if got[0] != "Chapter 1: Intro" {
		t.Fatalf("expected fallback chapter ordering, got %#v", got)
	}
}

func TestParseChapterTitlesSanitizesFrontMatterFromModel(t *testing.T) {
	raw := `{"chapters":["Preface","Chapter 1 Foundations","References","Chapter 2 Systems"]}`
	got := parser.ParseChapterTitles(raw)

	if len(got) != 2 {
		t.Fatalf("expected front matter removed, got %#v", got)
	}
	if got[0] != "Chapter 1 Foundations" || got[1] != "Chapter 2 Systems" {
		t.Fatalf("unexpected sanitized chapters: %#v", got)
	}
}

func TestPickTopicForSectionHeadingOverlapWins(t *testing.T) {
	section := notebook.ExtractedSection{
		Heading: "Thermodynamics laws and entropy",
		Text:    "This short section mentions biology once.",
	}
	topics := []string{"Cell Biology", "Thermodynamics"}

	idx, confident := parser.PickTopicForSection(section, topics, -1)
	if !confident {
		t.Fatalf("expected confident match from heading overlap")
	}
	if idx != 1 {
		t.Fatalf("expected heading-weighted match to favor thermodynamics, got %d", idx)
	}
}

func TestPickTopicForSectionZeroScoreUsesPriorMatch(t *testing.T) {
	section := notebook.ExtractedSection{
		Heading: "Appendix note",
		Text:    "zxqv mbnr plkm",
	}
	topics := []string{"Topic A", "Topic B", "Topic C"}

	idx, confident := parser.PickTopicForSection(section, topics, 2)
	if confident {
		t.Fatalf("expected zero-score fallback to prior match to be non-confident")
	}
	if idx != 2 {
		t.Fatalf("expected prior match index 2, got %d", idx)
	}
}

func TestExtractInlineChapterCandidatesFromFlattenedTOC(t *testing.T) {
	text := "A Note to Readers Chapter 1 The Truth about Relativity Why Everything Is Relative Chapter 2 The Fallacy of Supply and Demand Why Prices Drift Chapter 3 The Cost of Zero Cost"
	got := parser.ExtractInlineChapterCandidates(text)

	if len(got) < 3 {
		t.Fatalf("expected inline chapter extraction from flattened text, got %#v", got)
	}
	if got[0] != "Chapter 1 The Truth about Relativity Why Everything Is Relative" {
		t.Fatalf("unexpected first chapter segment: %#v", got)
	}
}

func TestExtractInlineNumericTOCCandidatesIncludesIntroAndPostscript(t *testing.T) {
	text := "Introduction The Greatest Show On Earth 1. No One's Crazy 2. Luck & Risk 3. Never Enough 20. Confessions Postscript A Brief History of Why the U.S. Consumer Thinks the Way They Do"
	got := parser.ExtractInlineNumericTOCCandidates(text)

	if len(got) < 6 {
		t.Fatalf("expected intro, numeric chapters, and postscript, got %#v", got)
	}
	if got[0] != "Introduction The Greatest Show On Earth" {
		t.Fatalf("expected introduction first, got %#v", got)
	}
	if got[1] != "1. No One's Crazy" {
		t.Fatalf("expected numeric chapter extraction, got %#v", got)
	}
	if got[len(got)-1] != "Postscript A Brief History of Why the U.S. Consumer Thinks the Way They Do" {
		t.Fatalf("expected postscript extraction, got %#v", got)
	}
}

func TestExtractChapterTitlesFallsBackWhenLLMTooLossy(t *testing.T) {
	app := &App{
		llmProvider: &chapterTestLLM{
			answer: `{"chapters":["The Truth about Relativity","The Power of Price"]}`,
		},
	}

	doc := &notebook.ExtractedDocument{
		Sections: []notebook.ExtractedSection{
			{
				Text: "Chapter 1 The Truth about Relativity Chapter 2 The Fallacy of Supply and Demand Chapter 3 The Cost of Zero Cost Chapter 4 The Cost of Social Norms Chapter 5 The Influence of Arousal",
			},
		},
	}

	got := tutor.ExtractChapterTitles(app.llmProvider, doc)
	if len(got) < 4 {
		t.Fatalf("expected deterministic fallback to preserve richer chapter list, got %#v", got)
	}
}

func TestShouldPreferDeterministicCandidatesRequiresNearCompleteCoverage(t *testing.T) {
	deterministic := []string{
		"Introduction The Greatest Show On Earth",
		"1. No One's Crazy",
		"2. Luck & Risk",
		"3. Never Enough",
		"4. Confounding Compounding",
		"5. Getting Wealthy vs. Staying Wealthy",
		"6. Tails, You Win",
		"7. Freedom",
		"8. Man in the Car Paradox",
		"9. Wealth Is What You Don't See",
		"10. Save Money",
		"11. Reasonable > Rational",
		"12. Surprise!",
		"13. Room for Error",
		"14. You'll Change",
		"15. Nothing's Free",
		"16. You & Me",
		"17. The Seduction of Pessimism",
		"18. When You'll Believe Anything",
		"19. All Together Now",
		"20. Confessions",
		"Postscript A Brief History",
	}

	llmReduced := []string{
		"4. Confounding Compounding",
		"5. Getting Wealthy vs. Staying Wealthy",
		"6. Tails, You Win",
		"7. Freedom",
		"8. Man in the Car Paradox",
		"9. Wealth Is What You Don't See",
		"10. Save Money",
		"11. Reasonable > Rational",
		"12. Surprise!",
		"13. Room for Error",
		"14. You'll Change",
	}

	if !parser.ShouldPreferDeterministicCandidates(llmReduced, deterministic) {
		t.Fatalf("expected deterministic preference when LLM drops large chapter ranges")
	}
}

func TestDetectChapterBoundaryTopicUsesChapterNumber(t *testing.T) {
	section := notebook.ExtractedSection{
		Heading: "Chapter 5",
		Text:    "The Influence of Arousal",
	}
	topics := []string{
		"The Truth about Relativity",
		"The Fallacy of Supply and Demand",
		"The Cost of Zero Cost",
		"The Cost of Social Norms",
		"The Influence of Arousal",
	}

	idx, ok := parser.DetectChapterBoundaryTopic(section, topics, 1)
	if !ok {
		t.Fatalf("expected chapter boundary detection to match")
	}
	if idx != 4 {
		t.Fatalf("expected chapter 5 to map to index 4, got %d", idx)
	}
}
