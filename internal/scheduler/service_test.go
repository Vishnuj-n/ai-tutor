package scheduler

import (
	"fmt"
	"testing"
	"time"

	"ai-tutor/internal/models"
)

func TestBuildTodayPlanEmptyFallback(t *testing.T) {
	svc := New(
		WithQueryDueReviewCards(func(string) (int, error) { return 0, nil }),
		WithQueryActiveTopics(func(int) ([]string, error) { return nil, nil }),
		WithQueryLearningTopics(func(int) ([]models.TopicSummary, error) { return nil, nil }),
		WithCountLearnedTopics(func() (int, error) { return 0, nil }),
	)

	plan, err := svc.BuildTodayPlan(time.Date(2026, 4, 12, 9, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("BuildTodayPlan returned error: %v", err)
	}

	if plan.ReviewMinutes != 0 {
		t.Fatalf("expected review minutes to be 0, got %d", plan.ReviewMinutes)
	}
	if plan.LearningMinutes != DefaultDailyStudyMinutes {
		t.Fatalf("expected learning minutes to be %d, got %d", DefaultDailyStudyMinutes, plan.LearningMinutes)
	}
	if len(plan.Tasks) != 1 || plan.Tasks[0].ActionType != "explore" {
		t.Fatalf("expected single explore fallback task, got %#v", plan.Tasks)
	}
}

func TestBuildTodayPlanPrioritizesDueReviews(t *testing.T) {
	svc := New(
		WithQueryDueReviewCards(func(string) (int, error) { return 10, nil }),
		WithQueryActiveTopics(func(int) ([]string, error) { return []string{"OS", "Networks"}, nil }),
		WithQueryLearningTopics(func(int) ([]models.TopicSummary, error) {
			return []models.TopicSummary{
				{ID: "topic-a", Title: "Topic A"},
				{ID: "topic-b", Title: "Topic B"},
			}, nil
		}),
		WithCountLearnedTopics(func() (int, error) { return 1, nil }),
	)

	plan, err := svc.BuildTodayPlan(time.Date(2026, 4, 12, 9, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("BuildTodayPlan returned error: %v", err)
	}

	if plan.ReviewMinutes != 20 {
		t.Fatalf("expected review minutes to be 20, got %d", plan.ReviewMinutes)
	}
	if plan.LearningMinutes != 70 {
		t.Fatalf("expected learning minutes to be 70, got %d", plan.LearningMinutes)
	}
	if len(plan.Tasks) < 3 {
		t.Fatalf("expected at least 3 tasks, got %d", len(plan.Tasks))
	}

	if plan.Tasks[0].ActionType != "review" || plan.Tasks[0].Priority != 1 {
		t.Fatalf("expected first task to be priority-1 review, got %#v", plan.Tasks[0])
	}
	if plan.Tasks[1].ActionType != "read" || plan.Tasks[1].Priority != 2 {
		t.Fatalf("expected second task to be read priority-2, got %#v", plan.Tasks[1])
	}
	if plan.Tasks[2].ActionType != "read" || plan.Tasks[2].Priority != 3 {
		t.Fatalf("expected third task to be read priority-3, got %#v", plan.Tasks[2])
	}

	assertPlanMinutesWithinBudget(t, plan)
}

func TestBuildTodayPlanReviewMinutesCap(t *testing.T) {
	svc := New(
		WithQueryDueReviewCards(func(string) (int, error) { return 100, nil }),
		WithQueryActiveTopics(func(int) ([]string, error) { return []string{"OS"}, nil }),
		WithQueryLearningTopics(func(int) ([]models.TopicSummary, error) {
			return []models.TopicSummary{
				{ID: "topic-a", Title: "Topic A"},
				{ID: "topic-b", Title: "Topic B"},
			}, nil
		}),
		WithCountLearnedTopics(func() (int, error) { return 0, nil }),
	)

	plan, err := svc.BuildTodayPlan(time.Date(2026, 4, 12, 9, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("BuildTodayPlan returned error: %v", err)
	}

	if plan.ReviewMinutes != MaxReviewMinutes {
		t.Fatalf("expected review minutes to be capped at %d, got %d", MaxReviewMinutes, plan.ReviewMinutes)
	}
	if plan.LearningMinutes != DefaultDailyStudyMinutes-MaxReviewMinutes {
		t.Fatalf("unexpected learning minutes: %d", plan.LearningMinutes)
	}
	if plan.ReviewMinutes > DefaultDailyStudyMinutes-MinLearningMinutes {
		t.Fatalf("review minutes violated min-learning guardrail: review=%d", plan.ReviewMinutes)
	}

	assertPlanMinutesWithinBudget(t, plan)
}

func TestBuildTodayPlanAddsLearnedTopicBonusesWhenTimeRemains(t *testing.T) {
	svc := New(
		WithQueryDueReviewCards(func(string) (int, error) { return 0, nil }),
		WithQueryActiveTopics(func(int) ([]string, error) { return []string{"OS"}, nil }),
		WithQueryLearningTopics(func(int) ([]models.TopicSummary, error) { return nil, nil }),
		WithCountLearnedTopics(func() (int, error) { return 2, nil }),
	)

	plan, err := svc.BuildTodayPlan(time.Date(2026, 4, 12, 9, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("BuildTodayPlan returned error: %v", err)
	}

	if len(plan.Tasks) != 2 {
		t.Fatalf("expected quiz and socratic tasks, got %#v", plan.Tasks)
	}
	if plan.Tasks[0].ActionType != "quiz" || plan.Tasks[0].Priority != 1 {
		t.Fatalf("expected first task to be quiz priority-1, got %#v", plan.Tasks[0])
	}
	if plan.Tasks[1].ActionType != "socratic" || plan.Tasks[1].Priority != 2 {
		t.Fatalf("expected second task to be socratic priority-2, got %#v", plan.Tasks[1])
	}

	assertPlanMinutesWithinBudget(t, plan)
}

func TestBuildTodayPlanQueryDueReviewCardsReturnsError(t *testing.T) {
	expectedErr := fmt.Errorf("database connection failed")
	svc := New(
		WithQueryDueReviewCards(func(string) (int, error) { return 0, expectedErr }),
		WithQueryActiveTopics(func(int) ([]string, error) { return nil, nil }),
		WithQueryLearningTopics(func(int) ([]models.TopicSummary, error) { return nil, nil }),
		WithCountLearnedTopics(func() (int, error) { return 0, nil }),
	)

	plan, err := svc.BuildTodayPlan(time.Date(2026, 4, 12, 9, 0, 0, 0, time.UTC))
	if err != expectedErr {
		t.Fatalf("expected error %v, got %v", expectedErr, err)
	}
	if plan != nil {
		t.Fatalf("expected nil plan on error, got %#v", plan)
	}
}

func TestBuildTodayPlanQueryActiveTopicsReturnsError(t *testing.T) {
	expectedErr := fmt.Errorf("topics query failed")
	svc := New(
		WithQueryDueReviewCards(func(string) (int, error) { return 5, nil }),
		WithQueryActiveTopics(func(int) ([]string, error) { return nil, expectedErr }),
		WithQueryLearningTopics(func(int) ([]models.TopicSummary, error) { return nil, nil }),
		WithCountLearnedTopics(func() (int, error) { return 0, nil }),
	)

	plan, err := svc.BuildTodayPlan(time.Date(2026, 4, 12, 9, 0, 0, 0, time.UTC))
	if err != expectedErr {
		t.Fatalf("expected error %v, got %v", expectedErr, err)
	}
	if plan != nil {
		t.Fatalf("expected nil plan on error, got %#v", plan)
	}
}

func TestBuildTodayPlanQueryLearningTopicsReturnsError(t *testing.T) {
	expectedErr := fmt.Errorf("learning topics query failed")
	svc := New(
		WithQueryDueReviewCards(func(string) (int, error) { return 0, nil }),
		WithQueryActiveTopics(func(int) ([]string, error) { return []string{}, nil }),
		WithQueryLearningTopics(func(int) ([]models.TopicSummary, error) { return nil, expectedErr }),
		WithCountLearnedTopics(func() (int, error) { return 0, nil }),
	)

	plan, err := svc.BuildTodayPlan(time.Date(2026, 4, 12, 9, 0, 0, 0, time.UTC))
	if err != expectedErr {
		t.Fatalf("expected error %v, got %v", expectedErr, err)
	}
	if plan != nil {
		t.Fatalf("expected nil plan on error, got %#v", plan)
	}
}

func TestBuildTodayPlanCountLearnedTopicsReturnsError(t *testing.T) {
	expectedErr := fmt.Errorf("learned topics count failed")
	svc := New(
		WithQueryDueReviewCards(func(string) (int, error) { return 0, nil }),
		WithQueryActiveTopics(func(int) ([]string, error) { return []string{}, nil }),
		WithQueryLearningTopics(func(int) ([]models.TopicSummary, error) { return []models.TopicSummary{}, nil }),
		WithCountLearnedTopics(func() (int, error) { return 0, expectedErr }),
	)

	plan, err := svc.BuildTodayPlan(time.Date(2026, 4, 12, 9, 0, 0, 0, time.UTC))
	if err != expectedErr {
		t.Fatalf("expected error %v, got %v", expectedErr, err)
	}
	if plan != nil {
		t.Fatalf("expected nil plan on error, got %#v", plan)
	}
}

func assertPlanMinutesWithinBudget(t *testing.T, plan *models.TodayPlan) {
	t.Helper()

	totalTaskMinutes := 0
	for _, task := range plan.Tasks {
		totalTaskMinutes += task.EstimateMinutes
	}
	if totalTaskMinutes > DefaultDailyStudyMinutes {
		t.Fatalf("task minutes exceeded daily budget: %d > %d", totalTaskMinutes, DefaultDailyStudyMinutes)
	}
	if plan.ReviewMinutes+plan.LearningMinutes != DefaultDailyStudyMinutes {
		t.Fatalf("review+learning minutes must equal daily budget, got review=%d learning=%d total=%d",
			plan.ReviewMinutes, plan.LearningMinutes, DefaultDailyStudyMinutes)
	}
}
