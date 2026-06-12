package study

import (
	"ai-tutor/internal/db"
	"ai-tutor/internal/models"
	"ai-tutor/internal/utils"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

type SyncPayload struct {
	UserToken string               `json:"user_token"`
	Notebooks []models.Notebook    `json:"notebooks"`
	Logs      []models.FSRSReviewLog `json:"logs"`
}

type SyncResponse struct {
	NewNotebooks []AssignedNotebook `json:"new_notebooks"`
}

type AssignedNotebook struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	DownloadURL string `json:"download_url"`
}

func StartCloudSyncLoop() {
	ticker := time.NewTicker(15 * time.Minute)
	go func() {
		utils.Warnf("[SYNC] Background cloud sync worker started.")
		// Run initial sync on launch
		if err := TriggerCloudSync(); err != nil {
			utils.Warnf("[SYNC] Initial launch sync warning: %v", err)
		}
		for range ticker.C {
			if err := TriggerCloudSync(); err != nil {
				utils.Warnf("[SYNC] Periodic sync warning: %v", err)
			}
		}
	}()
}

func TriggerCloudSync() error {
	settings, err := db.GetUserSettings()
	if err != nil {
		return err
	}
	if settings.CloudSyncURL == "" {
		return nil // Cloud sync not configured
	}

	utils.Warnf("[SYNC] Running cloud sync to: %s", settings.CloudSyncURL)

	// Gather notebooks and logs from DB
	notebooks, err := db.GetNotebooks("")
	if err != nil {
		return fmt.Errorf("failed to fetch notebooks: %w", err)
	}

	// For simplicity, fetch recent review logs (e.g., last 100)
	var logs []models.FSRSReviewLog
	rows, err := db.GetConnection().Query(`
		SELECT id, topic_id, activity_type, reference_id, reviewed_at, rating, scheduled_days, state_before_json, state_after_json
		FROM fsrs_review_log
		ORDER BY reviewed_at DESC
		LIMIT 100
	`)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var log models.FSRSReviewLog
			if err := rows.Scan(&log.ID, &log.TopicID, &log.ActivityType, &log.ReferenceID, &log.ReviewedAt, &log.Rating, &log.ScheduledDays, &log.StateBeforeJSON, &log.StateAfterJSON); err == nil {
				logs = append(logs, log)
			}
		}
	}

	payload := SyncPayload{
		UserToken: settings.CloudAPIToken,
		Notebooks: notebooks,
		Logs:      logs,
	}

	jsonBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal sync payload: %w", err)
	}

	req, err := http.NewRequest("POST", settings.CloudSyncURL, bytes.NewBuffer(jsonBytes))
	if err != nil {
		return fmt.Errorf("failed to create http request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if settings.CloudAPIToken != "" {
		req.Header.Set("Authorization", "Bearer "+settings.CloudAPIToken)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("network error during sync: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("sync server returned status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var syncResp SyncResponse
	if err := json.NewDecoder(resp.Body).Decode(&syncResp); err != nil {
		return fmt.Errorf("failed to decode sync response: %w", err)
	}

	// Handle assigned notebooks from teacher
	if len(syncResp.NewNotebooks) > 0 {
		utils.Warnf("[SYNC] Found %d new teacher assignments", len(syncResp.NewNotebooks))
		for _, assigned := range syncResp.NewNotebooks {
			go func(nb AssignedNotebook) {
				if err := downloadAndRegisterNotebook(nb); err != nil {
					utils.Warnf("[SYNC] Failed to download assigned notebook %s: %v", nb.Title, err)
				}
			}(assigned)
		}
	}

	utils.Warnf("[SYNC] Cloud sync completed successfully.")
	return nil
}

func downloadAndRegisterNotebook(nb AssignedNotebook) error {
	// 1. Create a local path for the downloaded PDF
	dataDir := filepath.Join(os.Getenv("APPDATA"), "ai-tutor", "notebooks")
	_ = os.MkdirAll(dataDir, 0755)
	localPath := filepath.Join(dataDir, nb.ID+".pdf")

	// 2. Download from remote URL
	resp, err := http.Get(nb.DownloadURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download server returned status %d", resp.StatusCode)
	}

	out, err := os.Create(localPath)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return err
	}

	// 3. Register in SQLite
	// Note: We register with status 'uploaded' and indexer will process it normally.
	err = db.CreateNotebook(nb.ID, nb.Title, localPath, "pdf", "", 0)
	if err != nil {
		return fmt.Errorf("failed to insert notebook to database: %w", err)
	}

	utils.Warnf("[SYNC] Automatically registered newly assigned notebook: %s", nb.Title)
	return nil
}
