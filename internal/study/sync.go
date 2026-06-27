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

// ResolveCloudSyncURL returns the effective sync URL.
// Priority: stored SQLite value → CLOUD_SYNC_URL env var → empty (sync skipped).
func ResolveCloudSyncURL(storedURL string) string {
	if storedURL != "" {
		return storedURL
	}
	return os.Getenv("CLOUD_SYNC_URL")
}

// ResolveCloudAPIToken returns the effective API token.
// Priority: stored SQLite value → CLOUD_API_TOKEN env var → empty.
func ResolveCloudAPIToken(storedToken string) string {
	if storedToken != "" {
		return storedToken
	}
	return os.Getenv("CLOUD_API_TOKEN")
}

// NotebookSyncRecord is the minimal notebook identity the server needs.
// filepath.Base strips the local path — only the filename crosses the wire.
type NotebookSyncRecord struct {
	Filename    string `json:"filename"`
	Title       string `json:"title"`
	StudyStatus string `json:"study_status"`
}

type SyncPayload struct {
	UserToken     string                 `json:"user_token"`
	ClassroomCode string                 `json:"classroom_code"`
	Notebooks     []NotebookSyncRecord   `json:"notebooks"`
	Logs          []models.FSRSReviewLog `json:"logs"`
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

	syncURL := ResolveCloudSyncURL(settings.CloudSyncURL)
	apiToken := ResolveCloudAPIToken(settings.CloudAPIToken)

	if syncURL == "" {
		if syncErr := repo.ResolveFlashcardSyncTasks(); syncErr != nil {
			utils.Warnf("[SYNC] failed to resolve FLASHCARD_SYNC tasks: %v", syncErr)
		}
		return nil // Cloud sync not configured
	}

	utils.Warnf("[SYNC] Running cloud sync to: %s", syncURL)

	// Build slim notebook records — filename only, no local paths or internal IDs
	notebooks, err := repo.GetNotebooks("", "")
	if err != nil {
		return fmt.Errorf("failed to fetch notebooks: %w", err)
	}
	notebookRecords := make([]NotebookSyncRecord, 0, len(notebooks))
	for _, nb := range notebooks {
		notebookRecords = append(notebookRecords, NotebookSyncRecord{
			Filename:    filepath.Base(nb.FilePath),
			Title:       nb.Title,
			StudyStatus: nb.StudyStatus,
		})
	}

	// Delta: only logs newer than the last successful sync
	logs, err := repo.GetReviewLogsSince(settings.LastSyncedAt)
	if err != nil {
		utils.Warnf("[SYNC] failed to fetch delta review logs: %v", err)
		return err
	}
	utils.Warnf("[SYNC] delta logs to send: %d (since %d)", len(logs), settings.LastSyncedAt)

	payload := SyncPayload{
		UserToken:     apiToken,
		ClassroomCode: settings.ClassroomCode,
		Notebooks:     notebookRecords,
		Logs:          logs,
	}

	jsonBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal sync payload: %w", err)
	}

	var resp *http.Response
	var lastErr error
	const attempts = 3
	client := &http.Client{Timeout: 10 * time.Second}

	for i := 0; i < attempts; i++ {
		if i > 0 {
			utils.Warnf("[SYNC] Retrying cloud sync, attempt %d/%d due to: %v", i+1, attempts, lastErr)
			time.Sleep(1 * time.Second)
		}

		var req *http.Request
		req, lastErr = http.NewRequest("POST", syncURL, bytes.NewBuffer(jsonBytes))
		if lastErr != nil {
			lastErr = fmt.Errorf("failed to create http request: %w", lastErr)
			continue
		}
		req.Header.Set("Content-Type", "application/json")
		if apiToken != "" {
			req.Header.Set("Authorization", "Bearer "+apiToken)
		}

		resp, lastErr = client.Do(req)
		if lastErr != nil {
			lastErr = fmt.Errorf("network error during sync: %w", lastErr)
			continue
		}

		if resp.StatusCode != http.StatusOK {
			bodyBytes, _ := io.ReadAll(resp.Body)
			_ = resp.Body.Close()
			lastErr = fmt.Errorf("sync server returned status %d: %s", resp.StatusCode, string(bodyBytes))
			continue
		}

		// Decode response body inside loop to catch decode failures as errors
		var syncResp SyncResponse
		decodeErr := json.NewDecoder(resp.Body).Decode(&syncResp)
		_ = resp.Body.Close()
		if decodeErr != nil {
			lastErr = fmt.Errorf("failed to decode sync response: %w", decodeErr)
			continue
		}

		// Success!
		lastErr = nil

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

		// Advance the delta cursor so next sync only sends new events
		if setErr := repo.SetLastSyncedAt(time.Now().Unix()); setErr != nil {
			utils.Warnf("[SYNC] failed to persist last_synced_at: %v", setErr)
		}

		// Sync completed successfully. Clear any pending FLASHCARD_SYNC tasks.
		if syncErr := repo.ResolveFlashcardSyncTasks(); syncErr != nil {
			utils.Warnf("[SYNC] failed to resolve FLASHCARD_SYNC tasks: %v", syncErr)
		}

		break
	}

	if lastErr != nil {
		utils.Warnf("[SYNC] Cloud sync failed after %d attempts: %v", attempts, lastErr)
		// Insert FLASHCARD_SYNC task if not already pending/active and a valid notebook exists
		if len(notebooks) > 0 {
			notebookID := notebooks[0].ID
			if syncErr := repo.EnsurePendingFlashcardSyncTask(notebookID); syncErr != nil {
				utils.Warnf("[SYNC] failed to insert FLASHCARD_SYNC task: %v", syncErr)
			}
		}
		return lastErr
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
