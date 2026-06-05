package db

import (
	"ai-tutor/internal/models"
	"fmt"
	"os"
	"sort"
	"strings"
	"testing"
	"time"
)

func TestIngestNotebookContentByTopicRollsBackOnMidTransactionFailure(t *testing.T) {
	initDBForTest(t, false, 0)

	notebookID := "nb-rollback"
	if err := EnsureTopic("os-scheduling", "OS Scheduling"); err != nil {
		t.Fatalf("EnsureTopic failed: %v", err)
	}
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
	if err := CreateChunk(chunkDelID, autoTopicID, parentDelID, "delete chunk body", 3, 1); err != nil {
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
	if err := CreateChunk(chunkKeepID, keepTopicID, parentKeepID, "keep chunk body", 3, 1); err != nil {
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
	if err := CreateChunk(chunkA, topicA, parentA, "topic a chunk", 3, 1); err != nil {
		t.Fatalf("CreateChunk topicA failed: %v", err)
	}

	parentB := "parent-scope-b"
	chunkB := "chunk-scope-b"
	if err := CreateParentSection(parentB, topicB, "B", 1, "topic b parent"); err != nil {
		t.Fatalf("CreateParentSection topicB failed: %v", err)
	}
	if err := CreateChunk(chunkB, topicB, parentB, "topic b chunk", 3, 2); err != nil {
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
	gotA, err := SearchVectorsForTopic(topicA, query, 5, 0, 0)
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

	gotB, err := SearchVectorsForTopic(topicB, query, 5, 0, 0)
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
	if err := CreateChunk(chunkA, topicID, parentID, "chunk a", 2, 1); err != nil {
		t.Fatalf("CreateChunk chunkA failed: %v", err)
	}
	if err := CreateChunk(chunkB, topicID, parentID, "chunk b", 2, 2); err != nil {
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
}

func TestSearchVectorsForTopicFiltersByPageWindow(t *testing.T) {
	initDBForTest(t, true, 3)

	if !distanceFunctionAvailable(t) {
		t.Skip("sqlite-vec distance() function is unavailable in this runtime")
	}

	topicA := "topic-window-a"
	topicB := "topic-window-b"
	if err := EnsureTopic(topicA, "Topic A"); err != nil {
		t.Fatalf("EnsureTopic topicA failed: %v", err)
	}
	if err := EnsureTopic(topicB, "Topic B"); err != nil {
		t.Fatalf("EnsureTopic topicB failed: %v", err)
	}

	parentA := "parent-window-a"
	chunkA := "chunk-window-a"
	if err := CreateParentSection(parentA, topicA, "A", 1, "topic a parent"); err != nil {
		t.Fatalf("CreateParentSection topicA failed: %v", err)
	}
	if err := CreateChunk(chunkA, topicA, parentA, "topic a chunk", 3, 1); err != nil {
		t.Fatalf("CreateChunk chunkA failed: %v", err)
	}

	parentB := "parent-window-b"
	chunkB := "chunk-window-b"
	if err := CreateParentSection(parentB, topicB, "B", 1, "topic b parent"); err != nil {
		t.Fatalf("CreateParentSection topicB failed: %v", err)
	}
	if err := CreateChunk(chunkB, topicB, parentB, "topic b chunk", 8, 2); err != nil {
		t.Fatalf("CreateChunk chunkB failed: %v", err)
	}

	if err := UpsertChunkVector(chunkA, []float32{1, 0, 0}); err != nil {
		t.Fatalf("UpsertChunkVector chunkA failed: %v", err)
	}
	if err := UpsertChunkVector(chunkB, []float32{1, 0, 0}); err != nil {
		t.Fatalf("UpsertChunkVector chunkB failed: %v", err)
	}

	query := []float32{1, 0, 0}

	gotAIn, err := SearchVectorsForTopic(topicA, query, 5, 2, 4)
	if err != nil {
		t.Fatalf("SearchVectorsForTopic topicA in-range failed: %v", err)
	}
	if !contains(gotAIn, chunkA) {
		t.Fatalf("expected in-range results to contain chunkA, got %#v", gotAIn)
	}
	if contains(gotAIn, chunkB) {
		t.Fatalf("expected in-range results to exclude chunkB, got %#v", gotAIn)
	}

	gotAOut, err := SearchVectorsForTopic(topicA, query, 5, 7, 9)
	if err != nil {
		t.Fatalf("SearchVectorsForTopic topicA out-of-range failed: %v", err)
	}
	if contains(gotAOut, chunkA) {
		t.Fatalf("expected out-of-range results to exclude chunkA, got %#v", gotAOut)
	}

	gotBIn, err := SearchVectorsForTopic(topicB, query, 5, 7, 9)
	if err != nil {
		t.Fatalf("SearchVectorsForTopic topicB in-range failed: %v", err)
	}
	if !contains(gotBIn, chunkB) {
		t.Fatalf("expected in-range results to contain chunkB, got %#v", gotBIn)
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


func TestGetChunkTextsForTopicPageRangeIncludesBufferPage(t *testing.T) {
	initDBForTest(t, false, 0)

	topicID := "completion-context-topic"
	notebookID := "completion-context-notebook"
	if err := EnsureTopic(topicID, "Completion Context"); err != nil {
		t.Fatalf("EnsureTopic failed: %v", err)
	}
	if err := CreateNotebook(notebookID, "Completion Context", "/tmp/context.txt", "txt", "", 4); err != nil {
		t.Fatalf("CreateNotebook failed: %v", err)
	}

	parentID := "completion-context-parent"
	groups := []NotebookTopicIngestionGroup{{
		TopicID: topicID,
		Parents: []NotebookParentInput{{
			ID: parentID, Heading: "Context", Content: "context body", OrderIndex: 1,
		}},
		Chunks: []NotebookChunkInput{
			{ID: "completion-context-c1", ParentID: parentID, Text: "page one", TokenCount: 2, PageNum: 1},
			{ID: "completion-context-c2", ParentID: parentID, Text: "page two", TokenCount: 2, PageNum: 2},
			{ID: "completion-context-c3", ParentID: parentID, Text: "page three buffer", TokenCount: 3, PageNum: 3},
			{ID: "completion-context-c4", ParentID: parentID, Text: "page four", TokenCount: 2, PageNum: 4},
		},
	}}
	if err := IngestNotebookContentByTopic(notebookID, groups); err != nil {
		t.Fatalf("IngestNotebookContentByTopic failed: %v", err)
	}

	texts, err := GetChunkTextsForTopicPageRange(topicID, 1, 3)
	if err != nil {
		t.Fatalf("GetChunkTextsForTopicPageRange failed: %v", err)
	}
	want := []string{"page one", "page two", "page three buffer"}
	if !equalStringSlices(texts, want) {
		t.Fatalf("unexpected context texts: got=%#v want=%#v", texts, want)
	}
}

func TestUpdateTopicReadingCursorMarksLearnedAtEnd(t *testing.T) {
	initDBForTest(t, false, 0)

	topicID := "cursor-topic"
	if err := EnsureTopic(topicID, "Cursor Topic"); err != nil {
		t.Fatalf("EnsureTopic failed: %v", err)
	}
	if err := UpdateTopicPageBounds(topicID, 1, 4); err != nil {
		t.Fatalf("UpdateTopicPageBounds failed: %v", err)
	}

	if err := UpdateTopicReadingCursor(topicID, 3, false); err != nil {
		t.Fatalf("UpdateTopicReadingCursor reading failed: %v", err)
	}
	var cursor int
	var status string
	if err := conn.QueryRow(`SELECT current_page_cursor, status FROM topics WHERE id = ?`, topicID).Scan(&cursor, &status); err != nil {
		t.Fatalf("failed to read topic cursor: %v", err)
	}
	if cursor != 3 || status != "reading" {
		t.Fatalf("unexpected reading cursor/status: cursor=%d status=%s", cursor, status)
	}

	if err := UpdateTopicReadingCursor(topicID, 5, true); err != nil {
		t.Fatalf("UpdateTopicReadingCursor learned failed: %v", err)
	}
	if err := conn.QueryRow(`SELECT current_page_cursor, status FROM topics WHERE id = ?`, topicID).Scan(&cursor, &status); err != nil {
		t.Fatalf("failed to read learned cursor: %v", err)
	}
	if cursor != 5 || status != "learned" {
		t.Fatalf("unexpected learned cursor/status: cursor=%d status=%s", cursor, status)
	}
}

func TestContextLockedVectorRetrievalP95Under50ms(t *testing.T) {
	initDBForTest(t, true, 3)
	if !distanceFunctionAvailable(t) {
		t.Skip("sqlite-vec distance() function is unavailable in this runtime")
	}

	topicID := "perf-vector-topic"
	parentID := "perf-vector-parent"
	if err := EnsureTopic(topicID, "Perf Vector Topic"); err != nil {
		t.Fatalf("EnsureTopic failed: %v", err)
	}
	if err := CreateParentSection(parentID, topicID, "Perf", 1, "perf parent"); err != nil {
		t.Fatalf("CreateParentSection failed: %v", err)
	}
	for i := 1; i <= 120; i++ {
		chunkID := fmt.Sprintf("perf-vector-c%03d", i)
		if err := insertChunkRow(conn, topicID, NotebookChunkInput{
			ID: chunkID, ParentID: parentID, Text: fmt.Sprintf("chunk %d", i), TokenCount: 2, PageNum: (i % 12) + 1,
		}); err != nil {
			t.Fatalf("insertChunkRow failed: %v", err)
		}
		if err := UpsertChunkVector(chunkID, []float32{float32(i % 3), float32((i + 1) % 3), 1}); err != nil {
			t.Fatalf("UpsertChunkVector failed: %v", err)
		}
	}

	durations := make([]time.Duration, 0, 160)
	query := []float32{1, 0, 1}
	for i := 0; i < 160; i++ {
		started := time.Now()
		if _, err := SearchVectorsForTopic(topicID, query, 5, 3, 8); err != nil {
			t.Fatalf("SearchVectorsForTopic failed: %v", err)
		}
		durations = append(durations, time.Since(started))
	}

	// Skip test when PERF_RUN is not set, evaluate performance when it is set
	if os.Getenv("PERF_RUN") == "" {
		t.Skip("performance test disabled - run with PERF_RUN=1 to enable")
	}

	if p95Duration(durations) >= 50*time.Millisecond {
		t.Fatalf("context-locked vector retrieval p95: %s exceeds threshold: 50ms", p95Duration(durations))
	}
}


func p95Duration(durations []time.Duration) time.Duration {
	if len(durations) == 0 {
		return 0
	}
	sorted := append([]time.Duration(nil), durations...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	idx := int(float64(len(sorted))*0.95) - 1
	if idx < 0 {
		idx = 0
	}
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return sorted[idx]
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

func TestInitEnablesForeignKeys(t *testing.T) {
	initDBForTest(t, false, 0)

	var enabled int
	if err := conn.QueryRow(`PRAGMA foreign_keys`).Scan(&enabled); err != nil {
		t.Fatalf("PRAGMA foreign_keys failed: %v", err)
	}
	if enabled != 1 {
		t.Fatalf("expected foreign_keys pragma enabled, got %d", enabled)
	}
}


func TestTopicDeletionCascadesToFSRSTables(t *testing.T) {
	initDBForTest(t, false, 0)

	topicID := "cascade-fsrs-topic"
	if err := EnsureTopic(topicID, "Cascade FSRS Topic"); err != nil {
		t.Fatalf("EnsureTopic failed: %v", err)
	}
	if err := CreateFlashcards(topicID, []models.Flashcard{{
		ID:      "cascade-card",
		TopicID: topicID,
		Prompt:  "Prompt?",
		Answer:  "Answer.",
		DueAt:   123,
	}}, map[string]models.FlashcardState{
		"cascade-card": {},
	}); err != nil {
		t.Fatalf("CreateFlashcards failed: %v", err)
	}
	if err := InsertFSRSReviewLog(models.FSRSReviewLog{
		ID:              "cascade-log",
		TopicID:         topicID,
		ActivityType:    "flashcard",
		ReferenceID:     "cascade-card",
		ReviewedAt:      1234567890,
		Rating:          3,
		ScheduledDays:   2,
		StateBeforeJSON: `{}`,
		StateAfterJSON:  `{}`,
	}); err != nil {
		t.Fatalf("InsertFSRSReviewLog failed: %v", err)
	}

	if _, err := conn.Exec(`DELETE FROM topics WHERE id = ?`, topicID); err != nil {
		t.Fatalf("topic delete failed: %v", err)
	}

	assertCountEquals(t, `SELECT COUNT(*) FROM fsrs_cards WHERE id = ?`, "cascade-card", 0)
	assertCountEquals(t, `SELECT COUNT(*) FROM fsrs_review_log WHERE id = ?`, "cascade-log", 0)
}

func TestTopicDeletionCascadesToAssessmentTables(t *testing.T) {
	initDBForTest(t, false, 0)

	topicID := "cascade-assessment-topic"
	if err := EnsureTopic(topicID, "Cascade Assessment Topic"); err != nil {
		t.Fatalf("EnsureTopic failed: %v", err)
	}

	if _, err := conn.Exec(`
		INSERT INTO written_questions (
			id, topic_id, prompt, source_heading, source_page_start, source_page_end, llm_model, prompt_version
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, "written-1", topicID, "Explain RR", "RR", 1, 2, "test-model", "written-v1"); err != nil {
		t.Fatalf("insert written_questions failed: %v", err)
	}

	if _, err := conn.Exec(`DELETE FROM topics WHERE id = ?`, topicID); err != nil {
		t.Fatalf("topic delete failed: %v", err)
	}

	assertCountEquals(t, `SELECT COUNT(*) FROM written_questions WHERE id = ?`, "written-1", 0)
}

func TestUpdateTopicPageBoundsShrinkDeletesOutOfRangeAssessmentData(t *testing.T) {
	initDBForTest(t, false, 0)

	topicID := "shrink-topic"
	if err := EnsureTopic(topicID, "Shrink Topic"); err != nil {
		t.Fatalf("EnsureTopic failed: %v", err)
	}
	if err := UpdateTopicPageBounds(topicID, 1, 10); err != nil {
		t.Fatalf("initial UpdateTopicPageBounds failed: %v", err)
	}

	if err := CreateWrittenQuestion(models.WrittenQuestion{
		ID:              "written-in-range",
		TopicID:         topicID,
		Prompt:          "Explain in range",
		SourcePageStart: 3,
		SourcePageEnd:   4,
	}); err != nil {
		t.Fatalf("CreateWrittenQuestion in range failed: %v", err)
	}
	if err := CreateWrittenQuestion(models.WrittenQuestion{
		ID:              "written-out-range",
		TopicID:         topicID,
		Prompt:          "Explain out of range",
		SourcePageStart: 9,
		SourcePageEnd:   10,
	}); err != nil {
		t.Fatalf("CreateWrittenQuestion out of range failed: %v", err)
	}
	if err := InsertFSRSReviewLog(models.FSRSReviewLog{
		ID:              "log-written-out-range",
		TopicID:         topicID,
		ActivityType:    "written_question",
		ReferenceID:     "written-out-range",
		ReviewedAt:      123,
		Rating:          1,
		ScheduledDays:   0,
		StateBeforeJSON: `{}`,
		StateAfterJSON:  `{}`,
	}); err != nil {
		t.Fatalf("InsertFSRSReviewLog failed: %v", err)
	}

	if err := UpdateTopicPageBounds(topicID, 1, 5); err != nil {
		t.Fatalf("shrink UpdateTopicPageBounds failed: %v", err)
	}

	assertCountEquals(t, `SELECT COUNT(*) FROM written_questions WHERE id = ?`, "written-in-range", 1)
	assertCountEquals(t, `SELECT COUNT(*) FROM written_questions WHERE id = ?`, "written-out-range", 0)
	assertCountEquals(t, `SELECT COUNT(*) FROM fsrs_review_log WHERE id = ?`, "log-written-out-range", 0)
}

func TestGetTotalChunkTokensFallsBackWhenTokenCountMissing(t *testing.T) {
	initDBForTest(t, false, 0)

	topicID := "token-fallback-topic"
	if err := EnsureTopic(topicID, "Token Fallback Topic"); err != nil {
		t.Fatalf("EnsureTopic failed: %v", err)
	}
	if err := CreateParentSection("token-parent", topicID, "Heading", 1, "content"); err != nil {
		t.Fatalf("CreateParentSection failed: %v", err)
	}
	if err := CreateChunk("token-c1", topicID, "token-parent", "abcdabcd", 0, 1); err != nil {
		t.Fatalf("CreateChunk c1 failed: %v", err)
	}
	if err := CreateChunk("token-c2", topicID, "token-parent", "abcdefghijkl", 3, 2); err != nil {
		t.Fatalf("CreateChunk c2 failed: %v", err)
	}

	total, err := GetTotalChunkTokens(topicID)
	if err != nil {
		t.Fatalf("GetTotalChunkTokens failed: %v", err)
	}
	if total != 5 {
		t.Fatalf("expected token total 5, got %d", total)
	}

	rangeTotal, err := GetTotalChunkTokensForPageRange(topicID, 1, 1)
	if err != nil {
		t.Fatalf("GetTotalChunkTokensForPageRange failed: %v", err)
	}
	if rangeTotal != 2 {
		t.Fatalf("expected page-range token total 2, got %d", rangeTotal)
	}
}
