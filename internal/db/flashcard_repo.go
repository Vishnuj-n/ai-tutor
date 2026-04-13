package db

import (
	"database/sql"
	"encoding/json"
	"fmt"

	"ai-tutor/internal/models"
)

func createFlashcardsRepo(cards []models.Flashcard, states map[string]models.FlashcardState) error {
	tx, err := conn.Begin()
	if err != nil {
		return err
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	for _, card := range cards {
		stateJSON, marshalErr := json.Marshal(states[card.ID])
		if marshalErr != nil {
			err = fmt.Errorf("failed to encode flashcard state for %s: %w", card.ID, marshalErr)
			return err
		}

		result, execErr := tx.Exec(`
			INSERT OR IGNORE INTO fsrs_cards (id, topic_id, prompt, answer, state_json, due_at, suspended)
			VALUES (?, ?, ?, ?, ?, ?, ?)
		`, card.ID, card.TopicID, card.Prompt, card.Answer, string(stateJSON), card.DueAt, boolToInt(card.Suspended))
		if execErr != nil {
			return execErr
		}

		rowsAffected, rowsErr := result.RowsAffected()
		if rowsErr != nil {
			return rowsErr
		}
		if rowsAffected != 1 {
			return fmt.Errorf("flashcard %s was not inserted", card.ID)
		}
	}

	if err = tx.Commit(); err != nil {
		return err
	}
	committed = true
	return nil
}

func getFlashcardsForTopicRepo(topicID string, dueOnly bool, now int64) ([]models.Flashcard, error) {
	query := `
		SELECT id, topic_id, prompt, answer, COALESCE(due_at, 0), suspended
		FROM fsrs_cards
		WHERE topic_id = ?
		  AND suspended = 0
	`
	args := []interface{}{topicID}
	if dueOnly {
		query += `
		  AND due_at IS NOT NULL
		  AND due_at <= ?
		`
		args = append(args, now)
	}
	query += `
		ORDER BY
			CASE WHEN due_at IS NULL THEN 1 ELSE 0 END,
			due_at ASC,
			created_at ASC,
			id ASC
	`

	rows, err := conn.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = rows.Close()
	}()

	cards := make([]models.Flashcard, 0)
	for rows.Next() {
		var card models.Flashcard
		var suspended bool
		if err := rows.Scan(&card.ID, &card.TopicID, &card.Prompt, &card.Answer, &card.DueAt, &suspended); err != nil {
			return nil, err
		}
		card.Suspended = suspended
		cards = append(cards, card)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return cards, nil
}

func getFlashcardByIDRepo(cardID string) (*models.Flashcard, *models.FlashcardState, error) {
	var card models.Flashcard
	var stateJSON sql.NullString
	var suspended bool

	err := conn.QueryRow(`
		SELECT id, topic_id, prompt, answer, COALESCE(due_at, 0), suspended, state_json
		FROM fsrs_cards
		WHERE id = ?
	`, cardID).Scan(&card.ID, &card.TopicID, &card.Prompt, &card.Answer, &card.DueAt, &suspended, &stateJSON)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil, nil
		}
		return nil, nil, err
	}
	card.Suspended = suspended

	state := models.FlashcardState{}
	if stateJSON.Valid && stateJSON.String != "" {
		if unmarshalErr := json.Unmarshal([]byte(stateJSON.String), &state); unmarshalErr != nil {
			return nil, nil, fmt.Errorf("failed to decode flashcard state for %s: %w", card.ID, unmarshalErr)
		}
	}

	return &card, &state, nil
}

func updateFlashcardReviewRepo(cardID string, dueAt int64, expectedDueAt int64, state models.FlashcardState, reviewLog models.FSRSReviewLog) error {
	stateJSON, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("failed to encode flashcard state for %s: %w", cardID, err)
	}

	tx, err := conn.Begin()
	if err != nil {
		return err
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	result, err := tx.Exec(`
		UPDATE fsrs_cards
		SET state_json = ?, due_at = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ? AND due_at = ? AND state_json = ?
	`, string(stateJSON), dueAt, cardID, expectedDueAt, reviewLog.StateBeforeJSON)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows != 1 {
		err = fmt.Errorf("flashcard %s was modified concurrently", cardID)
		return err
	}

	var validatedTopicID string
	if err = tx.QueryRow(`
		SELECT t.id
		FROM fsrs_cards c
		JOIN topics t ON t.id = c.topic_id
		WHERE c.id = ?
	`, cardID).Scan(&validatedTopicID); err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("topic not found for flashcard %s", cardID)
		}
		return err
	}

	if _, err = tx.Exec(`
		INSERT INTO fsrs_review_log (
			id, topic_id, activity_type, reference_id, reviewed_at, rating,
			scheduled_days, state_before_json, state_after_json
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, reviewLog.ID, validatedTopicID, reviewLog.ActivityType, cardID,
		reviewLog.ReviewedAt, reviewLog.Rating, reviewLog.ScheduledDays,
		reviewLog.StateBeforeJSON, string(stateJSON)); err != nil {
		return err
	}

	if err = tx.Commit(); err != nil {
		return err
	}
	committed = true
	return nil
}

func countFlashcardsForTopicRepo(topicID string) (int, error) {
	var count int
	err := conn.QueryRow(`SELECT COUNT(*) FROM fsrs_cards WHERE topic_id = ? AND suspended = 0`, topicID).Scan(&count)
	return count, err
}

func getOrCreateFlashcardsForTopicRepo(topicID string, cardsIfNotExist []models.Flashcard, statesIfNotExist map[string]models.FlashcardState) ([]models.Flashcard, bool, error) {
	tx, err := conn.Begin()
	if err != nil {
		return nil, false, err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	var count int
	err = tx.QueryRow(`SELECT COUNT(*) FROM fsrs_cards WHERE topic_id = ? AND suspended = 0`, topicID).Scan(&count)
	if err != nil {
		return nil, false, err
	}

	if count > 0 {
		rows, err := tx.Query(`
			SELECT id, topic_id, prompt, answer, COALESCE(due_at, 0), suspended
			FROM fsrs_cards
			WHERE topic_id = ? AND suspended = 0
			ORDER BY
				CASE WHEN due_at IS NULL THEN 1 ELSE 0 END,
				due_at ASC,
				created_at ASC,
				id ASC
		`, topicID)
		if err != nil {
			return nil, false, err
		}
		defer func() {
			_ = rows.Close()
		}()

		cards := make([]models.Flashcard, 0)
		for rows.Next() {
			var card models.Flashcard
			var suspended bool
			if err := rows.Scan(&card.ID, &card.TopicID, &card.Prompt, &card.Answer, &card.DueAt, &suspended); err != nil {
				return nil, false, err
			}
			card.Suspended = suspended
			cards = append(cards, card)
		}
		if err := rows.Err(); err != nil {
			return nil, false, err
		}
		if err = tx.Commit(); err != nil {
			return nil, false, err
		}
		return cards, true, nil
	}

	for _, card := range cardsIfNotExist {
		stateJSON, marshalErr := json.Marshal(statesIfNotExist[card.ID])
		if marshalErr != nil {
			err = fmt.Errorf("failed to encode flashcard state for %s: %w", card.ID, marshalErr)
			return nil, false, err
		}

		result, execErr := tx.Exec(`
			INSERT OR IGNORE INTO fsrs_cards (id, topic_id, prompt, answer, state_json, due_at, suspended)
			VALUES (?, ?, ?, ?, ?, ?, ?)
		`, card.ID, card.TopicID, card.Prompt, card.Answer, string(stateJSON), card.DueAt, boolToInt(card.Suspended))
		if execErr != nil {
			return nil, false, execErr
		}

		rowsAffected, rowsErr := result.RowsAffected()
		if rowsErr != nil {
			return nil, false, rowsErr
		}
		if rowsAffected != 1 {
			return nil, false, fmt.Errorf("flashcard %s was not inserted", card.ID)
		}
	}

	if err = tx.Commit(); err != nil {
		return nil, false, err
	}

	return cardsIfNotExist, false, nil
}
