package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
)

// App struct
type App struct {
	ctx         context.Context
	ragPipeline *RAGPipeline
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{}
}

// startup is called when the app starts. The context is saved
// so we can call the runtime methods
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx

	// Initialize database
	dbPath := filepath.Join(os.TempDir(), "ai-tutor.db")
	_ = os.Remove(dbPath)
	if err := InitDB(dbPath); err != nil {
		fmt.Printf("Error initializing database: %v\n", err)
		return
	}
	fmt.Println("Database initialized")

	// Initialize embeddings
	embedStore := NewEmbeddingStore()

	// Load all chunks for all available topics and add to embedding store
	// For now, just load the hardcoded topic
	topicIDs := []string{"os-scheduling"}

	for _, topicID := range topicIDs {
		chunks, err := GetChunksForTopic(topicID)
		if err != nil {
			fmt.Printf("Warning: could not load chunks for topic %s: %v\n", topicID, err)
			continue
		}
		fmt.Printf("Loaded %d chunks for topic %s\n", len(chunks), topicID)
		for _, chunk := range chunks {
			embedStore.AddChunk(chunk)
		}
	}

	// Initialize LLM provider (user should set real config in settings)
	llmConfig := &LLMConfig{
		BaseURL:   os.Getenv("LLM_BASE_URL"),
		APIKey:    os.Getenv("LLM_API_KEY"),
		Model:     os.Getenv("LLM_MODEL"),
		TimeoutMs: 30000,
	}

	// Use defaults if env vars not set
	if llmConfig.BaseURL == "" {
		llmConfig.BaseURL = "http://localhost:8000"
	}
	if llmConfig.Model == "" {
		llmConfig.Model = "gpt-3.5-turbo"
	}
	if llmConfig.APIKey == "" {
		llmConfig.APIKey = "sk-test"
	}

	llmProvider := NewLLMProvider(llmConfig)

	// Create RAG pipeline
	a.ragPipeline = NewRAGPipeline(embedStore, llmProvider)

	fmt.Println("App initialized successfully")
}

// Greet returns a greeting for the given name
func (a *App) Greet(name string) string {
	return fmt.Sprintf("Hello %s, It's show time!", name)
}

// GetTopicContent retrieves the content for a specific topic
func (a *App) GetTopicContent(topicID string) map[string]interface{} {
	content, err := GetTopicContent(topicID)
	if err != nil {
		return map[string]interface{}{
			"error": err.Error(),
		}
	}
	return content
}

// GetAvailableTopics returns a list of available topics
func (a *App) GetAvailableTopics() []map[string]string {
	// TODO: Implement database query to get all topics
	// For now, return hardcoded list
	return []map[string]string{
		{
			"id":    "os-scheduling",
			"title": "Operating Systems: Scheduling",
		},
	}
}

// AskAI processes a question using RAG pipeline
func (a *App) AskAI(topicID string, question string) map[string]interface{} {
	if a.ragPipeline == nil {
		return map[string]interface{}{
			"error": "RAG pipeline not initialized",
		}
	}

	result, err := a.ragPipeline.ProcessQuery(topicID, question)
	if err != nil {
		return map[string]interface{}{
			"error": err.Error(),
		}
	}

	return map[string]interface{}{
		"answer":           result.Answer,
		"cited_sections":   result.CitedSections,
		"chunks_retrieved": result.ChunksRetrieved,
		"sections_used":    result.SectionsUsed,
	}
}
