package scheduler

import (
	"fmt"
	"time"

	"ai-tutor/internal/db"
	"ai-tutor/internal/models"
)

const (
	DefaultDailyStudyMinutes = 90
	MinLearningMinutes       = 20
	MaxReviewMinutes         = 60
)

type queryDueReviewCardsFn func(now int64) (int, error)
type queryActiveTopicsFn func(limit int) ([]string, error)
type queryLearningTopicsFn func(limit int) ([]models.TopicSummary, error)
type countLearnedTopicsFn func() (int, error)

// service builds one daily plan that can include all study modes.
// Use New() or exported functional options to construct safely.
type service struct {
	queryDueReviewCards queryDueReviewCardsFn
	queryActiveTopics   queryActiveTopicsFn
	queryLearningTopics queryLearningTopicsFn
	countLearnedTopics  countLearnedTopicsFn
}

// Option customizes service dependencies for testing and advanced setups.
type Option func(*service)

// WithQueryDueReviewCards overrides the review query dependency.
func WithQueryDueReviewCards(fn queryDueReviewCardsFn) Option {
	return func(s *service) {
		if fn != nil {
			s.queryDueReviewCards = fn
		}
	}
}

// WithQueryActiveTopics overrides the active topics query dependency.
func WithQueryActiveTopics(fn queryActiveTopicsFn) Option {
	return func(s *service) {
		if fn != nil {
			s.queryActiveTopics = fn
		}
	}
}

// WithQueryLearningTopics overrides the learning topics query dependency.
func WithQueryLearningTopics(fn queryLearningTopicsFn) Option {
	return func(s *service) {
		if fn != nil {
			s.queryLearningTopics = fn
		}
	}
}

// WithCountLearnedTopics overrides the learned topics count dependency.
func WithCountLearnedTopics(fn countLearnedTopicsFn) Option {
	return func(s *service) {
		if fn != nil {
			s.countLearnedTopics = fn
		}
	}
}

// Service is the public interface for daily plan scheduling.
type Service interface {
	BuildTodayPlan(now time.Time) (*models.TodayPlan, error)
}

// New creates a new scheduler service with real database queries.
// Use functional options to override dependencies for testing.
func New(opts ...Option) Service {
	s := &service{
		queryDueReviewCards: db.QueryDueReviewCards,
		queryActiveTopics:   db.QueryActiveTopics,
		queryLearningTopics: db.QueryLearningTopics,
		countLearnedTopics:  db.CountLearnedTopics,
	}

	for _, opt := range opts {
		opt(s)
	}

	return s
}

// BuildTodayPlan builds the complete daily schedule
func (s *service) BuildTodayPlan(now time.Time) (*models.TodayPlan, error) {

	dueCards, err := s.queryDueReviewCards(now.Unix())
	if err != nil {
		return nil, err
	}

	activeTopics, err := s.queryActiveTopics(3)
	if err != nil {
		return nil, err
	}

	// Determine catch-up mode first; learning queries are only needed if NOT in catch-up
	catchUpMode := dueCards*2 > MaxReviewMinutes

	reviewMinutes := dueCards * 2
	if catchUpMode {
		reviewMinutes = DefaultDailyStudyMinutes
	}

	// Skip learning queries during catch-up to avoid transient DB failures aborting the entire plan
	var learningTopics []models.TopicSummary
	var learnedTopicsCount int
	if !catchUpMode {
		var err error
		learningTopics, err = s.queryLearningTopics(2)
		if err != nil {
			return nil, err
		}

		learnedTopicsCount, err = s.countLearnedTopics()
		if err != nil {
			return nil, err
		}
	}

	if dueCards == 0 {
		reviewMinutes = 0
	}

	learningMinutes := DefaultDailyStudyMinutes - reviewMinutes

	tasks := make([]models.ScheduledTask, 0, 6)
	priority := 1
	usedMinutes := 0

	if dueCards > 0 {
		tasks = append(tasks, models.ScheduledTask{
			ID:              "task-review-due",
			ActionType:      "review",
			Title:           "Due Flashcard Reviews",
			EstimateMinutes: reviewMinutes,
			Priority:        priority,
			Meta:            fmt.Sprintf("%d cards due today", dueCards),
		})
		priority++
		usedMinutes += reviewMinutes
	}

	if !catchUpMode && len(learningTopics) > 0 {
		perTopic := learningMinutes / len(learningTopics)
		if perTopic < 15 {
			perTopic = 15
		}

		for _, topic := range learningTopics {
			tasks = append(tasks, models.ScheduledTask{
				ID:              "task-read-" + topic.ID,
				ActionType:      "read",
				Title:           "Read: " + topic.Title,
				TopicID:         topic.ID,
				EstimateMinutes: perTopic,
				Priority:        priority,
				Meta:            "Concept-building session",
			})
			priority++
			usedMinutes += perTopic
		}
	}

	if !catchUpMode && learnedTopicsCount > 0 {
		remaining := DefaultDailyStudyMinutes - usedMinutes
		if remaining >= 15 {
			tasks = append(tasks, models.ScheduledTask{
				ID:              "task-quiz-practice",
				ActionType:      "quiz",
				Title:           "Quiz Checkpoint",
				EstimateMinutes: 15,
				Priority:        priority,
				Meta:            "Validate understanding on learned topics",
			})
			priority++
			usedMinutes += 15
		}

		remaining = DefaultDailyStudyMinutes - usedMinutes
		if remaining >= 10 {
			tasks = append(tasks, models.ScheduledTask{
				ID:              "task-socratic-reflection",
				ActionType:      "socratic",
				Title:           "Socratic Reflection",
				EstimateMinutes: 10,
				Priority:        priority,
				Meta:            "Explain ideas in your own words",
			})
		}
	}

	if len(tasks) == 0 {
		tasks = append(tasks, models.ScheduledTask{
			ID:              "task-explore",
			ActionType:      "explore",
			Title:           "Optional Exploration",
			EstimateMinutes: 20,
			Priority:        1,
			Meta:            "Pick any topic and do a short learning pass",
		})
	}

	isEstimate := len(tasks) == 1 && tasks[0].ActionType == "explore"

	return &models.TodayPlan{
		Date:            now.Format("2006-01-02"),
		TotalMinutes:    DefaultDailyStudyMinutes,
		ReviewMinutes:   reviewMinutes,
		LearningMinutes: learningMinutes,
		DueReviewCards:  dueCards,
		ActiveTopics:    activeTopics,
		Tasks:           tasks,
		IsEstimate:      isEstimate,
	}, nil
}
