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

	// Init shared retrieval engine for Socratic + Reader scoped chat.
	a.retrievalEngine = retrieval.NewEngine(embedder)

	// Init topic-scoped RAG pipeline (used by AskAI).
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

func (a *App) AskReaderAI(topicID, notebookID, question, scope string, currentPage, chapterStartPage, chapterEndPage int) map[string]interface{} {
	if !a.aiReady {
		reason := a.aiInitError
		if reason == "" {
			reason = "local AI runtime is not ready"
		}
		return map[string]interface{}{"error": "Reader AI unavailable: " + reason}
	}
	if a.studyService == nil {
		return map[string]interface{}{"error": "study service not initialized"}
	}
	return a.studyService.AnswerReaderQuestion(study.ReaderAIRequest{
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

	// Canonical queue recovery/materialization path for dashboard:
	// if ACTIVE/PENDING queue tasks exist, surface those directly.
	activeQueueTasks, err := db.GetAllActiveTasks()
	if err != nil {
		return map[string]interface{}{"error": err.Error()}
	}
	pendingQueueTasks, err := db.GetAllPendingTasks()
	if err != nil {
		return map[string]interface{}{"error": err.Error()}
	}

	var plan *models.TodayPlan
	if len(activeQueueTasks) > 0 || len(pendingQueueTasks) > 0 {
		// Bypass scheduler's synthetic BuildTodayPlan to save DB scan and token budget cycles.
		// Query due review cards and daily minutes directly.
		dueCards, err := db.QueryDueReviewCards(now.Unix())
		if err != nil {
			return map[string]interface{}{"error": err.Error()}
		}
		dailyStudyMinutes, err := db.GetDailyStudyMinutes()
		if err != nil {
			return map[string]interface{}{"error": err.Error()}
		}
		if dailyStudyMinutes <= 0 {
			dailyStudyMinutes = scheduler.DefaultDailyStudyMinutes
		}

		totalDueCards := dueCards
		reviewBudget := int(math.Ceil(float64(dueCards) * scheduler.ReviewMinutesPerCard))
		proportionalCap := int(float64(dailyStudyMinutes) * scheduler.MaxReviewMinutesRatio)
		safeReviewBudget := reviewBudget
		if safeReviewBudget > proportionalCap {
			safeReviewBudget = proportionalCap
		}
		if safeReviewBudget > scheduler.MaxReviewMinutesSession {
			safeReviewBudget = scheduler.MaxReviewMinutesSession
		}
		materializedCards := int(float64(safeReviewBudget) / scheduler.ReviewMinutesPerCard)
		if materializedCards > dueCards {
			materializedCards = dueCards
		}
		deferredCards := totalDueCards - materializedCards
		if deferredCards < 0 {
			deferredCards = 0
		}

		queueTasks := make([]models.ScheduledTask, 0, len(activeQueueTasks)+len(pendingQueueTasks))
		actionCounts := make(map[string]int)
		activeTopicsMap := make(map[string]bool)

		for _, q := range activeQueueTasks {
			task := queueTaskToScheduledTask(q)
			queueTasks = append(queueTasks, task)
			actionCounts[task.ActionType]++
			if q.Title != "" {
				activeTopicsMap[q.Title] = true
			}
		}
		for _, q := range pendingQueueTasks {
			task := queueTaskToScheduledTask(q)
			queueTasks = append(queueTasks, task)
			actionCounts[task.ActionType]++
			if q.Title != "" {
				activeTopicsMap[q.Title] = true
			}
		}

		activeTopics := make([]string, 0, len(activeTopicsMap))
		for topicTitle := range activeTopicsMap {
			activeTopics = append(activeTopics, topicTitle)
		}

		learningMinutes := 0
		for _, task := range queueTasks {
			learningMinutes += task.EstimateMinutes
		}

		plan = &models.TodayPlan{
			Date:                now.Format("2006-01-02"),
			TotalMinutes:        dailyStudyMinutes,
			ReviewMinutes:       safeReviewBudget,
			LearningMinutes:     learningMinutes,
			DueReviewCards:      materializedCards,
			TotalDueReviewCards: totalDueCards,
			DeferredReviewCards: deferredCards,
			ActiveTopics:        activeTopics,
			Tasks:               queueTasks,
			IsEstimate:          false,
		}

		utils.Warnf("[TODAY_PLAN] queue materialization active=%d pending=%d merged=%d", len(activeQueueTasks), len(pendingQueueTasks), len(queueTasks))
		utils.Warnf("[TODAY_PLAN] planner aggregation dueReviewCards=%d reviewMinutes=%d queueActionCounts=%v", plan.DueReviewCards, plan.ReviewMinutes, actionCounts)
		if actionCounts["flashcard_review"] > 0 {
			utils.Warnf("[FLASHCARD_PIPELINE] today_plan_review_detected flashcard_review_count=%d", actionCounts["flashcard_review"])
		}
	} else {
		// Fallback to synthetic scheduler plan if queue is empty
		syntheticPlan, err := a.scheduler.BuildTodayPlan(now)
		if err != nil {
			return map[string]interface{}{"error": err.Error()}
		}
		plan = syntheticPlan
		utils.Warnf("[TODAY_PLAN] synthetic plan fallback taskCount=%d", len(plan.Tasks))
	}

	utils.Warnf("[TODAY_PLAN] GetTodayPlan response tasks=%d isEstimate=%t reviewMinutes=%d learningMinutes=%d", len(plan.Tasks), plan.IsEstimate, plan.ReviewMinutes, plan.LearningMinutes)
	for idx, task := range plan.Tasks {
		utils.Warnf("[TODAY_PLAN] GetTodayPlan task[%d] taskID=%s actionType=%s topicID=%s notebookID=%s startPage=%d endPage=%d priority=%d", idx, task.ID, task.ActionType, task.TopicID, task.NotebookID, task.StartPage, task.EndPage, task.Priority)
	}

	return map[string]interface{}{
		"date": plan.Date, "total_minutes": plan.TotalMinutes,
		"review_minutes": plan.ReviewMinutes, "learning_minutes": plan.LearningMinutes,
		"due_review_cards": plan.DueReviewCards, "total_due_review_cards": plan.TotalDueReviewCards,
		"deferred_review_cards": plan.DeferredReviewCards, "active_topics": plan.ActiveTopics,
		"tasks": plan.Tasks, "generated_at_unix": now.Unix(),
		"data_fresh": true, "is_estimate": plan.IsEstimate,
		"insights_available": false, "plan_source": "queue-materialized",
	}
}

func queueTaskToScheduledTask(task models.StudyQueueTask) models.ScheduledTask {
	actionType := strings.ToLower(string(task.TaskType))
	titleBase := strings.TrimSpace(task.Title)
	if titleBase == "" {
		titleBase = "Task"
	}

	titlePrefix := "Task"
	switch task.TaskType {
	case models.StudyTaskTypeReading:
		titlePrefix = "Read"
	case models.StudyTaskTypeQuiz:
		titlePrefix = "Quiz"
	case models.StudyTaskTypeReread:
		titlePrefix = "Reread"
	case models.StudyTaskTypeFlashcardReview:
		titlePrefix = "Flashcard Review"
	case models.StudyTaskTypeExaminer:
		titlePrefix = "Examiner"
	}

	meta := ""
	if task.StartPage > 0 && task.EndPage > 0 {
		meta = fmt.Sprintf("Pages %d-%d", task.StartPage, task.EndPage)
	}
	estimateMinutes := 10
	if task.TaskType == models.StudyTaskTypeFlashcardReview {
		// Use card count from payload if available
		var payload models.ReviewSessionPayload
		if err := json.Unmarshal([]byte(task.PayloadJSON), &payload); err == nil && payload.CardCount > 0 {
			estimateMinutes = int(math.Ceil(float64(payload.CardCount) * scheduler.ReviewMinutesPerCard))
		}
	} else if task.StartPage > 0 && task.EndPage >= task.StartPage {
		estimateMinutes = int(float64(task.EndPage-task.StartPage+1) * scheduler.MinutesPerPage)
	}

	return models.ScheduledTask{
		ID:              task.ID,
		ActionType:      actionType,
		Title:           fmt.Sprintf("%s: %s", titlePrefix, titleBase),
		TopicID:         task.TopicID,
		NotebookID:      task.NotebookID,
		StartPage:       task.StartPage,
		EndPage:         task.EndPage,
		EstimateMinutes: estimateMinutes,
		Priority:        task.Priority,
		Meta:            meta,
	}
}

func (a *App) ActivateTask(taskID string) map[string]interface{} {
	if taskID == models.ReviewTaskDailyID {
		return map[string]interface{}{"ok": true}
	}
	if task, err := db.GetTaskByID(taskID); err == nil {
		utils.Warnf("[QUEUE] ActivateTask precheck taskID=%s status=%s type=%s notebookID=%s topicID=%s", taskID, task.Status, task.TaskType, task.NotebookID, task.TopicID)
	} else {
		utils.Warnf("[QUEUE] ActivateTask precheck taskID=%s loadError=%v", taskID, err)
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
	utils.Warnf("[READER_INIT] InitializeReadingSession entry taskID=%s notebookID=%s topicID=%s startPage=%d endPage=%d", taskID, notebookID, topicID, startPage, endPage)

	seedTaskID := taskID
	rematerialized := false
	rematerializedFrom := ""
	rematerializedTo := ""
	existingTask, existingErr := db.GetTaskByID(seedTaskID)

	// If task doesn't exist yet (e.g. scheduler-generated synthetic ID),
	// insert it as a real READING task so the queue lifecycle can proceed.
	if existingErr == db.ErrTaskNotFound {
		utils.Warnf("[READER_INIT] InitializeReadingSession task missing, creating pending reading task taskID=%s notebookID=%s topicID=%s", taskID, notebookID, topicID)
		if notebookID == "" || topicID == "" {
			return map[string]interface{}{"error": "task not found and notebookID/topicID required to create it", "code": 400}
		}
		insertErr := db.InsertStudyTask(models.StudyQueueTask{
			ID:         seedTaskID,
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
		existingTask = &models.StudyQueueTask{
			ID:       seedTaskID,
			Status:   models.StudyTaskStatusPending,
			TaskType: models.StudyTaskTypeReading,
		}
	} else if existingErr != nil {
		return map[string]interface{}{"error": existingErr.Error()}
	}

	// Never reopen terminal queue rows. If deterministic scheduler ID collides with
	// an already completed/failed/skipped task, materialize a fresh queue row identity.
	if existingTask != nil && existingTask.Status != models.StudyTaskStatusPending && existingTask.Status != models.StudyTaskStatusActive {
		if notebookID == "" {
			notebookID = existingTask.NotebookID
		}
		if topicID == "" {
			topicID = existingTask.TopicID
		}
		if notebookID == "" || topicID == "" {
			return map[string]interface{}{"error": "terminal task cannot be reused and notebookID/topicID were not available", "code": 409}
		}
		taskID = uuid.NewString()
		rematerialized = true
		rematerializedFrom = seedTaskID
		rematerializedTo = taskID
		utils.Warnf("[READER_INIT] InitializeReadingSession task terminal, creating new queue row oldTaskID=%s newTaskID=%s oldStatus=%s notebookID=%s topicID=%s", seedTaskID, taskID, existingTask.Status, notebookID, topicID)
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
			return map[string]interface{}{"error": "failed to create replacement reading task: " + insertErr.Error()}
		}
	}

	// Activate task (idempotent if already active)
	if task, err := db.GetTaskByID(taskID); err == nil {
		utils.Warnf("[READER_INIT] InitializeReadingSession queue task before activate taskID=%s status=%s type=%s notebookID=%s topicID=%s", taskID, task.Status, task.TaskType, task.NotebookID, task.TopicID)
	} else {
		utils.Warnf("[READER_INIT] InitializeReadingSession queue task pre-activate load error taskID=%s err=%v", taskID, err)
	}
	if err := db.ActivateTask(taskID); err != nil {
		utils.Warnf("[READER_INIT] InitializeReadingSession activate result taskID=%s err=%v", taskID, err)
	} else {
		utils.Warnf("[READER_INIT] InitializeReadingSession activate result taskID=%s ok=true", taskID)
	}

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
	utils.Warnf("[READER_INIT] InitializeReadingSession response payload canonicalTaskID=%s rematerialized=%t oldTaskID=%s newTaskID=%s", task.TaskID, rematerialized, rematerializedFrom, rematerializedTo)

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
		},
	}
}

func (a *App) CompleteReading(taskID string) map[string]interface{} {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		utils.Warnf("[COMPLETE_SESSION] CompleteReading entry rejected: taskID empty")
		return map[string]interface{}{"error": "task ID is required", "code": 400}
	}
	utils.Warnf("[COMPLETE_SESSION] CompleteReading entry taskID=%s", taskID)

	// Trust-based completion: just validate task exists and is active
	task, err := db.GetReadingTask(taskID)
	if err != nil {
		switch err {
		case db.ErrTaskNotFound:
			utils.Warnf("[COMPLETE_SESSION] CompleteReading GetReadingTask error: task not found taskID=%s", taskID)
			return map[string]interface{}{"error": "ErrNotFound", "code": 404}
		default:
			utils.Warnf("[COMPLETE_SESSION] CompleteReading GetReadingTask error taskID=%s err=%v", taskID, err)
			return map[string]interface{}{"error": err.Error()}
		}
	}
	utils.Warnf("[COMPLETE_SESSION] CompleteReading loaded reading task taskID=%s startPage=%d endPage=%d currentPage=%d", taskID, task.StartPage, task.EndPage, task.CurrentPage)

	if a.studyService == nil {
		utils.Warnf("[COMPLETE_SESSION] CompleteReading error: study service not initialized taskID=%s", taskID)
		return map[string]interface{}{"error": "study service not initialized"}
	}

	// Generate quiz from full assigned chunk range (no page validation)
	chunks, err := db.GetChunksForTopicPageRange(task.TopicID, task.StartPage, task.EndPage)
	if err != nil {
		utils.Warnf("[COMPLETE_SESSION] CompleteReading chunk lookup error taskID=%s err=%v", taskID, err)
		return map[string]interface{}{"error": err.Error()}
	}

	chunkIDs := make([]string, 0, len(chunks))
	chunkTextByID := make(map[string]string, len(chunks))
	for _, chunk := range chunks {
		chunkIDs = append(chunkIDs, chunk.ID)
		chunkTextByID[chunk.ID] = chunk.Text
	}

	utils.Warnf("[QUIZ] CompleteReading before GenerateQuizSync taskID=%s topicID=%s chunkCount=%d", taskID, task.TopicID, len(chunkIDs))
	quizPayload, err := a.studyService.GenerateQuizSync(task.TopicID, chunkIDs, chunkTextByID)
	if err != nil {
		utils.Warnf("[QUIZ] CompleteReading GenerateQuizSync error taskID=%s err=%v", taskID, err)
		return map[string]interface{}{"error": err.Error()}
	}
	utils.Warnf("[QUIZ] CompleteReading after GenerateQuizSync taskID=%s questionCount=%d", taskID, len(quizPayload.Questions))

	// Complete reading task and generate follow-up quiz
	// No page completion validation required - user decides when done
	utils.Warnf("[COMPLETE_SESSION] CompleteReading before CompleteReadingWithGeneratedQuiz taskID=%s", taskID)
	quizTaskID, err := db.CompleteReadingWithGeneratedQuiz(taskID, quizPayload)
	if err != nil {
		switch err {
		case db.ErrTaskNotFound:
			utils.Warnf("[COMPLETE_SESSION] CompleteReading CompleteReadingWithGeneratedQuiz error: task not found taskID=%s", taskID)
			return map[string]interface{}{"error": "ErrNotFound", "code": 404}
		case db.ErrTaskNotActive:
			utils.Warnf("[COMPLETE_SESSION] CompleteReading CompleteReadingWithGeneratedQuiz error: task not active taskID=%s", taskID)
			return map[string]interface{}{"error": "ErrTaskNotActive", "code": 409}
		default:
			utils.Warnf("[COMPLETE_SESSION] CompleteReading CompleteReadingWithGeneratedQuiz error taskID=%s err=%v", taskID, err)
			return map[string]interface{}{"error": err.Error()}
		}
	}
	utils.Warnf("[COMPLETE_SESSION] CompleteReading CompleteReadingWithGeneratedQuiz result taskID=%s quizTaskID=%s", taskID, quizTaskID)
	utils.Warnf("[FLASHCARD_PIPELINE] flashcard_generation_trigger check stage=reading_completed taskID=%s topicID=%s result=not_triggered reason=no_flashcard_hook_in_complete_reading", taskID, task.TopicID)
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

// GenerateFlashcardsForQuizTask generates flashcards based on a passed quiz task.
// Newly generated cards are future-dated and do not create an immediate review task.
func (a *App) GenerateFlashcardsForQuizTask(taskID string) map[string]interface{} {
	if a.studyService == nil {
		return map[string]interface{}{"error": "study service not initialized"}
	}

	task, err := db.GetTaskByID(taskID)
	if err != nil {
		return map[string]interface{}{"error": err.Error()}
	}
	if task.TaskType != models.StudyTaskTypeQuiz {
		return map[string]interface{}{"error": "task is not a quiz"}
	}

	utils.Warnf("[FLASHCARD_PIPELINE] continue_button_flashcard_generation_started taskID=%s topicID=%s notebookID=%s", taskID, task.TopicID, task.NotebookID)

	cardCount, err := a.studyService.GenerateFlashcardsAfterQuiz(task.NotebookID, task.TopicID, task.StartPage, task.EndPage)
	if err != nil {
		utils.Warnf("[FLASHCARD_PIPELINE] flashcard_generation_failed taskID=%s reason=%v", taskID, err)
		return map[string]interface{}{"error": "failed to generate flashcards: " + err.Error()}
	}
	utils.Warnf("[FLASHCARD_PIPELINE] flashcard_generation_completed taskID=%s reviewTaskID=%s cardsScheduled=%d", taskID, "", cardCount)
	utils.Warnf("[DASHBOARD] dashboard_redirect_after_generation taskID=%s reviewTaskID=%s cardsScheduled=%d", taskID, "", cardCount)

	return map[string]interface{}{
		"review_task_id":    "",
		"cards_scheduled":   cardCount,
		"flashcards_gen_ok": true,
	}
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

func (a *App) GetReviewSession(taskID string, notebookID string) map[string]interface{} {
	if a.studyService == nil {
		return map[string]interface{}{"error": "study service not initialized"}
	}

	// Materialize synthetic tasks from the scheduler
	if taskID == models.ReviewTaskDailyID {
		requestedNotebookID := notebookID
		utils.Warnf("[FLASHCARD_PIPELINE] GetReviewSession materializing synthetic task notebookID=%s", notebookID)
		if notebookID == "" {
			resolvedNotebookID, dueCount, err := db.GetNextDueReviewNotebook(time.Now().Unix())
			if err != nil {
				return map[string]interface{}{"error": "Failed to resolve notebook for review materialization: " + err.Error()}
			}
			notebookID = resolvedNotebookID
			if notebookID != "" {
				utils.Warnf("[FLASHCARD_PIPELINE] synthetic_review_notebook_selected notebookID=%s dueCards=%d source=review_materialization", notebookID, dueCount)
			}
		}
		utils.Warnf("[FLASHCARD_PIPELINE] review_materialization_notebook_resolution taskID=%s requestedNotebookID=%s resolvedNotebookID=%s", taskID, requestedNotebookID, notebookID)

		if notebookID == "" {
			return map[string]interface{}{"error": "No due cards found for review materialization"}
		}

		// CreateReviewSession materializes a review session in the DB for the given notebookID.
		// Returns the session Task (with a newly-created ID) and a boolean indicating if an existing legacy session was reused.
		task, reused, err := db.CreateReviewSession(notebookID)
		if err != nil {
			return map[string]interface{}{"error": "Failed to materialize review session: " + err.Error()}
		}
		utils.Warnf("[FLASHCARD_PIPELINE] GetReviewSession materialized notebookID=%s taskID=%s reused=%t", notebookID, task.ID, reused)
		taskID = task.ID
	}

	session, err := a.studyService.GetReviewSession(taskID)
	if err != nil {
		switch err {
		case db.ErrTaskNotFound:
			return map[string]interface{}{"error": "ErrNotFound", "code": 404}
		default:
			return map[string]interface{}{"error": err.Error()}
		}
	}
	return map[string]interface{}{"session": session}
}

func (a *App) RecordCardReview(taskID, cardID string, rating int) map[string]interface{} {
	if a.studyService == nil {
		return map[string]interface{}{"error": "study service not initialized"}
	}
	remaining, err := a.studyService.RecordCardReview(taskID, cardID, rating)
	if err != nil {
		switch err {
		case db.ErrTaskNotFound:
			return map[string]interface{}{"error": "ErrNotFound", "code": 404}
		case db.ErrTaskNotActive:
			return map[string]interface{}{"error": "ErrTaskNotActive", "code": 409}
		case db.ErrReviewLinkNotPending:
			return map[string]interface{}{"error": "ErrCardAlreadyReviewed", "code": 409}
		default:
			return map[string]interface{}{"error": err.Error()}
		}
	}
	return map[string]interface{}{"ok": true, "remaining": remaining}
}

func (a *App) CompleteReviewSession(taskID string) map[string]interface{} {
	if a.studyService == nil {
		return map[string]interface{}{"error": "study service not initialized"}
	}
	if err := a.studyService.CompleteReviewSession(taskID); err != nil {
		switch err {
		case db.ErrTaskNotFound:
			return map[string]interface{}{"error": "ErrNotFound", "code": 404}
		case db.ErrTaskNotActive:
			return map[string]interface{}{"error": "ErrTaskNotActive", "code": 409}
		case db.ErrReviewSessionOpen:
			return map[string]interface{}{"error": "ErrReviewSessionIncomplete", "code": 409}
		default:
			return map[string]interface{}{"error": err.Error()}
		}
	}
	return map[string]interface{}{"ok": true}
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

func resolveAppDir() (string, error) {
	var appDir string

	// If APP_ENV is set to dev, use a local folder in the project root
	if os.Getenv("APP_ENV") == "dev" {
		// Resolve stable project root instead of using relative path
		projectRoot, err := os.Executable()
		if err != nil {
			// Fallback to current working directory if executable path fails
			projectRoot, err = os.Getwd()
			if err != nil {
				return "", fmt.Errorf("failed to resolve project root: %w", err)
			}
		}
		projectRoot = filepath.Dir(projectRoot)
		appDir = filepath.Join(projectRoot, "dev_data")
	} else {
		// Otherwise, use the standard system config directory (AppData)
		baseDir, err := os.UserConfigDir()
		if err != nil {
			return "", err
		}
		appDir = filepath.Join(baseDir, "ai-tutor")
	}

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
