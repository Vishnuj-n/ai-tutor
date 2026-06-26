package scheduler

import (
	"fmt"
	"testing"
	"time"

	"ai-tutor/internal/models"
)

func TestBuildTodayPlanGeneratesContextLockedReadTask(t *testing.T) {
	svc := New(nil, Dependencies{
		QueryDueReviewCards: func(int64) (int, error) { return 10, nil }, // 10 cards * 0.5m = 5 min review
		QueryUserSettings: func() (*models.UserSettings, error) {
			return &models.UserSettings{MaxFlashcardsPerSession: 30, StudyStartTime: "17:00", StudyEndTime: "18:30", RemindersEnabled: true}, nil
		},
		QueryNextReadingTopic: func() (models.ReadingTopicCursor, bool, error) {
			return models.ReadingTopicCursor{
				ID:                "ch1",
				Title:             "Chapter 1",
				StartPage:         1,
				EndPage:           40,
				CurrentPageCursor: 1,
				NotebookID:        "nb-1",
			}, true, nil
		},
		QueryTokensPerPageMap: func(topicID string, start, end int) (map[int]int, error) {
			// Simulate exactly 500 words per page to allow predictable math
			result := make(map[int]int)
			for page := start; page <= end; page++ {
				result[page] = 500
			}
			return result, nil
		},
		QueryNextDueReviewNotebook: func(now int64) (string, int, error) {
			return "nb-1", 10, nil
		},
	})

	plan, err := svc.BuildTodayPlan(time.Date(2026, 4, 12, 9, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("BuildTodayPlan returned error: %v", err)
	}

	if plan.ReviewMinutes != 5 {
		t.Errorf("expected 5 review minutes, got %d", plan.ReviewMinutes)
	}

	if len(plan.Tasks) != 2 {
		t.Fatalf("expected 2 tasks (1 review, 1 reading), got %d", len(plan.Tasks))
	}

	task := plan.Tasks[1] // Reading task is second
	if task.StartPage != 1 {
		t.Errorf("expected StartPage=1, got %d", task.StartPage)
	}
	// Reading budget allows 17,000 words, but TargetSessionWords caps it at 2500.
	// 2500 words / 500 words-per-page = 5 pages. Start 1 -> End 5.
	if task.EndPage != 5 {
		t.Errorf("expected EndPage=5 based on token cap, got %d", task.EndPage)
	}
	// Estimate minutes: 2500 words / 200 wpm = 12.5 -> ceil -> 13 mins
	if task.EstimateMinutes != 13 {
		t.Errorf("expected EstimateMinutes=13, got %d", task.EstimateMinutes)
	}
}

func TestBuildTodayPlanWithTokenQueryFailureFallback(t *testing.T) {
	svc := New(nil, Dependencies{
		QueryDueReviewCards: func(int64) (int, error) { return 0, nil },
		QueryUserSettings: func() (*models.UserSettings, error) {
			return &models.UserSettings{MaxFlashcardsPerSession: 30, StudyStartTime: "17:00", StudyEndTime: "18:30", RemindersEnabled: true}, nil
		},
		QueryNextReadingTopic: func() (models.ReadingTopicCursor, bool, error) {
			return models.ReadingTopicCursor{
				ID:                "ocr1",
				Title:             "Scanned Document",
				StartPage:         1,
				EndPage:           20,
				CurrentPageCursor: 1,
				NotebookID:        "nb-1",
			}, true, nil
		},
		QueryTokensPerPageMap: func(string, int, int) (map[int]int, error) {
			// Simulate token query failure (e.g., OCR-heavy pages with no text layer)
			return nil, fmt.Errorf("no chunks found")
		},
		QueryNextDueReviewNotebook: func(now int64) (string, int, error) {
			return "", 0, nil
		},
	})

	plan, err := svc.BuildTodayPlan(time.Date(2026, 5, 10, 9, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("BuildTodayPlan returned error: %v", err)
	}

	if len(plan.Tasks) != 1 {
		t.Fatalf("expected one read task, got %d", len(plan.Tasks))
	}

	task := plan.Tasks[0]
	// With token query failure, uses FallbackWordsPerPage (500)
	// Budget 2500 words / 500 = 5 pages -> EndPage = 5
	if task.EndPage != 5 {
		t.Errorf("expected fallback end page 5, got %d", task.EndPage)
	}
	// Estimate minutes fallback: 5 pages * 2.5 mins/page (MinutesPerPage) = 12.5 -> ceil -> 13
	if task.EstimateMinutes != 13 {
		t.Errorf("expected fallback EstimateMinutes=13, got %d", task.EstimateMinutes)
	}
}

func TestBuildTodayPlanClampWindowAbsorbsRemainingPages(t *testing.T) {
	svc := New(nil, Dependencies{
		QueryDueReviewCards: func(int64) (int, error) { return 0, nil },
		QueryUserSettings: func() (*models.UserSettings, error) {
			return &models.UserSettings{MaxFlashcardsPerSession: 30, StudyStartTime: "17:00", StudyEndTime: "18:30", RemindersEnabled: true}, nil
		},
		QueryNextReadingTopic: func() (models.ReadingTopicCursor, bool, error) {
			return models.ReadingTopicCursor{
				ID:                "ch2",
				Title:             "Chapter 2",
				StartPage:         1,
				EndPage:           7, // Small chapter, only 7 pages total
				CurrentPageCursor: 1,
				NotebookID:        "nb-1",
			}, true, nil
		},
		QueryTokensPerPageMap: func(topicID string, start, end int) (map[int]int, error) {
			result := make(map[int]int)
			for page := start; page <= end; page++ {
				result[page] = 500
			}
			return result, nil
		},
		QueryNextDueReviewNotebook: func(now int64) (string, int, error) {
			return "", 0, nil
		},
	})

	plan, err := svc.BuildTodayPlan(time.Now())
	if err != nil {
		t.Fatalf("BuildTodayPlan returned error: %v", err)
	}

	task := plan.Tasks[0]
	// Token limit naturally allows 5 pages (1-5).
	// Remaining pages in topic = 2 (pages 6 and 7).
	// ClampWindowPages = 4. Since 2 <= 4, it should absorb the rest and finish the chapter.
	if task.EndPage != 7 {
		t.Errorf("expected clamp window to absorb remaining pages and return 7, got %d", task.EndPage)
	}
}

func TestBuildTodayPlanNoReadingTopic(t *testing.T) {
	svc := New(nil, Dependencies{
		QueryDueReviewCards: func(int64) (int, error) { return 20, nil }, // 20 cards * 0.5 = 10 mins
		QueryUserSettings: func() (*models.UserSettings, error) {
			return &models.UserSettings{MaxFlashcardsPerSession: 20, StudyStartTime: "17:00", StudyEndTime: "17:30", RemindersEnabled: true}, nil
		},
		QueryNextReadingTopic: func() (models.ReadingTopicCursor, bool, error) {
			return models.ReadingTopicCursor{}, false, nil // No topic found
		},
		QueryTokensPerPageMap: func(string, int, int) (map[int]int, error) {
			return map[int]int{1: 1000}, nil
		},
		QueryNextDueReviewNotebook: func(now int64) (string, int, error) {
			return "nb-1", 20, nil
		},
	})

	plan, err := svc.BuildTodayPlan(time.Now())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if plan.ReviewMinutes != 10 {
		t.Errorf("expected 10 review minutes, got %d", plan.ReviewMinutes)
	}
	if len(plan.Tasks) != 1 {
		t.Fatalf("expected 1 task (review only), got %d", len(plan.Tasks))
	}
	if plan.Tasks[0].ActionType != "flashcard_review" {
		t.Errorf("expected review task, got %s", plan.Tasks[0].ActionType)
	}
	if len(plan.ActiveTopics) != 0 {
		t.Errorf("expected 0 active topics, got %d", len(plan.ActiveTopics))
	}
}
