package scheduler

import (
	"testing"
	"time"

	"ai-tutor/internal/models"
)

func mustNextState(t *testing.T, state models.FlashcardState, rating int, now time.Time, dueAt, lastReviewedAt int64) models.FlashcardState {
	t.Helper()
	s, err := NextFSRSState(state, rating, now, dueAt, lastReviewedAt)
	if err != nil {
		t.Fatalf("NextFSRSState failed: %v", err)
	}
	return s
}

func TestNextFSRSStateFirstReviewProducesValidStateForAllRatings(t *testing.T) {
	ratings := []int{Again, Hard, Good, Easy}
	for _, rating := range ratings {
		t.Run(ratingName(rating), func(t *testing.T) {
			state := mustNextState(t, models.FlashcardState{}, rating, time.Now(), 0, 0)
			if state.Reps != 1 {
				t.Fatalf("expected reps=1, got %d", state.Reps)
			}
			if state.Difficulty <= 0 {
				t.Fatalf("expected difficulty > 0, got %f", state.Difficulty)
			}
			if state.Stability <= 0 {
				t.Fatalf("expected stability > 0, got %f", state.Stability)
			}
			if state.Lapses != 0 {
				t.Fatalf("expected lapses=0 for first review, got %d", state.Lapses)
			}
		})
	}
}

func TestNextFSRSStateRepeatedReviewsIncreaseRepsAndAgainIncrementsLapses(t *testing.T) {
	t0 := time.Now()
	state := mustNextState(t, models.FlashcardState{}, Good, t0, 0, 0)
	if state.Reps != 1 {
		t.Fatalf("expected reps after first review = 1, got %d", state.Reps)
	}

	t1 := t0.Add(24 * time.Hour)
	next := mustNextState(t, state, Good, t1, t0.Unix(), t0.Unix())
	if next.Reps != 2 {
		t.Fatalf("expected reps after second review = 2, got %d", next.Reps)
	}
	if next.ScheduledDays <= 0 {
		t.Fatalf("expected positive scheduled_days after repeated good review, got %d", next.ScheduledDays)
	}

	t2 := t1.Add(24 * time.Hour)
	lapsed := mustNextState(t, next, Again, t2, t1.Unix(), t1.Unix())
	if lapsed.Reps != 3 {
		t.Fatalf("expected reps after again = 3, got %d", lapsed.Reps)
	}
	if lapsed.Lapses != 1 {
		t.Fatalf("expected lapses=1 after again, got %d", lapsed.Lapses)
	}
}

func ratingName(rating int) string {
	switch rating {
	case Again:
		return "again"
	case Hard:
		return "hard"
	case Good:
		return "good"
	case Easy:
		return "easy"
	default:
		return "unknown"
	}
}
