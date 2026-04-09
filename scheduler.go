package main

import (
	"fmt"
	"time"
)

const (
	defaultDailyStudyMinutes = 90
	minLearningMinutes       = 20
	maxReviewMinutes         = 60
)

// SchedulerService builds one daily plan that can include all study modes.
type SchedulerService struct{}

func NewSchedulerService() *SchedulerService {
	return &SchedulerService{}
}

func (s *SchedulerService) BuildTodayPlan(now time.Time) (*TodayPlan, error) {
	dueCards, err := queryDueReviewCards(now)
	if err != nil {
		return nil, err
	}

	activeTopics, err := queryActiveTopics(3)
	if err != nil {
		return nil, err
	}

	learningTopics, err := queryLearningTopics(2)
	if err != nil {
		return nil, err
	}

	learnedTopicsCount, err := countLearnedTopics()
	if err != nil {
		return nil, err
	}

	reviewMinutes := dueCards * 2
	if reviewMinutes > maxReviewMinutes {
		reviewMinutes = maxReviewMinutes
	}

	if reviewMinutes > defaultDailyStudyMinutes-minLearningMinutes {
		reviewMinutes = defaultDailyStudyMinutes - minLearningMinutes
	}

	if dueCards == 0 {
		reviewMinutes = 0
	}

	learningMinutes := defaultDailyStudyMinutes - reviewMinutes

	tasks := make([]ScheduledTask, 0, 6)
	priority := 1
	usedMinutes := 0

	if dueCards > 0 {
		tasks = append(tasks, ScheduledTask{
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
			tasks = append(tasks, ScheduledTask{
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
		remaining := defaultDailyStudyMinutes - usedMinutes
		if remaining >= 15 {
			tasks = append(tasks, ScheduledTask{
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

		remaining = defaultDailyStudyMinutes - usedMinutes
		if remaining >= 10 {
			tasks = append(tasks, ScheduledTask{
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
		tasks = append(tasks, ScheduledTask{
			ID:              "task-explore",
			ActionType:      "explore",
			Title:           "Optional Exploration",
			EstimateMinutes: 20,
			Priority:        1,
			Meta:            "Pick any topic and do a short learning pass",
		})
	}

	return &TodayPlan{
		Date:            now.Format("2006-01-02"),
		TotalMinutes:    defaultDailyStudyMinutes,
		ReviewMinutes:   reviewMinutes,
		LearningMinutes: learningMinutes,
		DueReviewCards:  dueCards,
		ActiveTopics:    activeTopics,
		Tasks:           tasks,
	}, nil
}

func queryDueReviewCards(now time.Time) (int, error) {
	var count int
	err := db.QueryRow(`
		SELECT COUNT(*)
		FROM fsrs_cards
		WHERE suspended = 0
		  AND due_at IS NOT NULL
		  AND due_at <= ?
	`, now.Format(time.RFC3339)).Scan(&count)
	if err != nil {
		return 0, err
	}
	return count, nil
}

func queryActiveTopics(limit int) ([]string, error) {
	rows, err := db.Query(`
		SELECT title
		FROM topics
		WHERE status IN ('reading', 'learned')
		ORDER BY updated_at DESC, created_at DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	active := []string{}
	for rows.Next() {
		var title string
		if err := rows.Scan(&title); err != nil {
			return nil, err
		}
		active = append(active, title)
	}

	return active, nil
}

func queryLearningTopics(limit int) ([]TopicSummary, error) {
	rows, err := db.Query(`
		SELECT id, title, status
		FROM topics
		WHERE status IN ('unseen', 'reading')
		ORDER BY updated_at ASC, created_at ASC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	topics := []TopicSummary{}
	for rows.Next() {
		var topic TopicSummary
		if err := rows.Scan(&topic.ID, &topic.Title, &topic.Status); err != nil {
			return nil, err
		}
		topics = append(topics, topic)
	}

	return topics, nil
}

func countLearnedTopics() (int, error) {
	var count int
	err := db.QueryRow(`
		SELECT COUNT(*)
		FROM topics
		WHERE status = 'learned'
	`).Scan(&count)
	if err != nil {
		return 0, err
	}
	return count, nil
}
