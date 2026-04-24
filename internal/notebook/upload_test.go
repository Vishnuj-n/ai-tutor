package notebook

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSplitPageIntoSemanticChunks(t *testing.T) {
	tests := []struct {
		name      string
		text      string
		target    int
		wantCount int
		assert    func(t *testing.T, got []string)
	}{
		{
			name:      "splits near period around 150 words",
			text:      buildSentenceBlob(12, 14),
			target:    150,
			wantCount: 2,
			assert: func(t *testing.T, got []string) {
				t.Helper()
				if !strings.HasSuffix(got[0], ".") {
					t.Fatalf("expected first chunk to end at sentence boundary, got=%q", got[0])
				}
			},
		},
		{
			name:      "prefers newline boundary in range",
			text:      buildWords(120) + "\n" + buildWordsRange(121, 220),
			target:    150,
			wantCount: 2,
			assert: func(t *testing.T, got []string) {
				t.Helper()
				if got[0] != buildWords(120) {
					t.Fatalf("expected newline split at 120 words, got first=%q", got[0])
				}
			},
		},
		{
			name:      "falls back to target when no period or newline",
			text:      buildWords(320),
			target:    150,
			wantCount: 2,
			assert: func(t *testing.T, got []string) {
				t.Helper()
				if len(strings.Fields(got[0])) != 150 {
					t.Fatalf("expected fallback first chunk size 150, got=%d", len(strings.Fields(got[0])))
				}
			},
		},
		{
			name:      "short text stays single chunk",
			text:      buildWords(40),
			target:    150,
			wantCount: 1,
		},
		{
			name:      "whitespace only returns nil",
			text:      " \n\t ",
			target:    150,
			wantCount: 0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := SplitPageIntoSemanticChunks(tc.text, tc.target)
			if len(got) != tc.wantCount {
				t.Fatalf("unexpected chunk count: got=%d want=%d chunks=%#v", len(got), tc.wantCount, got)
			}
			if tc.assert != nil {
				tc.assert(t, got)
			}
		})
	}
}

func TestExtractDocumentTXTNormalization(t *testing.T) {
	service := NewService(t.TempDir())
	txtPath := writeTempFile(t, t.TempDir(), "notes.txt", []byte("  Alpha\tbeta  \n\ngamma   "))

	doc, err := service.ExtractDocument(txtPath, "txt")
	if err != nil {
		t.Fatalf("ExtractDocument returned error: %v", err)
	}

	if doc.PageCount != 1 {
		t.Fatalf("expected page count 1, got %d", doc.PageCount)
	}
	if doc.WordCount != 3 {
		t.Fatalf("expected word count 3, got %d", doc.WordCount)
	}
	if len(doc.Sections) != 1 {
		t.Fatalf("expected one section, got %d", len(doc.Sections))
	}
	if doc.Sections[0].Heading != "Document" {
		t.Fatalf("expected heading Document, got %q", doc.Sections[0].Heading)
	}
	if doc.Sections[0].Text != "Alpha beta gamma" {
		t.Fatalf("unexpected normalized text: %q", doc.Sections[0].Text)
	}
}

func TestExtractDocumentMarkdownNormalization(t *testing.T) {
	service := NewService(t.TempDir())
	mdContent := "# Intro\n\n Alpha   beta \n\n## Deep Dive\n gamma\t delta \n"
	mdPath := writeTempFile(t, t.TempDir(), "notes.md", []byte(mdContent))

	doc, err := service.ExtractDocument(mdPath, "md")
	if err != nil {
		t.Fatalf("ExtractDocument returned error: %v", err)
	}

	if doc.PageCount != 1 {
		t.Fatalf("expected page count 1, got %d", doc.PageCount)
	}
	if doc.WordCount != 4 {
		t.Fatalf("expected word count 4, got %d", doc.WordCount)
	}
	if len(doc.Sections) != 2 {
		t.Fatalf("expected two sections, got %d", len(doc.Sections))
	}
	if doc.Sections[0].Heading != "Intro" || doc.Sections[0].Text != "Alpha beta" {
		t.Fatalf("unexpected first section: %#v", doc.Sections[0])
	}
	if doc.Sections[1].Heading != "Deep Dive" || doc.Sections[1].Text != "gamma delta" {
		t.Fatalf("unexpected second section: %#v", doc.Sections[1])
	}
}

func TestExtractDocumentPDFBranchViaSeam(t *testing.T) {
	pdfPath := writeTempFile(t, t.TempDir(), "notes.pdf", []byte("%PDF-1.4 placeholder"))

	service := NewService(t.TempDir(), WithExtractPDFFunc(func(filePath string, doc *ExtractedDocument) error {
		if filePath != pdfPath {
			return fmt.Errorf("unexpected file path: %s", filePath)
		}
		doc.PageCount = 2
		doc.WordCount = 5
		doc.Sections = []ExtractedSection{
			{Heading: "Page 1", Text: "alpha beta", PageNum: 1},
			{Heading: "Page 2", Text: "gamma delta epsilon", PageNum: 2},
		}
		return nil
	}))

	doc, err := service.ExtractDocument(pdfPath, "pdf")
	if err != nil {
		t.Fatalf("ExtractDocument returned error: %v", err)
	}

	if doc.PageCount != 2 {
		t.Fatalf("expected page count 2, got %d", doc.PageCount)
	}
	if doc.WordCount != 5 {
		t.Fatalf("expected word count 5, got %d", doc.WordCount)
	}
	if len(doc.Sections) != 2 {
		t.Fatalf("expected two sections, got %d", len(doc.Sections))
	}
	if doc.Sections[0].Heading != "Page 1" || doc.Sections[0].PageNum != 1 {
		t.Fatalf("unexpected first section: %#v", doc.Sections[0])
	}
	if doc.Sections[1].Heading != "Page 2" || doc.Sections[1].PageNum != 2 {
		t.Fatalf("unexpected second section: %#v", doc.Sections[1])
	}
}

func writeTempFile(t *testing.T, dir, fileName string, body []byte) string {
	t.Helper()

	path := filepath.Join(dir, fileName)
	if err := os.WriteFile(path, body, 0o644); err != nil {
		t.Fatalf("failed to write %s: %v", fileName, err)
	}
	return path
}

func buildWords(n int) string {
	return buildWordsRange(1, n)
}

func buildWordsRange(start, end int) string {
	out := make([]string, 0, end-start+1)
	for i := start; i <= end; i++ {
		out = append(out, fmt.Sprintf("w%d", i))
	}
	return strings.Join(out, " ")
}

func buildSentenceBlob(sentences, wordsPerSentence int) string {
	if sentences <= 0 || wordsPerSentence <= 0 {
		return ""
	}
	parts := make([]string, 0, sentences)
	word := 1
	for i := 0; i < sentences; i++ {
		line := make([]string, 0, wordsPerSentence)
		for j := 0; j < wordsPerSentence; j++ {
			line = append(line, fmt.Sprintf("w%d", word))
			word++
		}
		parts = append(parts, strings.Join(line, " ")+".")
	}
	return strings.Join(parts, " ")
}
