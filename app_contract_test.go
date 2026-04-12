package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"ai-tutor/internal/db"
	"ai-tutor/internal/llm"
	"ai-tutor/internal/rag"
)

func initTestDB(t *testing.T) {
	t.Helper()
	tempDB := filepath.Join(t.TempDir(), "ai-tutor-test.db")
	if err := db.Init(tempDB, ""); err != nil {
		t.Fatalf("failed to init test db: %v", err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Fatalf("failed to close test db: %v", err)
		}
	})
}

func initTestPipeline(t *testing.T) *rag.Pipeline {
	t.Helper()
	initTestDB(t)

	embedStore := rag.NewEmbeddingStore(nil)
	topicIDs, err := db.GetAllTopicIDs()
	if err != nil {
		t.Fatalf("failed to list topic IDs: %v", err)
	}

	for _, topicID := range topicIDs {
		chunks, chunksErr := db.GetChunksForTopic(topicID)
		if chunksErr != nil {
			t.Fatalf("failed to get chunks for topic %s: %v", topicID, chunksErr)
		}
		for _, chunk := range chunks {
			embedStore.AddChunk(chunk)
		}
	}

	mockLLM := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			http.NotFound(w, r)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"choices": []map[string]interface{}{
				{
					"message": map[string]string{"content": "Round Robin gives each process a fixed time slice."},
				},
			},
		})
	}))
	t.Cleanup(mockLLM.Close)

	t.Setenv("LLM_BASE_URL", mockLLM.URL)
	t.Setenv("LLM_API_KEY", "test-key")
	t.Setenv("LLM_MODEL", "test-model")

	provider := llm.NewProvider(llm.LoadConfigFromEnv())
	return rag.NewPipeline(embedStore, provider)
}

func TestAskAIResponseShape(t *testing.T) {
	app := &App{ragPipeline: initTestPipeline(t), aiReady: true}

	resp := app.AskAI("os-scheduling", "What is Round Robin scheduling?")

	if _, hasError := resp["error"]; hasError {
		t.Fatalf("expected success response, got error: %v", resp["error"])
	}

	if _, ok := resp["answer"].(string); !ok {
		t.Fatalf("expected answer string, got: %#v", resp["answer"])
	}

	switch cited := resp["cited_sections"].(type) {
	case []string:
		// valid typed response
	case []interface{}:
		for idx, item := range cited {
			if _, ok := item.(string); !ok {
				t.Fatalf("expected cited_sections[%d] to be string, got: %#v", idx, item)
			}
		}
	default:
		t.Fatalf("expected cited_sections []string or []interface{}, got: %#v", resp["cited_sections"])
	}

	if _, ok := resp["chunks_retrieved"].(int); !ok {
		t.Fatalf("expected chunks_retrieved int, got: %#v", resp["chunks_retrieved"])
	}

	if _, ok := resp["sections_used"].(int); !ok {
		t.Fatalf("expected sections_used int, got: %#v", resp["sections_used"])
	}
}

func TestAskAIInvalidTopicReturnsError(t *testing.T) {
	app := &App{ragPipeline: initTestPipeline(t), aiReady: true}

	resp := app.AskAI("missing-topic", "Any question")
	if _, ok := resp["error"].(string); !ok {
		t.Fatalf("expected error string for invalid topic, got: %#v", resp)
	}
}

func TestGetAvailableTopicsFromDB(t *testing.T) {
	initTestDB(t)
	app := &App{}

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
	initTestDB(t)
	app := &App{}

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
	initTestDB(t)
	app := &App{}

	notebookA := "nb-tree-a"
	notebookB := "nb-tree-b"
	if err := db.CreateNotebook(notebookA, "Physics", "/tmp/physics.txt", "txt", "", 1); err != nil {
		t.Fatalf("CreateNotebook notebookA failed: %v", err)
	}
	if err := db.CreateNotebook(notebookB, "History", "/tmp/history.txt", "txt", "", 1); err != nil {
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
		if err := db.EnsureTopic(topic.id, topic.title); err != nil {
			t.Fatalf("EnsureTopic %s failed: %v", topic.id, err)
		}
	}

	parentThermo := "parent-thermo"
	parentNewton := "parent-newton"
	parentRenaissance := "parent-renaissance"
	if err := db.CreateParentSection(parentThermo, "topic-thermo", "Thermo", 1, "heat"); err != nil {
		t.Fatalf("CreateParentSection thermo failed: %v", err)
	}
	if err := db.CreateParentSection(parentNewton, "topic-newton", "Newton", 1, "motion"); err != nil {
		t.Fatalf("CreateParentSection newton failed: %v", err)
	}
	if err := db.CreateParentSection(parentRenaissance, "topic-renaissance", "Renaissance", 1, "history"); err != nil {
		t.Fatalf("CreateParentSection renaissance failed: %v", err)
	}

	chunkThermo := "chunk-thermo"
	chunkNewton := "chunk-newton"
	chunkRenaissance := "chunk-renaissance"
	if err := db.CreateChunk(chunkThermo, "topic-thermo", parentThermo, "thermo chunk", 2); err != nil {
		t.Fatalf("CreateChunk thermo failed: %v", err)
	}
	if err := db.CreateChunk(chunkNewton, "topic-newton", parentNewton, "newton chunk", 2); err != nil {
		t.Fatalf("CreateChunk newton failed: %v", err)
	}
	if err := db.CreateChunk(chunkRenaissance, "topic-renaissance", parentRenaissance, "renaissance chunk", 2); err != nil {
		t.Fatalf("CreateChunk renaissance failed: %v", err)
	}

	if err := db.LinkChunksToNotebook(notebookA, []string{chunkThermo, chunkNewton}); err != nil {
		t.Fatalf("LinkChunksToNotebook notebookA failed: %v", err)
	}
	if err := db.LinkChunksToNotebook(notebookB, []string{chunkRenaissance}); err != nil {
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

func TestAskAINotReadyReturnsError(t *testing.T) {
	app := &App{aiReady: false, aiInitError: "missing runtime assets"}

	resp := app.AskAI("os-scheduling", "What is RR?")
	err, ok := resp["error"].(string)
	if !ok {
		t.Fatalf("expected error string, got: %#v", resp)
	}

	if err == "" {
		t.Fatalf("expected non-empty error message")
	}
}
