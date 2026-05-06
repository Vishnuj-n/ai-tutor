package scheduler

import (
	"fmt"
	"math"
	"time"

	"ai-tutor/internal/db"
	"ai-tutor/internal/models"
)

const (
	DefaultDailyStudyMinutes = 90
	ReviewMinutesPerCard     = 0.5
	MinutesPerPage           = 2.5
	ClampWindowPages         = 4
)

type queryDueReviewCardsFn func(now int64) (int, error)
type queryDailyStudyMinutesFn func() (int, error)
type queryNextReadingTopicFn func() (models.ReadingTopicCursor, bool, error)
type querySubtopicsByParentTopicFn func(parentTopicID string) ([]models.Subtopic, error)
type queryNextSubtopicFn func(parentTopicID string, currentPage int) (*models.Subtopic, error)

// service builds one context-locked daily reading task.
type service struct {
	queryDueReviewCards         queryDueReviewCardsFn
	queryDailyStudyMinute       queryDailyStudyMinutesFn
	queryNextReadingTopic       queryNextReadingTopicFn
	querySubtopicsByParentTopic querySubtopicsByParentTopicFn
	queryNextSubtopic           queryNextSubtopicFn
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

// WithQueryDailyStudyMinutes overrides the user settings query dependency.
func WithQueryDailyStudyMinutes(fn queryDailyStudyMinutesFn) Option {
	return func(s *service) {
		if fn != nil {
			s.queryDailyStudyMinute = fn
		}
	}
}

// WithQueryNextReadingTopic overrides the topic cursor query dependency.
func WithQueryNextReadingTopic(fn queryNextReadingTopicFn) Option {
	return func(s *service) {
		if fn != nil {
			s.queryNextReadingTopic = fn
		}
	}
}

// WithQuerySubtopicsByParentTopic overrides the subtopic query dependency.
func WithQuerySubtopicsByParentTopic(fn querySubtopicsByParentTopicFn) Option {
	return func(s *service) {
		if fn != nil {
			s.querySubtopicsByParentTopic = fn
		}
	}
}

// WithQueryNextSubtopic overrides the next subtopic query dependency.
func WithQueryNextSubtopic(fn queryNextSubtopicFn) Option {
	return func(s *service) {
		if fn != nil {
			s.queryNextSubtopic = fn
		}
	}
}

// Service is the public interface for daily plan scheduling.
type Service interface {
	BuildTodayPlan(now time.Time) (*models.TodayPlan, error)
}

// New creates a new scheduler service with real database queries.
func New(opts ...Option) Service {
	s := &service{
		queryDueReviewCards:         db.QueryDueReviewCards,
		queryDailyStudyMinute:       db.GetDailyStudyMinutes,
		queryNextReadingTopic:       db.QueryNextReadingTopic,
		querySubtopicsByParentTopic: db.GetSubtopicsByParentTopic,
		queryNextSubtopic:           db.GetNextSubtopic,
	}

	for _, opt := range opts {
		opt(s)
	}

	return s
}

// BuildTodayPlan calculates review budget, reading budget, and one context-locked reading task.
func (s *service) BuildTodayPlan(now time.Time) (*models.TodayPlan, error) {
	dueCards, err := s.queryDueReviewCards(now.Unix())
	if err != nil {
		return nil, err
	}

	dailyStudyMinutes, err := s.queryDailyStudyMinute()
	if err != nil {
		return nil, err
	}
	if dailyStudyMinutes <= 0 {
		dailyStudyMinutes = DefaultDailyStudyMinutes
	}

	reviewBudget := int(math.Ceil(float64(dueCards) * ReviewMinutesPerCard))
	if reviewBudget > dailyStudyMinutes {
		reviewBudget = dailyStudyMinutes
	}

	readingBudget := dailyStudyMinutes - reviewBudget
	if readingBudget < 0 {
		readingBudget = 0
	}

	pagesToRead := int(math.Floor(float64(readingBudget) / MinutesPerPage))
	if pagesToRead < 0 {
		pagesToRead = 0
	}

	readingTopic, foundReadingTopic, err := s.queryNextReadingTopic()
	if err != nil {
		return nil, err
	}

	tasks := make([]models.ScheduledTask, 0, 1)
	activeTopics := make([]string, 0, 1)

	if foundReadingTopic {
		// Try to use subtopic-based missions first
		if s.querySubtopicsByParentTopic != nil && s.queryNextSubtopic != nil {
			subtopics, err := s.querySubtopicsByParentTopic(readingTopic.ID)
			if err == nil && len(subtopics) > 0 {
				// Find the next subtopic based on current page cursor
				currentPage := readingTopic.CurrentPageCursor
				if currentPage <= 0 {
					currentPage = readingTopic.StartPage
				}
				if currentPage <= 0 {
					currentPage = 1
				}

				nextSubtopic, err := s.queryNextSubtopic(readingTopic.ID, currentPage)
				if err == nil && nextSubtopic != nil {
					// Create a subtopic-based mission
					activeTopics = append(activeTopics, readingTopic.Title)
					subtopicPages := nextSubtopic.EndPage - nextSubtopic.StartPage + 1
					actualTaskMinutes := int(float64(subtopicPages) * MinutesPerPage)

					// Truncate if exceeds reading budget
					if actualTaskMinutes > readingBudget {
						actualTaskMinutes = readingBudget
					}

					tasks = append(tasks, models.ScheduledTask{
						ID:              "task-read-" + nextSubtopic.ID,
						ActionType:      "read",
						Title:           fmt.Sprintf("Read: %s > %s (Pages %d to %d)", readingTopic.Title, nextSubtopic.Title, nextSubtopic.StartPage, nextSubtopic.EndPage),
						TopicID:         readingTopic.ID,
						StartPage:       nextSubtopic.StartPage,
						EndPage:         nextSubtopic.EndPage,
						EstimateMinutes: actualTaskMinutes,
						Priority:        1,
						Meta:            fmt.Sprintf("Subtopic: %s | Search snippet: %s", nextSubtopic.Title, nextSubtopic.SearchSnippet),
					})
				}
			}
		}

		// Fallback to topic-based mission if no subtopics available
		if len(tasks) == 0 {
			startPage, endPage, ok := resolvePageWindow(readingTopic, pagesToRead)
			if ok {
				activeTopics = append(activeTopics, readingTopic.Title)
				// Calculate actual task minutes based on page span
				actualTaskMinutes := int(float64(endPage-startPage+1) * MinutesPerPage)
				tasks = append(tasks, models.ScheduledTask{
					ID:              "task-read-" + readingTopic.ID,
					ActionType:      "read",
					Title:           fmt.Sprintf("Read: %s (Pages %d to %d)", readingTopic.Title, startPage, endPage),
					TopicID:         readingTopic.ID,
					StartPage:       startPage,
					EndPage:         endPage,
					EstimateMinutes: actualTaskMinutes,
					Priority:        1,
					Meta:            fmt.Sprintf("Context-locked to pages %d-%d", startPage, endPage),
				})
			}
		}
	}

	// Calculate total learning minutes from actual tasks
	totalLearningMinutes := 0
	for _, task := range tasks {
		totalLearningMinutes += task.EstimateMinutes
	}

	return &models.TodayPlan{
		Date:            now.Format("2006-01-02"),
		TotalMinutes:    dailyStudyMinutes,
		ReviewMinutes:   reviewBudget,
		LearningMinutes: totalLearningMinutes,
		DueReviewCards:  dueCards,
		ActiveTopics:    activeTopics,
		Tasks:           tasks,
		IsEstimate:      len(tasks) == 0,
	}, nil
}

func resolvePageWindow(topic models.ReadingTopicCursor, pagesToRead int) (int, int, bool) {
	if topic.EndPage <= 0 {
		return 0, 0, false
	}
	if pagesToRead <= 0 {
		return 0, 0, false
	}

	startPage := topic.CurrentPageCursor
	if startPage <= 0 {
		startPage = topic.StartPage
	}
	if startPage <= 0 {
		startPage = 1
	}
	if topic.StartPage > 0 && startPage < topic.StartPage {
		startPage = topic.StartPage
	}
	if startPage > topic.EndPage {
		return 0, 0, false
	}

	endPage := startPage + pagesToRead - 1
	if endPage < startPage {
		endPage = startPage
	}
	if endPage > topic.EndPage {
		endPage = topic.EndPage
	}
	if topic.EndPage-endPage <= ClampWindowPages {
		endPage = topic.EndPage
	}
	// Enforce hard cap: window should never exceed pagesToRead budget
	maxEndPage := startPage + pagesToRead - 1
	if endPage > maxEndPage {
		endPage = maxEndPage
	}

	if endPage < startPage {
		return 0, 0, false
	}

	return startPage, endPage, true
}
