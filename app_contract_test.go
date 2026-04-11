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

	if _, ok := resp["cited_sections"].([]string); !ok {
		t.Fatalf("expected cited_sections []string, got: %#v", resp["cited_sections"])
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
