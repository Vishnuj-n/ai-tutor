package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"ai-tutor/internal/db"
	"ai-tutor/internal/embeddings"
	"ai-tutor/internal/llm"
	"ai-tutor/internal/notebook"
	"ai-tutor/internal/rag"
	"ai-tutor/internal/runtime"
	"ai-tutor/internal/scheduler"

	"github.com/joho/godotenv"
)

// App struct
type App struct {
	ctx               context.Context
	ragPipeline       *rag.Pipeline
	scheduler         *scheduler.Service
	notebookService   *notebook.Service
	notebookUploadDir string
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

	// Validate required assets for local RAG
	assetValidator := runtime.NewAssetValidator("asset")
	if err := assetValidator.ValidateAll(); err != nil {
		fmt.Printf("Warning: local RAG assets missing: %v\n", err)
		fmt.Println("Ask AI features may be unavailable. Ensure asset/ contains tokenizer.json, model_int8.onnx, onnxruntime.dll, vec0.dll")
	}

	// Initialize persistent database
	dbPath, err := resolveDBPath()
	if err != nil {
		fmt.Printf("Error resolving database path: %v\n", err)
		return
	}

	vec0DllPath := assetValidator.Vec0DllPath()
	if err := db.Init(dbPath, vec0DllPath); err != nil {
		fmt.Printf("Error initializing database: %v\n", err)
		return
	}
	fmt.Printf("Database initialized at %s\n", dbPath)

	// Initialize ONNX embedder (Phase 10 wiring still pending for real inference output)
	var embedder *embeddings.OnnxEmbedder
	embedder, err = embeddings.NewOnnxEmbedder(assetValidator.ModelPath(), assetValidator.TokenizerPath())
	if err != nil {
		fmt.Printf("Warning: could not initialize ONNX embedder: %v\n", err)
	} else {
		if err := db.InitWithVectorDimension(embedder.GetDimension()); err != nil {
			fmt.Printf("Warning: could not initialize vector table: %v\n", err)
		} else {
			indexer := rag.NewVectorIndexer(embedder, rag.IndexerConfig{
				RecomputeOnHashMismatch: true,
				ForceReindex:            false,
			})
			if err := indexer.IndexAllTopics(); err != nil {
				fmt.Printf("Warning: vector indexing failed: %v\n", err)
			}
		}
	}

	// Initialize retrieval store with vector-first retrieval and lexical fallback
	embedStore := rag.NewEmbeddingStore(embedder)

	// Load chunks for lexical fallback retrieval path.
	topicIDs, err := db.GetAllTopicIDs()
	if err != nil {
		fmt.Printf("Warning: could not list topics for lexical fallback: %v\n", err)
		topicIDs = []string{"os-scheduling"}
	}

	for _, topicID := range topicIDs {
		chunks, err := db.GetChunksForTopic(topicID)
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
	llmConfig := llm.LoadConfigFromEnv()

	llmProvider := llm.NewProvider(llmConfig)

	// Create RAG pipeline
	a.ragPipeline = rag.NewPipeline(embedStore, llmProvider)
	a.scheduler = scheduler.New()

	// Initialize notebook service
	notebookDir, err := resolveNotebookDir()
	if err != nil {
		fmt.Printf("Error resolving notebook directory: %v\n", err)
		return
	}
	a.notebookUploadDir = notebookDir
	a.notebookService = notebook.NewService(notebookDir)
	fmt.Printf("Notebook service initialized at %s\n", notebookDir)

	fmt.Println("App initialized successfully")
}

// Greet returns a greeting for the given name
func (a *App) Greet(name string) string {
	return fmt.Sprintf("Hello %s, It's show time!", name)
}

// GetTopicContent retrieves the content for a specific topic
func (a *App) GetTopicContent(topicID string) map[string]interface{} {
	content, err := db.GetTopicContent(topicID)
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

func resolveNotebookDir() (string, error) {
	baseDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}

	appDir := filepath.Join(baseDir, "ai-tutor")
	uploadDir := filepath.Join(appDir, "uploads")
	if err := os.MkdirAll(uploadDir, 0o755); err != nil {
		return "", err
	}

	return uploadDir, nil
}
