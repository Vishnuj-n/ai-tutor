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
	// If APP_ENV was not set by .env or the environment, default to "production".
	// This prevents fresh dev/test environments from silently routing to an undefined
	// storage path. "dev" must be explicitly opted into.
	if os.Getenv("APP_ENV") == "" {
		_ = os.Setenv("APP_ENV", "production")
	}

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
					var loadErr error
					if err := db.Init(dbPath, am.Vec0DllPath()); err != nil {
						loadErr = fmt.Errorf("failed to reload DB with vector extension: %w", err)
					} else if !db.IsVecExtensionLoaded() {
						loadErr = fmt.Errorf("sqlite-vec extension is missing or failed to load (requires CGO and vec0 binary)")
					}

					if loadErr != nil {
						res.AiInitError = loadErr.Error()
						utils.Errorf("%s. Falling back to non-vector DB initialization.", res.AiInitError)
						// Fallback: reload DB without extension so startup doesn't fail
						if fbErr := db.Init(dbPath, ""); fbErr != nil {
							return nil, fmt.Errorf("failed to reload DB even without vector extension: %w", fbErr)
						}
					} else {
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
								if err := db.InitWithVectorDimension(emb.GetDimension()); err != nil {
									res.AiInitError = fmt.Sprintf("could not initialize vector table: %v", err)
									utils.Warnf("%s", res.AiInitError)
									_ = emb.Close()
								} else {
									indexer := retrieval.NewVectorIndexer(emb, retrieval.IndexerConfig{RecomputeOnHashMismatch: true}, ctx)
									if err := indexer.IndexAllTopics(); err != nil {
										utils.Warnf("vector indexing failed: %v", err)
									}
									res.AiReady = true
									res.AiInitError = ""
									res.Embedder = emb
									embedder = emb
								}
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

	llmSettings, err := db.GetLLMSettings()
	if err != nil {
		utils.Warnf("failed to load LLM settings: %v. Falling back to environment config.", err)
		llmSettings = nil
	}
	var fastLLMProvider *llm.Provider
	var heavyLLMProvider *llm.Provider
	if llmSettings != nil {
		fastKey, err := llm.GetAPIKey("fast")
		if err != nil {
			utils.Warnf("FAST_LLM keyring lookup failed or missing: %v", err)
		}
		heavyKey, err := llm.GetAPIKey("heavy")
		if err != nil {
			utils.Warnf("HEAVY_LLM keyring lookup failed or missing: %v", err)
		}
		if heavyKey == "" && fastKey != "" && llmSettings.UseSameForHeavy {
			heavyKey = fastKey
		}
		fastLLMProvider = llm.NewProvider(llm.LoadConfigFromSettingsForPrefix("FAST_LLM", llmSettings.Fast, fastKey))
		heavyLLMProvider = llm.NewProvider(llm.LoadConfigFromSettingsForPrefix("HEAVY_LLM", llmSettings.Heavy, heavyKey))
	} else {
		fastLLMProvider = llm.NewProvider(llm.LoadConfigFromEnvForPrefix("FAST_LLM"))
		heavyLLMProvider = llm.NewProvider(llm.LoadConfigFromEnvForPrefix("HEAVY_LLM"))
	}
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
		dir := filepath.Join(projectRoot, "dev_data")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return "", fmt.Errorf("failed to create dev_data directory: %w", err)
		}
		return dir, nil
	}

	// Prod/default: use a stable per-user directory and guarantee it exists.
	var dir string
	if cfgDir, err := os.UserConfigDir(); err == nil && cfgDir != "" {
		dir = filepath.Join(cfgDir, "ai-tutor")
	} else if cacheDir, err := os.UserCacheDir(); err == nil && cacheDir != "" {
		dir = filepath.Join(cacheDir, "ai-tutor")
	} else if homeDir, err := os.UserHomeDir(); err == nil && homeDir != "" {
		dir = filepath.Join(homeDir, ".ai-tutor")
	} else {
		return "", fmt.Errorf("failed to resolve application data directory")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("failed to create app data directory %s: %w", dir, err)
	}
	return dir, nil
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
