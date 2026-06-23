package db

import (
	"encoding/json"
	"math"
	"testing"
	"time"

	"ai-tutor/internal/models"

	fsrs "github.com/open-spaced-repetition/go-fsrs/v4"
)

// computeNextState is the test-local equivalent of scheduler.NextFSRSState,
// avoiding the import cycle db → scheduler → db.
func computeNextState(state models.FlashcardState, rating int, now time.Time, dueAt, lastReviewedAt int64) (models.FlashcardState, error) {
	p := fsrs.DefaultParam()
	p.RequestRetention = 0.9
	engine := fsrs.NewFSRS(p)

	fsrsCard := models.FlashcardStateToCard(state, dueAt, lastReviewedAt)
	if state.Reps == 0 || fsrsCard.Due.IsZero() {
		fsrsCard.Due = now
	}

	schedulingCards, err := engine.Repeat(fsrsCard, now)
	if err != nil {
		return state, err
	}

	chosenRecord := schedulingCards[fsrs.Rating(rating)]
	updatedState := models.CardToFlashcardState(chosenRecord.Card)

	if !fsrsCard.LastReview.IsZero() {
		elapsedDays := int(now.Sub(fsrsCard.LastReview).Hours() / 24)
		if elapsedDays < 0 {
			elapsedDays = 0
		}
		updatedState.ElapsedDays = elapsedDays
	} else if lastReviewedAt > 0 {
		elapsedSeconds := now.Unix() - lastReviewedAt
		if elapsedSeconds > 0 {
			updatedState.ElapsedDays = int(elapsedSeconds / (24 * 60 * 60))
		}
	}

	return updatedState, nil
}

// reviewCard performs one FSRS review with the given rating, persists it, and returns the new due_at.
func reviewCard(t *testing.T, repo *Repository, cardID, topicID string, rating fsrs.Rating, simNow time.Time, currentDueAt int64) int64 {
	t.Helper()

	card, state, err := repo.GetFlashcardByID(cardID)
	if err != nil {
		t.Fatalf("GetFlashcardByID failed: %v", err)
	}
	if card == nil || state == nil {
		t.Fatalf("card %s vanished", cardID)
	}

	lastReviewedAt, err := repo.GetLastFlashcardReviewTime(cardID)
	if err != nil {
		t.Fatalf("GetLastFlashcardReviewTime failed: %v", err)
	}

	newState, err := computeNextState(*state, int(rating), simNow, currentDueAt, lastReviewedAt)
	if err != nil {
		t.Fatalf("computeNextState failed: %v", err)
	}

	newDueAt := simNow.Add(time.Duration(newState.ScheduledDays) * 24 * time.Hour).Unix()
	stateBeforeJSON, _ := json.Marshal(*state)

	reviewLog := models.FSRSReviewLog{
		ID:              cardID + "-rev-" + simNow.Format("20060102"),
		TopicID:         topicID,
		ActivityType:    "flashcard",
		ReferenceID:     cardID,
		ReviewedAt:      simNow.Unix(),
		Rating:          int(rating),
		ScheduledDays:   newState.ScheduledDays,
		StateBeforeJSON: string(stateBeforeJSON),
	}

	if err := repo.UpdateFlashcardReview(cardID, newDueAt, currentDueAt, string(stateBeforeJSON), newState, reviewLog); err != nil {
		t.Fatalf("UpdateFlashcardReview failed: %v", err)
	}

	return newDueAt
}

// TestFSRS365DaySimulation runs a 365-day time-travel simulation of the FSRS
// spaced repetition algorithm. Each simulated day, the card is reviewed with
// Good if due. Intervals must grow monotonically, and no edge cases
// (explosion, permanent short-loops, integer overflows) should occur.
func TestFSRS365DaySimulation(t *testing.T) {
	repo, err := Init(":memory:", "")
	if err != nil {
		t.Fatalf("Init(:memory:) failed: %v", err)
	}
	defer func() {
		if err := repo.Close(); err != nil {
			t.Fatalf("Close failed: %v", err)
		}
	}()

	topicID := "sim-topic-1"
	if err := repo.EnsureTopic(topicID, "Simulation Topic"); err != nil {
		t.Fatalf("EnsureTopic failed: %v", err)
	}

	cardID := "sim-card-1"
	cards := []models.Flashcard{
		{ID: cardID, TopicID: topicID, Prompt: "What is FSRS?", Answer: "A spaced repetition algorithm"},
	}
	states := map[string]models.FlashcardState{cardID: {}}
	if err := repo.CreateFlashcards(topicID, cards, states); err != nil {
		t.Fatalf("CreateFlashcards failed: %v", err)
	}

	startTime := time.Date(2026, 1, 1, 9, 0, 0, 0, time.UTC)
	var intervals []int
	var lastReviewTime time.Time
	var totalReviews int
	const maxAcceptableInterval = 3652

	for day := 0; day < 365; day++ {
		simNow := startTime.Add(time.Duration(day) * 24 * time.Hour)

		card, state, err := repo.GetFlashcardByID(cardID)
		if err != nil {
			t.Fatalf("GetFlashcardByID failed on day %d: %v", day, err)
		}

		if state.Reps == 0 || card.DueAt <= simNow.Unix() {
			reviewCard(t, repo, cardID, topicID, fsrs.Good, simNow, card.DueAt)

			if !lastReviewTime.IsZero() {
				interval := int(simNow.Sub(lastReviewTime).Hours() / 24)
				if interval > 0 {
					intervals = append(intervals, interval)
				}
			}
			lastReviewTime = simNow
			totalReviews++
		}
	}

	t.Logf("Total reviews over 365 days: %d", totalReviews)
	t.Logf("Interval count: %d", len(intervals))

	if totalReviews == 0 {
		t.Fatal("expected at least one review over 365 days")
	}

	// Verify final state
	finalCard, finalState, err := repo.GetFlashcardByID(cardID)
	if err != nil {
		t.Fatalf("GetFlashcardByID (final) failed: %v", err)
	}
	if finalCard == nil || finalState == nil {
		t.Fatal("final card or state is nil")
	}
	t.Logf("Final state: reps=%d, stability=%.2f, difficulty=%.2f, scheduled_days=%d, lapses=%d, state_code=%d",
		finalState.Reps, finalState.Stability, finalState.Difficulty, finalState.ScheduledDays, finalState.Lapses, finalState.StateCode)

	// Assert: reps match total review count
	if finalState.Reps != totalReviews {
		t.Errorf("final reps=%d but totalReviews=%d", finalState.Reps, totalReviews)
	}

	// Assert: stability must be positive after reviews
	if finalState.Stability <= 0 {
		t.Errorf("final stability=%.2f, expected > 0", finalState.Stability)
	}

	// Assert: no integer overflow in scheduled_days
	if finalState.ScheduledDays < 0 {
		t.Errorf("scheduled_days=%d is negative (integer overflow?)", finalState.ScheduledDays)
	}

	// Assert: scheduled_days within reasonable bounds
	if finalState.ScheduledDays > maxAcceptableInterval {
		t.Errorf("scheduled_days=%d exceeds max acceptable interval %d (explosion?)", finalState.ScheduledDays, maxAcceptableInterval)
	}

	// Assert: review logs exist
	logs, err := repo.GetRecentReviewLogs(1000)
	if err != nil {
		t.Fatalf("GetRecentReviewLogs failed: %v", err)
	}
	if len(logs) != totalReviews {
		t.Errorf("expected %d review logs, got %d", totalReviews, len(logs))
	}

	// Assert: intervals grow monotonically (non-decreasing) after initial learning phase
	if len(intervals) > 5 {
		stableStart := 5
		for i := stableStart; i < len(intervals); i++ {
			if intervals[i] < intervals[i-1] {
				t.Errorf("interval regression at index %d: interval[%d]=%d < interval[%d]=%d",
					i, i, intervals[i], i-1, intervals[i-1])
			}
		}
	}

	// Assert: no permanent short-loops
	if len(intervals) > 10 {
		lastTen := intervals[len(intervals)-10:]
		allTiny := true
		for _, iv := range lastTen {
			if iv > 1 {
				allTiny = false
				break
			}
		}
		if allTiny {
			t.Errorf("detected permanent short-loop: last 10 intervals are all <= 1 day")
		}
	}

	// Assert: no interval explosion
	for i, iv := range intervals {
		if iv > maxAcceptableInterval {
			t.Errorf("interval explosion at index %d: interval=%d days exceeds max %d", i, iv, maxAcceptableInterval)
		}
	}

	// Assert: stability didn't explode to infinity
	if math.IsInf(finalState.Stability, 0) || math.IsNaN(finalState.Stability) {
		t.Errorf("stability is Inf or NaN: %f", finalState.Stability)
	}
	if finalState.Stability > 1e6 {
		t.Errorf("stability=%.2f is unreasonably large (>1M)", finalState.Stability)
	}

	if len(intervals) > 0 {
		t.Logf("Interval range: min=%d, max=%d days", intervals[0], intervals[len(intervals)-1])
		if len(intervals) > 1 {
			t.Logf("Last 5 intervals: %v", intervals[max(0, len(intervals)-5):])
		}
	}
}

// TestFSRS365DayMixedRatings runs the same 365-day simulation with a mix of
// ratings (Good, Hard, Again) to test realistic usage patterns.
func TestFSRS365DayMixedRatings(t *testing.T) {
	repo, err := Init(":memory:", "")
	if err != nil {
		t.Fatalf("Init(:memory:) failed: %v", err)
	}
	defer func() {
		if err := repo.Close(); err != nil {
			t.Fatalf("Close failed: %v", err)
		}
	}()

	topicID := "sim-topic-mixed"
	if err := repo.EnsureTopic(topicID, "Mixed Rating Topic"); err != nil {
		t.Fatalf("EnsureTopic failed: %v", err)
	}

	cardID := "sim-card-mixed"
	cards := []models.Flashcard{
		{ID: cardID, TopicID: topicID, Prompt: "What is 2+2?", Answer: "4"},
	}
	states := map[string]models.FlashcardState{cardID: {}}
	if err := repo.CreateFlashcards(topicID, cards, states); err != nil {
		t.Fatalf("CreateFlashcards failed: %v", err)
	}

	startTime := time.Date(2026, 1, 1, 9, 0, 0, 0, time.UTC)
	var totalReviews int

	for day := 0; day < 365; day++ {
		simNow := startTime.Add(time.Duration(day) * 24 * time.Hour)

		card, state, err := repo.GetFlashcardByID(cardID)
		if err != nil {
			t.Fatalf("GetFlashcardByID failed on day %d: %v", day, err)
		}

		if state.Reps == 0 || card.DueAt <= simNow.Unix() {
			rating := fsrs.Good
			switch {
			case day%10 == 0:
				rating = fsrs.Again
			case day%5 == 0:
				rating = fsrs.Hard
			}

			reviewCard(t, repo, cardID, topicID, rating, simNow, card.DueAt)
			totalReviews++
		}
	}

	t.Logf("Mixed ratings: total reviews=%d", totalReviews)

	finalCard, finalState, err := repo.GetFlashcardByID(cardID)
	if err != nil {
		t.Fatalf("final GetFlashcardByID failed: %v", err)
	}
	if finalCard == nil || finalState == nil {
		t.Fatal("final card/state nil")
	}

	t.Logf("Final: reps=%d, stability=%.2f, difficulty=%.2f, scheduled_days=%d, lapses=%d",
		finalState.Reps, finalState.Stability, finalState.Difficulty, finalState.ScheduledDays, finalState.Lapses)

	// Core invariants must hold even with mixed ratings
	if finalState.Stability <= 0 {
		t.Errorf("stability=%.2f, expected > 0", finalState.Stability)
	}
	if finalState.ScheduledDays < 0 {
		t.Errorf("scheduled_days=%d is negative", finalState.ScheduledDays)
	}
	if finalState.ScheduledDays > 3652 {
		t.Errorf("scheduled_days=%d exceeds 10 years", finalState.ScheduledDays)
	}
	if math.IsInf(finalState.Stability, 0) || math.IsNaN(finalState.Stability) {
		t.Errorf("stability is Inf/NaN: %f", finalState.Stability)
	}
	if finalState.Reps != totalReviews {
		t.Errorf("reps=%d != totalReviews=%d", finalState.Reps, totalReviews)
	}

	logs, err := repo.GetRecentReviewLogs(5000)
	if err != nil {
		t.Fatalf("GetRecentReviewLogs failed: %v", err)
	}
	if len(logs) != totalReviews {
		t.Errorf("review log count=%d != totalReviews=%d", len(logs), totalReviews)
	}
}

// TestFSRS365DayAllEasy tests the upper-bound scenario where the user
// always answers Easy, ensuring no runaway interval growth.
func TestFSRS365DayAllEasy(t *testing.T) {
	repo, err := Init(":memory:", "")
	if err != nil {
		t.Fatalf("Init(:memory:) failed: %v", err)
	}
	defer func() {
		if err := repo.Close(); err != nil {
			t.Fatalf("Close failed: %v", err)
		}
	}()

	topicID := "sim-topic-easy"
	if err := repo.EnsureTopic(topicID, "All Easy Topic"); err != nil {
		t.Fatalf("EnsureTopic failed: %v", err)
	}

	cardID := "sim-card-easy"
	cards := []models.Flashcard{
		{ID: cardID, TopicID: topicID, Prompt: "Easy Q?", Answer: "Easy A"},
	}
	states := map[string]models.FlashcardState{cardID: {}}
	if err := repo.CreateFlashcards(topicID, cards, states); err != nil {
		t.Fatalf("CreateFlashcards failed: %v", err)
	}

	startTime := time.Date(2026, 1, 1, 9, 0, 0, 0, time.UTC)

	for day := 0; day < 365; day++ {
		simNow := startTime.Add(time.Duration(day) * 24 * time.Hour)
		card, state, err := repo.GetFlashcardByID(cardID)
		if err != nil {
			t.Fatalf("GetFlashcardByID failed on day %d: %v", day, err)
		}

		if state.Reps == 0 || card.DueAt <= simNow.Unix() {
			reviewCard(t, repo, cardID, topicID, fsrs.Easy, simNow, card.DueAt)
		}
	}

	finalCard, finalState, err := repo.GetFlashcardByID(cardID)
	if err != nil {
		t.Fatalf("final GetFlashcardByID failed: %v", err)
	}
	if finalCard == nil || finalState == nil {
		t.Fatal("final card/state nil")
	}

	t.Logf("All Easy final: reps=%d, stability=%.2f, scheduled_days=%d",
		finalState.Reps, finalState.Stability, finalState.ScheduledDays)

	if finalState.ScheduledDays > 3652 {
		t.Errorf("all-easy scheduled_days=%d exceeds 10 years (interval explosion)", finalState.ScheduledDays)
	}
	if finalState.Stability > 1e6 {
		t.Errorf("all-easy stability=%.2f is unreasonably large", finalState.Stability)
	}
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
