package db

import (
	"encoding/json"
	"testing"

	"ai-tutor/internal/models"

	_ "github.com/mattn/go-sqlite3"
)

func TestUpdateFlashcardReviewTransactionalSave(t *testing.T) {
	initDBForTest(t, false, 0)

	cardID := "card-review-save"
	topicID := "topic-review-save"
	if err := testRepo.EnsureTopic(topicID, "Review Save Topic"); err != nil {
		t.Fatalf("EnsureTopic failed: %v", err)
	}

	initialState := models.FlashcardState{}
	if err := testRepo.CreateFlashcards(topicID, []models.Flashcard{{
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

	if err := testRepo.UpdateFlashcardReview(cardID, 200+3*86400, 100, string(beforeJSON), nextState, logRow); err != nil {
		t.Fatalf("UpdateFlashcardReview failed: %v", err)
	}

	card, state, err := testRepo.GetFlashcardByID(cardID)
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
	if err := testRepo.EnsureTopic(topicID, "Rollback Topic"); err != nil {
		t.Fatalf("EnsureTopic failed: %v", err)
	}

	originalState := models.FlashcardState{}
	if err := testRepo.CreateFlashcards(topicID, []models.Flashcard{{
		ID:        cardID,
		TopicID:   topicID,
		Prompt:    "Q",
		Answer:    "A",
		DueAt:     10,
		Suspended: false,
	}}, map[string]models.FlashcardState{cardID: originalState}); err != nil {
		t.Fatalf("CreateFlashcards failed: %v", err)
	}

	if _, err := testRepo.db.Exec(`
		INSERT INTO fsrs_review_log (
			id, topic_id, activity_type, reference_id, reviewed_at, rating,
			scheduled_days, state_before_json, state_after_json
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, "duplicate-log", topicID, "flashcard", cardID, 11, 3, 1, "{}", "{}"); err != nil {
		t.Fatalf("seed log failed: %v", err)
	}

	nextState := models.FlashcardState{Stability: 1, Difficulty: 5, ScheduledDays: 2, Reps: 1, StateCode: 2}
	origJSON, _ := json.Marshal(originalState)
	err := testRepo.UpdateFlashcardReview(cardID, 999, 10, string(origJSON), nextState, models.FSRSReviewLog{
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

	card, state, getErr := testRepo.GetFlashcardByID(cardID)
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

func TestQueryDueReviewCardsIgnoresSuspendedCards(t *testing.T) {
	initDBForTest(t, false, 0)

	topicID := "topic-suspend"
	if err := testRepo.EnsureTopic(topicID, "Suspend Topic"); err != nil {
		t.Fatalf("EnsureTopic failed: %v", err)
	}

	err := testRepo.CreateFlashcards(topicID, []models.Flashcard{
		{ID: "active-due", TopicID: topicID, Prompt: "Q1", Answer: "A1", DueAt: 100, Suspended: false},
		{ID: "suspended-due", TopicID: topicID, Prompt: "Q2", Answer: "A2", DueAt: 50, Suspended: true},
	}, map[string]models.FlashcardState{
		"active-due":    {},
		"suspended-due": {},
	})
	if err != nil {
		t.Fatalf("CreateFlashcards failed: %v", err)
	}

	count, err := testRepo.QueryDueReviewCards(200)
	if err != nil {
		t.Fatalf("QueryDueReviewCards failed: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected suspended card ignored, got count=%d", count)
	}
}

func TestQueryDueReviewCardsIgnoresOrphanedCards(t *testing.T) {
	initDBForTest(t, false, 0)

	topicID := "topic-orphan-check"
	orphanTopicID := "topic-orphan-deleted"
	if err := testRepo.EnsureTopic(topicID, "Active Topic"); err != nil {
		t.Fatalf("EnsureTopic active failed: %v", err)
	}
	if err := testRepo.EnsureTopic(orphanTopicID, "Orphan Topic"); err != nil {
		t.Fatalf("EnsureTopic orphan failed: %v", err)
	}

	// Create flashcards for both topics, both due
	err := testRepo.CreateFlashcards(topicID, []models.Flashcard{
		{ID: "active-card", TopicID: topicID, Prompt: "Q1", Answer: "A1", DueAt: 100, Suspended: false},
	}, map[string]models.FlashcardState{
		"active-card": {},
	})
	if err != nil {
		t.Fatalf("CreateFlashcards active failed: %v", err)
	}

	err = testRepo.CreateFlashcards(orphanTopicID, []models.Flashcard{
		{ID: "orphan-card", TopicID: orphanTopicID, Prompt: "Q2", Answer: "A2", DueAt: 50, Suspended: false},
	}, map[string]models.FlashcardState{
		"orphan-card": {},
	})
	if err != nil {
		t.Fatalf("CreateFlashcards orphan failed: %v", err)
	}

	// Verify both cards are due before deletion
	countBefore, err := testRepo.QueryDueReviewCards(200)
	if err != nil {
		t.Fatalf("QueryDueReviewCards before failed: %v", err)
	}
	if countBefore != 2 {
		t.Fatalf("expected 2 due cards before deletion, got %d", countBefore)
	}

	// Delete the orphan topic
	if _, err := testRepo.db.Exec(`DELETE FROM topics WHERE id = ?`, orphanTopicID); err != nil {
		t.Fatalf("failed to delete orphan topic: %v", err)
	}

	// Verify orphaned card is no longer counted
	countAfter, err := testRepo.QueryDueReviewCards(200)
	if err != nil {
		t.Fatalf("QueryDueReviewCards after failed: %v", err)
	}
	if countAfter != 1 {
		t.Fatalf("expected orphaned card excluded, got count=%d", countAfter)
	}
}

func TestGetNextDueReviewNotebookUsesPriorityAndLegacyTopicLink(t *testing.T) {
	initDBForTest(t, false, 0)

	if err := testRepo.EnsureTopic("topic-low", "Low Priority Topic"); err != nil {
		t.Fatalf("EnsureTopic low failed: %v", err)
	}
	if err := testRepo.EnsureTopic("topic-high", "High Priority Topic"); err != nil {
		t.Fatalf("EnsureTopic high failed: %v", err)
	}
	if err := testRepo.CreateNotebook("nb-low", "Low", "/tmp/low.pdf", "pdf", "topic-low", 10); err != nil {
		t.Fatalf("CreateNotebook low failed: %v", err)
	}
	if err := testRepo.CreateNotebook("nb-high", "High", "/tmp/high.pdf", "pdf", "", 10); err != nil {
		t.Fatalf("CreateNotebook high failed: %v", err)
	}
	if _, err := testRepo.db.Exec(`UPDATE notebooks SET priority = 9 WHERE id = 'nb-high'`); err != nil {
		t.Fatalf("update notebook priority failed: %v", err)
	}
	if _, err := testRepo.db.Exec(`INSERT INTO notebook_topics (notebook_id, topic_id) VALUES ('nb-high', 'topic-high')`); err != nil {
		t.Fatalf("link high topic failed: %v", err)
	}

	if err := testRepo.CreateFlashcards("topic-low", []models.Flashcard{
		{ID: "low-1", TopicID: "topic-low", Prompt: "Q1", Answer: "A1", DueAt: 100},
		{ID: "low-2", TopicID: "topic-low", Prompt: "Q2", Answer: "A2", DueAt: 100},
	}, map[string]models.FlashcardState{
		"low-1": {},
		"low-2": {},
	}); err != nil {
		t.Fatalf("CreateFlashcards low failed: %v", err)
	}
	if err := testRepo.CreateFlashcards("topic-high", []models.Flashcard{
		{ID: "high-1", TopicID: "topic-high", Prompt: "Q3", Answer: "A3", DueAt: 100},
		{ID: "high-2", TopicID: "topic-high", Prompt: "Q4", Answer: "A4", DueAt: 100},
	}, map[string]models.FlashcardState{
		"high-1": {},
		"high-2": {},
	}); err != nil {
		t.Fatalf("CreateFlashcards high failed: %v", err)
	}

	notebookID, dueCount, err := testRepo.GetNextDueReviewNotebook(200)
	if err != nil {
		t.Fatalf("GetNextDueReviewNotebook failed: %v", err)
	}
	if notebookID != "nb-high" {
		t.Fatalf("expected higher-priority notebook selected on tie, got %s", notebookID)
	}
	if dueCount != 2 {
		t.Fatalf("expected dueCount=2, got %d", dueCount)
	}
}

func TestQueryDueReviewCardsForRange(t *testing.T) {
	initDBForTest(t, false, 0)

	topicID := "topic-range"
	if err := testRepo.EnsureTopic(topicID, "Range Topic"); err != nil {
		t.Fatalf("EnsureTopic failed: %v", err)
	}

	err := testRepo.CreateFlashcards(topicID, []models.Flashcard{
		{ID: "card-1", TopicID: topicID, Prompt: "Q1", Answer: "A1", DueAt: 50, Suspended: false},
		{ID: "card-2", TopicID: topicID, Prompt: "Q2", Answer: "A2", DueAt: 150, Suspended: false},
		{ID: "card-3", TopicID: topicID, Prompt: "Q3", Answer: "A3", DueAt: 250, Suspended: false},
		{ID: "card-4", TopicID: topicID, Prompt: "Q4", Answer: "A4", DueAt: 150, Suspended: true}, // Suspended
	}, map[string]models.FlashcardState{
		"card-1": {},
		"card-2": {},
		"card-3": {},
		"card-4": {},
	})
	if err != nil {
		t.Fatalf("CreateFlashcards failed: %v", err)
	}

	// 1. Verify query within (0, 100] -> should find card-1 only
	count, err := testRepo.QueryDueReviewCardsForRange(0, 100)
	if err != nil {
		t.Fatalf("QueryDueReviewCardsForRange failed: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 card due in (0, 100], got %d", count)
	}

	// 2. Verify query within (100, 200] -> should find card-2 only (card-4 is suspended)
	count, err = testRepo.QueryDueReviewCardsForRange(100, 200)
	if err != nil {
		t.Fatalf("QueryDueReviewCardsForRange failed: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 active card due in (100, 200], got %d", count)
	}

	// 3. Verify query within (0, 300] -> should find card-1, card-2, card-3 (suspended ignored)
	count, err = testRepo.QueryDueReviewCardsForRange(0, 300)
	if err != nil {
		t.Fatalf("QueryDueReviewCardsForRange failed: %v", err)
	}
	if count != 3 {
		t.Errorf("expected 3 cards due in (0, 300], got %d", count)
	}
}

