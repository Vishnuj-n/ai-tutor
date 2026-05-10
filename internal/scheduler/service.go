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

	// Legacy fallback only
	MinutesPerPage = 2.5

	ClampWindowPages = 4

	// Reading assumptions
	WordsPerMinute     = 200
	TargetSessionWords = 2500

	// Fallback assumptions
	FallbackWordsPerPage = 500
	MaxPageScanLimit     = 100
	MinMinutesPerPage    = 1.0
)

type queryDueReviewCardsFn func(now int64) (int, error)
type queryDailyStudyMinutesFn func() (int, error)
type queryNextReadingTopicFn func() (models.ReadingTopicCursor, bool, error)
type queryTokensPerPageMapFn func(topicID string, startPage int, endPage int) (map[int]int, error)

// service builds one context-locked daily reading task.
type service struct {
	queryDueReviewCards   queryDueReviewCardsFn
	queryDailyStudyMinute queryDailyStudyMinutesFn
	queryNextReadingTopic queryNextReadingTopicFn
	queryTokensPerPageMap queryTokensPerPageMapFn
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

// WithQueryTokensPerPageMap overrides the chunk token query dependency.
func WithQueryTokensPerPageMap(fn queryTokensPerPageMapFn) Option {
	return func(s *service) {
		if fn != nil {
			s.queryTokensPerPageMap = fn
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
		queryDueReviewCards:   db.QueryDueReviewCards,
		queryDailyStudyMinute: db.GetDailyStudyMinutes,
		queryNextReadingTopic: db.QueryNextReadingTopic,
		queryTokensPerPageMap: db.GetTokensPerPageMap,
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

	// Convert reading budget into adaptive word budget
	tokenBudget := readingBudget * WordsPerMinute

	// Keep sessions near intended workload size
	if tokenBudget > TargetSessionWords {
		tokenBudget = TargetSessionWords
	}

	if tokenBudget < 0 {
		tokenBudget = 0
	}

	readingTopic, foundReadingTopic, err := s.queryNextReadingTopic()

	if err != nil {
		return nil, err
	}

	tasks := make([]models.ScheduledTask, 0, 1)
	activeTopics := make([]string, 0, 1)

	if foundReadingTopic {

		startPage, endPage, ok := resolvePageWindow(
			readingTopic,
			tokenBudget,
			s.queryTokensPerPageMap,
		)

		if ok {

			generatedTaskID := "task-read-" + readingTopic.ID

			fmt.Printf(
				"[TODAY_PLAN] adaptive reading window taskID=%s topicID=%s startPage=%d endPage=%d tokenBudget=%d\n",
				generatedTaskID,
				readingTopic.ID,
				startPage,
				endPage,
				tokenBudget,
			)

			activeTopics = append(activeTopics, readingTopic.Title)

			actualTaskMinutes := estimateTaskMinutes(
				s,
				readingTopic.ID,
				startPage,
				endPage,
			)

			tasks = append(tasks, models.ScheduledTask{
				ID:              generatedTaskID,
				ActionType:      "reading",
				Title:           fmt.Sprintf("Read: %s (Pages %d to %d)", readingTopic.Title, startPage, endPage),
				TopicID:         readingTopic.ID,
				NotebookID:      readingTopic.NotebookID,
				StartPage:       startPage,
				EndPage:         endPage,
				EstimateMinutes: actualTaskMinutes,
				Priority:        1,
				Meta:            fmt.Sprintf("Context-locked to pages %d-%d", startPage, endPage),
			})
		}
	}

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

func resolvePageWindow(
	topic models.ReadingTopicCursor,
	tokenBudget int,
	queryTokensPerPageMap queryTokensPerPageMapFn,
) (int, int, bool) {

	if topic.EndPage <= 0 {
		return 0, 0, false
	}

	if tokenBudget <= 0 {
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

	endPage := startPage
	accumulatedWords := 0

	// Batch fetch all page tokens in a single query to avoid N+1 problem
	tokenMap, err := queryTokensPerPageMap(topic.ID, startPage, topic.EndPage)
	if err != nil {
		// Fall back to single-page queries if batch fails
		tokenMap = make(map[int]int)
	}

	for page := startPage; page <= topic.EndPage; page++ {

		if page-startPage >= MaxPageScanLimit {
			break
		}

		pageWords, ok := tokenMap[page]
		useFallback := false
		if !ok || pageWords <= 0 {
			pageWords = FallbackWordsPerPage
			useFallback = true
		}

		// Check if adding this page would exceed budget BEFORE adding it
		if accumulatedWords+pageWords > tokenBudget && accumulatedWords > 0 {
			break
		}

		accumulatedWords += pageWords
		endPage = page

		fmt.Printf(
			"[RESOLVE_PAGE_WINDOW] page=%d pageWords=%d accumulatedWords=%d tokenBudget=%d useFallback=%v\n",
			page,
			pageWords,
			accumulatedWords,
			tokenBudget,
			useFallback,
		)
	}

	// Preserve original near-end behavior
	if topic.EndPage-endPage <= ClampWindowPages {
		endPage = topic.EndPage
	}

	if endPage < startPage {
		return 0, 0, false
	}

	return startPage, endPage, true
}

// estimateTaskMinutes calculates realistic workload using token counts.
func estimateTaskMinutes(
	s *service,
	topicID string,
	startPage,
	endPage int,
) int {

	pageCount := endPage - startPage + 1

	if pageCount <= 0 {
		return 0
	}

	// Calculate total words from the page map
	tokenMap, err := s.queryTokensPerPageMap(topicID, startPage, endPage)
	totalWords := 0
	if err == nil {
		for _, pageTokens := range tokenMap {
			totalWords += pageTokens
		}
	}

	// Primary token-aware estimation
	if err == nil && totalWords > 0 {

		minutes := int(
			math.Ceil(
				float64(totalWords) / float64(WordsPerMinute),
			),
		)

		// Safety floor for sparse pages
		pageFloor := int(
			math.Ceil(
				float64(pageCount) * MinMinutesPerPage,
			),
		)

		if minutes < pageFloor {
			return pageFloor
		}

		return minutes
	}

	// Legacy fallback
	return int(
		math.Ceil(
			float64(pageCount) * MinutesPerPage,
		),
	)
}
