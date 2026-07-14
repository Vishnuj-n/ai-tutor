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
	FileHash             string `json:"file_hash"`
	Filename             string `json:"filename"`
	Title                string `json:"title"`
	StudyStatus          string `json:"study_status"`
	ExternalHelpRequired bool   `json:"external_help_required"` // Red Alert indicator
}

type SyncPayload struct {
	UserToken     string                      `json:"user_token"`
	ClassroomCode string                      `json:"classroom_code"`
	Notebooks     []NotebookSyncRecord        `json:"notebooks"`
	Logs          []models.SyncLogEntry       `json:"logs"`
	Analytics     []models.AnalyticsEventSync `json:"analytics,omitempty"`
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
		if syncErr := repo.ResolveFlashcardGenerateTasksForTopic(""); syncErr != nil {
			utils.Warnf("[SYNC] failed to resolve FLASHCARD_GENERATE tasks: %v", syncErr)
		}
		if settings.AnalyticsEnabled {
			if fbErr := syncAnalyticsFallback(repo); fbErr != nil {
				utils.Warnf("[SYNC] fallback analytics upload failed: %v", fbErr)
			}
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
		if nb.FileHash == "" {
			utils.Warnf("[SYNC] skipping notebook with empty FileHash: title=%q, path=%q", nb.Title, nb.FilePath)
			continue
		}
		notebookRecords = append(notebookRecords, NotebookSyncRecord{
			FileHash:             nb.FileHash,
			Filename:             filepath.Base(nb.FilePath),
			Title:                nb.Title,
			StudyStatus:          nb.StudyStatus,
			ExternalHelpRequired: nb.ExternalHelpRequired,
		})
	}

	// Delta: only logs newer than the last successful sync
	logs, err := repo.GetReviewLogsSinceWithFileInfo(settings.LastSyncedAt)
	if err != nil {
		utils.Warnf("[SYNC] failed to fetch delta review logs: %v", err)
		return err
	}
	utils.Warnf("[SYNC] delta logs to send: %d (since %d)", len(logs), settings.LastSyncedAt)

	// Fetch unsynced local analytics events if consent is enabled
	var unsyncedEvents []models.AnalyticsEventSync
	var eventIDs []int64
	if settings.AnalyticsEnabled {
		var aErr error
		unsyncedEvents, eventIDs, aErr = repo.GetUnsyncedAnalyticsEvents()
		if aErr != nil {
			utils.Warnf("[SYNC] failed to fetch unsynced analytics: %v", aErr)
		}
	}

	payload := SyncPayload{
		UserToken:     apiToken,
		ClassroomCode: settings.ClassroomCode,
		Notebooks:     notebookRecords,
		Logs:          logs,
		Analytics:     unsyncedEvents,
	}

	jsonBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal sync payload: %w", err)
	}

	headers := make(map[string]string)
	anonKey := os.Getenv("CLOUD_API_TOKEN")
	if anonKey == "" {
		anonKey = os.Getenv("SUPABASE_ANON_KEY")
	}
	if anonKey != "" {
		headers["apikey"] = anonKey
		headers["Authorization"] = "Bearer " + anonKey
	}

	var syncResp SyncResponse
	lastErr := postJSONWithRetry(syncURL, jsonBytes, headers, 3, &syncResp)
	if lastErr == nil {
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
		maxReviewedAt := settings.LastSyncedAt
		for _, entry := range logs {
			if entry.ReviewedAt > maxReviewedAt {
				maxReviewedAt = entry.ReviewedAt
			}
		}
		if setErr := repo.SetLastSyncedAt(maxReviewedAt); setErr != nil {
			utils.Warnf("[SYNC] failed to persist last_synced_at: %v", setErr)
		}

		// Mark sent analytics events as synced in local SQLite
		if len(eventIDs) > 0 {
			if markErr := repo.MarkAnalyticsSynced(eventIDs); markErr != nil {
				utils.Warnf("[SYNC] failed to mark analytics synced: %v", markErr)
			}
		}

		// Sync completed successfully. Clear any pending FLASHCARD_GENERATE tasks.
		if syncErr := repo.ResolveFlashcardGenerateTasksForTopic(""); syncErr != nil {
			utils.Warnf("[SYNC] failed to resolve FLASHCARD_GENERATE tasks: %v", syncErr)
		}
	}

	if lastErr != nil {
		utils.Warnf("[SYNC] Cloud sync failed after %d attempts: %v", 3, lastErr)
		// Insert FLASHCARD_GENERATE task if not already pending/active and a valid notebook exists
		if len(notebooks) > 0 {
			notebookID := notebooks[0].ID
			if syncErr := repo.EnsurePendingFlashcardGenerateTask(notebookID, "", 0, 0, "Cloud Sync Recovery"); syncErr != nil {
				utils.Warnf("[SYNC] failed to insert FLASHCARD_GENERATE task: %v", syncErr)
			}
		}
		return lastErr
	}

	utils.Warnf("[SYNC] Cloud sync completed successfully.")
	return nil
}

func syncAnalyticsFallback(repo *db.Repository) error {
	events, ids, err := repo.GetUnsyncedAnalyticsEvents()
	if err != nil {
		return fmt.Errorf("failed to fetch unsynced analytics for fallback: %w", err)
	}
	if len(events) == 0 {
		return nil
	}

	researchURL := os.Getenv("RESEARCH_ANALYTICS_URL")
	if researchURL == "" {
		researchURL = "https://rptpauakhdsqinpcnebw.supabase.co/rest/v1/anonymous_analytics_events"
	}
	researchToken := os.Getenv("RESEARCH_ANALYTICS_ANON_KEY")
	if researchToken == "" {
		researchToken = "sb_publishable_aL0Wgco3ZzH_OS64pP4g-w_tWRN_bNf"
	}

	jsonBytes, err := json.Marshal(events)
	if err != nil {
		return fmt.Errorf("failed to marshal fallback analytics: %w", err)
	}

	headers := map[string]string{
		"apikey":        researchToken,
		"Authorization": "Bearer " + researchToken,
	}

	if err := postJSONWithRetry(researchURL, jsonBytes, headers, 3, nil); err != nil {
		return err
	}

	if err := repo.MarkAnalyticsSynced(ids); err != nil {
		utils.Warnf("[SYNC-ANALYTICS] failed to mark fallback events synced: %v", err)
	}
	utils.Warnf("[SYNC-ANALYTICS] Fallback analytics sync of %d events succeeded.", len(events))
	return nil
}

func postJSONWithRetry(url string, jsonBytes []byte, headers map[string]string, attempts int, decodeTarget interface{}) error {
	client := &http.Client{Timeout: 10 * time.Second}
	var lastErr error

	for i := 0; i < attempts; i++ {
		if i > 0 {
			utils.Warnf("[SYNC-RETRY] Attempt %d/%d due to: %v", i+1, attempts, lastErr)
			time.Sleep(1 * time.Second)
		}

		req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonBytes))
		if err != nil {
			lastErr = fmt.Errorf("failed to create http request: %w", err)
			continue
		}
		req.Header.Set("Content-Type", "application/json")
		for k, v := range headers {
			req.Header.Set(k, v)
		}

		resp, err := client.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("network error: %w", err)
			continue
		}

		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
			bodyBytes, _ := io.ReadAll(resp.Body)
			_ = resp.Body.Close()
			lastErr = fmt.Errorf("server returned status %d: %s", resp.StatusCode, string(bodyBytes))
			continue
		}

		if decodeTarget != nil {
			decodeErr := json.NewDecoder(resp.Body).Decode(decodeTarget)
			_ = resp.Body.Close()
			if decodeErr != nil {
				lastErr = fmt.Errorf("failed to decode response: %w", decodeErr)
				continue
			}
		} else {
			_ = resp.Body.Close()
		}

		return nil
	}
	return lastErr
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
		_ = out.Close()
		_ = os.Remove(localPath)
		return fmt.Errorf("download rejected: Content-Length %d exceeds 100 MiB limit", resp.ContentLength)
	}
	limitedBody := &io.LimitedReader{R: resp.Body, N: maxDownloadBytes + 1}
	if _, err = io.Copy(out, limitedBody); err != nil {
		_ = out.Close()
		_ = os.Remove(localPath)
		return err
	}
	_ = out.Close()
	if limitedBody.N <= 0 {
		_ = os.Remove(localPath)
		return fmt.Errorf("download aborted: response exceeded 100 MiB limit")
	}

	// 3. Register in SQLite
	// Note: We register with status 'uploaded' and indexer will process it normally.
	fileHash, hashErr := utils.FileSHA256(localPath)
	if hashErr != nil {
		_ = os.Remove(localPath)
		return fmt.Errorf("failed to compute file hash: %w", hashErr)
	}
	err = repo.CreateNotebook(nb.ID, nb.Title, localPath, "pdf", "", fileHash, 0)
	if err != nil {
		_ = os.Remove(localPath)
		return fmt.Errorf("failed to insert notebook to database: %w", err)
	}

	utils.Warnf("[SYNC] Automatically registered newly assigned notebook: %s", nb.Title)
	return nil
}
