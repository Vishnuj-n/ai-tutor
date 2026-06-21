package db

import "fmt"

// SeedDemoDataForTests inserts demo content for test isolation.
// This function is only called from test setUp helpers; never imported by production code.
func (r *Repository) SeedDemoDataForTests() error {
	// Guard against uninitialized database
	if r == nil || r.db == nil {
		return fmt.Errorf("database not initialized; call db.Init() first")
	}

	// Begin transaction for atomic seed operation to ensure idempotency and atomicity
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer func() {
		if tx != nil {
			_ = tx.Rollback()
		}
	}()

	// Check if demo topic already exists within transaction to prevent races
	var exists bool
	err = tx.QueryRow("SELECT EXISTS(SELECT 1 FROM topics WHERE id = ?)", "os-scheduling").Scan(&exists)
	if err != nil {
		return err
	}
	if exists {
		_ = tx.Commit()
		return nil // Already seeded
	}

	// Insert topics
	topic1 := "os-scheduling"
	title1 := "Operating Systems: Scheduling"

	_, err = tx.Exec(`
		INSERT INTO topics (id, title, status)
		VALUES (?, ?, ?)
	`, topic1, title1, "reading")
	if err != nil {
		return err
	}

	if _, err = tx.Exec(`
		UPDATE topics
		SET start_page = ?, end_page = ?, current_page_cursor = ?
		WHERE id = ?
	`, 1, 10, 0, topic1); err != nil {
		return err
	}

	// Insert notebook for topic (required by flashcard generation in Sprint 1+)
	notebook1 := "os-notebook"
	_, err = tx.Exec(`
		INSERT INTO notebooks (id, title, file_path, file_type, status, page_count)
		VALUES (?, ?, ?, ?, ?, ?)
	`, notebook1, "OS Notebook", "/tmp/os.pdf", "pdf", "uploaded", 10)
	if err != nil {
		return err
	}

	// Link topic to notebook
	_, err = tx.Exec(`
		INSERT INTO notebook_topics (notebook_id, topic_id)
		VALUES (?, ?)
	`, notebook1, topic1)
	if err != nil {
		return err
	}

	// Create chunks
	chunks := []struct {
		id   string
		text string
	}{
		{
			"chunk-1",
			"Round Robin (RR) is a preemptive scheduling algorithm where each process is assigned a fixed time slice called a time quantum.",
		},
		{
			"chunk-2",
			"Each process in the ready queue gets a turn to execute for the duration of the time quantum.",
		},
		{
			"chunk-3",
			"If the process does not complete within its time quantum, it is moved to the back of the queue and the next process gets a turn.",
		},
		{
			"chunk-4",
			"Round Robin ensures fair allocation of CPU time among all processes with characteristics like fair share, good for time-sharing systems, and context switching overhead.",
		},
		{
			"chunk-5",
			"Round Robin advantages include fair distribution of CPU time, no process starvation, good for interactive systems, and simple implementation.",
		},
		{
			"chunk-6",
			"Round Robin disadvantages include high context switching overhead, performance dependency on time quantum, unsuitability for batch processing, and tradeoffs between fairness and quantum size.",
		},
	}

	for _, chunk := range chunks {
		pageNum := 0
		switch chunk.id {
		case "chunk-1":
			pageNum = 1
		case "chunk-2":
			pageNum = 2
		case "chunk-3":
			pageNum = 3
		case "chunk-4":
			pageNum = 4
		case "chunk-5":
			pageNum = 5
		case "chunk-6":
			pageNum = 6
		}
		_, err = tx.Exec(`
			INSERT INTO chunks (id, topic_id, chunk_text, page_num, token_count, importance_score, weakness_score)
			VALUES (?, ?, ?, ?, ?, ?, ?)
		`, chunk.id, topic1, chunk.text, pageNum, len(chunk.text)/4, 0.0, 0.0)
		if err != nil {
			return err
		}
	}

	// Link chunks to notebook via notebook_chunks table
	for _, chunk := range chunks {
		pageNum := 0
		switch chunk.id {
		case "chunk-1":
			pageNum = 1
		case "chunk-2":
			pageNum = 2
		case "chunk-3":
			pageNum = 3
		case "chunk-4":
			pageNum = 4
		case "chunk-5":
			pageNum = 5
		case "chunk-6":
			pageNum = 6
		}
		notebookChunkID := fmt.Sprintf("notebook-%s", chunk.id)
		_, err = tx.Exec(`
			INSERT INTO notebook_chunks (id, notebook_id, chunk_id, page_num)
			VALUES (?, ?, ?, ?)
		`, notebookChunkID, notebook1, chunk.id, pageNum)
		if err != nil {
			return err
		}
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		return err
	}
	tx = nil // Mark tx as committed

	return nil
}
