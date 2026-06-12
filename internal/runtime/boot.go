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

	dbPath, err := ResolveDBPath()
	if err != nil {
		res.AiInitError = err.Error()
		utils.Errorf("resolving database path: %v", err)
		return nil, err
	}

	// 1. Initialize DB without loading vec0 extension first (so we can query settings safely)
	if err := db.Init(dbPath, ""); err != nil {
		res.AiInitError = err.Error()
		utils.Errorf("initializing database: %v", err)
		return nil, err
	}
	utils.Infof("Database initialized at %s (extension pre-load phase)", dbPath)

	// Check if RAG is enabled in database
	ragEnabled, err := db.GetRAGEnabled()
	if err != nil {
		utils.Warnf("failed to get RAGEnabled status: %v. Defaulting to false.", err)
		ragEnabled = false
	}

	var embedder *embeddings.OnnxEmbedder
	if ragEnabled {
		// 2. Initialize AssetManager to verify RAG assets are ready
		am, err := NewAssetManager(ctx)
		if err != nil {
			res.AiInitError = fmt.Sprintf("Asset manager init failed: %v", err)
			utils.Warnf("%s", res.AiInitError)
		} else {
			if err := am.EnsureAssetsReady(); err != nil {
				res.AiInitError = fmt.Sprintf("RAG assets not ready: %v", err)
				utils.Warnf("%s", res.AiInitError)
			} else {
				// Stage DLLs and re-init DB with vector support
				if _, err := am.StageDLLs(); err != nil {
					res.AiInitError = fmt.Sprintf("failed to stage DLLs: %v", err)
					utils.Warnf("%s", res.AiInitError)
				} else {
					// Re-initialize DB, this time with the staged vec0.dll
					if err := db.Init(dbPath, am.Vec0DllPath()); err != nil {
						res.AiInitError = fmt.Sprintf("failed to reload DB with vector extension: %v", err)
						utils.Errorf("%s", res.AiInitError)
						return nil, err
					}

					// Init ONNX embedder using paths from AssetManager
					emb, err := embeddings.NewOnnxEmbedder(am.ModelPath(), am.TokenizerPath(), am.OnnxRuntimePath())
					if err != nil {
						res.AiInitError = fmt.Sprintf("failed to load ONNX embedder: %v", err)
						utils.Warnf("%s", res.AiInitError)
					} else {
						if err := embeddings.InitPromptTokenizer(am.TokenizerPath()); err != nil {
							res.AiInitError = fmt.Sprintf("could not initialize prompt tokenizer: %v", err)
							utils.Warnf("%s", res.AiInitError)
							_ = emb.Close()
						} else {
							res.AiReady = true
							res.AiInitError = ""
							res.Embedder = emb
							embedder = emb
							if err := db.InitWithVectorDimension(emb.GetDimension()); err != nil {
								utils.Warnf("could not initialize vector table: %v", err)
							} else {
								indexer := retrieval.NewVectorIndexer(emb, retrieval.IndexerConfig{RecomputeOnHashMismatch: true}, ctx)
								go func() {
									if err := indexer.IndexAllTopics(); err != nil {
										utils.Warnf("vector indexing failed: %v", err)
									}
								}()
							}
						}
					}
				}
			}
		}
	} else {
		utils.Infof("RAG is disabled in user settings. Skipping asset validation and local AI initialization.")
	}

	study.StartCloudSyncLoop()

	res.Scheduler = scheduler.New()

	// Init shared retrieval engine (embedder may be nil, which triggers lexical fallback)
	res.RetrievalEngine = retrieval.NewEngine(embedder)

	topicIDs, err := db.GetAllTopicIDs()
	if err != nil {
		utils.Warnf("could not list topics for lexical fallback: %v", err)
		topicIDs = []string{}
	}
	chunksByTopic, err := db.GetChunksForTopics(topicIDs)
	if err != nil {
		utils.Warnf("could not batch-load chunks: %v", err)
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

	notebookDir, err := ResolveNotebookDir()
	if err != nil {
		utils.Errorf("resolving notebook directory: %v", err)
		return nil, err
	}
	res.NotebookUploadDir = notebookDir
	res.NotebookService = notebook.NewService(notebookDir)
	utils.Infof("App initialized successfully")

	return res, nil
}

func ResolveAppDir() (string, error) {
	// Dev: keep data in the repo for convenience.
	if os.Getenv("APP_ENV") == "dev" {
		projectRoot, err := os.Getwd()
		if err != nil {
			return "", fmt.Errorf("failed to resolve project root: %w", err)
		}
		return filepath.Join(projectRoot, "dev_data"), nil
	}

	// Prod/default: use a stable per-user directory.
	cfgDir, err := os.UserConfigDir()
	if err == nil && cfgDir != "" {
		return filepath.Join(cfgDir, "ai-tutor"), nil
	}
	cacheDir, err := os.UserCacheDir()
	if err == nil && cacheDir != "" {
		return filepath.Join(cacheDir, "ai-tutor"), nil
	}
	homeDir, err := os.UserHomeDir()
	if err == nil && homeDir != "" {
		return filepath.Join(homeDir, ".ai-tutor"), nil
	}
	return "", fmt.Errorf("failed to resolve application data directory")
}

func ResolveDBPath() (string, error) {
	appDir, err := ResolveAppDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(appDir, "ai-tutor.db"), nil
}

func ResolveNotebookDir() (string, error) {
	appDir, err := ResolveAppDir()
	if err != nil {
		return "", err
	}
	uploadDir := filepath.Join(appDir, "uploads")
	if err := os.MkdirAll(uploadDir, 0o755); err != nil {
		return "", err
	}
	return uploadDir, nil
}
