package runtime

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"ai-tutor/internal/db"
	"ai-tutor/internal/embeddings"
	"ai-tutor/internal/llm"
	"ai-tutor/internal/notebook"
	"ai-tutor/internal/retrieval"
	"ai-tutor/internal/scheduler"
	"ai-tutor/internal/study"
	"ai-tutor/internal/utils"

	"github.com/joho/godotenv"
)

// BootResult holds the initialized states and services for the application.
type BootResult struct {
	Embedder          *embeddings.OnnxEmbedder
	RetrievalEngine   *retrieval.Engine
	FastLLMProvider   *llm.Provider
	HeavyLLMProvider  *llm.Provider
	Scheduler         scheduler.Service
	NotebookService   *notebook.Service
	StudyService      *study.StudyService
	NotebookUploadDir string
	AiReady           bool
	AiInitError       string
}

// Bootstrap runs the entire system initialization block.
func Bootstrap(ctx context.Context) (*BootResult, error) {
	res := &BootResult{
		AiReady: false,
	}

	_ = godotenv.Load()

	assetValidator := NewAssetValidator("asset")
	if err := assetValidator.ValidateAll(); err != nil {
		res.AiInitError = err.Error()
		utils.Warnf("local RAG assets missing: %v", err)
	}

	appDir, err := resolveAppDir()
	if err != nil {
		res.AiInitError = err.Error()
		utils.Errorf("resolving app directory: %v", err)
		return nil, err
	}

	runtimeAssets, err := assetValidator.PrepareRuntimeAssets(appDir)
	if err != nil {
		res.AiInitError = err.Error()
		utils.Warnf("could not stage runtime assets: %v", err)
	}

	dbPath, err := resolveDBPath()
	if err != nil {
		res.AiInitError = err.Error()
		utils.Errorf("resolving database path: %v", err)
		return nil, err
	}

	vec0DllPath := assetValidator.Vec0DllPath()
	if staged, ok := runtimeAssets[filepath.Base(vec0DllPath)]; ok {
		vec0DllPath = staged
	}
	if err := db.Init(dbPath, vec0DllPath); err != nil {
		res.AiInitError = err.Error()
		utils.Errorf("initializing database: %v", err)
		return nil, err
	}
	utils.Infof("Database initialized at %s", dbPath)

	res.Scheduler = scheduler.New()

	// Init ONNX embedder
	onnxRuntimePath := assetValidator.OnnxRuntimePath()
	if staged, ok := runtimeAssets[filepath.Base(onnxRuntimePath)]; ok {
		onnxRuntimePath = staged
	}
	embedder, err := embeddings.NewOnnxEmbedder(assetValidator.ModelPath(), assetValidator.TokenizerPath(), onnxRuntimePath)
	if err != nil {
		res.AiInitError = err.Error()
		utils.Warnf("could not initialize ONNX embedder: %v", err)
	} else {
		if err := embeddings.InitPromptTokenizer(assetValidator.TokenizerPath()); err != nil {
			res.AiInitError = fmt.Sprintf("could not initialize prompt tokenizer: %v", err)
			utils.Warnf("%s", res.AiInitError)
			_ = embedder.Close()
			embedder = nil
		} else {
			res.AiReady = true
			res.AiInitError = ""
			res.Embedder = embedder
			if err := db.InitWithVectorDimension(embedder.GetDimension()); err != nil {
				utils.Warnf("could not initialize vector table: %v", err)
			} else {
				indexer := retrieval.NewVectorIndexer(embedder, retrieval.IndexerConfig{RecomputeOnHashMismatch: true}, ctx)
				go func() {
					if err := indexer.IndexAllTopics(); err != nil {
						utils.Warnf("vector indexing failed: %v", err)
					}
				}()
			}
		}
	}

	// Init shared retrieval engine for Socratic + Reader scoped chat.
	res.RetrievalEngine = retrieval.NewEngine(embedder)

	topicIDs, err := db.GetAllTopicIDs()
	if err != nil {
		utils.Warnf("could not list topics for lexical fallback: %v", err)
		topicIDs = []string{}
	}
	chunksByTopic, err := db.GetChunksForTopics(topicIDs)
	if err != nil {
		utils.Warnf("could not batch-load chunks: %v", err)
		// Continue without chunks rather than making redundant queries
	} else {
		for _, tid := range topicIDs {
			for _, c := range chunksByTopic[tid] {
				res.RetrievalEngine.AddChunk(c)
			}
		}
	}

	fastLLMProvider := llm.NewProvider(llm.LoadConfigFromEnvForPrefix("FAST_LLM"))
	heavyLLMProvider := llm.NewProvider(llm.LoadConfigFromEnvForPrefix("HEAVY_LLM"))
	res.FastLLMProvider = fastLLMProvider
	res.HeavyLLMProvider = heavyLLMProvider

	res.StudyService = study.NewStudyService(study.Config{
		FastLLMProvider:  fastLLMProvider,
		HeavyLLMProvider: heavyLLMProvider,
		RetrievalEngine:  res.RetrievalEngine,
	})

	notebookDir, err := resolveNotebookDir()
	if err != nil {
		utils.Errorf("resolving notebook directory: %v", err)
		return nil, err
	}
	res.NotebookUploadDir = notebookDir
	res.NotebookService = notebook.NewService(notebookDir)
	utils.Infof("App initialized successfully")

	return res, nil
}

func resolveAppDir() (string, error) {
	var appDir string

	// If APP_ENV is set to dev, use a local folder in the project root
	if os.Getenv("APP_ENV") == "dev" {
		// Use current working directory for dev mode
		projectRoot, err := os.Getwd()
		if err != nil {
			return "", fmt.Errorf("failed to resolve project root: %w", err)
		}
		appDir = filepath.Join(projectRoot, "dev_data")
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
