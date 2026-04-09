package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/joho/godotenv"
)

// App struct
type App struct {
	ctx         context.Context
	ragPipeline *RAGPipeline
	scheduler   *SchedulerService
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{}
}

// startup is called when the app starts. The context is saved
// so we can call the runtime methods
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx

	// Load local .env if present. Missing file is fine.
	_ = godotenv.Load()

	// Initialize persistent database
	dbPath, err := resolveDBPath()
	if err != nil {
		fmt.Printf("Error resolving database path: %v\n", err)
		return
	}

	if err := InitDB(dbPath); err != nil {
		fmt.Printf("Error initializing database: %v\n", err)
		return
	}
	fmt.Printf("Database initialized at %s\n", dbPath)

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

	// Initialize LLM provider from .env / environment variables.
	llmConfig := LoadLLMConfigFromEnv()

	llmProvider := NewLLMProvider(llmConfig)

	// Create RAG pipeline
	a.ragPipeline = NewRAGPipeline(embedStore, llmProvider)
	a.scheduler = NewSchedulerService()

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

// GetTodayPlan returns a unified daily schedule (review + learning + quiz/socratic when applicable).
func (a *App) GetTodayPlan() map[string]interface{} {
	if a.scheduler == nil {
		return map[string]interface{}{
			"error": "scheduler not initialized",
		}
	}

	plan, err := a.scheduler.BuildTodayPlan(time.Now())
	if err != nil {
		return map[string]interface{}{
			"error": err.Error(),
		}
	}

	return map[string]interface{}{
		"date":             plan.Date,
		"total_minutes":    plan.TotalMinutes,
		"review_minutes":   plan.ReviewMinutes,
		"learning_minutes": plan.LearningMinutes,
		"due_review_cards": plan.DueReviewCards,
		"active_topics":    plan.ActiveTopics,
		"tasks":            plan.Tasks,
	}
}

func resolveDBPath() (string, error) {
	baseDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}

	appDir := filepath.Join(baseDir, "ai-tutor")
	if err := os.MkdirAll(appDir, 0o755); err != nil {
		return "", err
	}

	return filepath.Join(appDir, "ai-tutor.db"), nil
}
