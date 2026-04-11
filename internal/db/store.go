package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"ai-tutor/internal/models"

	sqlite3 "github.com/mattn/go-sqlite3"
)

var conn *sql.DB
var embeddingDimension int32 = 0 // Will be set during DB initialization with vec0

// Close releases the active SQLite connection.
func Close() error {
	if conn == nil {
		return nil
	}
	err := conn.Close()
	conn = nil
	return err
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
				log.Printf("Successfully loaded sqlite-vec extension from %s", absPath)
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

	// Seed initial data
	if err := seedData(); err != nil {
		log.Printf("Warning: could not seed data: %v", err)
	}

	return nil
}

func loadExtension(db *sql.DB, extensionPath string) error {
	sqlConn, err := db.Conn(context.Background())
	if err != nil {
		return err
	}
	defer func() {
		_ = sqlConn.Close()
	}()

	return sqlConn.Raw(func(driverConn interface{}) error {
		sqliteConn, ok := driverConn.(*sqlite3.SQLiteConn)
		if !ok {
			return fmt.Errorf("unexpected sqlite driver connection type %T", driverConn)
		}

		entryPoints := []string{"sqlite3_vec_init", "sqlite3_extension_init", ""}
		var lastErr error
		for _, entry := range entryPoints {
			if loadErr := sqliteConn.LoadExtension(extensionPath, entry); loadErr == nil {
				return nil
			} else {
				lastErr = loadErr
			}
		}

		if lastErr == nil {
			lastErr = fmt.Errorf("unknown extension load failure")
		}
		return fmt.Errorf("could not load extension with known entry points: %w", lastErr)
	})
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

	CREATE TABLE IF NOT EXISTS fsrs_cards (
		id TEXT PRIMARY KEY,
		topic_id TEXT NOT NULL,
		prompt TEXT NOT NULL,
		answer TEXT NOT NULL,
		state_json TEXT,
		due_at TEXT,
		suspended INTEGER DEFAULT 0,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (topic_id) REFERENCES topics(id)
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
	return err
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

func seedData() error {
	// Check if topics already exist
	var count int
	err := conn.QueryRow("SELECT COUNT(*) FROM topics").Scan(&count)
	if err != nil {
		return err
	}
	if count > 0 {
		return nil // Already seeded
	}

	// Insert topics
	topic1 := "os-scheduling"
	title1 := "Operating Systems: Scheduling"

	_, err = conn.Exec(`
		INSERT INTO topics (id, title, status)
		VALUES (?, ?, ?)
	`, topic1, title1, "reading")
	if err != nil {
		return err
	}

	// Insert parent sections for topic 1
	parent1 := "parent-1"
	parent2 := "parent-2"

	_, err = conn.Exec(`
		INSERT INTO parents (id, topic_id, heading, order_index, content_text)
		VALUES (?, ?, ?, ?, ?)
	`, parent1, topic1, "Round Robin Scheduling", 1, `
Round Robin (RR) is a preemptive scheduling algorithm where each process is assigned a fixed time slice called a time quantum or time slice. 
Each process in the ready queue gets a turn to execute for the duration of the time quantum. 
If the process does not complete within its time quantum, it is moved to the back of the queue and the next process gets a turn.
This ensures fair allocation of CPU time among all processes.
Key characteristics:
- Fair share of CPU time
- Good for time-sharing systems
- Context switching overhead increases with more processes
- Performance depends on time quantum selection
`)
	if err != nil {
		return err
	}

	_, err = conn.Exec(`
		INSERT INTO parents (id, topic_id, heading, order_index, content_text)
		VALUES (?, ?, ?, ?, ?)
	`, parent2, topic1, "Advantages and Disadvantages", 2, `
Advantages of Round Robin:
- Fair distribution of CPU time
- No process starvation (all processes get a turn)
- Good for interactive systems
- Simple to implement

Disadvantages of Round Robin:
- High context switching overhead if time quantum is too small
- Performance depends heavily on time quantum selection
- Not suitable for batch processing
- Larger time quantum reduces fairness
`)
	if err != nil {
		return err
	}

	// Create chunks from parents
	chunks := []struct {
		id   string
		pID  string
		text string
	}{
		{
			"chunk-1",
			parent1,
			"Round Robin (RR) is a preemptive scheduling algorithm where each process is assigned a fixed time slice called a time quantum.",
		},
		{
			"chunk-2",
			parent1,
			"Each process in the ready queue gets a turn to execute for the duration of the time quantum.",
		},
		{
			"chunk-3",
			parent1,
			"If the process does not complete within its time quantum, it is moved to the back of the queue and the next process gets a turn.",
		},
		{
			"chunk-4",
			parent1,
			"Round Robin ensures fair allocation of CPU time among all processes with characteristics like fair share, good for time-sharing systems, and context switching overhead.",
		},
		{
			"chunk-5",
			parent2,
			"Round Robin advantages include fair distribution of CPU time, no process starvation, good for interactive systems, and simple implementation.",
		},
		{
			"chunk-6",
			parent2,
			"Round Robin disadvantages include high context switching overhead, performance dependency on time quantum, unsuitability for batch processing, and tradeoffs between fairness and quantum size.",
		},
	}

	for _, chunk := range chunks {
		_, err = conn.Exec(`
			INSERT INTO chunks (id, topic_id, parent_id, chunk_text, token_count, importance_score, weakness_score)
			VALUES (?, ?, ?, ?, ?, ?, ?)
		`, chunk.id, topic1, chunk.pID, chunk.text, len(chunk.text)/4, 0.0, 0.0)
		if err != nil {
			return err
		}
	}

	return nil
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

// QueryDueReviewCards counts cards due by the given time
func QueryDueReviewCards(now string) (int, error) {
	var count int
	err := conn.QueryRow(`
		SELECT COUNT(*)
		FROM fsrs_cards
		WHERE suspended = 0
		  AND due_at IS NOT NULL
		  AND due_at <= ?
	`, now).Scan(&count)
	return count, err
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
		INSERT INTO chunks (id, topic_id, parent_id, chunk_text, token_count, importance_score, weakness_score)
		VALUES (?, ?, ?, ?, ?, 0, 0)
	`, chunk.ID, topicID, chunk.ParentID, chunk.Text, chunk.TokenCount)
	return err
}

func linkNotebookChunkRow(exec sqlExecer, notebookID string, chunk NotebookChunkInput) error {
	linkID := "nb-chunk-" + notebookID + "-" + chunk.ID
	_, err := exec.Exec(`
		INSERT INTO notebook_chunks (id, notebook_id, chunk_id, page_num)
		VALUES (?, ?, ?, ?)
	`, linkID, notebookID, chunk.ID, chunk.PageNum)
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
func CreateChunk(id, topicID, parentID, text string, tokenCount int) error {
	return insertChunkRow(conn, topicID, NotebookChunkInput{
		ID:         id,
		ParentID:   parentID,
		Text:       text,
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

// IngestNotebookContent performs a transactional relational commit for notebook sections/chunks.
func IngestNotebookContent(notebookID string, topicID string, parents []NotebookParentInput, chunks []NotebookChunkInput) error {
	if topicID == "" {
		return fmt.Errorf("topic id is required for ingestion")
	}
	group := NotebookTopicIngestionGroup{
		TopicID: topicID,
		Parents: parents,
		Chunks:  chunks,
	}
	return IngestNotebookContentByTopic(notebookID, []NotebookTopicIngestionGroup{group})
}

// IngestNotebookContentByTopic ingests notebook content into multiple topic buckets in one transaction.
func IngestNotebookContentByTopic(notebookID string, groups []NotebookTopicIngestionGroup) error {
	if notebookID == "" {
		return fmt.Errorf("notebook id is required")
	}
	if len(groups) == 0 {
		return fmt.Errorf("at least one topic group is required")
	}

	tx, err := conn.Begin()
	if err != nil {
		return err
	}
	defer func() {
		_ = tx.Rollback()
	}()

	if _, err := tx.Exec(`
		UPDATE notebooks
		SET status = ?, chunk_count = 0
		WHERE id = ?
	`, "processing", notebookID); err != nil {
		return err
	}

	if _, err := tx.Exec("DELETE FROM notebook_chunks WHERE notebook_id = ?", notebookID); err != nil {
		return err
	}

	parentPrefix := fmt.Sprintf("nbp_%s_%%", notebookID)
	chunkPrefix := fmt.Sprintf("nbc_%s_%%", notebookID)

	if _, err := tx.Exec("DELETE FROM chunks WHERE id LIKE ?", chunkPrefix); err != nil {
		return err
	}
	if _, err := tx.Exec("DELETE FROM parents WHERE id LIKE ?", parentPrefix); err != nil {
		return err
	}

	totalChunks := 0
	for _, group := range groups {
		if group.TopicID == "" {
			return fmt.Errorf("topic id is required for each ingestion group")
		}

		for _, parent := range group.Parents {
			if err := insertParentRow(tx, group.TopicID, parent); err != nil {
				return err
			}
		}

		for _, chunk := range group.Chunks {
			if err := insertChunkRow(tx, group.TopicID, chunk); err != nil {
				return err
			}

			if err := linkNotebookChunkRow(tx, notebookID, chunk); err != nil {
				return err
			}

			totalChunks++
		}
	}

	if _, err := tx.Exec(`
		UPDATE notebooks
		SET chunk_count = ?, status = ?, topic_id = ?
		WHERE id = ?
	`, totalChunks, "chunked", groups[0].TopicID, notebookID); err != nil {
		return err
	}

	return tx.Commit()
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
	tx, err := conn.Begin()
	if err != nil {
		return err
	}
	defer func() {
		_ = tx.Rollback()
	}()

	chunkRows, err := tx.Query(`
		SELECT chunk_id
		FROM notebook_chunks
		WHERE notebook_id = ?
	`, notebookID)
	if err != nil {
		return err
	}

	chunkIDs := make([]string, 0)
	for chunkRows.Next() {
		var chunkID string
		if scanErr := chunkRows.Scan(&chunkID); scanErr != nil {
			_ = chunkRows.Close()
			return scanErr
		}
		chunkIDs = append(chunkIDs, chunkID)
	}
	if rowsErr := chunkRows.Err(); rowsErr != nil {
		_ = chunkRows.Close()
		return rowsErr
	}
	_ = chunkRows.Close()

	parentIDs := make(map[string]struct{})
	hasChunkVectors := false
	if exists, tableErr := doesTableExistTx(tx, "chunk_vectors"); tableErr != nil {
		return tableErr
	} else {
		hasChunkVectors = exists
	}

	for _, chunkID := range chunkIDs {
		var parentID string
		var chunkRowID int64
		if parentErr := tx.QueryRow(`
			SELECT rowid, parent_id FROM chunks WHERE id = ?
		`, chunkID).Scan(&chunkRowID, &parentID); parentErr == nil {
			parentIDs[parentID] = struct{}{}

			if hasChunkVectors {
				if _, delVecErr := tx.Exec(`
					DELETE FROM chunk_vectors WHERE rowid = ?
				`, chunkRowID); delVecErr != nil {
					return delVecErr
				}
			}
		}

		if _, delChunkErr := tx.Exec(`
			DELETE FROM chunks WHERE id = ?
		`, chunkID); delChunkErr != nil {
			return delChunkErr
		}
	}

	_, err = tx.Exec("DELETE FROM notebook_chunks WHERE notebook_id = ?", notebookID)
	if err != nil {
		return err
	}

	_, err = tx.Exec("DELETE FROM notebooks WHERE id = ?", notebookID)
	if err != nil {
		return err
	}

	for parentID := range parentIDs {
		var count int
		if countErr := tx.QueryRow(`
			SELECT COUNT(*) FROM chunks WHERE parent_id = ?
		`, parentID).Scan(&count); countErr != nil {
			return countErr
		}
		if count == 0 {
			if _, delParentErr := tx.Exec(`
				DELETE FROM parents WHERE id = ?
			`, parentID); delParentErr != nil {
				return delParentErr
			}
		}
	}

	topicRows, err := tx.Query(`
		SELECT id
		FROM topics
		WHERE id LIKE ?
	`, "nb-"+notebookID+"-%")
	if err != nil {
		return err
	}

	autoTopicIDs := make([]string, 0)
	for topicRows.Next() {
		var topicID string
		if scanErr := topicRows.Scan(&topicID); scanErr != nil {
			_ = topicRows.Close()
			return scanErr
		}
		autoTopicIDs = append(autoTopicIDs, topicID)
	}
	if rowsErr := topicRows.Err(); rowsErr != nil {
		_ = topicRows.Close()
		return rowsErr
	}
	_ = topicRows.Close()

	for _, topicID := range autoTopicIDs {
		var parentCount int
		if parentCountErr := tx.QueryRow(`
			SELECT COUNT(*) FROM parents WHERE topic_id = ?
		`, topicID).Scan(&parentCount); parentCountErr != nil {
			return parentCountErr
		}

		var chunkCount int
		if chunkCountErr := tx.QueryRow(`
			SELECT COUNT(*) FROM chunks WHERE topic_id = ?
		`, topicID).Scan(&chunkCount); chunkCountErr != nil {
			return chunkCountErr
		}

		if parentCount == 0 && chunkCount == 0 {
			if _, delProgressErr := tx.Exec(`
				DELETE FROM topic_progress WHERE topic_id = ?
			`, topicID); delProgressErr != nil {
				return delProgressErr
			}
			if _, delTopicErr := tx.Exec(`
				DELETE FROM topics WHERE id = ?
			`, topicID); delTopicErr != nil {
				return delTopicErr
			}
		}
	}

	return tx.Commit()
}

func doesTableExistTx(tx *sql.Tx, tableName string) (bool, error) {
	var count int
	err := tx.QueryRow(`
		SELECT COUNT(1)
		FROM sqlite_master
		WHERE type = 'table' AND name = ?
	`, tableName).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func vectorToJSON(vector []float32) (string, error) {
	if len(vector) == 0 {
		return "[]", nil
	}

	values := make([]float64, len(vector))
	for i, value := range vector {
		values[i] = float64(value)
	}

	encoded, err := json.Marshal(values)
	if err != nil {
		return "", err
	}

	return string(encoded), nil
}

func lookupChunkRowID(chunkID string) (int64, error) {
	var rowID int64
	if err := conn.QueryRow(`
		SELECT rowid FROM chunks WHERE id = ?
	`, chunkID).Scan(&rowID); err != nil {
		return 0, err
	}

	return rowID, nil
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

	log.Printf("Created vec0 virtual table with embedding dimension %d", embeddingDimension)
	return nil
}

// UpsertChunkVector stores or updates a chunk embedding vector.
// Returns true if inserted, false if updated.
func UpsertChunkVector(chunkID string, vector []float32) error {
	if len(vector) != int(embeddingDimension) {
		return fmt.Errorf("vector dimension mismatch: got %d, expected %d", len(vector), embeddingDimension)
	}

	vectorJSON, err := vectorToJSON(vector)
	if err != nil {
		return fmt.Errorf("failed to encode vector: %w", err)
	}

	rowID, err := lookupChunkRowID(chunkID)
	if err != nil {
		return fmt.Errorf("failed to resolve chunk rowid for %s: %w", chunkID, err)
	}

	// Check if chunk vector already exists
	var exists int
	err = conn.QueryRow(`
		SELECT COUNT(*) FROM chunk_vectors WHERE rowid = ?
	`, rowID).Scan(&exists)
	if err != nil && err != sql.ErrNoRows {
		return err
	}

	if exists > 0 {
		// Update existing
		_, err = conn.Exec(`
			UPDATE chunk_vectors SET embedding = ? WHERE rowid = ?
		`, vectorJSON, rowID)
		return err
	} else {
		// Insert new
		_, err = conn.Exec(`
			INSERT INTO chunk_vectors (rowid, embedding) VALUES (?, ?)
		`, rowID, vectorJSON)
		return err
	}
}

// SearchVectorsForTopic finds the top-k most similar vectors for a topic-scoped query.
// Returns list of chunk IDs ordered by similarity (highest first).
func SearchVectorsForTopic(topicID string, queryVector []float32, k int) ([]string, error) {
	if len(queryVector) != int(embeddingDimension) {
		return nil, fmt.Errorf("query vector dimension mismatch: got %d, expected %d", len(queryVector), embeddingDimension)
	}

	queryVectorJSON, err := vectorToJSON(queryVector)
	if err != nil {
		return nil, fmt.Errorf("failed to encode query vector: %w", err)
	}

	// Use vec0 distance search, scoped to topic
	// Note: vec0's distance metric is typically L2 (euclidean) or cosine depending on build
	rows, err := conn.Query(`
		SELECT c.id
		FROM chunk_vectors cv
		JOIN chunks c ON c.rowid = cv.rowid
		WHERE c.topic_id = ?
		ORDER BY distance(cv.embedding, ?) ASC
		LIMIT ?
	`, topicID, queryVectorJSON, k)
	if err != nil {
		return nil, fmt.Errorf("vector search failed: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			log.Printf("warning: failed to close vector search rows: %v", closeErr)
		}
	}()

	var chunkIDs []string
	for rows.Next() {
		var chunkID string
		if err := rows.Scan(&chunkID); err != nil {
			return nil, err
		}
		chunkIDs = append(chunkIDs, chunkID)
	}

	return chunkIDs, nil
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
