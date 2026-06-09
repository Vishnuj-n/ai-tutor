package study

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	"ai-tutor/internal/db"
	"ai-tutor/internal/embeddings"
	"ai-tutor/internal/models"
	"ai-tutor/internal/retrieval"
	"ai-tutor/internal/utils"

	"github.com/google/uuid"
)

// wordThresholdForHeavyLLM is the Marathon Mode routing threshold.
// Content at or above this word count is routed to HEAVY_LLM to prevent
// "Lost in the Middle" hallucinations.
const wordThresholdForHeavyLLM = 4000

// LLMProvider is the minimal interface both LLM tiers satisfy.
type LLMProvider interface {
	GenerateAnswer(prompt string) (string, error)
	ModelName() string
}

// Config wires all dependencies into StudyService via constructor injection.
type Config struct {
	FastLLMProvider  LLMProvider
	HeavyLLMProvider LLMProvider
	RetrievalEngine  *retrieval.Engine
}

// StudyService owns all study-mode generation and scoring logic.
// quiz.go, flashcard.go, examiner.go, and socratic.go add methods to this type.
type StudyService struct {
	fastLLMProvider  LLMProvider
	heavyLLMProvider LLMProvider
	retrievalEngine  *retrieval.Engine
}

// NewStudyService constructs a StudyService from injected dependencies.
func NewStudyService(cfg Config) *StudyService {
	return &StudyService{
		fastLLMProvider:  cfg.FastLLMProvider,
		heavyLLMProvider: cfg.HeavyLLMProvider,
		retrievalEngine:  cfg.RetrievalEngine,
	}
}

// selectLLM picks FAST or HEAVY based on the word count of the context.
func (s *StudyService) selectLLM(contextText string) (LLMProvider, string) {
	wordCount := len(strings.Fields(contextText))
	if wordCount >= wordThresholdForHeavyLLM {
		if s.heavyLLMProvider != nil {
			return s.heavyLLMProvider, "heavy"
		}
	}
	return s.fastLLMProvider, "fast"
}

// ScoreShortAnswer scores one persisted short-answer prompt and updates FSRS.
func (s *StudyService) ScoreShortAnswer(questionID, userAnswer string) map[string]interface{} {
	questionID = strings.TrimSpace(questionID)
	userAnswer = strings.TrimSpace(userAnswer)
	if questionID == "" || userAnswer == "" {
		return map[string]interface{}{"error": "question ID and user answer are required"}
	}
	if s.fastLLMProvider == nil {
		return map[string]interface{}{"error": "FAST_LLM provider not initialized"}
	}

	question, err := db.GetWrittenQuestionByID(questionID)
	if err != nil {
		return map[string]interface{}{"error": "failed to fetch written question: " + err.Error()}
	}
	if question == nil {
		return map[string]interface{}{"error": "written question not found"}
	}

	scorePrompt := fmt.Sprintf(`You are grading a student's short answer.
Return STRICT JSON only in this shape: {"score":number,"feedback":"..."}.

Scoring rubric:
- Score must be an integer from 1 to 10.
- 1-3 = major misunderstandings or mostly incorrect.
- 4-5 = partially correct with clear gaps.
- 6-8 = mostly correct with some omissions.
- 9-10 = strong, precise, and concise.
- Feedback must be concise (max 2 sentences), specific, and actionable.

Question: %s
Student answer: %s`, question.Prompt, userAnswer)

	raw, err := s.fastLLMProvider.GenerateAnswer(scorePrompt)
	if err != nil {
		return map[string]interface{}{"error": "short-answer scoring failed: " + err.Error()}
	}
	parsed, err := parseShortAnswerScoreLLMResponse(raw)
	if err != nil {
		return map[string]interface{}{"error": "short-answer scoring parse failed: " + err.Error()}
	}
	score := parsed.Score
	if score < 1 {
		score = 1
	}
	if score > 10 {
		score = 10
	}

	tx, err := db.GetConnection().Begin()
	if err != nil {
		return map[string]interface{}{"error": "failed to begin transaction: " + err.Error()}
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	writtenAnswer := models.WrittenAnswer{
		QuestionID:    question.ID,
		Score:         score,
		Feedback:      strings.TrimSpace(parsed.Feedback),
		UserAnswer:    userAnswer,
		SourceHeading: question.SourceHeading,
	}
	if err := db.SaveWrittenAnswerTx(tx, writtenAnswer); err != nil {
		return map[string]interface{}{"error": "failed to save written answer: " + err.Error()}
	}
	if err := tx.Commit(); err != nil {
		return map[string]interface{}{"error": "failed to commit transaction: " + err.Error()}
	}
	committed = true

	return map[string]interface{}{
		"question_id":       question.ID,
		"prompt":            question.Prompt,
		"score":             score,
		"feedback":          strings.TrimSpace(parsed.Feedback),
		"source_page_start": question.SourcePageStart,
		"source_page_end":   question.SourcePageEnd,
		"source_heading":    question.SourceHeading,
	}
}

// ---------- LLM response types (shared across sub-files) ----------

type quizLLMQuestion struct {
	SourceChunkID   string   `json:"source_chunk_id"`
	Prompt          string   `json:"prompt"`
	Options         []string `json:"options"`
	CorrectAnswer   string   `json:"correct_answer"`
	Explanation     string   `json:"explanation"`
	Hint            string   `json:"hint"`
	SourceHeading   string   `json:"source_heading"`
	SourceSnippet   string   `json:"source_snippet"`
	SourcePageStart int      `json:"source_page_start"`
	SourcePageEnd   int      `json:"source_page_end"`
}

type quizLLMResponse struct {
	Questions []quizLLMQuestion `json:"questions"`
}

type flashcardLLMCard struct {
	SourceChunkID string `json:"source_chunk_id"`
	Prompt        string `json:"prompt"`
	Answer        string `json:"answer"`
}

type flashcardLLMResponse struct {
	Cards []flashcardLLMCard `json:"cards"`
}

type shortAnswerPromptLLMResponse struct {
	Prompt string `json:"prompt"`
}

type shortAnswerScoreLLMResponse struct {
	Score    int    `json:"score"`
	Feedback string `json:"feedback"`
}

// ---------- Shared helpers ----------

func providerModelName(provider LLMProvider) string {
	typed, ok := provider.(interface{ ModelName() string })
	if !ok {
		return "unknown-model"
	}
	modelName := strings.TrimSpace(typed.ModelName())
	if modelName == "" {
		return "unknown-model"
	}
	return modelName
}

func parseQuizLLMResponse(raw string) (*quizLLMResponse, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, fmt.Errorf("empty LLM response")
	}
	start := strings.Index(raw, "{")
	end := strings.LastIndex(raw, "}")
	if start >= 0 && end > start {
		raw = raw[start : end+1]
	}
	var out quizLLMResponse
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil, err
	}
	if len(out.Questions) == 0 {
		return nil, fmt.Errorf("no questions in LLM response")
	}
	return &out, nil
}

func parseFlashcardLLMResponse(raw string) (*flashcardLLMResponse, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, fmt.Errorf("empty LLM response")
	}

	// Strip markdown code fences if present
	if strings.HasPrefix(raw, "```json") {
		raw = strings.TrimPrefix(raw, "```json")
		raw = strings.TrimSpace(raw)
		if strings.HasSuffix(raw, "```") {
			raw = strings.TrimSuffix(raw, "```")
			raw = strings.TrimSpace(raw)
		}
	} else if strings.HasPrefix(raw, "```") {
		raw = strings.TrimPrefix(raw, "```")
		raw = strings.TrimSpace(raw)
		if strings.HasSuffix(raw, "```") {
			raw = strings.TrimSuffix(raw, "```")
			raw = strings.TrimSpace(raw)
		}
	}

	// Extract JSON between first { and last }
	start := strings.Index(raw, "{")
	end := strings.LastIndex(raw, "}")
	if start >= 0 && end > start {
		raw = raw[start : end+1]
	}

	// Truncation recovery: fix common JSON malformations
	// Case 1: Extra } after array element (e.g., {"cards":[{...},{...}}})
	if strings.Contains(raw, "}}") && strings.Count(raw, "}") > strings.Count(raw, "{") {
		// Single-pass brace-balance scanner: count and remove only necessary closing braces
		openBraces := strings.Count(raw, "{")
		closeBraces := strings.Count(raw, "}")
		extraBraces := closeBraces - openBraces
		if extraBraces > 0 {
			// Remove trailing extra braces
			raw = raw[:len(raw)-extraBraces]
			utils.Warnf("[FLASHCARD_PARSE] truncation_recovery removed_extra_closing_braces count=%d", extraBraces)
		}
	}

	// Case 2: Missing closing braces
	openBraces := strings.Count(raw, "{")
	closeBraces := strings.Count(raw, "}")
	if closeBraces < openBraces {
		missing := openBraces - closeBraces
		for i := 0; i < missing; i++ {
			raw += "}"
		}
		utils.Warnf("[FLASHCARD_PARSE] truncation_recovery added=%d_closing_braces", missing)
	}

	// Case 3: Missing closing bracket for cards array
	if strings.Contains(raw, `"cards":[`) && !strings.Contains(raw, "]") {
		raw += "]"
		utils.Warnf("[FLASHCARD_PARSE] truncation_recovery added_closing_bracket")
	}

	var out flashcardLLMResponse
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		// Log truncated raw prefix to avoid huge/secret-bearing logs
		rawPrefix := raw
		if len(rawPrefix) > 512 {
			rawPrefix = rawPrefix[:512]
		}
		utils.Warnf("[FLASHCARD_PARSE] json_unmarshal_failed error=%v raw_length=%d raw_prefix=%s", err, len(raw), rawPrefix)

		// Graceful degradation: try to salvage partial cards from malformed JSON
		if salvaged := salvagePartialCards(raw); salvaged != nil && len(salvaged.Cards) > 0 {
			// Validate salvaged cards using the same validation as normal cards
			validCards := make([]flashcardLLMCard, 0, len(salvaged.Cards))
			for i, card := range salvaged.Cards {
				if strings.TrimSpace(card.SourceChunkID) == "" {
					utils.Warnf("[FLASHCARD_PARSE] salvage_validation_failed card_index=%d missing_field=source_chunk_id skipping", i)
					continue
				}
				if strings.TrimSpace(card.Prompt) == "" {
					utils.Warnf("[FLASHCARD_PARSE] salvage_validation_failed card_index=%d missing_field=prompt skipping", i)
					continue
				}
				if strings.TrimSpace(card.Answer) == "" {
					utils.Warnf("[FLASHCARD_PARSE] salvage_validation_failed card_index=%d missing_field=answer skipping", i)
					continue
				}
				validCards = append(validCards, card)
			}
			if len(validCards) > 0 {
				utils.Warnf("[FLASHCARD_PARSE] graceful_degradation salvaged=%d_cards valid=%d from_malformed_json", len(salvaged.Cards), len(validCards))
				return &flashcardLLMResponse{Cards: validCards}, nil
			}
		}

		return nil, err
	}
	if len(out.Cards) == 0 {
		utils.Warnf("[FLASHCARD_PARSE] no_cards_in_response raw_response=%s", raw)
		return nil, fmt.Errorf("no cards in LLM response")
	}

	// Schema validation: ensure each card has required fields
	validCards := make([]flashcardLLMCard, 0, len(out.Cards))
	for i, card := range out.Cards {
		if strings.TrimSpace(card.SourceChunkID) == "" {
			utils.Warnf("[FLASHCARD_PARSE] schema_validation_failed card_index=%d missing_field=source_chunk_id skipping", i)
			continue
		}
		if strings.TrimSpace(card.Prompt) == "" {
			utils.Warnf("[FLASHCARD_PARSE] schema_validation_failed card_index=%d missing_field=prompt skipping", i)
			continue
		}
		if strings.TrimSpace(card.Answer) == "" {
			utils.Warnf("[FLASHCARD_PARSE] schema_validation_failed card_index=%d missing_field=answer skipping", i)
			continue
		}
		validCards = append(validCards, card)
	}

	if len(validCards) == 0 {
		utils.Warnf("[FLASHCARD_PARSE] no_valid_cards_after_validation original_count=%d", len(out.Cards))
		return nil, fmt.Errorf("no valid cards after schema validation")
	}

	if len(validCards) < len(out.Cards) {
		utils.Warnf("[FLASHCARD_PARSE] graceful_degradation original=%d valid=%d skipped=%d", len(out.Cards), len(validCards), len(out.Cards)-len(validCards))
		out.Cards = validCards
	}

	return &out, nil
}

// salvagePartialCards attempts to extract valid cards from malformed JSON
// by looking for card-like patterns in the raw response.
func salvagePartialCards(raw string) *flashcardLLMResponse {
	utils.Warnf("[FLASHCARD_PARSE] salvage_attempt initiated raw_length=%d", len(raw))

	// Try to find card objects by splitting on '{' boundaries
	cards := make([]flashcardLLMCard, 0)

	// Find all '{' positions to identify potential card objects
	for i := 0; i < len(raw); i++ {
		if raw[i] == '{' {
			// Find matching closing brace or next card start
			braceCount := 1
			j := i + 1
			for j < len(raw) && braceCount > 0 {
				switch raw[j] {
				case '{':
					braceCount++
				case '}':
					braceCount--
				}
				j++
			}

			// Extract potential card object
			if braceCount == 0 && j > i+1 {
				cardStr := raw[i+1 : j-1] // Content between { and }

				// Try to extract fields from this object
				sourceChunkID := extractJSONField("{"+cardStr+"}", "source_chunk_id")
				prompt := extractJSONField("{"+cardStr+"}", "prompt")
				answer := extractJSONField("{"+cardStr+"}", "answer")

				// Only add if we have at least some non-empty required fields
				if sourceChunkID != "" && prompt != "" && answer != "" {
					cards = append(cards, flashcardLLMCard{
						SourceChunkID: sourceChunkID,
						Prompt:        prompt,
						Answer:        answer,
					})
				}
			}
		}
	}

	if len(cards) == 0 {
		utils.Warnf("[FLASHCARD_PARSE] salvage_failed no_cards_extracted")
		return nil
	}

	utils.Warnf("[FLASHCARD_PARSE] salvage_success extracted=%d_cards", len(cards))
	return &flashcardLLMResponse{Cards: cards}
}

// extractJSONField attempts to extract a field value from a malformed JSON string,
// properly handling escaped quotes (e.g., \")
func extractJSONField(raw, fieldName string) string {
	// Look for pattern: "field":"value" or "field":"value"
	pattern := `"` + fieldName + `":"`
	idx := strings.Index(raw, pattern)
	if idx == -1 {
		return ""
	}

	// Skip past the pattern and opening quote
	start := idx + len(pattern)
	if start >= len(raw) {
		return ""
	}

	// Find the closing quote, accounting for escaped quotes
	var end int
	for i := start; i < len(raw); i++ {
		if raw[i] == '\\' && i+1 < len(raw) {
			// Skip escaped character
			i++
			continue
		}
		if raw[i] == '"' {
			end = i - start
			break
		}
	}
	if end == 0 && (len(raw) <= start || raw[start] == '"') {
		return "" // No value found or empty string
	}

	// Handle case where we didn't find closing quote
	if end == 0 {
		end = len(raw) - start
	}

	return strings.TrimSpace(raw[start : start+end])
}

func parseShortAnswerPromptLLMResponse(raw string) (*shortAnswerPromptLLMResponse, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, fmt.Errorf("empty LLM response")
	}
	jsonSlice := raw
	start := strings.Index(raw, "{")
	end := strings.LastIndex(raw, "}")
	if start >= 0 && end > start {
		jsonSlice = raw[start : end+1]
	}
	var out shortAnswerPromptLLMResponse
	if err := json.Unmarshal([]byte(jsonSlice), &out); err != nil {
		fallback := strings.TrimSpace(raw)
		if fallback == "" {
			return nil, err
		}
		out.Prompt = fallback
	}
	if strings.TrimSpace(out.Prompt) == "" {
		return nil, fmt.Errorf("no prompt in LLM response")
	}
	return &out, nil
}

func parseShortAnswerScoreLLMResponse(raw string) (*shortAnswerScoreLLMResponse, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, fmt.Errorf("empty LLM response")
	}
	start := strings.Index(raw, "{")
	end := strings.LastIndex(raw, "}")
	if start >= 0 && end > start {
		raw = raw[start : end+1]
	}
	var out shortAnswerScoreLLMResponse
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil, err
	}
	if strings.TrimSpace(out.Feedback) == "" {
		out.Feedback = "Review key topic concepts and tighten your explanation."
	}
	return &out, nil
}

func ResolveCorrectOption(correctAnswer string, options []string) (string, bool) {
	canonical := strings.TrimSpace(strings.ToLower(correctAnswer))
	for _, opt := range options {
		if strings.TrimSpace(strings.ToLower(opt)) == canonical {
			return strings.TrimSpace(opt), true
		}
	}
	return "", false
}

// ---------- Env-configurable density thresholds ----------

const (
	DefaultMaxInputTokens  = 30000
	DefaultMaxOutputTokens = 3000
)

var (
	QuizTokenThresholdLow    = getEnvInt("QUIZ_TOKEN_THRESHOLD_LOW", 600)
	QuizTokenThresholdMedium = getEnvInt("QUIZ_TOKEN_THRESHOLD_MEDIUM", 1500)
	QuizTokenThresholdHigh   = getEnvInt("QUIZ_TOKEN_THRESHOLD_HIGH", 3000)
	QuizQuestionCountLow     = getEnvInt("QUIZ_QUESTION_COUNT_LOW", 3)
	QuizQuestionCountMedium  = getEnvInt("QUIZ_QUESTION_COUNT_MEDIUM", 5)
	QuizQuestionCountHigh    = getEnvInt("QUIZ_QUESTION_COUNT_HIGH", 7)
	QuizQuestionCountMax     = getEnvInt("QUIZ_QUESTION_COUNT_MAX", 10)
	FlashcardCountLow        = getEnvInt("FLASHCARD_COUNT_LOW", 5)
	FlashcardCountMedium     = getEnvInt("FLASHCARD_COUNT_MEDIUM", 8)
	FlashcardCountHigh       = getEnvInt("FLASHCARD_COUNT_HIGH", 12)
	FlashcardCountMax        = getEnvInt("FLASHCARD_COUNT_MAX", 16)
)

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

func scaledQuizQuestionCount(wordCount int) int {
	switch {
	case wordCount <= QuizTokenThresholdLow:
		return QuizQuestionCountLow
	case wordCount <= QuizTokenThresholdMedium:
		return QuizQuestionCountMedium
	case wordCount <= QuizTokenThresholdHigh:
		return QuizQuestionCountHigh
	default:
		return QuizQuestionCountMax
	}
}

func ScaledFlashcardCount(wordCount int) int {
	switch {
	case wordCount <= QuizTokenThresholdLow:
		return FlashcardCountLow
	case wordCount <= QuizTokenThresholdMedium:
		return FlashcardCountMedium
	case wordCount <= QuizTokenThresholdHigh:
		return FlashcardCountHigh
	default:
		return FlashcardCountMax
	}
}

// buildPageBoundedContext fetches structured chunk context for a notebook page range
// and returns (chunks, tokenCount, error).
// This is the canonical bounded context pipeline used by both manual and automatic flashcard generation.
func buildPageBoundedContext(notebookID string, startPage, endPage int) ([]models.ChunkWithContext, int, error) {
	utils.Warnf("[FLASHCARD_PIPELINE] buildPageBoundedContext entry notebookID=%s page_range=%d-%d", notebookID, startPage, endPage)
	chunks, err := db.GetChunksWithContextByNotebookPageRange(notebookID, startPage, endPage)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to load page-bounded context: %w", err)
	}
	utils.Warnf("[FLASHCARD_PIPELINE] buildPageBoundedContext raw_chunks=%d", len(chunks))
	if len(chunks) == 0 {
		// Return empty response instead of error for manual mode compatibility
		return []models.ChunkWithContext{}, 0, nil
	}

	const maxContextChunks = 120
	if len(chunks) > maxContextChunks {
		chunks = chunks[:maxContextChunks]
	}

	// Calculate final token count for the bounded chunk set
	finalTokenCount := calculatePromptTokenCount(chunks)
	utils.Warnf("[FLASHCARD_PIPELINE] buildPageBoundedContext exit chunks=%d token_count=%d", len(chunks), finalTokenCount)

	return chunks, finalTokenCount, nil
}

// calculatePromptTokenCount estimates the actual token count that will be sent to the LLM
// including prompt overhead and chunk formatting (chunk_id: | page_num: | text: format)
func calculatePromptTokenCount(chunks []models.ChunkWithContext) int {
	const maxContextChunks = 120
	limit := len(chunks)
	if limit > maxContextChunks {
		limit = maxContextChunks
	}

	// Base prompt overhead (instructions, format, etc.)
	baseOverhead := 200 // Estimated tokens for prompt template and instructions

	// Build the complete formatted content once for efficient token counting
	var contentBuilder strings.Builder
	for i := 0; i < limit; i++ {
		chunk := chunks[i]
		text := strings.TrimSpace(chunk.Text)
		if text == "" {
			continue
		}
		// Format: "- chunk_id: %s | page_num: %d | text: %s\n"
		fmt.Fprintf(&contentBuilder, "- chunk_id: %s | page_num: %d | text: %s\n", chunk.ChunkID, chunk.PageNum, text)
	}

	// Add truncation notice if needed
	if len(chunks) > maxContextChunks {
		contentBuilder.WriteString("[...additional chunks truncated...]")
	}

	formattedContent := contentBuilder.String()

	// Count tokens once for the complete formatted content
	if contentTokens, err := embeddings.CountTokens(formattedContent); err == nil {
		return baseOverhead + contentTokens
	} else {
		// Fallback to word count if tokenization fails
		return baseOverhead + len(strings.Fields(formattedContent))
	}
}

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

// suppressedUnusedImportForUUID ensures uuid is imported in sub-files via
// a blank declaration here so the import is always visible to the compiler.
// (Individual sub-files call uuid.NewString() directly.)
var _ = uuid.NewString
var _ = utils.Warnf
