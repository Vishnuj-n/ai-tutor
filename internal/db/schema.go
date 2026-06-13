package db

import (
	"database/sql"
	"fmt"
	"strings"

	"ai-tutor/internal/utils"
)

// InitSchema creates all tables and indexes with a single clean schema.
// This is a "nuclear" initialization that does NOT preserve existing data.
// All tables are created with CREATE TABLE IF NOT EXISTS and include all
// Sprint 14 requirements: FSRS tables, written questions, and current_page_cursor.
func InitSchema(tx *sql.Tx) error {
	schema := []string{
		// Profiles table
		`CREATE TABLE IF NOT EXISTS study_profiles (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			deadline_at INTEGER NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,

		// Core tables
		`CREATE TABLE IF NOT EXISTS topics (
			id TEXT PRIMARY KEY,
			title TEXT NOT NULL,
			status TEXT DEFAULT 'reading',
			start_page INTEGER DEFAULT 0,
			end_page INTEGER DEFAULT 0,
			current_page_cursor INTEGER DEFAULT 0,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,

		`CREATE TABLE IF NOT EXISTS chunks (
			id TEXT PRIMARY KEY,
			topic_id TEXT NOT NULL,
			chunk_text TEXT NOT NULL,
			page_num INTEGER DEFAULT 0,
			token_count INTEGER DEFAULT 0,
			importance_score REAL DEFAULT 0,
			weakness_score REAL DEFAULT 0,
			embedding_ref TEXT,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (topic_id) REFERENCES topics(id)
		)`,

		`CREATE TABLE IF NOT EXISTS topic_progress (
			topic_id TEXT PRIMARY KEY,
			learned_at TIMESTAMP,
			last_read_at TIMESTAMP,
			mastery_score REAL DEFAULT 0,
			review_enabled INTEGER DEFAULT 0,
			status TEXT DEFAULT 'active'
		)`,

		`CREATE TABLE IF NOT EXISTS quiz_attempts (
			id TEXT PRIMARY KEY,
			task_id TEXT NOT NULL,
			score INTEGER NOT NULL,
			passed INTEGER NOT NULL,
			answers_json TEXT NOT NULL,
			feedback TEXT,
			completed_at INTEGER NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (task_id) REFERENCES study_queue(id) ON DELETE CASCADE
		)`,

		`CREATE TABLE IF NOT EXISTS reread_attempts (
			topic_id TEXT PRIMARY KEY,
			attempt_count INTEGER NOT NULL DEFAULT 0,
			last_attempt_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (topic_id) REFERENCES topics(id) ON DELETE CASCADE
		)`,

		// Written questions and user answers
		`CREATE TABLE IF NOT EXISTS written_questions (
			id TEXT PRIMARY KEY,
			topic_id TEXT NOT NULL,
			prompt TEXT NOT NULL,
			source_chunk_id TEXT,
			source_heading TEXT,
			source_page_start INTEGER DEFAULT 0,
			source_page_end INTEGER DEFAULT 0,
			llm_model TEXT,
			prompt_version TEXT,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (topic_id) REFERENCES topics(id) ON DELETE CASCADE,
			FOREIGN KEY (source_chunk_id) REFERENCES chunks(id) ON DELETE SET NULL
		)`,

		`CREATE TABLE IF NOT EXISTS written_user_answers (
			id TEXT PRIMARY KEY,
			written_question_id TEXT NOT NULL,
			user_answer TEXT NOT NULL,
			score INTEGER NOT NULL,
			feedback TEXT,
			source_heading TEXT,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (written_question_id) REFERENCES written_questions(id) ON DELETE CASCADE
		)`,

		// User settings
		`CREATE TABLE IF NOT EXISTS user_settings (
			id INTEGER PRIMARY KEY CHECK (id = 1),
			daily_study_minutes INTEGER NOT NULL DEFAULT 90,
			active_profile_id TEXT,
			skip_to_reading_active BOOLEAN DEFAULT 0,
			cloud_sync_url TEXT DEFAULT '',
			cloud_api_token TEXT DEFAULT '',
			theme TEXT DEFAULT 'light-classic',
			rag_enabled BOOLEAN DEFAULT 0,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (active_profile_id) REFERENCES study_profiles(id) ON DELETE SET NULL
		)`,

		// Notebooks
		`CREATE TABLE IF NOT EXISTS notebooks (
			id TEXT PRIMARY KEY,
			title TEXT NOT NULL,
			file_path TEXT NOT NULL,
			file_type TEXT DEFAULT 'pdf',
			topic_id TEXT,
			priority INTEGER DEFAULT 5,
			status TEXT DEFAULT 'uploaded',
			indexing_status TEXT DEFAULT 'PENDING',
			page_count INTEGER,
			chunk_count INTEGER DEFAULT 0,
			syllabus_draft_json TEXT,
			exam_deadline TEXT,
			uploaded_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			profile_id TEXT,
			study_status TEXT DEFAULT 'dormant',
			FOREIGN KEY (topic_id) REFERENCES topics(id),
			FOREIGN KEY (profile_id) REFERENCES study_profiles(id) ON DELETE SET NULL
		)`,

		`CREATE TABLE IF NOT EXISTS notebook_topics (
			notebook_id TEXT NOT NULL,
			topic_id TEXT NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (notebook_id, topic_id),
			FOREIGN KEY (notebook_id) REFERENCES notebooks(id) ON DELETE CASCADE,
			FOREIGN KEY (topic_id) REFERENCES topics(id) ON DELETE CASCADE
		)`,

		`CREATE TABLE IF NOT EXISTS notebook_chunks (
			id TEXT PRIMARY KEY,
			notebook_id TEXT NOT NULL,
			chunk_id TEXT NOT NULL,
			page_num INTEGER DEFAULT 0,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			UNIQUE (notebook_id, chunk_id),
			FOREIGN KEY (notebook_id) REFERENCES notebooks(id),
			FOREIGN KEY (chunk_id) REFERENCES chunks(id)
		)`,

		// FSRS tables (Sprint 14)
		`CREATE TABLE IF NOT EXISTS fsrs_cards (
			id TEXT PRIMARY KEY,
			topic_id TEXT NOT NULL,
			source_chunk_id TEXT,
			prompt TEXT NOT NULL,
			answer TEXT NOT NULL,
			state_json TEXT,
			due_at INTEGER,
			suspended BOOLEAN DEFAULT 0,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (topic_id) REFERENCES topics(id) ON DELETE CASCADE,
			FOREIGN KEY (source_chunk_id) REFERENCES chunks(id) ON DELETE SET NULL
		)`,

		`CREATE TABLE IF NOT EXISTS fsrs_review_log (
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

		// Study queue (Sprint 1 foundation)
		`CREATE TABLE IF NOT EXISTS study_queue (
			id TEXT PRIMARY KEY,
			notebook_id TEXT NOT NULL,
			topic_id TEXT,
			task_type TEXT NOT NULL,
			status TEXT NOT NULL,
			priority INTEGER DEFAULT 0,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			activated_at TIMESTAMP,
			completed_at TIMESTAMP,
			payload_json TEXT,
			start_page INTEGER,
			end_page INTEGER,
			FOREIGN KEY (notebook_id) REFERENCES notebooks(id),
			FOREIGN KEY (topic_id) REFERENCES topics(id)
		)`,

		`CREATE TABLE IF NOT EXISTS reading_progress (
			task_id TEXT PRIMARY KEY,
			current_page INTEGER DEFAULT 0,
			last_accessed_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (task_id) REFERENCES study_queue(id)
		)`,

		`CREATE TABLE IF NOT EXISTS review_task_cards (
			task_id TEXT NOT NULL,
			card_id TEXT NOT NULL,
			status TEXT NOT NULL DEFAULT 'pending',
			PRIMARY KEY (task_id, card_id),
			FOREIGN KEY (task_id) REFERENCES study_queue(id) ON DELETE CASCADE,
			FOREIGN KEY (card_id) REFERENCES fsrs_cards(id) ON DELETE CASCADE
		)`,

		// Add this block inside the schema array in InitSchema
		`CREATE TABLE IF NOT EXISTS manual_flashcards (
		id TEXT PRIMARY KEY,
		notebook_id TEXT NOT NULL,
		prompt TEXT NOT NULL,
		answer TEXT NOT NULL,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (notebook_id) REFERENCES notebooks(id) ON DELETE CASCADE
)`,
	}

	// Execute all table creation statements
	for _, stmt := range schema {
		if _, err := tx.Exec(stmt); err != nil {
			return fmt.Errorf("failed to create table: %w", err)
		}
	}

	// Create indexes
	indexes := []string{
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_fsrs_cards_topic_prompt ON fsrs_cards(topic_id, prompt)`,
		`CREATE INDEX IF NOT EXISTS idx_fsrs_review_log_activity_ref_reviewed_at ON fsrs_review_log(activity_type, reference_id, reviewed_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_fsrs_review_log_topic_reviewed_at ON fsrs_review_log(topic_id, reviewed_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_fsrs_cards_suspended_due_at ON fsrs_cards(suspended, due_at)`,
		`CREATE INDEX IF NOT EXISTS idx_written_questions_topic_created_at ON written_questions(topic_id, created_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_chunks_topic_page_num ON chunks(topic_id, page_num)`,
		`CREATE INDEX IF NOT EXISTS idx_topics_status_updated_at ON topics(status, updated_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_topics_status_created_at ON topics(status, created_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_study_queue_status_priority_created ON study_queue(status, priority, created_at)`,
		`CREATE INDEX IF NOT EXISTS idx_study_queue_notebook_status ON study_queue(notebook_id, status)`,
		`CREATE INDEX IF NOT EXISTS idx_review_task_cards_task_status ON review_task_cards(task_id, status)`,
		`CREATE INDEX IF NOT EXISTS idx_quiz_attempts_task_completed_at ON quiz_attempts(task_id, completed_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_reread_attempts_last_attempt_at ON reread_attempts(last_attempt_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_manual_flashcards_notebook_id ON manual_flashcards(notebook_id)`,
	}

	for _, stmt := range indexes {
		if _, err := tx.Exec(stmt); err != nil {
			return fmt.Errorf("failed to create index: %w", err)
		}
	}

	// Ensure uniqueness of (notebook_id, chunk_id) for existing databases.
	// For DBs created earlier without the UNIQUE constraint, dedupe existing
	// rows first, then create a unique index. This avoids adding the constraint
	// directly which would fail if duplicates exist.
	{
		rows, err := tx.Query(`
			SELECT id, notebook_id, chunk_id, created_at
			FROM notebook_chunks
			ORDER BY notebook_id, chunk_id, created_at ASC
		`)
		if err != nil {
			return fmt.Errorf("failed to query notebook_chunks for dedupe: %w", err)
		}
		defer func() {
			_ = rows.Close()
		}()
		seen := make(map[string]bool)
		var idsToDelete []string
		for rows.Next() {
			var id, nb, cid string
			var createdAt string
			if err := rows.Scan(&id, &nb, &cid, &createdAt); err != nil {
				return fmt.Errorf("failed to scan notebook_chunks dedupe row id=%s notebook_id=%s chunk_id=%s: %w", id, nb, cid, err)
			}
			key := nb + "::" + cid
			if seen[key] {
				idsToDelete = append(idsToDelete, id)
				continue
			}
			seen[key] = true
		}
		if err := rows.Err(); err != nil {
			return fmt.Errorf("failed while iterating notebook_chunks dedupe rows: %w", err)
		}

		for _, id := range idsToDelete {
			if _, err := tx.Exec(`DELETE FROM notebook_chunks WHERE id = ?`, id); err != nil {
				return fmt.Errorf("failed to delete duplicate notebook_chunks row id=%s: %w", id, err)
			}
		}

		// Create unique index (works on SQLite/Postgres with IF NOT EXISTS semantics for sqlite)
		if _, err := tx.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_notebook_chunk_unique ON notebook_chunks(notebook_id, chunk_id)`); err != nil {
			// Ignore duplicate index errors where they indicate index already exists.
			if !strings.Contains(strings.ToLower(err.Error()), "already exists") {
				return fmt.Errorf("failed to create unique index on notebook_chunks: %w", err)
			}
		}
	}

	// Safely add columns if they do not exist (active migration support)
	alterStatements := []struct {
		table  string
		column string
		sql    string
	}{
		{"notebooks", "profile_id", "ALTER TABLE notebooks ADD COLUMN profile_id TEXT REFERENCES study_profiles(id) ON DELETE SET NULL"},
		{"notebooks", "study_status", "ALTER TABLE notebooks ADD COLUMN study_status TEXT DEFAULT 'dormant'"},
		{"notebooks", "exam_deadline", "ALTER TABLE notebooks ADD COLUMN exam_deadline TEXT"},
		{"notebooks", "priority", "ALTER TABLE notebooks ADD COLUMN priority INTEGER DEFAULT 5"},
		{"notebooks", "indexing_status", "ALTER TABLE notebooks ADD COLUMN indexing_status TEXT DEFAULT 'PENDING'"},
		{"notebooks", "syllabus_draft_json", "ALTER TABLE notebooks ADD COLUMN syllabus_draft_json TEXT"},
		{"topic_progress", "status", "ALTER TABLE topic_progress ADD COLUMN status TEXT DEFAULT 'active'"},
		{"user_settings", "active_profile_id", "ALTER TABLE user_settings ADD COLUMN active_profile_id TEXT REFERENCES study_profiles(id) ON DELETE SET NULL"},
		{"user_settings", "skip_to_reading_active", "ALTER TABLE user_settings ADD COLUMN skip_to_reading_active BOOLEAN DEFAULT 0"},
		{"user_settings", "cloud_sync_url", "ALTER TABLE user_settings ADD COLUMN cloud_sync_url TEXT DEFAULT ''"},
		{"user_settings", "cloud_api_token", "ALTER TABLE user_settings ADD COLUMN cloud_api_token TEXT DEFAULT ''"},
		{"user_settings", "theme", "ALTER TABLE user_settings ADD COLUMN theme TEXT DEFAULT 'light-classic'"},
		{"user_settings", "rag_enabled", "ALTER TABLE user_settings ADD COLUMN rag_enabled BOOLEAN DEFAULT 0"},
	}

	for _, stmt := range alterStatements {
		var count int
		err := tx.QueryRow(fmt.Sprintf("SELECT count(*) FROM pragma_table_info('%s') WHERE name='%s'", stmt.table, stmt.column)).Scan(&count)
		if err == nil && count == 0 {
			if _, err := tx.Exec(stmt.sql); err != nil {
				return fmt.Errorf("[SCHEMA] failed to add column %s to %s (%s): %w", stmt.column, stmt.table, stmt.sql, err)
			}
			utils.Warnf("[SCHEMA] added column %s to %s", stmt.column, stmt.table)
		}
	}

	// Initialize default user settings
	if _, err := tx.Exec(`
		INSERT INTO user_settings (id, daily_study_minutes)
		VALUES (1, 90)
		ON CONFLICT(id) DO NOTHING
	`); err != nil {
		return fmt.Errorf("failed to initialize user settings: %w", err)
	}

	return nil
}
