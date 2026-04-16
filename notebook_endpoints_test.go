package main

import (
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

	got := extractDeterministicChapterCandidates(doc)
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

	got := extractDeterministicChapterCandidates(doc)
	if len(got) == 0 {
		t.Fatalf("expected at least one candidate, got %#v", got)
	}
	for _, title := range got {
		if isFrontMatterTitle(title) {
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

	got := extractDeterministicChapterCandidates(doc)
	if len(got) != maxChapterTitles {
		t.Fatalf("expected candidate cap of %d, got %d (%#v)", maxChapterTitles, len(got), got)
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

	got := app.extractChapterTitles(doc)
	if len(got) < 2 {
		t.Fatalf("expected deterministic fallback chapters, got %#v", got)
	}
	if got[0] != "Chapter 1: Intro" {
		t.Fatalf("expected fallback chapter ordering, got %#v", got)
	}
}

func TestParseChapterTitlesSanitizesFrontMatterFromModel(t *testing.T) {
	raw := `{"chapters":["Preface","Chapter 1 Foundations","References","Chapter 2 Systems"]}`
	got := parseChapterTitles(raw)

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

	idx, confident := pickTopicForSection(section, topics, -1)
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

	idx, confident := pickTopicForSection(section, topics, 2)
	if confident {
		t.Fatalf("expected zero-score fallback to prior match to be non-confident")
	}
	if idx != 2 {
		t.Fatalf("expected prior match index 2, got %d", idx)
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

	idx, ok := detectChapterBoundaryTopic(section, topics, 1)
	if !ok {
		t.Fatalf("expected chapter boundary detection to match")
	}
	if idx != 4 {
		t.Fatalf("expected chapter 5 to map to index 4, got %d", idx)
	}
}
