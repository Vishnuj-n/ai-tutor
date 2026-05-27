package scheduler

import (
	"log"
	"time"

	"ai-tutor/internal/models"

	fsrs "github.com/open-spaced-repetition/go-fsrs/v4"
)

// Standard FSRS rating definitions mapping straight to your app inputs
const (
	Again = int(fsrs.Again) // 1
	Hard  = int(fsrs.Hard)  // 2
	Good  = int(fsrs.Good)  // 3
	Easy  = int(fsrs.Easy)  // 4
)

// NextFSRSState calls the official open-spaced-repetition engine using your model helpers.
func NextFSRSState(state models.FlashcardState, rating int, now time.Time, dueAt, lastReviewedAt int64) (models.FlashcardState, error) {
	// 1. Initialize the official engine configuration parameters
	p := fsrs.DefaultParam()
	p.RequestRetention = 0.9 // Enforces our 90% retention profile target
	engine := fsrs.NewFSRS(p)

	// 2. Use your model's existing function to convert data types to go-fsrs format
	fsrsCard := models.FlashcardStateToCard(state, dueAt, lastReviewedAt)

	// Fallback mechanism for brand new cards flowing into the scheduling window
	if state.Reps == 0 || fsrsCard.Due.IsZero() {
		fsrsCard.Due = now
	}

	// Keep track of the pre-transition last review timestamp
	originalLastReview := fsrsCard.LastReview

	// 3. Compute all 4 timeline variations simultaneously
	schedulingCards, err := engine.Repeat(fsrsCard, now)
	if err != nil {
		log.Printf("FSRS error: engine.Repeat failed: %v (card: %+v, now: %v)", err, fsrsCard, now)
		return state, err
	}

	// 4. Extract the exact button response clicked by the user
	chosenRecord := schedulingCards[fsrs.Rating(rating)]

	// 5. Use your model's existing converter to translate the results back to your database struct
	updatedState := models.CardToFlashcardState(chosenRecord.Card)

	// Track the operational increments that go-fsrs handles internally using pre-transition timestamp
	if !originalLastReview.IsZero() {
		elapsedDays := int(now.Sub(originalLastReview).Hours() / 24)
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
