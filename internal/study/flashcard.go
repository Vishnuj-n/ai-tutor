package study

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"ai-tutor/internal/db"
	"ai-tutor/internal/embeddings"
	"ai-tutor/internal/models"
	"ai-tutor/internal/utils"

	"github.com/google/uuid"
)

// GenerateManualFlashcards generates flashcards for a synthetic topic based on a page range (manual sandbox)
func (s *StudyService) GenerateManualFlashcards(notebookID string, startPage, endPage int) map[string]interface{} {
	cards, tier, err := s.generateFlashcardsCore(notebookID, startPage, endPage, nil)
	if err != nil {
		return map[string]interface{}{"error": err.Error()}
	}

	syntheticTopicID := fmt.Sprintf("marathon-%s-p%d-%d", notebookID, startPage, endPage)
	for i := range cards {
		cards[i].TopicID = syntheticTopicID
	}

	err = s.repo.SaveManualFlashcardsBatch(notebookID, cards)
	if err != nil {
		utils.Warnf("[FLASHCARD_PIPELINE] manual_flashcard_persistence result=error notebookID=%s err=%v", notebookID, err)
		return map[string]interface{}{"error": "failed to persist manual flashcards: " + err.Error()}
	}
	utils.Warnf("[FLASHCARD_PIPELINE] manual_flashcard_persistence result=ok notebookID=%s cardCount=%d", notebookID, len(cards))

	now := time.Now().Unix()
	return map[string]interface{}{
		"notebook_id":       notebookID,
		"existing":          false,
		"start_page":        startPage,
		"end_page":          endPage,
		"topic_id":          syntheticTopicID,
		"cards":             cards,
		"states":            map[string]models.FlashcardState{},
		"card_count":        len(cards),
		"llm_tier":          tier,
		"generated_at_unix": now,
	}
}

// GenerateFSRSCardsForTopic generates and persists FSRS flashcards in the core fsrs_cards table for a topic.
func (s *StudyService) GenerateFSRSCardsForTopic(topicID, notebookID string, startPage, endPage int) ([]models.Flashcard, map[string]models.FlashcardState, bool, string, error) {
	topicID = strings.TrimSpace(topicID)
	notebookID = strings.TrimSpace(notebookID)
	if topicID == "" || notebookID == "" {
		return nil, nil, false, "", fmt.Errorf("topic ID and notebook ID are required")
	}

	// ponytail: lookup latest quiz attempt and extract failed questions
	var failedQuestions []models.FailedQuestionDetail
	if payloadJSON, answersJSON, err := s.repo.GetLatestQuizAttemptDetailsByTopic(topicID); err == nil && payloadJSON != "" && answersJSON != "" {
		var payload models.QuizTaskPayload
		var answers []models.QuizAnswer
		if json.Unmarshal([]byte(payloadJSON), &payload) == nil && json.Unmarshal([]byte(answersJSON), &answers) == nil {
			selectedByQuestionID := make(map[string]string)
			for _, ans := range answers {
				selectedByQuestionID[ans.QuestionID] = strings.TrimSpace(ans.Selected)
			}
			for _, q := range payload.Questions {
				userAns := selectedByQuestionID[q.ID]
				if !strings.EqualFold(strings.TrimSpace(q.CorrectAnswer), userAns) {
					failedQuestions = append(failedQuestions, models.FailedQuestionDetail{
						Prompt:        q.Prompt,
						Options:       q.Options,
						CorrectAnswer: q.CorrectAnswer,
						UserAnswer:    userAns,
					})
				}
			}
		}
	}

	cards, tier, err := s.generateFlashcardsCore(notebookID, startPage, endPage, failedQuestions)
	if err != nil {
		return nil, nil, false, "", err
	}

	for i := range cards {
		cards[i].TopicID = topicID
	}

	topicTitle := topicID // Fallback title
	err = s.repo.EnsureTopicsBatch([]db.TopicBatchItem{{TopicID: topicID, Title: topicTitle}})
	if err != nil {
		return nil, nil, false, "", fmt.Errorf("failed to ensure topic: %w", err)
	}

	err = s.repo.EnsureNotebookTopic(notebookID, topicID)
	if err != nil {
		return nil, nil, false, "", fmt.Errorf("failed to link topic to notebook: %w", err)
	}

	// Start cards in Review state (bypass learning phase) with day-based offsets
	initialState := models.FlashcardState{
		StateCode: 2, // 2 = Review state in models.go
	}
	now := time.Now().Unix()
	dueAt := now + 24*60*60 // Default fallback: tomorrow

	score, passedAttempt, err := s.repo.GetLatestQuizAttemptScoreByTopic(topicID)
	if err == nil && passedAttempt {
		switch score {
		case 100:
			dueAt = now + 3*24*60*60 // Ace: 3 days
			utils.Warnf("[FSRS_CALIBRATION] Ace detected (score=100) for topicID=%s. Scheduled in 3 days.", topicID)
		default:
			dueAt = now + 24*60*60 // Pass: tomorrow (1 day)
			utils.Warnf("[FSRS_CALIBRATION] Pass detected (score=%d) for topicID=%s. Scheduled in 1 day.", score, topicID)
		}
	} else {
		utils.Warnf("[FSRS_CALIBRATION] Using default tomorrow offset for topicID=%s (no passed quiz attempt found, err=%v)", topicID, err)
	}

	states := make(map[string]models.FlashcardState, len(cards))
	for i := range cards {
		cards[i].DueAt = dueAt
		states[cards[i].ID] = initialState
	}

	cards, existing, err := s.repo.GetOrCreateFlashcardsForTopic(topicID, cards, states)
	if err != nil {
		return nil, nil, false, "", fmt.Errorf("failed to persist FSRS flashcards: %w", err)
	}

	cardIDs := make([]string, len(cards))
	for i, card := range cards {
		cardIDs[i] = card.ID
	}
	persistedStates, err := s.repo.GetFlashcardStatesByIDs(cardIDs)
	if err != nil {
		return nil, nil, false, "", fmt.Errorf("failed to fetch flashcard states: %w", err)
	}

	return cards, persistedStates, existing, tier, nil
}

func (s *StudyService) generateFlashcardsCore(notebookID string, startPage, endPage int, failedQuestions []models.FailedQuestionDetail) ([]models.Flashcard, string, error) {
	generationSource := "flashcard_pipeline_core"
	notebookID = strings.TrimSpace(notebookID)
	if notebookID == "" {
		return nil, "", fmt.Errorf("notebook ID is required")
	}
	if startPage <= 0 || endPage <= 0 || endPage < startPage {
		return nil, "", fmt.Errorf("invalid page range: start=%d end=%d", startPage, endPage)
	}

	notebookTitle := notebookID
	nb, err := s.repo.GetNotebookByID(notebookID)
	if err == nil && nb != nil && nb.Title != "" {
		notebookTitle = nb.Title
	}

	contextChunks, tokenCount, err := s.buildPageBoundedContext(notebookID, startPage, endPage)
	if err != nil {
		return nil, "", err
	}
	if len(contextChunks) == 0 {
		return nil, "", fmt.Errorf("no content found in page range %d-%d", startPage, endPage)
	}
	utils.Warnf("[FLASHCARD_PIPELINE] flashcard_auto_generation_batch generation_source=%s chunk_count=%d token_estimate=%d page_range=%d-%d", generationSource, len(contextChunks), tokenCount, startPage, endPage)
	contextText := buildContextTextFromChunks(contextChunks)

	llm, tier := s.selectLLM(contextText)
	if llm == nil {
		return nil, "", fmt.Errorf("no LLM provider available (tier: %s)", tier)
	}

	// Get model-specific token limits
	modelName := providerModelName(llm)
	limits := llm.GetLimits()
	maxInputTokens := limits.MaxInputTokens
	maxOutputTokens := limits.MaxOutputTokens
	utils.Warnf("[FLASHCARD_PIPELINE] model_limits model=%s max_input=%d max_output=%d", modelName, maxInputTokens, maxOutputTokens)

	// Default to 5 base flashcards. Additional cards will be added for failed questions in buildMarathonFlashcardPromptWithBudget.
	targetCount := 5

	// Build prompt with token budgeting
	prompt, promptTokenCount, includedChunkIDs := buildMarathonFlashcardPromptWithBudget(notebookTitle, startPage, endPage, contextChunks, targetCount, maxInputTokens, failedQuestions)
	if len(includedChunkIDs) == 0 {
		return nil, "", fmt.Errorf("no chunks fit within prompt token budget for page range %d-%d", startPage, endPage)
	}

	// Log token estimates before generation
	utils.Warnf("[FLASHCARD_PIPELINE] token_budget_estimate prompt_tokens=%d max_input=%d budget_used_pct=%.2f", promptTokenCount, maxInputTokens, float64(promptTokenCount)/float64(maxInputTokens)*100)

	if promptTokenCount > maxInputTokens {
		return nil, "", fmt.Errorf("prompt exceeds model context limit: %d > %d", promptTokenCount, maxInputTokens)
	}

	raw, err := llm.GenerateAnswer(prompt)
	if err != nil {
		return nil, "", fmt.Errorf("flashcard generation failed: %w", err)
	}

	// Validate output size before parsing
	outputTokenEstimate := len(strings.Fields(raw))
	utils.Warnf("[FLASHCARD_PIPELINE] output_validation output_tokens_est=%d max_output=%d", outputTokenEstimate, maxOutputTokens)

	parsed, err := parseFlashcardLLMResponse(raw)
	if err != nil {
		return nil, "", fmt.Errorf("flashcard parsing failed: %w", err)
	}

	// Apply "Hard Slice" (The Array Truncation Trick) to prevent flashcard avalanche.
	maxCardsAllowed := 5 + len(failedQuestions)
	if len(parsed.Cards) > maxCardsAllowed {
		originalCount := len(parsed.Cards)
		parsed.Cards = parsed.Cards[:maxCardsAllowed]
		utils.Warnf("[FLASHCARD_PIPELINE] hard_slice applied original_count=%d capped_to=%d", originalCount, maxCardsAllowed)
	}

	now := time.Now().Unix()
	dueAt := now + 24*60*60 // Schedule new cards for delayed reinforcement, not same-day review.

	cards := make([]models.Flashcard, 0, len(parsed.Cards))
	allowedChunkIDs := make(map[string]struct{}, len(includedChunkIDs))
	for _, chunkID := range includedChunkIDs {
		allowedChunkIDs[chunkID] = struct{}{}
	}
	defaultChunkID := ""
	if len(includedChunkIDs) > 0 {
		defaultChunkID = includedChunkIDs[0]
	}

	for _, candidate := range parsed.Cards {
		sourceChunkID := strings.TrimSpace(candidate.SourceChunkID)
		cardPrompt := strings.TrimSpace(candidate.Prompt)
		answer := strings.TrimSpace(candidate.Answer)
		if cardPrompt == "" || answer == "" {
			utils.Warnf("Skipping flashcard: missing required prompt or answer")
			continue
		}
		if _, ok := allowedChunkIDs[sourceChunkID]; !ok {
			sourceChunkID = defaultChunkID
		}
		id := uuid.NewString()
		cards = append(cards, models.Flashcard{
			ID:            id,
			SourceChunkID: sourceChunkID,
			Prompt:        cardPrompt,
			Answer:        answer,
			DueAt:         dueAt,
			Suspended:     false,
		})
	}
	if len(cards) == 0 {
		return nil, "", fmt.Errorf("no valid flashcards generated from page range")
	}

	return cards, tier, nil
}

func buildMarathonFlashcardPromptWithBudget(notebookTitle string, startPage, endPage int, contextChunks []models.ChunkWithContext, targetCount, maxInputTokens int, failedQuestions []models.FailedQuestionDetail) (string, int, []string) {
	// Base prompt overhead (instructions, format, etc.)
	const baseOverheadTokens = 300
	const safetyMarginTokens = 500 // Reserve for output tokens and safety margin

	// Calculate available budget for chunks
	availableBudget := maxInputTokens - baseOverheadTokens - safetyMarginTokens
	if availableBudget < 1000 {
		availableBudget = 1000 // Minimum budget for meaningful content
	}

	// ponytail: increment target count by failed questions count to generate extra targeted corrective cards
	if len(failedQuestions) > 0 {
		targetCount += len(failedQuestions)
	}

	var b strings.Builder
	b.WriteString("You are an expert academic tutor and flashcard generator creating study materials for spaced repetition (FSRS).\n")
	b.WriteString("CRITICAL: Return ONLY valid JSON. No markdown. No code blocks. No explanations.\n")
	b.WriteString("Output must start with { and end with }. No prefix or suffix text.\n")
	fmt.Fprintf(&b, "Notebook: \"%s\"\n", notebookTitle)

	if len(failedQuestions) > 0 {
		b.WriteString("\n=== TARGETED REVIEW: MISCONCEPTIONS ===\n")
		b.WriteString("The user recently took a quiz and got these questions wrong. You MUST generate targeted corrective flashcards (at least 1 per misconception) specifically addressing the concepts, definitions, or facts tested in these incorrect questions to correct their understanding:\n")
		for _, q := range failedQuestions {
			fmt.Fprintf(&b, "- Quiz Question: %s | Correct Answer: %s | User's wrong selection: %s\n", q.Prompt, q.CorrectAnswer, q.UserAnswer)
		}
		b.WriteString("\n")
	}

	b.WriteString("\n=== JSON FORMAT (FOLLOW EXACTLY) ===\n")
	b.WriteString(`{"cards":[{"prompt":"What is X?","answer":"X is..."},{"prompt":"How does Y work?","answer":"Y works by..."}]}` + "\n")
	b.WriteString("\n=== GROUNDING & CONTENT RULES ===\n")
	b.WriteString("- Use ONLY the information contained in the provided source material.\n")
	b.WriteString("- Do not use outside knowledge or invent information.\n")
	b.WriteString("- Generate cards from important, learnable concepts rather than minor details.\n")
	b.WriteString("- Concepts may be synthesized across multiple source chunks.\n")
	b.WriteString("- The cards belong to the requested page range; exact chunk attribution is not required.\n")
	b.WriteString("\n=== ADAPTIVE CONTENT RULES ===\n")
	b.WriteString("Classify the source material before generating cards:\n\n")
	b.WriteString("- FACTUAL: Prioritize important facts, definitions, formulas, terminology, and concrete relationships.\n")
	b.WriteString("- CONCEPTUAL: Prioritize core ideas, reasoning, principles, comparisons, and cause-effect relationships.\n")
	b.WriteString("- TECHNICAL: Prioritize definitions, mechanisms, algorithms, workflows, constraints, trade-offs, and practical application.\n")
	b.WriteString("\n=== FLASHCARD RULES ===\n")
	b.WriteString("- Each card must test exactly ONE concept.\n")
	b.WriteString("- Prefer concepts that are useful for long-term recall.\n")
	b.WriteString("- Avoid trivial details, incidental examples, and book metadata.\n")
	b.WriteString("- Avoid duplicate cards testing the same concept.\n")
	b.WriteString("- Prefer \"why\", \"how\", \"what is\", and \"explain\" questions.\n")
	b.WriteString("- Questions should require recall, not simple recognition.\n")
	b.WriteString("\n=== ANSWER RULES ===\n")
	b.WriteString("- Answers must be concise: 1–2 sentences.\n")
	b.WriteString("- Answer directly and completely.\n")
	b.WriteString("- Preserve the terminology and meaning of the source.\n")
	b.WriteString("- Do not add information that is not supported by the source.\n")
	b.WriteString("\n")
	fmt.Fprintf(&b, "Generate up to %d flashcards from the provided source material (pages %d-%d).\n", targetCount, startPage, endPage)
	b.WriteString("Generate fewer if there are not enough distinct important concepts.\n")
	b.WriteString("\n=== SOURCE CHUNKS ===\n")

	// Trim chunks based on token budget
	currentTokens := baseOverheadTokens
	var includedChunks []models.ChunkWithContext
	var includedChunkIDs []string
	truncatedCount := 0

	for _, chunk := range contextChunks {
		text := strings.TrimSpace(chunk.Text)
		if text == "" {
			continue
		}

		// Estimate tokens for this chunk with formatting
		chunkLine := fmt.Sprintf("- chunk_id: %s | page_num: %d | text: %s\n", chunk.ChunkID, chunk.PageNum, text)
		chunkTokens, err := embeddings.CountTokens(chunkLine)
		if err != nil {
			// Fallback to word count if tokenization fails
			chunkTokens = len(strings.Fields(chunkLine))
		}

		// Check if adding this chunk would exceed budget
		if currentTokens+chunkTokens > availableBudget {
			truncatedCount++
			continue
		}

		includedChunks = append(includedChunks, chunk)
		includedChunkIDs = append(includedChunkIDs, chunk.ChunkID)
		currentTokens += chunkTokens
	}

	// Add chunks to prompt
	for _, chunk := range includedChunks {
		text := strings.TrimSpace(chunk.Text)
		fmt.Fprintf(&b, "- chunk_id: %s | page_num: %d | text: %s\n", chunk.ChunkID, chunk.PageNum, text)
	}

	if truncatedCount > 0 {
		fmt.Fprintf(&b, "[...%d additional chunks truncated to stay within token budget...]\n", truncatedCount)
	}

	utils.Warnf("[FLASHCARD_PIPELINE] chunk_trimming total_chunks=%d included=%d truncated=%d budget_used=%d available=%d",
		len(contextChunks), len(includedChunks), truncatedCount, currentTokens, availableBudget)

	return b.String(), currentTokens, includedChunkIDs
}
