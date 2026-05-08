package models

import (
	"time"

	fsrs "github.com/open-spaced-repetition/go-fsrs/v4"
)

// safeUint64FromInt clamps negative ints to 0 before casting to uint64.
func safeUint64FromInt(v int) uint64 {
	if v < 0 {
		return 0
	}
	return uint64(v)
}

// ScheduledTask represents one actionable study task for the day.
type ScheduledTask struct {
	ID              string `json:"id"`
	ActionType      string `json:"action_type"`
	Title           string `json:"title"`
	TopicID         string `json:"topic_id,omitempty"`
	StartPage       int    `json:"start_page,omitempty"`
	EndPage         int    `json:"end_page,omitempty"`
	EstimateMinutes int    `json:"estimate_minutes"`
	Priority        int    `json:"priority"`
	Meta            string `json:"meta,omitempty"`
}

type StudyTaskType string

const (
	StudyTaskTypeFlashcardReview StudyTaskType = "FLASHCARD_REVIEW"
	StudyTaskTypeReread          StudyTaskType = "REREAD"
	StudyTaskTypeQuiz            StudyTaskType = "QUIZ"
	StudyTaskTypeReading         StudyTaskType = "READING"
	StudyTaskTypeExaminer        StudyTaskType = "EXAMINER"
)

type StudyTaskStatus string

const (
	StudyTaskStatusPending   StudyTaskStatus = "PENDING"
	StudyTaskStatusActive    StudyTaskStatus = "ACTIVE"
	StudyTaskStatusCompleted StudyTaskStatus = "COMPLETED"
	StudyTaskStatusSkipped   StudyTaskStatus = "SKIPPED"
	StudyTaskStatusFailed    StudyTaskStatus = "FAILED"
)

// StudyQueueTask is the persisted queue task driving guided study progression.
type StudyQueueTask struct {
	ID          string          `json:"id"`
	NotebookID  string          `json:"notebook_id"`
	TopicID     string          `json:"topic_id,omitempty"`
	TaskType    StudyTaskType   `json:"task_type"`
	Status      StudyTaskStatus `json:"status"`
	Priority    int             `json:"priority"`
	CreatedAt   string          `json:"created_at"`
	ActivatedAt string          `json:"activated_at,omitempty"`
	CompletedAt string          `json:"completed_at,omitempty"`
	PayloadJSON string          `json:"payload_json,omitempty"`
	StartPage   int             `json:"start_page,omitempty"`
	EndPage     int             `json:"end_page,omitempty"`
}

// QueueState provides pending counts grouped by task type for dashboard summaries.
type QueueState struct {
	NotebookID string         `json:"notebook_id,omitempty"`
	Pending    map[string]int `json:"pending"`
	Total      int            `json:"total"`
}

// CompletionResult captures explicit completion outcome and optional explicit follow-up inserts.
type CompletionResult struct {
	Status    StudyTaskStatus  `json:"status"`
	Payload   string           `json:"payload_json,omitempty"`
	FollowUps []StudyQueueTask `json:"follow_ups,omitempty"`
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
	IsEstimate      bool            `json:"is_estimate"`
}

// TopicSummary keeps scheduler queries simple and explicit.
type TopicSummary struct {
	ID     string
	Title  string
	Status string
}

// ReadingTopicCursor contains page bounds and the active cursor for one reading topic.
type ReadingTopicCursor struct {
	ID                string
	Title             string
	StartPage         int
	EndPage           int
	CurrentPageCursor int
}

// Chunk represents a retrieval chunk with metadata and future scoring hooks.
type Chunk struct {
	ID              string
	TopicID         string
	ParentID        string
	Text            string
	ImportanceScore float64
	WeaknessScore   float64
	PageNum         int
}

// ChunkWithContext is the structured prompt context passed to LLM generation.
type ChunkWithContext struct {
	ChunkID  string `json:"chunk_id"`
	ParentID string `json:"parent_id,omitempty"`
	PageNum  int    `json:"page_num"`
	Text     string `json:"text"`
}

// Notebook represents a user-uploaded document (PDF, text, etc)
type Notebook struct {
	ID         string `json:"id"`
	Title      string `json:"title"`
	FilePath   string `json:"file_path"`
	FileType   string `json:"file_type"` // "pdf", "txt", "md"
	TopicID    string `json:"topic_id,omitempty"`
	Status     string `json:"status"`
	UploadedAt string `json:"uploaded_at"`
	PageCount  int    `json:"page_count,omitempty"`
	ChunkCount int    `json:"chunk_count"`
}

// NotebookChunk links a chunk to a notebook (many chunks per notebook)
type NotebookChunk struct {
	ID         string
	NotebookID string
	ChunkID    string
	PageNum    int // for PDFs
}

// NotebookTopicTreeTopic is one topic option nested under a notebook.
type NotebookTopicTreeTopic struct {
	TopicID string `json:"topic_id"`
	Title   string `json:"title"`
}

// NotebookTopicTreeNode is the notebook-scoped topic tree returned to the UI.
type NotebookTopicTreeNode struct {
	NotebookID string                   `json:"notebook_id"`
	Title      string                   `json:"title"`
	Topics     []NotebookTopicTreeTopic `json:"topics"`
}

// SyllabusChapterDraft represents one editable chapter range proposed during notebook ingestion.
type SyllabusChapterDraft struct {
	Title     string `json:"title"`
	StartPage int    `json:"start_page"`
	EndPage   int    `json:"end_page"`
}

// SyllabusDraft captures the backend-generated chapter draft shown in the Notebook verification modal.
type SyllabusDraft struct {
	NotebookID string                 `json:"notebook_id"`
	PageCount  int                    `json:"page_count"`
	Chapters   []SyllabusChapterDraft `json:"chapters"`
}

// ReaderSection is one ordered section used by the augmented reader.
type ReaderSection struct {
	ID      string `json:"id"`
	Heading string `json:"heading"`
	Content string `json:"content"`
	Order   int    `json:"order"`
	PageNum int    `json:"page_num"`
}

// ReaderTopicBundle contains notebook metadata plus section/page mapping for reader UI.
type ReaderTopicBundle struct {
	TopicID       string          `json:"topic_id"`
	TopicTitle    string          `json:"topic_title"`
	NotebookID    string          `json:"notebook_id,omitempty"`
	NotebookTitle string          `json:"notebook_title,omitempty"`
	NotebookURL   string          `json:"notebook_url,omitempty"`
	FileType      string          `json:"file_type,omitempty"`
	PageCount     int             `json:"page_count"`
	Sections      []ReaderSection `json:"sections"`
	Subtopics     []Subtopic      `json:"subtopics,omitempty"`
}

// QuizQuestion is a generated question persisted per topic.
type QuizQuestion struct {
	ID              string   `json:"id"`
	TopicID         string   `json:"topic_id"`
	SourceChunkID   string   `json:"source_chunk_id,omitempty"`
	Prompt          string   `json:"prompt"`
	Options         []string `json:"options"`
	CorrectAnswer   string   `json:"correct_answer"`
	Explanation     string   `json:"explanation"`
	Hint            string   `json:"hint,omitempty"`
	SourceHeading   string   `json:"source_heading,omitempty"`
	SourceSnippet   string   `json:"source_snippet,omitempty"`
	SourcePageStart int      `json:"source_page_start,omitempty"`
	SourcePageEnd   int      `json:"source_page_end,omitempty"`
	LLMModel        string   `json:"llm_model,omitempty"`
	PromptVersion   string   `json:"prompt_version,omitempty"`
}

// QuizScore is returned after scoring a user's answer.
type QuizScore struct {
	QuestionID    string `json:"question_id"`
	Correct       bool   `json:"correct"`
	Score         int    `json:"score"`
	Expected      string `json:"expected"`
	Feedback      string `json:"feedback"`
	Hint          string `json:"hint"`
	UserAnswer    string `json:"user_answer"`
	SourceHeading string `json:"source_heading,omitempty"`
}

// WrittenAnswer is returned after scoring a user's written response.
type WrittenAnswer struct {
	QuestionID    string `json:"question_id"`
	Score         int    `json:"score"`
	Feedback      string `json:"feedback"`
	UserAnswer    string `json:"user_answer"`
	SourceHeading string `json:"source_heading,omitempty"`
}

// WrittenQuestion is a persisted examiner prompt with lineage metadata.
type WrittenQuestion struct {
	ID              string `json:"id"`
	TopicID         string `json:"topic_id"`
	Prompt          string `json:"prompt"`
	SourceChunkID   string `json:"source_chunk_id,omitempty"`
	SourceHeading   string `json:"source_heading,omitempty"`
	SourcePageStart int    `json:"source_page_start,omitempty"`
	SourcePageEnd   int    `json:"source_page_end,omitempty"`
	LLMModel        string `json:"llm_model,omitempty"`
	PromptVersion   string `json:"prompt_version,omitempty"`
}

// Flashcard is a persisted review card scoped to one topic.
type Flashcard struct {
	ID            string `json:"id"`
	TopicID       string `json:"topic_id"`
	SourceChunkID string `json:"source_chunk_id,omitempty"`
	Prompt        string `json:"prompt"`
	Answer        string `json:"answer"`
	DueAt         int64  `json:"due_at,omitempty"`
	Suspended     bool   `json:"suspended"`
}

// FlashcardState stores the local review scheduler state in fsrs_cards.state_json.
type FlashcardState struct {
	Stability     float64 `json:"stability"`
	Difficulty    float64 `json:"difficulty"`
	ElapsedDays   int     `json:"elapsed_days"`
	ScheduledDays int     `json:"scheduled_days"`
	Reps          int     `json:"reps"`
	Lapses        int     `json:"lapses"`
	StateCode     int     `json:"state_code"`
}

// FSRSReviewLog stores generic review events for flashcards and future activity types.
type FSRSReviewLog struct {
	ID              string `json:"id"`
	TopicID         string `json:"topic_id"`
	ActivityType    string `json:"activity_type"`
	ReferenceID     string `json:"reference_id"`
	ReviewedAt      int64  `json:"reviewed_at"`
	Rating          int    `json:"rating"`
	ScheduledDays   int    `json:"scheduled_days"`
	StateBeforeJSON string `json:"state_before_json"`
	StateAfterJSON  string `json:"state_after_json"`
}

// FlashcardStateToCard converts our FlashcardState to go-fsrs Card
func FlashcardStateToCard(state FlashcardState, dueAt, lastReviewedAt int64) fsrs.Card {
	var dueTime, lastReviewTime time.Time
	if dueAt > 0 {
		dueTime = time.Unix(dueAt, 0)
	}
	if lastReviewedAt > 0 {
		lastReviewTime = time.Unix(lastReviewedAt, 0)
	}

	// Map StateCode to fsrs.State
	var fsrsState fsrs.State
	switch state.StateCode {
	case 0:
		fsrsState = fsrs.New
	case 1:
		fsrsState = fsrs.Learning
	case 2:
		fsrsState = fsrs.Review
	case 3:
		fsrsState = fsrs.Relearning
	default:
		fsrsState = fsrs.New
	}

	return fsrs.Card{
		Due:            dueTime,
		Stability:      state.Stability,
		Difficulty:     state.Difficulty,
		ElapsedDays:    safeUint64FromInt(state.ElapsedDays),
		ScheduledDays:  safeUint64FromInt(state.ScheduledDays),
		Reps:           safeUint64FromInt(state.Reps),
		Lapses:         safeUint64FromInt(state.Lapses),
		State:          fsrsState,
		LastReview:     lastReviewTime,
		RemainingSteps: 0, // Not tracked in our current implementation
	}
}

// CardToFlashcardState converts go-fsrs Card to our FlashcardState
func CardToFlashcardState(card fsrs.Card) FlashcardState {
	// Map fsrs.State to StateCode
	stateCode := 0
	switch card.State {
	case fsrs.New:
		stateCode = 0
	case fsrs.Learning:
		stateCode = 1
	case fsrs.Review:
		stateCode = 2
	case fsrs.Relearning:
		stateCode = 3
	}

	return FlashcardState{
		Stability:     card.Stability,
		Difficulty:    card.Difficulty,
		ElapsedDays:   int(card.ElapsedDays),
		ScheduledDays: int(card.ScheduledDays),
		Reps:          int(card.Reps),
		Lapses:        int(card.Lapses),
		StateCode:     stateCode,
	}
}

// Subtopic represents a logical section within a parent topic for micro-mission orchestration.
type Subtopic struct {
	ID            string `json:"id"`
	ParentTopicID string `json:"parent_topic_id"`
	Title         string `json:"title"`
	StartPage     int    `json:"start_page"`
	EndPage       int    `json:"end_page"`
	SearchSnippet string `json:"search_snippet,omitempty"`
	CreatedAt     string `json:"created_at,omitempty"`
	UpdatedAt     string `json:"updated_at,omitempty"`
}

// SubtopicExtractionResult is the structured output from LLM subtopic extraction.
type SubtopicExtractionResult struct {
	Subtopics []ExtractedSubtopic `json:"subtopics"`
}

// ExtractedSubtopic is one subtopic identified by the LLM with page boundaries.
type ExtractedSubtopic struct {
	Title         string                `json:"title"`
	StartPage     int                   `json:"start_page"`
	EndPage       int                   `json:"end_page"`
	SearchSnippet string                `json:"search_snippet"`
	Flashcards    []GeneratedFlashcard  `json:"flashcards"`
	QuizQuestion  GeneratedQuizQuestion `json:"quiz_question"`
}

// GeneratedFlashcard is a flashcard generated during subtopic extraction.
type GeneratedFlashcard struct {
	Prompt string `json:"prompt"`
	Answer string `json:"answer"`
}

// GeneratedQuizQuestion is a quiz question generated during subtopic extraction.
type GeneratedQuizQuestion struct {
	Prompt        string   `json:"prompt"`
	Options       []string `json:"options"`
	CorrectAnswer string   `json:"correct_answer"`
	Explanation   string   `json:"explanation"`
}
