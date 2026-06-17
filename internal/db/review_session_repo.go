package db

import (
	"ai-tutor/internal/models"
	"ai-tutor/internal/utils"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

const maxReviewSessionCards = 60

var (
	ErrReviewLinkNotPending  = errors.New("review task card link is not pending")
	ErrReviewSessionComplete = errors.New("review session already complete")
	ErrReviewSessionOpen     = errors.New("review session still has pending cards")
)

func (r *Repository) fetchExistingReviewTask(q querier, notebookID string) (*models.StudyQueueTask, error) {
	task := &models.StudyQueueTask{}
	err := q.QueryRow(`
		SELECT
			id, notebook_id, COALESCE(topic_id, ''), task_type, status, priority,
			COALESCE(created_at, ''), COALESCE(activated_at, ''), COALESCE(completed_at, ''),
			COALESCE(payload_json, ''), COALESCE(start_page, 0), COALESCE(end_page, 0)
		FROM study_queue
		WHERE notebook_id = ?
		  AND task_type = 'FLASHCARD_REVIEW'
		  AND status IN ('PENDING', 'ACTIVE')
		ORDER BY
			CASE status WHEN 'ACTIVE' THEN 0 ELSE 1 END,
			created_at ASC,
			id ASC
		LIMIT 1
	`, notebookID).Scan(
		&task.ID, &task.NotebookID, &task.TopicID, &task.TaskType, &task.Status, &task.Priority,
		&task.CreatedAt, &task.ActivatedAt, &task.CompletedAt, &task.PayloadJSON, &task.StartPage, &task.EndPage,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return task, nil
}

func (r *Repository) GetDueReviewCardsForNotebook(notebookID string, now int64, limit int) ([]models.Flashcard, error) {
	utils.Warnf("[FLASHCARD_PIPELINE] due_card_scan notebookID=%s now=%d limit=%d", notebookID, now, limit)
	rows, err := r.db.Query(`
		SELECT
			fc.id,
			fc.topic_id,
			COALESCE(fc.source_chunk_id, ''),
			fc.prompt,
			fc.answer,
			COALESCE(fc.due_at, 0),
			fc.suspended
		FROM fsrs_cards fc
		WHERE EXISTS (
			SELECT 1
			FROM notebooks n
			LEFT JOIN notebook_topics nt
				ON nt.notebook_id = n.id
			   AND nt.topic_id = fc.topic_id
			WHERE n.id = ?
			  AND (
				nt.topic_id IS NOT NULL
				OR COALESCE(n.topic_id, '') = fc.topic_id
			  )
		)
		  AND fc.suspended = 0
		  AND fc.due_at IS NOT NULL
		  AND fc.due_at <= ?
		  AND NOT EXISTS (
			SELECT 1
			FROM review_task_cards rtc
			JOIN study_queue sq ON sq.id = rtc.task_id
			WHERE rtc.card_id = fc.id
			  AND sq.task_type = 'FLASHCARD_REVIEW'
			  AND sq.status IN ('PENDING', 'ACTIVE')
		  )
		ORDER BY fc.due_at ASC, fc.created_at ASC, fc.id ASC
		LIMIT ?
	`, notebookID, now, limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	cards := make([]models.Flashcard, 0)
	for rows.Next() {
		var card models.Flashcard
		var suspended bool
		if err := rows.Scan(&card.ID, &card.TopicID, &card.SourceChunkID, &card.Prompt, &card.Answer, &card.DueAt, &suspended); err != nil {
			return nil, err
		}
		card.Suspended = suspended
		cards = append(cards, card)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	utils.Warnf("[FLASHCARD_PIPELINE] due_card_scan result notebookID=%s dueCards=%d", notebookID, len(cards))
	return cards, nil
}

func (r *Repository) GetNextDueReviewNotebook(now int64) (string, int, error) {
	settings, err := r.GetUserSettings()
	if err != nil {
		return "", 0, fmt.Errorf("getNextDueReviewNotebook: getting user settings: %w", err)
	}
	activeProfileStr := settings.ActiveProfileID

	var notebookID string
	var dueCount int
	query := `
		SELECT
			n.id,
			COUNT(fc.id) AS due_count
		FROM notebooks n
		JOIN fsrs_cards fc
		  ON (
			COALESCE(n.topic_id, '') = fc.topic_id
			OR EXISTS (
				SELECT 1
				FROM notebook_topics nt
				WHERE nt.notebook_id = n.id
				  AND nt.topic_id = fc.topic_id
			)
		  )
		JOIN topics t ON t.id = fc.topic_id
		WHERE fc.suspended = 0
		  AND fc.due_at IS NOT NULL
		  AND fc.due_at <= ?
		  AND NOT EXISTS (
			SELECT 1
			FROM review_task_cards rtc
			JOIN study_queue sq ON sq.id = rtc.task_id
			WHERE rtc.card_id = fc.id
			  AND sq.task_type = 'FLASHCARD_REVIEW'
			  AND sq.status IN ('PENDING', 'ACTIVE')
		  )
	`
	var args []interface{}
	args = append(args, now)
	if activeProfileStr != "" {
		query += ` AND (n.profile_id = ? OR n.profile_id IS NULL OR n.profile_id = '') `
		args = append(args, activeProfileStr)
	}

	query += `
		GROUP BY n.id, COALESCE(n.priority, 5)
		ORDER BY due_count DESC, COALESCE(n.priority, 5) DESC, n.id ASC
		LIMIT 1
	`

	err = r.db.QueryRow(query, args...).Scan(&notebookID, &dueCount)
	if errors.Is(err, sql.ErrNoRows) {
		return "", 0, nil
	}
	if err != nil {
		return "", 0, err
	}
	return notebookID, dueCount, nil
}

func (r *Repository) CreateReviewSession(notebookID string) (*models.StudyQueueTask, bool, error) {
	now := reviewSessionNow()
	utils.Warnf("[FLASHCARD_PIPELINE] review_task_creation start notebookID=%s now=%d", notebookID, now)
	if existing, err := r.fetchExistingReviewTask(r.db, notebookID); err != nil {
		return nil, false, err
	} else if existing != nil {
		utils.Warnf("[FLASHCARD_PIPELINE] review_task_creation reused_existing notebookID=%s taskID=%s status=%s", notebookID, existing.ID, existing.Status)
		return existing, true, nil
	}

	cards, err := r.GetDueReviewCardsForNotebook(notebookID, now, maxReviewSessionCards)
	if err != nil {
		return nil, false, err
	}
	if len(cards) == 0 {
		utils.LogReviewSession("", notebookID, "0", "no_due_cards")
		utils.Warnf("[FLASHCARD_PIPELINE] review_task_creation skipped notebookID=%s reason=no_due_cards", notebookID)
		return nil, false, nil
	}

	var task *models.StudyQueueTask
	var reused bool
	err = r.withTx(func(tx *sql.Tx) error {
		if existing, err := r.getExistingReviewTaskForNotebookTxRepo(tx, notebookID); err != nil {
			return err
		} else if existing != nil {
			task = existing
			reused = true
			return nil
		}

		payloadBytes, err := json.Marshal(models.ReviewSessionPayload{
			CardCount:     len(cards),
			CreatedAtUnix: now,
		})
		if err != nil {
			return err
		}

		// Determine if the session spans a single topic or multiple topics
		sessionTopicID := ""
		if len(cards) > 0 {
			sameTopic := true
			firstTopic := cards[0].TopicID
			for _, card := range cards {
				if card.TopicID != firstTopic {
					sameTopic = false
					break
				}
			}
			if sameTopic {
				sessionTopicID = firstTopic
			}
		}

		task = &models.StudyQueueTask{
			ID:          uuid.NewString(),
			NotebookID:  notebookID,
			TopicID:     sessionTopicID,
			TaskType:    models.StudyTaskTypeFlashcardReview,
			Status:      models.StudyTaskStatusPending,
			Priority:    0,
			PayloadJSON: string(payloadBytes),
		}
		if _, err := tx.Exec(`
			INSERT INTO study_queue (
				id, notebook_id, topic_id, task_type, status, priority, payload_json
			) VALUES (?, ?, NULLIF(?, ''), ?, ?, ?, ?)
		`, task.ID, task.NotebookID, task.TopicID, string(task.TaskType), string(task.Status), task.Priority, task.PayloadJSON); err != nil {
			return err
		}
		utils.Warnf("[FLASHCARD_PIPELINE] queue_insertion taskID=%s taskType=%s notebookID=%s topicID=%s cardCount=%d", task.ID, task.TaskType, task.NotebookID, task.TopicID, len(cards))

		for _, card := range cards {
			if _, err := tx.Exec(`
				INSERT INTO review_task_cards (task_id, card_id, status)
				VALUES (?, ?, 'pending')
			`, task.ID, card.ID); err != nil {
				return err
			}
		}
		return nil
	})

	if err != nil {
		return nil, false, err
	}
	if reused {
		return task, true, nil
	}

	utils.LogReviewSession(task.ID, notebookID, strconv.Itoa(len(cards)), "session_created")
	utils.Warnf("[FLASHCARD_PIPELINE] review_task_creation committed taskID=%s notebookID=%s linkedCards=%d", task.ID, notebookID, len(cards))
	createdTask, err := r.GetTaskByID(task.ID)
	return createdTask, false, err
}

func (r *Repository) getExistingReviewTaskForNotebookTxRepo(tx *sql.Tx, notebookID string) (*models.StudyQueueTask, error) {
	return r.fetchExistingReviewTask(tx, notebookID)
}

func (r *Repository) GetReviewSession(taskID string) (*models.ReviewSession, error) {
	task, err := r.GetTaskByID(taskID)
	if err != nil {
		return nil, err
	}
	if task.TaskType != models.StudyTaskTypeFlashcardReview {
		return nil, fmt.Errorf("task %s is not a flashcard review task", taskID)
	}

	session := &models.ReviewSession{
		Task:           task,
		Cards:          make([]models.ReviewSessionCard, 0),
		NextPendingIdx: -1,
	}
	if payload := strings.TrimSpace(task.PayloadJSON); payload != "" {
		_ = json.Unmarshal([]byte(payload), &session.Payload)
	}

	rows, err := r.db.Query(`
		SELECT
			rtc.card_id,
			rtc.status,
			fc.topic_id,
			COALESCE(fc.source_chunk_id, ''),
			fc.prompt,
			fc.answer,
			COALESCE(fc.due_at, 0),
			fc.suspended
		FROM review_task_cards rtc
		JOIN fsrs_cards fc ON fc.id = rtc.card_id
		WHERE rtc.task_id = ?
		ORDER BY
			CASE rtc.status WHEN 'pending' THEN 0 ELSE 1 END,
			fc.due_at ASC,
			fc.created_at ASC,
			fc.id ASC
	`, taskID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var card models.ReviewSessionCard
		var suspended bool
		card.TaskID = taskID
		if err := rows.Scan(&card.CardID, &card.Status, &card.TopicID, &card.SourceChunkID, &card.Prompt, &card.Answer, &card.DueAt, &suspended); err != nil {
			return nil, err
		}
		card.Suspended = suspended
		card.Position = len(session.Cards)
		session.Cards = append(session.Cards, card)
		if card.Status == models.ReviewTaskCardStatusPending {
			if session.NextPendingIdx == -1 {
				session.NextPendingIdx = card.Position
			}
			session.Remaining++
		} else {
			session.ReviewedCount++
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	session.CardCount = len(session.Cards)
	if session.Payload.CardCount == 0 {
		session.Payload.CardCount = session.CardCount
	}
	if session.NextPendingIdx >= 0 {
		session.CurrentCard = &session.Cards[session.NextPendingIdx]
	}
	return session, nil
}

func (r *Repository) MarkReviewTaskCardReviewedTx(tx *sql.Tx, taskID, cardID string) error {
	res, err := tx.Exec(`
		UPDATE review_task_cards
		SET status = 'reviewed'
		WHERE task_id = ? AND card_id = ? AND status = 'pending'
	`, taskID, cardID)
	if err != nil {
		return err
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 1 {
		return nil
	}

	var status string
	err = tx.QueryRow(`
		SELECT COALESCE(status, '')
		FROM review_task_cards
		WHERE task_id = ? AND card_id = ?
	`, taskID, cardID).Scan(&status)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrTaskNotFound
	}
	if err != nil {
		return err
	}
	if status == string(models.ReviewTaskCardStatusReviewed) {
		return ErrReviewLinkNotPending
	}
	return fmt.Errorf("unexpected review task card state")
}

func (r *Repository) RemainingReviewTaskCardsTx(tx *sql.Tx, taskID string) (int, error) {
	var remaining int
	err := tx.QueryRow(`
		SELECT COUNT(*)
		FROM review_task_cards
		WHERE task_id = ? AND status = 'pending'
	`, taskID).Scan(&remaining)
	return remaining, err
}

func (r *Repository) CompleteReviewSession(taskID string) error {
	return r.withTx(func(tx *sql.Tx) error {
		task, err := r.GetTaskByIDTx(tx, taskID)
		if err != nil {
			return err
		}
		if task.TaskType != models.StudyTaskTypeFlashcardReview {
			return fmt.Errorf("task %s is not a flashcard review task", taskID)
		}
		if task.Status != models.StudyTaskStatusActive {
			return ErrTaskNotActive
		}

		remaining, err := r.RemainingReviewTaskCardsTx(tx, taskID)
		if err != nil {
			return err
		}
		if remaining > 0 {
			return ErrReviewSessionOpen
		}

		if _, err := tx.Exec(`
			UPDATE study_queue
			SET status = 'COMPLETED', completed_at = CURRENT_TIMESTAMP
			WHERE id = ? AND status = 'ACTIVE'
		`, taskID); err != nil {
			return err
		}
		utils.LogReviewSession(taskID, "", "0", "session_completed")
		return nil
	})
}

// GetTaskByIDTx returns one queue task by ID within a transaction scope.
func (r *Repository) GetTaskByIDTx(tx *sql.Tx, taskID string) (*models.StudyQueueTask, error) {
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
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrTaskNotFound
	}
	if err != nil {
		return nil, err
	}
	return task, nil
}

func reviewSessionNow() int64 {
	return time.Now().Unix()
}
