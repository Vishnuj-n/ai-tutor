package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"ai-tutor/internal/models"
)

func createFlashcardsRepo(cards []models.Flashcard, states map[string]models.FlashcardState) error {
	return withTx(func(tx *sql.Tx) error {
		for _, card := range cards {
			stateJSON, marshalErr := json.Marshal(states[card.ID])
			if marshalErr != nil {
				return fmt.Errorf("failed to encode flashcard state for %s: %w", card.ID, marshalErr)
			}

			_, execErr := tx.Exec(`
				INSERT OR IGNORE INTO fsrs_cards (id, topic_id, source_chunk_id, prompt, answer, state_json, due_at, suspended)
				VALUES (?, ?, NULLIF(?, ''), ?, ?, ?, ?, ?)
			`, card.ID, card.TopicID, card.SourceChunkID, card.Prompt, card.Answer, string(stateJSON), card.DueAt, boolToInt(card.Suspended))
			if execErr != nil {
				return execErr
			}
		}
		return nil
	})
}

func getFlashcardByIDRepo(cardID string) (*models.Flashcard, *models.FlashcardState, error) {
	return getFlashcardByIDQuerier(conn, cardID)
}

func getFlashcardByIDRepoTx(tx *sql.Tx, cardID string) (*models.Flashcard, *models.FlashcardState, error) {
	return getFlashcardByIDQuerier(tx, cardID)
}

func getFlashcardByIDQuerier(q querier, cardID string) (*models.Flashcard, *models.FlashcardState, error) {
	var card models.Flashcard
	var stateJSON sql.NullString
	var suspended bool

	err := q.QueryRow(`
		SELECT id, topic_id, COALESCE(source_chunk_id, ''), prompt, answer, COALESCE(due_at, 0), suspended, state_json
		FROM fsrs_cards
		WHERE id = ?
	`, cardID).Scan(&card.ID, &card.TopicID, &card.SourceChunkID, &card.Prompt, &card.Answer, &card.DueAt, &suspended, &stateJSON)
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

// getFlashcardStatesByIDsRepo returns a map of flashcard states keyed by card ID for the given card IDs
func getFlashcardStatesByIDsRepo(cardIDs []string) (map[string]models.FlashcardState, error) {
	if len(cardIDs) == 0 {
		return make(map[string]models.FlashcardState), nil
	}

	// Create placeholders for the IN clause
	placeholders := make([]string, len(cardIDs))
	args := make([]interface{}, len(cardIDs))
	for i, id := range cardIDs {
		placeholders[i] = "?"
		args[i] = id
	}

	query := fmt.Sprintf(`
		SELECT id, state_json
		FROM fsrs_cards
		WHERE id IN (%s)
	`, strings.Join(placeholders, ","))

	rows, err := conn.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = rows.Close()
	}()

	states := make(map[string]models.FlashcardState)
	for rows.Next() {
		var cardID string
		var stateJSON sql.NullString

		if err := rows.Scan(&cardID, &stateJSON); err != nil {
			return nil, err
		}

		state := models.FlashcardState{}
		if stateJSON.Valid && stateJSON.String != "" {
			if unmarshalErr := json.Unmarshal([]byte(stateJSON.String), &state); unmarshalErr != nil {
				return nil, fmt.Errorf("failed to decode flashcard state for %s: %w", cardID, unmarshalErr)
			}
		}

		states[cardID] = state
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return states, nil
}

func updateFlashcardReviewRepo(cardID string, dueAt int64, expectedDueAt int64, expectedStateJSON string, state models.FlashcardState, reviewLog models.FSRSReviewLog) error {
	stateJSON, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("failed to encode flashcard state for %s: %w", cardID, err)
	}

	return withTx(func(tx *sql.Tx) error {
		result, err := tx.Exec(`
			UPDATE fsrs_cards
			SET state_json = ?, due_at = ?, updated_at = CURRENT_TIMESTAMP
			WHERE id = ? AND due_at = ? AND state_json = ?
		`, string(stateJSON), dueAt, cardID, expectedDueAt, expectedStateJSON)
		if err != nil {
			return err
		}

		rows, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if rows != 1 {
			return fmt.Errorf("flashcard %s was modified concurrently", cardID)
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
		return nil
	})
}

func updateFlashcardReviewRepoTx(tx *sql.Tx, cardID string, dueAt int64, expectedDueAt int64, expectedStateJSON string, state models.FlashcardState, reviewLog models.FSRSReviewLog) error {
	stateJSON, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("failed to encode flashcard state for %s: %w", cardID, err)
	}

	result, err := tx.Exec(`
		UPDATE fsrs_cards
		SET state_json = ?, due_at = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ? AND due_at = ? AND state_json = ?
	`, string(stateJSON), dueAt, cardID, expectedDueAt, expectedStateJSON)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows != 1 {
		return fmt.Errorf("flashcard %s was modified concurrently", cardID)
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
	return nil
}

func countFlashcardsForTopicRepo(topicID string) (int, error) {
	var count int
	err := conn.QueryRow(`SELECT COUNT(*) FROM fsrs_cards WHERE topic_id = ? AND suspended = 0`, topicID).Scan(&count)
	return count, err
}

func getOrCreateFlashcardsForTopicRepo(topicID string, cardsIfNotExist []models.Flashcard, statesIfNotExist map[string]models.FlashcardState) ([]models.Flashcard, bool, error) {
	var cards []models.Flashcard
	var existing bool
	err := withTx(func(tx *sql.Tx) error {
		var count int
		err := tx.QueryRow(`SELECT COUNT(*) FROM fsrs_cards WHERE topic_id = ? AND suspended = 0`, topicID).Scan(&count)
		if err != nil {
			return err
		}

		if count > 0 {
			rows, err := tx.Query(`
				SELECT id, topic_id, COALESCE(source_chunk_id, ''), prompt, answer, COALESCE(due_at, 0), suspended
				FROM fsrs_cards
				WHERE topic_id = ? AND suspended = 0
				ORDER BY
					CASE WHEN due_at IS NULL THEN 1 ELSE 0 END,
					due_at ASC,
					created_at ASC,
					id ASC
			`, topicID)
			if err != nil {
				return err
			}
			defer func() {
				_ = rows.Close()
			}()

			cards = make([]models.Flashcard, 0)
			for rows.Next() {
				var card models.Flashcard
				var suspended bool
				if err := rows.Scan(&card.ID, &card.TopicID, &card.SourceChunkID, &card.Prompt, &card.Answer, &card.DueAt, &suspended); err != nil {
					return err
				}
				card.Suspended = suspended
				cards = append(cards, card)
			}
			if err := rows.Err(); err != nil {
				return err
			}
			existing = true
			return nil
		}

		insertedCards := make([]models.Flashcard, 0, len(cardsIfNotExist))
		for _, card := range cardsIfNotExist {
			stateJSON, marshalErr := json.Marshal(statesIfNotExist[card.ID])
			if marshalErr != nil {
				return fmt.Errorf("failed to encode flashcard state for %s: %w", card.ID, marshalErr)
			}

			result, execErr := tx.Exec(`
				INSERT OR IGNORE INTO fsrs_cards (id, topic_id, source_chunk_id, prompt, answer, state_json, due_at, suspended)
				VALUES (?, ?, NULLIF(?, ''), ?, ?, ?, ?, ?)
			`, card.ID, card.TopicID, card.SourceChunkID, card.Prompt, card.Answer, string(stateJSON), card.DueAt, boolToInt(card.Suspended))
			if execErr != nil {
				return execErr
			}

			rowsAffected, rowsErr := result.RowsAffected()
			if rowsErr != nil {
				return rowsErr
			}
			if rowsAffected == 1 {
				insertedCards = append(insertedCards, card)
			}
		}

		cards = insertedCards
		existing = false
		return nil
	})

	if err != nil {
		return nil, false, err
	}
	return cards, existing, nil
}

func getLastFlashcardReviewTimeRepo(cardID string) (int64, error) {
	var lastReviewedAt int64
	err := conn.QueryRow(`
		SELECT COALESCE(MAX(reviewed_at), 0)
		FROM fsrs_review_log
		WHERE activity_type = 'flashcard' AND reference_id = ?
	`, cardID).Scan(&lastReviewedAt)
	return lastReviewedAt, err
}

func getLastFlashcardReviewTimeRepoTx(tx *sql.Tx, cardID string) (int64, error) {
	var lastReviewedAt int64
	err := tx.QueryRow(`
		SELECT COALESCE(MAX(reviewed_at), 0)
		FROM fsrs_review_log
		WHERE activity_type = 'flashcard' AND reference_id = ?
	`, cardID).Scan(&lastReviewedAt)
	return lastReviewedAt, err
}
