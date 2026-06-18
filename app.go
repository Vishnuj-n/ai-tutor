package main

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"strings"
	"sync"
	"time"
	"database/sql"

	"ai-tutor/internal/db"
	"ai-tutor/internal/embeddings"
	"ai-tutor/internal/llm"
	"ai-tutor/internal/models"
	"ai-tutor/internal/notebook"
	"ai-tutor/internal/retrieval"
	"ai-tutor/internal/runtime"
	"ai-tutor/internal/scheduler"
	"ai-tutor/internal/study"
	"ai-tutor/internal/utils"

	"github.com/google/uuid"
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
}

func (a *App) Greet(name string) string {
	return fmt.Sprintf("Hello %s, It's show time!", name)
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

func (a *App) AskSocratic(notebookID string, topicID string, question string) map[string]interface{} {
	repo := a.getRepo()
	if repo == nil {
		return map[string]interface{}{"error": "database repository not initialized"}
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

func (a *App) GetTodayPlan() map[string]interface{} {
	repo := a.getRepo()
	if repo == nil {
		return map[string]interface{}{"error": "database repository not initialized"}
	}
	if a.scheduler == nil {
		return map[string]interface{}{"error": "scheduler not initialized"}
	}
	now := time.Now()

	// Canonical queue recovery/materialization path for dashboard:
	// if ACTIVE/PENDING queue tasks exist, surface those directly.
	activeQueueTasks, err := repo.GetAllActiveTasks()
	if err != nil {
		return map[string]interface{}{"error": err.Error()}
	}
	pendingQueueTasks, err := repo.GetAllPendingTasks()
	if err != nil {
		return map[string]interface{}{"error": err.Error()}
	}

	var plan *models.TodayPlan
	planSource := "queue-materialized"
	if len(activeQueueTasks) > 0 || len(pendingQueueTasks) > 0 {
		// Bypass scheduler's synthetic BuildTodayPlan to save DB scan and token budget cycles.
		// Query due review cards and daily minutes directly.
		dueCards, err := repo.QueryDueReviewCards(now.Unix())
		if err != nil {
			return map[string]interface{}{"error": err.Error()}
		}
		dailyStudyMinutes, err := repo.GetDailyStudyMinutes()
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
		planSource = "scheduler-fallback"
		utils.Warnf("[TODAY_PLAN] synthetic plan fallback taskCount=%d", len(plan.Tasks))
	}

	// Count active notebooks for the dashboard empty-state distinction.
	var activeNotebookCount int
	var activeProfileID sql.NullString
	if err := repo.GetConnection().QueryRow(`SELECT COALESCE(active_profile_id, '') FROM user_settings WHERE id = 1`).Scan(&activeProfileID); err == nil && activeProfileID.Valid && activeProfileID.String != "" {
		_ = repo.GetConnection().QueryRow(`
			SELECT COUNT(*) FROM notebooks 
			WHERE study_status = 'active' 
			  AND (profile_id = ? OR profile_id IS NULL OR profile_id = '')
		`, activeProfileID.String).Scan(&activeNotebookCount)
	} else {
		_ = repo.GetConnection().QueryRow(`SELECT COUNT(*) FROM notebooks WHERE study_status = 'active'`).Scan(&activeNotebookCount)
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
		"insights_available": false, "plan_source": planSource,
		"active_notebook_count": activeNotebookCount,
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
	repo := a.getRepo()
	if repo == nil {
		return map[string]interface{}{"error": "database repository not initialized"}
	}
	if taskID == models.ReviewTaskDailyID {
		return map[string]interface{}{"ok": true}
	}
	if task, err := repo.GetTaskByID(taskID); err == nil {
		utils.Warnf("[QUEUE] ActivateTask precheck taskID=%s status=%s type=%s notebookID=%s topicID=%s", taskID, task.Status, task.TaskType, task.NotebookID, task.TopicID)
	} else {
		utils.Warnf("[QUEUE] ActivateTask precheck taskID=%s loadError=%v", taskID, err)
	}
	if err := repo.ActivateTask(taskID); err != nil {
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
	repo := a.getRepo()
	if repo == nil {
		return map[string]interface{}{"error": "database repository not initialized"}
	}
	if strings.TrimSpace(taskID) == "" {
		return map[string]interface{}{"error": "task ID is required", "code": 400}
	}
	if err := repo.CompleteTask(taskID, result); err != nil {
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
	repo := a.getRepo()
	if repo == nil {
		return map[string]interface{}{"error": "database repository not initialized"}
	}
	if strings.TrimSpace(taskID) == "" {
		return map[string]interface{}{"error": "task ID is required", "code": 400}
	}
	if err := repo.SkipTask(taskID); err != nil {
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
	repo := a.getRepo()
	if repo == nil {
		return map[string]interface{}{"error": "database repository not initialized"}
	}
	if strings.TrimSpace(notebookID) == "" {
		return map[string]interface{}{"error": "notebook ID is required", "code": 400}
	}
	state, err := repo.GetQueueState(notebookID)
	if err != nil {
		return map[string]interface{}{"error": err.Error()}
	}
	return map[string]interface{}{"queue_state": state}
}

// InitializeReadingSession consolidates task activation, reading task loading,
// and page bounds resolution into a single canonical backend call.
// Accepts the full routing context so scheduler-suggested tasks (not yet in study_queue)
// can be materialized as real queue rows on first open.
func (a *App) InitializeReadingSession(taskID, notebookID, topicID string, startPage, endPage int) map[string]interface{} {
	repo := a.getRepo()
	if repo == nil {
		return map[string]interface{}{"error": "database repository not initialized"}
	}
	taskID = strings.TrimSpace(taskID)
	notebookID = strings.TrimSpace(notebookID)
	topicID = strings.TrimSpace(topicID)
	if taskID == "" {
		return map[string]interface{}{"error": "task ID is required", "code": 400}
	}
	utils.Warnf("[READER_INIT] InitializeReadingSession entry taskID=%s notebookID=%s topicID=%s startPage=%d endPage=%d", taskID, notebookID, topicID, startPage, endPage)

	seedTaskID := taskID
	existingTask, existingErr := repo.GetTaskByID(seedTaskID)

	// If task doesn't exist yet (e.g. scheduler-generated synthetic ID),
	// insert it as a real READING task so the queue lifecycle can proceed.
	if existingErr == db.ErrTaskNotFound {
		utils.Warnf("[READER_INIT] InitializeReadingSession task missing, creating pending reading task taskID=%s notebookID=%s topicID=%s", taskID, notebookID, topicID)
		if notebookID == "" || topicID == "" {
			return map[string]interface{}{"error": "task not found and notebookID/topicID required to create it", "code": 400}
		}
		insertErr := repo.InsertStudyTask(models.StudyQueueTask{
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
		utils.Warnf("[READER_INIT] InitializeReadingSession task terminal, creating new queue row taskID=%s oldStatus=%s notebookID=%s topicID=%s", taskID, existingTask.Status, notebookID, topicID)
		insertErr := repo.InsertStudyTask(models.StudyQueueTask{
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
	if task, err := repo.GetTaskByID(taskID); err == nil {
		utils.Warnf("[READER_INIT] InitializeReadingSession queue task before activate taskID=%s status=%s type=%s notebookID=%s topicID=%s", taskID, task.Status, task.TaskType, task.NotebookID, task.TopicID)
	} else {
		utils.Warnf("[READER_INIT] InitializeReadingSession queue task pre-activate load error taskID=%s err=%v", taskID, err)
	}
	if err := repo.ActivateTask(taskID); err != nil {
		utils.Warnf("[READER_INIT] InitializeReadingSession activate result taskID=%s err=%v", taskID, err)
	} else {
		utils.Warnf("[READER_INIT] InitializeReadingSession activate result taskID=%s ok=true", taskID)
	}

	// Load reading task with all context
	task, err := repo.GetReadingTask(taskID)
	if err != nil {
		if err == db.ErrTaskNotFound {
			return map[string]interface{}{"error": "ErrNotFound", "code": 404}
		}
		return map[string]interface{}{"error": err.Error()}
	}

	// Get topic bundle for additional metadata
	bundle, err := repo.GetReaderTopicBundle(task.TopicID, task.NotebookID)
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
	err = repo.GetConnection().QueryRow(`
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
	utils.Warnf("[READER_INIT] InitializeReadingSession response payload canonicalTaskID=%s", task.TaskID)

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
	repo := a.getRepo()
	if repo == nil {
		return map[string]interface{}{"error": "database repository not initialized"}
	}
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		utils.Warnf("[COMPLETE_SESSION] CompleteReading entry rejected: taskID empty")
		return map[string]interface{}{"error": "task ID is required", "code": 400}
	}
	utils.Warnf("[COMPLETE_SESSION] CompleteReading entry taskID=%s", taskID)

	// Trust-based completion: just validate task exists and is active
	task, err := repo.GetReadingTask(taskID)
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
	chunks, err := repo.GetChunksForTopicPageRange(task.TopicID, task.StartPage, task.EndPage)
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
	quizTaskID, err := repo.CompleteReadingWithGeneratedQuiz(taskID, quizPayload)
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
	repo := a.getRepo()
	if repo == nil {
		return map[string]interface{}{"error": "database repository not initialized"}
	}
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return map[string]interface{}{"error": "task ID is required", "code": 400}
	}
	task, err := repo.GetTaskByID(taskID)
	if err != nil {
		if err == db.ErrTaskNotFound {
			return map[string]interface{}{"error": "ErrNotFound", "code": 404}
		}
		return map[string]interface{}{"error": err.Error()}
	}
	return map[string]interface{}{"task": task}
}

func (a *App) GenerateQuizForPageRange(notebookID string, startPage, endPage int) map[string]interface{} {
	repo := a.getRepo()
	if repo == nil {
		return map[string]interface{}{"error": "database repository not initialized"}
	}
	if a.studyService == nil {
		return map[string]interface{}{"error": "study service not initialized"}
	}
	return a.studyService.GenerateQuizForPageRange(notebookID, startPage, endPage)
}

func (a *App) SubmitQuizAttempt(taskID string, answers []models.QuizAnswer) map[string]interface{} {
	repo := a.getRepo()
	if repo == nil {
		return map[string]interface{}{"error": "database repository not initialized"}
	}
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
	repo := a.getRepo()
	if repo == nil {
		return map[string]interface{}{"error": "database repository not initialized"}
	}
	if a.studyService == nil {
		return map[string]interface{}{"error": "study service not initialized"}
	}

	task, err := repo.GetTaskByID(taskID)
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
	repo := a.getRepo()
	if repo == nil {
		return map[string]interface{}{"error": "database repository not initialized"}
	}
	minutes, err := repo.GetDailyStudyMinutes()
	if err != nil {
		return map[string]interface{}{"error": err.Error()}
	}
	return map[string]interface{}{"daily_study_minutes": minutes}
}

func (a *App) UpdateDailyStudyMinutes(minutes int) map[string]interface{} {
	repo := a.getRepo()
	if repo == nil {
		return map[string]interface{}{"error": "database repository not initialized"}
	}
	if minutes < 15 || minutes > 480 {
		return map[string]interface{}{"error": "daily study minutes must be between 15 and 480"}
	}
	if err := repo.UpsertDailyStudyMinutes(minutes); err != nil {
		return map[string]interface{}{"error": err.Error()}
	}
	return map[string]interface{}{"ok": true, "daily_study_minutes": minutes}
}
func (a *App) GetUserSettings() map[string]interface{} {
	repo := a.getRepo()
	if repo == nil {
		return map[string]interface{}{"error": "database repository not initialized"}
	}
	s, err := repo.GetUserSettings()
	if err != nil {
		return map[string]interface{}{"error": err.Error()}
	}
	return map[string]interface{}{
		"daily_study_minutes":    s.DailyStudyMinutes,
		"active_profile_id":      s.ActiveProfileID,
		"skip_to_reading_active": s.SkipToReadingActive,
		"cloud_sync_url":         s.CloudSyncURL,
		"cloud_api_token":        s.CloudAPIToken,
		"theme":                  s.Theme,
		"rag_enabled":            s.RAGEnabled,
		"rag_notebook_chapter":   s.RAGNotebookChapter,
		"rag_entire_notebook":    s.RAGEntireNotebook,
		"rag_queue_study":        s.RAGQueueStudy,
	}
}

func (a *App) UpdateUserSettings(minutes int, activeProfileID string, skipToReading bool, syncURL, apiToken string, theme string, ragEnabled bool, ragNotebookChapter bool, ragEntireNotebook bool, ragQueueStudy bool) map[string]interface{} {
	repo := a.getRepo()
	if repo == nil {
		return map[string]interface{}{"error": "database repository not initialized"}
	}
	if minutes < 15 || minutes > 480 {
		return map[string]interface{}{"error": "daily study minutes must be between 15 and 480"}
	}
	s := models.UserSettings{
		DailyStudyMinutes:   minutes,
		ActiveProfileID:     activeProfileID,
		SkipToReadingActive: skipToReading,
		CloudSyncURL:        syncURL,
		CloudAPIToken:       apiToken,
		Theme:               theme,
		RAGEnabled:          ragEnabled,
		RAGNotebookChapter:  ragNotebookChapter,
		RAGEntireNotebook:   ragEntireNotebook,
		RAGQueueStudy:       ragQueueStudy,
	}
	// Persist settings first so SQLite is never stale if runtime mutation fails.
	if err := repo.UpdateUserSettings(s); err != nil {
		return map[string]interface{}{"error": err.Error()}
	}

	// Only mutate runtime after successful persistence.
	a.aiMutex.Lock()
	if !ragEnabled && a.embedder != nil {
		utils.Infof("RAG disabled dynamically in settings. Closing ONNX embedder.")
		_ = a.embedder.Close()
		a.embedder = nil
		a.aiReady = false
	}
	a.aiMutex.Unlock()

	if !ragEnabled {
		if err := a.reloadRetrievalEngine(); err != nil {
			utils.Errorf("reloadRetrievalEngine after RAG disable: %v", err)
		}
	}

	return map[string]interface{}{"ok": true}
}

func (a *App) GetLLMSettings() map[string]interface{} {
	repo := a.getRepo()
	if repo == nil {
		return map[string]interface{}{"error": "database repository not initialized"}
	}
	settings, err := repo.GetLLMSettings()
	if err != nil {
		return map[string]interface{}{"error": err.Error()}
	}
	settings.Fast.HasAPIKey = settings.Fast.HasAPIKey || llm.HasAPIKey("fast") || envHasLLMAPIKey("FAST_LLM")
	settings.Heavy.HasAPIKey = settings.Heavy.HasAPIKey || llm.HasAPIKey("heavy") || envHasLLMAPIKey("HEAVY_LLM")
	settings.UseSameForHeavy = sameLLMSettingsForUI(settings.Fast, settings.Heavy)
	return map[string]interface{}{"settings": settings}
}

func (a *App) GetLLMProviderPreset(provider string) map[string]interface{} {
	provider = strings.TrimSpace(strings.ToLower(provider))
	switch provider {
	case "groq":
		return map[string]interface{}{
			"provider": "groq",
			"base_url": "https://api.groq.com/openai",
			"model":    "openai/gpt-oss-120b",
		}
	case "openai":
		return map[string]interface{}{
			"provider": "openai",
			"base_url": "https://api.openai.com",
			"model":    "gpt-4.1-mini",
		}
	case "openrouter":
		return map[string]interface{}{
			"provider": "openrouter",
			"base_url": "https://openrouter.ai/api",
			"model":    "openai/gpt-4.1-mini",
		}
	default:
		return map[string]interface{}{
			"provider": "custom",
			"base_url": "",
			"model":    "",
		}
	}
}

func (a *App) UpdateLLMSettings(settings models.LLMSettings) map[string]interface{} {
	repo := a.getRepo()
	if repo == nil {
		return map[string]interface{}{"error": "database repository not initialized"}
	}
	current, err := repo.GetLLMSettings()
	if err != nil {
		return map[string]interface{}{"error": err.Error()}
	}
	settings.Fast.Tier = "fast"
	if settings.Fast.TimeoutMs <= 0 {
		settings.Fast.TimeoutMs = 30000
	}
	if settings.UseSameForHeavy {
		settings.Heavy = settings.Fast
		settings.Heavy.Tier = "heavy"
		settings.Heavy.TimeoutMs = 90000
	} else {
		settings.Heavy.Tier = "heavy"
		if settings.Heavy.TimeoutMs <= 0 {
			settings.Heavy.TimeoutMs = 90000
		}
	}
	settings.Fast.HasAPIKey = current.Fast.HasAPIKey || llm.HasAPIKey("fast") || envHasLLMAPIKey("FAST_LLM")
	settings.Heavy.HasAPIKey = current.Heavy.HasAPIKey || llm.HasAPIKey("heavy") || envHasLLMAPIKey("HEAVY_LLM")
	if settings.UseSameForHeavy {
		settings.Heavy.HasAPIKey = settings.Fast.HasAPIKey
	}
	if err := repo.UpdateLLMSettings(settings); err != nil {
		return map[string]interface{}{"error": err.Error()}
	}
	if err := a.reloadLLMProviders(); err != nil {
		return map[string]interface{}{"error": "settings saved but LLM reload failed: " + err.Error()}
	}
	return map[string]interface{}{"ok": true}
}

func (a *App) SaveLLMAPIKey(tier string, key string) map[string]interface{} {
	repo := a.getRepo()
	if repo == nil {
		return map[string]interface{}{"error": "database repository not initialized"}
	}
	tier = normalizeLLMTierForApp(tier)
	if tier == "" {
		return map[string]interface{}{"error": "tier must be fast or heavy"}
	}
	if err := llm.SaveAPIKey(tier, key); err != nil {
		return map[string]interface{}{"error": err.Error()}
	}
	if err := repo.MarkLLMKeyStored(tier, true); err != nil {
		return map[string]interface{}{"error": err.Error()}
	}
	if err := a.reloadLLMProviders(); err != nil {
		return map[string]interface{}{"error": "key saved but LLM reload failed: " + err.Error()}
	}
	return map[string]interface{}{"ok": true}
}

func (a *App) DeleteLLMAPIKey(tier string) map[string]interface{} {
	repo := a.getRepo()
	if repo == nil {
		return map[string]interface{}{"error": "database repository not initialized"}
	}
	tier = normalizeLLMTierForApp(tier)
	if tier == "" {
		return map[string]interface{}{"error": "tier must be fast or heavy"}
	}
	if err := llm.DeleteAPIKey(tier); err != nil {
		return map[string]interface{}{"error": err.Error()}
	}
	if err := repo.MarkLLMKeyStored(tier, false); err != nil {
		return map[string]interface{}{"error": err.Error()}
	}
	if err := a.reloadLLMProviders(); err != nil {
		return map[string]interface{}{"error": "key deleted but LLM reload failed: " + err.Error()}
	}
	return map[string]interface{}{"ok": true}
}

func (a *App) reloadLLMProviders() error {
	settings, err := a.repo.GetLLMSettings()
	if err != nil {
		return err
	}

	fastKey, _ := llm.GetAPIKey("fast")
	heavyKey, _ := llm.GetAPIKey("heavy")
	if settings.UseSameForHeavy && heavyKey == "" {
		heavyKey = fastKey
	}
	fastProvider := llm.NewProvider(llm.LoadConfigFromSettingsForPrefix("FAST_LLM", settings.Fast, fastKey))
	heavyProvider := llm.NewProvider(llm.LoadConfigFromSettingsForPrefix("HEAVY_LLM", settings.Heavy, heavyKey))

	a.aiMutex.Lock()
	a.fastLLMProvider = fastProvider
	a.heavyLLMProvider = heavyProvider
	engine := a.retrievalEngine
	a.studyService = study.NewStudyService(study.Config{
		FastLLMProvider:  fastProvider,
		HeavyLLMProvider: heavyProvider,
		RetrievalEngine:  engine,
	})
	a.aiMutex.Unlock()
	return nil
}

func normalizeLLMTierForApp(tier string) string {
	tier = strings.TrimSpace(strings.ToLower(tier))
	switch tier {
	case "fast", "heavy":
		return tier
	default:
		return ""
	}
}

func envHasLLMAPIKey(prefix string) bool {
	prefix = strings.TrimSuffix(strings.TrimSpace(prefix), "_")
	keys := []string{"LLM_API_KEY", "OPENAI_API_KEY", "API_KEY"}
	for _, key := range keys {
		if prefix != "" && strings.TrimSpace(os.Getenv(prefix+"_"+key)) != "" {
			return true
		}
		if strings.TrimSpace(os.Getenv(key)) != "" {
			return true
		}
	}
	return false
}

func sameLLMSettingsForUI(a, b models.LLMTierSettings) bool {
	return strings.EqualFold(a.Provider, b.Provider) &&
		strings.TrimSpace(a.BaseURL) == strings.TrimSpace(b.BaseURL) &&
		strings.TrimSpace(a.Model) == strings.TrimSpace(b.Model) &&
		a.TimeoutMs == b.TimeoutMs &&
		a.HasAPIKey == b.HasAPIKey
}

func (a *App) GetProfiles() map[string]interface{} {
	repo := a.getRepo()
	if repo == nil {
		return map[string]interface{}{"error": "database repository not initialized"}
	}
	profiles, err := repo.GetProfiles()
	if err != nil {
		return map[string]interface{}{"error": err.Error()}
	}
	return map[string]interface{}{"profiles": profiles}
}

func (a *App) CreateProfile(name string, deadlineStr string) map[string]interface{} {
	repo := a.getRepo()
	if repo == nil {
		return map[string]interface{}{"error": "database repository not initialized"}
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return map[string]interface{}{"error": "profile name is required"}
	}
	deadlineTime, err := time.Parse("2006-01-02", deadlineStr)
	if err != nil {
		return map[string]interface{}{"error": "failed to parse deadline: " + err.Error()}
	}
	p := models.StudyProfile{
		ID:         uuid.NewString(),
		Name:       name,
		DeadlineAt: deadlineTime.Unix(),
	}
	if err := repo.CreateProfile(p); err != nil {
		return map[string]interface{}{"error": err.Error()}
	}

	// If no active profile is set yet, make this the default automatically.
	// First profile created = default active profile.
	s, err := repo.GetUserSettings()
	if err == nil && s != nil && s.ActiveProfileID == "" {
		s.ActiveProfileID = p.ID
		if err := repo.UpdateUserSettings(*s); err != nil {
			return map[string]interface{}{"error": "profile created but failed to set as active: " + err.Error()}
		}
	}

	return map[string]interface{}{"ok": true, "profile": p}
}

func (a *App) UpdateProfile(id string, name string, deadlineStr string) map[string]interface{} {
	repo := a.getRepo()
	if repo == nil {
		return map[string]interface{}{"error": "database repository not initialized"}
	}
	id = strings.TrimSpace(id)
	name = strings.TrimSpace(name)
	if id == "" || name == "" {
		return map[string]interface{}{"error": "id and name are required"}
	}
	deadlineTime, err := time.Parse("2006-01-02", deadlineStr)
	if err != nil {
		return map[string]interface{}{"error": "failed to parse deadline: " + err.Error()}
	}
	p := models.StudyProfile{
		ID:         id,
		Name:       name,
		DeadlineAt: deadlineTime.Unix(),
	}
	if err := repo.UpdateProfile(p); err != nil {
		return map[string]interface{}{"error": err.Error()}
	}
	return map[string]interface{}{"ok": true}
}

func (a *App) DeleteProfile(id string) map[string]interface{} {
	repo := a.getRepo()
	if repo == nil {
		return map[string]interface{}{"error": "database repository not initialized"}
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return map[string]interface{}{"error": "profile id is required"}
	}
	if err := repo.DeleteProfile(id); err != nil {
		return map[string]interface{}{"error": err.Error()}
	}
	return map[string]interface{}{"ok": true}
}

func (a *App) AssignNotebookToProfile(notebookID, profileID string) map[string]interface{} {
	repo := a.getRepo()
	if repo == nil {
		return map[string]interface{}{"error": "database repository not initialized"}
	}
	if err := repo.AssignNotebookToProfile(notebookID, profileID); err != nil {
		return map[string]interface{}{"error": err.Error()}
	}
	return map[string]interface{}{"ok": true}
}

func (a *App) UpdateNotebookStudyStatus(notebookID, studyStatus string) map[string]interface{} {
	repo := a.getRepo()
	if repo == nil {
		return map[string]interface{}{"error": "database repository not initialized"}
	}
	if err := repo.UpdateNotebookStudyStatus(notebookID, studyStatus); err != nil {
		return map[string]interface{}{"error": err.Error()}
	}
	return map[string]interface{}{"ok": true}
}

func (a *App) IsOnboarded() map[string]interface{} {
	repo := a.getRepo()
	if repo == nil {
		return map[string]interface{}{"error": "database repository not initialized", "onboarded": false}
	}
	profiles, err := repo.GetProfiles()
	if err != nil {
		return map[string]interface{}{"error": err.Error(), "onboarded": false}
	}
	onboarded := len(profiles) > 0
	return map[string]interface{}{"onboarded": onboarded}
}

func (a *App) TriggerCloudSync() map[string]interface{} {
	repo := a.getRepo()
	if repo == nil {
		return map[string]interface{}{"error": "database repository not initialized"}
	}
	if err := study.TriggerCloudSync(a.repo); err != nil {
		return map[string]interface{}{"error": err.Error()}
	}
	return map[string]interface{}{"ok": true}
}

// ---------- Manual Mode endpoints (Phase 1 new) ---------

func (a *App) GenerateManualFlashcards(notebookID string, startPage, endPage int) map[string]interface{} {
	repo := a.getRepo()
	if repo == nil {
		return map[string]interface{}{"error": "database repository not initialized"}
	}
	if a.studyService == nil {
		return map[string]interface{}{"error": "study service not initialized"}
	}
	return a.studyService.GenerateManualFlashcards(notebookID, startPage, endPage)
}

func (a *App) GenerateComprehensiveExam(notebookID string, startPage, endPage int) map[string]interface{} {
	repo := a.getRepo()
	if repo == nil {
		return map[string]interface{}{"error": "database repository not initialized"}
	}
	if a.studyService == nil {
		return map[string]interface{}{"error": "study service not initialized"}
	}
	return a.studyService.GenerateComprehensiveExam(notebookID, startPage, endPage)
}

func (a *App) GenerateFlashcards(topicID string) map[string]interface{} {
	repo := a.getRepo()
	if repo == nil {
		return map[string]interface{}{"error": "database repository not initialized"}
	}
	if a.studyService == nil {
		return map[string]interface{}{"error": "study service not initialized"}
	}

	// Get notebook for this topic (no profile filter - topic-scoped)
	notebooks, err := repo.GetNotebooks(topicID, "")
	if err != nil {
		return map[string]interface{}{"error": "failed to get notebook: " + err.Error()}
	}
	if len(notebooks) == 0 {
		return map[string]interface{}{"error": "no notebook found for topic"}
	}
	notebookID := notebooks[0].ID

	// Get page bounds for this topic
	startPage, endPage, err := repo.GetTopicPageBounds(topicID)
	if err != nil {
		return map[string]interface{}{"error": "failed to get topic page bounds: " + err.Error()}
	}

	cards, states, existing, tier, err := a.studyService.GenerateFSRSCardsForTopic(topicID, notebookID, startPage, endPage)
	if err != nil {
		return map[string]interface{}{"error": err.Error()}
	}

	now := time.Now().Unix()
	response := map[string]interface{}{
		"notebook_id":       notebookID,
		"existing":          existing,
		"start_page":        startPage,
		"end_page":          endPage,
		"topic_id":          topicID,
		"cards":             cards,
		"states":            states,
		"card_count":        len(cards),
		"llm_tier":          tier,
		"generated_at_unix": now,
	}

	// Find the minimum due time for existing/generated cards to populate initial_due_at
	var initialDueAt int64 = 0
	for _, card := range cards {
		if card.DueAt > 0 && (initialDueAt == 0 || card.DueAt < initialDueAt) {
			initialDueAt = card.DueAt
		}
	}
	if initialDueAt > 0 {
		response["initial_due_at"] = initialDueAt
	}

	return response
}

func (a *App) GetReviewSession(taskID string, notebookID string) map[string]interface{} {
	repo := a.getRepo()
	if repo == nil {
		return map[string]interface{}{"error": "database repository not initialized"}
	}
	if a.studyService == nil {
		return map[string]interface{}{"error": "study service not initialized"}
	}

	// Materialize synthetic tasks from the scheduler
	if taskID == models.ReviewTaskDailyID {
		requestedNotebookID := notebookID
		utils.Warnf("[FLASHCARD_PIPELINE] GetReviewSession materializing synthetic task notebookID=%s", notebookID)
		if notebookID == "" {
			resolvedNotebookID, dueCount, err := repo.GetNextDueReviewNotebook(time.Now().Unix())
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
		task, reused, err := repo.CreateReviewSession(notebookID)
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
	repo := a.getRepo()
	if repo == nil {
		return map[string]interface{}{"error": "database repository not initialized"}
	}
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
	repo := a.getRepo()
	if repo == nil {
		return map[string]interface{}{"error": "database repository not initialized"}
	}
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

func (a *App) ScoreShortAnswer(questionID, userAnswer string) map[string]interface{} {
	repo := a.getRepo()
	if repo == nil {
		return map[string]interface{}{"error": "database repository not initialized"}
	}
	if a.studyService == nil {
		return map[string]interface{}{"error": "study service not initialized"}
	}
	return a.studyService.ScoreShortAnswer(questionID, userAnswer)
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
				a.repo = fbRepo
				emitRagSetupFailed(a, fmt.Sprintf("failed to reload DB with vector extension: %v", err))
			}
			return
		}
		a.repo = newRepo

		if !repo.IsVecExtensionLoaded() {
			fbRepo, fbErr := db.Init(dbPath, "")
			if fbErr != nil {
				emitRagSetupFailed(a, fmt.Sprintf("sqlite-vec extension is missing or failed to load (requires CGO and vec0 binary), and fallback non-vector initialization also failed: %v", fbErr))
			} else {
				a.repo = fbRepo
				emitRagSetupFailed(a, "sqlite-vec extension is missing or failed to load (requires CGO and vec0 binary)")
			}
			return
		}

		// Init ONNX embedder using paths from AssetManager
		emb, err := embeddings.NewOnnxEmbedder(am.ModelPath(), am.TokenizerPath(), am.OnnxRuntimePath())
		if err != nil {
			emitRagSetupFailed(a, fmt.Sprintf("failed to load ONNX embedder: %v", err))
			return
		}

		if err := embeddings.InitPromptTokenizer(am.TokenizerPath()); err != nil {
			_ = emb.Close()
			emitRagSetupFailed(a, fmt.Sprintf("could not initialize prompt tokenizer: %v", err))
			return
		}

		// Set dimensions
		if err := repo.InitWithVectorDimension(emb.GetDimension()); err != nil {
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
		settings, err := repo.GetUserSettings()
		if err == nil {
			settings.RAGEnabled = true
			_ = repo.UpdateUserSettings(*settings)
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
