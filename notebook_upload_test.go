package main

import (
	"fmt"
	"os"
	"testing"

	"ai-tutor/internal/models"
	"ai-tutor/internal/notebook"
)

// ============================================================================
// NOTEBOOK/TOPIC TESTS
// ============================================================================

func TestGetAvailableTopicsFromDB(t *testing.T) {
	initTestDB(t)
	app := &App{repo: testRepo}

	topics := app.GetAvailableTopics()
	if len(topics) == 0 {
		t.Fatalf("expected at least one topic")
	}

	found := false
	for _, topic := range topics {
		if topic["id"] == "os-scheduling" {
			found = true
			break
		}
	}

	if !found {
		t.Fatalf("expected seeded topic os-scheduling in available topics: %#v", topics)
	}
}

func TestGetNotebookTopicTreeEmptyReturnsArray(t *testing.T) {
	initCleanTestDB(t)
	app := &App{repo: testRepo}

	tree, err := app.GetNotebookTopicTree()
	if err != nil {
		t.Fatalf("GetNotebookTopicTree failed: %v", err)
	}
	if tree == nil {
		t.Fatalf("expected empty array, got nil")
	}
	if len(tree) != 0 {
		t.Fatalf("expected no notebooks in tree, got %#v", tree)
	}
}

func TestGetNotebookTopicTreeReturnsNestedTopics(t *testing.T) {
	initCleanTestDB(t)
	app := &App{repo: testRepo}

	notebookA := "nb-tree-a"
	notebookB := "nb-tree-b"
	if err := testRepo.CreateNotebook(notebookA, "Physics", "/tmp/physics.txt", "txt", "", 1); err != nil {
		t.Fatalf("CreateNotebook notebookA failed: %v", err)
	}
	if err := testRepo.CreateNotebook(notebookB, "History", "/tmp/history.txt", "txt", "", 1); err != nil {
		t.Fatalf("CreateNotebook notebookB failed: %v", err)
	}

	for _, topic := range []struct {
		id    string
		title string
	}{
		{id: "topic-thermo", title: "Thermodynamics"},
		{id: "topic-newton", title: "Newton's Laws"},
		{id: "topic-renaissance", title: "The Renaissance"},
	} {
		if err := testRepo.EnsureTopic(topic.id, topic.title); err != nil {
			t.Fatalf("EnsureTopic %s failed: %v", topic.id, err)
		}
	}

	chunkThermo := "chunk-thermo"
	chunkNewton := "chunk-newton"
	chunkRenaissance := "chunk-renaissance"
	if err := testRepo.CreateChunk(chunkThermo, "topic-thermo", "thermo chunk", 2, 1); err != nil {
		t.Fatalf("CreateChunk thermo failed: %v", err)
	}
	if err := testRepo.CreateChunk(chunkNewton, "topic-newton", "newton chunk", 2, 2); err != nil {
		t.Fatalf("CreateChunk newton failed: %v", err)
	}
	if err := testRepo.CreateChunk(chunkRenaissance, "topic-renaissance", "renaissance chunk", 2, 3); err != nil {
		t.Fatalf("CreateChunk renaissance failed: %v", err)
	}

	if err := testRepo.LinkChunksToNotebook(notebookA, []string{chunkThermo, chunkNewton}); err != nil {
		t.Fatalf("LinkChunksToNotebook notebookA failed: %v", err)
	}
	if err := testRepo.LinkChunksToNotebook(notebookB, []string{chunkRenaissance}); err != nil {
		t.Fatalf("LinkChunksToNotebook notebookB failed: %v", err)
	}

	tree, err := app.GetNotebookTopicTree()
	if err != nil {
		t.Fatalf("GetNotebookTopicTree failed: %v", err)
	}
	if len(tree) != 2 {
		t.Fatalf("expected 2 notebooks, got %#v", tree)
	}

	var physicsTopics []string
	var historyTopics []string
	for _, node := range tree {
		switch node.NotebookID {
		case notebookA:
			for _, topic := range node.Topics {
				physicsTopics = append(physicsTopics, topic.Title)
			}
		case notebookB:
			for _, topic := range node.Topics {
				historyTopics = append(historyTopics, topic.Title)
			}
		}
	}

	if len(physicsTopics) != 2 || physicsTopics[0] != "Newton's Laws" || physicsTopics[1] != "Thermodynamics" {
		t.Fatalf("unexpected physics topics: %#v", physicsTopics)
	}
	if len(historyTopics) != 1 || historyTopics[0] != "The Renaissance" {
		t.Fatalf("unexpected history topics: %#v", historyTopics)
	}
}

// ============================================================================
// NOTEBOOK UPLOAD TESTS
// ============================================================================

func TestDraftNotebookSyllabus_FallbackCreatesEditableChapter(t *testing.T) {
	initTestDB(t)
	uploadDir := t.TempDir()
	service := notebook.NewService(uploadDir)
	app := &App{repo: testRepo, notebookService: service}

	uploadResult, err := service.SaveUploadedFile([]byte("Alpha beta gamma"), "draft.txt")
	if err != nil {
		t.Fatalf("SaveUploadedFile failed: %v", err)
	}

	doc, err := service.ExtractDocument(uploadResult.FilePath, uploadResult.FileType)
	if err != nil {
		t.Fatalf("ExtractDocument failed: %v", err)
	}

	if err := testRepo.CreateNotebook(uploadResult.ID, uploadResult.FileName, uploadResult.FilePath, uploadResult.FileType, "", doc.PageCount); err != nil {
		t.Fatalf("CreateNotebook failed: %v", err)
	}

	resp := app.DraftNotebookSyllabus(uploadResult.ID, false)
	if _, hasErr := resp["error"]; hasErr {
		t.Fatalf("expected successful draft response, got error: %v", resp["error"])
	}

	chapters, ok := resp["chapters"].([]models.SyllabusChapterDraft)
	if !ok {
		t.Fatalf("expected typed chapters slice, got %#v", resp["chapters"])
	}
	if len(chapters) == 0 {
		t.Fatalf("expected at least one chapter in draft")
	}

	draftJSON, err := testRepo.GetNotebookSyllabusDraft(uploadResult.ID)
	if err != nil {
		t.Fatalf("GetNotebookSyllabusDraft failed: %v", err)
	}
	if draftJSON == "" {
		t.Fatalf("expected draft to be persisted to DB, but got empty string")
	}

	resp2 := app.DraftNotebookSyllabus(uploadResult.ID, false)
	if _, hasErr := resp2["error"]; hasErr {
		t.Fatalf("expected successful draft response on reload, got error: %v", resp2["error"])
	}

	chapters2, ok := resp2["chapters"].([]models.SyllabusChapterDraft)
	if !ok {
		t.Fatalf("expected typed chapters slice on reload, got %#v", resp2["chapters"])
	}
	if len(chapters2) != len(chapters) {
		t.Fatalf("expected same number of chapters on reload, got %d vs %d", len(chapters2), len(chapters))
	}

	resp3 := app.DraftNotebookSyllabus(uploadResult.ID, true)
	if _, hasErr := resp3["error"]; hasErr {
		t.Fatalf("expected successful draft response on regenerate, got error: %v", resp3["error"])
	}
	if chapters[0].StartPage < 1 || chapters[0].EndPage < chapters[0].StartPage {
		t.Fatalf("invalid chapter page bounds: %#v", chapters[0])
	}
}

func TestConfirmNotebookSyllabus_PersistsBoundsAndPageAwareChunks(t *testing.T) {
	initTestDB(t)
	uploadDir := t.TempDir()
	service := notebook.NewService(uploadDir)
	app := &App{repo: testRepo, notebookService: service}

	uploadResult, err := service.SaveUploadedFile([]byte("# Intro\n\nAlpha beta gamma\n\n## Details\n\nDelta epsilon zeta"), "confirm.md")
	if err != nil {
		t.Fatalf("SaveUploadedFile failed: %v", err)
	}

	doc, err := service.ExtractDocument(uploadResult.FilePath, uploadResult.FileType)
	if err != nil {
		t.Fatalf("ExtractDocument failed: %v", err)
	}

	if err := testRepo.CreateNotebook(uploadResult.ID, uploadResult.FileName, uploadResult.FilePath, uploadResult.FileType, "", doc.PageCount); err != nil {
		t.Fatalf("CreateNotebook failed: %v", err)
	}

	resp := app.ConfirmNotebookSyllabus(uploadResult.ID, []models.SyllabusChapterDraft{{
		Title:     "Confirmed Chapter",
		StartPage: 1,
		EndPage:   doc.PageCount,
	}})
	if _, hasErr := resp["error"]; hasErr {
		t.Fatalf("expected confirm success, got error: %v", resp["error"])
	}

	topicIDs, ok := resp["topic_ids"].([]string)
	if !ok || len(topicIDs) == 0 {
		t.Fatalf("expected topic ids, got %#v", resp["topic_ids"])
	}

	startPage, endPage, err := testRepo.GetTopicPageBounds(topicIDs[0])
	if err != nil {
		t.Fatalf("GetTopicPageBounds failed: %v", err)
	}
	if startPage != 1 || endPage != doc.PageCount {
		t.Fatalf("unexpected persisted bounds: got [%d,%d] want [1,%d]", startPage, endPage, doc.PageCount)
	}

	bundle, err := testRepo.GetReaderTopicBundle(topicIDs[0], uploadResult.ID)
	if err != nil {
		t.Fatalf("GetReaderTopicBundle failed: %v", err)
	}
	if len(bundle.Sections) == 0 {
		t.Fatalf("expected reader sections after confirm ingestion")
	}
	if bundle.Sections[0].PageNum <= 0 {
		t.Fatalf("expected page-aware section mapping, got page_num=%d", bundle.Sections[0].PageNum)
	}
}

func TestConfirmNotebookSyllabus_AutoActivatesIfLessThansFourActive(t *testing.T) {
	initTestDB(t)
	uploadDir := t.TempDir()
	service := notebook.NewService(uploadDir)
	app := &App{repo: testRepo, notebookService: service}

	profileID := "test-profile-auto"
	err := testRepo.CreateProfile(models.StudyProfile{ID: profileID, Name: "Test Profile Auto", DeadlineAt: 0})
	if err != nil {
		t.Fatalf("CreateProfile failed: %v", err)
	}
	err = testRepo.UpdateUserSettings(models.UserSettings{ActiveProfileID: profileID})
	if err != nil {
		t.Fatalf("UpdateUserSettings failed: %v", err)
	}

	uploadResult, err := service.SaveUploadedFile([]byte("# Intro\n\nSome book content here"), "book1.md")
	if err != nil {
		t.Fatalf("SaveUploadedFile failed: %v", err)
	}

	doc, err := service.ExtractDocument(uploadResult.FilePath, uploadResult.FileType)
	if err != nil {
		t.Fatalf("ExtractDocument failed: %v", err)
	}

	if err := testRepo.CreateNotebook(uploadResult.ID, uploadResult.FileName, uploadResult.FilePath, uploadResult.FileType, "", doc.PageCount); err != nil {
		t.Fatalf("CreateNotebook failed: %v", err)
	}
	if err := testRepo.AssignNotebookToProfile(uploadResult.ID, profileID); err != nil {
		t.Fatalf("AssignNotebookToProfile failed: %v", err)
	}

	resp := app.ConfirmNotebookSyllabus(uploadResult.ID, []models.SyllabusChapterDraft{{
		Title:     "Chapter 1",
		StartPage: 1,
		EndPage:   doc.PageCount,
	}})
	if _, hasErr := resp["error"]; hasErr {
		t.Fatalf("expected confirm success, got error: %v", resp["error"])
	}

	nb, err := testRepo.GetNotebookByID(uploadResult.ID)
	if err != nil {
		t.Fatalf("GetNotebookByID failed: %v", err)
	}
	if nb.StudyStatus != "active" {
		t.Fatalf("expected study status to be auto-activated to 'active', got %q", nb.StudyStatus)
	}
}

func TestConfirmNotebookSyllabus_DoesNotAutoActivateIfFourOrMoreActive(t *testing.T) {
	initTestDB(t)
	uploadDir := t.TempDir()
	service := notebook.NewService(uploadDir)
	app := &App{repo: testRepo, notebookService: service}

	profileID := "test-profile-limit"
	err := testRepo.CreateProfile(models.StudyProfile{ID: profileID, Name: "Test Profile Limit", DeadlineAt: 0})
	if err != nil {
		t.Fatalf("CreateProfile failed: %v", err)
	}
	err = testRepo.UpdateUserSettings(models.UserSettings{ActiveProfileID: profileID})
	if err != nil {
		t.Fatalf("UpdateUserSettings failed: %v", err)
	}

	for i := 1; i <= 4; i++ {
		id := fmt.Sprintf("nb-active-%d", i)
		err = testRepo.CreateNotebook(id, fmt.Sprintf("Active %d", i), "dummy", "md", "", 1)
		if err != nil {
			t.Fatalf("CreateNotebook failed: %v", err)
		}
		err = testRepo.AssignNotebookToProfile(id, profileID)
		if err != nil {
			t.Fatalf("AssignNotebookToProfile failed: %v", err)
		}
		err = testRepo.UpdateNotebookStudyStatus(id, "active")
		if err != nil {
			t.Fatalf("UpdateNotebookStudyStatus failed: %v", err)
		}
	}

	uploadResult, err := service.SaveUploadedFile([]byte("# Intro\n\nSome book content here"), "book5.md")
	if err != nil {
		t.Fatalf("SaveUploadedFile failed: %v", err)
	}

	doc, err := service.ExtractDocument(uploadResult.FilePath, uploadResult.FileType)
	if err != nil {
		t.Fatalf("ExtractDocument failed: %v", err)
	}

	if err := testRepo.CreateNotebook(uploadResult.ID, uploadResult.FileName, uploadResult.FilePath, uploadResult.FileType, "", doc.PageCount); err != nil {
		t.Fatalf("CreateNotebook failed: %v", err)
	}
	if err := testRepo.AssignNotebookToProfile(uploadResult.ID, profileID); err != nil {
		t.Fatalf("AssignNotebookToProfile failed: %v", err)
	}

	resp := app.ConfirmNotebookSyllabus(uploadResult.ID, []models.SyllabusChapterDraft{{
		Title:     "Chapter 1",
		StartPage: 1,
		EndPage:   doc.PageCount,
	}})
	if _, hasErr := resp["error"]; hasErr {
		t.Fatalf("expected confirm success, got error: %v", resp["error"])
	}

	nb, err := testRepo.GetNotebookByID(uploadResult.ID)
	if err != nil {
		t.Fatalf("GetNotebookByID failed: %v", err)
	}
	if nb.StudyStatus == "active" {
		t.Fatalf("expected study status to remain dormant/empty, got %q", nb.StudyStatus)
	}
}

func TestConfirmNotebookSyllabus_MetadataOnlySkipsExtraction(t *testing.T) {
	app, uploadResult, chapters := setupConfirmedChunkedNotebook(t, "confirm-metadata-only.md")

	if err := os.Remove(uploadResult.FilePath); err != nil {
		t.Fatalf("Remove source file failed: %v", err)
	}
	beforeChunks := mustNotebookChunkCount(t, uploadResult.ID)

	resp := app.ConfirmNotebookSyllabus(uploadResult.ID, chapters)
	if _, hasErr := resp["error"]; hasErr {
		t.Fatalf("expected metadata-only confirm success, got error: %v", resp["error"])
	}
	if mode, ok := resp["mode"].(string); !ok || mode != "metadata_only" {
		t.Fatalf("expected metadata_only mode, got %#v", resp["mode"])
	}
	if afterChunks := mustNotebookChunkCount(t, uploadResult.ID); afterChunks != beforeChunks {
		t.Fatalf("expected chunks unchanged, got %d want %d", afterChunks, beforeChunks)
	}
}

func TestConfirmNotebookSyllabus_TitleOnlyUpdatesTopicsAndSkipsExtraction(t *testing.T) {
	app, uploadResult, chapters := setupConfirmedChunkedNotebook(t, "confirm-title-only.md")

	if err := os.Remove(uploadResult.FilePath); err != nil {
		t.Fatalf("Remove source file failed: %v", err)
	}
	beforeChunks := mustNotebookChunkCount(t, uploadResult.ID)
	renamed := []models.SyllabusChapterDraft{
		{Title: "Renamed Intro", StartPage: chapters[0].StartPage, EndPage: chapters[0].EndPage},
		{Title: "Renamed Details", StartPage: chapters[1].StartPage, EndPage: chapters[1].EndPage},
	}

	resp := app.ConfirmNotebookSyllabus(uploadResult.ID, renamed)
	if _, hasErr := resp["error"]; hasErr {
		t.Fatalf("expected title-only confirm success, got error: %v", resp["error"])
	}
	if mode, ok := resp["mode"].(string); !ok || mode != "topic_metadata_only" {
		t.Fatalf("expected topic_metadata_only mode, got %#v", resp["mode"])
	}
	if afterChunks := mustNotebookChunkCount(t, uploadResult.ID); afterChunks != beforeChunks {
		t.Fatalf("expected chunks unchanged, got %d want %d", afterChunks, beforeChunks)
	}

	topics, err := testRepo.GetNotebookTopicsWithBounds(uploadResult.ID)
	if err != nil {
		t.Fatalf("GetNotebookTopicsWithBounds failed: %v", err)
	}
	if len(topics) != len(renamed) {
		t.Fatalf("expected %d topics, got %d", len(renamed), len(topics))
	}
	for i := range renamed {
		if topics[i].Title != renamed[i].Title {
			t.Fatalf("topic %d title mismatch: got %q want %q", i, topics[i].Title, renamed[i].Title)
		}
		if topics[i].StartPage != renamed[i].StartPage || topics[i].EndPage != renamed[i].EndPage {
			t.Fatalf("topic %d bounds changed: got [%d,%d] want [%d,%d]", i, topics[i].StartPage, topics[i].EndPage, renamed[i].StartPage, renamed[i].EndPage)
		}
	}
}

func TestConfirmNotebookSyllabus_BoundaryChangeFallsBackToFullReingest(t *testing.T) {
	app, uploadResult, _ := setupConfirmedChunkedNotebook(t, "confirm-boundary-change.md")

	resp := app.ConfirmNotebookSyllabus(uploadResult.ID, []models.SyllabusChapterDraft{{
		Title:     "Intro",
		StartPage: 1,
		EndPage:   2,
	}})
	if _, hasErr := resp["error"]; hasErr {
		t.Fatalf("expected boundary-change confirm success, got error: %v", resp["error"])
	}
	if mode, ok := resp["mode"].(string); !ok || mode != "full_reingest" {
		t.Fatalf("expected full_reingest mode, got %#v", resp["mode"])
	}

	topicIDs, ok := resp["topic_ids"].([]string)
	if !ok || len(topicIDs) != 1 {
		t.Fatalf("expected one topic id after merged bounds, got %#v", resp["topic_ids"])
	}
	startPage, endPage, err := testRepo.GetTopicPageBounds(topicIDs[0])
	if err != nil {
		t.Fatalf("GetTopicPageBounds failed: %v", err)
	}
	if startPage != 1 || endPage != 2 {
		t.Fatalf("unexpected persisted bounds: got [%d,%d] want [1,2]", startPage, endPage)
	}
}

func TestConfirmNotebookSyllabus_MixedTitleAndBoundaryChangeFullReingests(t *testing.T) {
	app, uploadResult, _ := setupConfirmedChunkedNotebook(t, "confirm-mixed-change.md")

	resp := app.ConfirmNotebookSyllabus(uploadResult.ID, []models.SyllabusChapterDraft{{
		Title:     "Renamed Combined Chapter",
		StartPage: 1,
		EndPage:   2,
	}})
	if _, hasErr := resp["error"]; hasErr {
		t.Fatalf("expected mixed-change confirm success, got error: %v", resp["error"])
	}
	if mode, ok := resp["mode"].(string); !ok || mode != "full_reingest" {
		t.Fatalf("expected full_reingest mode, got %#v", resp["mode"])
	}

	topicIDs, ok := resp["topic_ids"].([]string)
	if !ok || len(topicIDs) != 1 {
		t.Fatalf("expected one topic id after mixed change, got %#v", resp["topic_ids"])
	}
	topics, err := testRepo.GetNotebookTopicsWithBounds(uploadResult.ID)
	if err != nil {
		t.Fatalf("GetNotebookTopicsWithBounds failed: %v", err)
	}
	found := false
	for _, topic := range topics {
		if topic.TopicID == topicIDs[0] {
			found = true
			if topic.Title != "Renamed Combined Chapter" {
				t.Fatalf("expected renamed topic title, got %q", topic.Title)
			}
			if topic.StartPage != 1 || topic.EndPage != 2 {
				t.Fatalf("unexpected renamed topic bounds: got [%d,%d] want [1,2]", topic.StartPage, topic.EndPage)
			}
		}
	}
	if !found {
		t.Fatalf("expected new mixed-change topic %q in notebook topic bounds", topicIDs[0])
	}
}

func setupConfirmedChunkedNotebook(t *testing.T, fileName string) (*App, notebook.UploadResult, []models.SyllabusChapterDraft) {
	t.Helper()
	initTestDB(t)
	uploadDir := t.TempDir()
	service := notebook.NewService(uploadDir)
	app := &App{repo: testRepo, notebookService: service}

	content := []byte("# Intro\n\nAlpha beta gamma\n\n## Details\n\nDelta epsilon zeta")
	uploadResult, err := service.SaveUploadedFile(content, fileName)
	if err != nil {
		t.Fatalf("SaveUploadedFile failed: %v", err)
	}
	doc, err := service.ExtractDocument(uploadResult.FilePath, uploadResult.FileType)
	if err != nil {
		t.Fatalf("ExtractDocument failed: %v", err)
	}
	if doc.PageCount != 2 {
		t.Fatalf("expected two-page markdown fixture, got %d", doc.PageCount)
	}
	if err := testRepo.CreateNotebook(uploadResult.ID, uploadResult.FileName, uploadResult.FilePath, uploadResult.FileType, "", doc.PageCount); err != nil {
		t.Fatalf("CreateNotebook failed: %v", err)
	}

	chapters := []models.SyllabusChapterDraft{
		{Title: "Intro", StartPage: 1, EndPage: 1},
		{Title: "Details", StartPage: 2, EndPage: 2},
	}
	resp := app.ConfirmNotebookSyllabus(uploadResult.ID, chapters)
	if _, hasErr := resp["error"]; hasErr {
		t.Fatalf("initial ConfirmNotebookSyllabus failed: %v", resp["error"])
	}
	if status, ok := resp["status"].(string); !ok || status != "chunked" {
		t.Fatalf("expected initial chunked status, got %#v", resp["status"])
	}
	if count := mustNotebookChunkCount(t, uploadResult.ID); count == 0 {
		t.Fatalf("expected initial chunks")
	}

	return app, *uploadResult, chapters
}

func mustNotebookChunkCount(t *testing.T, notebookID string) int {
	t.Helper()
	chunks, err := testRepo.GetChunksForNotebook(notebookID)
	if err != nil {
		t.Fatalf("GetChunksForNotebook failed: %v", err)
	}
	return len(chunks)
}
