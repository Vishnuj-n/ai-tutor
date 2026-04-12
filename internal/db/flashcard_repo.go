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

	_, err = conn.Exec(`
		UPDATE fsrs_cards
		SET state_json = ?, due_at = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, string(stateJSON), dueAt, cardID)
	return err
}

func countFlashcardsForTopicRepo(topicID string) (int, error) {
	var count int
	err := conn.QueryRow(`SELECT COUNT(*) FROM fsrs_cards WHERE topic_id = ?`, topicID).Scan(&count)
	return count, err
}

func nullableString(value string) interface{} {
	if value == "" {
		return nil
	}
	return value
}
