package study

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"ai-tutor/internal/db"
	"ai-tutor/internal/models"
	"ai-tutor/internal/utils"
)

type SyncPayload struct {
	UserToken string                 `json:"user_token"`
	Notebooks []models.Notebook      `json:"notebooks"`
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

func StartCloudSyncLoop(repo *db.Repository) {
	ticker := time.NewTicker(15 * time.Minute)
	go func() {
		utils.Warnf("[SYNC] Background cloud sync worker started.")
		// Run initial sync on launch
		if err := TriggerCloudSync(repo); err != nil {
			utils.Warnf("[SYNC] Initial launch sync warning: %v", err)
		}
		for range ticker.C {
			if err := TriggerCloudSync(repo); err != nil {
				utils.Warnf("[SYNC] Periodic sync warning: %v", err)
			}
		}
	}()
}

func TriggerCloudSync(repo *db.Repository) error {
	settings, err := repo.GetUserSettings()
	if err != nil {
		return err
	}
	if settings.CloudSyncURL == "" {
		return nil // Cloud sync not configured
	}

	utils.Warnf("[SYNC] Running cloud sync to: %s", settings.CloudSyncURL)

	// Gather notebooks and logs from DB (all notebooks for sync)
	notebooks, err := repo.GetNotebooks("", "")
	if err != nil {
		return fmt.Errorf("failed to fetch notebooks: %w", err)
	}

	// For simplicity, fetch recent review logs (e.g., last 100)
	logs, err := repo.GetRecentReviewLogs(100)
	if err != nil {
		utils.Warnf("[SYNC] failed to fetch recent review logs: %v", err)
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
	defer func() { _ = resp.Body.Close() }()

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
				if err := downloadAndRegisterNotebook(repo, nb); err != nil {
					utils.Warnf("[SYNC] Failed to download assigned notebook %s: %v", nb.Title, err)
				}
			}(assigned)
		}
	}

	utils.Warnf("[SYNC] Cloud sync completed successfully.")
	return nil
}

func downloadAndRegisterNotebook(repo *db.Repository, nb AssignedNotebook) error {
	// 1. Create a local path for the downloaded PDF
	baseDir := os.Getenv("APPDATA")
	if baseDir == "" {
		if dir, err := os.UserConfigDir(); err == nil {
			baseDir = dir
		}
	}
	if baseDir == "" {
		baseDir = os.TempDir()
	}
	dataDir := filepath.Join(baseDir, "ai-tutor", "notebooks")
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return err
	}
	localPath := filepath.Join(dataDir, nb.ID+".pdf")
	// 2. Download from remote URL
	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequest("GET", nb.DownloadURL, nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download server returned status %d", resp.StatusCode)
	}

	out, err := os.Create(localPath)
	if err != nil {
		return err
	}
	defer func() { _ = out.Close() }()

	const maxDownloadBytes = 100 << 20 // 100 MiB
	if resp.ContentLength > maxDownloadBytes {
		return fmt.Errorf("download rejected: Content-Length %d exceeds 100 MiB limit", resp.ContentLength)
	}
	limitedBody := &io.LimitedReader{R: resp.Body, N: maxDownloadBytes + 1}
	if _, err = io.Copy(out, limitedBody); err != nil {
		return err
	}
	if limitedBody.N <= 0 {
		return fmt.Errorf("download aborted: response exceeded 100 MiB limit")
	}

	// 3. Register in SQLite
	// Note: We register with status 'uploaded' and indexer will process it normally.
	err = repo.CreateNotebook(nb.ID, nb.Title, localPath, "pdf", "", 0)
	if err != nil {
		return fmt.Errorf("failed to insert notebook to database: %w", err)
	}

	utils.Warnf("[SYNC] Automatically registered newly assigned notebook: %s", nb.Title)
	return nil
}
