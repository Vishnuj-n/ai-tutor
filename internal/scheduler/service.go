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

// New creates a new scheduler service
func New() *Service {
	return &Service{
		queryDueReviewCards: db.QueryDueReviewCards,
		queryActiveTopics:   db.QueryActiveTopics,
		queryLearningTopics: db.QueryLearningTopics,
		countLearnedTopics:  db.CountLearnedTopics,
	}
}

type queryDueReviewCardsFn func(now string) (int, error)
type queryActiveTopicsFn func(limit int) ([]string, error)
type queryLearningTopicsFn func(limit int) ([]models.TopicSummary, error)
type countLearnedTopicsFn func() (int, error)

// Service builds one daily plan that can include all study modes.
type Service struct {
	queryDueReviewCards queryDueReviewCardsFn
	queryActiveTopics   queryActiveTopicsFn
	queryLearningTopics queryLearningTopicsFn
	countLearnedTopics  countLearnedTopicsFn
}

func (s *Service) ensureQueryFns() {
	if s.queryDueReviewCards == nil {
		s.queryDueReviewCards = db.QueryDueReviewCards
	}
	if s.queryActiveTopics == nil {
		s.queryActiveTopics = db.QueryActiveTopics
	}
	if s.queryLearningTopics == nil {
		s.queryLearningTopics = db.QueryLearningTopics
	}
	if s.countLearnedTopics == nil {
		s.countLearnedTopics = db.CountLearnedTopics
	}
}

// BuildTodayPlan builds the complete daily schedule
func (s *Service) BuildTodayPlan(now time.Time) (*models.TodayPlan, error) {
	s.ensureQueryFns()

	dueCards, err := s.queryDueReviewCards(now.Format(time.RFC3339))
	if err != nil {
		return nil, err
	}

	activeTopics, err := s.queryActiveTopics(3)
	if err != nil {
		return nil, err
	}

	learningTopics, err := s.queryLearningTopics(2)
	if err != nil {
		return nil, err
	}

	learnedTopicsCount, err := s.countLearnedTopics()
	if err != nil {
		return nil, err
	}

	reviewMinutes := dueCards * 2
	if reviewMinutes > MaxReviewMinutes {
		reviewMinutes = MaxReviewMinutes
	}

	if reviewMinutes > DefaultDailyStudyMinutes-MinLearningMinutes {
		reviewMinutes = DefaultDailyStudyMinutes - MinLearningMinutes
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

	if len(learningTopics) > 0 {
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

	if learnedTopicsCount > 0 {
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

	return &models.TodayPlan{
		Date:            now.Format("2006-01-02"),
		TotalMinutes:    DefaultDailyStudyMinutes,
		ReviewMinutes:   reviewMinutes,
		LearningMinutes: learningMinutes,
		DueReviewCards:  dueCards,
		ActiveTopics:    activeTopics,
		Tasks:           tasks,
	}, nil
}
