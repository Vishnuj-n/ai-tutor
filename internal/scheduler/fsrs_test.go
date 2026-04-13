package scheduler

import (
	"testing"

	"ai-tutor/internal/models"
)

func TestNextFSRSStateFirstReviewProducesValidStateForAllRatings(t *testing.T) {
	ratings := []int{Again, Hard, Good, Easy}
	for _, rating := range ratings {
		t.Run(ratingName(rating), func(t *testing.T) {
			state := NextFSRSState(models.FlashcardState{}, rating)
			if state.Reps != 1 {
				t.Fatalf("expected reps=1, got %d", state.Reps)
			}
			if state.Difficulty <= 0 {
				t.Fatalf("expected difficulty > 0, got %f", state.Difficulty)
			}
			if state.Stability <= 0 {
				t.Fatalf("expected stability > 0, got %f", state.Stability)
			}
			if rating == Again {
				if state.Lapses != 1 {
					t.Fatalf("expected lapses=1 for again, got %d", state.Lapses)
				}
			} else if state.Lapses != 0 {
				t.Fatalf("expected lapses=0 for first non-again review, got %d", state.Lapses)
			}
		})
	}
}

func TestNextFSRSStateRepeatedReviewsIncreaseRepsAndAgainIncrementsLapses(t *testing.T) {
	state := NextFSRSState(models.FlashcardState{}, Good)
	if state.Reps != 1 {
		t.Fatalf("expected reps after first review = 1, got %d", state.Reps)
	}

	next := NextFSRSState(state, Good)
	if next.Reps != 2 {
		t.Fatalf("expected reps after second review = 2, got %d", next.Reps)
	}
	if next.ScheduledDays <= 0 {
		t.Fatalf("expected positive scheduled_days after repeated good review, got %d", next.ScheduledDays)
	}

	lapsed := NextFSRSState(next, Again)
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
