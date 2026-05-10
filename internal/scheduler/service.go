package scheduler

import (
	"fmt"
	"math"
	"strconv"
	"time"

	"ai-tutor/internal/db"
	"ai-tutor/internal/models"
	"ai-tutor/internal/utils"
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

	// tokenBudget cannot be negative since readingBudget is clamped to >=0 and WordsPerMinute is positive

	readingTopic, foundReadingTopic, err := s.queryNextReadingTopic()

	if err != nil {
		return nil, err
	}

	tasks := make([]models.ScheduledTask, 0, 1)
	activeTopics := make([]string, 0, 1)

	if foundReadingTopic {

		startPage, endPage, ok, tokenMap := resolvePageWindow(
			readingTopic,
			tokenBudget,
			s.queryTokensPerPageMap,
		)

		if ok {

			generatedTaskID := "task-read-" + readingTopic.ID

			utils.LogSchedulerDecision(readingTopic.ID, startPage, endPage, strconv.Itoa(tokenBudget), "adaptive_window_resolved")

			activeTopics = append(activeTopics, readingTopic.Title)

			actualTaskMinutes := s.estimateTaskMinutes(
				readingTopic.ID,
				startPage,
				endPage,
				tokenMap,
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
) (int, int, bool, map[int]int) {

	if topic.EndPage <= 0 {
		return 0, 0, false, nil
	}

	if tokenBudget <= 0 {
		return 0, 0, false, nil
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
		return 0, 0, false, nil
	}

	endPage := startPage
	accumulatedWords := 0

	// Batch fetch all page tokens in a single query to avoid N+1 problem
	tokenMap, err := queryTokensPerPageMap(topic.ID, startPage, topic.EndPage)
	if err != nil {
		// On error, initialize an empty tokenMap so the subsequent logic will use
		// FallbackWordsPerPage for all pages instead of performing single-page queries
		tokenMap = make(map[int]int)
	}

	for page := startPage; page <= topic.EndPage; page++ {

		if page-startPage >= MaxPageScanLimit {
			break
		}

		pageWords, ok := tokenMap[page]
		if !ok || pageWords <= 0 {
			pageWords = FallbackWordsPerPage
		}

		// Check if adding this page would exceed budget BEFORE adding it
		if accumulatedWords+pageWords > tokenBudget && accumulatedWords > 0 {
			break
		}

		accumulatedWords += pageWords
		endPage = page

		// Structured debug logging for page-by-page resolution
		// Use utils.Debugf if available, otherwise comment out for production
		// TODO: Add utils.Debugf support when debug logging is needed
		// utils.Debugf("[RESOLVE_PAGE_WINDOW] page=%d pageWords=%d accumulatedWords=%d tokenBudget=%d useFallback=%v",
		// 	page, pageWords, accumulatedWords, tokenBudget, useFallback)
	}

	// Preserve original near-end behavior
	if topic.EndPage-endPage <= ClampWindowPages {
		endPage = topic.EndPage
	}

	if endPage < startPage {
		return 0, 0, false, nil
	}

	return startPage, endPage, true, tokenMap
}

// estimateTaskMinutes calculates realistic workload using token counts.
// Accepts optional pre-fetched tokenMap to avoid redundant DB queries.
func (s *service) estimateTaskMinutes(
	topicID string,
	startPage,
	endPage int,
	tokenMap map[int]int,
) int {

	pageCount := endPage - startPage + 1

	if pageCount <= 0 {
		return 0
	}

	// Use pre-fetched tokenMap if provided, otherwise query DB
	totalWords := 0
	var err error
	if tokenMap == nil {
		fetchedMap, fetchErr := s.queryTokensPerPageMap(topicID, startPage, endPage)
		if fetchErr == nil {
			for _, pageTokens := range fetchedMap {
				totalWords += pageTokens
			}
		}
		err = fetchErr
	} else {
		for page := startPage; page <= endPage; page++ {
			pageTokens := tokenMap[page]
			if pageTokens <= 0 {
				pageTokens = FallbackWordsPerPage
			}
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
