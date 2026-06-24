package db

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"ai-tutor/internal/models"
)

// CreateFlashcards stores a new set of flashcards for one topic.
// Used by: app_contract_test.go (test-only coverage, production code path utilizes GetOrCreateFlashcardsForTopic)
func (r *Repository) CreateFlashcards(topicID string, cards []models.Flashcard, states map[string]models.FlashcardState) error {
	topicID = strings.TrimSpace(topicID)
	if topicID == "" {
		return fmt.Errorf("topic id is required")
	}
	if len(cards) == 0 {
		return fmt.Errorf("at least one flashcard is required")
	}
	if len(states) == 0 {
		return fmt.Errorf("flashcard states are required")
	}

	normalizedCards, err := normalizeValidateFlashcards(topicID, cards, states)
	if err != nil {
		return err
	}

	return r.withTx(func(tx *sql.Tx) error {
		for _, card := range normalizedCards {
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

// GetFlashcardByID returns one flashcard and its scheduler state.
func (r *Repository) GetFlashcardByID(cardID string) (*models.Flashcard, *models.FlashcardState, error) {
	cardID = strings.TrimSpace(cardID)
	if cardID == "" {
		return nil, nil, fmt.Errorf("flashcard id is required")
	}
	return r.getFlashcardByIDQuerier(r.db, cardID)
}

func (r *Repository) GetFlashcardByIDTx(tx *sql.Tx, cardID string) (*models.Flashcard, *models.FlashcardState, error) {
	cardID = strings.TrimSpace(cardID)
	if cardID == "" {
		return nil, nil, fmt.Errorf("flashcard id is required")
	}
	return r.getFlashcardByIDQuerier(tx, cardID)
}

func (r *Repository) getFlashcardByIDQuerier(q querier, cardID string) (*models.Flashcard, *models.FlashcardState, error) {
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

// GetFlashcardStatesByIDs returns a map of flashcard states keyed by card ID for the given card IDs
func (r *Repository) GetFlashcardStatesByIDs(cardIDs []string) (map[string]models.FlashcardState, error) {
	if len(cardIDs) == 0 {
		return make(map[string]models.FlashcardState), nil
	}

	// Trim and validate card IDs
	trimmedIDs := make([]string, 0, len(cardIDs))
	for _, id := range cardIDs {
		trimmedID := strings.TrimSpace(id)
		if trimmedID != "" {
			trimmedIDs = append(trimmedIDs, trimmedID)
		}
	}

	if len(trimmedIDs) == 0 {
		return make(map[string]models.FlashcardState), nil
	}

	// Create placeholders for the IN clause
	placeholders := make([]string, len(trimmedIDs))
	args := make([]interface{}, len(trimmedIDs))
	for i, id := range trimmedIDs {
		placeholders[i] = "?"
		args[i] = id
	}

	query := fmt.Sprintf(`
		SELECT id, state_json
		FROM fsrs_cards
		WHERE id IN (%s)
	`, strings.Join(placeholders, ","))

	rows, err := r.db.Query(query, args...)
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

// UpdateFlashcardReview updates scheduling state after a review grade.
func (r *Repository) UpdateFlashcardReview(cardID string, dueAt int64, expectedDueAt int64, expectedStateJSON string, state models.FlashcardState, reviewLog models.FSRSReviewLog) error {
	cardID = strings.TrimSpace(cardID)
	if cardID == "" {
		return fmt.Errorf("flashcard id is required")
	}
	if dueAt <= 0 {
		return fmt.Errorf("due time is required")
	}
	stateJSON, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("failed to encode flashcard state for %s: %w", cardID, err)
	}

	return r.withTx(func(tx *sql.Tx) error {
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

func (r *Repository) UpdateFlashcardReviewTx(tx *sql.Tx, cardID string, dueAt int64, expectedDueAt int64, expectedStateJSON string, state models.FlashcardState, reviewLog models.FSRSReviewLog) error {
	cardID = strings.TrimSpace(cardID)
	if cardID == "" {
		return fmt.Errorf("flashcard id is required")
	}
	if dueAt <= 0 {
		return fmt.Errorf("due time is required")
	}
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

// CountFlashcardsForTopic returns how many flashcards exist for a topic.
func (r *Repository) CountFlashcardsForTopic(topicID string) (int, error) {
	topicID = strings.TrimSpace(topicID)
	if topicID == "" {
		return 0, fmt.Errorf("topic id is required")
	}
	var count int
	err := r.db.QueryRow(`SELECT COUNT(*) FROM fsrs_cards WHERE topic_id = ? AND suspended = 0`, topicID).Scan(&count)
	return count, err
}

// GetOrCreateFlashcardsForTopic atomically fetches existing non-suspended flashcards or creates new ones.
func (r *Repository) GetOrCreateFlashcardsForTopic(topicID string, cardsIfNotExist []models.Flashcard, statesIfNotExist map[string]models.FlashcardState) ([]models.Flashcard, bool, error) {
	topicID = strings.TrimSpace(topicID)
	if topicID == "" {
		return nil, false, fmt.Errorf("topic id is required")
	}

	if len(cardsIfNotExist) == 0 {
		return nil, false, fmt.Errorf("at least one flashcard is required to create")
	}
	if len(statesIfNotExist) == 0 {
		return nil, false, fmt.Errorf("flashcard states are required to create")
	}

	normalizedCards, err := normalizeValidateFlashcards(topicID, cardsIfNotExist, statesIfNotExist)
	if err != nil {
		return nil, false, err
	}

	var cards []models.Flashcard
	var existing bool
	err = r.withTx(func(tx *sql.Tx) error {
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

		for _, card := range normalizedCards {
			stateJSON, marshalErr := json.Marshal(statesIfNotExist[card.ID])
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

		existing = false
		return nil
	})

	if err != nil {
		return nil, false, err
	}
	return cards, existing, nil
}

// GetLastFlashcardReviewTime retrieves the last review time for a flashcard.
func (r *Repository) GetLastFlashcardReviewTime(cardID string) (int64, error) {
	cardID = strings.TrimSpace(cardID)
	if cardID == "" {
		return 0, fmt.Errorf("flashcard id is required")
	}
	var lastReviewedAt int64
	err := r.db.QueryRow(`
		SELECT COALESCE(MAX(reviewed_at), 0)
		FROM fsrs_review_log
		WHERE activity_type = 'flashcard' AND reference_id = ?
	`, cardID).Scan(&lastReviewedAt)
	return lastReviewedAt, err
}

// GetLastFlashcardReviewTimeTx retrieves the last review time for a flashcard within a transaction.
func (r *Repository) GetLastFlashcardReviewTimeTx(tx *sql.Tx, cardID string) (int64, error) {
	cardID = strings.TrimSpace(cardID)
	if cardID == "" {
		return 0, fmt.Errorf("flashcard id is required")
	}
	var lastReviewedAt int64
	err := tx.QueryRow(`
		SELECT COALESCE(MAX(reviewed_at), 0)
		FROM fsrs_review_log
		WHERE activity_type = 'flashcard' AND reference_id = ?
	`, cardID).Scan(&lastReviewedAt)
	return lastReviewedAt, err
}

// SaveManualFlashcardsBatch handles the storage of sandbox flashcards.
func (r *Repository) SaveManualFlashcardsBatch(notebookID string, cards []models.Flashcard) error {
	return r.withTx(func(tx *sql.Tx) error {
		// Clean out the old manual sandbox cards for this specific notebook
		_, err := tx.Exec(`DELETE FROM manual_flashcards WHERE notebook_id = ?`, notebookID)
		if err != nil {
			return err
		}

		// Insert the fresh sandbox generation batch
		for _, card := range cards {
			_, err = tx.Exec(`
				INSERT INTO manual_flashcards (id, notebook_id, prompt, answer)
				VALUES (?, ?, ?, ?)
			`, card.ID, notebookID, card.Prompt, card.Answer)
			if err != nil {
				return err
			}
		}
		return nil
	})
}

func normalizeValidateFlashcards(topicID string, cards []models.Flashcard, states map[string]models.FlashcardState) ([]models.Flashcard, error) {
	normalizedCards := make([]models.Flashcard, 0, len(cards))
	seenIDs := make(map[string]bool)
	seenTopicPrompts := make(map[string]bool)

	for _, card := range cards {
		card.ID = strings.TrimSpace(card.ID)
		card.TopicID = strings.TrimSpace(card.TopicID)
		if card.TopicID == "" {
			card.TopicID = topicID
		} else if card.TopicID != topicID {
			return nil, fmt.Errorf("flashcard topic id must match topic id")
		}
		card.Prompt = strings.TrimSpace(card.Prompt)
		card.Answer = strings.TrimSpace(card.Answer)
		if card.ID == "" {
			return nil, fmt.Errorf("flashcard id is required")
		}
		if card.Prompt == "" || card.Answer == "" {
			return nil, fmt.Errorf("flashcard prompt and answer are required")
		}
		if _, ok := states[card.ID]; !ok {
			return nil, fmt.Errorf("flashcard state is required for %s", card.ID)
		}

		// Check for duplicate IDs
		if seenIDs[card.ID] {
			return nil, fmt.Errorf("duplicate flashcard id found: %s", card.ID)
		}
		seenIDs[card.ID] = true

		// Check for duplicate (topic_id, prompt) pairs
		topicPromptKey := card.TopicID + "|" + card.Prompt
		if seenTopicPrompts[topicPromptKey] {
			return nil, fmt.Errorf("duplicate (topic_id, prompt) pair found: topic_id=%s, prompt=%s", card.TopicID, card.Prompt)
		}
		seenTopicPrompts[topicPromptKey] = true

		normalizedCards = append(normalizedCards, card)
	}

	return normalizedCards, nil
}

// FlashcardExistsByID returns true if a flashcard with the given ID exists.
func (r *Repository) FlashcardExistsByID(cardID string) (bool, error) {
	var exists int
	err := r.db.QueryRow(`SELECT COUNT(*) FROM fsrs_cards WHERE id = ?`, cardID).Scan(&exists)
	return exists > 0, err
}

// SuspendFlashcard sets the suspended flag on a flashcard, removing it from all future review sessions.
func (r *Repository) SuspendFlashcard(cardID string) error {
	cardID = strings.TrimSpace(cardID)
	if cardID == "" {
		return fmt.Errorf("flashcard id is required")
	}
	result, err := r.db.Exec(`
		UPDATE fsrs_cards
		SET suspended = 1, updated_at = CURRENT_TIMESTAMP
		WHERE id = ? AND suspended = 0
	`, cardID)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return fmt.Errorf("flashcard %s not found or already suspended", cardID)
	}
	return nil
}

// SuspendFlashcardTx sets the suspended flag on a flashcard within a transaction.
func (r *Repository) SuspendFlashcardTx(tx *sql.Tx, cardID string) error {
	cardID = strings.TrimSpace(cardID)
	if cardID == "" {
		return fmt.Errorf("flashcard id is required")
	}
	var suspended int
	err := tx.QueryRow(`SELECT suspended FROM fsrs_cards WHERE id = ?`, cardID).Scan(&suspended)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("flashcard %s not found", cardID)
		}
		return err
	}
	if suspended == 1 {
		return nil // already suspended, success
	}
	result, err := tx.Exec(`
		UPDATE fsrs_cards
		SET suspended = 1, updated_at = CURRENT_TIMESTAMP
		WHERE id = ? AND suspended = 0
	`, cardID)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("checking affected rows: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("flashcard %s already suspended or not found", cardID)
	}
	return nil
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
