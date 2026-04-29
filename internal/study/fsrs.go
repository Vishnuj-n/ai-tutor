package study

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"ai-tutor/internal/db"
	"ai-tutor/internal/models"
	"ai-tutor/internal/scheduler"

	"github.com/google/uuid"
)

// LogReview applies FSRS update for one assessment item.
func (s *StudyService) LogReview(topicID, activityType, referenceID, sourceChunkID string, score int) error {
	_, err := s.logReview(nil, topicID, activityType, referenceID, sourceChunkID, score)
	return err
}

func (s *StudyService) LogReviewTx(tx *sql.Tx, topicID, activityType, referenceID, sourceChunkID string, score int) (map[string]interface{}, error) {
	return s.logReview(tx, topicID, activityType, referenceID, sourceChunkID, score)
}

func (s *StudyService) logReview(tx *sql.Tx, topicID, activityType, referenceID, sourceChunkID string, score int) (map[string]interface{}, error) {
	topicID = strings.TrimSpace(topicID)
	activityType = strings.TrimSpace(activityType)
	referenceID = strings.TrimSpace(referenceID)
	sourceChunkID = strings.TrimSpace(sourceChunkID)
	if topicID == "" || activityType == "" || referenceID == "" {
		return nil, fmt.Errorf("topicID, activityType, and referenceID are required")
	}

	// Ensure sourceChunkID defaults to empty string to avoid NULL handling issues
	if sourceChunkID == "" {
		sourceChunkID = ""
	}

	ratingCode, err := mapScoreToFSRSRating(activityType, score)
	if err != nil {
		return nil, err
	}

	var current *db.AssessmentFSRSRecord
	if tx != nil {
		current, err = db.GetAssessmentFSRSStateTx(tx, activityType, referenceID, sourceChunkID)
	} else {
		current, err = db.GetAssessmentFSRSState(activityType, referenceID, sourceChunkID)
	}
	if err != nil {
		return nil, err
	}

	state := models.FlashcardState{}
	stateBeforeJSON := "{}"
	if current != nil {
		state = current.GetState()
		beforeBytes, marshalErr := json.Marshal(state)
		if marshalErr != nil {
			return nil, marshalErr
		}
		stateBeforeJSON = string(beforeBytes)
		elapsedDays := 0
		if current.GetDueAt() > 0 {
			elapsedSeconds := time.Now().Unix() - current.GetDueAt()
			if elapsedSeconds > 0 {
				elapsedDays = int(elapsedSeconds / (24 * 60 * 60))
			}
		}
		state.ElapsedDays = elapsedDays
	}

	nextState := scheduler.NextFSRSState(state, ratingCode)
	now := time.Now().Unix()
	dueAt := now + int64(nextState.ScheduledDays)*24*60*60
	if nextState.ScheduledDays == 0 {
		dueAt = now
	}
	stateAfterBytes, err := json.Marshal(nextState)
	if err != nil {
		return nil, err
	}

	reviewLog := models.FSRSReviewLog{
		ID:              uuid.NewString(),
		TopicID:         topicID,
		ActivityType:    activityType,
		ReferenceID:     referenceID,
		ReviewedAt:      now,
		Rating:          ratingCode,
		ScheduledDays:   nextState.ScheduledDays,
		StateBeforeJSON: stateBeforeJSON,
		StateAfterJSON:  string(stateAfterBytes),
	}

	if tx != nil {
		err = db.UpsertAssessmentFSRSReviewTx(tx, activityType, referenceID, topicID, sourceChunkID, nextState, dueAt, now, reviewLog)
	} else {
		err = db.UpsertAssessmentFSRSReview(activityType, referenceID, topicID, sourceChunkID, nextState, dueAt, now, reviewLog)
	}
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"fsrs_rating":    ratingCodeToTitle(ratingCode),
		"scheduled_days": nextState.ScheduledDays,
		"next_review_at": time.Unix(dueAt, 0).Format(time.RFC3339),
		"review_log_id":  reviewLog.ID,
	}, nil
}

func mapScoreToFSRSRating(activityType string, score int) (int, error) {
	switch strings.ToLower(strings.TrimSpace(activityType)) {
	case "quiz", "quiz_question", "written", "written_question":
		switch {
		case score < 0 || score > 100:
			return 0, fmt.Errorf("percentage score must be between 0 and 100")
		case score < 30:
			return scheduler.Again, nil
		case score <= 60:
			return scheduler.Hard, nil
		case score <= 90:
			return scheduler.Good, nil
		default:
			return scheduler.Easy, nil
		}
	default:
		if score < 1 || score > 4 {
			return 0, fmt.Errorf("score for %s must be between 1 and 4", activityType)
		}
		return score, nil
	}
}

func ratingCodeToTitle(code int) string {
	switch code {
	case scheduler.Again:
		return "Again"
	case scheduler.Hard:
		return "Hard"
	case scheduler.Good:
		return "Good"
	case scheduler.Easy:
		return "Easy"
	default:
		return "Again"
	}
}
