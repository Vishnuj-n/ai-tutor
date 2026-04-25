package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"ai-tutor/internal/models"
	"ai-tutor/internal/utils"
)

var conn *sql.DB
var embeddingDimension int32 = 0 // Will be set during DB initialization with vec0

const maxRetrievalK = 100 // Maximum k allowed for vector search retrieval

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
	conn, err = sql.Open("sqlite3", dbPath)
	if err != nil {
		return err
	}
	conn.SetMaxOpenConns(1)
	conn.SetMaxIdleConns(1)

	if err := conn.Ping(); err != nil {
		return err
	}
	if _, err := conn.Exec(`PRAGMA foreign_keys = ON`); err != nil {
		return fmt.Errorf("failed to enable foreign keys: %w", err)
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

	// Create tables
	if err := createTables(); err != nil {
		return err
	}

	if err := ensureNotebookSchema(); err != nil {
		return err
	}

	if err := ensureTopicBoundsSchema(); err != nil {
		return err
	}

	if err := ensureQuestionsSchema(); err != nil {
		return err
	}

	if err := ensureUserSettingsSchema(); err != nil {
		return err
	}

	if err := ensureFSRSSchema(); err != nil {
		return err
	}
	if err := ensureAssessmentSchema(); err != nil {
		return err
	}
	if err := ensureCascadeForeignKeys(); err != nil {
		return err
	}

	return nil
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

func createTables() error {
	schema := `
	CREATE TABLE IF NOT EXISTS topics (
		id TEXT PRIMARY KEY,
		title TEXT NOT NULL,
		status TEXT DEFAULT 'reading',
		start_page INTEGER DEFAULT 0,
		end_page INTEGER DEFAULT 0,
		current_page_cursor INTEGER DEFAULT 0,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS parents (
		id TEXT PRIMARY KEY,
		topic_id TEXT NOT NULL,
		heading TEXT,
		order_index INTEGER,
		content_text TEXT NOT NULL,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (topic_id) REFERENCES topics(id)
	);

	CREATE TABLE IF NOT EXISTS chunks (
		id TEXT PRIMARY KEY,
		topic_id TEXT NOT NULL,
		parent_id TEXT NOT NULL,
		chunk_text TEXT NOT NULL,
		page_num INTEGER DEFAULT 0,
		token_count INTEGER DEFAULT 0,
		importance_score REAL DEFAULT 0,
		weakness_score REAL DEFAULT 0,
		embedding_ref TEXT,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (topic_id) REFERENCES topics(id),
		FOREIGN KEY (parent_id) REFERENCES parents(id)
	);

	CREATE TABLE IF NOT EXISTS topic_progress (
		topic_id TEXT PRIMARY KEY,
		learned_at TIMESTAMP,
		last_read_at TIMESTAMP,
		mastery_score REAL DEFAULT 0,
		review_enabled INTEGER DEFAULT 0,
		FOREIGN KEY (topic_id) REFERENCES topics(id)
	);

	CREATE TABLE IF NOT EXISTS questions (
		id TEXT PRIMARY KEY,
		topic_id TEXT NOT NULL,
		prompt TEXT NOT NULL,
		options_json TEXT NOT NULL,
		correct_answer TEXT NOT NULL,
		explanation TEXT,
		hint TEXT,
		source_heading TEXT,
		source_snippet TEXT,
		source_page_start INTEGER DEFAULT 0,
		source_page_end INTEGER DEFAULT 0,
		llm_model TEXT,
		prompt_version TEXT,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (topic_id) REFERENCES topics(id) ON DELETE CASCADE
	);

	CREATE TABLE IF NOT EXISTS user_settings (
		id INTEGER PRIMARY KEY CHECK (id = 1),
		daily_study_minutes INTEGER NOT NULL DEFAULT 90,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS user_answers (
		id TEXT PRIMARY KEY,
		question_id TEXT NOT NULL,
		user_answer TEXT NOT NULL,
		is_correct INTEGER NOT NULL,
		score INTEGER NOT NULL,
		feedback TEXT,
		hint TEXT,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (question_id) REFERENCES questions(id) ON DELETE CASCADE
	);

	CREATE TABLE IF NOT EXISTS written_user_answers (
		id TEXT PRIMARY KEY,
		written_question_id TEXT NOT NULL,
		user_answer TEXT NOT NULL,
		score INTEGER NOT NULL,
		feedback TEXT,
		source_heading TEXT,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (written_question_id) REFERENCES written_questions(id) ON DELETE CASCADE
	);

	CREATE TABLE IF NOT EXISTS notebooks (
		id TEXT PRIMARY KEY,
		title TEXT NOT NULL,
		file_path TEXT NOT NULL,
		file_type TEXT DEFAULT 'pdf',
		topic_id TEXT,
		status TEXT DEFAULT 'uploaded',
		page_count INTEGER,
		chunk_count INTEGER DEFAULT 0,
		uploaded_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (topic_id) REFERENCES topics(id)
	);

	CREATE TABLE IF NOT EXISTS notebook_chunks (
		id TEXT PRIMARY KEY,
		notebook_id TEXT NOT NULL,
		chunk_id TEXT NOT NULL,
		page_num INTEGER DEFAULT 0,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (notebook_id) REFERENCES notebooks(id),
		FOREIGN KEY (chunk_id) REFERENCES chunks(id)
	);
	`

	_, err := conn.Exec(schema)
	if err != nil {
		return err
	}

	_, err = conn.Exec(`
		INSERT INTO user_settings (id, daily_study_minutes)
		VALUES (1, 90)
		ON CONFLICT(id) DO NOTHING
	`)
	if err != nil {
		return err
	}

	return nil
}

func ensureNotebookSchema() error {
	rows, err := conn.Query("PRAGMA table_info(notebooks)")
	if err != nil {
		return err
	}
	defer func() {
		_ = rows.Close()
	}()

	hasStatus := false
	for rows.Next() {
		var cid int
		var name string
		var ctype string
		var notnull int
		var dflt sql.NullString
		var pk int
		if scanErr := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); scanErr != nil {
			return scanErr
		}
		if name == "status" {
			hasStatus = true
			break
		}
	}

	if !hasStatus {
		if _, alterErr := conn.Exec("ALTER TABLE notebooks ADD COLUMN status TEXT DEFAULT 'uploaded'"); alterErr != nil {
			return alterErr
		}
	}

	return rows.Err()
}

func ensureTopicBoundsSchema() error {
	rows, err := conn.Query("PRAGMA table_info(topics)")
	if err != nil {
		return err
	}
	defer func() {
		_ = rows.Close()
	}()

	hasStartPage := false
	hasEndPage := false
	hasCursor := false
	for rows.Next() {
		var cid int
		var name string
		var ctype string
		var notnull int
		var dflt sql.NullString
		var pk int
		if scanErr := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); scanErr != nil {
			return scanErr
		}
		switch name {
		case "start_page":
			hasStartPage = true
		case "end_page":
			hasEndPage = true
		case "current_page_cursor":
			hasCursor = true
		}
	}

	if !hasStartPage {
		if _, alterErr := conn.Exec("ALTER TABLE topics ADD COLUMN start_page INTEGER DEFAULT 0"); alterErr != nil {
			return alterErr
		}
	}

	if !hasEndPage {
		if _, alterErr := conn.Exec("ALTER TABLE topics ADD COLUMN end_page INTEGER DEFAULT 0"); alterErr != nil {
			return alterErr
		}
	}

	if !hasCursor {
		if _, alterErr := conn.Exec("ALTER TABLE topics ADD COLUMN current_page_cursor INTEGER DEFAULT 0"); alterErr != nil {
			return alterErr
		}
	}

	return rows.Err()
}

func ensureQuestionsSchema() error {
	// Check for missing columns in questions table
	rows, err := conn.Query("PRAGMA table_info(questions)")
	if err != nil {
		return err
	}
	defer func() {
		_ = rows.Close()
	}()

	columnsFound := map[string]bool{}
	for rows.Next() {
		var cid int
		var name string
		var ctype string
		var notnull int
		var dflt sql.NullString
		var pk int
		if scanErr := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); scanErr != nil {
			return scanErr
		}
		columnsFound[name] = true
	}
	if err := rows.Err(); err != nil {
		return err
	}

	requiredColumns := map[string]string{
		"hint":              "TEXT",
		"source_heading":    "TEXT",
		"source_snippet":    "TEXT",
		"source_page_start": "INTEGER DEFAULT 0",
		"source_page_end":   "INTEGER DEFAULT 0",
		"llm_model":         "TEXT",
		"prompt_version":    "TEXT",
	}

	for col, colType := range requiredColumns {
		if !columnsFound[col] {
			if _, alterErr := conn.Exec(fmt.Sprintf("ALTER TABLE questions ADD COLUMN %s %s", col, colType)); alterErr != nil {
				return alterErr
			}
		}
	}

	// Check for missing columns in user_answers table
	rows2, err := conn.Query("PRAGMA table_info(user_answers)")
	if err != nil {
		return err
	}
	defer func() {
		_ = rows2.Close()
	}()

	columnsFound2 := map[string]bool{}
	for rows2.Next() {
		var cid int
		var name string
		var ctype string
		var notnull int
		var dflt sql.NullString
		var pk int
		if scanErr := rows2.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); scanErr != nil {
			return scanErr
		}
		columnsFound2[name] = true
	}

	if !columnsFound2["hint"] {
		if _, alterErr := conn.Exec("ALTER TABLE user_answers ADD COLUMN hint TEXT"); alterErr != nil {
			return alterErr
		}
	}

	return rows2.Err()
}

func ensureUserSettingsSchema() error {
	rows, err := conn.Query("PRAGMA table_info(user_settings)")
	if err != nil {
		return err
	}
	defer func() {
		_ = rows.Close()
	}()

	hasDailyStudyMinutes := false
	for rows.Next() {
		var cid int
		var name string
		var ctype string
		var notnull int
		var dflt sql.NullString
		var pk int
		if scanErr := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); scanErr != nil {
			return scanErr
		}
		if name == "daily_study_minutes" {
			hasDailyStudyMinutes = true
		}
	}

	if !hasDailyStudyMinutes {
		if _, alterErr := conn.Exec("ALTER TABLE user_settings ADD COLUMN daily_study_minutes INTEGER NOT NULL DEFAULT 90"); alterErr != nil {
			return alterErr
		}
	}

	if err := rows.Err(); err != nil {
		return err
	}

	_, err = conn.Exec(`
		INSERT INTO user_settings (id, daily_study_minutes)
		VALUES (1, 90)
		ON CONFLICT(id) DO NOTHING
	`)
	return err
}

func ensureFSRSSchema() error {
	var tableName string
	err := conn.QueryRow(`
		SELECT name
		FROM sqlite_master
		WHERE type = 'table' AND name = 'fsrs_review_log'
	`).Scan(&tableName)
	if err != nil && err != sql.ErrNoRows {
		return err
	}

	// Clean-slate migration for old heuristic schema.
	if err == sql.ErrNoRows {
		tx, beginErr := conn.Begin()
		if beginErr != nil {
			return beginErr
		}
		defer func() {
			_ = tx.Rollback()
		}()

		stmts := []string{
			`DROP TABLE IF EXISTS fsrs_cards`,
			`DROP TABLE IF EXISTS fsrs_review_log`,
			`CREATE TABLE fsrs_cards (
					id TEXT PRIMARY KEY,
					topic_id TEXT NOT NULL,
					prompt TEXT NOT NULL,
					answer TEXT NOT NULL,
					state_json TEXT,
					due_at INTEGER,
					suspended BOOLEAN DEFAULT 0,
					created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
					updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
					FOREIGN KEY (topic_id) REFERENCES topics(id) ON DELETE CASCADE
				)`,
			`CREATE TABLE fsrs_review_log (
					id TEXT PRIMARY KEY,
					topic_id TEXT NOT NULL,
					activity_type TEXT NOT NULL,
					reference_id TEXT NOT NULL,
					reviewed_at INTEGER NOT NULL,
					rating INTEGER NOT NULL,
					scheduled_days INTEGER NOT NULL,
					state_before_json TEXT NOT NULL,
					state_after_json TEXT NOT NULL,
					created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
					FOREIGN KEY (topic_id) REFERENCES topics(id) ON DELETE CASCADE
				)`,
		}

		for _, stmt := range stmts {
			if _, err = tx.Exec(stmt); err != nil {
				return err
			}
		}

		if err = tx.Commit(); err != nil {
			return err
		}
	}

	indexes := []string{
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_fsrs_cards_topic_prompt ON fsrs_cards(topic_id, prompt)`,
		`CREATE INDEX IF NOT EXISTS idx_fsrs_review_log_activity_ref_reviewed_at ON fsrs_review_log(activity_type, reference_id, reviewed_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_fsrs_review_log_topic_reviewed_at ON fsrs_review_log(topic_id, reviewed_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_fsrs_cards_suspended_due_at ON fsrs_cards(suspended, due_at)`,
	}
	for _, stmt := range indexes {
		if _, err := conn.Exec(stmt); err != nil {
			return err
		}
	}

	return nil
}

func ensureAssessmentSchema() error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS written_questions (
			id TEXT PRIMARY KEY,
			topic_id TEXT NOT NULL,
			prompt TEXT NOT NULL,
			source_heading TEXT,
			source_page_start INTEGER DEFAULT 0,
			source_page_end INTEGER DEFAULT 0,
			llm_model TEXT,
			prompt_version TEXT,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (topic_id) REFERENCES topics(id) ON DELETE CASCADE
		)`,
		`CREATE TABLE IF NOT EXISTS assessment_fsrs (
			activity_type TEXT NOT NULL,
			reference_id TEXT NOT NULL,
			topic_id TEXT NOT NULL,
			state_json TEXT NOT NULL,
			due_at INTEGER,
			last_reviewed_at INTEGER,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (activity_type, reference_id),
			FOREIGN KEY (topic_id) REFERENCES topics(id) ON DELETE CASCADE
		)`,
		`CREATE INDEX IF NOT EXISTS idx_written_questions_topic_created_at ON written_questions(topic_id, created_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_assessment_fsrs_topic_due_at ON assessment_fsrs(topic_id, due_at)`,
	}

	for _, stmt := range stmts {
		if _, err := conn.Exec(stmt); err != nil {
			return err
		}
	}

	rows, err := conn.Query("PRAGMA table_info(written_questions)")
	if err != nil {
		return err
	}
	defer func() {
		_ = rows.Close()
	}()

	hasUpdatedAt := false
	for rows.Next() {
		var cid int
		var name string
		var ctype string
		var notnull int
		var dflt sql.NullString
		var pk int
		if scanErr := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); scanErr != nil {
			return scanErr
		}
		if name == "updated_at" {
			hasUpdatedAt = true
			break
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if !hasUpdatedAt {
		if _, err := conn.Exec("ALTER TABLE written_questions ADD COLUMN updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP"); err != nil {
			return err
		}
	}

	return nil
}

func ensureCascadeForeignKeys() error {
	if needsCascadeRebuild("questions", []string{"foreign key (topic_id) references topics(id) on delete cascade"}) ||
		needsCascadeRebuild("user_answers", []string{"foreign key (question_id) references questions(id) on delete cascade"}) {
		if err := rebuildQuestionsAndUserAnswersForCascade(); err != nil {
			return err
		}
	}

	if needsCascadeRebuild("fsrs_cards", []string{"foreign key (topic_id) references topics(id) on delete cascade"}) ||
		needsCascadeRebuild("fsrs_review_log", []string{"foreign key (topic_id) references topics(id) on delete cascade"}) {
		if err := rebuildFSRSTablesForCascade(); err != nil {
			return err
		}
	}

	if needsCascadeRebuild("written_questions", []string{"foreign key (topic_id) references topics(id) on delete cascade"}) {
		if err := rebuildWrittenQuestionsForCascade(); err != nil {
			return err
		}
	}

	if needsCascadeRebuild("assessment_fsrs", []string{"foreign key (topic_id) references topics(id) on delete cascade"}) {
		if err := rebuildAssessmentFSRSForCascade(); err != nil {
			return err
		}
	}

	return nil
}

func needsCascadeRebuild(tableName string, requiredSnippets []string) bool {
	createSQL, err := tableCreateSQL(tableName)
	if err != nil || createSQL == "" {
		return false
	}

	normalized := strings.ToLower(strings.Join(strings.Fields(createSQL), " "))
	for _, snippet := range requiredSnippets {
		if !strings.Contains(normalized, snippet) {
			return true
		}
	}
	return false
}

func tableCreateSQL(tableName string) (string, error) {
	var createSQL sql.NullString
	err := conn.QueryRow(`
		SELECT sql
		FROM sqlite_master
		WHERE type = 'table' AND name = ?
	`, tableName).Scan(&createSQL)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return createSQL.String, nil
}

func rebuildQuestionsAndUserAnswersForCascade() (err error) {
	if _, err = conn.Exec(`PRAGMA foreign_keys = OFF`); err != nil {
		return err
	}
	defer func() {
		if _, pragmaErr := conn.Exec(`PRAGMA foreign_keys = ON`); err == nil && pragmaErr != nil {
			err = pragmaErr
		}
	}()

	tx, err := conn.Begin()
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	stmts := []string{
		`CREATE TABLE questions_new (
			id TEXT PRIMARY KEY,
			topic_id TEXT NOT NULL,
			prompt TEXT NOT NULL,
			options_json TEXT NOT NULL,
			correct_answer TEXT NOT NULL,
			explanation TEXT,
			hint TEXT,
			source_heading TEXT,
			source_snippet TEXT,
			source_page_start INTEGER DEFAULT 0,
			source_page_end INTEGER DEFAULT 0,
			llm_model TEXT,
			prompt_version TEXT,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (topic_id) REFERENCES topics(id) ON DELETE CASCADE
		)`,
		`INSERT INTO questions_new (
			id, topic_id, prompt, options_json, correct_answer, explanation, hint, source_heading, source_snippet,
			source_page_start, source_page_end, llm_model, prompt_version, created_at
		)
		SELECT
			id, topic_id, prompt, options_json, correct_answer, explanation, hint, source_heading, source_snippet,
			source_page_start, source_page_end, llm_model, prompt_version, created_at
		FROM questions`,
		`CREATE TABLE user_answers_new (
			id TEXT PRIMARY KEY,
			question_id TEXT NOT NULL,
			user_answer TEXT NOT NULL,
			is_correct INTEGER NOT NULL,
			score INTEGER NOT NULL,
			feedback TEXT,
			hint TEXT,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (question_id) REFERENCES questions(id) ON DELETE CASCADE
		)`,
		`INSERT INTO user_answers_new (id, question_id, user_answer, is_correct, score, feedback, hint, created_at)
		SELECT id, question_id, user_answer, is_correct, score, feedback, hint, created_at
		FROM user_answers`,
		`DROP TABLE user_answers`,
		`DROP TABLE questions`,
		`ALTER TABLE questions_new RENAME TO questions`,
		`ALTER TABLE user_answers_new RENAME TO user_answers`,
	}

	for _, stmt := range stmts {
		if _, err = tx.Exec(stmt); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func rebuildFSRSTablesForCascade() (err error) {
	if _, err = conn.Exec(`PRAGMA foreign_keys = OFF`); err != nil {
		return err
	}
	defer func() {
		if _, pragmaErr := conn.Exec(`PRAGMA foreign_keys = ON`); err == nil && pragmaErr != nil {
			err = pragmaErr
		}
	}()

	tx, err := conn.Begin()
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	stmts := []string{
		`DROP INDEX IF EXISTS idx_fsrs_cards_topic_prompt`,
		`DROP INDEX IF EXISTS idx_fsrs_review_log_activity_ref_reviewed_at`,
		`DROP INDEX IF EXISTS idx_fsrs_review_log_topic_reviewed_at`,
		`DROP INDEX IF EXISTS idx_fsrs_cards_suspended_due_at`,
		`CREATE TABLE fsrs_cards_new (
			id TEXT PRIMARY KEY,
			topic_id TEXT NOT NULL,
			prompt TEXT NOT NULL,
			answer TEXT NOT NULL,
			state_json TEXT,
			due_at INTEGER,
			suspended BOOLEAN DEFAULT 0,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (topic_id) REFERENCES topics(id) ON DELETE CASCADE
		)`,
		`INSERT INTO fsrs_cards_new (id, topic_id, prompt, answer, state_json, due_at, suspended, created_at, updated_at)
		SELECT id, topic_id, prompt, answer, state_json, due_at, suspended, created_at, updated_at
		FROM fsrs_cards`,
		`CREATE TABLE fsrs_review_log_new (
			id TEXT PRIMARY KEY,
			topic_id TEXT NOT NULL,
			activity_type TEXT NOT NULL,
			reference_id TEXT NOT NULL,
			reviewed_at INTEGER NOT NULL,
			rating INTEGER NOT NULL,
			scheduled_days INTEGER NOT NULL,
			state_before_json TEXT NOT NULL,
			state_after_json TEXT NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (topic_id) REFERENCES topics(id) ON DELETE CASCADE
		)`,
		`INSERT INTO fsrs_review_log_new (
			id, topic_id, activity_type, reference_id, reviewed_at, rating,
			scheduled_days, state_before_json, state_after_json, created_at
		)
		SELECT
			id, topic_id, activity_type, reference_id, reviewed_at, rating,
			scheduled_days, state_before_json, state_after_json, created_at
		FROM fsrs_review_log`,
		`DROP TABLE fsrs_review_log`,
		`DROP TABLE fsrs_cards`,
		`ALTER TABLE fsrs_cards_new RENAME TO fsrs_cards`,
		`ALTER TABLE fsrs_review_log_new RENAME TO fsrs_review_log`,
		`CREATE UNIQUE INDEX idx_fsrs_cards_topic_prompt ON fsrs_cards(topic_id, prompt)`,
		`CREATE INDEX idx_fsrs_review_log_activity_ref_reviewed_at ON fsrs_review_log(activity_type, reference_id, reviewed_at DESC)`,
		`CREATE INDEX idx_fsrs_review_log_topic_reviewed_at ON fsrs_review_log(topic_id, reviewed_at DESC)`,
		`CREATE INDEX idx_fsrs_cards_suspended_due_at ON fsrs_cards(suspended, due_at)`,
	}

	for _, stmt := range stmts {
		if _, err = tx.Exec(stmt); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func rebuildWrittenQuestionsForCascade() (err error) {
	if _, err = conn.Exec(`PRAGMA foreign_keys = OFF`); err != nil {
		return err
	}
	defer func() {
		if _, pragmaErr := conn.Exec(`PRAGMA foreign_keys = ON`); err == nil && pragmaErr != nil {
			err = pragmaErr
		}
	}()

	tx, err := conn.Begin()
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	stmts := []string{
		`DROP INDEX IF EXISTS idx_written_questions_topic_created_at`,
		`CREATE TABLE written_questions_new (
			id TEXT PRIMARY KEY,
			topic_id TEXT NOT NULL,
			prompt TEXT NOT NULL,
			source_heading TEXT,
			source_page_start INTEGER DEFAULT 0,
			source_page_end INTEGER DEFAULT 0,
			llm_model TEXT,
			prompt_version TEXT,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (topic_id) REFERENCES topics(id) ON DELETE CASCADE
		)`,
		`INSERT INTO written_questions_new (
			id, topic_id, prompt, source_heading, source_page_start, source_page_end,
			llm_model, prompt_version, created_at, updated_at
		)
		SELECT
			id, topic_id, prompt, source_heading, source_page_start, source_page_end,
			llm_model, prompt_version, created_at, COALESCE(updated_at, created_at)
		FROM written_questions`,
		`DROP TABLE written_questions`,
		`ALTER TABLE written_questions_new RENAME TO written_questions`,
		`CREATE INDEX idx_written_questions_topic_created_at ON written_questions(topic_id, created_at DESC)`,
	}

	for _, stmt := range stmts {
		if _, err = tx.Exec(stmt); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func rebuildAssessmentFSRSForCascade() (err error) {
	if _, err = conn.Exec(`PRAGMA foreign_keys = OFF`); err != nil {
		return err
	}
	defer func() {
		if _, pragmaErr := conn.Exec(`PRAGMA foreign_keys = ON`); err == nil && pragmaErr != nil {
			err = pragmaErr
		}
	}()

	tx, err := conn.Begin()
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	stmts := []string{
		`DROP INDEX IF EXISTS idx_assessment_fsrs_topic_due_at`,
		`CREATE TABLE assessment_fsrs_new (
			activity_type TEXT NOT NULL,
			reference_id TEXT NOT NULL,
			topic_id TEXT NOT NULL,
			state_json TEXT NOT NULL,
			due_at INTEGER,
			last_reviewed_at INTEGER,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (activity_type, reference_id),
			FOREIGN KEY (topic_id) REFERENCES topics(id) ON DELETE CASCADE
		)`,
		`INSERT INTO assessment_fsrs_new (
			activity_type, reference_id, topic_id, state_json, due_at, last_reviewed_at, created_at, updated_at
		)
		SELECT
			activity_type, reference_id, topic_id, state_json, due_at, last_reviewed_at, created_at, updated_at
		FROM assessment_fsrs`,
		`DROP TABLE assessment_fsrs`,
		`ALTER TABLE assessment_fsrs_new RENAME TO assessment_fsrs`,
		`CREATE INDEX idx_assessment_fsrs_topic_due_at ON assessment_fsrs(topic_id, due_at)`,
	}

	for _, stmt := range stmts {
		if _, err = tx.Exec(stmt); err != nil {
			return err
		}
	}

	return tx.Commit()
}

// GetTopicContent retrieves all parent sections for a topic
func GetTopicContent(topicID string) (map[string]interface{}, error) {
	var title string
	err := conn.QueryRow("SELECT title FROM topics WHERE id = ?", topicID).Scan(&title)
	if err != nil {
		return nil, err
	}

	rows, err := conn.Query(`
		SELECT id, heading, content_text, order_index
		FROM parents
		WHERE topic_id = ?
		ORDER BY order_index
	`, topicID)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = rows.Close()
	}()

	var sections []map[string]interface{}
	for rows.Next() {
		var id, heading, content string
		var order int
		if err := rows.Scan(&id, &heading, &content, &order); err != nil {
			return nil, err
		}
		sections = append(sections, map[string]interface{}{
			"id":      id,
			"heading": heading,
			"content": content,
			"order":   order,
		})
	}

	return map[string]interface{}{
		"title":    title,
		"sections": sections,
	}, nil
}

// GetChunksForTopic retrieves all chunks for a topic.
func GetChunksForTopic(topicID string) ([]models.Chunk, error) {
	rows, err := conn.Query(`
		SELECT id, topic_id, parent_id, chunk_text, importance_score, weakness_score
		FROM chunks
		WHERE topic_id = ?
	`, topicID)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = rows.Close()
	}()

	var chunks []models.Chunk
	for rows.Next() {
		var chunk models.Chunk
		if err := rows.Scan(
			&chunk.ID,
			&chunk.TopicID,
			&chunk.ParentID,
			&chunk.Text,
			&chunk.ImportanceScore,
			&chunk.WeaknessScore,
		); err != nil {
			return nil, err
		}
		chunks = append(chunks, chunk)
	}

	return chunks, nil
}

// GetChunksForTopics retrieves chunks for multiple topics in a single batch query
func GetChunksForTopics(topicIDs []string) (map[string][]models.Chunk, error) {
	if len(topicIDs) == 0 {
		return make(map[string][]models.Chunk), nil
	}

	// Build IN clause placeholders
	placeholders := make([]string, len(topicIDs))
	args := make([]interface{}, len(topicIDs))
	for i, topicID := range topicIDs {
		placeholders[i] = "?"
		args[i] = topicID
	}

	query := fmt.Sprintf(`
		SELECT id, topic_id, parent_id, chunk_text, importance_score, weakness_score
		FROM chunks
		WHERE topic_id IN (%s)
	`, strings.Join(placeholders, ","))

	rows, err := conn.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = rows.Close()
	}()

	// Group chunks by topic_id
	result := make(map[string][]models.Chunk)
	for rows.Next() {
		var chunk models.Chunk
		if err := rows.Scan(
			&chunk.ID,
			&chunk.TopicID,
			&chunk.ParentID,
			&chunk.Text,
			&chunk.ImportanceScore,
			&chunk.WeaknessScore,
		); err != nil {
			return nil, err
		}
		result[chunk.TopicID] = append(result[chunk.TopicID], chunk)
	}

	return result, nil
}

// GetParentSection retrieves a parent section by ID
func GetParentSection(parentID string) (map[string]string, error) {
	var id, heading, content string
	err := conn.QueryRow(`
		SELECT id, heading, content_text
		FROM parents
		WHERE id = ?
	`, parentID).Scan(&id, &heading, &content)
	if err != nil {
		return nil, err
	}

	return map[string]string{
		"id":      id,
		"heading": heading,
		"content": content,
	}, nil
}

// QueryDueReviewCards counts cards due by the given time, scoped to existing topics
func QueryDueReviewCards(now int64) (int, error) {
	var count int
	err := conn.QueryRow(`
		SELECT COUNT(*)
		FROM fsrs_cards fc
		JOIN topics t ON t.id = fc.topic_id
		WHERE fc.suspended = 0
		  AND fc.due_at IS NOT NULL
		  AND fc.due_at <= ?
	`, now).Scan(&count)
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

// QueryNextReadingTopic returns the next reading topic with deterministic page bounds and cursor.
func QueryNextReadingTopic() (models.ReadingTopicCursor, bool, error) {
	var topic models.ReadingTopicCursor
	err := conn.QueryRow(`
		SELECT
			id,
			title,
			COALESCE(start_page, 0),
			COALESCE(end_page, 0),
			COALESCE(current_page_cursor, 0)
		FROM topics
		WHERE status IN ('unseen', 'reading')
		  AND COALESCE(end_page, 0) > 0
		  AND COALESCE(current_page_cursor, 0) < COALESCE(end_page, 0)
		ORDER BY updated_at ASC, created_at ASC
		LIMIT 1
	`).Scan(&topic.ID, &topic.Title, &topic.StartPage, &topic.EndPage, &topic.CurrentPageCursor)
	if err == sql.ErrNoRows {
		return models.ReadingTopicCursor{}, false, nil
	}
	if err != nil {
		return models.ReadingTopicCursor{}, false, err
	}
	return topic, true, nil
}

// QueryActiveTopics returns top N active topic titles
func QueryActiveTopics(limit int) ([]string, error) {
	rows, err := conn.Query(`
		SELECT title
		FROM topics
		WHERE status IN ('reading', 'learned')
		ORDER BY updated_at DESC, created_at DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = rows.Close()
	}()

	var active []string
	for rows.Next() {
		var title string
		if err := rows.Scan(&title); err != nil {
			return nil, err
		}
		active = append(active, title)
	}
	return active, nil
}

// QueryLearningTopics returns topics ready for learning
func QueryLearningTopics(limit int) ([]models.TopicSummary, error) {
	rows, err := conn.Query(`
		SELECT id, title, status
		FROM topics
		WHERE status IN ('unseen', 'reading')
		ORDER BY updated_at ASC, created_at ASC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = rows.Close()
	}()

	var topics []models.TopicSummary
	for rows.Next() {
		var topic models.TopicSummary
		if err := rows.Scan(&topic.ID, &topic.Title, &topic.Status); err != nil {
			return nil, err
		}
		topics = append(topics, topic)
	}
	return topics, nil
}

// QueryUpcomingReadingTopics returns ordered unread/in-progress topics with configured bounds.
func QueryUpcomingReadingTopics(limit int) ([]models.ReadingTopicCursor, error) {
	if limit <= 0 {
		return []models.ReadingTopicCursor{}, nil
	}

	rows, err := conn.Query(`
		SELECT
			id,
			title,
			COALESCE(start_page, 0),
			COALESCE(end_page, 0),
			COALESCE(current_page_cursor, 0)
		FROM topics
		WHERE status IN ('unseen', 'reading')
		  AND COALESCE(end_page, 0) > 0
		  AND COALESCE(current_page_cursor, 0) < COALESCE(end_page, 0)
		ORDER BY updated_at ASC, created_at ASC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = rows.Close()
	}()

	topics := make([]models.ReadingTopicCursor, 0, limit)
	for rows.Next() {
		var topic models.ReadingTopicCursor
		if err := rows.Scan(&topic.ID, &topic.Title, &topic.StartPage, &topic.EndPage, &topic.CurrentPageCursor); err != nil {
			return nil, err
		}
		topics = append(topics, topic)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return topics, nil
}

// CountLearnedTopics returns the count of fully learned topics
func CountLearnedTopics() (int, error) {
	var count int
	err := conn.QueryRow(`
		SELECT COUNT(*)
		FROM topics
		WHERE status = 'learned'
	`).Scan(&count)
	return count, err
}

// CreateFlashcards stores a new set of flashcards for one topic.
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

// GetFlashcardsForTopic returns topic-scoped flashcards, optionally only those due now.
func GetFlashcardsForTopic(topicID string, dueOnly bool, now int64) ([]models.Flashcard, error) {
	topicID = strings.TrimSpace(topicID)
	if topicID == "" {
		return nil, fmt.Errorf("topic id is required")
	}
	if dueOnly && now <= 0 {
		return nil, fmt.Errorf("current time is required when filtering due flashcards")
	}
	return getFlashcardsForTopicRepo(topicID, dueOnly, now)
}

// GetFlashcardByID returns one flashcard and its scheduler state.
func GetFlashcardByID(cardID string) (*models.Flashcard, *models.FlashcardState, error) {
	cardID = strings.TrimSpace(cardID)
	if cardID == "" {
		return nil, nil, fmt.Errorf("flashcard id is required")
	}
	return getFlashcardByIDRepo(cardID)
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
func UpdateFlashcardReview(cardID string, dueAt int64, expectedDueAt int64, state models.FlashcardState, reviewLog models.FSRSReviewLog) error {
	cardID = strings.TrimSpace(cardID)
	if cardID == "" {
		return fmt.Errorf("flashcard id is required")
	}
	if dueAt <= 0 {
		return fmt.Errorf("due time is required")
	}
	return updateFlashcardReviewRepo(cardID, dueAt, expectedDueAt, state, reviewLog)
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
		normalizedCards = append(normalizedCards, card)
	}

	return normalizedCards, nil
}

// CreateNotebook saves a notebook record to the database
func CreateNotebook(id, title, filePath, fileType, topicID string, pageCount int) error {
	var topicValue interface{}
	if topicID != "" {
		topicValue = topicID
	}

	_, err := conn.Exec(`
		INSERT INTO notebooks (id, title, file_path, file_type, topic_id, status, page_count)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, id, title, filePath, fileType, topicValue, "uploaded", pageCount)
	return err
}

// NotebookParentInput is a parent section row used by notebook ingestion transactions.
type NotebookParentInput struct {
	ID         string
	Heading    string
	Content    string
	OrderIndex int
}

// NotebookChunkInput is a chunk row used by notebook ingestion transactions.
type NotebookChunkInput struct {
	ID         string
	ParentID   string
	Text       string
	TokenCount int
	PageNum    int
}

// NotebookTopicIngestionGroup contains topic-scoped parent/chunk rows for one notebook ingestion run.
type NotebookTopicIngestionGroup struct {
	TopicID string
	Parents []NotebookParentInput
	Chunks  []NotebookChunkInput
}

type sqlExecer interface {
	Exec(query string, args ...interface{}) (sql.Result, error)
}

func insertParentRow(exec sqlExecer, topicID string, parent NotebookParentInput) error {
	_, err := exec.Exec(`
		INSERT INTO parents (id, topic_id, heading, order_index, content_text)
		VALUES (?, ?, ?, ?, ?)
	`, parent.ID, topicID, parent.Heading, parent.OrderIndex, parent.Content)
	return err
}

func insertChunkRow(exec sqlExecer, topicID string, chunk NotebookChunkInput) error {
	_, err := exec.Exec(`
		INSERT INTO chunks (id, topic_id, parent_id, chunk_text, page_num, token_count, importance_score, weakness_score)
		VALUES (?, ?, ?, ?, ?, ?, 0, 0)
	`, chunk.ID, topicID, chunk.ParentID, chunk.Text, chunk.PageNum, chunk.TokenCount)
	return err
}

// CreateParentSection inserts a parent section row.
func CreateParentSection(id, topicID, heading string, orderIndex int, content string) error {
	return insertParentRow(conn, topicID, NotebookParentInput{
		ID:         id,
		Heading:    heading,
		Content:    content,
		OrderIndex: orderIndex,
	})
}

// CreateChunk inserts a chunk row.
func CreateChunk(id, topicID, parentID, text string, tokenCount int, pageNum int) error {
	return insertChunkRow(conn, topicID, NotebookChunkInput{
		ID:         id,
		ParentID:   parentID,
		Text:       text,
		PageNum:    pageNum,
		TokenCount: tokenCount,
	})
}

// UpdateNotebookStatus updates the notebook ingestion status.
func UpdateNotebookStatus(notebookID string, status string) error {
	_, err := conn.Exec(`
		UPDATE notebooks
		SET status = ?
		WHERE id = ?
	`, status, notebookID)
	return err
}

// UpdateNotebookTopic updates the notebook topic link used by UI-level notebook metadata.
func UpdateNotebookTopic(notebookID string, topicID string) error {
	if strings.TrimSpace(topicID) == "" {
		_, err := conn.Exec(`
			UPDATE notebooks
			SET topic_id = NULL
			WHERE id = ?
		`, notebookID)
		return err
	}

	_, err := conn.Exec(`
		UPDATE notebooks
		SET topic_id = ?
		WHERE id = ?
	`, topicID, notebookID)
	return err
}

// UpdateNotebookTitle updates notebook display title.
func UpdateNotebookTitle(notebookID string, title string) error {
	notebookID = strings.TrimSpace(notebookID)
	title = strings.TrimSpace(title)
	if notebookID == "" {
		return fmt.Errorf("notebook id is required")
	}
	if title == "" {
		return fmt.Errorf("title is required")
	}

	result, err := conn.Exec(`
		UPDATE notebooks
		SET title = ?
		WHERE id = ?
	`, title, notebookID)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return sql.ErrNoRows
	}

	return nil
}

// EnsureTopic inserts a topic if it does not already exist.
func EnsureTopic(topicID, title string) error {
	if topicID == "" {
		return fmt.Errorf("topic id is required")
	}
	if title == "" {
		title = topicID
	}

	_, err := conn.Exec(`
		INSERT INTO topics (id, title, status)
		VALUES (?, ?, 'reading')
		ON CONFLICT(id) DO UPDATE SET title = excluded.title
	`, topicID, title)
	return err
}

// TopicBatchItem represents a topic to be created/updated in batch
type TopicBatchItem struct {
	TopicID string
	Title   string
}

// EnsureTopicsBatch creates or updates multiple topics in a single transaction
func EnsureTopicsBatch(items []TopicBatchItem) error {
	if len(items) == 0 {
		return nil
	}

	tx, err := conn.Begin()
	if err != nil {
		return err
	}
	defer func() {
		_ = tx.Rollback()
	}()

	stmt, err := tx.Prepare(`
		INSERT INTO topics (id, title, status)
		VALUES (?, ?, 'reading')
		ON CONFLICT(id) DO UPDATE SET title = excluded.title, status = 'reading'
	`)
	if err != nil {
		return err
	}
	defer func() {
		_ = stmt.Close()
	}()

	for _, item := range items {
		if item.TopicID == "" {
			err = fmt.Errorf("topic id is required for all batch items")
			return err
		}
		title := item.Title
		if title == "" {
			title = item.TopicID
		}

		_, err = stmt.Exec(item.TopicID, title)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

// UpdateTopicPageBounds stores deterministic chapter bounds for a topic.
// Initializes current_page_cursor to startPage if it is 0 (uninitialized).
func UpdateTopicPageBounds(topicID string, startPage, endPage int) error {
	topicID = strings.TrimSpace(topicID)
	if topicID == "" {
		return fmt.Errorf("topic id is required")
	}
	if startPage < 0 {
		startPage = 0
	}
	if endPage < 0 {
		endPage = 0
	}
	if startPage > endPage {
		startPage, endPage = endPage, startPage
	}

	tx, err := conn.Begin()
	if err != nil {
		return err
	}
	defer func() {
		_ = tx.Rollback()
	}()

	// Determine if cursor needs initialization and detect shrinkage
	var previousStart int
	var previousEnd int
	var currentCursor int
	if err := tx.QueryRow(`
		SELECT COALESCE(start_page, 0), COALESCE(end_page, 0), COALESCE(current_page_cursor, 0)
		FROM topics WHERE id = ?
	`, topicID).Scan(&previousStart, &previousEnd, &currentCursor); err != nil && err != sql.ErrNoRows {
		return err
	}

	// Check if bounds shrunk (start moved forward OR end moved backward)
	shrunk := (previousStart > 0 && startPage > 0 && startPage > previousStart) ||
		(previousEnd > 0 && endPage > 0 && endPage < previousEnd)

	// Initialize cursor to startPage if uninitialized (0), otherwise clamp to new bounds
	var newCursor int
	if currentCursor == 0 {
		newCursor = startPage
	} else {
		// Clamp cursor to new bounds
		if currentCursor < startPage {
			newCursor = startPage
		} else if currentCursor > endPage {
			newCursor = endPage
		} else {
			newCursor = currentCursor
		}
	}

	// Update bounds and cursor
	result, err := tx.Exec(`
		UPDATE topics
		SET start_page = ?, end_page = ?, current_page_cursor = ?
		WHERE id = ?
	`, startPage, endPage, newCursor, topicID)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return sql.ErrNoRows
	}

	if shrunk {
		if err := deleteAssessmentDataOutsideBoundsTx(tx, topicID, startPage, endPage); err != nil {
			return err
		}
	}

	return tx.Commit()
}

// TopicPageBoundsBatchItem represents topic page bounds to be updated in batch
type TopicPageBoundsBatchItem struct {
	TopicID   string
	StartPage int
	EndPage   int
}

// UpdateTopicPageBoundsBatch updates page bounds for multiple topics in a single transaction
func UpdateTopicPageBoundsBatch(items []TopicPageBoundsBatchItem) error {
	if len(items) == 0 {
		return nil
	}

	tx, err := conn.Begin()
	if err != nil {
		return err
	}
	defer func() {
		_ = tx.Rollback()
	}()

	for _, item := range items {
		topicID := strings.TrimSpace(item.TopicID)
		if topicID == "" {
			err = fmt.Errorf("topic id is required for all batch items")
			return err
		}

		startPage := item.StartPage
		endPage := item.EndPage
		if startPage < 0 {
			startPage = 0
		}
		if endPage < 0 {
			endPage = 0
		}
		if startPage > endPage {
			startPage, endPage = endPage, startPage
		}

		// Check current cursor and detect shrinkage
		var previousStart int
		var previousEnd int
		var currentCursor int
		if cursorErr := tx.QueryRow(`
			SELECT COALESCE(start_page, 0), COALESCE(end_page, 0), COALESCE(current_page_cursor, 0)
			FROM topics WHERE id = ?
		`, topicID).Scan(&previousStart, &previousEnd, &currentCursor); cursorErr != nil && cursorErr != sql.ErrNoRows {
			return cursorErr
		}

		// Check if bounds shrunk (start moved forward OR end moved backward)
		shrunk := (previousStart > 0 && startPage > 0 && startPage > previousStart) ||
			(previousEnd > 0 && endPage > 0 && endPage < previousEnd)

		// Initialize cursor to startPage if uninitialized (0), otherwise clamp to new bounds
		var newCursor int
		if currentCursor == 0 {
			newCursor = startPage
		} else {
			// Clamp cursor to new bounds
			if currentCursor < startPage {
				newCursor = startPage
			} else if currentCursor > endPage {
				newCursor = endPage
			} else {
				newCursor = currentCursor
			}
		}

		// Update bounds and cursor
		res, err := tx.Exec(`
			UPDATE topics
			SET start_page = ?, end_page = ?, current_page_cursor = ?
			WHERE id = ?
		`, startPage, endPage, newCursor, topicID)
		if err != nil {
			return err
		}
		rowsAffected, err := res.RowsAffected()
		if err != nil {
			return err
		}
		if rowsAffected == 0 {
			return fmt.Errorf("no rows updated for topicID %s", topicID)
		}

		if shrunk {
			if err := deleteAssessmentDataOutsideBoundsTx(tx, topicID, startPage, endPage); err != nil {
				return err
			}
		}
	}

	return tx.Commit()
}

func deleteAssessmentDataOutsideBoundsTx(tx *sql.Tx, topicID string, startPage int, endPage int) error {
	if _, err := tx.Exec(`
		DELETE FROM user_answers
		WHERE question_id IN (
			SELECT id
			FROM questions
			WHERE topic_id = ?
			  AND (COALESCE(source_page_start, 0) < ? OR COALESCE(source_page_end, 0) > ?)
		)
	`, topicID, startPage, endPage); err != nil {
		return fmt.Errorf("delete out-of-range user answers: %w", err)
	}

	if _, err := tx.Exec(`
		DELETE FROM fsrs_review_log
		WHERE activity_type = 'quiz_question'
		  AND reference_id IN (
			SELECT id
			FROM questions
			WHERE topic_id = ?
			  AND (COALESCE(source_page_start, 0) < ? OR COALESCE(source_page_end, 0) > ?)
		)
	`, topicID, startPage, endPage); err != nil {
		return fmt.Errorf("delete out-of-range quiz review logs: %w", err)
	}

	if _, err := tx.Exec(`
		DELETE FROM assessment_fsrs
		WHERE activity_type = 'quiz_question'
		  AND reference_id IN (
			SELECT id
			FROM questions
			WHERE topic_id = ?
			  AND (COALESCE(source_page_start, 0) < ? OR COALESCE(source_page_end, 0) > ?)
		)
	`, topicID, startPage, endPage); err != nil {
		return fmt.Errorf("delete out-of-range quiz fsrs state: %w", err)
	}

	if _, err := tx.Exec(`
		DELETE FROM questions
		WHERE topic_id = ?
		  AND (COALESCE(source_page_start, 0) < ? OR COALESCE(source_page_end, 0) > ?)
	`, topicID, startPage, endPage); err != nil {
		return fmt.Errorf("delete out-of-range questions: %w", err)
	}

	if _, err := tx.Exec(`
		DELETE FROM fsrs_review_log
		WHERE activity_type = 'written_question'
		  AND reference_id IN (
			SELECT id
			FROM written_questions
			WHERE topic_id = ?
			  AND (COALESCE(source_page_start, 0) < ? OR COALESCE(source_page_end, 0) > ?)
		)
	`, topicID, startPage, endPage); err != nil {
		return fmt.Errorf("delete out-of-range written review logs: %w", err)
	}

	if _, err := tx.Exec(`
		DELETE FROM assessment_fsrs
		WHERE activity_type = 'written_question'
		  AND reference_id IN (
			SELECT id
			FROM written_questions
			WHERE topic_id = ?
			  AND (COALESCE(source_page_start, 0) < ? OR COALESCE(source_page_end, 0) > ?)
		)
	`, topicID, startPage, endPage); err != nil {
		return fmt.Errorf("delete out-of-range written fsrs state: %w", err)
	}

	if _, err := tx.Exec(`
		DELETE FROM written_questions
		WHERE topic_id = ?
		  AND (COALESCE(source_page_start, 0) < ? OR COALESCE(source_page_end, 0) > ?)
	`, topicID, startPage, endPage); err != nil {
		return fmt.Errorf("delete out-of-range written questions: %w", err)
	}

	return nil
}

// GetTopicPageBounds returns persisted chapter bounds for a topic.
func GetTopicPageBounds(topicID string) (int, int, error) {
	topicID = strings.TrimSpace(topicID)
	if topicID == "" {
		return 0, 0, fmt.Errorf("topic id is required")
	}

	var startPage int
	var endPage int
	err := conn.QueryRow(`
		SELECT COALESCE(start_page, 0), COALESCE(end_page, 0)
		FROM topics
		WHERE id = ?
	`, topicID).Scan(&startPage, &endPage)
	if err != nil {
		return 0, 0, err
	}

	return startPage, endPage, nil
}

// GetTopicCurrentPageCursor returns the current page cursor for a topic.
func GetTopicCurrentPageCursor(topicID string) (int, error) {
	topicID = strings.TrimSpace(topicID)
	if topicID == "" {
		return 0, fmt.Errorf("topic id is required")
	}

	var cursor int
	err := conn.QueryRow(`
		SELECT COALESCE(current_page_cursor, 0)
		FROM topics
		WHERE id = ?
	`, topicID).Scan(&cursor)
	if err != nil {
		return 0, err
	}

	return cursor, nil
}

// GetTotalChunkTokens returns estimated total tokens for one topic.
// It prefers stored token_count values and falls back to len(chunk_text)/4 when token_count is zero or missing.
func GetTotalChunkTokens(topicID string) (int, error) {
	return getTotalChunkTokens(topicID, 0, 0)
}

// GetTotalChunkTokensForPageRange returns estimated total tokens for one topic/page window.
// It prefers stored token_count values and falls back to len(chunk_text)/4 when token_count is zero or missing.
func GetTotalChunkTokensForPageRange(topicID string, startPage int, endPage int) (int, error) {
	return getTotalChunkTokens(topicID, startPage, endPage)
}

func getTotalChunkTokens(topicID string, startPage int, endPage int) (int, error) {
	topicID = strings.TrimSpace(topicID)
	if topicID == "" {
		return 0, fmt.Errorf("topic id is required")
	}

	// Validate page bounds
	var filterByPage bool
	if startPage == 0 && endPage == 0 {
		// No page filter - entire topic
		filterByPage = false
	} else if startPage <= 0 || endPage <= 0 {
		// Mixed positive/negative bounds are invalid
		return 0, fmt.Errorf("invalid page bounds: both startPage and endPage must be positive or both must be zero")
	} else {
		// Both bounds are positive - filter by page range
		filterByPage = true
		if startPage > endPage {
			startPage, endPage = endPage, startPage
		}
	}

	query := `
		SELECT COALESCE(token_count, 0), COALESCE(chunk_text, '')
		FROM chunks
		WHERE topic_id = ?
	`
	args := []interface{}{topicID}
	if filterByPage {
		query += ` AND page_num BETWEEN ? AND ?`
		args = append(args, startPage, endPage)
	}

	rows, err := conn.Query(query, args...)
	if err != nil {
		return 0, err
	}
	defer func() {
		_ = rows.Close()
	}()

	total := 0
	for rows.Next() {
		var tokenCount int
		var chunkText string
		if err := rows.Scan(&tokenCount, &chunkText); err != nil {
			return 0, err
		}

		if tokenCount > 0 {
			total += tokenCount
			continue
		}

		fallback := len(chunkText) / 4
		if fallback <= 0 && strings.TrimSpace(chunkText) != "" {
			fallback = 1
		}
		total += fallback
	}

	if err := rows.Err(); err != nil {
		return 0, err
	}

	return total, nil
}

// GetChunkTextsForTopicPageRange returns chunk_text rows ordered by chunk id for one topic/page window.
func GetChunkTextsForTopicPageRange(topicID string, startPage int, endPage int) ([]string, error) {
	topicID = strings.TrimSpace(topicID)
	if topicID == "" {
		return nil, fmt.Errorf("topic id is required")
	}
	if startPage <= 0 || endPage <= 0 {
		return nil, fmt.Errorf("start page and end page must be positive")
	}
	if startPage > endPage {
		startPage, endPage = endPage, startPage
	}

	rows, err := conn.Query(`
		SELECT chunk_text
		FROM chunks
		WHERE topic_id = ?
		  AND page_num BETWEEN ? AND ?
		ORDER BY page_num ASC, id ASC
	`, topicID, startPage, endPage)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = rows.Close()
	}()

	var chunkTexts []string
	for rows.Next() {
		var chunkText string
		if err := rows.Scan(&chunkText); err != nil {
			return nil, err
		}
		chunkTexts = append(chunkTexts, chunkText)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return chunkTexts, nil
}

// GetParentPassagesForTopicPageRange retrieves chunks with their parent passage context for a topic page range
func GetParentPassagesForTopicPageRange(topicID string, startPage int, endPage int) ([]string, error) {
	topicID = strings.TrimSpace(topicID)
	if topicID == "" {
		return nil, fmt.Errorf("topic id is required")
	}
	if startPage <= 0 || endPage <= 0 {
		return nil, fmt.Errorf("start page and end page must be positive")
	}
	if startPage > endPage {
		startPage, endPage = endPage, startPage
	}

	rows, err := conn.Query(`
		SELECT c.chunk_text, COALESCE(p.heading, ''), COALESCE(p.content_text, '')
		FROM chunks c
		LEFT JOIN parents p ON c.parent_id = p.id AND p.topic_id = c.topic_id
		WHERE c.topic_id = ?
		  AND c.page_num BETWEEN ? AND ?
		ORDER BY c.page_num ASC, c.id ASC
	`, topicID, startPage, endPage)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = rows.Close()
	}()

	var parentPassages []string
	for rows.Next() {
		var chunkText, parentHeading, parentContent string
		if err := rows.Scan(&chunkText, &parentHeading, &parentContent); err != nil {
			return nil, err
		}

		// Build parent passage context
		var passage strings.Builder
		if parentHeading != "" {
			passage.WriteString("Section: ")
			passage.WriteString(parentHeading)
			passage.WriteString("\n")
		}
		if parentContent != "" {
			passage.WriteString("Context: ")
			passage.WriteString(parentContent)
			passage.WriteString("\n")
		}
		passage.WriteString("Content: ")
		passage.WriteString(chunkText)

		parentPassages = append(parentPassages, passage.String())
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return parentPassages, nil
}

// GetTopicHeadingPageRanges returns resolved page bounds per heading for a topic.
// Key format is normalized lower-case heading text with single spaces.
func GetTopicHeadingPageRanges(topicID string) (map[string][2]int, error) {
	if conn == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	topicID = strings.TrimSpace(topicID)
	if topicID == "" {
		return nil, fmt.Errorf("topic id is required")
	}

	rows, err := conn.Query(`
		SELECT
			COALESCE(NULLIF(TRIM(p.heading), ''), ''),
			COALESCE(MIN(NULLIF(c.page_num, 0)), 0) AS start_page,
			COALESCE(MAX(NULLIF(c.page_num, 0)), 0) AS end_page
		FROM parents p
		LEFT JOIN chunks c ON c.parent_id = p.id AND c.topic_id = p.topic_id
		WHERE p.topic_id = ?
		GROUP BY p.id, p.heading
	`, topicID)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = rows.Close()
	}()

	ranges := make(map[string][2]int)
	for rows.Next() {
		var heading string
		var startPage int
		var endPage int
		if err := rows.Scan(&heading, &startPage, &endPage); err != nil {
			return nil, err
		}

		key := strings.ToLower(strings.Join(strings.Fields(strings.TrimSpace(heading)), " "))
		if key == "" {
			continue
		}

		if startPage > 0 && endPage <= 0 {
			endPage = startPage
		}
		if endPage > 0 && startPage <= 0 {
			startPage = endPage
		}
		if startPage <= 0 || endPage <= 0 {
			continue
		}
		if startPage > endPage {
			startPage, endPage = endPage, startPage
		}

		existing, ok := ranges[key]
		if !ok {
			ranges[key] = [2]int{startPage, endPage}
			continue
		}

		mergedStart := existing[0]
		mergedEnd := existing[1]
		if startPage < mergedStart {
			mergedStart = startPage
		}
		if endPage > mergedEnd {
			mergedEnd = endPage
		}
		ranges[key] = [2]int{mergedStart, mergedEnd}
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return ranges, nil
}

// IngestNotebookContent performs a transactional relational commit for notebook sections/chunks.
func IngestNotebookContent(notebookID string, topicID string, parents []NotebookParentInput, chunks []NotebookChunkInput) error {
	notebookID = strings.TrimSpace(notebookID)
	topicID = strings.TrimSpace(topicID)
	if notebookID == "" {
		return fmt.Errorf("notebook id is required")
	}
	if topicID == "" {
		return fmt.Errorf("topic id is required")
	}

	group := NotebookTopicIngestionGroup{
		TopicID: topicID,
		Parents: parents,
		Chunks:  chunks,
	}

	// Route legacy single-topic ingestion through the multi-topic transaction path.
	return IngestNotebookContentByTopic(notebookID, []NotebookTopicIngestionGroup{group})
}

// IngestNotebookContentByTopic ingests notebook content into multiple topic buckets in one transaction.
func IngestNotebookContentByTopic(notebookID string, groups []NotebookTopicIngestionGroup) error {
	notebookID = strings.TrimSpace(notebookID)
	if notebookID == "" {
		return fmt.Errorf("notebook id is required")
	}
	if len(groups) == 0 {
		return fmt.Errorf("at least one ingestion group is required")
	}
	normalizedGroups := make([]NotebookTopicIngestionGroup, 0, len(groups))
	for _, group := range groups {
		group.TopicID = strings.TrimSpace(group.TopicID)
		if group.TopicID == "" {
			return fmt.Errorf("topic id is required for every ingestion group")
		}
		normalizedGroups = append(normalizedGroups, group)
	}
	return ingestNotebookContentByTopicRepo(notebookID, normalizedGroups)
}

// GetNotebooks retrieves all notebooks, optionally filtered by topic
func GetNotebooks(topicID string) ([]models.Notebook, error) {
	query := "SELECT id, title, file_path, file_type, COALESCE(topic_id, ''), COALESCE(status, 'uploaded'), page_count, chunk_count, uploaded_at FROM notebooks"
	args := []interface{}{}

	if topicID != "" {
		query += " WHERE topic_id = ?"
		args = append(args, topicID)
	}
	query += " ORDER BY uploaded_at DESC"

	rows, err := conn.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = rows.Close()
	}()

	var notebooks []models.Notebook
	for rows.Next() {
		var nb models.Notebook
		if err := rows.Scan(&nb.ID, &nb.Title, &nb.FilePath, &nb.FileType, &nb.TopicID, &nb.Status, &nb.PageCount, &nb.ChunkCount, &nb.UploadedAt); err != nil {
			return nil, err
		}
		notebooks = append(notebooks, nb)
	}
	return notebooks, nil
}

// GetNotebookByID retrieves a single notebook by ID
func GetNotebookByID(notebookID string) (*models.Notebook, error) {
	var nb models.Notebook
	err := conn.QueryRow(`
		SELECT id, title, file_path, file_type, COALESCE(topic_id, ''), COALESCE(status, 'uploaded'), page_count, chunk_count, uploaded_at
		FROM notebooks
		WHERE id = ?
	`, notebookID).Scan(&nb.ID, &nb.Title, &nb.FilePath, &nb.FileType, &nb.TopicID, &nb.Status, &nb.PageCount, &nb.ChunkCount, &nb.UploadedAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &nb, nil
}

// LinkChunksToNotebook associates chunks with a notebook
func LinkChunksToNotebook(notebookID string, chunkIDs []string) error {
	for _, chunkID := range chunkIDs {
		id := "nb-chunk-" + notebookID + "-" + chunkID // simple composite ID
		_, err := conn.Exec(`
			INSERT INTO notebook_chunks (id, notebook_id, chunk_id)
			VALUES (?, ?, ?)
		`, id, notebookID, chunkID)
		if err != nil {
			return err
		}
	}
	return nil
}

// UpdateNotebookChunkCount updates the chunk count for a notebook
func UpdateNotebookChunkCount(notebookID string, count int) error {
	_, err := conn.Exec(`
		UPDATE notebooks
		SET chunk_count = ?
		WHERE id = ?
	`, count, notebookID)
	return err
}

// DeleteNotebook removes a notebook and its chunk links
func DeleteNotebook(notebookID string) error {
	notebookID = strings.TrimSpace(notebookID)
	if notebookID == "" {
		return fmt.Errorf("notebook id is required")
	}
	return deleteNotebookRepo(notebookID)
}

// DeleteTopic removes a topic and all associated data
func DeleteTopic(topicID string) error {
	topicID = strings.TrimSpace(topicID)
	if topicID == "" {
		return fmt.Errorf("topic id is required")
	}

	// Begin transaction for atomic deletion
	tx, err := conn.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	// Delete dependent tables in order to respect foreign key constraints

	// Delete user_answers (via questions)
	if _, err = tx.Exec("DELETE FROM user_answers WHERE question_id IN (SELECT id FROM questions WHERE topic_id = ?)", topicID); err != nil {
		return fmt.Errorf("failed to delete user_answers: %w", err)
	}

	// Delete notebook_chunks (via chunks)
	if _, err = tx.Exec("DELETE FROM notebook_chunks WHERE chunk_id IN (SELECT id FROM chunks WHERE topic_id = ?)", topicID); err != nil {
		return fmt.Errorf("failed to delete notebook_chunks: %w", err)
	}

	// Delete fsrs_review_log
	if _, err = tx.Exec("DELETE FROM fsrs_review_log WHERE topic_id = ?", topicID); err != nil {
		return fmt.Errorf("failed to delete fsrs_review_log: %w", err)
	}

	// Delete fsrs_cards
	if _, err = tx.Exec("DELETE FROM fsrs_cards WHERE topic_id = ?", topicID); err != nil {
		return fmt.Errorf("failed to delete fsrs_cards: %w", err)
	}

	// Delete questions
	if _, err = tx.Exec("DELETE FROM questions WHERE topic_id = ?", topicID); err != nil {
		return fmt.Errorf("failed to delete questions: %w", err)
	}

	// Delete topic_progress
	if _, err = tx.Exec("DELETE FROM topic_progress WHERE topic_id = ?", topicID); err != nil {
		return fmt.Errorf("failed to delete topic_progress: %w", err)
	}

	// Delete chunks
	if _, err = tx.Exec("DELETE FROM chunks WHERE topic_id = ?", topicID); err != nil {
		return fmt.Errorf("failed to delete chunks: %w", err)
	}

	// Delete parents
	if _, err = tx.Exec("DELETE FROM parents WHERE topic_id = ?", topicID); err != nil {
		return fmt.Errorf("failed to delete parents: %w", err)
	}

	// Update notebooks that reference this topic to null
	if _, err = tx.Exec("UPDATE notebooks SET topic_id = NULL WHERE topic_id = ?", topicID); err != nil {
		return fmt.Errorf("failed to update notebooks: %w", err)
	}

	// Finally delete the topic
	if _, err = tx.Exec("DELETE FROM topics WHERE id = ?", topicID); err != nil {
		return fmt.Errorf("failed to delete topic: %w", err)
	}

	// Commit the transaction
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
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

// GetAllTopicIDs returns all topic IDs currently in the database.
func GetAllTopicIDs() ([]string, error) {
	rows, err := conn.Query("SELECT id FROM topics ORDER BY id")
	if err != nil {
		return nil, err
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			log.Printf("warning: failed to close topic rows: %v", closeErr)
		}
	}()

	var topicIDs []string
	for rows.Next() {
		var topicID string
		if err := rows.Scan(&topicID); err != nil {
			return nil, err
		}
		topicIDs = append(topicIDs, topicID)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return topicIDs, nil
}

// GetAllTopics returns all topics as id/title pairs.
func GetAllTopics() ([]map[string]string, error) {
	rows, err := conn.Query("SELECT id, title FROM topics ORDER BY title")
	if err != nil {
		return nil, err
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			log.Printf("warning: failed to close topics rows: %v", closeErr)
		}
	}()

	topics := make([]map[string]string, 0)
	for rows.Next() {
		var id string
		var title string
		if err := rows.Scan(&id, &title); err != nil {
			return nil, err
		}
		topics = append(topics, map[string]string{
			"id":    id,
			"title": title,
		})
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return topics, nil
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

	tx, err := conn.Begin()
	if err != nil {
		return err
	}
	defer func() {
		_ = tx.Rollback()
	}()

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
			err = fmt.Errorf("chunk id is required for all batch items")
			return err
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

	return tx.Commit()
}

// GetChunkEmbeddingRef returns the stored embedding_ref hash for a topic-scoped chunk.
func GetChunkEmbeddingRef(topicID, chunkID string) (string, error) {
	var hash string
	if err := conn.QueryRow(`
		SELECT COALESCE(embedding_ref, '') FROM chunks WHERE id = ? AND topic_id = ?
	`, chunkID, topicID).Scan(&hash); err != nil {
		return "", err
	}

	return hash, nil
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

// ReplaceQuestionsForTopic replaces generated quiz questions for a topic in one transaction.
func ReplaceQuestionsForTopic(topicID string, questions []models.QuizQuestion) error {
	topicID = strings.TrimSpace(topicID)
	if topicID == "" {
		return fmt.Errorf("topic id is required")
	}

	normalized := make([]models.QuizQuestion, 0, len(questions))
	for _, q := range questions {
		q.TopicID = strings.TrimSpace(q.TopicID)
		if q.TopicID == "" {
			q.TopicID = topicID
		} else if q.TopicID != topicID {
			return fmt.Errorf("question topic id must match topic id")
		}
		normalized = append(normalized, q)
	}

	return replaceQuestionsForTopicRepo(topicID, normalized)
}

// AppendQuestionsForTopic appends generated quiz questions without deleting existing rows.
func AppendQuestionsForTopic(topicID string, questions []models.QuizQuestion) error {
	topicID = strings.TrimSpace(topicID)
	if topicID == "" {
		return fmt.Errorf("topic id is required")
	}
	if len(questions) == 0 {
		return fmt.Errorf("at least one question is required")
	}

	normalized := make([]models.QuizQuestion, 0, len(questions))
	for _, q := range questions {
		q.TopicID = strings.TrimSpace(q.TopicID)
		if q.TopicID == "" {
			q.TopicID = topicID
		} else if q.TopicID != topicID {
			return fmt.Errorf("question topic id must match topic id")
		}
		normalized = append(normalized, q)
	}

	return appendQuestionsForTopicRepo(topicID, normalized)
}

// GetQuestionsForTopic returns generated quiz questions for a topic.
func GetQuestionsForTopic(topicID string) ([]models.QuizQuestion, error) {
	topicID = strings.TrimSpace(topicID)
	if topicID == "" {
		return nil, fmt.Errorf("topic id is required")
	}
	return getQuestionsForTopicRepo(topicID)
}

// GetQuestionByID returns a single quiz question by ID.
func GetQuestionByID(questionID string) (*models.QuizQuestion, error) {
	questionID = strings.TrimSpace(questionID)
	if questionID == "" {
		return nil, fmt.Errorf("question id is required")
	}
	return getQuestionByIDRepo(questionID)
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

// GetAssessmentFSRSState returns shared assessment FSRS state for one quiz/written reference.
func GetAssessmentFSRSState(activityType, referenceID string) (*AssessmentFSRSRecord, error) {
	activityType = strings.TrimSpace(activityType)
	referenceID = strings.TrimSpace(referenceID)
	if activityType == "" || referenceID == "" {
		return nil, fmt.Errorf("activity type and reference id are required")
	}
	return getAssessmentFSRSStateRepo(activityType, referenceID)
}

// GetAssessmentFSRSStateTx returns shared assessment FSRS state for one quiz/written reference within a transaction.
func GetAssessmentFSRSStateTx(tx *sql.Tx, activityType, referenceID string) (*AssessmentFSRSRecord, error) {
	activityType = strings.TrimSpace(activityType)
	referenceID = strings.TrimSpace(referenceID)
	if activityType == "" || referenceID == "" {
		return nil, fmt.Errorf("activity type and reference id are required")
	}
	return getAssessmentFSRSStateRepoTx(tx, activityType, referenceID)
}

// UpsertAssessmentFSRSReview saves shared assessment FSRS state and corresponding review log.
func UpsertAssessmentFSRSReview(activityType, referenceID, topicID string, state models.FlashcardState, dueAt, reviewedAt int64, reviewLog models.FSRSReviewLog) error {
	activityType = strings.TrimSpace(activityType)
	referenceID = strings.TrimSpace(referenceID)
	topicID = strings.TrimSpace(topicID)
	if activityType == "" || referenceID == "" || topicID == "" {
		return fmt.Errorf("activity type, reference id, and topic id are required")
	}
	return upsertAssessmentFSRSReviewRepo(activityType, referenceID, topicID, state, dueAt, reviewedAt, reviewLog)
}

// UpsertAssessmentFSRSReviewTx saves shared assessment FSRS state and corresponding review log within a transaction.
func UpsertAssessmentFSRSReviewTx(tx *sql.Tx, activityType, referenceID, topicID string, state models.FlashcardState, dueAt, reviewedAt int64, reviewLog models.FSRSReviewLog) error {
	activityType = strings.TrimSpace(activityType)
	referenceID = strings.TrimSpace(referenceID)
	topicID = strings.TrimSpace(topicID)
	if activityType == "" || referenceID == "" || topicID == "" {
		return fmt.Errorf("activity type, reference id, and topic id are required")
	}
	return upsertAssessmentFSRSReviewRepoTx(tx, activityType, referenceID, topicID, state, dueAt, reviewedAt, reviewLog)
}

// SaveUserAnswer stores a scored quiz response.
func SaveUserAnswer(score models.QuizScore) error {
	score.QuestionID = strings.TrimSpace(score.QuestionID)
	if score.QuestionID == "" {
		return fmt.Errorf("question id is required")
	}
	// Validate UserAnswer without mutating original free-text input
	trimmedAnswer := strings.TrimSpace(score.UserAnswer)
	if trimmedAnswer == "" {
		return fmt.Errorf("user answer is required")
	}
	return saveUserAnswerRepo(score)
}

// SaveUserAnswerTx stores a scored quiz response within a transaction.
func SaveUserAnswerTx(tx *sql.Tx, score models.QuizScore) error {
	score.QuestionID = strings.TrimSpace(score.QuestionID)
	if score.QuestionID == "" {
		return fmt.Errorf("question id is required")
	}
	// Validate UserAnswer without mutating original free-text input
	trimmedAnswer := strings.TrimSpace(score.UserAnswer)
	if trimmedAnswer == "" {
		return fmt.Errorf("user answer is required")
	}
	return saveUserAnswerRepoTx(tx, score)
}

// SaveWrittenAnswer stores a scored written response.
func SaveWrittenAnswer(answer models.WrittenAnswer) error {
	answer.QuestionID = strings.TrimSpace(answer.QuestionID)
	if answer.QuestionID == "" {
		return fmt.Errorf("question id is required")
	}
	// Validate UserAnswer without mutating original free-text input
	trimmedAnswer := strings.TrimSpace(answer.UserAnswer)
	if trimmedAnswer == "" {
		return fmt.Errorf("user answer is required")
	}
	return saveWrittenAnswerRepo(answer)
}

// SaveWrittenAnswerTx stores a scored written response within a transaction.
func SaveWrittenAnswerTx(tx *sql.Tx, answer models.WrittenAnswer) error {
	answer.QuestionID = strings.TrimSpace(answer.QuestionID)
	if answer.QuestionID == "" {
		return fmt.Errorf("question id is required")
	}
	// Validate UserAnswer without mutating original free-text input
	trimmedAnswer := strings.TrimSpace(answer.UserAnswer)
	if trimmedAnswer == "" {
		return fmt.Errorf("user answer is required")
	}
	return saveWrittenAnswerRepoTx(tx, answer)
}

// UpdateTopicReadingCursor persists the current page cursor and optionally marks topic as learned.
func UpdateTopicReadingCursor(topicID string, cursor int, markLearned bool) error {
	topicID = strings.TrimSpace(topicID)
	if topicID == "" {
		return fmt.Errorf("topic id is required")
	}
	if cursor < 0 {
		cursor = 0
	}

	status := "reading"
	if markLearned {
		status = "learned"
	}

	result, err := conn.Exec(`
		UPDATE topics
		SET current_page_cursor = ?, status = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, cursor, status, topicID)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return fmt.Errorf("topic not found: %s", topicID)
	}

	return nil
}

// AppendQuestionsAndAdvanceCursor atomically appends questions and updates the reading cursor in a single transaction
func AppendQuestionsAndAdvanceCursor(topicID string, questions []models.QuizQuestion, nextCursor int, markLearned bool) error {
	if len(questions) == 0 {
		return fmt.Errorf("at least one question is required")
	}

	topicID = strings.TrimSpace(topicID)
	if topicID == "" {
		return fmt.Errorf("topic id is required")
	}
	if nextCursor < 0 {
		nextCursor = 0
	}

	tx, err := conn.Begin()
	if err != nil {
		return err
	}
	defer func() {
		_ = tx.Rollback()
	}()

	// Append questions first
	for _, q := range questions {
		if q.TopicID != topicID {
			err = fmt.Errorf("question topic_id %s does not match target topic %s", q.TopicID, topicID)
			return err
		}
		optionsJSON, marshalErr := json.Marshal(q.Options)
		if marshalErr != nil {
			err = fmt.Errorf("failed to encode options for question %s: %w", q.ID, marshalErr)
			return err
		}

		if _, err = tx.Exec(`
			INSERT INTO questions (
				id, topic_id, prompt, options_json, correct_answer, explanation, hint, source_heading, source_snippet,
				source_page_start, source_page_end, llm_model, prompt_version
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, q.ID, topicID, q.Prompt, string(optionsJSON), q.CorrectAnswer, q.Explanation, q.Hint, q.SourceHeading, q.SourceSnippet,
			q.SourcePageStart, q.SourcePageEnd, q.LLMModel, q.PromptVersion); err != nil {
			return err
		}
	}

	// Update cursor
	status := "reading"
	if markLearned {
		status = "learned"
	}

	result, err := tx.Exec(`
		UPDATE topics
		SET current_page_cursor = ?, status = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, nextCursor, status, topicID)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return fmt.Errorf("topic not found: %s", topicID)
	}

	return tx.Commit()
}
