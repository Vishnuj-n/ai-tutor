package db

import (
	"testing"
)

func TestNotebookExamDeadlineAndRemainingWords(t *testing.T) {
	initDBForTest(t, false, 0)

	notebookID := "nb-deadline-test"
	topicID := "topic-deadline-test"

	if err := EnsureTopic(topicID, "Deadline Topic"); err != nil {
		t.Fatalf("EnsureTopic failed: %v", err)
	}

	if err := CreateNotebook(notebookID, "Deadline Notebook", "/tmp/deadline.txt", "txt", topicID, 10); err != nil {
		t.Fatalf("CreateNotebook failed: %v", err)
	}

	// 1. Verify initially exam_deadline is nil
	nb, err := GetNotebookByID(notebookID)
	if err != nil {
		t.Fatalf("GetNotebookByID failed: %v", err)
	}
	if nb.ExamDeadline != nil {
		t.Fatalf("expected nil exam deadline initially, got %v", *nb.ExamDeadline)
	}

	// 2. Set deadline
	deadline := "2026-06-15"
	if err := UpdateNotebookExamDeadline(notebookID, &deadline); err != nil {
		t.Fatalf("UpdateNotebookExamDeadline failed: %v", err)
	}

	// 3. Verify read back
	nb, err = GetNotebookByID(notebookID)
	if err != nil {
		t.Fatalf("GetNotebookByID failed: %v", err)
	}
	if nb.ExamDeadline == nil || *nb.ExamDeadline != deadline {
		t.Fatalf("expected deadline %q, got %v", deadline, nb.ExamDeadline)
	}

	// 4. Test GetRemainingWords logic
	// Create chunks with different pages:
	// Chunk 1: Page 1, Text: "word1 word2" (2 words)
	// Chunk 2: Page 2, Text: "word1 word2 word3" (3 words)
	// Chunk 3: Page 3, Text: "word1 word2 word3 word4" (4 words)
	chunks := []struct {
		id   string
		text string
		page int
	}{
		{"c-d-1", "word1 word2", 1},
		{"c-d-2", "word1 word2 word3", 2},
		{"c-d-3", "word1 word2 word3 word4", 3},
	}

	for _, c := range chunks {
		if err := CreateChunk(c.id, topicID, c.text, 2, c.page); err != nil {
			t.Fatalf("CreateChunk failed: %v", err)
		}
		if err := LinkChunksToNotebook(notebookID, []string{c.id}); err != nil {
			t.Fatalf("LinkChunksToNotebook failed: %v", err)
		}
	}

	// Initially, topic.current_page_cursor is 0.
	// So remaining words should be all words: 2 + 3 + 4 = 9 words.
	remWords, err := GetRemainingWords(notebookID)
	if err != nil {
		t.Fatalf("GetRemainingWords failed: %v", err)
	}
	if remWords != 9 {
		t.Fatalf("expected 9 remaining words, got %d", remWords)
	}

	// Set topic start_page and end_page, and cursor = 1
	if err := UpdateTopicPageBounds(topicID, 1, 3); err != nil {
		t.Fatalf("UpdateTopicPageBounds failed: %v", err)
	}
	if err := UpdateTopicReadingCursor(topicID, 1, false); err != nil {
		t.Fatalf("UpdateTopicReadingCursor failed: %v", err)
	}

	// Since cursor is 1:
	// Chunk 1 (page 1) is read (1 <= cursor)
	// Chunks 2 & 3 (pages 2 & 3) are remaining (2 > 1, 3 > 1)
	// Remaining words = 3 + 4 = 7 words
	remWords, err = GetRemainingWords(notebookID)
	if err != nil {
		t.Fatalf("GetRemainingWords failed: %v", err)
	}
	if remWords != 7 {
		t.Fatalf("expected 7 remaining words, got %d", remWords)
	}

	// Set cursor = 2
	if err := UpdateTopicReadingCursor(topicID, 2, false); err != nil {
		t.Fatalf("UpdateTopicReadingCursor failed: %v", err)
	}
	// Chunks 1 & 2 read. Chunk 3 (page 3) remaining.
	// Remaining words = 4 words
	remWords, err = GetRemainingWords(notebookID)
	if err != nil {
		t.Fatalf("GetRemainingWords failed: %v", err)
	}
	if remWords != 4 {
		t.Fatalf("expected 4 remaining words, got %d", remWords)
	}

	// Set cursor = 3 (topic completed/learned)
	if err := UpdateTopicReadingCursor(topicID, 3, true); err != nil {
		t.Fatalf("UpdateTopicReadingCursor failed: %v", err)
	}
	// All read.
	// Remaining words = 0 words
	remWords, err = GetRemainingWords(notebookID)
	if err != nil {
		t.Fatalf("GetRemainingWords failed: %v", err)
	}
	if remWords != 0 {
		t.Fatalf("expected 0 remaining words, got %d", remWords)
	}

	// Clear deadline
	if err := UpdateNotebookExamDeadline(notebookID, nil); err != nil {
		t.Fatalf("UpdateNotebookExamDeadline to nil failed: %v", err)
	}
	nb, err = GetNotebookByID(notebookID)
	if err != nil {
		t.Fatalf("GetNotebookByID failed: %v", err)
	}
	if nb.ExamDeadline != nil {
		t.Fatalf("expected nil exam deadline after clearing, got %v", *nb.ExamDeadline)
	}
}
