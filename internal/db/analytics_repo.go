package db

import (
	"ai-tutor/internal/models"
	"fmt"
	"time"
)

// TrackAnalyticsEvent inserts a new anonymous telemetry event into SQLite if enabled.
func (r *Repository) TrackAnalyticsEvent(eventType, fileHash string, pageNumber int, metadata string) error {
	// First check if user settings allow tracking
	settings, err := r.GetUserSettings()
	if err != nil {
		return err
	}
	if !settings.AnalyticsEnabled {
		return nil // telemetry is opted out, do nothing
	}

	// Validate event type
	switch eventType {
	case "reading_complete", "quiz_complete":
		// Allowed
	default:
		return fmt.Errorf("unsupported analytics event type: %s", eventType)
	}

	// Validate metadata size to prevent database bloating (limit to 2KB)
	if len(metadata) > 2048 {
		return fmt.Errorf("metadata size (%d bytes) exceeds maximum limit of 2048 bytes", len(metadata))
	}

	_, err = r.db.Exec(`
		INSERT INTO analytics_events (event_type, file_hash, page_number, metadata, synced)
		VALUES (?, ?, ?, ?, 0)
	`, eventType, fileHash, pageNumber, metadata)
	return err
}

// GetUnsyncedAnalyticsEvents returns all unsynced events mapped to the JSON API format and their DB row IDs.
func (r *Repository) GetUnsyncedAnalyticsEvents() ([]models.AnalyticsEventSync, []int64, error) {
	rows, err := r.db.Query(`
		SELECT id, event_type, file_hash, page_number, metadata, created_at
		FROM analytics_events
		WHERE synced = 0
		ORDER BY created_at ASC
	`)
	if err != nil {
		return nil, nil, err
	}
	defer func() { _ = rows.Close() }()

	var payloads []models.AnalyticsEventSync
	var ids []int64

	for rows.Next() {
		var id int64
		var eventType, fileHash, metadata string
		var pageNumber int
		var createdAt time.Time

		if err := rows.Scan(&id, &eventType, &fileHash, &pageNumber, &metadata, &createdAt); err != nil {
			return nil, nil, err
		}

		payloads = append(payloads, models.AnalyticsEventSync{
			EventType:  eventType,
			FileHash:   fileHash,
			PageNumber: pageNumber,
			Metadata:   metadata,
			CreatedAt:  createdAt.Format(time.RFC3339),
		})
		ids = append(ids, id)
	}

	if err := rows.Err(); err != nil {
		return nil, nil, err
	}

	return payloads, ids, nil
}

// MarkAnalyticsSynced flags the events with the given IDs as synced.
func (r *Repository) MarkAnalyticsSynced(ids []int64) error {
	if len(ids) == 0 {
		return nil
	}

	query := `UPDATE analytics_events SET synced = 1 WHERE id IN (`
	args := make([]interface{}, len(ids))
	for i, id := range ids {
		if i > 0 {
			query += ", "
		}
		query += "?"
		args[i] = id
	}
	query += ")"

	_, err := r.db.Exec(query, args...)
	return err
}
