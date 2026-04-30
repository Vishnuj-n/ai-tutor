package orchestrator

import (
	"fmt"
	"time"

	"ai-tutor/internal/db"
	"ai-tutor/internal/models"
)

const (
	// Task limits
	maxReviewTasks  = 10
	maxReadingTasks = 3
	minTasks        = 5
	maxTasks        = 10

	// Time estimates in minutes
	estimateFlashcard = 2
	estimateQuiz      = 5
	estimateWritten   = 8

	// Reading speed (pages per minute)
	readingSpeedPagesPerMinute = 1.0
)

// Service provides task orchestration functionality
type Service struct{}

// NewService creates a new orchestrator service
func NewService() *Service {
	return &Service{}
}

// GetDailyAgenda generates the daily agenda combining review and reading tasks
func (s *Service) GetDailyAgenda() ([]models.ScheduledTask, error) {
	now := time.Now().Unix()

	// Get daily study minutes
	dailyMinutes, err := db.GetDailyStudyMinutes()
	if err != nil {
		return nil, fmt.Errorf("failed to get daily study minutes: %w", err)
	}

	// Get due review items from assessment_fsrs
	reviewTasks, reviewMinutes, err := s.getReviewTasks(now)
	if err != nil {
		return nil, fmt.Errorf("failed to get review tasks: %w", err)
	}

	// Calculate remaining time for reading
	remainingMinutes := dailyMinutes - reviewMinutes
	if remainingMinutes < 0 {
		remainingMinutes = 0
	}

	// Get active reading notebooks
	readingTasks, err := s.getReadingTasks(remainingMinutes)
	if err != nil {
		return nil, fmt.Errorf("failed to get reading tasks: %w", err)
	}

	// Combine tasks: Priority 1 (review) first, then Priority 2 (reading)
	allTasks := append(reviewTasks, readingTasks...)

	// Enforce task limit (5-10 tasks max)
	if len(allTasks) > maxTasks {
		allTasks = allTasks[:maxTasks]
	}

	// If we have fewer than 5 tasks, return as-is (empty state handling)
	// The requirement states 5-10 tasks max, but also requires empty state handling
	// when no tasks are available. We don't artificially pad with more reading tasks
	// beyond what's available.

	return allTasks, nil
}

// getReviewTasks queries due review items from assessment_fsrs
func (s *Service) getReviewTasks(now int64) ([]models.ScheduledTask, int, error) {
	items, err := db.GetDueAssessmentItems(now, maxReviewTasks)
	if err != nil {
		return nil, 0, err
	}

	var tasks []models.ScheduledTask
	var totalMinutes int
	reviewIndex := 1

	for _, item := range items {
		// Determine estimate based on activity type
		estimate := estimateQuiz // default
		meta := "quiz"
		switch item.ActivityType {
		case "flashcard":
			estimate = estimateFlashcard
			meta = "flashcard"
		case "quiz_question":
			estimate = estimateQuiz
			meta = "quiz"
		case "written_question":
			estimate = estimateWritten
			meta = "written"
		}

		task := models.ScheduledTask{
			ID:              fmt.Sprintf("review-%d", reviewIndex),
			ActionType:      "Review",
			Title:           fmt.Sprintf("Review %s: %s", meta, item.TopicTitle),
			TopicID:         item.TopicID,
			StartPage:       item.CurrentCursor,
			EndPage:         item.EndPage,
			EstimateMinutes: estimate,
			Priority:        1,
			Meta:            meta,
		}

		tasks = append(tasks, task)
		totalMinutes += estimate
		reviewIndex++
	}

	return tasks, totalMinutes, nil
}

// getReadingTasks queries active notebooks and creates reading tasks
func (s *Service) getReadingTasks(availableMinutes int) ([]models.ScheduledTask, error) {
	notebooks, err := db.GetActiveNotebooks(maxReadingTasks)
	if err != nil {
		return nil, err
	}

	var tasks []models.ScheduledTask
	readingIndex := 1
	remainingMinutes := availableMinutes

	for _, nb := range notebooks {
		// Stop if no time remaining
		if remainingMinutes <= 0 {
			break
		}

		startPage := nb.CurrentCursor
		var endPage int

		// Use mission_end_page if set, otherwise calculate based on remaining minutes
		if nb.MissionEndPage > 0 {
			endPage = nb.MissionEndPage
		} else {
			availablePages := int(float64(remainingMinutes) * readingSpeedPagesPerMinute)
			endPage = nb.CurrentCursor + availablePages
		}

		// Cap at page count if available
		if nb.PageCount > 0 && endPage > nb.PageCount {
			endPage = nb.PageCount
		}

		// Only create task if there's progress to be made
		if endPage > startPage {
			task := models.ScheduledTask{
				ID:              fmt.Sprintf("read-%d", readingIndex),
				ActionType:      "Read",
				Title:           fmt.Sprintf("Read: %s", nb.Title),
				TopicID:         nb.TopicID,
				StartPage:       startPage,
				EndPage:         endPage,
				EstimateMinutes: endPage - startPage,
				Priority:        2,
				Meta:            "reading",
			}
			tasks = append(tasks, task)
			remainingMinutes -= task.EstimateMinutes
			readingIndex++
		}
	}

	return tasks, nil
}
