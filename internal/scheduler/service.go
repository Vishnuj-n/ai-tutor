package scheduler

import (
	"fmt"
	"math"
	"strconv"
	"strings"
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

	// Review workload caps
	MaxReviewMinutesRatio   = 0.5 // Allow max 50% of session for reviews
	MaxReviewMinutesSession = 30  // Hard cap of 30 mins for reviews
	MaxReviewSessionCards   = 60  // Max cards per total workload (across sessions)
)

type queryDueReviewCardsFn func(now int64) (int, error)
type queryUserSettingsFn func() (*models.UserSettings, error)
type queryNextReadingTopicFn func() (models.ReadingTopicCursor, bool, error)
type queryTokensPerPageMapFn func(topicID string, startPage int, endPage int) (map[int]int, error)
type queryNextDueReviewNotebookFn func(now int64) (string, int, error)

// service builds one context-locked daily reading task.
type service struct {
	queryDueReviewCards        queryDueReviewCardsFn
	queryUserSettings          queryUserSettingsFn
	queryNextReadingTopic      queryNextReadingTopicFn
	queryTokensPerPageMap      queryTokensPerPageMapFn
	queryNextDueReviewNotebook queryNextDueReviewNotebookFn
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
// WithQueryNextDueReviewNotebook overrides the due-review notebook query dependency.
// A nil fn is ignored so the default set in New() is preserved.
func WithQueryNextDueReviewNotebook(fn queryNextDueReviewNotebookFn) Option {
	return func(s *service) {
		if fn != nil {
			s.queryNextDueReviewNotebook = fn
		}
	}
}

// WithQueryUserSettings overrides the user settings query dependency.
func WithQueryUserSettings(fn queryUserSettingsFn) Option {
	return func(s *service) {
		if fn != nil {
			s.queryUserSettings = fn
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
func New(repo *db.Repository, opts ...Option) Service {
	s := &service{}
	if repo != nil {
		s.queryDueReviewCards = repo.QueryDueReviewCards
		s.queryUserSettings = repo.GetUserSettings
		s.queryNextReadingTopic = repo.QueryNextReadingTopic
		s.queryTokensPerPageMap = repo.GetTokensPerPageMap
		s.queryNextDueReviewNotebook = repo.GetNextDueReviewNotebook
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

	settings, err := s.queryUserSettings()
	if err != nil {
		return nil, err
	}

	maxFlashcards := settings.MaxFlashcardsPerSession
	if maxFlashcards <= 0 {
		maxFlashcards = 30
	}

	dailyStudyMinutes := calculateDurationMinutes(settings.StudyStartTime, settings.StudyEndTime)

	totalDueCards := dueCards
	materializedCards := dueCards
	if materializedCards > maxFlashcards {
		materializedCards = maxFlashcards
	}
	deferredCards := totalDueCards - materializedCards
	if deferredCards < 0 {
		deferredCards = 0
	}

	finalDueReviewCards := materializedCards
	finalReviewMinutes := int(math.Ceil(float64(materializedCards) * ReviewMinutesPerCard))

	utils.Warnf("[SCHEDULER] workload_audit total_due_cards=%d review_cards_materialized=%d estimated_review_minutes=%d deferred_review_cards=%d max_flashcards=%d daily_mins=%d",
		totalDueCards, materializedCards, finalReviewMinutes, deferredCards, maxFlashcards, dailyStudyMinutes)

	// Keep sessions near intended workload size
	tokenBudget := TargetSessionWords

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
	if finalDueReviewCards > 0 {
		bestNotebookID, selectedDueCards, err := s.queryNextDueReviewNotebook(now.Unix())
		if err != nil {
			return nil, err
		}
		if bestNotebookID == "" {
			return nil, fmt.Errorf("failed to resolve notebook for due review cards")
		}

		reviewCardsForTask := finalDueReviewCards
		if selectedDueCards < reviewCardsForTask {
			reviewCardsForTask = selectedDueCards
		}
		if reviewCardsForTask < 0 {
			reviewCardsForTask = 0
		}
		finalDueReviewCards = reviewCardsForTask
		finalReviewMinutes = int(math.Ceil(float64(reviewCardsForTask) * ReviewMinutesPerCard))
		deferredCards = totalDueCards - finalDueReviewCards
		if deferredCards < 0 {
			deferredCards = 0
		}

		utils.Warnf("[FLASHCARD_PIPELINE] synthetic_review_notebook_selected notebookID=%s dueCards=%d selectedDueCards=%d source=scheduler", bestNotebookID, finalDueReviewCards, selectedDueCards)

		reviewTask := models.ScheduledTask{
			ID:              models.ReviewTaskDailyID,
			ActionType:      "flashcard_review",
			Title:           fmt.Sprintf("Flashcard Review: %d cards", finalDueReviewCards),
			EstimateMinutes: finalReviewMinutes,
			Priority:        1,
			NotebookID:      bestNotebookID,
			Meta:            fmt.Sprintf("Spaced repetition review (%d cards)", finalDueReviewCards),
		}
		utils.Warnf("[FLASHCARD_PIPELINE] synthetic_review_task_created taskID=%s notebookID=%s dueCards=%d selectedDueCards=%d materializedCards=%d", reviewTask.ID, reviewTask.NotebookID, totalDueCards, selectedDueCards, finalDueReviewCards)
		tasks = append([]models.ScheduledTask{reviewTask}, tasks...)
	}

	for _, task := range tasks {
		totalLearningMinutes += task.EstimateMinutes
	}

	return &models.TodayPlan{
		Date:                now.Format("2006-01-02"),
		TotalMinutes:        dailyStudyMinutes,
		ReviewMinutes:       finalReviewMinutes,
		LearningMinutes:     totalLearningMinutes,
		DueReviewCards:      finalDueReviewCards,
		TotalDueReviewCards: totalDueCards,
		DeferredReviewCards: deferredCards,
		ActiveTopics:        activeTopics,
		Tasks:               tasks,
		IsEstimate:          len(tasks) == 0,
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

func parseTimeToMinutes(t string) (int, bool) {
	parts := strings.Split(t, ":")
	if len(parts) != 2 {
		return 0, false
	}
	var h, m int
	if _, err := fmt.Sscanf(t, "%d:%d", &h, &m); err != nil {
		return 0, false
	}
	if h < 0 || h > 23 || m < 0 || m > 59 {
		return 0, false
	}
	return h*60 + m, true
}

func calculateDurationMinutes(start, end string) int {
	startMins, ok1 := parseTimeToMinutes(start)
	endMins, ok2 := parseTimeToMinutes(end)
	if !ok1 || !ok2 {
		return 60 // Default fallback
	}
	diff := endMins - startMins
	if diff < 0 {
		diff += 1440 // Wraps around midnight
	}
	if diff == 0 {
		return 60 // Default fallback if start == end
	}
	return diff
}
