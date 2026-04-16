package db

import (
	"ai-tutor/internal/models"
	"strings"
	"testing"
)

func TestIngestNotebookContentByTopicRollsBackOnMidTransactionFailure(t *testing.T) {
	initDBForTest(t, false, 0)

	notebookID := "nb-rollback"
	if err := CreateNotebook(notebookID, "Rollback Notebook", "/tmp/rollback.txt", "txt", "os-scheduling", 1); err != nil {
		t.Fatalf("CreateNotebook failed: %v", err)
	}
	if err := UpdateNotebookStatus(notebookID, "uploaded_unlinked"); err != nil {
		t.Fatalf("UpdateNotebookStatus failed: %v", err)
	}
	if err := UpdateNotebookChunkCount(notebookID, 7); err != nil {
		t.Fatalf("UpdateNotebookChunkCount failed: %v", err)
	}

	groups := []NotebookTopicIngestionGroup{
		{
			TopicID: "os-scheduling",
			Parents: []NotebookParentInput{
				{ID: "nbp_nb-rollback_1", Heading: "Valid", Content: "valid parent", OrderIndex: 1},
			},
			Chunks: []NotebookChunkInput{
				{ID: "nbc_nb-rollback_1_1", ParentID: "nbp_nb-rollback_1", Text: "valid chunk", TokenCount: 2, PageNum: 1},
			},
		},
		{
			TopicID: "",
			Parents: []NotebookParentInput{
				{ID: "nbp_nb-rollback_2", Heading: "Invalid", Content: "invalid parent", OrderIndex: 2},
			},
		},
	}

	err := IngestNotebookContentByTopic(notebookID, groups)
	if err == nil {
		t.Fatalf("expected ingestion to fail for empty topic id")
	}

	assertCountEquals(t, `SELECT COUNT(*) FROM parents WHERE id LIKE ?`, "nbp_nb-rollback_%", 0)
	assertCountEquals(t, `SELECT COUNT(*) FROM chunks WHERE id LIKE ?`, "nbc_nb-rollback_%", 0)
	assertCountEquals(t, `SELECT COUNT(*) FROM notebook_chunks WHERE notebook_id = ?`, notebookID, 0)

	var status string
	var chunkCount int
	if err := conn.QueryRow(`SELECT status, chunk_count FROM notebooks WHERE id = ?`, notebookID).Scan(&status, &chunkCount); err != nil {
		t.Fatalf("failed to read notebook state: %v", err)
	}
	if status != "uploaded_unlinked" {
		t.Fatalf("expected notebook status rollback to uploaded_unlinked, got %q", status)
	}
	if chunkCount != 7 {
		t.Fatalf("expected notebook chunk_count rollback to 7, got %d", chunkCount)
	}
}

func TestDeleteNotebookRemovesLinkedDataAndPreservesUnrelatedRows(t *testing.T) {
	initDBForTest(t, true, 3)

	notebookID := "nb-delete"
	keepNotebookID := "nb-keep"
	autoTopicID := "nb-" + notebookID + "-topic-a"
	keepTopicID := "topic-keep"

	if err := EnsureTopic(autoTopicID, "Auto Topic"); err != nil {
		t.Fatalf("EnsureTopic auto failed: %v", err)
	}
	if err := EnsureTopic(keepTopicID, "Keep Topic"); err != nil {
		t.Fatalf("EnsureTopic keep failed: %v", err)
	}
	if _, err := conn.Exec(`INSERT INTO topic_progress (topic_id, mastery_score) VALUES (?, 0.1)`, autoTopicID); err != nil {
		t.Fatalf("failed to insert topic_progress: %v", err)
	}

	if err := CreateNotebook(notebookID, "Delete Notebook", "/tmp/del.txt", "txt", autoTopicID, 1); err != nil {
		t.Fatalf("CreateNotebook delete target failed: %v", err)
	}
	if err := CreateNotebook(keepNotebookID, "Keep Notebook", "/tmp/keep.txt", "txt", keepTopicID, 1); err != nil {
		t.Fatalf("CreateNotebook keep target failed: %v", err)
	}

	parentDelID := "parent-del"
	chunkDelID := "chunk-del"
	if err := CreateParentSection(parentDelID, autoTopicID, "Delete Heading", 1, "delete parent body"); err != nil {
		t.Fatalf("CreateParentSection delete failed: %v", err)
	}
	if err := CreateChunk(chunkDelID, autoTopicID, parentDelID, "delete chunk body", 3); err != nil {
		t.Fatalf("CreateChunk delete failed: %v", err)
	}
	if err := LinkChunksToNotebook(notebookID, []string{chunkDelID}); err != nil {
		t.Fatalf("LinkChunksToNotebook delete failed: %v", err)
	}

	parentKeepID := "parent-keep"
	chunkKeepID := "chunk-keep"
	if err := CreateParentSection(parentKeepID, keepTopicID, "Keep Heading", 1, "keep parent body"); err != nil {
		t.Fatalf("CreateParentSection keep failed: %v", err)
	}
	if err := CreateChunk(chunkKeepID, keepTopicID, parentKeepID, "keep chunk body", 3); err != nil {
		t.Fatalf("CreateChunk keep failed: %v", err)
	}
	if err := LinkChunksToNotebook(keepNotebookID, []string{chunkKeepID}); err != nil {
		t.Fatalf("LinkChunksToNotebook keep failed: %v", err)
	}

	if err := UpsertChunkVector(chunkDelID, []float32{1, 0, 0}); err != nil {
		t.Fatalf("UpsertChunkVector delete failed: %v", err)
	}
	if err := UpsertChunkVector(chunkKeepID, []float32{0, 1, 0}); err != nil {
		t.Fatalf("UpsertChunkVector keep failed: %v", err)
	}

	if err := DeleteNotebook(notebookID); err != nil {
		t.Fatalf("DeleteNotebook failed: %v", err)
	}

	assertCountEquals(t, `SELECT COUNT(*) FROM notebooks WHERE id = ?`, notebookID, 0)
	assertCountEquals(t, `SELECT COUNT(*) FROM notebook_chunks WHERE notebook_id = ?`, notebookID, 0)
	assertCountEquals(t, `SELECT COUNT(*) FROM chunks WHERE id = ?`, chunkDelID, 0)
	assertCountEquals(t, `SELECT COUNT(*) FROM parents WHERE id = ?`, parentDelID, 0)
	assertCountEquals(t, `SELECT COUNT(*) FROM topic_progress WHERE topic_id = ?`, autoTopicID, 0)
	assertCountEquals(t, `SELECT COUNT(*) FROM topics WHERE id = ?`, autoTopicID, 0)
	assertCountEquals(t, `SELECT COUNT(*) FROM chunk_vectors cv JOIN chunks c ON c.rowid = cv.rowid WHERE c.id = ?`, chunkDelID, 0)

	assertCountEquals(t, `SELECT COUNT(*) FROM notebooks WHERE id = ?`, keepNotebookID, 1)
	assertCountEquals(t, `SELECT COUNT(*) FROM chunks WHERE id = ?`, chunkKeepID, 1)
	assertCountEquals(t, `SELECT COUNT(*) FROM parents WHERE id = ?`, parentKeepID, 1)
	assertCountEquals(t, `SELECT COUNT(*) FROM topics WHERE id = ?`, keepTopicID, 1)
	assertCountEquals(t, `SELECT COUNT(*) FROM chunk_vectors cv JOIN chunks c ON c.rowid = cv.rowid WHERE c.id = ?`, chunkKeepID, 1)
}

func TestSearchVectorsForTopicScopesResultsByTopicID(t *testing.T) {
	initDBForTest(t, true, 3)
	if !distanceFunctionAvailable(t) {
		t.Skip("sqlite-vec distance() function is unavailable in this runtime")
	}

	topicA := "topic-scope-a"
	topicB := "topic-scope-b"
	if err := EnsureTopic(topicA, "Topic A"); err != nil {
		t.Fatalf("EnsureTopic topicA failed: %v", err)
	}
	if err := EnsureTopic(topicB, "Topic B"); err != nil {
		t.Fatalf("EnsureTopic topicB failed: %v", err)
	}

	parentA := "parent-scope-a"
	chunkA := "chunk-scope-a"
	if err := CreateParentSection(parentA, topicA, "A", 1, "topic a parent"); err != nil {
		t.Fatalf("CreateParentSection topicA failed: %v", err)
	}
	if err := CreateChunk(chunkA, topicA, parentA, "topic a chunk", 3); err != nil {
		t.Fatalf("CreateChunk topicA failed: %v", err)
	}

	parentB := "parent-scope-b"
	chunkB := "chunk-scope-b"
	if err := CreateParentSection(parentB, topicB, "B", 1, "topic b parent"); err != nil {
		t.Fatalf("CreateParentSection topicB failed: %v", err)
	}
	if err := CreateChunk(chunkB, topicB, parentB, "topic b chunk", 3); err != nil {
		t.Fatalf("CreateChunk topicB failed: %v", err)
	}

	// Topic B is globally closer to the query, but scoped search for topic A must never return it.
	if err := UpsertChunkVector(chunkA, []float32{0, 1, 0}); err != nil {
		t.Fatalf("UpsertChunkVector chunkA failed: %v", err)
	}
	if err := UpsertChunkVector(chunkB, []float32{1, 0, 0}); err != nil {
		t.Fatalf("UpsertChunkVector chunkB failed: %v", err)
	}

	query := []float32{1, 0, 0}
	gotA, err := SearchVectorsForTopic(topicA, query, 5)
	if err != nil {
		t.Fatalf("SearchVectorsForTopic topicA failed: %v", err)
	}
	if len(gotA) == 0 {
		t.Fatalf("expected at least one scoped result for topicA")
	}
	if contains(gotA, chunkB) {
		t.Fatalf("scoped results leaked chunk from another topic: %#v", gotA)
	}
	if !contains(gotA, chunkA) {
		t.Fatalf("expected scoped results to contain chunkA, got %#v", gotA)
	}

	gotB, err := SearchVectorsForTopic(topicB, query, 5)
	if err != nil {
		t.Fatalf("SearchVectorsForTopic topicB failed: %v", err)
	}
	if len(gotB) == 0 || gotB[0] != chunkB {
		t.Fatalf("expected topicB to return its own chunk first, got %#v", gotB)
	}
}

func TestGetNotebookTopicTreeDeduplicatesTopicRowsPerNotebook(t *testing.T) {
	initDBForTest(t, false, 0)

	notebookID := "nb-tree-dedupe"
	topicID := "topic-tree-dedupe"
	if err := CreateNotebook(notebookID, "Dedupe Notebook", "/tmp/dedupe.txt", "txt", "", 1); err != nil {
		t.Fatalf("CreateNotebook failed: %v", err)
	}
	if err := EnsureTopic(topicID, "Shared Topic"); err != nil {
		t.Fatalf("EnsureTopic failed: %v", err)
	}

	parentID := "parent-tree-dedupe"
	if err := CreateParentSection(parentID, topicID, "Shared Heading", 1, "shared parent"); err != nil {
		t.Fatalf("CreateParentSection failed: %v", err)
	}

	chunkA := "chunk-tree-dedupe-a"
	chunkB := "chunk-tree-dedupe-b"
	if err := CreateChunk(chunkA, topicID, parentID, "chunk a", 2); err != nil {
		t.Fatalf("CreateChunk chunkA failed: %v", err)
	}
	if err := CreateChunk(chunkB, topicID, parentID, "chunk b", 2); err != nil {
		t.Fatalf("CreateChunk chunkB failed: %v", err)
	}

	if err := LinkChunksToNotebook(notebookID, []string{chunkA, chunkB}); err != nil {
		t.Fatalf("LinkChunksToNotebook failed: %v", err)
	}

	tree, err := GetNotebookTopicTree()
	if err != nil {
		t.Fatalf("GetNotebookTopicTree failed: %v", err)
	}
	if len(tree) != 1 {
		t.Fatalf("expected 1 notebook, got %#v", tree)
	}
	if len(tree[0].Topics) != 1 {
		t.Fatalf("expected deduped single topic entry, got %#v", tree[0].Topics)
	}
	if tree[0].Topics[0].TopicID != topicID {
		t.Fatalf("unexpected topic id: %#v", tree[0].Topics)
	}
}

func TestGetNotebookTopicTreeIncludesTopiclessAndIgnoresBrokenLinks(t *testing.T) {
	initDBForTest(t, false, 0)

	notebookID := "nb-tree-empty"
	if err := CreateNotebook(notebookID, "Empty Notebook", "/tmp/empty.txt", "txt", "", 1); err != nil {
		t.Fatalf("CreateNotebook failed: %v", err)
	}

	if _, err := conn.Exec(`PRAGMA foreign_keys = OFF`); err != nil {
		t.Fatalf("failed to disable foreign keys: %v", err)
	}
	t.Cleanup(func() {
		if _, err := conn.Exec(`PRAGMA foreign_keys = ON`); err != nil {
			t.Fatalf("failed to re-enable foreign keys: %v", err)
		}
	})

	if _, err := conn.Exec(`
		INSERT INTO notebook_chunks (id, notebook_id, chunk_id, page_num)
		VALUES (?, ?, ?, ?)
	`, "broken-link", notebookID, "missing-chunk", 1); err != nil {
		t.Fatalf("failed to insert broken notebook chunk link: %v", err)
	}

	tree, err := GetNotebookTopicTree()
	if err != nil {
		t.Fatalf("GetNotebookTopicTree failed: %v", err)
	}
	if len(tree) != 1 {
		t.Fatalf("expected 1 notebook, got %#v", tree)
	}
	if tree[0].NotebookID != notebookID {
		t.Fatalf("unexpected notebook entry: %#v", tree[0])
	}
	if len(tree[0].Topics) != 0 {
		t.Fatalf("expected empty topics for topicless/broken notebook, got %#v", tree[0].Topics)
	}
}

func TestGetNotebookTopicTreeIncludesNotebookChapterTopicsWithoutChunks(t *testing.T) {
	initDBForTest(t, false, 0)

	notebookID := "nb-tree-canonical"
	if err := CreateNotebook(notebookID, "Canonical Notebook", "/tmp/canonical.txt", "txt", "", 1); err != nil {
		t.Fatalf("CreateNotebook failed: %v", err)
	}

	topicIDs := []string{
		"nb-" + notebookID + "-ch-01-relativity",
		"nb-" + notebookID + "-ch-02-demand",
		"nb-" + notebookID + "-ch-03-zero",
	}
	topicTitles := []string{
		"The Truth about Relativity",
		"The Fallacy of Supply and Demand",
		"The Cost of Zero Cost",
	}

	for i := range topicIDs {
		if err := EnsureTopic(topicIDs[i], topicTitles[i]); err != nil {
			t.Fatalf("EnsureTopic %d failed: %v", i, err)
		}
	}

	tree, err := GetNotebookTopicTree()
	if err != nil {
		t.Fatalf("GetNotebookTopicTree failed: %v", err)
	}
	if len(tree) != 1 {
		t.Fatalf("expected 1 notebook, got %#v", tree)
	}
	if tree[0].NotebookID != notebookID {
		t.Fatalf("unexpected notebook id: %#v", tree[0])
	}
	if len(tree[0].Topics) != 3 {
		t.Fatalf("expected all canonical chapter topics to appear, got %#v", tree[0].Topics)
	}
	if tree[0].Topics[0].TopicID != topicIDs[0] || tree[0].Topics[1].TopicID != topicIDs[1] || tree[0].Topics[2].TopicID != topicIDs[2] {
		t.Fatalf("expected chapter topic order by chapter id, got %#v", tree[0].Topics)
	}
}

func distanceFunctionAvailable(t *testing.T) bool {
	t.Helper()

	var distance float64
	err := conn.QueryRow(`SELECT distance(?, ?)`, "[1,0,0]", "[1,0,0]").Scan(&distance)
	if err != nil {
		return false
	}
	// Identical vectors should yield ~0 distance
	return distance < 1e-9 && distance > -1e-9
}

func assertCountEquals(t *testing.T, query string, arg interface{}, want int) {
	t.Helper()

	var got int
	if err := conn.QueryRow(query, arg).Scan(&got); err != nil {
		t.Fatalf("query failed (%s): %v", sanitizeWhitespace(query), err)
	}
	if got != want {
		t.Fatalf("unexpected count for query (%s): got=%d want=%d", sanitizeWhitespace(query), got, want)
	}
}

func contains(items []string, target string) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
}

func sanitizeWhitespace(input string) string {
	return strings.Join(strings.Fields(input), " ")
}

func TestIngestNotebookContentByTopicRejectsWhitespaceOnlyIDs(t *testing.T) {
	initDBForTest(t, false, 0)

	notebookID := "nb-whitespace-test"
	if err := CreateNotebook(notebookID, "Whitespace Test Notebook", "/tmp/ws.txt", "txt", "", 1); err != nil {
		t.Fatalf("CreateNotebook failed: %v", err)
	}

	// Test 1: Whitespace-only notebookID should be rejected
	groups := []NotebookTopicIngestionGroup{
		{
			TopicID: "valid-topic",
			Parents: []NotebookParentInput{
				{ID: "p1", Heading: "H", Content: "c", OrderIndex: 1},
			},
			Chunks: []NotebookChunkInput{},
		},
	}

	err := IngestNotebookContentByTopic("   ", groups)
	if err == nil {
		t.Fatal("expected IngestNotebookContentByTopic to reject whitespace-only notebookID")
	}
	if !strings.Contains(err.Error(), "notebook id is required") {
		t.Fatalf("unexpected error message: %v", err)
	}

	// Test 2: Whitespace-only TopicID should be rejected
	groups2 := []NotebookTopicIngestionGroup{
		{
			TopicID: "   ",
			Parents: []NotebookParentInput{},
			Chunks:  []NotebookChunkInput{},
		},
	}

	err = IngestNotebookContentByTopic(notebookID, groups2)
	if err == nil {
		t.Fatal("expected IngestNotebookContentByTopic to reject whitespace-only TopicID")
	}
	if !strings.Contains(err.Error(), "topic id is required") {
		t.Fatalf("unexpected error message: %v", err)
	}

	// Test 3: Leading/trailing whitespace should be trimmed for valid IDs
	validGroups := []NotebookTopicIngestionGroup{
		{
			TopicID: "  valid-topic-ws  ",
			Parents: []NotebookParentInput{
				{ID: "p-ws-1", Heading: "H", Content: "c", OrderIndex: 1},
			},
			Chunks: []NotebookChunkInput{
				{ID: "c-ws-1", ParentID: "p-ws-1", Text: "chunk text", TokenCount: 2, PageNum: 1},
			},
		},
	}

	if err := EnsureTopic("valid-topic-ws", "Valid Topic"); err != nil {
		t.Fatalf("EnsureTopic failed: %v", err)
	}

	// This should succeed - whitespace should be trimmed from TopicID
	err = IngestNotebookContentByTopic("  "+notebookID+"  ", validGroups)
	if err != nil {
		t.Fatalf("IngestNotebookContentByTopic with trimmed IDs failed: %v", err)
	}

	// Verify that the topic without whitespace was used (not with whitespace)
	assertCountEquals(t, `SELECT COUNT(*) FROM parents WHERE id = ?`, "p-ws-1", 1)
	assertCountEquals(t, `SELECT COUNT(*) FROM chunks WHERE id = ?`, "c-ws-1", 1)

	// Verify the persisted topic_id is the trimmed value, not the original with whitespace
	var parentTopicID string
	if err := conn.QueryRow(`SELECT topic_id FROM parents WHERE id = ?`, "p-ws-1").Scan(&parentTopicID); err != nil {
		t.Fatalf("failed to query parent topic_id: %v", err)
	}
	if parentTopicID != "valid-topic-ws" {
		t.Fatalf("expected parent topic_id to be trimmed 'valid-topic-ws', got %q", parentTopicID)
	}

	var chunkTopicID string
	if err := conn.QueryRow(`SELECT topic_id FROM chunks WHERE id = ?`, "c-ws-1").Scan(&chunkTopicID); err != nil {
		t.Fatalf("failed to query chunk topic_id: %v", err)
	}
	if chunkTopicID != "valid-topic-ws" {
		t.Fatalf("expected chunk topic_id to be trimmed 'valid-topic-ws', got %q", chunkTopicID)
	}
}

func TestReplaceQuestionsForTopicRejectsTopicIDMismatch(t *testing.T) {
	initDBForTest(t, true, 0)

	topicID := "quiz-mismatch-topic"
	if err := EnsureTopic(topicID, "Quiz Test Topic"); err != nil {
		t.Fatalf("EnsureTopic failed: %v", err)
	}

	// Seed a valid question in the target topic to make rollback assertion meaningful
	seededQuestion := []models.QuizQuestion{
		{
			ID:            "seed-q1",
			TopicID:       topicID,
			Prompt:        "Seeded Question",
			Options:       []string{"yes", "no"},
			CorrectAnswer: "yes",
			Explanation:   "This question tests rollback preservation",
		},
	}
	if err := ReplaceQuestionsForTopic(topicID, seededQuestion); err != nil {
		t.Fatalf("failed to seed question: %v", err)
	}

	// Create questions with mismatched TopicID
	questions := []models.QuizQuestion{
		{
			ID:            "q1",
			TopicID:       "different-topic",
			Prompt:        "Question 1",
			Options:       []string{"a", "b"},
			CorrectAnswer: "a",
			Explanation:   "Explanation",
		},
	}

	// This should fail because question TopicID doesn't match the provided topicID
	err := ReplaceQuestionsForTopic(topicID, questions)
	if err == nil {
		t.Fatal("expected ReplaceQuestionsForTopic to reject question with mismatched TopicID")
	}
	if !strings.Contains(err.Error(), "question topic id must match topic id") {
		t.Fatalf("unexpected error message: %v", err)
	}

	// Verify the seeded question still exists (rollback preserved it)
	assertCountEquals(t, `SELECT COUNT(*) FROM questions WHERE topic_id = ?`, topicID, 1)

	// Verify rollback atomicity: the target topic still exists (not deleted)
	assertCountEquals(t, `SELECT COUNT(*) FROM topics WHERE id = ?`, topicID, 1)

	// Verify no cross-topic side effects: the mismatched topic should not have questions created
	assertCountEquals(t, `SELECT COUNT(*) FROM questions WHERE topic_id = ?`, "different-topic", 0)

	// Verify the mismatched topic was not auto-created during the failed insert attempt
	assertCountEquals(t, `SELECT COUNT(*) FROM topics WHERE id = ?`, "different-topic", 0)

	// Test with valid matching TopicID (either explicit or "" to auto-assign)
	validQuestions := []models.QuizQuestion{
		{
			ID:            "q2",
			TopicID:       "", // Empty will be auto-assigned to topicID
			Prompt:        "Question 2",
			Options:       []string{"x", "y"},
			CorrectAnswer: "x",
			Explanation:   "Valid question",
		},
		{
			ID:            "q3",
			TopicID:       topicID, // Explicit match
			Prompt:        "Question 3",
			Options:       []string{"p", "q"},
			CorrectAnswer: "p",
			Explanation:   "Another valid question",
		},
	}

	// This should succeed
	err = ReplaceQuestionsForTopic(topicID, validQuestions)
	if err != nil {
		t.Fatalf("ReplaceQuestionsForTopic with matching TopicIDs failed: %v", err)
	}

	// Verify questions were inserted with correct TopicID
	assertCountEquals(t, `SELECT COUNT(*) FROM questions WHERE topic_id = ?`, topicID, 2)
}

func TestInsertFSRSReviewLogSuccessfulInsertion(t *testing.T) {
	initDBForTest(t, false, 0)

	topicID := "test-fsrs-topic"
	if err := EnsureTopic(topicID, "FSRS Test Topic"); err != nil {
		t.Fatalf("EnsureTopic failed: %v", err)
	}

	reviewLog := models.FSRSReviewLog{
		ID:              "log-success",
		TopicID:         topicID,
		ActivityType:    "flashcard",
		ReferenceID:     "card-123",
		ReviewedAt:      1234567890,
		Rating:          3,
		ScheduledDays:   7,
		StateBeforeJSON: `{"reps":0,"stability":1.0}`,
		StateAfterJSON:  `{"reps":1,"stability":2.5}`,
	}

	if err := InsertFSRSReviewLog(reviewLog); err != nil {
		t.Fatalf("InsertFSRSReviewLog failed: %v", err)
	}

	// Verify log was persisted
	var id, activity, ref, before, after string
	var reviewed, rating, scheduled int64
	if err := conn.QueryRow(`
		SELECT id, activity_type, reference_id, reviewed_at, rating, scheduled_days,
		       state_before_json, state_after_json
		FROM fsrs_review_log
		WHERE id = ?
	`, "log-success").Scan(&id, &activity, &ref, &reviewed, &rating, &scheduled, &before, &after); err != nil {
		t.Fatalf("failed to query inserted log: %v", err)
	}

	if id != "log-success" || activity != "flashcard" || ref != "card-123" {
		t.Fatalf("unexpected log data: id=%s activity=%s ref=%s", id, activity, ref)
	}
	if reviewed != 1234567890 || rating != 3 || scheduled != 7 {
		t.Fatalf("unexpected log values: reviewed=%d rating=%d scheduled=%d", reviewed, rating, scheduled)
	}
	if before != `{"reps":0,"stability":1.0}` || after != `{"reps":1,"stability":2.5}` {
		t.Fatalf("unexpected state JSON: before=%s after=%s", before, after)
	}

	assertCountEquals(t, `SELECT COUNT(*) FROM fsrs_review_log WHERE topic_id = ?`, topicID, 1)
}

func TestInsertFSRSReviewLogRejectsMissingTopic(t *testing.T) {
	initDBForTest(t, false, 0)

	reviewLog := models.FSRSReviewLog{
		ID:              "log-bad-topic",
		TopicID:         "nonexistent-topic",
		ActivityType:    "flashcard",
		ReferenceID:     "card-456",
		ReviewedAt:      1234567890,
		Rating:          2,
		ScheduledDays:   3,
		StateBeforeJSON: `{}`,
		StateAfterJSON:  `{}`,
	}

	err := InsertFSRSReviewLog(reviewLog)
	if err == nil {
		t.Fatalf("expected error for missing topic, got success")
	}
	if !strings.Contains(err.Error(), "topic not found") {
		t.Fatalf("expected 'topic not found' error, got: %v", err)
	}

	assertCountEquals(t, `SELECT COUNT(*) FROM fsrs_review_log WHERE id = ?`, "log-bad-topic", 0)
}

func TestInsertFSRSReviewLogRejectsInvalidRating(t *testing.T) {
	initDBForTest(t, false, 0)

	topicID := "test-rating-topic"
	if err := EnsureTopic(topicID, "Rating Test Topic"); err != nil {
		t.Fatalf("EnsureTopic failed: %v", err)
	}

	tests := []struct {
		name   string
		rating int
	}{
		{"rating_zero", 0},
		{"rating_negative", -1},
		{"rating_five", 5},
		{"rating_large", 100},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reviewLog := models.FSRSReviewLog{
				ID:              "log-" + tt.name,
				TopicID:         topicID,
				ActivityType:    "flashcard",
				ReferenceID:     "card-" + tt.name,
				ReviewedAt:      1234567890,
				Rating:          tt.rating,
				ScheduledDays:   1,
				StateBeforeJSON: `{}`,
				StateAfterJSON:  `{}`,
			}

			err := InsertFSRSReviewLog(reviewLog)
			if err == nil {
				t.Fatalf("expected error for rating %d, got success", tt.rating)
			}
			if !strings.Contains(err.Error(), "rating must be between 1 and 4") {
				t.Fatalf("expected rating validation error, got: %v", err)
			}

			assertCountEquals(t, `SELECT COUNT(*) FROM fsrs_review_log WHERE id = ?`, "log-"+tt.name, 0)
		})
	}
}

func TestInsertFSRSReviewLogRejectsEmptyID(t *testing.T) {
	initDBForTest(t, false, 0)

	topicID := "test-empty-id-topic"
	if err := EnsureTopic(topicID, "Empty ID Test Topic"); err != nil {
		t.Fatalf("EnsureTopic failed: %v", err)
	}

	reviewLog := models.FSRSReviewLog{
		ID:              "",
		TopicID:         topicID,
		ActivityType:    "flashcard",
		ReferenceID:     "card-789",
		ReviewedAt:      1234567890,
		Rating:          1,
		ScheduledDays:   1,
		StateBeforeJSON: `{}`,
		StateAfterJSON:  `{}`,
	}

	err := InsertFSRSReviewLog(reviewLog)
	if err == nil {
		t.Fatalf("expected error for empty ID, got success")
	}
	if !strings.Contains(err.Error(), "review log id is required") {
		t.Fatalf("expected 'review log id is required' error, got: %v", err)
	}
}

func TestInsertFSRSReviewLogRejectsEmptyActivityType(t *testing.T) {
	initDBForTest(t, false, 0)

	topicID := "test-activity-topic"
	if err := EnsureTopic(topicID, "Activity Test Topic"); err != nil {
		t.Fatalf("EnsureTopic failed: %v", err)
	}

	reviewLog := models.FSRSReviewLog{
		ID:              "log-activity",
		TopicID:         topicID,
		ActivityType:    "",
		ReferenceID:     "card-999",
		ReviewedAt:      1234567890,
		Rating:          1,
		ScheduledDays:   1,
		StateBeforeJSON: `{}`,
		StateAfterJSON:  `{}`,
	}

	err := InsertFSRSReviewLog(reviewLog)
	if err == nil {
		t.Fatalf("expected error for empty activity type, got success")
	}
	if !strings.Contains(err.Error(), "activity type is required") {
		t.Fatalf("expected 'activity type is required' error, got: %v", err)
	}
}

func TestInsertFSRSReviewLogRejectsEmptyReferenceID(t *testing.T) {
	initDBForTest(t, false, 0)

	topicID := "test-ref-id-topic"
	if err := EnsureTopic(topicID, "Ref ID Test Topic"); err != nil {
		t.Fatalf("EnsureTopic failed: %v", err)
	}

	reviewLog := models.FSRSReviewLog{
		ID:              "log-ref",
		TopicID:         topicID,
		ActivityType:    "flashcard",
		ReferenceID:     "",
		ReviewedAt:      1234567890,
		Rating:          1,
		ScheduledDays:   1,
		StateBeforeJSON: `{}`,
		StateAfterJSON:  `{}`,
	}

	err := InsertFSRSReviewLog(reviewLog)
	if err == nil {
		t.Fatalf("expected error for empty reference id, got success")
	}
	if !strings.Contains(err.Error(), "reference id is required") {
		t.Fatalf("expected 'reference id is required' error, got: %v", err)
	}
}

func TestInsertFSRSReviewLogRejectsInvalidReviewedAt(t *testing.T) {
	initDBForTest(t, false, 0)

	topicID := "test-reviewed-at-topic"
	if err := EnsureTopic(topicID, "Reviewed At Test Topic"); err != nil {
		t.Fatalf("EnsureTopic failed: %v", err)
	}

	tests := []struct {
		name       string
		reviewedAt int64
	}{
		{"zero", 0},
		{"negative", -1000},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reviewLog := models.FSRSReviewLog{
				ID:              "log-" + tt.name,
				TopicID:         topicID,
				ActivityType:    "flashcard",
				ReferenceID:     "card-" + tt.name,
				ReviewedAt:      tt.reviewedAt,
				Rating:          1,
				ScheduledDays:   1,
				StateBeforeJSON: `{}`,
				StateAfterJSON:  `{}`,
			}

			err := InsertFSRSReviewLog(reviewLog)
			if err == nil {
				t.Fatalf("expected error for reviewed_at=%d, got success", tt.reviewedAt)
			}
			if !strings.Contains(err.Error(), "reviewed at is required") {
				t.Fatalf("expected reviewed at validation error, got: %v", err)
			}

			assertCountEquals(t, `SELECT COUNT(*) FROM fsrs_review_log WHERE id = ?`, "log-"+tt.name, 0)
		})
	}
}

func TestInsertFSRSReviewLogRejectsEmptyStateJSON(t *testing.T) {
	initDBForTest(t, false, 0)

	topicID := "test-json-topic"
	if err := EnsureTopic(topicID, "JSON Test Topic"); err != nil {
		t.Fatalf("EnsureTopic failed: %v", err)
	}

	tests := []struct {
		name              string
		stateBeforeJSON   string
		stateAfterJSON    string
		shouldFail        bool
		expectedErrorPart string
	}{
		{"both_empty", "", "", true, "review state json values are required"},
		{"before_empty", "", `{}`, true, "review state json values are required"},
		{"after_empty", `{}`, "", true, "review state json values are required"},
		{"both_valid", `{"x":1}`, `{"x":2}`, false, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reviewLog := models.FSRSReviewLog{
				ID:              "log-" + tt.name,
				TopicID:         topicID,
				ActivityType:    "flashcard",
				ReferenceID:     "card-" + tt.name,
				ReviewedAt:      1234567890,
				Rating:          1,
				ScheduledDays:   1,
				StateBeforeJSON: tt.stateBeforeJSON,
				StateAfterJSON:  tt.stateAfterJSON,
			}

			err := InsertFSRSReviewLog(reviewLog)
			if tt.shouldFail {
				if err == nil {
					t.Fatalf("expected error for %s, got success", tt.name)
				}
				if !strings.Contains(err.Error(), tt.expectedErrorPart) {
					t.Fatalf("expected error containing %q, got: %v", tt.expectedErrorPart, err)
				}
				assertCountEquals(t, `SELECT COUNT(*) FROM fsrs_review_log WHERE id = ?`, "log-"+tt.name, 0)
			} else {
				if err != nil {
					t.Fatalf("expected success for %s, got error: %v", tt.name, err)
				}
				assertCountEquals(t, `SELECT COUNT(*) FROM fsrs_review_log WHERE id = ?`, "log-"+tt.name, 1)
			}
		})
	}
}

func TestInsertFSRSReviewLogRejectsNegativeScheduledDays(t *testing.T) {
	initDBForTest(t, false, 0)

	topicID := "test-scheduled-days-topic"
	if err := EnsureTopic(topicID, "Scheduled Days Test Topic"); err != nil {
		t.Fatalf("EnsureTopic failed: %v", err)
	}

	reviewLog := models.FSRSReviewLog{
		ID:              "log-neg-scheduled",
		TopicID:         topicID,
		ActivityType:    "flashcard",
		ReferenceID:     "card-scheduled",
		ReviewedAt:      1234567890,
		Rating:          1,
		ScheduledDays:   -5,
		StateBeforeJSON: `{}`,
		StateAfterJSON:  `{}`,
	}

	err := InsertFSRSReviewLog(reviewLog)
	if err == nil {
		t.Fatalf("expected error for negative scheduled_days, got success")
	}
	if !strings.Contains(err.Error(), "scheduled days must be non-negative") {
		t.Fatalf("expected 'scheduled days must be non-negative' error, got: %v", err)
	}

	assertCountEquals(t, `SELECT COUNT(*) FROM fsrs_review_log WHERE id = ?`, "log-neg-scheduled", 0)
}
