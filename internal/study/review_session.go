package study

import (
	"ai-tutor/internal/db"
	"ai-tutor/internal/models"
	"ai-tutor/internal/scheduler"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	fsrs "github.com/open-spaced-repetition/go-fsrs/v4"
)

func (s *StudyService) GetReviewSession(taskID string) (*models.ReviewSession, error) {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return nil, fmt.Errorf("task ID is required")
	}
	return s.repo.GetReviewSession(taskID)
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
		card, state, err = s.repo.GetFlashcardByIDTx(tx, cardID)
	} else {
		card, state, err = s.repo.GetFlashcardByID(cardID)
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
		lastReviewedAt, err = s.repo.GetLastFlashcardReviewTimeTx(tx, cardID)
	} else {
		lastReviewedAt, err = s.repo.GetLastFlashcardReviewTime(cardID)
	}
	if err != nil {
		return nil, nil, "", fmt.Errorf("failed to retrieve last reviewed time: %w", err)
	}

	fsrsCard := FlashcardStateToCard(*state, card.DueAt, lastReviewedAt)
	originalLastReview := fsrsCard.LastReview
	tNow := time.Now()

	nextFsrsCard, err := scheduler.NextFSRSState(fsrsCard, ratingCode, tNow)
	if err != nil {
		return nil, nil, "", err
	}
	nextState := models.CardToFlashcardState(nextFsrsCard)

	// Calculate ElapsedDays
	if !originalLastReview.IsZero() {
		elapsedDays = int(tNow.Sub(originalLastReview).Hours() / 24)
		if elapsedDays < 0 {
			elapsedDays = 0
		}
		nextState.ElapsedDays = elapsedDays
	}
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
		ReviewedAt:      time.Now().Unix(),
		Rating:          ratingCode,
		ScheduledDays:   nextState.ScheduledDays,
		StateBeforeJSON: string(stateBeforeJSONBytes),
		StateAfterJSON:  string(stateAfterJSONBytes),
	}
	if tx != nil {
		if err := s.repo.UpdateFlashcardReviewTx(tx, cardID, dueAt, card.DueAt, string(stateBeforeJSONBytes), nextState, reviewLog); err != nil {
			return nil, nil, "", err
		}
	} else {
		if err := s.repo.UpdateFlashcardReview(cardID, dueAt, card.DueAt, string(stateBeforeJSONBytes), nextState, reviewLog); err != nil {
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

	return s.withFlashcardTaskTx(taskID, func(tx *sql.Tx) error {
		if err := s.repo.MarkReviewTaskCardReviewedTx(tx, taskID, cardID); err != nil {
			return err
		}
		if _, _, _, err := s.applyFlashcardReview(tx, cardID, rating); err != nil {
			return err
		}
		return nil
	})
}

func (s *StudyService) CompleteReviewSession(taskID string) error {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return fmt.Errorf("task ID is required")
	}
	return s.repo.CompleteReviewSession(taskID)
}

// SuspendFlashcard marks a card as suspended, removing it from future reviews.
// Returns the remaining pending card count in the current session.
func (s *StudyService) SuspendFlashcard(taskID, cardID string) (int, error) {
	taskID = strings.TrimSpace(taskID)
	cardID = strings.TrimSpace(cardID)
	if taskID == "" || cardID == "" {
		return 0, fmt.Errorf("task ID and card ID are required")
	}

	return s.withFlashcardTaskTx(taskID, func(tx *sql.Tx) error {
		if err := s.repo.SuspendFlashcardTx(tx, cardID); err != nil {
			return err
		}
		if err := s.repo.MarkReviewTaskCardReviewedTx(tx, taskID, cardID); err != nil && !errors.Is(err, db.ErrReviewLinkNotPending) {
			return err
		}
		return nil
	})
}

func (s *StudyService) withFlashcardTaskTx(taskID string, fn func(tx *sql.Tx) error) (int, error) {
	tx, err := s.repo.Begin()
	if err != nil {
		return 0, err
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	task, err := s.repo.GetTaskByIDTx(tx, taskID)
	if err != nil {
		return 0, err
	}
	if task.TaskType != models.StudyTaskTypeFlashcardReview {
		return 0, fmt.Errorf("task %s is not a flashcard review task", taskID)
	}
	if task.Status != models.StudyTaskStatusActive {
		return 0, db.ErrTaskNotActive
	}

	if err := fn(tx); err != nil {
		return 0, err
	}

	remaining, err := s.repo.RemainingReviewTaskCardsTx(tx, taskID)
	if err != nil {
		return 0, err
	}

	if err := tx.Commit(); err != nil {
		return 0, err
	}
	committed = true
	return remaining, nil
}

const UnixMillisecondThreshold = 1e12

func FlashcardStateToCard(state models.FlashcardState, dueAt, lastReviewedAt int64) fsrs.Card {
	var dueTime, lastReviewTime time.Time
	if dueAt > 0 {
		dueTime = time.Unix(dueAt, 0)
	}
	if lastReviewedAt > 0 {
		if lastReviewedAt > UnixMillisecondThreshold {
			lastReviewTime = time.UnixMilli(lastReviewedAt)
		} else {
			lastReviewTime = time.Unix(lastReviewedAt, 0)
		}
	}

	var fsrsState fsrs.State
	switch state.StateCode {
	case 0:
		fsrsState = fsrs.New
	case 1:
		fsrsState = fsrs.Learning
	case 2:
		fsrsState = fsrs.Review
	case 3:
		fsrsState = fsrs.Relearning
	default:
		fsrsState = fsrs.New
	}

	stability := state.Stability
	if fsrsState != fsrs.New && stability < 0.001 {
		stability = 0.001
	}

	return fsrs.Card{
		Due:            dueTime,
		Stability:      stability,
		Difficulty:     state.Difficulty,
		ScheduledDays:  uint64(state.ScheduledDays),
		Reps:           uint64(state.Reps),
		Lapses:         uint64(state.Lapses),
		State:          fsrsState,
		LastReview:     lastReviewTime,
		RemainingSteps: 0,
	}
}
