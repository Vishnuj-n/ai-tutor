package scheduler

import (
	"testing"
	"time"

	"ai-tutor/internal/models"
)

func TestBuildTodayPlanGeneratesContextLockedReadTask(t *testing.T) {
	svc := New(
		WithQueryDueReviewCards(func(int64) (int, error) { return 10, nil }),
		WithQueryDailyStudyMinutes(func() (int, error) { return 90, nil }),
		WithQueryNextReadingTopic(func() (*models.ReadingTopicCursor, error) {
			return &models.ReadingTopicCursor{
				ID:                "ch1",
				Title:             "Chapter 1",
				StartPage:         1,
				EndPage:           40,
				CurrentPageCursor: 1,
			}, nil
		}),
	)

	plan, err := svc.BuildTodayPlan(time.Date(2026, 4, 12, 9, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("BuildTodayPlan returned error: %v", err)
	}

	if plan.ReviewMinutes != 5 {
		t.Fatalf("expected review minutes 5, got %d", plan.ReviewMinutes)
	}
	if plan.LearningMinutes != 85 {
		t.Fatalf("expected learning minutes 85, got %d", plan.LearningMinutes)
	}
	if len(plan.Tasks) != 1 {
		t.Fatalf("expected exactly one read task, got %d", len(plan.Tasks))
	}

	task := plan.Tasks[0]
	if task.ActionType != "read" {
		t.Fatalf("expected read task, got %s", task.ActionType)
	}
	if task.StartPage != 1 || task.EndPage != 34 {
		t.Fatalf("expected pages 1-34, got %d-%d", task.StartPage, task.EndPage)
	}
	if task.Title != "Read: Chapter 1 (Pages 1 to 34)" {
		t.Fatalf("unexpected task title: %s", task.Title)
	}
}

func TestBuildTodayPlanClampNearTopicEnd(t *testing.T) {
	svc := New(
		WithQueryDueReviewCards(func(int64) (int, error) { return 10, nil }),
		WithQueryDailyStudyMinutes(func() (int, error) { return 30, nil }),
		WithQueryNextReadingTopic(func() (*models.ReadingTopicCursor, error) {
			return &models.ReadingTopicCursor{
				ID:                "ch1",
				Title:             "Chapter 1",
				StartPage:         1,
				EndPage:           20,
				CurrentPageCursor: 13,
			}, nil
		}),
	)

	plan, err := svc.BuildTodayPlan(time.Date(2026, 4, 12, 9, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("BuildTodayPlan returned error: %v", err)
	}

	if len(plan.Tasks) != 1 {
		t.Fatalf("expected one read task, got %d", len(plan.Tasks))
	}

	task := plan.Tasks[0]
	if task.StartPage != 13 {
		t.Fatalf("expected start page 13, got %d", task.StartPage)
	}
	if task.EndPage != 20 {
		t.Fatalf("expected end page clamped to 20, got %d", task.EndPage)
	}
}

func TestBuildTodayPlanNoTopicReturnsEstimate(t *testing.T) {
	svc := New(
		WithQueryDueReviewCards(func(int64) (int, error) { return 0, nil }),
		WithQueryDailyStudyMinutes(func() (int, error) { return 90, nil }),
		WithQueryNextReadingTopic(func() (*models.ReadingTopicCursor, error) {
			return nil, nil
		}),
	)

	plan, err := svc.BuildTodayPlan(time.Date(2026, 4, 12, 9, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("BuildTodayPlan returned error: %v", err)
	}

	if len(plan.Tasks) != 0 {
		t.Fatalf("expected no tasks, got %d", len(plan.Tasks))
	}
	if !plan.IsEstimate {
		t.Fatalf("expected estimate=true when no reading task exists")
	}
}

func TestResolvePageWindowRejectsZeroPagesToRead(t *testing.T) {
	start, end, ok := resolvePageWindow(models.ReadingTopicCursor{
		ID:                "ch1",
		Title:             "Chapter 1",
		StartPage:         1,
		EndPage:           20,
		CurrentPageCursor: 3,
	}, 0)

	if ok {
		t.Fatalf("expected ok=false, got ok=true with window %d-%d", start, end)
	}
	if start != 0 || end != 0 {
		t.Fatalf("expected window 0-0 for zero pages, got %d-%d", start, end)
	}
}
