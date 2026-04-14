package notebook

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSplitIntoWordChunks(t *testing.T) {
	text10 := "one two three four five six seven eight nine ten"
	text9 := "one two three four five six seven eight nine"
	text8 := "one two three four five six seven eight"

	tests := []struct {
		name      string
		text      string
		chunkSize int
		overlap   int
		want      []string
	}{
		{
			name:      "exact fit single chunk",
			text:      text10,
			chunkSize: 10,
			overlap:   2,
			want:      []string{"one two three four five six seven eight nine ten"},
		},
		{
			name:      "overlap stride math",
			text:      text10,
			chunkSize: 4,
			overlap:   1,
			want: []string{
				"one two three four",
				"four five six seven",
				"seven eight nine ten",
			},
		},
		{
			name:      "trailing short chunk",
			text:      text9,
			chunkSize: 4,
			overlap:   0,
			want: []string{
				"one two three four",
				"five six seven eight",
				"nine",
			},
		},
		{
			name:      "negative overlap normalizes to zero",
			text:      text9,
			chunkSize: 4,
			overlap:   -3,
			want: []string{
				"one two three four",
				"five six seven eight",
				"nine",
			},
		},
		{
			name:      "overlap at or above chunk size normalizes",
			text:      text8,
			chunkSize: 4,
			overlap:   4,
			want: []string{
				"one two three four",
				"three four five six",
				"five six seven eight",
			},
		},
		{
			name:      "whitespace only returns nil",
			text:      " \n\t ",
			chunkSize: 5,
			overlap:   1,
			want:      nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := SplitIntoWordChunks(tc.text, tc.chunkSize, tc.overlap)
			if !equalStringSlices(got, tc.want) {
				t.Fatalf("unexpected chunks:\n got=%#v\nwant=%#v", got, tc.want)
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

func equalStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if strings.TrimSpace(a[i]) != strings.TrimSpace(b[i]) {
			return false
		}
	}
	return true
}
