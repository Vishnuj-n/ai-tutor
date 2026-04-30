package db

import (
	"database/sql"
	"fmt"
)

// InitSchema creates all tables and indexes with a single clean schema.
// This is a "nuclear" initialization that does NOT preserve existing data.
// All tables are created with CREATE TABLE IF NOT EXISTS and include all
// Sprint 14 requirements: FSRS tables, written questions, and current_page_cursor.
func InitSchema(tx *sql.Tx) error {
	schema := []string{
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

		`CREATE TABLE IF NOT EXISTS parents (
			id TEXT PRIMARY KEY,
			topic_id TEXT NOT NULL,
			heading TEXT,
			order_index INTEGER,
			content_text TEXT NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (topic_id) REFERENCES topics(id)
		)`,

		`CREATE TABLE IF NOT EXISTS chunks (
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
		)`,

		`CREATE TABLE IF NOT EXISTS topic_progress (
			topic_id TEXT PRIMARY KEY,
			learned_at TIMESTAMP,
			last_read_at TIMESTAMP,
			mastery_score REAL DEFAULT 0,
			review_enabled INTEGER DEFAULT 0,
			FOREIGN KEY (topic_id) REFERENCES topics(id)
		)`,

		// Quiz questions and user answers
		`CREATE TABLE IF NOT EXISTS questions (
			id TEXT PRIMARY KEY,
			topic_id TEXT NOT NULL,
			source_chunk_id TEXT,
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
			FOREIGN KEY (topic_id) REFERENCES topics(id) ON DELETE CASCADE,
			FOREIGN KEY (source_chunk_id) REFERENCES chunks(id) ON DELETE SET NULL
		)`,

		`CREATE TABLE IF NOT EXISTS user_answers (
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
			student_id TEXT,
			institutional_sync INTEGER DEFAULT 0,
			dashboard_endpoint TEXT,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,

		// Notebooks
		`CREATE TABLE IF NOT EXISTS notebooks (
			id TEXT PRIMARY KEY,
			title TEXT NOT NULL,
			file_path TEXT NOT NULL,
			file_type TEXT DEFAULT 'pdf',
			topic_id TEXT,
			status TEXT DEFAULT 'uploaded',
			page_count INTEGER,
			chunk_count INTEGER DEFAULT 0,
			mission_end_page INTEGER DEFAULT 0,
			uploaded_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (topic_id) REFERENCES topics(id)
		)`,

		`CREATE TABLE IF NOT EXISTS notebook_chunks (
			id TEXT PRIMARY KEY,
			notebook_id TEXT NOT NULL,
			chunk_id TEXT NOT NULL,
			page_num INTEGER DEFAULT 0,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
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

		// Assessment FSRS (Sprint 14)
		`CREATE TABLE IF NOT EXISTS assessment_fsrs (
			activity_type TEXT NOT NULL,
			reference_id TEXT NOT NULL,
			topic_id TEXT NOT NULL,
			source_chunk_id TEXT DEFAULT '',
			state_json TEXT NOT NULL,
			due_at INTEGER,
			last_reviewed_at INTEGER,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (activity_type, reference_id, source_chunk_id),
			FOREIGN KEY (topic_id) REFERENCES topics(id) ON DELETE CASCADE,
			FOREIGN KEY (source_chunk_id) REFERENCES chunks(id) ON DELETE SET NULL
		)`,

		// Sync outbox for institutional telemetry (Sprint 15)
		`CREATE TABLE IF NOT EXISTS sync_outbox (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			payload TEXT NOT NULL,
			event_type TEXT NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
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
		`CREATE INDEX IF NOT EXISTS idx_assessment_fsrs_topic_due_at ON assessment_fsrs(topic_id, due_at)`,
		`CREATE INDEX IF NOT EXISTS idx_chunks_topic_page_num ON chunks(topic_id, page_num)`,
		`CREATE INDEX IF NOT EXISTS idx_topics_status_updated_at ON topics(status, updated_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_topics_status_created_at ON topics(status, created_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_sync_outbox_event_type_created_at ON sync_outbox(event_type, created_at DESC)`,
	}

	for _, stmt := range indexes {
		if _, err := tx.Exec(stmt); err != nil {
			return fmt.Errorf("failed to create index: %w", err)
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
