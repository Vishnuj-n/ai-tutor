package db

import (
	"ai-tutor/internal/models"
	"database/sql"
	"errors"
	"fmt"
	"strings"
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

// GetNextTask returns the next pending task ordered by deterministic queue rules.
func GetNextTask(notebookID string) (*models.StudyQueueTask, error) {
	notebookID = strings.TrimSpace(notebookID)

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
	return task, nil
}

// ActivateTask moves one task from PENDING to ACTIVE.
func ActivateTask(taskID string) error {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return fmt.Errorf("task id is required")
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
		return nil
	}
	var exists int
	if err := conn.QueryRow(`SELECT COUNT(*) FROM study_queue WHERE id = ?`, taskID).Scan(&exists); err != nil {
		return err
	}
	if exists == 0 {
		return ErrTaskNotFound
	}
	return ErrTaskNotPending
}

// CompleteTask marks ACTIVE task as terminal and inserts explicit follow-up tasks transactionally.
func CompleteTask(taskID string, result models.CompletionResult) error {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return fmt.Errorf("task id is required")
	}
	status := strings.TrimSpace(string(result.Status))
	if status == "" {
		status = string(models.StudyTaskStatusCompleted)
	}
	if status != string(models.StudyTaskStatusCompleted) && status != string(models.StudyTaskStatusFailed) {
		return fmt.Errorf("completion status must be COMPLETED or FAILED")
	}

	tx, err := conn.Begin()
	if err != nil {
		return err
	}
	defer func() {
		_ = tx.Rollback()
	}()

	// Note: Empty string payload preserves existing payload (sentinel value)
	// To clear payload, use a non-empty sentinel value in application logic
	res, err := tx.Exec(`
		UPDATE study_queue
		SET status = ?, completed_at = CURRENT_TIMESTAMP, payload_json = CASE WHEN ? = '' THEN payload_json ELSE ? END
		WHERE id = ? AND status = 'ACTIVE'
	`, status, strings.TrimSpace(result.Payload), strings.TrimSpace(result.Payload), taskID)
	if err != nil {
		return err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		var exists int
		if err := tx.QueryRow(`SELECT COUNT(*) FROM study_queue WHERE id = ?`, taskID).Scan(&exists); err != nil {
			return err
		}
		if exists == 0 {
			return ErrTaskNotFound
		}
		return ErrTaskNotActive
	}

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

		if _, err := tx.Exec(`
			INSERT INTO study_queue (
				id, notebook_id, topic_id, task_type, status, priority, payload_json, start_page, end_page
			) VALUES (?, ?, NULLIF(?, ''), ?, ?, ?, NULLIF(?, ''), ?, ?)
		`, followUp.ID, followUp.NotebookID, followUp.TopicID, string(followUp.TaskType), string(followUp.Status), followUp.Priority, followUp.PayloadJSON, followUp.StartPage, followUp.EndPage); err != nil {
			return err
		}
	}

	return tx.Commit()
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
