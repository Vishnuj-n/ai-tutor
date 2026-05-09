package db

import (
	"ai-tutor/internal/models"
	"ai-tutor/internal/utils"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
)

var (
	ErrNoPendingTasks = errors.New("no pending tasks in queue")
	ErrTaskNotPending = errors.New("task is not in PENDING status")
	ErrTaskNotActive  = errors.New("task is not in ACTIVE status")
	ErrTaskNotFound   = errors.New("task not found")
)

// InsertStudyTask inserts one task row in study_queue.
func InsertStudyTask(task models.StudyQueueTask) error {
	task.ID = strings.TrimSpace(task.ID)
	task.NotebookID = strings.TrimSpace(task.NotebookID)
	task.TopicID = strings.TrimSpace(task.TopicID)
	task.PayloadJSON = strings.TrimSpace(task.PayloadJSON)
	if task.ID == "" {
		return fmt.Errorf("task id is required")
	}
	if task.NotebookID == "" {
		return fmt.Errorf("notebook id is required")
	}
	if strings.TrimSpace(string(task.TaskType)) == "" {
		return fmt.Errorf("task type is required")
	}
	if strings.TrimSpace(string(task.Status)) == "" {
		task.Status = models.StudyTaskStatusPending
	}

	_, err := conn.Exec(`
		INSERT INTO study_queue (
			id, notebook_id, topic_id, task_type, status, priority, payload_json, start_page, end_page
		) VALUES (?, ?, NULLIF(?, ''), ?, ?, ?, NULLIF(?, ''), ?, ?)
	`, task.ID, task.NotebookID, task.TopicID, string(task.TaskType), string(task.Status), task.Priority, task.PayloadJSON, task.StartPage, task.EndPage)
	return err
}

// GetTaskByID returns one queue task by id.
func GetTaskByID(taskID string) (*models.StudyQueueTask, error) {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return nil, fmt.Errorf("task id is required")
	}
	task := &models.StudyQueueTask{}
	err := conn.QueryRow(`
		SELECT
			id, notebook_id, COALESCE(topic_id, ''), task_type, status, priority,
			COALESCE(created_at, ''), COALESCE(activated_at, ''), COALESCE(completed_at, ''),
			COALESCE(payload_json, ''), COALESCE(start_page, 0), COALESCE(end_page, 0)
		FROM study_queue
		WHERE id = ?
	`, taskID).Scan(
		&task.ID, &task.NotebookID, &task.TopicID, &task.TaskType, &task.Status, &task.Priority,
		&task.CreatedAt, &task.ActivatedAt, &task.CompletedAt, &task.PayloadJSON, &task.StartPage, &task.EndPage,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrTaskNotFound
	}
	if err != nil {
		return nil, err
	}
	return task, nil
}

// GetAllPendingTasks returns all pending tasks ordered by deterministic queue rules.
func GetAllPendingTasks() ([]models.StudyQueueTask, error) {
	utils.Warnf("[QUEUE] GetAllPendingTasks filter status=PENDING order=task_type, notebook_priority desc, task_priority asc, created_at asc")
	query := `
		SELECT
			sq.id,
			sq.notebook_id,
			COALESCE(sq.topic_id, ''),
			sq.task_type,
			sq.status,
			sq.priority,
			COALESCE(sq.created_at, ''),
			COALESCE(sq.activated_at, ''),
			COALESCE(sq.completed_at, ''),
			COALESCE(sq.payload_json, ''),
			COALESCE(sq.start_page, 0),
			COALESCE(sq.end_page, 0),
			COALESCE(t.title, '')
		FROM study_queue sq
		LEFT JOIN notebooks n ON sq.notebook_id = n.id
		LEFT JOIN topics t ON sq.topic_id = t.id
		WHERE sq.status = 'PENDING'
		ORDER BY
			CASE sq.task_type
				WHEN 'FLASHCARD_REVIEW' THEN 1
				WHEN 'REREAD' THEN 2
				WHEN 'QUIZ' THEN 3
				WHEN 'READING' THEN 4
				WHEN 'EXAMINER' THEN 5
				ELSE 6
			END,
			COALESCE(n.priority, 5) DESC,
			sq.priority ASC,
			sq.created_at ASC
	`

	rows, err := conn.Query(query)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = rows.Close()
	}()

	tasks := make([]models.StudyQueueTask, 0)
	for rows.Next() {
		var task models.StudyQueueTask
		var topicTitle string
		err := rows.Scan(
			&task.ID,
			&task.NotebookID,
			&task.TopicID,
			&task.TaskType,
			&task.Status,
			&task.Priority,
			&task.CreatedAt,
			&task.ActivatedAt,
			&task.CompletedAt,
			&task.PayloadJSON,
			&task.StartPage,
			&task.EndPage,
			&topicTitle,
		)
		if err != nil {
			return nil, err
		}
		if topicTitle != "" {
			task.Title = topicTitle
		}
		tasks = append(tasks, task)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}
	utils.Warnf("[QUEUE] GetAllPendingTasks result count=%d", len(tasks))
	return tasks, nil
}

// GetAllActiveTasks returns all active tasks ordered by activation time.
func GetAllActiveTasks() ([]models.StudyQueueTask, error) {
	query := `
		SELECT
			sq.id,
			sq.notebook_id,
			COALESCE(sq.topic_id, ''),
			sq.task_type,
			sq.status,
			sq.priority,
			COALESCE(sq.created_at, ''),
			COALESCE(sq.activated_at, ''),
			COALESCE(sq.completed_at, ''),
			COALESCE(sq.payload_json, ''),
			COALESCE(sq.start_page, 0),
			COALESCE(sq.end_page, 0),
			COALESCE(t.title, '')
		FROM study_queue sq
		LEFT JOIN notebooks n ON sq.notebook_id = n.id
		LEFT JOIN topics t ON sq.topic_id = t.id
		WHERE sq.status = 'ACTIVE'
		ORDER BY sq.activated_at ASC
	`

	rows, err := conn.Query(query)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = rows.Close()
	}()

	tasks := make([]models.StudyQueueTask, 0)
	for rows.Next() {
		var task models.StudyQueueTask
		var topicTitle string
		err := rows.Scan(
			&task.ID,
			&task.NotebookID,
			&task.TopicID,
			&task.TaskType,
			&task.Status,
			&task.Priority,
			&task.CreatedAt,
			&task.ActivatedAt,
			&task.CompletedAt,
			&task.PayloadJSON,
			&task.StartPage,
			&task.EndPage,
			&topicTitle,
		)
		if err != nil {
			return nil, err
		}
		if topicTitle != "" {
			task.Title = topicTitle
		}
		tasks = append(tasks, task)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}
	return tasks, nil
}

// GetNextTask returns the next pending task ordered by deterministic queue rules.
func GetNextTask(notebookID string) (*models.StudyQueueTask, error) {
	notebookID = strings.TrimSpace(notebookID)
	utils.Warnf("[QUEUE] GetNextTask filter status=PENDING notebookID=%q order=task_type, notebook_priority desc, task_priority asc, created_at asc", notebookID)

	query := `
		SELECT
			sq.id,
			sq.notebook_id,
			COALESCE(sq.topic_id, ''),
			sq.task_type,
			sq.status,
			sq.priority,
			COALESCE(sq.created_at, ''),
			COALESCE(sq.activated_at, ''),
			COALESCE(sq.completed_at, ''),
			COALESCE(sq.payload_json, ''),
			COALESCE(sq.start_page, 0),
			COALESCE(sq.end_page, 0)
		FROM study_queue sq
		LEFT JOIN notebooks n ON sq.notebook_id = n.id
		WHERE sq.status = 'PENDING'
	`
	args := make([]interface{}, 0, 1)
	if notebookID != "" {
		query += ` AND sq.notebook_id = ?`
		args = append(args, notebookID)
	}
	query += `
		ORDER BY
			CASE sq.task_type
				WHEN 'FLASHCARD_REVIEW' THEN 1
				WHEN 'REREAD' THEN 2
				WHEN 'QUIZ' THEN 3
				WHEN 'READING' THEN 4
				WHEN 'EXAMINER' THEN 5
				ELSE 6
			END,
			COALESCE(n.priority, 5) DESC,
			sq.priority ASC,
			sq.created_at ASC
		LIMIT 1
	`

	task := &models.StudyQueueTask{}
	err := conn.QueryRow(query, args...).Scan(
		&task.ID,
		&task.NotebookID,
		&task.TopicID,
		&task.TaskType,
		&task.Status,
		&task.Priority,
		&task.CreatedAt,
		&task.ActivatedAt,
		&task.CompletedAt,
		&task.PayloadJSON,
		&task.StartPage,
		&task.EndPage,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNoPendingTasks
	}
	if err != nil {
		return nil, err
	}
	utils.Warnf("[QUEUE] GetNextTask result taskID=%s status=%s type=%s notebookID=%s topicID=%s", task.ID, task.Status, task.TaskType, task.NotebookID, task.TopicID)
	return task, nil
}

// ActivateTask moves one task from PENDING to ACTIVE.
func ActivateTask(taskID string) error {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return fmt.Errorf("task id is required")
	}
	var beforeStatus string
	if err := conn.QueryRow(`SELECT COALESCE(status, '') FROM study_queue WHERE id = ?`, taskID).Scan(&beforeStatus); err == nil {
		utils.Warnf("[QUEUE] ActivateTask before update taskID=%s status=%s", taskID, beforeStatus)
	} else {
		utils.Warnf("[QUEUE] ActivateTask before update taskID=%s statusLoadErr=%v", taskID, err)
	}
	res, err := conn.Exec(`
		UPDATE study_queue
		SET status = 'ACTIVE', activated_at = CURRENT_TIMESTAMP
		WHERE id = ? AND status = 'PENDING'
	`, taskID)
	if err != nil {
		return err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 1 {
		utils.Warnf("[QUEUE] ActivateTask transition taskID=%s from=PENDING to=ACTIVE", taskID)
		return nil
	}
	var exists int
	if err := conn.QueryRow(`SELECT COUNT(*) FROM study_queue WHERE id = ?`, taskID).Scan(&exists); err != nil {
		utils.Warnf("[QUEUE] ActivateTask existence check error taskID=%s err=%v", taskID, err)
		return err
	}
	if exists == 0 {
		utils.Warnf("[QUEUE] ActivateTask rejected taskID=%s reason=not_found", taskID)
		return ErrTaskNotFound
	}
	utils.Warnf("[QUEUE] ActivateTask rejected taskID=%s reason=not_pending status=%s", taskID, beforeStatus)
	return ErrTaskNotPending
}

// CompleteTaskTx marks ACTIVE task as terminal and inserts explicit follow-up tasks transactionally.
func CompleteTaskTx(tx *sql.Tx, taskID string, result models.CompletionResult) error {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return fmt.Errorf("task id is required")
	}
	utils.Warnf("[QUEUE] CompleteTaskTx reading task completion update start taskID=%s", taskID)
	status := strings.TrimSpace(string(result.Status))
	if status == "" {
		status = string(models.StudyTaskStatusCompleted)
	}
	if status != string(models.StudyTaskStatusCompleted) && status != string(models.StudyTaskStatusFailed) {
		return fmt.Errorf("completion status must be COMPLETED or FAILED")
	}

	// Note: Empty string payload preserves existing payload (sentinel value)
	// To clear payload, use a non-empty sentinel value in application logic
	res, err := tx.Exec(`
		UPDATE study_queue
		SET status = ?, completed_at = CURRENT_TIMESTAMP, payload_json = CASE WHEN ? = '' THEN payload_json ELSE ? END
		WHERE id = ? AND status = 'ACTIVE'
	`, status, strings.TrimSpace(result.Payload), strings.TrimSpace(result.Payload), taskID)
	if err != nil {
		utils.Warnf("[QUEUE] CompleteTaskTx reading task completion update error taskID=%s err=%v", taskID, err)
		return err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		utils.Warnf("[QUEUE] CompleteTaskTx reading task completion rows affected error taskID=%s err=%v", taskID, err)
		return err
	}
	if affected == 0 {
		var exists int
		if err := tx.QueryRow(`SELECT COUNT(*) FROM study_queue WHERE id = ?`, taskID).Scan(&exists); err != nil {
			utils.Warnf("[QUEUE] CompleteTaskTx reading task completion existence check error taskID=%s err=%v", taskID, err)
			return err
		}
		if exists == 0 {
			utils.Warnf("[QUEUE] CompleteTaskTx reading task completion task not found taskID=%s", taskID)
			return ErrTaskNotFound
		}
		utils.Warnf("[QUEUE] CompleteTaskTx reading task completion task not active taskID=%s", taskID)
		return ErrTaskNotActive
	}
	utils.Warnf("[QUEUE] CompleteTaskTx reading task completion update success taskID=%s", taskID)

	for _, followUp := range result.FollowUps {
		followUp.ID = strings.TrimSpace(followUp.ID)
		followUp.NotebookID = strings.TrimSpace(followUp.NotebookID)
		followUp.TopicID = strings.TrimSpace(followUp.TopicID)
		followUp.PayloadJSON = strings.TrimSpace(followUp.PayloadJSON)
		if followUp.ID == "" {
			return fmt.Errorf("follow-up task id is required")
		}
		if followUp.NotebookID == "" {
			return fmt.Errorf("follow-up notebook id is required")
		}
		if strings.TrimSpace(string(followUp.TaskType)) == "" {
			return fmt.Errorf("follow-up task type is required")
		}
		if strings.TrimSpace(string(followUp.Status)) == "" {
			followUp.Status = models.StudyTaskStatusPending
		}

		utils.Warnf("[QUEUE] CompleteTaskTx quiz task insertion start taskID=%s followUpID=%s taskType=%s", taskID, followUp.ID, followUp.TaskType)
		if _, err := tx.Exec(`
			INSERT INTO study_queue (
				id, notebook_id, topic_id, task_type, status, priority, payload_json, start_page, end_page
			) VALUES (?, ?, NULLIF(?, ''), ?, ?, ?, NULLIF(?, ''), ?, ?)
		`, followUp.ID, followUp.NotebookID, followUp.TopicID, string(followUp.TaskType), string(followUp.Status), followUp.Priority, followUp.PayloadJSON, followUp.StartPage, followUp.EndPage); err != nil {
			utils.Warnf("[QUEUE] CompleteTaskTx quiz task insertion error taskID=%s followUpID=%s err=%v", taskID, followUp.ID, err)
			return err
		}
		utils.Warnf("[QUEUE] CompleteTaskTx quiz task insertion success taskID=%s followUpID=%s", taskID, followUp.ID)
	}

	return nil
}

// CompleteTask marks ACTIVE task as terminal and inserts explicit follow-up tasks transactionally.
func CompleteTask(taskID string, result models.CompletionResult) error {
	utils.Warnf("[QUEUE] CompleteTask transaction start taskID=%s", strings.TrimSpace(taskID))
	tx, err := conn.Begin()
	if err != nil {
		utils.Warnf("[QUEUE] CompleteTask transaction begin error taskID=%s err=%v", strings.TrimSpace(taskID), err)
		return err
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err != sql.ErrTxDone {
			utils.Warnf("[QUEUE] CompleteTask rollback error taskID=%s err=%v", strings.TrimSpace(taskID), err)
		}
	}()

	if err := CompleteTaskTx(tx, taskID, result); err != nil {
		utils.Warnf("[QUEUE] CompleteTask transaction error taskID=%s err=%v", strings.TrimSpace(taskID), err)
		return err
	}

	if err := tx.Commit(); err != nil {
		utils.Warnf("[QUEUE] CompleteTask tx commit error taskID=%s err=%v", strings.TrimSpace(taskID), err)
		return err
	}
	utils.Warnf("[QUEUE] CompleteTask tx commit success taskID=%s", strings.TrimSpace(taskID))
	return nil
}

// SkipTask marks one task as SKIPPED.
func SkipTask(taskID string) error {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return fmt.Errorf("task id is required")
	}
	res, err := conn.Exec(`
		UPDATE study_queue
		SET status = 'SKIPPED', completed_at = CURRENT_TIMESTAMP
		WHERE id = ? AND status IN ('PENDING', 'ACTIVE')
	`, taskID)
	if err != nil {
		return err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 1 {
		return nil
	}
	var exists int
	if err := conn.QueryRow(`SELECT COUNT(*) FROM study_queue WHERE id = ?`, taskID).Scan(&exists); err != nil {
		return err
	}
	if exists == 0 {
		return ErrTaskNotFound
	}
	return fmt.Errorf("task cannot be skipped from current status")
}

// GetQueueState returns pending counts by task type, optionally filtered by notebook.
func GetQueueState(notebookID string) (models.QueueState, error) {
	notebookID = strings.TrimSpace(notebookID)
	state := models.QueueState{
		NotebookID: notebookID,
		Pending:    map[string]int{},
	}

	query := `
		SELECT task_type, COUNT(*)
		FROM study_queue
		WHERE status = 'PENDING'
	`
	args := make([]interface{}, 0, 1)
	if notebookID != "" {
		query += ` AND notebook_id = ?`
		args = append(args, notebookID)
	}
	query += ` GROUP BY task_type`

	rows, err := conn.Query(query, args...)
	if err != nil {
		return state, err
	}
	defer func() {
		_ = rows.Close()
	}()

	for rows.Next() {
		var taskType string
		var count int
		if err := rows.Scan(&taskType, &count); err != nil {
			return state, err
		}
		state.Pending[taskType] = count
		state.Total += count
	}
	if err := rows.Err(); err != nil {
		return state, err
	}
	return state, nil
}

// GetReadingTask returns one READING task with locked bounds and persisted cursor.
func GetReadingTask(taskID string) (*models.ReadingTask, error) {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return nil, fmt.Errorf("task id is required")
	}

	task := &models.ReadingTask{}
	err := conn.QueryRow(`
		SELECT
			sq.id,
			sq.notebook_id,
			COALESCE(sq.topic_id, ''),
			COALESCE(sq.start_page, 0),
			COALESCE(sq.end_page, 0),
			COALESCE(rp.current_page, COALESCE(sq.start_page, 0))
		FROM study_queue sq
		LEFT JOIN reading_progress rp ON rp.task_id = sq.id
		WHERE sq.id = ? AND sq.task_type = 'READING'
	`, taskID).Scan(
		&task.TaskID,
		&task.NotebookID,
		&task.TopicID,
		&task.StartPage,
		&task.EndPage,
		&task.CurrentPage,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrTaskNotFound
	}
	if err != nil {
		return nil, err
	}
	// If task was inserted without explicit page bounds, fall back to topic page bounds.
	// This allows READING tasks created without bounds to still be initialized and completed.
	if (task.StartPage <= 0 || task.EndPage <= 0) && task.TopicID != "" {
		var topicStart, topicEnd int
		boundsErr := conn.QueryRow(`
			SELECT COALESCE(start_page, 1), COALESCE(end_page, start_page)
			FROM topics WHERE id = ?
		`, task.TopicID).Scan(&topicStart, &topicEnd)
		if boundsErr == nil && topicStart > 0 && topicEnd >= topicStart {
			if task.StartPage <= 0 {
				task.StartPage = topicStart
			}
			if task.EndPage <= 0 {
				task.EndPage = topicEnd
			}
		}
	}
	// After fallback: if bounds are still missing or invalid, return an explicit error.
	if task.StartPage <= 0 || task.EndPage <= 0 {
		return nil, fmt.Errorf("reading task has no valid page bounds: startPage=%d, endPage=%d — set start_page/end_page on the task or ensure topic has page bounds", task.StartPage, task.EndPage)
	}
	if task.EndPage < task.StartPage {
		return nil, fmt.Errorf("reading task has invalid page bounds: endPage=%d must be >= startPage=%d", task.EndPage, task.StartPage)
	}

	// Clamp current page to bounds
	if task.CurrentPage < task.StartPage {
		task.CurrentPage = task.StartPage
	}
	if task.CurrentPage > task.EndPage {
		task.CurrentPage = task.EndPage
	}
	return task, nil
}

// PersistReadingProgress persists page progress without validating completion.
// Used in trust-based completion model where user decides when reading is complete.
func PersistReadingProgress(taskID string, finalPage int) (bool, error) {
	task, err := GetReadingTask(taskID)
	if err != nil {
		return false, err
	}
	reachedEnd := finalPage >= task.EndPage
	if finalPage < task.StartPage {
		finalPage = task.StartPage
	}
	if finalPage > task.EndPage {
		finalPage = task.EndPage
	}

	tx, err := conn.Begin()
	if err != nil {
		return false, err
	}
	defer func() {
		_ = tx.Rollback()
	}()

	_, err = tx.Exec(`
		INSERT INTO reading_progress (task_id, current_page, last_accessed_at)
		VALUES (?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(task_id) DO UPDATE
		SET current_page = excluded.current_page,
		    last_accessed_at = CURRENT_TIMESTAMP
	`, task.TaskID, finalPage)
	if err != nil {
		return false, err
	}

	// Synchronize topics.current_page_cursor to keep both cursor systems aligned
	if task.TopicID != "" {
		_, err = tx.Exec(`
			UPDATE topics
			SET current_page_cursor = ?,
			    updated_at = CURRENT_TIMESTAMP
			WHERE id = ? AND current_page_cursor < ?
		`, finalPage, task.TopicID, finalPage)
		if err != nil {
			return false, err
		}
	}

	if err = tx.Commit(); err != nil {
		return false, err
	}

	return reachedEnd, nil
}

// ValidateReadingCompletion persists page progress and returns true when final page is reached.
// DEPRECATED: Use PersistReadingProgress for trust-based completion.
func ValidateReadingCompletion(taskID string, finalPage int) (bool, error) {
	// Delegate to PersistReadingProgress - trust-based model
	return PersistReadingProgress(taskID, finalPage)
}

// CompleteReading completes an ACTIVE READING task only if user reached end_page and inserts QUIZ follow-up.
func CompleteReading(taskID string) error {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return fmt.Errorf("task id is required")
	}

	type completionSeed struct {
		ID         string
		NotebookID string
		TopicID    string
		StartPage  int
		EndPage    int
	}
	seed := completionSeed{}
	var currentPage int
	var status string

	err := conn.QueryRow(`
		SELECT
			sq.id,
			sq.notebook_id,
			COALESCE(sq.topic_id, ''),
			COALESCE(sq.start_page, 0),
			COALESCE(sq.end_page, 0),
			sq.status,
			COALESCE(rp.current_page, COALESCE(sq.start_page, 0))
		FROM study_queue sq
		LEFT JOIN reading_progress rp ON rp.task_id = sq.id
		WHERE sq.id = ? AND sq.task_type = 'READING'
	`, taskID).Scan(
		&seed.ID,
		&seed.NotebookID,
		&seed.TopicID,
		&seed.StartPage,
		&seed.EndPage,
		&status,
		&currentPage,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrTaskNotFound
	}
	if err != nil {
		return err
	}
	if status != string(models.StudyTaskStatusActive) {
		return ErrTaskNotActive
	}
	// In backend-authoritative session model, 0 means "unspecified / hydrate from DB session state
	// Reject only: negative values, or invalid ordering when both are specified
	if seed.StartPage < 0 || seed.EndPage < 0 {
		return fmt.Errorf("reading task has invalid page bounds: negative values not allowed")
	}
	if seed.StartPage > 0 && seed.EndPage > 0 && seed.EndPage < seed.StartPage {
		return fmt.Errorf("reading task has invalid page bounds: endPage must be >= startPage when both specified")
	}

	return CompleteTask(seed.ID, models.CompletionResult{
		Status: models.StudyTaskStatusCompleted,
		FollowUps: []models.StudyQueueTask{
			{
				ID:         uuid.NewString(),
				NotebookID: seed.NotebookID,
				TopicID:    seed.TopicID,
				TaskType:   models.StudyTaskTypeQuiz,
				Status:     models.StudyTaskStatusPending,
				Priority:   0,
				StartPage:  seed.StartPage,
				EndPage:    seed.EndPage,
			},
		},
	})
}

// CompleteReadingWithGeneratedQuiz completes ACTIVE READING task and inserts a QUIZ follow-up with payload.
func CompleteReadingWithGeneratedQuiz(taskID string, quizPayload models.QuizTaskPayload) (string, error) {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return "", fmt.Errorf("task id is required")
	}
	if len(quizPayload.Questions) == 0 {
		return "", fmt.Errorf("quiz payload must include questions")
	}
	if quizPayload.PassingScore <= 0 {
		quizPayload.PassingScore = 70
	}
	payloadBytes, err := json.Marshal(quizPayload)
	if err != nil {
		return "", fmt.Errorf("failed to marshal quiz payload: %w", err)
	}

	type completionSeed struct {
		ID         string
		NotebookID string
		TopicID    string
		StartPage  int
		EndPage    int
	}
	seed := completionSeed{}
	var currentPage int
	var status string

	err = conn.QueryRow(`
		SELECT
			sq.id,
			sq.notebook_id,
			COALESCE(sq.topic_id, ''),
			COALESCE(sq.start_page, 0),
			COALESCE(sq.end_page, 0),
			sq.status,
			COALESCE(rp.current_page, COALESCE(sq.start_page, 0))
		FROM study_queue sq
		LEFT JOIN reading_progress rp ON rp.task_id = sq.id
		WHERE sq.id = ? AND sq.task_type = 'READING'
	`, taskID).Scan(
		&seed.ID,
		&seed.NotebookID,
		&seed.TopicID,
		&seed.StartPage,
		&seed.EndPage,
		&status,
		&currentPage,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrTaskNotFound
	}
	if err != nil {
		return "", err
	}
	if status != string(models.StudyTaskStatusActive) {
		return "", ErrTaskNotActive
	}

	// Synchronize topics.current_page_cursor to keep both cursor systems aligned.
	// Completion is authoritative for the assigned reading window, so cursor must
	// advance to at least end_page to prevent scheduler rematerializing the same window.
	if seed.TopicID != "" {
		cursorAfterCompletion := currentPage
		if seed.EndPage > cursorAfterCompletion {
			cursorAfterCompletion = seed.EndPage
		}
		_, err = conn.Exec(`
			UPDATE topics
			SET current_page_cursor = ?,
			    updated_at = CURRENT_TIMESTAMP
			WHERE id = ? AND current_page_cursor < ?
		`, cursorAfterCompletion, seed.TopicID, cursorAfterCompletion)
		if err != nil {
			return "", fmt.Errorf("failed to synchronize topic cursor: %w", err)
		}
	}

	quizTaskID := uuid.NewString()
	err = CompleteTask(seed.ID, models.CompletionResult{
		Status: models.StudyTaskStatusCompleted,
		FollowUps: []models.StudyQueueTask{
			{
				ID:          quizTaskID,
				NotebookID:  seed.NotebookID,
				TopicID:     seed.TopicID,
				TaskType:    models.StudyTaskTypeQuiz,
				Status:      models.StudyTaskStatusPending,
				Priority:    0,
				PayloadJSON: string(payloadBytes),
				StartPage:   seed.StartPage,
				EndPage:     seed.EndPage,
			},
		},
	})
	if err != nil {
		return "", err
	}
	return quizTaskID, nil
}

func GetChunkIDsForTopicPageRange(topicID string, startPage int, endPage int) ([]string, error) {
	topicID = strings.TrimSpace(topicID)
	if topicID == "" {
		return nil, fmt.Errorf("topic id is required")
	}
	if startPage <= 0 || endPage <= 0 {
		return nil, fmt.Errorf("start page and end page must be positive")
	}
	if startPage > endPage {
		startPage, endPage = endPage, startPage
	}
	rows, err := conn.Query(`
		SELECT id
		FROM chunks
		WHERE topic_id = ? AND page_num BETWEEN ? AND ?
		ORDER BY page_num ASC, id ASC
	`, topicID, startPage, endPage)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = rows.Close()
	}()
	ids := make([]string, 0)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return ids, nil
}
