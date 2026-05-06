package subtopic

import (
	"encoding/json"
	"fmt"
	"strings"

	"ai-tutor/internal/db"
	"ai-tutor/internal/models"
	"ai-tutor/internal/study"

	"github.com/google/uuid"
)

// Worker handles greedy subtopic extraction and assessment generation.
type Worker struct {
	studyService *study.StudyService
}

// NewWorker creates a new subtopic worker.
func NewWorker(studyService *study.StudyService) *Worker {
	return &Worker{
		studyService: studyService,
	}
}

// Config holds configuration for subtopic extraction.
type Config struct {
	NotebookID        string
	ParentTopicID     string
	StartPage         int
	EndPage           int
	DailyStudyMinutes int
	LookaheadMissions int // Number of missions to pre-generate (default: 2)
}

// ExtractSubtopics performs greedy subtopic extraction for a page range.
// It uses a single-pass LLM call to extract subtopics and generate assessments.
func (w *Worker) ExtractSubtopics(cfg Config) (*ExtractionResult, error) {
	if cfg.NotebookID == "" {
		return nil, fmt.Errorf("notebook ID is required")
	}
	if cfg.ParentTopicID == "" {
		return nil, fmt.Errorf("parent topic ID is required")
	}
	if cfg.StartPage < 0 || cfg.EndPage < cfg.StartPage {
		return nil, fmt.Errorf("invalid page range: start=%d end=%d", cfg.StartPage, cfg.EndPage)
	}
	if cfg.DailyStudyMinutes <= 0 {
		cfg.DailyStudyMinutes = 90 // Default to 90 minutes
	}
	if cfg.LookaheadMissions <= 0 {
		cfg.LookaheadMissions = 2 // Default to 2 missions lookahead
	}

	// Calculate page budget based on time budget
	// Assume average reading speed: 1 page per 2.5 minutes
	pagesPerMission := cfg.DailyStudyMinutes / 2
	if pagesPerMission < 5 {
		pagesPerMission = 5 // Minimum 5 pages per mission
	}
	if pagesPerMission > 15 {
		pagesPerMission = 15 // Maximum 15 pages per mission
	}

	// Calculate total pages to process (lookahead missions)
	totalPagesToProcess := pagesPerMission * cfg.LookaheadMissions
	availablePages := cfg.EndPage - cfg.StartPage + 1
	if totalPagesToProcess > availablePages {
		totalPagesToProcess = availablePages
	}

	// Extract subtopics for the calculated page range
	extractionEndPage := cfg.StartPage + totalPagesToProcess - 1
	if extractionEndPage > cfg.EndPage {
		extractionEndPage = cfg.EndPage
	}

	subtopics, err := w.extractWithLLM(cfg.NotebookID, cfg.ParentTopicID, cfg.StartPage, extractionEndPage)
	if err != nil {
		return nil, fmt.Errorf("LLM extraction failed: %w", err)
	}

	// Persist subtopics to database
	for _, extracted := range subtopics.Subtopics {
		// Truncate subtopic boundaries if they exceed the mission budget
		if extracted.EndPage > extractionEndPage {
			extracted.EndPage = extractionEndPage
		}

		subtopic := models.Subtopic{
			ID:            uuid.NewString(),
			ParentTopicID: cfg.ParentTopicID,
			Title:         extracted.Title,
			StartPage:     extracted.StartPage,
			EndPage:       extracted.EndPage,
			SearchSnippet: extracted.SearchSnippet,
		}

		if err := db.CreateSubtopic(subtopic); err != nil {
			return nil, fmt.Errorf("failed to persist subtopic: %w", err)
		}

		// Generate and persist flashcards for this subtopic
		if err := w.persistFlashcards(cfg.ParentTopicID, extracted.Flashcards, extracted.StartPage, extracted.EndPage); err != nil {
			return nil, fmt.Errorf("failed to persist flashcards: %w", err)
		}

		// Generate and persist quiz question for this subtopic
		if err := w.persistQuizQuestion(cfg.ParentTopicID, extracted.QuizQuestion, extracted.StartPage, extracted.EndPage); err != nil {
			return nil, fmt.Errorf("failed to persist quiz question: %w", err)
		}
	}

	return &ExtractionResult{
		SubtopicsCreated:   len(subtopics.Subtopics),
		FlashcardsCreated:  w.countFlashcards(subtopics),
		QuestionsCreated:   len(subtopics.Subtopics),
		PageRangeProcessed: fmt.Sprintf("%d-%d", cfg.StartPage, extractionEndPage),
		PagesPerMission:    pagesPerMission,
	}, nil
}

// extractWithLLM calls the LLM to extract subtopics and generate assessments in a single pass.
func (w *Worker) extractWithLLM(notebookID, parentTopicID string, startPage, endPage int) (*models.SubtopicExtractionResult, error) {
	// Build page-bounded context
	contextChunks, tokenCount, err := study.BuildPageBoundedContext(notebookID, startPage, endPage)
	if err != nil {
		return nil, fmt.Errorf("failed to build context: %w", err)
	}

	// Enforce token budget
	const maxTokensForSubtopicExtraction = 10000
	if tokenCount > maxTokensForSubtopicExtraction {
		return nil, fmt.Errorf("content exceeds token budget: %d > %d", tokenCount, maxTokensForSubtopicExtraction)
	}

	contextText := buildContextTextFromChunks(contextChunks)

	// Build the single-pass extraction prompt
	prompt := buildSubtopicExtractionPrompt(notebookID, startPage, endPage, contextText)

	// Call LLM (use heavy LLM for complex extraction task)
	llm := w.studyService.GetHeavyLLMProvider()
	if llm == nil {
		llm = w.studyService.GetFastLLMProvider()
	}
	if llm == nil {
		return nil, fmt.Errorf("no LLM provider available")
	}

	raw, err := llm.GenerateAnswer(prompt)
	if err != nil {
		return nil, fmt.Errorf("LLM call failed: %w", err)
	}

	// Parse the structured response
	result, err := parseSubtopicExtractionResponse(raw)
	if err != nil {
		return nil, fmt.Errorf("failed to parse LLM response: %w", err)
	}

	return result, nil
}

// persistFlashcards creates flashcards for a subtopic with FSRS state.
func (w *Worker) persistFlashcards(parentTopicID string, flashcards []models.GeneratedFlashcard, startPage, endPage int) error {
	if len(flashcards) == 0 {
		return nil
	}

	cards := make([]models.Flashcard, 0, len(flashcards))
	states := make(map[string]models.FlashcardState)

	for _, fc := range flashcards {
		cardID := uuid.NewString()
		card := models.Flashcard{
			ID:      cardID,
			TopicID: parentTopicID,
			Prompt:  fc.Prompt,
			Answer:  fc.Answer,
			DueAt:   0, // Due immediately for new cards
		}
		cards = append(cards, card)

		// Initialize FSRS state for new card
		state := models.FlashcardState{
			Stability:     0,
			Difficulty:    0,
			ElapsedDays:   0,
			ScheduledDays: 0,
			Reps:          0,
			Lapses:        0,
			StateCode:     0, // New state
		}
		states[cardID] = state
	}

	return db.CreateFlashcards(parentTopicID, cards, states)
}

// persistQuizQuestion creates a quiz question for a subtopic.
func (w *Worker) persistQuizQuestion(parentTopicID string, quiz models.GeneratedQuizQuestion, startPage, endPage int) error {
	if quiz.Prompt == "" {
		return nil
	}

	question := models.QuizQuestion{
		ID:              uuid.NewString(),
		TopicID:         parentTopicID,
		Prompt:          quiz.Prompt,
		Options:         quiz.Options,
		CorrectAnswer:   quiz.CorrectAnswer,
		Explanation:     quiz.Explanation,
		SourcePageStart: startPage,
		SourcePageEnd:   endPage,
	}

	return db.AppendQuestionsForTopic(parentTopicID, []models.QuizQuestion{question})
}

// countFlashcards counts total flashcards across all extracted subtopics.
func (w *Worker) countFlashcards(result *models.SubtopicExtractionResult) int {
	count := 0
	for _, sub := range result.Subtopics {
		count += len(sub.Flashcards)
	}
	return count
}

// ExtractionResult summarizes the outcome of a subtopic extraction job.
type ExtractionResult struct {
	SubtopicsCreated   int    `json:"subtopics_created"`
	FlashcardsCreated  int    `json:"flashcards_created"`
	QuestionsCreated   int    `json:"questions_created"`
	PageRangeProcessed string `json:"page_range_processed"`
	PagesPerMission    int    `json:"pages_per_mission"`
}

// buildSubtopicExtractionPrompt creates the single-pass LLM prompt for subtopic extraction.
func buildSubtopicExtractionPrompt(notebookID string, startPage, endPage int, contextText string) string {
	var b strings.Builder
	b.WriteString("You are an AI tutor analyzing a textbook to create micro-missions for a student.\n")
	fmt.Fprintf(&b, "Analyze pages %d-%d of notebook '%s'.\n", startPage, endPage, notebookID)
	b.WriteString("\nYour task:\n")
	b.WriteString("1. Identify 2-4 logical sub-topic transitions within this page range.\n")
	b.WriteString("2. For each sub-topic, provide:\n")
	b.WriteString("   - A concise title (max 8 words)\n")
	b.WriteString("   - Exact start and end page numbers\n")
	b.WriteString("   - A search snippet (the first sentence of the sub-topic, max 15 words)\n")
	b.WriteString("   - 3 FSRS-compliant flashcards (prompt/answer pairs)\n")
	b.WriteString("   - 1 multiple-choice quiz question with 4 options, correct answer, and explanation\n")
	b.WriteString("\nRules:\n")
	b.WriteString("- Subtopics must be ordered by page number.\n")
	b.WriteString("- Page boundaries must be exact and non-overlapping.\n")
	b.WriteString("- Flashcards should test key concepts, not definitions.\n")
	b.WriteString("- Quiz questions should require understanding, not recall.\n")
	b.WriteString("- Return STRICT JSON only in the specified format.\n")
	b.WriteString("\nReturn JSON in this shape:\n")
	b.WriteString(`{"subtopics":[{"title":"...","start_page":N,"end_page":N,"search_snippet":"...","flashcards":[{"prompt":"...","answer":"..."}],"quiz_question":{"prompt":"...","options":["A","B","C","D"],"correct_answer":"...","explanation":"..."}}]}` + "\n")
	b.WriteString("\n=== SOURCE MATERIAL ===\n")
	const maxContextRunes = 25000
	runes := []rune(contextText)
	if len(runes) > maxContextRunes {
		runes = runes[:maxContextRunes]
		contextText = string(runes) + "\n[...content truncated...]"
	}
	b.WriteString(contextText)
	return b.String()
}

// parseSubtopicExtractionResponse parses the LLM response into a structured result.
func parseSubtopicExtractionResponse(raw string) (*models.SubtopicExtractionResult, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, fmt.Errorf("empty LLM response")
	}

	// Extract JSON from the response
	start := strings.Index(raw, "{")
	end := strings.LastIndex(raw, "}")
	if start >= 0 && end > start {
		raw = raw[start : end+1]
	}

	var result models.SubtopicExtractionResult
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		return nil, fmt.Errorf("JSON parse failed: %w", err)
	}

	if len(result.Subtopics) == 0 {
		return nil, fmt.Errorf("no subtopics extracted")
	}

	// Validate each subtopic
	for i, sub := range result.Subtopics {
		if sub.Title == "" {
			return nil, fmt.Errorf("subtopic %d: title is required", i)
		}
		if sub.StartPage < 0 || sub.EndPage < sub.StartPage {
			return nil, fmt.Errorf("subtopic %d: invalid page range", i)
		}
		if len(sub.Flashcards) == 0 {
			return nil, fmt.Errorf("subtopic %d: at least one flashcard required", i)
		}
		if sub.QuizQuestion.Prompt == "" {
			return nil, fmt.Errorf("subtopic %d: quiz question required", i)
		}
	}

	return &result, nil
}

// buildContextTextFromChunks converts chunks to plain text for LLM context.
func buildContextTextFromChunks(chunks []models.ChunkWithContext) string {
	var b strings.Builder
	for _, chunk := range chunks {
		text := strings.TrimSpace(chunk.Text)
		if text == "" {
			continue
		}
		b.WriteString(text)
		b.WriteByte('\n')
	}
	return strings.TrimSpace(b.String())
}
