package study

import (
	"ai-tutor/internal/db"
	"ai-tutor/internal/models"
	"ai-tutor/internal/scheduler"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

func (s *StudyService) GenerateReviewTasks(notebookID string) ([]models.StudyQueueTask, error) {
	notebookID = strings.TrimSpace(notebookID)
	if notebookID == "" {
		return nil, fmt.Errorf("notebook ID is required")
	}
	task, _, err := db.CreateReviewSession(notebookID)
	if err != nil {
		return nil, err
	}
	if task == nil {
		return []models.StudyQueueTask{}, nil
	}
	return []models.StudyQueueTask{*task}, nil
}

func (s *StudyService) GetReviewSession(taskID string) (*models.ReviewSession, error) {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return nil, fmt.Errorf("task ID is required")
	}
	return db.GetReviewSession(taskID)
}

func (s *StudyService) ApplyFlashcardReview(cardID string, ratingCode int) (*models.Flashcard, *models.FlashcardState, string, error) {
	return s.applyFlashcardReview(nil, cardID, ratingCode)
}

func (s *StudyService) applyFlashcardReview(tx *sql.Tx, cardID string, ratingCode int) (*models.Flashcard, *models.FlashcardState, string, error) {
	var (
		card  *models.Flashcard
		state *models.FlashcardState
		err   error
	)
	if tx != nil {
		card, state, err = db.GetFlashcardByIDTx(tx, cardID)
	} else {
		card, state, err = db.GetFlashcardByID(cardID)
	}
	if err != nil {
		return nil, nil, "", err
	}
	if card == nil || state == nil {
		return nil, nil, "", fmt.Errorf("flashcard not found")
	}
	stateBeforeJSONBytes, err := json.Marshal(state)
	if err != nil {
		return nil, nil, "", fmt.Errorf("failed to encode flashcard state: %w", err)
	}
	now := time.Now().Unix()
	elapsedSeconds := now - card.DueAt
	elapsedDays := 0
	if elapsedSeconds > 0 {
		elapsedDays = int(elapsedSeconds / (24 * 60 * 60))
	}
	state.ElapsedDays = elapsedDays

	var lastReviewedAt int64
	if tx != nil {
		err = tx.QueryRow(`SELECT COALESCE(MAX(reviewed_at), 0) FROM fsrs_review_log WHERE activity_type = 'flashcard' AND reference_id = ?`, cardID).Scan(&lastReviewedAt)
	} else {
		err = db.GetConnection().QueryRow(`SELECT COALESCE(MAX(reviewed_at), 0) FROM fsrs_review_log WHERE activity_type = 'flashcard' AND reference_id = ?`, cardID).Scan(&lastReviewedAt)
	}
	if err != nil {
		return nil, nil, "", fmt.Errorf("failed to retrieve last reviewed time: %w", err)
	}

	nextState := scheduler.NextFSRSState(*state, ratingCode, time.Now(), card.DueAt, lastReviewedAt)
	dueAt := now + int64(nextState.ScheduledDays)*24*60*60
	if nextState.ScheduledDays == 0 {
		dueAt = now
	}
	stateAfterJSONBytes, err := json.Marshal(nextState)
	if err != nil {
		return nil, nil, "", fmt.Errorf("failed to encode updated flashcard state: %w", err)
	}
	reviewLog := models.FSRSReviewLog{
		ID:              uuid.NewString(),
		TopicID:         card.TopicID,
		ActivityType:    "flashcard",
		ReferenceID:     card.ID,
		ReviewedAt:      now,
		Rating:          ratingCode,
		ScheduledDays:   nextState.ScheduledDays,
		StateBeforeJSON: string(stateBeforeJSONBytes),
		StateAfterJSON:  string(stateAfterJSONBytes),
	}
	if tx != nil {
		if err := db.UpdateFlashcardReviewTx(tx, cardID, dueAt, card.DueAt, nextState, reviewLog); err != nil {
			return nil, nil, "", err
		}
	} else {
		if err := db.UpdateFlashcardReview(cardID, dueAt, card.DueAt, nextState, reviewLog); err != nil {
			return nil, nil, "", err
		}
	}
	card.DueAt = dueAt
	return card, &nextState, reviewLog.ID, nil
}

func (s *StudyService) RecordCardReview(taskID, cardID string, rating int) (int, error) {
	taskID = strings.TrimSpace(taskID)
	cardID = strings.TrimSpace(cardID)
	if taskID == "" || cardID == "" {
		return 0, fmt.Errorf("task ID and card ID are required")
	}
	if rating < scheduler.Again || rating > scheduler.Easy {
		return 0, fmt.Errorf("rating must be between 1 and 4")
	}

	tx, err := db.GetConnection().Begin()
	if err != nil {
		return 0, err
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	task, err := getTaskForReviewTx(tx, taskID)
	if err != nil {
		return 0, err
	}
	if task.TaskType != models.StudyTaskTypeFlashcardReview {
		return 0, fmt.Errorf("task %s is not a flashcard review task", taskID)
	}
	if task.Status != models.StudyTaskStatusActive {
		return 0, db.ErrTaskNotActive
	}

	if err := db.MarkReviewTaskCardReviewedTx(tx, taskID, cardID); err != nil {
		return 0, err
	}
	if _, _, _, err := s.applyFlashcardReview(tx, cardID, rating); err != nil {
		return 0, err
	}

	remaining, err := db.RemainingReviewTaskCardsTx(tx, taskID)
	if err != nil {
		return 0, err
	}

	if err := tx.Commit(); err != nil {
		return 0, err
	}
	committed = true
	return remaining, nil
}

func (s *StudyService) CompleteReviewSession(taskID string) error {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return fmt.Errorf("task ID is required")
	}
	return db.CompleteReviewSession(taskID)
}

func getTaskForReviewTx(tx *sql.Tx, taskID string) (*models.StudyQueueTask, error) {
	task := &models.StudyQueueTask{}
	err := tx.QueryRow(`
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
	if err == sql.ErrNoRows {
		return nil, db.ErrTaskNotFound
	}
	if err != nil {
		return nil, err
	}
	return task, nil
}
