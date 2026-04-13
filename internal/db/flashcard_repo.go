package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"time"

	"ai-tutor/internal/models"
)

func createFlashcardsRepo(cards []models.Flashcard, states map[string]models.FlashcardState) error {
	tx, err := conn.Begin()
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	for _, card := range cards {
		stateJSON, marshalErr := json.Marshal(states[card.ID])
		if marshalErr != nil {
			err = fmt.Errorf("failed to encode flashcard state for %s: %w", card.ID, marshalErr)
			return err
		}

		if _, err = tx.Exec(`
			INSERT INTO fsrs_cards (id, topic_id, prompt, answer, state_json, due_at, suspended)
			VALUES (?, ?, ?, ?, ?, ?, ?)
		`, card.ID, card.TopicID, card.Prompt, card.Answer, string(stateJSON), nullableString(card.DueAt), boolToInt(card.Suspended)); err != nil {
			return err
		}
	}

	err = tx.Commit()
	return err
}

func getFlashcardsForTopicRepo(topicID string, dueOnly bool, now string) ([]models.Flashcard, error) {
	query := `
		SELECT id, topic_id, prompt, answer, COALESCE(due_at, ''), suspended
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
			CASE WHEN due_at IS NULL OR due_at = '' THEN 1 ELSE 0 END,
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
		var suspended int
		if err := rows.Scan(&card.ID, &card.TopicID, &card.Prompt, &card.Answer, &card.DueAt, &suspended); err != nil {
			return nil, err
		}
		card.Suspended = suspended == 1
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
	var suspended int

	err := conn.QueryRow(`
		SELECT id, topic_id, prompt, answer, COALESCE(due_at, ''), suspended, state_json
		FROM fsrs_cards
		WHERE id = ?
	`, cardID).Scan(&card.ID, &card.TopicID, &card.Prompt, &card.Answer, &card.DueAt, &suspended, &stateJSON)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil, nil
		}
		return nil, nil, err
	}
	card.Suspended = suspended == 1

	state := models.FlashcardState{}
	if stateJSON.Valid && stateJSON.String != "" {
		if unmarshalErr := json.Unmarshal([]byte(stateJSON.String), &state); unmarshalErr != nil {
			return nil, nil, fmt.Errorf("failed to decode flashcard state for %s: %w", card.ID, unmarshalErr)
		}
	}

	return &card, &state, nil
}

func updateFlashcardReviewRepo(cardID string, dueAt string, state models.FlashcardState) error {
	stateJSON, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("failed to encode flashcard state for %s: %w", cardID, err)
	}

	result, err := conn.Exec(`
		UPDATE fsrs_cards
		SET state_json = ?, due_at = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, string(stateJSON), dueAt, cardID)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows != 1 {
		return fmt.Errorf("flashcard %s not found", cardID)
	}
	return nil
}

func applyFlashcardReviewRepo(cardID string, rating string, reviewedAt time.Time) (*models.Flashcard, *models.FlashcardState, int, error) {
	tx, err := conn.Begin()
	if err != nil {
		return nil, nil, 0, err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	var card models.Flashcard
	var stateJSON sql.NullString
	var suspended int
	err = tx.QueryRow(`
		SELECT id, topic_id, prompt, answer, COALESCE(due_at, ''), suspended, state_json
		FROM fsrs_cards
		WHERE id = ?
	`, cardID).Scan(&card.ID, &card.TopicID, &card.Prompt, &card.Answer, &card.DueAt, &suspended, &stateJSON)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil, 0, nil
		}
		return nil, nil, 0, err
	}
	card.Suspended = suspended == 1

	state := models.FlashcardState{}
	if stateJSON.Valid && stateJSON.String != "" {
		if unmarshalErr := json.Unmarshal([]byte(stateJSON.String), &state); unmarshalErr != nil {
			return nil, nil, 0, fmt.Errorf("failed to decode flashcard state for %s: %w", card.ID, unmarshalErr)
		}
	}

	nextDelay, intervalHours, stage := nextFlashcardScheduleRepo(state, rating)
	dueAt := reviewedAt.Add(nextDelay).Format(time.RFC3339)

	state.Stage = stage
	state.LastIntervalHours = intervalHours
	state.LastRating = rating
	state.LastReviewedAt = reviewedAt.Format(time.RFC3339)
	if rating == "again" {
		state.LapseCount++
	} else {
		state.SuccessCount++
	}

	updatedStateJSON, marshalErr := json.Marshal(state)
	if marshalErr != nil {
		return nil, nil, 0, fmt.Errorf("failed to encode flashcard state for %s: %w", cardID, marshalErr)
	}

	result, err := tx.Exec(`
		UPDATE fsrs_cards
		SET state_json = ?, due_at = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, string(updatedStateJSON), dueAt, cardID)
	if err != nil {
		return nil, nil, 0, err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return nil, nil, 0, err
	}
	if rows != 1 {
		return nil, nil, 0, fmt.Errorf("flashcard %s not found", cardID)
	}

	if err = tx.Commit(); err != nil {
		return nil, nil, 0, err
	}

	card.DueAt = dueAt
	return &card, &state, intervalHours, nil
}

func nextFlashcardScheduleRepo(state models.FlashcardState, rating string) (time.Duration, int, string) {
	baseInterval := state.LastIntervalHours
	if baseInterval <= 0 {
		switch rating {
		case "again":
			return 10 * time.Minute, 0, "learning"
		case "hard":
			return 8 * time.Hour, 8, "learning"
		case "good":
			return 24 * time.Hour, 24, "review"
		case "easy":
			return 72 * time.Hour, 72, "review"
		default:
			return 24 * time.Hour, 24, "review"
		}
	}

	switch rating {
	case "again":
		return 10 * time.Minute, 0, "learning"
	case "hard":
		nextHours := maxIntRepo(24, int(math.Ceil(float64(baseInterval)*1.5)))
		return time.Duration(nextHours) * time.Hour, nextHours, "review"
	case "good":
		nextHours := maxIntRepo(48, baseInterval*2)
		return time.Duration(nextHours) * time.Hour, nextHours, "review"
	case "easy":
		nextHours := maxIntRepo(96, baseInterval*4)
		return time.Duration(nextHours) * time.Hour, nextHours, "review"
	default:
		nextHours := maxIntRepo(48, baseInterval*2)
		return time.Duration(nextHours) * time.Hour, nextHours, "review"
	}
}

func maxIntRepo(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func countFlashcardsForTopicRepo(topicID string) (int, error) {
	var count int
	err := conn.QueryRow(`SELECT COUNT(*) FROM fsrs_cards WHERE topic_id = ? AND suspended = 0`, topicID).Scan(&count)
	return count, err
}

// getOrCreateFlashcardsForTopicRepo atomically checks if non-suspended flashcards exist.
// If they do, it returns them. If not, it inserts the provided cards and returns them.
// This prevents race conditions where two concurrent requests both see zero cards.
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

	// Check if non-suspended flashcards already exist (within transaction)
	var count int
	err = tx.QueryRow(`SELECT COUNT(*) FROM fsrs_cards WHERE topic_id = ? AND suspended = 0`, topicID).Scan(&count)
	if err != nil {
		return nil, false, err
	}

	if count > 0 {
		// Cards exist; retrieve and return them
		query := `
			SELECT id, topic_id, prompt, answer, COALESCE(due_at, ''), suspended
			FROM fsrs_cards
			WHERE topic_id = ? AND suspended = 0
			ORDER BY
				CASE WHEN due_at IS NULL OR due_at = '' THEN 1 ELSE 0 END,
				due_at ASC,
				created_at ASC,
				id ASC
		`
		rows, err := tx.Query(query, topicID)
		if err != nil {
			return nil, false, err
		}
		defer func() {
			_ = rows.Close()
		}()

		cards := make([]models.Flashcard, 0)
		for rows.Next() {
			var card models.Flashcard
			var suspended int
			if err := rows.Scan(&card.ID, &card.TopicID, &card.Prompt, &card.Answer, &card.DueAt, &suspended); err != nil {
				return nil, false, err
			}
			card.Suspended = suspended == 1
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

	// No cards exist; insert the provided ones
	for _, card := range cardsIfNotExist {
		stateJSON, marshalErr := json.Marshal(statesIfNotExist[card.ID])
		if marshalErr != nil {
			err = fmt.Errorf("failed to encode flashcard state for %s: %w", card.ID, marshalErr)
			return nil, false, err
		}

		if _, err = tx.Exec(`
			INSERT INTO fsrs_cards (id, topic_id, prompt, answer, state_json, due_at, suspended)
			VALUES (?, ?, ?, ?, ?, ?, ?)
		`, card.ID, card.TopicID, card.Prompt, card.Answer, string(stateJSON), nullableString(card.DueAt), boolToInt(card.Suspended)); err != nil {
			return nil, false, err
		}
	}

	if err = tx.Commit(); err != nil {
		return nil, false, err
	}

	return cardsIfNotExist, false, nil
}

func nullableString(value string) interface{} {
	if value == "" {
		return nil
	}
	return value
}
