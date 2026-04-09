package main

import (
	"database/sql"
	"log"

	_ "github.com/mattn/go-sqlite3"
)

var db *sql.DB

// Chunk represents a retrieval chunk with metadata and future scoring hooks.
type Chunk struct {
	ID              string
	TopicID         string
	ParentID        string
	Text            string
	ImportanceScore float64
	WeaknessScore   float64
}

// InitDB initializes the SQLite database and creates tables
func InitDB(dbPath string) error {
	var err error
	db, err = sql.Open("sqlite3", dbPath)
	if err != nil {
		return err
	}

	if err := db.Ping(); err != nil {
		return err
	}

	// Create tables
	if err := createTables(); err != nil {
		return err
	}

	// Seed initial data
	if err := seedData(); err != nil {
		log.Printf("Warning: could not seed data: %v", err)
	}

	return nil
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
	`

	_, err := db.Exec(schema)
	return err
}

func seedData() error {
	// Check if topics already exist
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM topics").Scan(&count)
	if err != nil {
		return err
	}
	if count > 0 {
		return nil // Already seeded
	}

	// Insert topics
	topic1 := "os-scheduling"
	title1 := "Operating Systems: Scheduling"

	_, err = db.Exec(`
		INSERT INTO topics (id, title, status)
		VALUES (?, ?, ?)
	`, topic1, title1, "reading")
	if err != nil {
		return err
	}

	// Insert parent sections for topic 1
	parent1 := "parent-1"
	parent2 := "parent-2"

	_, err = db.Exec(`
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

	_, err = db.Exec(`
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
		_, err = db.Exec(`
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
	err := db.QueryRow("SELECT title FROM topics WHERE id = ?", topicID).Scan(&title)
	if err != nil {
		return nil, err
	}

	rows, err := db.Query(`
		SELECT id, heading, content_text, order_index
		FROM parents
		WHERE topic_id = ?
		ORDER BY order_index
	`, topicID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

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
func GetChunksForTopic(topicID string) ([]Chunk, error) {
	rows, err := db.Query(`
		SELECT id, topic_id, parent_id, chunk_text, importance_score, weakness_score
		FROM chunks
		WHERE topic_id = ?
	`, topicID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var chunks []Chunk
	for rows.Next() {
		var chunk Chunk
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
	err := db.QueryRow(`
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
