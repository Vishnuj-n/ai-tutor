package main

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"ai-tutor/internal/db"
	"ai-tutor/internal/embeddings"
	"ai-tutor/internal/llm"
	"ai-tutor/internal/models"
	"ai-tutor/internal/notebook"
	"ai-tutor/internal/rag"
	"ai-tutor/internal/retrieval"
	"ai-tutor/internal/runtime"
	"ai-tutor/internal/scheduler"
	"ai-tutor/internal/study"
	"ai-tutor/internal/utils"

	"github.com/google/uuid"
	"github.com/joho/godotenv"
)

type llmProviderInterface interface {
	GenerateAnswer(prompt string) (string, error)
}

type ragPipelineInterface interface {
	ProcessQuery(topicID, question string, startPage, endPage int) (*rag.Response, error)
}

// App is the thin Wails bridge — no business logic lives here.
type App struct {
	ctx               context.Context
	ragPipeline       ragPipelineInterface
	embedStore        *rag.EmbeddingStore
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
}

func NewApp() *App { return &App{} }

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	_ = godotenv.Load()

	assetValidator := runtime.NewAssetValidator("asset")
	if err := assetValidator.ValidateAll(); err != nil {
		a.aiInitError = err.Error()
		utils.Warnf("local RAG assets missing: %v", err)
	}

	appDir, err := resolveAppDir()
	if err != nil {
		a.aiInitError = err.Error()
		utils.Errorf("resolving app directory: %v", err)
		return
	}

	runtimeAssets, err := assetValidator.PrepareRuntimeAssets(appDir)
	if err != nil {
		a.aiInitError = err.Error()
		utils.Warnf("could not stage runtime assets: %v", err)
	}

	dbPath, err := resolveDBPath()
	if err != nil {
		a.aiInitError = err.Error()
		utils.Errorf("resolving database path: %v", err)
		return
	}

	vec0DllPath := assetValidator.Vec0DllPath()
	if staged, ok := runtimeAssets[filepath.Base(vec0DllPath)]; ok {
		vec0DllPath = staged
	}
	if err := db.Init(dbPath, vec0DllPath); err != nil {
		a.aiInitError = err.Error()
		utils.Errorf("initializing database: %v", err)
		return
	}
	utils.Infof("Database initialized at %s", dbPath)

	a.scheduler = scheduler.New()

	// Init ONNX embedder
	onnxRuntimePath := assetValidator.OnnxRuntimePath()
	if staged, ok := runtimeAssets[filepath.Base(onnxRuntimePath)]; ok {
		onnxRuntimePath = staged
	}
	embedder, err := embeddings.NewOnnxEmbedder(assetValidator.ModelPath(), assetValidator.TokenizerPath(), onnxRuntimePath)
	if err != nil {
		a.aiInitError = err.Error()
		utils.Warnf("could not initialize ONNX embedder: %v", err)
	} else {
		if err := embeddings.InitPromptTokenizer(assetValidator.TokenizerPath()); err != nil {
			a.aiInitError = fmt.Sprintf("could not initialize prompt tokenizer: %v", err)
			utils.Warnf("%s", a.aiInitError)
			_ = embedder.Close()
			embedder = nil
		} else {
			a.aiReady = true
			a.aiInitError = ""
			a.embedder = embedder
			if err := db.InitWithVectorDimension(embedder.GetDimension()); err != nil {
				utils.Warnf("could not initialize vector table: %v", err)
			} else {
				indexer := rag.NewVectorIndexer(embedder, rag.IndexerConfig{RecomputeOnHashMismatch: true}, ctx)
				go func() {
					if err := indexer.IndexAllTopics(); err != nil {
						utils.Warnf("vector indexing failed: %v", err)
					}
				}()
			}
		}
	}

	// Init retrieval engine (standalone, used only by Socratic mode)
	a.retrievalEngine = retrieval.NewEngine(embedder)

	// Init RAG embedding store + pipeline (used by AskAI / Reader)
	embedStore := rag.NewEmbeddingStore(embedder)
	a.embedStore = embedStore
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
				embedStore.AddChunk(c)
				a.retrievalEngine.AddChunk(c)
			}
		}
	}

	fastLLMProvider := llm.NewProvider(llm.LoadConfigFromEnvForPrefix("FAST_LLM"))
	heavyLLMProvider := llm.NewProvider(llm.LoadConfigFromEnvForPrefix("HEAVY_LLM"))
	a.fastLLMProvider = fastLLMProvider
	a.heavyLLMProvider = heavyLLMProvider

	a.ragPipeline = rag.NewPipeline(embedStore, heavyLLMProvider)
	a.studyService = study.NewStudyService(study.Config{
		FastLLMProvider:  fastLLMProvider,
		HeavyLLMProvider: heavyLLMProvider,
		RetrievalEngine:  a.retrievalEngine,
	})

	notebookDir, err := resolveNotebookDir()
	if err != nil {
		utils.Errorf("resolving notebook directory: %v", err)
		return
	}
	a.notebookUploadDir = notebookDir
	a.notebookService = notebook.NewService(notebookDir)
	utils.Infof("App initialized successfully")
}

func (a *App) Greet(name string) string {
	return fmt.Sprintf("Hello %s, It's show time!", name)
}

func (a *App) GetTopicContent(topicID string) map[string]interface{} {
	content, err := db.GetTopicContent(topicID)
	if err != nil {
		return map[string]interface{}{"error": err.Error()}
	}
	return content
}

func (a *App) GetReaderTopicBundle(topicID string, notebookID string) map[string]interface{} {
	bundle, err := db.GetReaderTopicBundle(topicID, notebookID)
	if err != nil {
		return map[string]interface{}{"error": err.Error()}
	}
	topicStartPage, topicEndPage, boundsErr := db.GetTopicPageBounds(topicID)
	if boundsErr != nil {
		topicStartPage, topicEndPage = 0, 0
	}
	if bundle.NotebookURL != "" {
		bundle.NotebookURL = notebookAssetURL(bundle.NotebookURL)
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
	topics, err := db.GetAllTopics()
	if err != nil {
		return []map[string]string{}
	}
	return topics
}

func (a *App) AskAI(topicID string, question string) map[string]interface{} {
	if !a.aiReady {
		reason := a.aiInitError
		if reason == "" {
			reason = "local AI runtime is not ready"
		}
		return map[string]interface{}{"error": "Ask AI unavailable: " + reason}
	}
	if a.ragPipeline == nil {
		return map[string]interface{}{"error": "RAG pipeline not initialized"}
	}
	result, err := a.ragPipeline.ProcessQuery(topicID, question, 0, 0)
	if err != nil {
		return map[string]interface{}{"error": err.Error()}
	}
	return map[string]interface{}{
		"answer": result.Answer, "cited_sections": result.CitedSections,
		"chunks_retrieved": result.ChunksRetrieved, "sections_used": result.SectionsUsed,
	}
}

func (a *App) ExplainReaderSection(sectionID string, question string) map[string]interface{} {
	if a.studyService == nil {
		return map[string]interface{}{"error": "study service not initialized"}
	}
	return a.studyService.ExplainReaderSection(sectionID, question)
}

func (a *App) GetEmbeddingDiagnostics(text string) map[string]interface{} {
	if !a.aiReady || a.embedder == nil {
		reason := a.aiInitError
		if reason == "" {
			reason = "local AI runtime is not ready"
		}
		return map[string]interface{}{"error": "Embedding diagnostics unavailable: " + reason}
	}
	input := strings.TrimSpace(text)
	if input == "" {
		input = "quick embedding diagnostic sentence"
	}
	vector, err := a.embedder.Embed(input)
	if err != nil {
		return map[string]interface{}{"error": "embedding run failed: " + err.Error()}
	}
	declaredDim := int(a.embedder.GetDimension())
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

func (a *App) GetTodayPlan() map[string]interface{} {
	if a.scheduler == nil {
		return map[string]interface{}{"error": "scheduler not initialized"}
	}
	now := time.Now()
	plan, err := a.scheduler.BuildTodayPlan(now)
	if err != nil {
		return map[string]interface{}{"error": err.Error()}
	}
	return map[string]interface{}{
		"date": plan.Date, "total_minutes": plan.TotalMinutes,
		"review_minutes": plan.ReviewMinutes, "learning_minutes": plan.LearningMinutes,
		"due_review_cards": plan.DueReviewCards, "active_topics": plan.ActiveTopics,
		"tasks": plan.Tasks, "generated_at_unix": now.Unix(),
		"data_fresh": true, "is_estimate": plan.IsEstimate,
		"insights_available": false, "plan_source": "scheduler-v2-context-locked",
	}
}

func (a *App) GetNextTask(notebookID string) map[string]interface{} {
	if strings.TrimSpace(notebookID) == "" {
		return map[string]interface{}{"error": "notebook ID is required", "code": 400}
	}
	task, err := db.GetNextTask(notebookID)
	if err != nil {
		if err == db.ErrNoPendingTasks {
			return map[string]interface{}{
				"error": "ErrNoPendingTasks",
				"code":  204,
			}
		}
		return map[string]interface{}{"error": err.Error()}
	}
	return map[string]interface{}{"task": task}
}

func (a *App) ActivateTask(taskID string) map[string]interface{} {
	if strings.TrimSpace(taskID) == "" {
		return map[string]interface{}{"error": "task ID is required", "code": 400}
	}
	if err := db.ActivateTask(taskID); err != nil {
		switch err {
		case db.ErrTaskNotFound:
			return map[string]interface{}{"error": "ErrNotFound", "code": 404}
		case db.ErrTaskNotPending:
			return map[string]interface{}{"error": "ErrTaskNotPending", "code": 409}
		default:
			return map[string]interface{}{"error": err.Error()}
		}
	}
	return map[string]interface{}{"ok": true}
}

func (a *App) CompleteTask(taskID string, result models.CompletionResult) map[string]interface{} {
	if strings.TrimSpace(taskID) == "" {
		return map[string]interface{}{"error": "task ID is required", "code": 400}
	}
	if err := db.CompleteTask(taskID, result); err != nil {
		switch err {
		case db.ErrTaskNotFound:
			return map[string]interface{}{"error": "ErrNotFound", "code": 404}
		case db.ErrTaskNotActive:
			return map[string]interface{}{"error": "ErrTaskNotActive", "code": 409}
		default:
			return map[string]interface{}{"error": err.Error()}
		}
	}
	return map[string]interface{}{"ok": true}
}

func (a *App) SkipTask(taskID string) map[string]interface{} {
	if strings.TrimSpace(taskID) == "" {
		return map[string]interface{}{"error": "task ID is required", "code": 400}
	}
	if err := db.SkipTask(taskID); err != nil {
		switch err {
		case db.ErrTaskNotFound:
			return map[string]interface{}{"error": "ErrNotFound", "code": 404}
		default:
			return map[string]interface{}{"error": err.Error()}
		}
	}
	return map[string]interface{}{"ok": true}
}

func (a *App) GetQueueState(notebookID string) map[string]interface{} {
	if strings.TrimSpace(notebookID) == "" {
		return map[string]interface{}{"error": "notebook ID is required", "code": 400}
	}
	state, err := db.GetQueueState(notebookID)
	if err != nil {
		return map[string]interface{}{"error": err.Error()}
	}
	return map[string]interface{}{"queue_state": state}
}

func (a *App) GetReadingTask(taskID string) map[string]interface{} {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return map[string]interface{}{"error": "task ID is required", "code": 400}
	}
	task, err := db.GetReadingTask(taskID)
	if err != nil {
		if err == db.ErrTaskNotFound {
			return map[string]interface{}{"error": "ErrNotFound", "code": 404}
		}
		return map[string]interface{}{"error": err.Error()}
	}
	return map[string]interface{}{"task": task}
}

// InitializeReadingSession consolidates task activation, reading task loading,
// and page bounds resolution into a single canonical backend call.
// Accepts the full routing context so scheduler-suggested tasks (not yet in study_queue)
// can be materialized as real queue rows on first open.
func (a *App) InitializeReadingSession(taskID, notebookID, topicID string, startPage, endPage int) map[string]interface{} {
	taskID = strings.TrimSpace(taskID)
	notebookID = strings.TrimSpace(notebookID)
	topicID = strings.TrimSpace(topicID)
	if taskID == "" {
		return map[string]interface{}{"error": "task ID is required", "code": 400}
	}

	// If task doesn't exist yet (e.g. scheduler-generated synthetic ID),
	// insert it as a real READING task so the queue lifecycle can proceed.
	if _, err := db.GetTaskByID(taskID); err == db.ErrTaskNotFound {
		if notebookID == "" || topicID == "" {
			return map[string]interface{}{"error": "task not found and notebookID/topicID required to create it", "code": 400}
		}
		insertErr := db.InsertStudyTask(models.StudyQueueTask{
			ID:         taskID,
			NotebookID: notebookID,
			TopicID:    topicID,
			TaskType:   models.StudyTaskTypeReading,
			Status:     models.StudyTaskStatusPending,
			Priority:   1,
			StartPage:  startPage,
			EndPage:    endPage,
		})
		if insertErr != nil {
			return map[string]interface{}{"error": "failed to create reading task: " + insertErr.Error()}
		}
	}

	// Activate task (idempotent if already active)
	_ = db.ActivateTask(taskID)

	// Load reading task with all context
	task, err := db.GetReadingTask(taskID)
	if err != nil {
		if err == db.ErrTaskNotFound {
			return map[string]interface{}{"error": "ErrNotFound", "code": 404}
		}
		return map[string]interface{}{"error": err.Error()}
	}

	// Get topic bundle for additional metadata
	bundle, err := db.GetReaderTopicBundle(task.TopicID, task.NotebookID)
	if err != nil {
		// Return task-only response if bundle fails
		return map[string]interface{}{
			"ok":   true,
			"task": task,
			"page_bounds": map[string]interface{}{
				"start_page":   task.StartPage,
				"end_page":     task.EndPage,
				"current_page": task.StartPage,
				"page_count":   0,
			},
			"navigation": map[string]interface{}{
				"can_go_prev": task.StartPage > 1,
				"can_go_next": true,
			},
		}
	}

	// Get current progress from reading_progress table
	var currentPage int
	err = db.GetConnection().QueryRow(`
		SELECT COALESCE(current_page, 0) FROM reading_progress WHERE task_id = ?
	`, taskID).Scan(&currentPage)
	if err != nil || currentPage == 0 {
		currentPage = task.StartPage
	}
	// Clamp to bounds
	if currentPage < task.StartPage {
		currentPage = task.StartPage
	}
	if currentPage > task.EndPage {
		currentPage = task.EndPage
	}

	return map[string]interface{}{
		"ok":     true,
		"task":   task,
		"bundle": bundle,
		"page_bounds": map[string]interface{}{
			"start_page":   task.StartPage,
			"end_page":     task.EndPage,
			"current_page": currentPage,
			"page_count":   bundle.PageCount,
		},
		"navigation": map[string]interface{}{
			"can_go_prev": currentPage > task.StartPage,
			"can_go_next": currentPage < task.EndPage,
			// Note: can_complete removed - trust-based completion, user decides when done
		},
	}
}

// ValidateReadingCompletion - DEPRECATED/LEGACY
// Trust-based completion model: user decides when reading is complete.
// This endpoint only persists reading progress, it does NOT validate page completion.
func (a *App) ValidateReadingCompletion(taskID string, finalPage int) map[string]interface{} {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return map[string]interface{}{"error": "task ID is required", "code": 400}
	}
	// Persist progress only, no validation
	_, err := db.PersistReadingProgress(taskID, finalPage)
	if err != nil {
		if err == db.ErrTaskNotFound {
			return map[string]interface{}{"error": "ErrNotFound", "code": 404}
		}
		return map[string]interface{}{"error": err.Error()}
	}
	return map[string]interface{}{"ok": true}
}

func (a *App) CompleteReading(taskID string) map[string]interface{} {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return map[string]interface{}{"error": "task ID is required", "code": 400}
	}

	// Trust-based completion: just validate task exists and is active
	task, err := db.GetReadingTask(taskID)
	if err != nil {
		switch err {
		case db.ErrTaskNotFound:
			return map[string]interface{}{"error": "ErrNotFound", "code": 404}
		default:
			return map[string]interface{}{"error": err.Error()}
		}
	}

	if a.studyService == nil {
		return map[string]interface{}{"error": "study service not initialized"}
	}

	// Generate quiz from full assigned chunk range (no page validation)
	chunkIDs, err := db.GetChunkIDsForTopicPageRange(task.TopicID, task.StartPage, task.EndPage)
	if err != nil {
		return map[string]interface{}{"error": err.Error()}
	}

	quizPayload, err := a.studyService.GenerateQuizSync(task.TopicID, chunkIDs, nil)
	if err != nil {
		return map[string]interface{}{"error": err.Error()}
	}

	// Complete reading task and generate follow-up quiz
	// No page completion validation required - user decides when done
	quizTaskID, err := db.CompleteReadingWithGeneratedQuiz(taskID, quizPayload)
	if err != nil {
		switch err {
		case db.ErrTaskNotFound:
			return map[string]interface{}{"error": "ErrNotFound", "code": 404}
		case db.ErrTaskNotActive:
			return map[string]interface{}{"error": "ErrTaskNotActive", "code": 409}
		default:
			return map[string]interface{}{"error": err.Error()}
		}
	}
	return map[string]interface{}{"ok": true, "quiz_task_id": quizTaskID}
}

func (a *App) GetTask(taskID string) map[string]interface{} {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return map[string]interface{}{"error": "task ID is required", "code": 400}
	}
	task, err := db.GetTaskByID(taskID)
	if err != nil {
		if err == db.ErrTaskNotFound {
			return map[string]interface{}{"error": "ErrNotFound", "code": 404}
		}
		return map[string]interface{}{"error": err.Error()}
	}
	return map[string]interface{}{"task": task}
}

func (a *App) GenerateQuizForPageRange(notebookID string, startPage, endPage int) map[string]interface{} {
	if a.studyService == nil {
		return map[string]interface{}{"error": "study service not initialized"}
	}
	return a.studyService.GenerateQuizForPageRange(notebookID, startPage, endPage)
}

func (a *App) GenerateQuizSync(topicID string, chunkIDs []string) map[string]interface{} {
	if a.studyService == nil {
		return map[string]interface{}{"error": "study service not initialized"}
	}
	payload, err := a.studyService.GenerateQuizSync(topicID, chunkIDs, nil)
	if err != nil {
		return map[string]interface{}{"error": err.Error()}
	}
	return map[string]interface{}{"quiz_task": payload}
}

func (a *App) SubmitQuizAttempt(taskID string, answers []models.QuizAnswer) map[string]interface{} {
	if a.studyService == nil {
		return map[string]interface{}{"error": "study service not initialized"}
	}
	result, err := a.studyService.SubmitQuizAttempt(taskID, answers)
	if err != nil {
		switch err {
		case db.ErrTaskNotFound:
			return map[string]interface{}{"error": "ErrNotFound", "code": 404}
		case db.ErrTaskNotActive:
			return map[string]interface{}{"error": "ErrTaskNotActive", "code": 409}
		default:
			return map[string]interface{}{"error": err.Error()}
		}
	}
	return map[string]interface{}{"result": result}
}

func (a *App) GetDailyStudySettings() map[string]interface{} {
	minutes, err := db.GetDailyStudyMinutes()
	if err != nil {
		return map[string]interface{}{"error": err.Error()}
	}
	return map[string]interface{}{"daily_study_minutes": minutes}
}

func (a *App) UpdateDailyStudyMinutes(minutes int) map[string]interface{} {
	if minutes < 15 || minutes > 480 {
		return map[string]interface{}{"error": "daily study minutes must be between 15 and 480"}
	}
	if err := db.UpsertDailyStudyMinutes(minutes); err != nil {
		return map[string]interface{}{"error": err.Error()}
	}
	return map[string]interface{}{"ok": true, "daily_study_minutes": minutes}
}

// ---------- Marathon Mode endpoints (Phase 1 new) ----------

func (a *App) GenerateMarathonQuiz(notebookID string, startPage, endPage int) map[string]interface{} {
	if a.studyService == nil {
		return map[string]interface{}{"error": "study service not initialized"}
	}
	return a.studyService.GenerateMarathonQuiz(notebookID, startPage, endPage)
}

func (a *App) GenerateMarathonFlashcards(notebookID string, startPage, endPage int) map[string]interface{} {
	if a.studyService == nil {
		return map[string]interface{}{"error": "study service not initialized"}
	}
	return a.studyService.GenerateMarathonFlashcards(notebookID, startPage, endPage)
}

func (a *App) GenerateComprehensiveExam(notebookID string, startPage, endPage int) map[string]interface{} {
	if a.studyService == nil {
		return map[string]interface{}{"error": "study service not initialized"}
	}
	return a.studyService.GenerateComprehensiveExam(notebookID, startPage, endPage)
}

func (a *App) GenerateFlashcards(topicID string) map[string]interface{} {
	if a.studyService == nil {
		return map[string]interface{}{"error": "study service not initialized"}
	}

	// Get notebook for this topic
	notebooks, err := db.GetNotebooks(topicID)
	if err != nil {
		return map[string]interface{}{"error": "failed to get notebook: " + err.Error()}
	}
	if len(notebooks) == 0 {
		return map[string]interface{}{"error": "no notebook found for topic"}
	}
	notebookID := notebooks[0].ID

	// Get page bounds for this topic
	startPage, endPage, err := db.GetTopicPageBounds(topicID)
	if err != nil {
		return map[string]interface{}{"error": "failed to get topic page bounds: " + err.Error()}
	}

	return a.studyService.GenerateMarathonFlashcardsWithTopic(topicID, notebookID, startPage, endPage)
}

// ---------- Reader / existing flows ----------

func (a *App) CompleteReadingSession(topicID string, startPage int, targetPage int) map[string]interface{} {
	if a.studyService == nil {
		return map[string]interface{}{"error": "study service not initialized"}
	}
	return a.studyService.CompleteReadingSession(topicID, startPage, targetPage)
}

func (a *App) GetFlashcards(topicID string, dueOnly bool) map[string]interface{} {
	topicID = strings.TrimSpace(topicID)
	if topicID == "" {
		return map[string]interface{}{"error": "topic ID is required"}
	}
	var now int64
	if dueOnly {
		now = time.Now().Unix()
	}
	cards, err := db.GetFlashcardsForTopic(topicID, dueOnly, now)
	if err != nil {
		return map[string]interface{}{"error": "failed to fetch flashcards: " + err.Error()}
	}
	return map[string]interface{}{"topic_id": topicID, "cards": cards}
}

func (a *App) RecordFlashcardReview(cardID string, rating string) map[string]interface{} {
	cardID = strings.TrimSpace(cardID)
	rating = strings.ToLower(strings.TrimSpace(rating))
	if cardID == "" {
		return map[string]interface{}{"error": "flashcard ID is required"}
	}
	ratingCode, ok := mapReviewRating(rating)
	if !ok {
		return map[string]interface{}{"error": "rating must be one of again, hard, good, easy"}
	}
	card, state, err := db.GetFlashcardByID(cardID)
	if err != nil {
		return map[string]interface{}{"error": "failed to fetch flashcard: " + err.Error()}
	}
	if card == nil || state == nil {
		return map[string]interface{}{"error": "flashcard not found"}
	}
	stateBeforeJSONBytes, err := json.Marshal(state)
	if err != nil {
		return map[string]interface{}{"error": "failed to encode flashcard state: " + err.Error()}
	}
	now := time.Now().Unix()
	elapsedSeconds := now - card.DueAt
	elapsedDays := 0
	if elapsedSeconds > 0 {
		elapsedDays = int(elapsedSeconds / (24 * 60 * 60))
	}
	state.ElapsedDays = elapsedDays
	nextState := scheduler.NextFSRSState(*state, ratingCode)
	dueAt := now + int64(nextState.ScheduledDays)*24*60*60
	if nextState.ScheduledDays == 0 {
		dueAt = now
	}
	stateAfterJSONBytes, err := json.Marshal(nextState)
	if err != nil {
		return map[string]interface{}{"error": "failed to encode updated flashcard state: " + err.Error()}
	}
	reviewLog := models.FSRSReviewLog{
		ID: uuid.NewString(), TopicID: card.TopicID, ActivityType: "flashcard",
		ReferenceID: card.ID, ReviewedAt: now, Rating: ratingCode,
		ScheduledDays:   nextState.ScheduledDays,
		StateBeforeJSON: string(stateBeforeJSONBytes), StateAfterJSON: string(stateAfterJSONBytes),
	}
	if err := db.UpdateFlashcardReview(cardID, dueAt, card.DueAt, nextState, reviewLog); err != nil {
		return map[string]interface{}{"error": "failed to update flashcard review: " + err.Error()}
	}
	// Note: UpdateFlashcardReview already logs to fsrs_review_log, so no need to call studyService.LogReview
	// Only update local state after successful database transaction
	card.DueAt = dueAt
	return map[string]interface{}{"card": card, "state": &nextState, "review_log_id": reviewLog.ID}
}

func (a *App) ScoreAnswer(questionID, userAnswer string) map[string]interface{} {
	questionID = strings.TrimSpace(questionID)
	userAnswer = strings.TrimSpace(userAnswer)
	if questionID == "" || userAnswer == "" {
		return map[string]interface{}{"error": "question ID and user answer are required"}
	}
	question, err := db.GetQuestionByID(questionID)
	if err != nil {
		return map[string]interface{}{"error": "failed to fetch question: " + err.Error()}
	}
	if question == nil {
		return map[string]interface{}{"error": "question not found"}
	}
	expected := normalizeQuizAnswer(question.CorrectAnswer, question.Options)
	actual := normalizeQuizAnswer(userAnswer, question.Options)
	correct := expected != "" && expected == actual
	hint := question.Hint
	if hint == "" {
		hint = "Review the cited section and compare each option against the source."
	}
	score := models.QuizScore{
		QuestionID: question.ID, Correct: correct, Score: 0,
		Expected: question.CorrectAnswer, Feedback: question.Explanation,
		Hint: hint, UserAnswer: userAnswer, SourceHeading: question.SourceHeading,
	}
	if correct {
		score.Score = 100
		if score.Hint == "Review the cited section and compare each option against the source." {
			score.Hint = "Great job. Move to the next question."
		}
	} else if strings.TrimSpace(question.Explanation) == "" {
		score.Feedback = "That answer is not correct."
	}
	tx, err := db.GetConnection().Begin()
	if err != nil {
		return map[string]interface{}{"error": "failed to begin transaction: " + err.Error()}
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()
	if err := db.SaveUserAnswerTx(tx, score); err != nil {
		return map[string]interface{}{"error": "failed to save score: " + err.Error()}
	}
	if a.studyService == nil {
		return map[string]interface{}{"error": "study service not initialized"}
	}
	fsrsResult, err := a.studyService.LogReviewTx(tx, question.TopicID, "quiz_question", question.ID, question.SourceChunkID, score.Score)
	if err != nil {
		return map[string]interface{}{"error": "failed to update quiz FSRS: " + err.Error()}
	}
	if err := tx.Commit(); err != nil {
		return map[string]interface{}{"error": "failed to commit transaction: " + err.Error()}
	}
	committed = true
	return map[string]interface{}{
		"question_id": score.QuestionID, "correct": score.Correct,
		"score": score.Score, "expected": score.Expected,
		"feedback": score.Feedback, "hint": score.Hint,
		"user_answer": score.UserAnswer, "source_heading": score.SourceHeading,
		"fsrsRating": fsrsResult["fsrs_rating"], "scheduled_days": fsrsResult["scheduled_days"],
		"next_review_at": fsrsResult["next_review_at"], "review_log_id": fsrsResult["review_log_id"],
	}
}

func (a *App) LogReview(topicID, activityType, referenceID, sourceChunkID string, score int) map[string]interface{} {
	if a.studyService == nil {
		return map[string]interface{}{"error": "study service not initialized"}
	}
	if err := a.studyService.LogReview(topicID, activityType, referenceID, sourceChunkID, score); err != nil {
		return map[string]interface{}{"error": err.Error()}
	}
	return map[string]interface{}{"ok": true}
}

func (a *App) GenerateShortAnswerPrompt(topicID string) map[string]interface{} {
	if a.studyService == nil {
		return map[string]interface{}{"error": "study service not initialized"}
	}
	return a.studyService.GenerateShortAnswerPrompt(topicID)
}

func (a *App) ScoreShortAnswer(questionID, userAnswer string) map[string]interface{} {
	if a.studyService == nil {
		return map[string]interface{}{"error": "study service not initialized"}
	}
	return a.studyService.ScoreShortAnswer(questionID, userAnswer)
}

func mapReviewRating(rating string) (int, bool) {
	switch rating {
	case "again":
		return scheduler.Again, true
	case "hard":
		return scheduler.Hard, true
	case "good":
		return scheduler.Good, true
	case "easy":
		return scheduler.Easy, true
	default:
		return 0, false
	}
}

func normalizeQuizAnswer(answer string, options []string) string {
	ans := strings.TrimSpace(strings.ToLower(answer))
	if ans == "" {
		return ""
	}
	if len(ans) == 1 {
		idx := int(ans[0] - 'a')
		if idx >= 0 && idx < len(options) {
			return strings.ToLower(strings.TrimSpace(options[idx]))
		}
	}
	return ans
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

func notebookAssetURL(filePath string) string {
	normPath := strings.TrimSpace(strings.ReplaceAll(filePath, "\\", "/"))
	if normPath == "" || normPath == "." || normPath == ".." {
		return ""
	}
	name := strings.TrimSpace(path.Base(normPath))
	if name == "" || name == "." || name == ".." {
		return ""
	}
	return "/notebooks/" + url.PathEscape(name)
}
