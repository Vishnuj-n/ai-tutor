package main

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"strings"
	"time"

	"ai-tutor/internal/db"
	"ai-tutor/internal/models"
	"ai-tutor/internal/scheduler"
	"ai-tutor/internal/utils"

	"github.com/google/uuid"
)

// ---------- Helpers for GetTodayPlan ----------

func calculateDailyStudyMinutes(studyStart, studyEnd string) int {
	dailyStudyMinutes := 60 // default fallback
	var sh, sm, eh, em int
	if _, errS := fmt.Sscanf(studyStart, "%d:%d", &sh, &sm); errS == nil {
		if _, errE := fmt.Sscanf(studyEnd, "%d:%d", &eh, &em); errE == nil {
			startMins := sh*60 + sm
			endMins := eh*60 + em
			diff := endMins - startMins
			if diff < 0 {
				diff += 1440
			}
			if diff > 0 {
				dailyStudyMinutes = diff
			}
		}
	}
	return dailyStudyMinutes
}

func calculateFlashcardBudgets(dueCards, maxFlashcards int) (int, int, int) {
	materializedCards := dueCards
	if materializedCards > maxFlashcards {
		materializedCards = maxFlashcards
	}
	deferredCards := dueCards - materializedCards
	if deferredCards < 0 {
		deferredCards = 0
	}
	safeReviewBudget := int(math.Ceil(float64(materializedCards) * scheduler.ReviewMinutesPerCard))
	return materializedCards, deferredCards, safeReviewBudget
}

func aggregateQueueTasks(active, pending []models.StudyQueueTask) ([]models.ScheduledTask, []string, int, map[string]int) {
	queueTasks := make([]models.ScheduledTask, 0, len(active)+len(pending))
	actionCounts := make(map[string]int)
	activeTopicsMap := make(map[string]bool)

	processTasks := func(tasks []models.StudyQueueTask) {
		for _, q := range tasks {
			task := queueTaskToScheduledTask(q)
			queueTasks = append(queueTasks, task)
			actionCounts[task.ActionType]++
			if q.Title != "" {
				activeTopicsMap[q.Title] = true
			}
		}
	}

	processTasks(active)
	processTasks(pending)

	activeTopics := make([]string, 0, len(activeTopicsMap))
	for topicTitle := range activeTopicsMap {
		activeTopics = append(activeTopics, topicTitle)
	}

	learningMinutes := 0
	for _, task := range queueTasks {
		learningMinutes += task.EstimateMinutes
	}

	return queueTasks, activeTopics, learningMinutes, actionCounts
}

// ---------- Main App Methods ----------

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
		settings, err := repo.GetUserSettings()
		if err != nil {
			return map[string]interface{}{"error": err.Error()}
		}
		maxFlashcards := settings.MaxFlashcardsPerSession
		if maxFlashcards <= 0 {
			maxFlashcards = 30
		}

		dailyStudyMinutes := calculateDailyStudyMinutes(settings.StudyStartTime, settings.StudyEndTime)
		materializedCards, deferredCards, safeReviewBudget := calculateFlashcardBudgets(dueCards, maxFlashcards)
		queueTasks, activeTopics, learningMinutes, actionCounts := aggregateQueueTasks(activeQueueTasks, pendingQueueTasks)

		plan = &models.TodayPlan{
			Date:                now.Format("2006-01-02"),
			TotalMinutes:        dailyStudyMinutes,
			ReviewMinutes:       safeReviewBudget,
			LearningMinutes:     learningMinutes,
			DueReviewCards:      materializedCards,
			TotalDueReviewCards: dueCards,
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
	activeProfileID, err := repo.GetActiveProfileID()
	if err != nil {
		return map[string]interface{}{"error": err.Error()}
	}
	activeNotebookCount, err := repo.CountActiveNotebooksForActiveProfile(activeProfileID)
	if err != nil {
		return map[string]interface{}{"error": err.Error()}
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
	case models.StudyTaskTypeSocraticRemedial:
		titlePrefix = "Concept Rescue"
	case models.StudyTaskTypeFlashcardSync:
		titlePrefix = "Sync Flashcards"
	}

	meta := ""
	if task.StartPage > 0 && task.EndPage > 0 {
		meta = fmt.Sprintf("Pages %d-%d", task.StartPage, task.EndPage)
	}
	estimateMinutes := 10
	if task.TaskType == models.StudyTaskTypeFlashcardSync {
		estimateMinutes = 0
	} else if task.TaskType == models.StudyTaskTypeFlashcardReview {
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

// ---------- Helpers for InitializeReadingSession ----------

func (a *App) resolveReadingTaskIdentity(taskID, notebookID, topicID string, startPage, endPage int) (string, map[string]interface{}) {
	repo := a.getRepo()
	seedTaskID := taskID
	existingTask, existingErr := repo.GetTaskByID(seedTaskID)

	// If task doesn't exist yet, insert it as a real READING task.
	if existingErr == db.ErrTaskNotFound {
		utils.Warnf("[READER_INIT] InitializeReadingSession task missing, creating pending reading task taskID=%s notebookID=%s topicID=%s", taskID, notebookID, topicID)
		if notebookID == "" || topicID == "" {
			return "", map[string]interface{}{"error": "task not found and notebookID/topicID required to create it", "code": 400}
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
			return "", map[string]interface{}{"error": "failed to create reading task: " + insertErr.Error()}
		}
		return seedTaskID, nil
	} else if existingErr != nil {
		return "", map[string]interface{}{"error": existingErr.Error()}
	}

	// Never reopen terminal queue rows.
	if existingTask != nil && existingTask.Status != models.StudyTaskStatusPending && existingTask.Status != models.StudyTaskStatusActive {
		if notebookID == "" {
			notebookID = existingTask.NotebookID
		}
		if topicID == "" {
			topicID = existingTask.TopicID
		}
		if notebookID == "" || topicID == "" {
			return "", map[string]interface{}{"error": "terminal task cannot be reused and notebookID/topicID were not available", "code": 409}
		}
		newTaskID := uuid.NewString()
		utils.Warnf("[READER_INIT] InitializeReadingSession task terminal, creating new queue row taskID=%s oldStatus=%s notebookID=%s topicID=%s", newTaskID, existingTask.Status, notebookID, topicID)
		insertErr := repo.InsertStudyTask(models.StudyQueueTask{
			ID:         newTaskID,
			NotebookID: notebookID,
			TopicID:    topicID,
			TaskType:   models.StudyTaskTypeReading,
			Status:     models.StudyTaskStatusPending,
			Priority:   1,
			StartPage:  startPage,
			EndPage:    endPage,
		})
		if insertErr != nil {
			return "", map[string]interface{}{"error": "failed to create replacement reading task: " + insertErr.Error()}
		}
		return newTaskID, nil
	}

	return taskID, nil
}

func (a *App) activateReadingSessionTask(taskID string) map[string]interface{} {
	repo := a.getRepo()
	qTask, qErr := repo.GetTaskByID(taskID)

	if qErr != nil || qTask == nil {
		var errDetail error
		if qErr != nil {
			errDetail = qErr
		} else {
			errDetail = fmt.Errorf("nil task loaded from database")
		}
		utils.Errorf("InitializeReadingSession loading anomaly: taskID=%s err=%v", taskID, errDetail)
		utils.QueueLogger.Info("queue task pre-activate loading anomaly", "taskID", taskID)

		if err := repo.ActivateTask(taskID); err != nil {
			utils.Errorf("InitializeReadingSession activation failed: taskID=%s err=%v", taskID, err)
			utils.QueueLogger.Info("queue task activation failed", "taskID", taskID)
			return map[string]interface{}{"error": "failed to activate task: " + err.Error()}
		} else {
			utils.QueueLogger.Info("queue task activated", "taskID", taskID)
		}
	} else {
		switch qTask.Status {
		case models.StudyTaskStatusPending:
			if err := repo.ActivateTask(taskID); err != nil {
				utils.Errorf("InitializeReadingSession activation failed: taskID=%s err=%v", taskID, err)
				utils.QueueLogger.Info("queue task activation failed", "taskID", taskID)
				return map[string]interface{}{"error": "failed to activate task: " + err.Error()}
			} else {
				utils.QueueLogger.Info("queue task activated", "taskID", taskID)
			}
		case models.StudyTaskStatusActive:
			utils.QueueLogger.Info("idempotent resume: task already active", "taskID", taskID, "status", qTask.Status, "type", qTask.TaskType, "notebookID", qTask.NotebookID, "topicID", qTask.TopicID)
		default:
			utils.QueueLogger.Info("task terminal", "status", qTask.Status, "taskID", taskID)
			return map[string]interface{}{"error": "task is in terminal status: " + string(qTask.Status), "code": 409}
		}
	}
	return nil
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

	resolvedTaskID, errMap := a.resolveReadingTaskIdentity(taskID, notebookID, topicID, startPage, endPage)
	if errMap != nil {
		return errMap
	}
	taskID = resolvedTaskID

	if errMap := a.activateReadingSessionTask(taskID); errMap != nil {
		return errMap
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

	// Use repository-provided and clamped CurrentPage from GetReadingTask
	currentPage := task.CurrentPage
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

	queueTask, qErr := repo.GetTaskByID(taskID)
	if qErr != nil {
		utils.Warnf("[COMPLETE_SESSION] CompleteReading GetTaskByID error taskID=%s err=%v", taskID, qErr)
		return map[string]interface{}{"error": qErr.Error()}
	}
	if queueTask.Status != models.StudyTaskStatusActive {
		utils.Warnf("[COMPLETE_SESSION] CompleteReading task not active taskID=%s status=%s", taskID, queueTask.Status)
		return map[string]interface{}{"error": "task is not active", "code": 409}
	}

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

func (a *App) GetTaskContext(taskID string) map[string]interface{} {
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
	return map[string]interface{}{
		"task": task,
		"topic": map[string]interface{}{
			"id": task.TopicID,
		},
		"notebook": map[string]interface{}{
			"id": task.NotebookID,
		},
	}
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

// ---------- Manual Mode endpoints ----------

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

	notebooks, err := repo.GetNotebooks(topicID, "")
	if err != nil {
		return map[string]interface{}{"error": "failed to get notebook: " + err.Error()}
	}
	if len(notebooks) == 0 {
		return map[string]interface{}{"error": "no notebook found for topic"}
	}
	notebookID := notebooks[0].ID

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

		task, reused, err := repo.CreateReviewSession(notebookID)
		if err != nil {
			return map[string]interface{}{"error": "Failed to materialize review session: " + err.Error()}
		}
		if task == nil {
			return map[string]interface{}{"error": "No due cards found for review materialization"}
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

func (a *App) SuspendFlashcard(taskID, cardID string) map[string]interface{} {
	repo := a.getRepo()
	if repo == nil {
		return map[string]interface{}{"error": "database repository not initialized"}
	}
	if a.studyService == nil {
		return map[string]interface{}{"error": "study service not initialized"}
	}
	remaining, err := a.studyService.SuspendFlashcard(taskID, cardID)
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
	return map[string]interface{}{"ok": true, "remaining": remaining}
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

// CompleteSocraticRescue completes the socratic rescue session and inserts a re-quiz.
func (a *App) CompleteSocraticRescue(taskID string) map[string]interface{} {
	if a.studyService == nil {
		return map[string]interface{}{"error": "study service not initialized"}
	}
	quizTaskID, err := a.studyService.CompleteSocraticRescue(taskID)
	if err != nil {
		return map[string]interface{}{"error": err.Error()}
	}
	return map[string]interface{}{"ok": true, "quiz_task_id": quizTaskID}
}

// GetAppEnv returns the current value of the APP_ENV environment variable.
func (a *App) GetAppEnv() map[string]interface{} {
	return map[string]interface{}{
		"env": os.Getenv("APP_ENV"),
	}
}

// DevForceSocraticRescue forces a topic into the SOCRATIC_REMEDIAL queue task state.
// Only accessible when APP_ENV = dev.
func (a *App) DevForceSocraticRescue(notebookID, topicID string) map[string]interface{} {
	if os.Getenv("APP_ENV") != "dev" {
		return map[string]interface{}{"error": "forbidden: dev mode only"}
	}
	repo := a.getRepo()
	if repo == nil {
		return map[string]interface{}{"error": "database repository not initialized"}
	}

	tx, err := repo.Begin()
	if err != nil {
		return map[string]interface{}{"error": err.Error()}
	}
	defer func() { _ = tx.Rollback() }()

	// Wipe FSRS flashcards for this topic to protect purity
	if err := repo.DeleteFSRSCardsByTopicIDTx(tx, topicID); err != nil {
		return map[string]interface{}{"error": err.Error()}
	}

	feedback := "Concept rescue activated. Complete the Socratic session to retry."
	socraticTaskID := uuid.NewString()
	socraticPayload, _ := json.Marshal(map[string]string{
		"feedback": feedback,
		"lane":     "socratic_rescue",
		"mode":     "external_prompt",
	})

	// Note: the hardcoded start_page value of 1 and end_page value of 10 are placeholder bounds used only for this dev helper function.
	socraticTask := models.StudyQueueTask{
		ID:          socraticTaskID,
		NotebookID:  notebookID,
		TopicID:     topicID,
		TaskType:    models.StudyTaskTypeSocraticRemedial,
		Status:      models.StudyTaskStatusPending,
		Priority:    0,
		PayloadJSON: string(socraticPayload),
		StartPage:   1,
		EndPage:     10,
	}
	err = repo.InsertStudyTaskTx(tx, socraticTask)
	if err != nil {
		return map[string]interface{}{"error": err.Error()}
	}

	if err := tx.Commit(); err != nil {
		return map[string]interface{}{"error": err.Error()}
	}

	return map[string]interface{}{"ok": true, "task_id": socraticTaskID}
}

// DevForceFlashcardSync forces a FLASHCARD_SYNC task into the pending queue.
// Only accessible when APP_ENV = dev.
func (a *App) DevForceFlashcardSync(notebookID string) map[string]interface{} {
	if os.Getenv("APP_ENV") != "dev" {
		return map[string]interface{}{"error": "forbidden: dev mode only"}
	}
	repo := a.getRepo()
	if repo == nil {
		return map[string]interface{}{"error": "database repository not initialized"}
	}
	if err := repo.EnsurePendingFlashcardSyncTask(notebookID); err != nil {
		return map[string]interface{}{"error": err.Error()}
	}
	return map[string]interface{}{"ok": true}
}

type FlashcardDuePoint struct {
	Date      string `json:"date"`
	DayLabel  string `json:"day_label"`
	CardCount int    `json:"card_count"`
}

// GetFlashcardDueTimeline returns the review card load over the next 7 days.
func (a *App) GetFlashcardDueTimeline() map[string]interface{} {
	repo := a.getRepo()
	if repo == nil {
		return map[string]interface{}{"error": "database repository not initialized"}
	}

	now := time.Now()
	y, m, d := now.Date()
	midnight := time.Date(y, m, d, 0, 0, 0, 0, now.Location())
	endOfToday := midnight.Add(24 * time.Hour).Unix()

	timeline := make([]FlashcardDuePoint, 7)

	// Day 0: Today (due_at in (midnight, endOfToday])
	midnightUnix := midnight.Unix()
	count, err := repo.QueryDueReviewCardsForRange(midnightUnix, endOfToday)
	if err != nil {
		return map[string]interface{}{"error": err.Error()}
	}
	timeline[0] = FlashcardDuePoint{
		Date:      midnight.Format("2006-01-02"),
		DayLabel:  "Today",
		CardCount: count,
	}

	// Days 1 to 6
	dayNames := []string{"Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"}
	for i := 1; i < 7; i++ {
		dayStart := endOfToday + int64(i-1)*24*3600
		dayEnd := endOfToday + int64(i)*24*3600

		count, err := repo.QueryDueReviewCardsForRange(dayStart, dayEnd)
		if err != nil {
			return map[string]interface{}{"error": err.Error()}
		}

		targetDay := midnight.Add(time.Duration(i*24) * time.Hour)
		dayLabel := ""
		if i == 1 {
			dayLabel = "Tomorrow"
		} else {
			dayLabel = dayNames[targetDay.Weekday()]
		}

		timeline[i] = FlashcardDuePoint{
			Date:      targetDay.Format("2006-01-02"),
			DayLabel:  dayLabel,
			CardCount: count,
		}
	}

	return map[string]interface{}{
		"timeline": timeline,
	}
}
