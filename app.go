package main

import (
	"context"
	"fmt"
	"log"
	"math"
	"strings"
	"sync"

	"ai-tutor/internal/db"
	"ai-tutor/internal/embeddings"
	"ai-tutor/internal/llm"
	"ai-tutor/internal/notebook"
	"ai-tutor/internal/retrieval"
	"ai-tutor/internal/runtime"
	"ai-tutor/internal/scheduler"
	"ai-tutor/internal/study"
	"ai-tutor/internal/utils"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

type llmProviderInterface interface {
	GenerateAnswer(prompt string) (string, error)
	ModelName() string
	GetLimits() llm.ModelLimits
}

// App is the thin Wails bridge — no business logic lives here.
type App struct {
	ctx               context.Context
	repo              *db.Repository
	readyChan         chan struct{}
	// aiMutex guards aiReady, aiInitError, embedder, retrievalEngine, and studyService
	// which are written by the InitializeRAG goroutine and read by handler methods.
	aiMutex           sync.Mutex
	embedder          *embeddings.OnnxEmbedder
	retrievalEngine   *retrieval.Engine
	fastLLMProvider   llmProviderInterface
	heavyLLMProvider  llmProviderInterface
	scheduler         scheduler.Service
	notebookService   *notebook.Service
	studyService      *study.StudyService
	notebookUploadDir string
	aiReady           bool
	aiInitError       string
	indexQueue        *retrieval.VectorIndexQueue
}

func NewApp() *App {
	return &App{
		readyChan: make(chan struct{}),
	}
}

func (a *App) waitForReady() {
	if a.readyChan != nil {
		<-a.readyChan
	}
}

func (a *App) getRepo() *db.Repository {
	a.waitForReady()
	return a.repo
}

func (a *App) startup(ctx context.Context) {
	if a.readyChan != nil {
		defer close(a.readyChan)
	}
	a.ctx = ctx

	// Initialize the structured logging pipeline first using the resolved app data directory
	if appDir, err := runtime.ResolveAppDir(); err == nil {
		if logErr := utils.InitMultiFileLogger(appDir); logErr != nil {
			log.Printf("Failed to initialize multi-file logger: %v", logErr)
		}
	} else {
		log.Printf("Failed to resolve app directory: %v", err)
	}

	boot, err := runtime.Bootstrap(ctx)
	if err != nil {
		a.aiInitError = err.Error()
		a.aiReady = false
		return
	}

	// Direct, thin structural assignment handoff
	a.repo = boot.Repo
	a.embedder = boot.Embedder
	a.retrievalEngine = boot.RetrievalEngine
	a.fastLLMProvider = boot.FastLLMProvider
	a.heavyLLMProvider = boot.HeavyLLMProvider
	a.scheduler = boot.Scheduler
	a.notebookService = boot.NotebookService
	a.studyService = boot.StudyService
	a.notebookUploadDir = boot.NotebookUploadDir
	a.aiReady = boot.AiReady
	a.aiInitError = boot.AiInitError

	if a.aiReady && a.embedder != nil {
		a.indexQueue = retrieval.NewVectorIndexQueue(a.repo, a.embedder, ctx)
		a.indexQueue.Start()

		pendingIDs, err := a.repo.GetPendingNotebookIDs()
		if err == nil {
			for _, id := range pendingIDs {
				a.indexQueue.Enqueue(id)
			}
		} else {
			utils.Warnf("failed to retrieve pending notebooks for indexing queue: %v", err)
		}
	}
}

// shutdown is called when the Wails application is shutting down.
func (a *App) shutdown(ctx context.Context) {
	if a.indexQueue != nil {
		a.indexQueue.Stop()
	}
	utils.CloseMultiFileLogger()
}

func (a *App) Greet(name string) string {
	return fmt.Sprintf("Hello %s, It's show time!", name)
}

// LogFrontendEvent accepts a structured log event from the frontend and writes it to the queue logger.
func (a *App) LogFrontendEvent(level string, component string, event string, details string) {
	switch strings.ToLower(level) {
	case "debug":
		utils.QueueLogger.Debug("frontend_event", "component", component, "event", event, "details", details)
	case "warn":
		utils.QueueLogger.Warn("frontend_event", "component", component, "event", event, "details", details)
	case "error":
		utils.QueueLogger.Error("frontend_event", "component", component, "event", event, "details", details)
	default:
		utils.QueueLogger.Info("frontend_event", "component", component, "event", event, "details", details)
	}
}

func (a *App) GetReaderTopicBundle(topicID string, notebookID string) map[string]interface{} {
	repo := a.getRepo()
	if repo == nil {
		return map[string]interface{}{"error": "database repository not initialized"}
	}
	bundle, err := repo.GetReaderTopicBundle(topicID, notebookID)
	if err != nil {
		return map[string]interface{}{"error": err.Error()}
	}
	topicStartPage, topicEndPage, boundsErr := repo.GetTopicPageBounds(topicID)
	if boundsErr != nil {
		topicStartPage, topicEndPage = 0, 0
	}

	lightSections := make([]map[string]interface{}, 0, len(bundle.Sections))
	for _, s := range bundle.Sections {
		lightSections = append(lightSections, map[string]interface{}{
			"id": s.ID, "heading": s.Heading, "page_num": s.PageNum, "order": s.Order,
		})
	}
	return map[string]interface{}{
		"topic_id": bundle.TopicID, "topic_title": bundle.TopicTitle,
		"topic_start_page": topicStartPage, "topic_end_page": topicEndPage,
		"notebook_id": bundle.NotebookID, "notebook_title": bundle.NotebookTitle,
		"notebook_url": bundle.NotebookURL, "file_type": bundle.FileType,
		"page_count": bundle.PageCount, "sections": lightSections,
	}
}

func (a *App) GetAvailableTopics() []map[string]string {
	repo := a.getRepo()
	if repo == nil {
		return []map[string]string{}
	}
	topics, err := repo.GetAllTopics()
	if err != nil {
		return []map[string]string{}
	}
	return topics
}

// checkNotebookIndexingStatus checks if a notebook is currently indexing and returns a progress status map if it is not ready.
func (a *App) checkNotebookIndexingStatus(notebookID, topicID string) (map[string]interface{}, bool) {
	repo := a.repo
	if repo == nil {
		return nil, false
	}

	// Resolve notebook ID if not provided but topic ID is present
	if notebookID == "" && topicID != "" {
		if resolvedID, err := repo.GetNotebookIDByTopic(topicID); err == nil && resolvedID != "" {
			notebookID = resolvedID
		}
	}

	if notebookID == "" {
		return nil, false
	}

	indexed, total, status, err := repo.GetNotebookIndexingProgress(notebookID)
	if err != nil {
		return map[string]interface{}{"error": "failed to check notebook indexing progress: " + err.Error()}, true
	}

	if status != "READY" {
		progress := 0
		if total > 0 {
			progress = (indexed * 100) / total
		}
		return map[string]interface{}{
			"status":   "indexing",
			"progress": progress,
			"error":    "AI features are disabled while this notebook is indexing.",
		}, true
	}

	return nil, false
}

func (a *App) AskSocratic(notebookID string, topicID string, question string) map[string]interface{} {
	repo := a.getRepo()
	if repo == nil {
		return map[string]interface{}{"error": "database repository not initialized"}
	}
	if payload, isIndexing := a.checkNotebookIndexingStatus(notebookID, topicID); isIndexing {
		return payload
	}
	a.aiMutex.Lock()
	if !a.aiReady {
		reason := a.aiInitError
		a.aiMutex.Unlock()
		if reason == "" {
			reason = "local AI runtime is not ready"
		}
		return map[string]interface{}{"error": "Socratic Tutor unavailable: " + reason}
	}
	if a.studyService == nil {
		a.aiMutex.Unlock()
		return map[string]interface{}{"error": "study service not initialized"}
	}
	svc := a.studyService
	a.aiMutex.Unlock()
	res, err := svc.AskSocratic(notebookID, topicID, question)
	if err != nil {
		return map[string]interface{}{"error": err.Error()}
	}
	return res
}

func (a *App) AskReaderAI(topicID, notebookID, question, scope string, currentPage, chapterStartPage, chapterEndPage int) map[string]interface{} {
	repo := a.getRepo()
	if repo == nil {
		return map[string]interface{}{"error": "database repository not initialized"}
	}
	if payload, isIndexing := a.checkNotebookIndexingStatus(notebookID, topicID); isIndexing {
		return payload
	}
	a.aiMutex.Lock()
	if !a.aiReady {
		reason := a.aiInitError
		a.aiMutex.Unlock()
		if reason == "" {
			reason = "local AI runtime is not ready"
		}
		return map[string]interface{}{"error": "Reader AI unavailable: " + reason}
	}
	if a.studyService == nil {
		a.aiMutex.Unlock()
		return map[string]interface{}{"error": "study service not initialized"}
	}
	svc := a.studyService
	a.aiMutex.Unlock()
	return svc.AnswerReaderQuestion(study.ReaderAIRequest{
		TopicID:          topicID,
		NotebookID:       notebookID,
		Question:         question,
		Scope:            study.ReaderRetrievalScope(strings.ToLower(strings.TrimSpace(scope))),
		CurrentPage:      currentPage,
		ChapterStartPage: chapterStartPage,
		ChapterEndPage:   chapterEndPage,
	})
}

func (a *App) GetEmbeddingDiagnostics(text string) map[string]interface{} {
	repo := a.getRepo()
	if repo == nil {
		return map[string]interface{}{"error": "database repository not initialized"}
	}
	a.aiMutex.Lock()
	if !a.aiReady || a.embedder == nil {
		reason := a.aiInitError
		a.aiMutex.Unlock()
		if reason == "" {
			reason = "local AI runtime is not ready"
		}
		return map[string]interface{}{"error": "Embedding diagnostics unavailable: " + reason}
	}
	emb := a.embedder
	a.aiMutex.Unlock()
	input := strings.TrimSpace(text)
	if input == "" {
		input = "quick embedding diagnostic sentence"
	}
	vector, err := emb.Embed(input)
	if err != nil {
		return map[string]interface{}{"error": "embedding run failed: " + err.Error()}
	}
	declaredDim := int(emb.GetDimension())
	count := len(vector)
	if count > 8 {
		count = 8
	}
	sample := make([]float32, count)
	copy(sample, vector[:count])
	var sumSquares float64
	for _, v := range vector {
		sumSquares += float64(v * v)
	}
	return map[string]interface{}{
		"ok": true, "input_chars": len(input),
		"declared_dimension": declaredDim, "vector_length": len(vector),
		"dimension_match": len(vector) == declaredDim,
		"sample_norm_l2":  math.Sqrt(sumSquares), "sample_first_values": sample,
	}
}


// Global state variables for RAG initialization lock
var ragSetupMutex sync.Mutex
var isRagSettingUp bool

func (a *App) InitializeRAG() map[string]interface{} {
	repo := a.getRepo()
	if repo == nil {
		return map[string]interface{}{"error": "database repository not initialized"}
	}
	ragSetupMutex.Lock()
	if isRagSettingUp {
		ragSetupMutex.Unlock()
		return map[string]interface{}{"error": "RAG initialization is already in progress"}
	}
	isRagSettingUp = true
	ragSetupMutex.Unlock()

	go func() {
		defer func() {
			ragSetupMutex.Lock()
			isRagSettingUp = false
			ragSetupMutex.Unlock()
		}()

		am, err := runtime.NewAssetManager(a.ctx)
		if err != nil {
			emitRagSetupFailed(a, fmt.Sprintf("failed to create asset manager: %v", err))
			return
		}

		// Run simulated download
		err = am.AcquireAssets(func(status string, percent int, msg, detail string) {
			wailsruntime.EventsEmit(a.ctx, "rag-setup-progress", map[string]interface{}{
				"status":  status,
				"percent": percent,
				"message": msg,
				"detail":  detail,
			})
		})

		if err != nil {
			emitRagSetupFailed(a, fmt.Sprintf("acquisition failed: %v", err))
			return
		}

		// Stage DLLs and re-init DB with vector support
		if _, err := am.StageDLLs(); err != nil {
			emitRagSetupFailed(a, fmt.Sprintf("failed to stage DLLs: %v", err))
			return
		}

		dbPath, err := runtime.ResolveDBPath()
		if err != nil {
			emitRagSetupFailed(a, fmt.Sprintf("failed to resolve database path: %v", err))
			return
		}

		// Re-initialize DB, this time with the staged vec0.dll
		newRepo, err := db.Init(dbPath, am.Vec0DllPath())
		if err != nil {
			fbRepo, fbErr := db.Init(dbPath, "")
			if fbErr != nil {
				emitRagSetupFailed(a, fmt.Sprintf("failed to reload DB with vector extension: %v, and fallback non-vector initialization also failed: %v", err, fbErr))
			} else {
				_ = repo.Close()
				a.repo = fbRepo
				emitRagSetupFailed(a, fmt.Sprintf("failed to reload DB with vector extension: %v", err))
			}
			return
		}

		if !newRepo.IsVecExtensionLoaded() {
			_ = newRepo.Close()
			fbRepo, fbErr := db.Init(dbPath, "")
			if fbErr != nil {
				emitRagSetupFailed(a, fmt.Sprintf("sqlite-vec extension is missing or failed to load (requires CGO and vec0 binary), and fallback non-vector initialization also failed: %v", fbErr))
			} else {
				_ = repo.Close()
				a.repo = fbRepo
				emitRagSetupFailed(a, "sqlite-vec extension is missing or failed to load (requires CGO and vec0 binary)")
			}
			return
		}

		// Init ONNX embedder using paths from AssetManager
		emb, err := embeddings.NewOnnxEmbedder(am.ModelPath(), am.TokenizerPath(), am.OnnxRuntimePath())
		if err != nil {
			_ = newRepo.Close()
			emitRagSetupFailed(a, fmt.Sprintf("failed to load ONNX embedder: %v", err))
			return
		}

		if err := embeddings.InitPromptTokenizer(am.TokenizerPath()); err != nil {
			_ = emb.Close()
			_ = newRepo.Close()
			emitRagSetupFailed(a, fmt.Sprintf("could not initialize prompt tokenizer: %v", err))
			return
		}

		// Success loading vec0 extension! Close old repo connection, set new repo.
		_ = repo.Close()
		a.repo = newRepo

		// Set dimensions
		if err := newRepo.InitWithVectorDimension(emb.GetDimension()); err != nil {
			utils.Warnf("could not initialize vector table: %v", err)
		}

		// Update app fields with embedder (under lock); readiness waits for full bootstrap.
		a.aiMutex.Lock()
		a.embedder = emb
		a.aiReady = false
		a.aiInitError = ""
		a.aiMutex.Unlock()

		// Rebuild engine and study service
		if err := a.reloadRetrievalEngine(); err != nil {
			a.aiMutex.Lock()
			a.aiReady = false
			a.aiInitError = fmt.Sprintf("failed to reload retrieval engine: %v", err)
			a.aiMutex.Unlock()
			emitRagSetupFailed(a, a.aiInitError)
			return
		}

		// Save settings in DB to reflect RAG is enabled
		settings, err := newRepo.GetUserSettings()
		if err == nil {
			settings.RAGEnabled = true
			_ = newRepo.UpdateUserSettings(*settings)
		}

		// Emit indexing-in-progress event before starting vector indexing
		wailsruntime.EventsEmit(a.ctx, "rag-setup-progress", map[string]interface{}{
			"status":  "indexing",
			"percent": 98,
			"message": "Indexing topics for AI retrieval...",
			"detail":  "Building vector index",
		})

		// Index all existing topics; emit ready only after success
		indexer := retrieval.NewVectorIndexer(a.repo, emb, retrieval.IndexerConfig{RecomputeOnHashMismatch: true}, a.ctx)
		if err := indexer.IndexAllTopics(); err != nil {
			utils.Errorf("vector indexing failed after RAG enable: %v", err)
			a.aiMutex.Lock()
			a.aiReady = false
			a.aiInitError = fmt.Sprintf("vector indexing failed: %v", err)
			a.aiMutex.Unlock()
			emitRagSetupFailed(a, a.aiInitError)
			return
		}

		// Mark ready only after all bootstrap steps succeed.
		a.aiMutex.Lock()
		a.aiReady = true
		a.aiInitError = ""
		a.aiMutex.Unlock()

		// Emit final ready event only after indexing succeeds
		wailsruntime.EventsEmit(a.ctx, "rag-setup-progress", map[string]interface{}{
			"status":  "ready",
			"percent": 100,
			"message": "Local AI retrieval is fully ready!",
			"detail":  "RAG engine active",
		})
	}()

	return map[string]interface{}{"ok": true}
}

func (a *App) reloadRetrievalEngine() error {
	a.aiMutex.Lock()
	emb := a.embedder
	a.aiMutex.Unlock()

	engine := retrieval.NewEngine(a.repo, emb)
	topicIDs, err := a.repo.GetAllTopicIDs()
	if err != nil {
		return fmt.Errorf("reloadRetrievalEngine: GetAllTopicIDs: %w", err)
	}
	chunksByTopic, err := a.repo.GetChunksForTopics(topicIDs)
	if err != nil {
		return fmt.Errorf("reloadRetrievalEngine: GetChunksForTopics: %w", err)
	}
	for _, tid := range topicIDs {
		for _, c := range chunksByTopic[tid] {
			engine.AddChunk(c)
		}
	}

	// Recreate study service to bind the new engine; update both under lock.
	newSvc := study.NewStudyService(study.Config{
		Repo:             a.repo,
		FastLLMProvider:  a.fastLLMProvider,
		HeavyLLMProvider: a.heavyLLMProvider,
		RetrievalEngine:  engine,
	})
	a.aiMutex.Lock()
	a.retrievalEngine = engine
	a.studyService = newSvc
	a.aiMutex.Unlock()
	return nil
}

func emitRagSetupFailed(a *App, reason string) {
	utils.Errorf("RAG setup failed: %s", reason)
	wailsruntime.EventsEmit(a.ctx, "rag-setup-progress", map[string]interface{}{
		"status":      "failed",
		"percent":     100,
		"message":     "RAG initialization failed",
		"detail":      reason,
		"errorReason": reason,
	})
}
