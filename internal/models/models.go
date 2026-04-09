package models

// ScheduledTask represents one actionable study task for the day.
type ScheduledTask struct {
	ID              string `json:"id"`
	ActionType      string `json:"action_type"`
	Title           string `json:"title"`
	TopicID         string `json:"topic_id,omitempty"`
	EstimateMinutes int    `json:"estimate_minutes"`
	Priority        int    `json:"priority"`
	Meta            string `json:"meta,omitempty"`
}

// TodayPlan is the scheduler output consumed by the dashboard.
type TodayPlan struct {
	Date            string          `json:"date"`
	TotalMinutes    int             `json:"total_minutes"`
	ReviewMinutes   int             `json:"review_minutes"`
	LearningMinutes int             `json:"learning_minutes"`
	DueReviewCards  int             `json:"due_review_cards"`
	ActiveTopics    []string        `json:"active_topics"`
	Tasks           []ScheduledTask `json:"tasks"`
}

// TopicSummary keeps scheduler queries simple and explicit.
type TopicSummary struct {
	ID     string
	Title  string
	Status string
}

// Chunk represents a retrieval chunk with metadata and future scoring hooks.
type Chunk struct {
	ID              string
	TopicID         string
	ParentID        string
	Text            string
	ImportanceScore float64
	WeaknessScore   float64
}
