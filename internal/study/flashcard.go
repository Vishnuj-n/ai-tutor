package study

import (
	"fmt"
	"strings"
	"time"

	"ai-tutor/internal/db"
	"ai-tutor/internal/models"

	"github.com/google/uuid"
)

// GenerateMarathonFlashcards generates FSRS flashcards from the raw text
// of a notebook's page range, injecting context directly into the prompt.
func (s *StudyService) GenerateMarathonFlashcardsWithTopic(topicID, notebookID string, startPage, endPage int) map[string]interface{} {
	notebookID = strings.TrimSpace(notebookID)
	if notebookID == "" {
		return map[string]interface{}{"error": "notebook ID is required"}
	}
	if startPage <= 0 || endPage <= 0 || endPage < startPage {
		return map[string]interface{}{"error": fmt.Sprintf("invalid page range: start=%d end=%d", startPage, endPage)}
	}

	contextText, tokenCount, err := buildPageBoundedContext(notebookID, startPage, endPage)
	if err != nil {
		return map[string]interface{}{"error": err.Error()}
	}

	llm, tier := s.selectLLM(contextText)
	if llm == nil {
		return map[string]interface{}{"error": "no LLM provider available (tier: " + tier + ")"}
	}

	targetCount := ScaledFlashcardCount(tokenCount)
	prompt := buildMarathonFlashcardPrompt(notebookID, startPage, endPage, contextText, tokenCount, targetCount)

	raw, err := llm.GenerateAnswer(prompt)
	if err != nil {
		return map[string]interface{}{"error": "flashcard generation failed: " + err.Error()}
	}
	parsed, err := parseFlashcardLLMResponse(raw)
	if err != nil {
		return map[string]interface{}{"error": "flashcard parsing failed: " + err.Error()}
	}

	now := time.Now().Unix()

	cards := make([]models.Flashcard, 0, len(parsed.Cards))
	states := make(map[string]models.FlashcardState, len(parsed.Cards))
	for _, candidate := range parsed.Cards {
		cardPrompt := strings.TrimSpace(candidate.Prompt)
		answer := strings.TrimSpace(candidate.Answer)
		if cardPrompt == "" || answer == "" {
			continue
		}
		id := uuid.NewString()
		cards = append(cards, models.Flashcard{
			ID:      id,
			TopicID: topicID, // Use the original topicID instead of synthetic one
			Prompt:  cardPrompt,
			Answer:  answer,
			DueAt:   now,
		})
		states[id] = models.FlashcardState{
			Stability:     0.0,
			Difficulty:    0.0,
			ElapsedDays:   0,
			ScheduledDays: 0,
			Reps:          0,
			Lapses:        0,
			StateCode:     0,
		}
	}

	if len(cards) == 0 {
		return map[string]interface{}{"error": "no valid flashcards generated from page range"}
	}

	// Ensure the original topic exists
	err = db.EnsureTopicsBatch([]db.TopicBatchItem{{TopicID: topicID, Title: topicID}})
	if err != nil {
		return map[string]interface{}{"error": fmt.Sprintf("failed to ensure topic %s: %s", topicID, err.Error())}
	}

	cards, _, err = db.GetOrCreateFlashcardsForTopic(topicID, cards, states)
	if err != nil {
		return map[string]interface{}{"error": "failed to persist flashcards: " + err.Error()}
	}

	// Fetch the actual persisted states for the returned cards
	cardIDs := make([]string, len(cards))
	for i, card := range cards {
		cardIDs[i] = card.ID
	}
	persistedStates, err := db.GetFlashcardStatesByIDs(cardIDs)
	if err != nil {
		return map[string]interface{}{"error": "failed to fetch flashcard states: " + err.Error()}
	}

	return map[string]interface{}{
		"notebook_id": notebookID,
		"start_page":  startPage,
		"end_page":    endPage,
		"topic_id":    topicID, // Return the original topicID
		"cards":       cards,
		"states":      persistedStates,
		"card_count":  len(cards),
	}
}

// GenerateMarathonFlashcards generates flashcards for a synthetic topic based on a page range
func (s *StudyService) GenerateMarathonFlashcards(notebookID string, startPage, endPage int) map[string]interface{} {
	notebookID = strings.TrimSpace(notebookID)
	if notebookID == "" {
		return map[string]interface{}{"error": "notebook ID is required"}
	}
	if startPage <= 0 || endPage <= 0 || endPage < startPage {
		return map[string]interface{}{"error": fmt.Sprintf("invalid page range: start=%d end=%d", startPage, endPage)}
	}

	contextText, tokenCount, err := buildPageBoundedContext(notebookID, startPage, endPage)
	if err != nil {
		return map[string]interface{}{"error": err.Error()}
	}

	llm, tier := s.selectLLM(contextText)
	if llm == nil {
		return map[string]interface{}{"error": "no LLM provider available (tier: " + tier + ")"}
	}

	targetCount := ScaledFlashcardCount(tokenCount)
	prompt := buildMarathonFlashcardPrompt(notebookID, startPage, endPage, contextText, tokenCount, targetCount)

	raw, err := llm.GenerateAnswer(prompt)
	if err != nil {
		return map[string]interface{}{"error": "flashcard generation failed: " + err.Error()}
	}
	parsed, err := parseFlashcardLLMResponse(raw)
	if err != nil {
		return map[string]interface{}{"error": "flashcard parsing failed: " + err.Error()}
	}

	syntheticTopicID := fmt.Sprintf("marathon-%s-p%d-%d", notebookID, startPage, endPage)
	now := time.Now().Unix()

	cards := make([]models.Flashcard, 0, len(parsed.Cards))
	states := make(map[string]models.FlashcardState, len(parsed.Cards))
	for _, candidate := range parsed.Cards {
		cardPrompt := strings.TrimSpace(candidate.Prompt)
		answer := strings.TrimSpace(candidate.Answer)
		if cardPrompt == "" || answer == "" {
			continue
		}
		id := uuid.NewString()
		cards = append(cards, models.Flashcard{
			ID:        id,
			TopicID:   syntheticTopicID,
			Prompt:    cardPrompt,
			Answer:    answer,
			DueAt:     now,
			Suspended: false,
		})
		states[id] = models.FlashcardState{}
	}
	if len(cards) == 0 {
		return map[string]interface{}{"error": "no valid flashcards generated from page range"}
	}

	err = db.EnsureTopicsBatch([]db.TopicBatchItem{{TopicID: syntheticTopicID, Title: fmt.Sprintf("Marathon %s p%d-%d", notebookID, startPage, endPage)}})
	if err != nil {
		return map[string]interface{}{"error": fmt.Sprintf("failed to ensure topic %s (notebookID: %s, pages %d-%d): %s", syntheticTopicID, notebookID, startPage, endPage, err.Error())}
	}
	cards, _, err = db.GetOrCreateFlashcardsForTopic(syntheticTopicID, cards, states)
	if err != nil {
		return map[string]interface{}{"error": "failed to persist marathon flashcards: " + err.Error()}
	}

	// Fetch the actual persisted states for the returned cards
	cardIDs := make([]string, len(cards))
	for i, card := range cards {
		cardIDs[i] = card.ID
	}
	persistedStates, err := db.GetFlashcardStatesByIDs(cardIDs)
	if err != nil {
		return map[string]interface{}{"error": "failed to fetch flashcard states: " + err.Error()}
	}

	return map[string]interface{}{
		"notebook_id":       notebookID,
		"start_page":        startPage,
		"end_page":          endPage,
		"topic_id":          syntheticTopicID,
		"cards":             cards,
		"states":            persistedStates,
		"card_count":        len(cards),
		"llm_tier":          tier,
		"generated_at_unix": now,
	}
}

func buildMarathonFlashcardPrompt(notebookID string, startPage, endPage int, contextText string, tokenCount, targetCount int) string {
	var b strings.Builder
	b.WriteString("You are an AI tutor flashcard generator optimized for spaced repetition (FSRS). Return STRICT JSON only. No markdown.\n")
	fmt.Fprintf(&b, "Generate exactly %d flashcards covering pages %d-%d of notebook '%s'.\n",
		targetCount, startPage, endPage, notebookID)
	fmt.Fprintf(&b, "Content token count: %d\n", tokenCount)
	b.WriteString(`JSON format: {"cards":[{"prompt":string,"answer":string}]}` + "\n")
	b.WriteString("\n=== ATOMIC KNOWLEDGE (CRITICAL) ===\n")
	b.WriteString("Each card must test exactly ONE concept. Multi-part answers are forbidden.\n")
	b.WriteString("\n=== PROMPT QUALITY ===\n")
	b.WriteString("- AVOID yes/no questions.\n")
	b.WriteString("- PREFER 'why', 'how', 'what is', 'explain' questions.\n")
	b.WriteString("\n=== ANSWER QUALITY ===\n")
	b.WriteString("- Answers must be short (1-2 sentences max, grounded in source).\n")
	b.WriteString("\n=== SOURCE MATERIAL ===\n")
	const maxContextRunes = 24000
	runes := []rune(contextText)
	if len(runes) > maxContextRunes {
		runes = runes[:maxContextRunes]
		contextText = string(runes) + "\n[...content truncated to fit context window...]"
	}
	b.WriteString(contextText)
	return b.String()
}
