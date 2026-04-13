package scheduler

import (
	"math"

	"ai-tutor/internal/models"
)

const (
	Again = 1
	Hard  = 2
	Good  = 3
	Easy  = 4
)

const (
	FSRSStateNew        = 0
	FSRSStateLearning   = 1
	FSRSStateReview     = 2
	FSRSStateRelearning = 3
)

// NextFSRSState applies one pure FSRS-style transition.
func NextFSRSState(current models.FlashcardState, rating int) models.FlashcardState {
	state := normalizeFSRSState(current)

	elapsedDays := state.ScheduledDays
	if state.Reps == 0 {
		elapsedDays = 0
	}
	state.ElapsedDays = maxInt(0, elapsedDays)
	state.Reps++

	if rating == Again {
		state.Lapses++
	}

	if state.Reps == 1 {
		return firstReviewState(state, rating)
	}
	return nextReviewState(state, rating)
}

func normalizeFSRSState(state models.FlashcardState) models.FlashcardState {
	if state.Difficulty <= 0 {
		state.Difficulty = 5
	}
	if state.Stability < 0 {
		state.Stability = 0
	}
	if state.StateCode < FSRSStateNew || state.StateCode > FSRSStateRelearning {
		state.StateCode = FSRSStateNew
	}
	return state
}

func firstReviewState(state models.FlashcardState, rating int) models.FlashcardState {
	switch rating {
	case Again:
		state.Stability = 0.1
		state.Difficulty = 7.5
		state.ScheduledDays = 0
		state.StateCode = FSRSStateLearning
	case Hard:
		state.Stability = 0.5
		state.Difficulty = 6.5
		state.ScheduledDays = 1
		state.StateCode = FSRSStateLearning
	case Good:
		state.Stability = 1.2
		state.Difficulty = 5.5
		state.ScheduledDays = 3
		state.StateCode = FSRSStateReview
	case Easy:
		state.Stability = 2.5
		state.Difficulty = 4.5
		state.ScheduledDays = 5
		state.StateCode = FSRSStateReview
	default:
		return firstReviewState(state, Good)
	}
	return state
}

func nextReviewState(state models.FlashcardState, rating int) models.FlashcardState {
	retrievability := estimateRetrievability(state.ElapsedDays, state.Stability)

	switch rating {
	case Again:
		state.Difficulty = clampFloat(state.Difficulty+0.8, 1, 10)
		state.Stability = clampFloat(state.Stability*0.45, 0.1, 36500)
		state.ScheduledDays = 1
		state.StateCode = FSRSStateRelearning
	case Hard:
		state.Difficulty = clampFloat(state.Difficulty+0.15, 1, 10)
		state.Stability = clampFloat(state.Stability*hardStabilityFactor(retrievability), 0.1, 36500)
		state.ScheduledDays = maxInt(1, int(math.Round(state.Stability*1.2)))
		state.StateCode = FSRSStateReview
	case Good:
		state.Difficulty = clampFloat(state.Difficulty-0.1, 1, 10)
		state.Stability = clampFloat(state.Stability*goodStabilityFactor(retrievability), 0.1, 36500)
		state.ScheduledDays = maxInt(1, int(math.Round(state.Stability)))
		state.StateCode = FSRSStateReview
	case Easy:
		state.Difficulty = clampFloat(state.Difficulty-0.25, 1, 10)
		state.Stability = clampFloat(state.Stability*easyStabilityFactor(retrievability), 0.1, 36500)
		state.ScheduledDays = maxInt(2, int(math.Round(state.Stability*1.3)))
		state.StateCode = FSRSStateReview
	default:
		return nextReviewState(state, Good)
	}

	return state
}

func estimateRetrievability(elapsedDays int, stability float64) float64 {
	if stability <= 0 {
		return 0
	}
	// TODO: Replace with exact FSRS v4 forgetting curve/exponent constants if product wants parity.
	r := math.Exp(-float64(maxInt(0, elapsedDays)) / stability)
	return clampFloat(r, 0, 1)
}

func hardStabilityFactor(retrievability float64) float64 {
	// TODO: Replace with exact FSRS v4 hard update equation when finalized.
	return 1.2 + 0.2*retrievability
}

func goodStabilityFactor(retrievability float64) float64 {
	// TODO: Replace with exact FSRS v4 good update equation when finalized.
	return 1.8 + 0.5*retrievability
}

func easyStabilityFactor(retrievability float64) float64 {
	// TODO: Replace with exact FSRS v4 easy update equation when finalized.
	return 2.3 + 0.7*retrievability
}

func clampFloat(v, minV, maxV float64) float64 {
	if v < minV {
		return minV
	}
	if v > maxV {
		return maxV
	}
	return v
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
