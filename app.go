package main

import (
	"context"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
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
	embedStore        *rag.EmbeddingStore
	embedder          *embeddings.OnnxEmbedder
	llmProvider       *llm.Provider
	scheduler         *scheduler.Service
	notebookService   *notebook.Service
	notebookUploadDir string
	aiReady           bool
	aiInitError       string
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
		a.aiInitError = err.Error()
		fmt.Printf("Warning: local RAG assets missing: %v\n", err)
		fmt.Println("Ask AI features may be unavailable. Ensure asset/ contains tokenizer.json, model_int8.onnx, onnxruntime.dll, vec0.dll")
	}

	appDir, err := resolveAppDir()
	if err != nil {
		fmt.Printf("Error resolving app directory: %v\n", err)
		return
	}

	runtimeAssets, err := assetValidator.PrepareRuntimeAssets(appDir)
	if err != nil {
		a.aiInitError = err.Error()
		fmt.Printf("Warning: could not stage runtime assets to app-data: %v\n", err)
	}

	// Initialize persistent database
	dbPath, err := resolveDBPath()
	if err != nil {
		fmt.Printf("Error resolving database path: %v\n", err)
		return
	}

	vec0DllPath := assetValidator.Vec0DllPath()
	if stagedVec0Path, ok := runtimeAssets["vec0.dll"]; ok {
		vec0DllPath = stagedVec0Path
	}
	if err := db.Init(dbPath, vec0DllPath); err != nil {
		fmt.Printf("Error initializing database: %v\n", err)
		return
	}
	fmt.Printf("Database initialized at %s\n", dbPath)

	// Initialize ONNX embedder for local vector generation.
	var embedder *embeddings.OnnxEmbedder
	embedder, err = embeddings.NewOnnxEmbedder(assetValidator.ModelPath(), assetValidator.TokenizerPath())
	if err != nil {
		a.aiInitError = err.Error()
		fmt.Printf("Warning: could not initialize ONNX embedder: %v\n", err)
	} else {
		if err := db.InitWithVectorDimension(embedder.GetDimension()); err != nil {
			a.aiInitError = err.Error()
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

	a.embedder = embedder
	a.aiReady = embedder != nil && a.aiInitError == ""

	// Initialize retrieval store with vector-first retrieval and lexical fallback
	embedStore := rag.NewEmbeddingStore(embedder)
	a.embedStore = embedStore

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
	a.llmProvider = llmProvider

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
	topics, err := db.GetAllTopics()
	if err != nil {
		return []map[string]string{}
	}

	return topics
}

// AskAI processes a question using RAG pipeline
func (a *App) AskAI(topicID string, question string) map[string]interface{} {
	if !a.aiReady {
		reason := a.aiInitError
		if reason == "" {
			reason = "local AI runtime is not ready"
		}
		return map[string]interface{}{
			"error": "Ask AI unavailable: " + reason,
		}
	}

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

// GetEmbeddingDiagnostics runs a live embedding call and returns quick sanity metrics.
func (a *App) GetEmbeddingDiagnostics(text string) map[string]interface{} {
	if !a.aiReady || a.embedder == nil {
		reason := a.aiInitError
		if reason == "" {
			reason = "local AI runtime is not ready"
		}
		return map[string]interface{}{
			"error": "Embedding diagnostics unavailable: " + reason,
		}
	}

	input := strings.TrimSpace(text)
	if input == "" {
		input = "quick embedding diagnostic sentence"
	}

	vector, err := a.embedder.Embed(input)
	if err != nil {
		return map[string]interface{}{
			"error": "embedding run failed: " + err.Error(),
		}
	}

	declaredDim := int(a.embedder.GetDimension())
	length := len(vector)
	count := length
	if count > 8 {
		count = 8
	}

	sample := make([]float32, count)
	copy(sample, vector[:count])

	var sumSquares float64
	for _, value := range vector {
		sumSquares += float64(value * value)
	}

	return map[string]interface{}{
		"ok":                  true,
		"input_chars":         len(input),
		"declared_dimension":  declaredDim,
		"vector_length":       length,
		"dimension_match":     length == declaredDim,
		"sample_norm_l2":      math.Sqrt(sumSquares),
		"sample_first_values": sample,
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

func resolveAppDir() (string, error) {
	baseDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}

	appDir := filepath.Join(baseDir, "ai-tutor")
	if err := os.MkdirAll(appDir, 0o755); err != nil {
		return "", err
	}

	return appDir, nil
}

func resolveDBPath() (string, error) {
	appDir, err := resolveAppDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(appDir, "ai-tutor.db"), nil
}

func resolveNotebookDir() (string, error) {
	appDir, err := resolveAppDir()
	if err != nil {
		return "", err
	}
	uploadDir := filepath.Join(appDir, "uploads")
	if err := os.MkdirAll(uploadDir, 0o755); err != nil {
		return "", err
	}

	return uploadDir, nil
}
