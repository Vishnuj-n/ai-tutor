package db

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"ai-tutor/internal/models"
	"ai-tutor/internal/utils"
)

// querier interface allows both *sql.DB and *sql.Tx to be used with database helper functions
type querier interface {
	QueryRow(query string, args ...any) *sql.Row
}

var conn *sql.DB
var embeddingDimension int32 = 0 // Will be set during DB initialization with vec0

const maxRetrievalK = 100 // Maximum k allowed for vector search retrieval

const (
	llmTierFast  = "fast"
	llmTierHeavy = "heavy"
)

// Close releases the active SQLite connection.
func Close() error {
	if conn == nil {
		return nil
	}
	err := conn.Close()
	conn = nil
	return err
}

// GetConnection returns the underlying database connection for transaction management.
func GetConnection() *sql.DB {
	return conn
}

// Init initializes the SQLite database and creates tables
// vec0DllPath should be the absolute path to vec0.dll (sqlite-vec extension)
func Init(dbPath, vec0DllPath string) error {
	if conn != nil {
		if err := conn.Close(); err != nil {
			return fmt.Errorf("failed to close previous database connection: %w", err)
		}
		conn = nil
	}

	var err error
	conn, err = sql.Open("sqlite3", "file:"+dbPath+"?_foreign_keys=on&_busy_timeout=5000")
	if err != nil {
		return err
	}
	conn.SetMaxOpenConns(1)
	conn.SetMaxIdleConns(1)

	if err := conn.Ping(); err != nil {
		if closeErr := conn.Close(); closeErr != nil {
			log.Printf("Warning: failed to close database connection after ping error: %v", closeErr)
		}
		conn = nil
		return err
	}

	// Load sqlite-vec extension if available
	if vec0DllPath != "" {
		// Verify file exists before attempting to load
		if _, err := os.Stat(vec0DllPath); err == nil {
			// Use absolute path for the extension
			absPath, err := filepath.Abs(vec0DllPath)
			if err != nil {
				absPath = vec0DllPath
			}
			// Use driver-level extension loading (SQL load_extension may be blocked as "not authorized").
			if err := loadExtension(conn, absPath); err != nil {
				log.Printf("Warning: could not load sqlite-vec extension from %s: %v", absPath, err)
				// Non-fatal; continue without vec0 for backward compat
			} else {
				utils.Infof("Successfully loaded sqlite-vec extension from %s", absPath)
			}
		} else {
			log.Printf("Warning: vec0.dll not found at %s", vec0DllPath)
		}
	}

	// Nuclear strategy: Initialize schema with a single transaction
	tx, err := conn.Begin()
	if err != nil {
		if closeErr := conn.Close(); closeErr != nil {
			log.Printf("Warning: failed to close database connection after begin error: %v", closeErr)
		}
		conn = nil
		return fmt.Errorf("failed to begin schema transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	if err := InitSchema(tx); err != nil {
		if closeErr := conn.Close(); closeErr != nil {
			log.Printf("Warning: failed to close database connection after schema error: %v", closeErr)
		}
		conn = nil
		return fmt.Errorf("failed to initialize schema: %w", err)
	}

	if err := tx.Commit(); err != nil {
		if closeErr := conn.Close(); closeErr != nil {
			log.Printf("Warning: failed to close database connection after commit error: %v", closeErr)
		}
		conn = nil
		return fmt.Errorf("failed to commit schema transaction: %w", err)
	}

	return nil
}

// IsVecExtensionLoaded checks if the sqlite-vec (vec0) extension is loaded and functional.
func IsVecExtensionLoaded() bool {
	if conn == nil {
		return false
	}
	var version string
	err := conn.QueryRow("SELECT vec_version()").Scan(&version)
	return err == nil
}

// InitWithVectorDimension initializes the database and creates the vec0 virtual table.
// Called after ONNX embedder dimension is discovered.
func InitWithVectorDimension(embeddingDim int32) error {
	if embeddingDim <= 0 {
		return fmt.Errorf("invalid embedding dimension: %d", embeddingDim)
	}
	embeddingDimension = embeddingDim

	// Create vec0 virtual table with the discovered dimension
	return createVectorTable()
}

// QueryDueReviewCards counts cards due by the given time, scoped to existing topics.
// Excludes cards already linked to pending/active review tasks to avoid double-counting.
func QueryDueReviewCards(now int64) (int, error) {
	var activeProfileID sql.NullString
	if err := conn.QueryRow(`
		SELECT COALESCE(active_profile_id, '') FROM user_settings WHERE id = 1
	`).Scan(&activeProfileID); err != nil && err != sql.ErrNoRows {
		return 0, fmt.Errorf("QueryDueReviewCards: reading active_profile_id: %w", err)
	}

	activeProfileStr := ""
	if activeProfileID.Valid {
		activeProfileStr = activeProfileID.String
	}

	var count int
	query := `
		SELECT COUNT(DISTINCT fc.id)
		FROM fsrs_cards fc
		JOIN topics t ON t.id = fc.topic_id
		LEFT JOIN notebook_topics nt ON nt.topic_id = t.id
		LEFT JOIN notebooks n ON n.id = nt.notebook_id
		WHERE fc.suspended = 0
		  AND fc.due_at IS NOT NULL
		  AND fc.due_at <= ?
		  AND NOT EXISTS (
			SELECT 1
			FROM review_task_cards rtc
			JOIN study_queue sq ON sq.id = rtc.task_id
			WHERE rtc.card_id = fc.id
			  AND sq.task_type = 'FLASHCARD_REVIEW'
			  AND sq.status IN ('PENDING', 'ACTIVE')
		  )
	`
	var args []interface{}
	args = append(args, now)
	if activeProfileStr != "" {
		query += ` AND (n.profile_id = ? OR n.profile_id IS NULL OR n.profile_id = '') `
		args = append(args, activeProfileStr)
	}

	err := conn.QueryRow(query, args...).Scan(&count)
	return count, err
}

// GetDailyStudyMinutes returns the persisted global daily study budget.
func GetDailyStudyMinutes() (int, error) {
	var minutes int
	err := conn.QueryRow(`
		SELECT daily_study_minutes
		FROM user_settings
		WHERE id = 1
	`).Scan(&minutes)
	if err == sql.ErrNoRows {
		return 90, nil
	}
	return minutes, err
}

// UpsertDailyStudyMinutes stores the global daily study budget.
func UpsertDailyStudyMinutes(minutes int) error {
	if minutes <= 0 {
		return fmt.Errorf("daily study minutes must be positive")
	}

	_, err := conn.Exec(`
		INSERT INTO user_settings (id, daily_study_minutes)
		VALUES (1, ?)
		ON CONFLICT(id) DO UPDATE SET
			daily_study_minutes = excluded.daily_study_minutes,
			updated_at = CURRENT_TIMESTAMP
	`, minutes)
	return err
}

// GetRAGEnabled returns the status of RAG flag.
func GetRAGEnabled() (bool, error) {
	var enabled bool
	err := conn.QueryRow(`
		SELECT COALESCE(rag_enabled, 0)
		FROM user_settings
		WHERE id = 1
	`).Scan(&enabled)
	if err == sql.ErrNoRows {
		return false, nil
	}
	return enabled, err
}

// GetDefaultProfileID retrieves the oldest profile ID.
func GetDefaultProfileID() (string, error) {
	var id string
	err := conn.QueryRow(`
		SELECT id FROM study_profiles ORDER BY created_at ASC LIMIT 1
	`).Scan(&id)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return id, err
}

// GetUserSettings returns the full settings config.
func GetUserSettings() (*models.UserSettings, error) {
	var s models.UserSettings
	var activeProfileID sql.NullString
	err := conn.QueryRow(`
		SELECT daily_study_minutes, COALESCE(active_profile_id, ''), skip_to_reading_active, COALESCE(cloud_sync_url, ''), COALESCE(cloud_api_token, ''), COALESCE(theme, 'light-classic'), COALESCE(rag_enabled, 0)
		FROM user_settings
		WHERE id = 1
	`).Scan(&s.DailyStudyMinutes, &activeProfileID, &s.SkipToReadingActive, &s.CloudSyncURL, &s.CloudAPIToken, &s.Theme, &s.RAGEnabled)
	if err == sql.ErrNoRows {
		s = models.UserSettings{
			DailyStudyMinutes: 90,
			Theme:             "light-classic",
			RAGEnabled:        false,
		}
	} else if err != nil {
		return nil, err
	} else {
		if activeProfileID.Valid {
			s.ActiveProfileID = activeProfileID.String
		}
	}

	// Dynamic fallback for active profile ID
	if s.ActiveProfileID == "" {
		defaultID, err := GetDefaultProfileID()
		if err != nil {
			return nil, fmt.Errorf("failed to resolve default profile: %w", err)
		}
		if defaultID != "" {
			if _, err := conn.Exec(`UPDATE user_settings SET active_profile_id = ? WHERE id = 1`, defaultID); err != nil {
				return nil, fmt.Errorf("failed to persist active profile ID: %w", err)
			}
			s.ActiveProfileID = defaultID
		}
	} else {
		// Verify if the active profile still exists
		var exists bool
		if err := conn.QueryRow(`SELECT EXISTS(SELECT 1 FROM study_profiles WHERE id = ?)`, s.ActiveProfileID).Scan(&exists); err != nil {
			return nil, fmt.Errorf("failed to check active profile existence: %w", err)
		}
		if !exists {
			defaultID, err := GetDefaultProfileID()
			if err != nil {
				return nil, fmt.Errorf("failed to resolve fallback default profile: %w", err)
			}
			if defaultID != "" {
				if _, err := conn.Exec(`UPDATE user_settings SET active_profile_id = ? WHERE id = 1`, defaultID); err != nil {
					return nil, fmt.Errorf("failed to persist fallback active profile ID: %w", err)
				}
				s.ActiveProfileID = defaultID
			} else {
				if _, err := conn.Exec(`UPDATE user_settings SET active_profile_id = NULL WHERE id = 1`); err != nil {
					return nil, fmt.Errorf("failed to clear inactive active profile ID: %w", err)
				}
				s.ActiveProfileID = ""
			}
		}
	}

	return &s, nil
}

// UpdateUserSettings updates the user settings.
func UpdateUserSettings(s models.UserSettings) error {
	var activeProfileID interface{} = nil
	if s.ActiveProfileID != "" {
		activeProfileID = s.ActiveProfileID
	}
	theme := s.Theme
	if theme == "" {
		theme = "light-classic"
	}
	_, err := conn.Exec(`
		INSERT INTO user_settings (id, daily_study_minutes, active_profile_id, skip_to_reading_active, cloud_sync_url, cloud_api_token, theme, rag_enabled)
		VALUES (1, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			daily_study_minutes = excluded.daily_study_minutes,
			active_profile_id = excluded.active_profile_id,
			skip_to_reading_active = excluded.skip_to_reading_active,
			cloud_sync_url = excluded.cloud_sync_url,
			cloud_api_token = excluded.cloud_api_token,
			theme = excluded.theme,
			rag_enabled = excluded.rag_enabled,
			updated_at = CURRENT_TIMESTAMP
	`, s.DailyStudyMinutes, activeProfileID, s.SkipToReadingActive, s.CloudSyncURL, s.CloudAPIToken, theme, s.RAGEnabled)
	return err
}

func GetLLMSettings() (*models.LLMSettings, error) {
	rows, err := conn.Query(`
		SELECT tier, provider, base_url, model, timeout_ms, api_key_source, COALESCE(has_api_key, 0)
		FROM llm_settings
		WHERE tier IN ('fast', 'heavy')
	`)
	if err != nil {
		return nil, err
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			log.Printf("warning: failed to close LLM settings rows: %v", closeErr)
		}
	}()

	settings := defaultLLMSettings()
	seenFast := false
	seenHeavy := false
	for rows.Next() {
		var tier models.LLMTierSettings
		if err := rows.Scan(&tier.Tier, &tier.Provider, &tier.BaseURL, &tier.Model, &tier.TimeoutMs, &tier.APIKeySource, &tier.HasAPIKey); err != nil {
			return nil, err
		}
		tier = normalizeLLMTierSettings(tier)
		switch tier.Tier {
		case llmTierFast:
			settings.Fast = tier
			seenFast = true
		case llmTierHeavy:
			settings.Heavy = tier
			seenHeavy = true
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if !seenFast {
		settings.Fast = defaultLLMTier(llmTierFast)
	}
	if !seenHeavy {
		settings.Heavy = defaultLLMTier(llmTierHeavy)
	}
	settings.UseSameForHeavy = sameLLMConfig(settings.Fast, settings.Heavy)
	return &settings, nil
}

func UpdateLLMSettings(settings models.LLMSettings) error {
	fast := normalizeLLMTierSettings(settings.Fast)
	fast.Tier = llmTierFast
	heavy := normalizeLLMTierSettings(settings.Heavy)
	heavy.Tier = llmTierHeavy
	if settings.UseSameForHeavy {
		heavy.Provider = fast.Provider
		heavy.BaseURL = fast.BaseURL
		heavy.Model = fast.Model
		heavy.TimeoutMs = fast.TimeoutMs
		heavy.APIKeySource = fast.APIKeySource
		heavy.HasAPIKey = fast.HasAPIKey
	}

	tx, err := conn.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	for _, tier := range []models.LLMTierSettings{fast, heavy} {
		if _, err := tx.Exec(`
			INSERT INTO llm_settings (tier, provider, base_url, model, timeout_ms, api_key_source, has_api_key)
			VALUES (?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT(tier) DO UPDATE SET
				provider = excluded.provider,
				base_url = excluded.base_url,
				model = excluded.model,
				timeout_ms = excluded.timeout_ms,
				api_key_source = excluded.api_key_source,
				has_api_key = excluded.has_api_key,
				updated_at = CURRENT_TIMESTAMP
		`, tier.Tier, tier.Provider, tier.BaseURL, tier.Model, tier.TimeoutMs, tier.APIKeySource, tier.HasAPIKey); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func MarkLLMKeyStored(tier string, stored bool) error {
	tier = normalizeLLMTier(tier)
	if tier == "" {
		return fmt.Errorf("llm tier is required")
	}
	_, err := conn.Exec(`
		UPDATE llm_settings
		SET has_api_key = ?, api_key_source = 'keyring', updated_at = CURRENT_TIMESTAMP
		WHERE tier = ?
	`, stored, tier)
	return err
}

func defaultLLMSettings() models.LLMSettings {
	return models.LLMSettings{
		UseSameForHeavy: true,
		Fast:            defaultLLMTier(llmTierFast),
		Heavy:           defaultLLMTier(llmTierHeavy),
	}
}

func defaultLLMTier(tier string) models.LLMTierSettings {
	timeout := 60000
	if tier == llmTierHeavy {
		timeout = 90000
	}
	return models.LLMTierSettings{
		Tier:         tier,
		Provider:     "groq",
		BaseURL:      "https://api.groq.com/openai",
		Model:        "openai/gpt-oss-120b",
		TimeoutMs:    timeout,
		APIKeySource: "keyring",
		HasAPIKey:    false,
	}
}

func normalizeLLMTierSettings(tier models.LLMTierSettings) models.LLMTierSettings {
	tier.Tier = normalizeLLMTier(tier.Tier)
	if tier.Tier == "" {
		tier.Tier = llmTierFast
	}
	tier.Provider = strings.TrimSpace(strings.ToLower(tier.Provider))
	if tier.Provider == "" {
		tier.Provider = "custom"
	}
	tier.BaseURL = strings.TrimSpace(tier.BaseURL)
	if tier.BaseURL == "" {
		tier.BaseURL = defaultBaseURLForProvider(tier.Provider)
	}
	tier.Model = strings.TrimSpace(tier.Model)
	if tier.Model == "" {
		tier.Model = defaultModelForProvider(tier.Provider)
	}
	if tier.TimeoutMs <= 0 {
		tier.TimeoutMs = 30000
	}
	tier.APIKeySource = strings.TrimSpace(strings.ToLower(tier.APIKeySource))
	if tier.APIKeySource == "" {
		tier.APIKeySource = "keyring"
	}
	return tier
}

func normalizeLLMTier(tier string) string {
	tier = strings.TrimSpace(strings.ToLower(tier))
	switch tier {
	case llmTierFast, llmTierHeavy:
		return tier
	default:
		return ""
	}
}

func defaultBaseURLForProvider(provider string) string {
	switch provider {
	case "groq":
		return "https://api.groq.com/openai"
	case "openai":
		return "https://api.openai.com"
	case "openrouter":
		return "https://openrouter.ai/api"
	default:
		return ""
	}
}

func defaultModelForProvider(provider string) string {
	switch provider {
	case "groq":
		return "openai/gpt-oss-120b"
	case "openai":
		return "gpt-4.1-mini"
	case "openrouter":
		return "openai/gpt-4.1-mini"
	default:
		return ""
	}
}

func sameLLMConfig(a, b models.LLMTierSettings) bool {
	return strings.EqualFold(a.Provider, b.Provider) &&
		strings.TrimSpace(a.BaseURL) == strings.TrimSpace(b.BaseURL) &&
		strings.TrimSpace(a.Model) == strings.TrimSpace(b.Model) &&
		a.TimeoutMs == b.TimeoutMs &&
		strings.EqualFold(a.APIKeySource, b.APIKeySource) &&
		a.HasAPIKey == b.HasAPIKey
}

// GetProfiles retrieves all study profiles.
func GetProfiles() ([]models.StudyProfile, error) {
	rows, err := conn.Query(`
		SELECT id, name, deadline_at, created_at
		FROM study_profiles
		ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			log.Printf("warning: failed to close profiles rows: %v", closeErr)
		}
	}()

	profiles := make([]models.StudyProfile, 0)
	for rows.Next() {
		var p models.StudyProfile
		if err := rows.Scan(&p.ID, &p.Name, &p.DeadlineAt, &p.CreatedAt); err != nil {
			return nil, err
		}
		profiles = append(profiles, p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return profiles, nil
}

// GetProfileByID retrieves a specific profile by ID.
func GetProfileByID(id string) (*models.StudyProfile, error) {
	var p models.StudyProfile
	err := conn.QueryRow(`
		SELECT id, name, deadline_at, created_at
		FROM study_profiles
		WHERE id = ?
	`, id).Scan(&p.ID, &p.Name, &p.DeadlineAt, &p.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// CreateProfile creates a new study profile.
func CreateProfile(p models.StudyProfile) error {
	_, err := conn.Exec(`
		INSERT INTO study_profiles (id, name, deadline_at)
		VALUES (?, ?, ?)
	`, p.ID, p.Name, p.DeadlineAt)
	return err
}

// UpdateProfile updates an existing profile.
func UpdateProfile(p models.StudyProfile) error {
	_, err := conn.Exec(`
		UPDATE study_profiles
		SET name = ?, deadline_at = ?
		WHERE id = ?
	`, p.Name, p.DeadlineAt, p.ID)
	return err
}

// DeleteProfile deletes a profile atomically.
func DeleteProfile(id string) error {
	tx, err := conn.Begin()
	if err != nil {
		return fmt.Errorf("DeleteProfile: failed to begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.Exec(`UPDATE notebooks SET profile_id = NULL WHERE profile_id = ?`, id); err != nil {
		return fmt.Errorf("DeleteProfile: failed to unlink notebooks: %w", err)
	}
	if _, err := tx.Exec(`UPDATE user_settings SET active_profile_id = NULL WHERE active_profile_id = ?`, id); err != nil {
		return fmt.Errorf("DeleteProfile: failed to unlink user_settings: %w", err)
	}
	if _, err := tx.Exec(`DELETE FROM study_profiles WHERE id = ?`, id); err != nil {
		return fmt.Errorf("DeleteProfile: failed to delete profile: %w", err)
	}
	return tx.Commit()
}

// CreateFlashcards stores a new set of flashcards for one topic.
// Used by: app_contract_test.go (test-only coverage, production code path utilizes GetOrCreateFlashcardsForTopic)
func CreateFlashcards(topicID string, cards []models.Flashcard, states map[string]models.FlashcardState) error {
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

	return createFlashcardsRepo(normalizedCards, states)
}

// CountFlashcardsForTopic returns how many flashcards exist for a topic.
func CountFlashcardsForTopic(topicID string) (int, error) {
	topicID = strings.TrimSpace(topicID)
	if topicID == "" {
		return 0, fmt.Errorf("topic id is required")
	}
	return countFlashcardsForTopicRepo(topicID)
}

// GetFlashcardByID returns one flashcard and its scheduler state.
func GetFlashcardByID(cardID string) (*models.Flashcard, *models.FlashcardState, error) {
	cardID = strings.TrimSpace(cardID)
	if cardID == "" {
		return nil, nil, fmt.Errorf("flashcard id is required")
	}
	return getFlashcardByIDRepo(cardID)
}

func GetFlashcardByIDTx(tx *sql.Tx, cardID string) (*models.Flashcard, *models.FlashcardState, error) {
	cardID = strings.TrimSpace(cardID)
	if cardID == "" {
		return nil, nil, fmt.Errorf("flashcard id is required")
	}
	return getFlashcardByIDRepoTx(tx, cardID)
}

// GetLastFlashcardReviewTime retrieves the last review time for a flashcard.
func GetLastFlashcardReviewTime(cardID string) (int64, error) {
	cardID = strings.TrimSpace(cardID)
	if cardID == "" {
		return 0, fmt.Errorf("flashcard id is required")
	}
	return getLastFlashcardReviewTimeRepo(cardID)
}

// GetLastFlashcardReviewTimeTx retrieves the last review time for a flashcard within a transaction.
func GetLastFlashcardReviewTimeTx(tx *sql.Tx, cardID string) (int64, error) {
	cardID = strings.TrimSpace(cardID)
	if cardID == "" {
		return 0, fmt.Errorf("flashcard id is required")
	}
	return getLastFlashcardReviewTimeRepoTx(tx, cardID)
}

// GetFlashcardStatesByIDs returns a map of flashcard states keyed by card ID for the given card IDs
func GetFlashcardStatesByIDs(cardIDs []string) (map[string]models.FlashcardState, error) {
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

	return getFlashcardStatesByIDsRepo(trimmedIDs)
}

// UpdateFlashcardReview updates scheduling state after a review grade.
func UpdateFlashcardReview(cardID string, dueAt int64, expectedDueAt int64, expectedStateJSON string, state models.FlashcardState, reviewLog models.FSRSReviewLog) error {
	cardID = strings.TrimSpace(cardID)
	if cardID == "" {
		return fmt.Errorf("flashcard id is required")
	}
	if dueAt <= 0 {
		return fmt.Errorf("due time is required")
	}
	return updateFlashcardReviewRepo(cardID, dueAt, expectedDueAt, expectedStateJSON, state, reviewLog)
}

func UpdateFlashcardReviewTx(tx *sql.Tx, cardID string, dueAt int64, expectedDueAt int64, expectedStateJSON string, state models.FlashcardState, reviewLog models.FSRSReviewLog) error {
	cardID = strings.TrimSpace(cardID)
	if cardID == "" {
		return fmt.Errorf("flashcard id is required")
	}
	if dueAt <= 0 {
		return fmt.Errorf("due time is required")
	}
	return updateFlashcardReviewRepoTx(tx, cardID, dueAt, expectedDueAt, expectedStateJSON, state, reviewLog)
}

func GetDueReviewCardsForNotebook(notebookID string, now int64, limit int) ([]models.Flashcard, error) {
	notebookID = strings.TrimSpace(notebookID)
	if notebookID == "" {
		return nil, fmt.Errorf("notebook id is required")
	}
	if now <= 0 {
		return nil, fmt.Errorf("current time is required")
	}
	if limit <= 0 {
		limit = maxReviewSessionCards
	}
	return getDueReviewCardsForNotebookRepo(notebookID, now, limit)
}

func GetNextDueReviewNotebook(now int64) (string, int, error) {
	if now <= 0 {
		return "", 0, fmt.Errorf("current time is required")
	}
	return getNextDueReviewNotebookRepo(now)
}

func CreateReviewSession(notebookID string) (*models.StudyQueueTask, bool, error) {
	notebookID = strings.TrimSpace(notebookID)
	if notebookID == "" {
		return nil, false, fmt.Errorf("notebook id is required")
	}
	return createReviewSessionRepo(notebookID, reviewSessionNow())
}

func GetReviewSession(taskID string) (*models.ReviewSession, error) {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return nil, fmt.Errorf("task id is required")
	}
	return getReviewSessionRepo(taskID)
}

func MarkReviewTaskCardReviewedTx(tx *sql.Tx, taskID, cardID string) error {
	taskID = strings.TrimSpace(taskID)
	cardID = strings.TrimSpace(cardID)
	if taskID == "" || cardID == "" {
		return fmt.Errorf("task id and card id are required")
	}
	return markReviewTaskCardReviewedTxRepo(tx, taskID, cardID)
}

func RemainingReviewTaskCardsTx(tx *sql.Tx, taskID string) (int, error) {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return 0, fmt.Errorf("task id is required")
	}
	return remainingReviewTaskCardsTxRepo(tx, taskID)
}

func CompleteReviewSession(taskID string) error {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return fmt.Errorf("task id is required")
	}
	return completeReviewSessionRepo(taskID)
}

// InsertFSRSReviewLog inserts one generic FSRS review event.
func InsertFSRSReviewLog(reviewLog models.FSRSReviewLog) error {
	reviewLog.ID = strings.TrimSpace(reviewLog.ID)
	reviewLog.TopicID = strings.TrimSpace(reviewLog.TopicID)
	reviewLog.ActivityType = strings.TrimSpace(reviewLog.ActivityType)
	reviewLog.ReferenceID = strings.TrimSpace(reviewLog.ReferenceID)
	reviewLog.StateBeforeJSON = strings.TrimSpace(reviewLog.StateBeforeJSON)
	reviewLog.StateAfterJSON = strings.TrimSpace(reviewLog.StateAfterJSON)

	if reviewLog.ID == "" {
		return fmt.Errorf("review log id is required")
	}
	if reviewLog.TopicID == "" {
		return fmt.Errorf("topic id is required")
	}
	if reviewLog.ActivityType == "" {
		return fmt.Errorf("activity type is required")
	}
	if reviewLog.ReferenceID == "" {
		return fmt.Errorf("reference id is required")
	}
	if reviewLog.ReviewedAt <= 0 {
		return fmt.Errorf("reviewed at is required")
	}
	if reviewLog.Rating < 1 || reviewLog.Rating > 4 {
		return fmt.Errorf("rating must be between 1 and 4")
	}
	if reviewLog.StateBeforeJSON == "" || reviewLog.StateAfterJSON == "" {
		return fmt.Errorf("review state json values are required")
	}
	if reviewLog.ScheduledDays < 0 {
		return fmt.Errorf("scheduled days must be non-negative")
	}

	return insertFSRSReviewLogRepo(reviewLog)
}

// GetOrCreateFlashcardsForTopic atomically fetches existing non-suspended flashcards or creates new ones.
// If non-suspended flashcards already exist for the topic, they are returned and existing=true.
// If the topic has no non-suspended flashcards, the provided cards and states are inserted transactionally,
// and the inserted cards are returned with existing=false.
// This prevents race conditions where multiple concurrent requests both see zero cards.
func GetOrCreateFlashcardsForTopic(topicID string, cardsIfNotExist []models.Flashcard, statesIfNotExist map[string]models.FlashcardState) ([]models.Flashcard, bool, error) {
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

	return getOrCreateFlashcardsForTopicRepo(topicID, normalizedCards, statesIfNotExist)
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

// Vector Search and Storage Functions

// createVectorTable creates the vec0 virtual table with the discovered embedding dimension.
func createVectorTable() error {
	if embeddingDimension <= 0 {
		return fmt.Errorf("embedding dimension not initialized")
	}

	// Create vec0 virtual table for vector search
	// Format: vec0(embedding float[dimension])
	schema := fmt.Sprintf(`
		CREATE VIRTUAL TABLE IF NOT EXISTS chunk_vectors USING vec0(
			embedding float[%d]
		);
	`, embeddingDimension)

	_, err := conn.Exec(schema)
	if err != nil {
		return fmt.Errorf("failed to create vec0 table: %w", err)
	}

	utils.Infof("Created vec0 virtual table with embedding dimension %d", embeddingDimension)
	return nil
}

// UpsertChunkVector stores or updates a chunk embedding vector.
// It returns an error if validation fails or the vector cannot be persisted.
func UpsertChunkVector(chunkID string, vector []float32) error {
	chunkID = strings.TrimSpace(chunkID)
	if chunkID == "" {
		return fmt.Errorf("chunk id is required")
	}
	if len(vector) == 0 {
		return fmt.Errorf("vector is required")
	}
	return upsertChunkVectorRepo(chunkID, vector)
}

// ChunkVectorBatchItem contains one vector persistence request.
type ChunkVectorBatchItem struct {
	ChunkID      string
	Vector       []float32
	EmbeddingRef string
}

// UpsertChunkVectorsBatch stores vectors and embedding refs in a single transaction.
func UpsertChunkVectorsBatch(items []ChunkVectorBatchItem) error {
	if len(items) == 0 {
		return nil
	}

	repoItems := make([]chunkVectorBatchItemRepo, 0, len(items))
	for _, item := range items {
		item.ChunkID = strings.TrimSpace(item.ChunkID)
		if item.ChunkID == "" {
			return fmt.Errorf("chunk id is required for each batch item")
		}
		if len(item.Vector) == 0 {
			return fmt.Errorf("vector is required for each batch item")
		}
		repoItems = append(repoItems, chunkVectorBatchItemRepo(item))
	}

	return upsertChunkVectorsBatchRepo(repoItems)
}

// SearchVectorsForTopic finds the top-k most similar vectors for a topic-scoped query.
// When startPage and endPage are positive, search is context-locked to that page window.
func SearchVectorsForTopic(topicID string, queryVector []float32, k int, startPage int, endPage int) ([]string, error) {
	topicID = strings.TrimSpace(topicID)
	if topicID == "" {
		return nil, fmt.Errorf("topic id is required")
	}
	if len(queryVector) == 0 {
		return nil, fmt.Errorf("query vector is required")
	}
	if k <= 0 || k > maxRetrievalK {
		return nil, fmt.Errorf("k must be between 1 and %d", maxRetrievalK)
	}
	return searchVectorsForTopicRepo(topicID, queryVector, k, startPage, endPage)
}

// SearchVectorsForNotebook finds the top-k most similar vectors for a notebook-scoped query.
func SearchVectorsForNotebook(notebookID string, queryVector []float32, k int) ([]string, error) {
	notebookID = strings.TrimSpace(notebookID)
	if notebookID == "" {
		return nil, fmt.Errorf("notebook id is required")
	}
	if len(queryVector) == 0 {
		return nil, fmt.Errorf("query vector is required")
	}
	if k <= 0 || k > maxRetrievalK {
		return nil, fmt.Errorf("k must be between 1 and %d", maxRetrievalK)
	}
	return searchVectorsForNotebookRepo(notebookID, queryVector, k)
}

// UpdateChunkEmbedding updates the embedding_ref (hash) for a chunk to track changes.
func UpdateChunkEmbedding(chunkID string, hash string) error {
	_, err := conn.Exec(`
		UPDATE chunks SET embedding_ref = ? WHERE id = ?
	`, hash, chunkID)
	return err
}

// ChunkEmbeddingBatchItem represents a chunk embedding update to be processed in batch
type ChunkEmbeddingBatchItem struct {
	ChunkID string
	Hash    string
}

// UpdateChunkEmbeddingsBatch updates embedding metadata for multiple chunks in a single transaction
func UpdateChunkEmbeddingsBatch(items []ChunkEmbeddingBatchItem) error {
	if len(items) == 0 {
		return nil
	}

	return withTx(func(tx *sql.Tx) error {
		stmt, err := tx.Prepare(`
			UPDATE chunks SET embedding_ref = ? WHERE id = ?
		`)
		if err != nil {
			return err
		}
		defer func() {
			_ = stmt.Close()
		}()

		for _, item := range items {
			if item.ChunkID == "" {
				return fmt.Errorf("chunk id is required for all batch items")
			}

			res, err := stmt.Exec(item.Hash, item.ChunkID)
			if err != nil {
				return err
			}
			rowsAffected, err := res.RowsAffected()
			if err != nil {
				return err
			}
			if rowsAffected == 0 {
				return fmt.Errorf("no rows inserted for chunk_id %s", item.ChunkID)
			}
		}
		return nil
	})
}

// GetChunkEmbeddingRefsForTopic returns embedding_ref values for all chunks in a topic.
func GetChunkEmbeddingRefsForTopic(topicID string) (map[string]string, error) {
	rows, err := conn.Query(`
		SELECT id, COALESCE(embedding_ref, '')
		FROM chunks
		WHERE topic_id = ?
	`, topicID)
	if err != nil {
		return nil, err
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			log.Printf("warning: failed to close chunk embedding refs rows: %v", closeErr)
		}
	}()

	refs := make(map[string]string)
	for rows.Next() {
		var chunkID string
		var hash string
		if err := rows.Scan(&chunkID, &hash); err != nil {
			return nil, err
		}
		refs[chunkID] = hash
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return refs, nil
}

// CreateWrittenQuestion stores one persisted written assessment prompt.
func CreateWrittenQuestion(question models.WrittenQuestion) error {
	question.ID = strings.TrimSpace(question.ID)
	question.TopicID = strings.TrimSpace(question.TopicID)
	question.Prompt = strings.TrimSpace(question.Prompt)
	if question.ID == "" {
		return fmt.Errorf("question id is required")
	}
	if question.TopicID == "" {
		return fmt.Errorf("topic id is required")
	}
	if question.Prompt == "" {
		return fmt.Errorf("prompt is required")
	}
	return createWrittenQuestionRepo(question)
}

// GetWrittenQuestionByID fetches one persisted written assessment prompt.
func GetWrittenQuestionByID(questionID string) (*models.WrittenQuestion, error) {
	questionID = strings.TrimSpace(questionID)
	if questionID == "" {
		return nil, fmt.Errorf("question id is required")
	}
	return getWrittenQuestionByIDRepo(questionID)
}
