package scheduler

import (
	"time"

	fsrs "github.com/open-spaced-repetition/go-fsrs/v4"
)

// Standard FSRS rating definitions mapping straight to your app inputs
const (
	Again = int(fsrs.Again) // 1
	Hard  = int(fsrs.Hard)  // 2
	Good  = int(fsrs.Good)  // 3
	Easy  = int(fsrs.Easy)  // 4
)

// NextFSRSState calls the official open-spaced-repetition engine.
func NextFSRSState(fsrsCard fsrs.Card, rating int, now time.Time) (fsrs.Card, error) {
	// 1. Initialize the official engine configuration parameters
	p := fsrs.DefaultParam()
	p.RequestRetention = 0.9 // Enforces our 90% retention profile target
	engine := fsrs.NewFSRS(p)

	// Fallback mechanism for brand new cards flowing into the scheduling window
	if fsrsCard.Reps == 0 || fsrsCard.Due.IsZero() {
		fsrsCard.Due = now
	}

	// 3. Compute all 4 timeline variations simultaneously
	schedulingCards, err := engine.Repeat(fsrsCard, now)
	if err != nil {
		return fsrsCard, err
	}

	// 4. Extract the exact button response clicked by the user
	chosenRecord := schedulingCards[fsrs.Rating(rating)]

	return chosenRecord.Card, nil
}
