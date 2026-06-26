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

type Repository struct {
	db                 *sql.DB
	embeddingDimension int32
}

const maxRetrievalK = 100 // Maximum k allowed for vector search retrieval

const (
	llmTierFast  = "fast"
	llmTierHeavy = "heavy"
)

// Close releases the active SQLite connection.
func (r *Repository) Close() error {
	if r.db == nil {
		return nil
	}
	err := r.db.Close()
	r.db = nil
	return err
}

// SwapDB swaps the underlying database connection in-place and returns the old connection.
func (r *Repository) SwapDB(newRepo *Repository) *sql.DB {
	oldDB := r.db
	r.db = newRepo.db
	r.embeddingDimension = newRepo.embeddingDimension
	return oldDB
}


// Begin starts a new transaction on the database.
func (r *Repository) Begin() (*sql.Tx, error) {
	return r.db.Begin()
}

// GetActiveProfileID retrieves the active profile ID from settings.
func (r *Repository) GetActiveProfileID() (string, error) {
	var activeProfileID sql.NullString
	err := r.db.QueryRow(`SELECT COALESCE(active_profile_id, '') FROM user_settings WHERE id = 1`).Scan(&activeProfileID)
	if err != nil {
		return "", err
	}
	return activeProfileID.String, nil
}



// ExecForTest executes a query directly on the underlying database. ONLY for test usage.
func (r *Repository) ExecForTest(query string, args ...interface{}) (sql.Result, error) {
	return r.db.Exec(query, args...)
}

// QueryRowForTest runs a QueryRow directly on the underlying database. ONLY for test usage.
func (r *Repository) QueryRowForTest(query string, args ...interface{}) *sql.Row {
	return r.db.QueryRow(query, args...)
}

// Init initializes the SQLite database and creates tables
// vec0DllPath should be the absolute path to vec0.dll (sqlite-vec extension)
func Init(dbPath, vec0DllPath string) (*Repository, error) {
	utils.RagLogger.Info("db.Init: initializing database pool", "dbPath", dbPath, "vec0DllPath", vec0DllPath)
	driverName := "sqlite3"
	if vec0DllPath != "" {
		if _, err := os.Stat(vec0DllPath); err == nil {
			absPath, err := filepath.Abs(vec0DllPath)
			if err == nil {
				utils.RagLogger.Info("db.Init: vec0 file verified, preparing sqlite3_tutor driver", "absPath", absPath)
				setExtensionPath(absPath)
				driverName = "sqlite3_tutor"
			} else {
				utils.RagLogger.Warn("db.Init: failed to resolve absolute path for vec0 file", "path", vec0DllPath, "error", err)
			}
		} else {
			utils.RagLogger.Warn("db.Init: vec0 file not found at path", "path", vec0DllPath, "error", err)
		}
	}

	utils.RagLogger.Info("db.Init: opening SQL connection pool", "driverName", driverName)
	dbConn, err := sql.Open(driverName, "file:"+dbPath+"?_foreign_keys=on&_busy_timeout=5000")
	if err != nil {
		utils.RagLogger.Error("db.Init: failed to open SQL connection pool", "driverName", driverName, "error", err)
		return nil, err
	}
	dbConn.SetMaxOpenConns(1)
	dbConn.SetMaxIdleConns(1)

	utils.RagLogger.Info("db.Init: pinging database connection")
	if err := dbConn.Ping(); err != nil {
		utils.RagLogger.Error("db.Init: database connection ping failed", "error", err)
		if closeErr := dbConn.Close(); closeErr != nil {
			utils.RagLogger.Warn("db.Init: failed to close database connection after ping error", "error", closeErr)
		}
		return nil, err
	}
	utils.RagLogger.Info("db.Init: database connection ping succeeded")

	// Verify extension load if custom driver was used
	if driverName == "sqlite3_tutor" {
		var version string
		utils.RagLogger.Info("db.Init: verifying extension load by running SELECT vec_version()")
		if err := dbConn.QueryRow("SELECT vec_version()").Scan(&version); err != nil {
			utils.RagLogger.Warn("db.Init: vector verification failed, falling back to standard sqlite3", "error", err, "path", vec0DllPath)
			setExtensionPath("")
			_ = dbConn.Close()

			// Fallback to standard sqlite3
			var fbErr error
			utils.RagLogger.Info("db.Init: opening fallback standard sqlite3 connection pool")
			dbConn, fbErr = sql.Open("sqlite3", "file:"+dbPath+"?_foreign_keys=on&_busy_timeout=5000")
			if fbErr != nil {
				utils.RagLogger.Error("db.Init: standard sqlite3 fallback connection pool failed to open", "error", fbErr)
				return nil, fbErr
			}
			dbConn.SetMaxOpenConns(1)
			dbConn.SetMaxIdleConns(1)
		} else {
			utils.RagLogger.Info("db.Init: successfully verified sqlite-vec extension", "path", vec0DllPath, "version", version)
		}
	}

	// Nuclear strategy: Initialize schema with a single transaction
	tx, err := dbConn.Begin()
	if err != nil {
		if closeErr := dbConn.Close(); closeErr != nil {
			log.Printf("Warning: failed to close database connection after begin error: %v", closeErr)
		}
		return nil, fmt.Errorf("failed to begin schema transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	if err := InitSchema(tx); err != nil {
		if closeErr := dbConn.Close(); closeErr != nil {
			log.Printf("Warning: failed to close database connection after schema error: %v", closeErr)
		}
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	if err := tx.Commit(); err != nil {
		if closeErr := dbConn.Close(); closeErr != nil {
			log.Printf("Warning: failed to close database connection after commit error: %v", closeErr)
		}
		return nil, fmt.Errorf("failed to commit schema transaction: %w", err)
	}

	return &Repository{db: dbConn}, nil
}

// IsVecExtensionLoaded checks if the sqlite-vec (vec0) extension is loaded and functional.
func (r *Repository) IsVecExtensionLoaded() bool {
	if r.db == nil {
		return false
	}
	var version string
	err := r.db.QueryRow("SELECT vec_version()").Scan(&version)
	return err == nil
}

// InitWithVectorDimension initializes the database and creates the vec0 virtual table.
// Called after ONNX embedder dimension is discovered.
func (r *Repository) InitWithVectorDimension(embeddingDim int32) error {
	if embeddingDim <= 0 {
		return fmt.Errorf("invalid embedding dimension: %d", embeddingDim)
	}
	r.embeddingDimension = embeddingDim

	// Create vec0 virtual table with the discovered dimension
	return r.createVectorTable()
}

// QueryDueReviewCards counts cards due by the given time, scoped to existing topics.
// Excludes cards already linked to pending/active review tasks to avoid double-counting.
func (r *Repository) QueryDueReviewCards(now int64) (int, error) {
	var activeProfileID sql.NullString
	if err := r.db.QueryRow(`
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

	err := r.db.QueryRow(query, args...).Scan(&count)
	return count, err
}

// QueryDueReviewCardsForRange counts cards due within a specific time range (start, end], scoped to the active profile.
// Excludes cards already linked to pending/active review tasks to avoid double-counting.
func (r *Repository) QueryDueReviewCardsForRange(start int64, end int64) (int, error) {
	var activeProfileID sql.NullString
	if err := r.db.QueryRow(`
		SELECT COALESCE(active_profile_id, '') FROM user_settings WHERE id = 1
	`).Scan(&activeProfileID); err != nil && err != sql.ErrNoRows {
		return 0, fmt.Errorf("QueryDueReviewCardsForRange: reading active_profile_id: %w", err)
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
		  AND fc.due_at > ?
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
	args = append(args, start, end)
	if activeProfileStr != "" {
		query += ` AND (n.profile_id = ? OR n.profile_id IS NULL OR n.profile_id = '') `
		args = append(args, activeProfileStr)
	}

	err := r.db.QueryRow(query, args...).Scan(&count)
	return count, err
}




// GetRAGEnabled returns the status of RAG flag.
func (r *Repository) GetRAGEnabled() (bool, error) {
	var enabled bool
	err := r.db.QueryRow(`
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
func (r *Repository) GetDefaultProfileID() (string, error) {
	var id string
	err := r.db.QueryRow(`
		SELECT id FROM study_profiles ORDER BY created_at ASC LIMIT 1
	`).Scan(&id)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return id, err
}

// GetUserSettings returns the full settings config.
func (r *Repository) GetUserSettings() (*models.UserSettings, error) {
	var s models.UserSettings
	var activeProfileID sql.NullString
	err := r.db.QueryRow(`
		SELECT max_flashcards_per_session, COALESCE(study_start_time, '17:00'), COALESCE(study_end_time, '18:00'), COALESCE(reminders_enabled, 1), COALESCE(active_profile_id, ''), skip_to_reading_active, COALESCE(cloud_sync_url, ''), COALESCE(cloud_api_token, ''), COALESCE(theme, 'light-classic'), COALESCE(rag_enabled, 0), COALESCE(rag_notebook_chapter, 1), COALESCE(rag_entire_notebook, 1), COALESCE(rag_queue_study, 1), COALESCE(default_remedial_strategy, 'CLASSIC')
		FROM user_settings
		WHERE id = 1
	`).Scan(&s.MaxFlashcardsPerSession, &s.StudyStartTime, &s.StudyEndTime, &s.RemindersEnabled, &activeProfileID, &s.SkipToReadingActive, &s.CloudSyncURL, &s.CloudAPIToken, &s.Theme, &s.RAGEnabled, &s.RAGNotebookChapter, &s.RAGEntireNotebook, &s.RAGQueueStudy, &s.DefaultRemedialStrategy)
	if err == sql.ErrNoRows {
		s = models.UserSettings{
			MaxFlashcardsPerSession: 30,
			StudyStartTime:          "17:00",
			StudyEndTime:            "18:00",
			RemindersEnabled:        true,
			Theme:                   "light-classic",
			RAGEnabled:              false,
			RAGNotebookChapter:     true,
			RAGEntireNotebook:      true,
			RAGQueueStudy:          true,
			DefaultRemedialStrategy: "CLASSIC",
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
		defaultID, err := r.GetDefaultProfileID()
		if err != nil {
			return nil, fmt.Errorf("failed to resolve default profile: %w", err)
		}
		if defaultID != "" {
			if _, err := r.db.Exec(`UPDATE user_settings SET active_profile_id = ? WHERE id = 1`, defaultID); err != nil {
				return nil, fmt.Errorf("failed to persist active profile ID: %w", err)
			}
			s.ActiveProfileID = defaultID
		}
	} else {
		// Verify if the active profile still exists
		var exists bool
		if err := r.db.QueryRow(`SELECT EXISTS(SELECT 1 FROM study_profiles WHERE id = ?)`, s.ActiveProfileID).Scan(&exists); err != nil {
			return nil, fmt.Errorf("failed to check active profile existence: %w", err)
		}
		if !exists {
			defaultID, err := r.GetDefaultProfileID()
			if err != nil {
				return nil, fmt.Errorf("failed to resolve fallback default profile: %w", err)
			}
			if defaultID != "" {
				if _, err := r.db.Exec(`UPDATE user_settings SET active_profile_id = ? WHERE id = 1`, defaultID); err != nil {
					return nil, fmt.Errorf("failed to persist fallback active profile ID: %w", err)
				}
				s.ActiveProfileID = defaultID
			} else {
				if _, err := r.db.Exec(`UPDATE user_settings SET active_profile_id = NULL WHERE id = 1`); err != nil {
					return nil, fmt.Errorf("failed to clear inactive active profile ID: %w", err)
				}
				s.ActiveProfileID = ""
			}
		}
	}

	return &s, nil
}

// UpdateUserSettings updates the user settings.
func (r *Repository) UpdateUserSettings(s models.UserSettings) error {
	var activeProfileID interface{} = nil
	if s.ActiveProfileID != "" {
		activeProfileID = s.ActiveProfileID
	}
	theme := s.Theme
	if theme == "" {
		theme = "light-classic"
	}
	strategy := s.DefaultRemedialStrategy
	if strategy == "" {
		strategy = "CLASSIC"
	}
	_, err := r.db.Exec(`
		INSERT INTO user_settings (id, max_flashcards_per_session, study_start_time, study_end_time, reminders_enabled, active_profile_id, skip_to_reading_active, cloud_sync_url, cloud_api_token, theme, rag_enabled, rag_notebook_chapter, rag_entire_notebook, rag_queue_study, default_remedial_strategy)
		VALUES (1, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			max_flashcards_per_session = excluded.max_flashcards_per_session,
			study_start_time = excluded.study_start_time,
			study_end_time = excluded.study_end_time,
			reminders_enabled = excluded.reminders_enabled,
			active_profile_id = excluded.active_profile_id,
			skip_to_reading_active = excluded.skip_to_reading_active,
			cloud_sync_url = excluded.cloud_sync_url,
			cloud_api_token = excluded.cloud_api_token,
			theme = excluded.theme,
			rag_enabled = excluded.rag_enabled,
			rag_notebook_chapter = excluded.rag_notebook_chapter,
			rag_entire_notebook = excluded.rag_entire_notebook,
			rag_queue_study = excluded.rag_queue_study,
			default_remedial_strategy = excluded.default_remedial_strategy,
			updated_at = CURRENT_TIMESTAMP
	`, s.MaxFlashcardsPerSession, s.StudyStartTime, s.StudyEndTime, s.RemindersEnabled, activeProfileID, s.SkipToReadingActive, s.CloudSyncURL, s.CloudAPIToken, theme, s.RAGEnabled, s.RAGNotebookChapter, s.RAGEntireNotebook, s.RAGQueueStudy, strategy)
	return err
}

func (r *Repository) GetLLMSettings() (*models.LLMSettings, error) {
	rows, err := r.db.Query(`
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

func (r *Repository) UpdateLLMSettings(settings models.LLMSettings) error {
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

	tx, err := r.db.Begin()
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

func (r *Repository) MarkLLMKeyStored(tier string, stored bool) error {
	tier = normalizeLLMTier(tier)
	if tier == "" {
		return fmt.Errorf("llm tier is required")
	}
	_, err := r.db.Exec(`
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
func (r *Repository) GetProfiles() ([]models.StudyProfile, error) {
	rows, err := r.db.Query(`
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
func (r *Repository) GetProfileByID(id string) (*models.StudyProfile, error) {
	var p models.StudyProfile
	err := r.db.QueryRow(`
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
func (r *Repository) CreateProfile(p models.StudyProfile) error {
	_, err := r.db.Exec(`
		INSERT INTO study_profiles (id, name, deadline_at)
		VALUES (?, ?, ?)
	`, p.ID, p.Name, p.DeadlineAt)
	return err
}

// UpdateProfile updates an existing profile.
func (r *Repository) UpdateProfile(p models.StudyProfile) error {
	_, err := r.db.Exec(`
		UPDATE study_profiles
		SET name = ?, deadline_at = ?
		WHERE id = ?
	`, p.Name, p.DeadlineAt, p.ID)
	return err
}

// DeleteProfile deletes a profile atomically.
func (r *Repository) DeleteProfile(id string) error {
	tx, err := r.db.Begin()
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

// createVectorTable creates the vec0 virtual table with the discovered embedding dimension.
func (r *Repository) createVectorTable() error {
	if r.embeddingDimension <= 0 {
		return fmt.Errorf("embedding dimension not initialized")
	}

	// Create vec0 virtual table for vector search
	schema := fmt.Sprintf(`
		CREATE VIRTUAL TABLE IF NOT EXISTS chunk_vectors USING vec0(
			embedding float[%d]
		);
	`, r.embeddingDimension)

	_, err := r.db.Exec(schema)
	if err != nil {
		return fmt.Errorf("failed to create vec0 table: %w", err)
	}

	utils.Infof("Created vec0 virtual table with embedding dimension %d", r.embeddingDimension)
	return nil
}

// UpdateChunkEmbedding updates the embedding_ref (hash) for a chunk to track changes.
func (r *Repository) UpdateChunkEmbedding(chunkID string, hash string) error {
	_, err := r.db.Exec(`
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
func (r *Repository) UpdateChunkEmbeddingsBatch(items []ChunkEmbeddingBatchItem) error {
	if len(items) == 0 {
		return nil
	}

	return r.withTx(func(tx *sql.Tx) error {
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
func (r *Repository) GetChunkEmbeddingRefsForTopic(topicID string) (map[string]string, error) {
	rows, err := r.db.Query(`
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

func (r *Repository) GetRemedialStrategy() (string, error) {
	var strategy string
	err := r.db.QueryRow(
		`SELECT COALESCE(default_remedial_strategy, 'CLASSIC') FROM user_settings WHERE id = 1`,
	).Scan(&strategy)
	if err == sql.ErrNoRows {
		return "CLASSIC", nil
	}
	if err != nil {
		return "", err
	}
	if strategy == "" {
		return "CLASSIC", nil
	}
	return strategy, nil
}

func (r *Repository) SetRemedialStrategy(strategy string) error {
	_, err := r.db.Exec(
		`UPDATE user_settings SET default_remedial_strategy = ? WHERE id = 1`, strategy,
	)
	return err
}

