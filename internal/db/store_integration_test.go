package db

import (
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
