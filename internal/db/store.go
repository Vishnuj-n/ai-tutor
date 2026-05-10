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
	conn, err = sql.Open("sqlite3", "file:"+dbPath+"?_foreign_keys=on")
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

func GetFlashcardByIDTx(tx *sql.Tx, cardID string) (*models.Flashcard, *models.FlashcardState, error) {
	cardID = strings.TrimSpace(cardID)
	if cardID == "" {
		return nil, nil, fmt.Errorf("flashcard id is required")
	}
	return getFlashcardByIDRepoTx(tx, cardID)
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

func UpdateFlashcardReviewTx(tx *sql.Tx, cardID string, dueAt int64, expectedDueAt int64, state models.FlashcardState, reviewLog models.FSRSReviewLog) error {
	cardID = strings.TrimSpace(cardID)
	if cardID == "" {
		return fmt.Errorf("flashcard id is required")
	}
	if dueAt <= 0 {
		return fmt.Errorf("due time is required")
	}
	return updateFlashcardReviewRepoTx(tx, cardID, dueAt, expectedDueAt, state, reviewLog)
}

func GetExistingReviewTaskForNotebook(notebookID string) (*models.StudyQueueTask, error) {
	notebookID = strings.TrimSpace(notebookID)
	if notebookID == "" {
		return nil, fmt.Errorf("notebook id is required")
	}
	return getExistingReviewTaskForNotebookRepo(notebookID)
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
func GetAssessmentFSRSState(activityType, referenceID, sourceChunkID string) (*AssessmentFSRSRecord, error) {
	activityType = strings.TrimSpace(activityType)
	referenceID = strings.TrimSpace(referenceID)
	if activityType == "" || referenceID == "" {
		return nil, fmt.Errorf("activity type and reference id are required")
	}
	return getAssessmentFSRSStateRepo(activityType, referenceID, sourceChunkID)
}

// GetAssessmentFSRSStateTx returns shared assessment FSRS state for one quiz/written reference within a transaction.
func GetAssessmentFSRSStateTx(tx *sql.Tx, activityType, referenceID, sourceChunkID string) (*AssessmentFSRSRecord, error) {
	activityType = strings.TrimSpace(activityType)
	referenceID = strings.TrimSpace(referenceID)
	if activityType == "" || referenceID == "" {
		return nil, fmt.Errorf("activity type and reference id are required")
	}
	return getAssessmentFSRSStateRepoTx(tx, activityType, referenceID, sourceChunkID)
}

// UpsertAssessmentFSRSReview saves shared assessment FSRS state and corresponding review log.
func UpsertAssessmentFSRSReview(activityType, referenceID, topicID, sourceChunkID string, state models.FlashcardState, dueAt, reviewedAt int64, reviewLog models.FSRSReviewLog) error {
	activityType = strings.TrimSpace(activityType)
	referenceID = strings.TrimSpace(referenceID)
	topicID = strings.TrimSpace(topicID)
	if activityType == "" || referenceID == "" || topicID == "" {
		return fmt.Errorf("activity type, reference id, and topic id are required")
	}
	return upsertAssessmentFSRSReviewRepo(activityType, referenceID, topicID, sourceChunkID, state, dueAt, reviewedAt, reviewLog)
}

// UpsertAssessmentFSRSReviewTx saves shared assessment FSRS state and corresponding review log within a transaction.
func UpsertAssessmentFSRSReviewTx(tx *sql.Tx, activityType, referenceID, topicID, sourceChunkID string, state models.FlashcardState, dueAt, reviewedAt int64, reviewLog models.FSRSReviewLog) error {
	activityType = strings.TrimSpace(activityType)
	referenceID = strings.TrimSpace(referenceID)
	topicID = strings.TrimSpace(topicID)
	if activityType == "" || referenceID == "" || topicID == "" {
		return fmt.Errorf("activity type, reference id, and topic id are required")
	}
	return upsertAssessmentFSRSReviewRepoTx(tx, activityType, referenceID, topicID, sourceChunkID, state, dueAt, reviewedAt, reviewLog)
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

func SaveQuizAttempt(attempt models.QuizAttemptRecord) error {
	attempt.ID = strings.TrimSpace(attempt.ID)
	attempt.TaskID = strings.TrimSpace(attempt.TaskID)
	attempt.AnswersJSON = strings.TrimSpace(attempt.AnswersJSON)
	if attempt.ID == "" {
		return fmt.Errorf("attempt id is required")
	}
	if attempt.TaskID == "" {
		return fmt.Errorf("task id is required")
	}
	if attempt.AnswersJSON == "" {
		return fmt.Errorf("answers json is required")
	}
	if attempt.CompletedAt <= 0 {
		return fmt.Errorf("completed at is required")
	}
	return saveQuizAttemptRepo(attempt)
}

func SaveQuizAttemptTx(tx *sql.Tx, attempt models.QuizAttemptRecord) error {
	attempt.ID = strings.TrimSpace(attempt.ID)
	attempt.TaskID = strings.TrimSpace(attempt.TaskID)
	attempt.AnswersJSON = strings.TrimSpace(attempt.AnswersJSON)
	if attempt.ID == "" {
		return fmt.Errorf("attempt id is required")
	}
	if attempt.TaskID == "" {
		return fmt.Errorf("task id is required")
	}
	if attempt.AnswersJSON == "" {
		return fmt.Errorf("answers json is required")
	}
	if attempt.CompletedAt <= 0 {
		return fmt.Errorf("completed at is required")
	}
	return saveQuizAttemptRepoTx(tx, attempt)
}
