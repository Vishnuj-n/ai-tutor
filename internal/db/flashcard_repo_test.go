package db

import (
	"database/sql"
	"encoding/json"
	"path/filepath"
	"testing"

	"ai-tutor/internal/models"

	_ "github.com/mattn/go-sqlite3"
)

func TestUpdateFlashcardReviewTransactionalSave(t *testing.T) {
	initDBForTest(t, false, 0)

	cardID := "card-review-save"
	topicID := "topic-review-save"
	if err := EnsureTopic(topicID, "Review Save Topic"); err != nil {
		t.Fatalf("EnsureTopic failed: %v", err)
	}

	initialState := models.FlashcardState{}
	if err := CreateFlashcards(topicID, []models.Flashcard{{
		ID:        cardID,
		TopicID:   topicID,
		Prompt:    "Q",
		Answer:    "A",
		DueAt:     100,
		Suspended: false,
	}}, map[string]models.FlashcardState{cardID: initialState}); err != nil {
		t.Fatalf("CreateFlashcards failed: %v", err)
	}

	nextState := models.FlashcardState{
		Stability:     1.2,
		Difficulty:    5.5,
		ElapsedDays:   0,
		ScheduledDays: 3,
		Reps:          1,
		Lapses:        0,
		StateCode:     2,
	}
	beforeJSON, _ := json.Marshal(initialState)
	afterJSON, _ := json.Marshal(nextState)
	logRow := models.FSRSReviewLog{
		ID:              "log-review-save",
		TopicID:         topicID,
		ActivityType:    "flashcard",
		ReferenceID:     cardID,
		ReviewedAt:      200,
		Rating:          3,
		ScheduledDays:   3,
		StateBeforeJSON: string(beforeJSON),
		StateAfterJSON:  string(afterJSON),
	}

	if err := UpdateFlashcardReview(cardID, 200+3*86400, 100, nextState, logRow); err != nil {
		t.Fatalf("UpdateFlashcardReview failed: %v", err)
	}

	card, state, err := GetFlashcardByID(cardID)
	if err != nil {
		t.Fatalf("GetFlashcardByID failed: %v", err)
	}
	if card.DueAt != 200+3*86400 {
		t.Fatalf("unexpected due_at: got=%d", card.DueAt)
	}
	if state.Reps != 1 || state.ScheduledDays != 3 {
		t.Fatalf("unexpected persisted state: %#v", state)
	}

	assertCountEquals(t, `SELECT COUNT(*) FROM fsrs_review_log WHERE reference_id = ?`, cardID, 1)
}

func TestUpdateFlashcardReviewRollsBackCardOnLogInsertFailure(t *testing.T) {
	initDBForTest(t, false, 0)

	cardID := "card-review-rollback"
	topicID := "topic-review-rollback"
	if err := EnsureTopic(topicID, "Rollback Topic"); err != nil {
		t.Fatalf("EnsureTopic failed: %v", err)
	}

	originalState := models.FlashcardState{}
	if err := CreateFlashcards(topicID, []models.Flashcard{{
		ID:        cardID,
		TopicID:   topicID,
		Prompt:    "Q",
		Answer:    "A",
		DueAt:     10,
		Suspended: false,
	}}, map[string]models.FlashcardState{cardID: originalState}); err != nil {
		t.Fatalf("CreateFlashcards failed: %v", err)
	}

	if _, err := conn.Exec(`
		INSERT INTO fsrs_review_log (
			id, topic_id, activity_type, reference_id, reviewed_at, rating,
			scheduled_days, state_before_json, state_after_json
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, "duplicate-log", topicID, "flashcard", cardID, 11, 3, 1, "{}", "{}"); err != nil {
		t.Fatalf("seed log failed: %v", err)
	}

	nextState := models.FlashcardState{Stability: 1, Difficulty: 5, ScheduledDays: 2, Reps: 1, StateCode: 2}
	err := UpdateFlashcardReview(cardID, 999, 10, nextState, models.FSRSReviewLog{
		ID:              "duplicate-log",
		TopicID:         topicID,
		ActivityType:    "flashcard",
		ReferenceID:     cardID,
		ReviewedAt:      12,
		Rating:          4,
		ScheduledDays:   2,
		StateBeforeJSON: "{}",
		StateAfterJSON:  "{}",
	})
	if err == nil {
		t.Fatalf("expected duplicate log insert to fail")
	}

	card, state, getErr := GetFlashcardByID(cardID)
	if getErr != nil {
		t.Fatalf("GetFlashcardByID failed: %v", getErr)
	}
	if card.DueAt != 10 {
		t.Fatalf("expected card due_at rollback to original value, got %d", card.DueAt)
	}
	if state.Reps != 0 {
		t.Fatalf("expected state rollback, got %#v", state)
	}
}

func TestEnsureFSRSSchemaPreservesCardsWhenLogMissing(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "legacy-fsrs.sqlite")

	rawConn, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("sql.Open failed: %v", err)
	}
	defer func() {
		_ = rawConn.Close()
	}()

	legacySchema := `
		CREATE TABLE topics (id TEXT PRIMARY KEY, title TEXT NOT NULL);
		CREATE TABLE fsrs_cards (
			id TEXT PRIMARY KEY,
			topic_id TEXT NOT NULL,
			prompt TEXT NOT NULL,
			answer TEXT NOT NULL,
			state_json TEXT,
			due_at TEXT
		);
		INSERT INTO topics (id, title) VALUES ('legacy-topic', 'Legacy Topic');
		INSERT INTO fsrs_cards (id, topic_id, prompt, answer, state_json, due_at)
		VALUES ('legacy-card', 'legacy-topic', 'Old Q', 'Old A', '{}', 'old-ts');
	`
	if _, err := rawConn.Exec(legacySchema); err != nil {
		t.Fatalf("failed to seed legacy schema: %v", err)
	}
	_ = rawConn.Close()

	if err := Init(dbPath, ""); err != nil {
		t.Fatalf("Init failed: %v", err)
	}
	defer func() {
		if err := Close(); err != nil {
			t.Fatalf("Close failed: %v", err)
		}
	}()

	var cardsCount int
	if err := conn.QueryRow(`SELECT COUNT(*) FROM fsrs_cards`).Scan(&cardsCount); err != nil {
		t.Fatalf("count fsrs_cards failed: %v", err)
	}
	if cardsCount != 1 {
		t.Fatalf("expected non-destructive migration to preserve fsrs_cards rows, got %d rows", cardsCount)
	}

	var logTableCount int
	if err := conn.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = 'fsrs_review_log'`).Scan(&logTableCount); err != nil {
		t.Fatalf("count fsrs_review_log table failed: %v", err)
	}
	if logTableCount != 1 {
		t.Fatalf("expected fsrs_review_log table to exist, got %d", logTableCount)
	}
}

func TestQueryDueReviewCardsIgnoresSuspendedCards(t *testing.T) {
	initDBForTest(t, false, 0)

	topicID := "topic-suspend"
	if err := EnsureTopic(topicID, "Suspend Topic"); err != nil {
		t.Fatalf("EnsureTopic failed: %v", err)
	}

	err := CreateFlashcards(topicID, []models.Flashcard{
		{ID: "active-due", TopicID: topicID, Prompt: "Q1", Answer: "A1", DueAt: 100, Suspended: false},
		{ID: "suspended-due", TopicID: topicID, Prompt: "Q2", Answer: "A2", DueAt: 50, Suspended: true},
	}, map[string]models.FlashcardState{
		"active-due":    {},
		"suspended-due": {},
	})
	if err != nil {
		t.Fatalf("CreateFlashcards failed: %v", err)
	}

	count, err := QueryDueReviewCards(200)
	if err != nil {
		t.Fatalf("QueryDueReviewCards failed: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected suspended card ignored, got count=%d", count)
	}
}
