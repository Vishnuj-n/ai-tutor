package study

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	"ai-tutor/internal/db"
	"ai-tutor/internal/embeddings"
	"ai-tutor/internal/models"
	"ai-tutor/internal/retrieval"
	"ai-tutor/internal/scheduler"
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
}

// Config wires all dependencies into StudyService via constructor injection.
type Config struct {
	FastLLMProvider         LLMProvider
	HeavyLLMProvider        LLMProvider
	RetrievalEngine         *retrieval.Engine
	ApplyAssessmentReviewTx func(tx *sql.Tx, topicID, activityType, referenceID, ratingLabel string) (map[string]interface{}, error)
}

// StudyService owns all study-mode generation and scoring logic.
// quiz.go, flashcard.go, examiner.go, and socratic.go add methods to this type.
type StudyService struct {
	fastLLMProvider         LLMProvider
	heavyLLMProvider        LLMProvider
	retrievalEngine         *retrieval.Engine
	applyAssessmentReviewTx func(tx *sql.Tx, topicID, activityType, referenceID, ratingLabel string) (map[string]interface{}, error)
}

// NewStudyService constructs a StudyService from injected dependencies.
func NewStudyService(cfg Config) *StudyService {
	return &StudyService{
		fastLLMProvider:         cfg.FastLLMProvider,
		heavyLLMProvider:        cfg.HeavyLLMProvider,
		retrievalEngine:         cfg.RetrievalEngine,
		applyAssessmentReviewTx: cfg.ApplyAssessmentReviewTx,
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

// ExplainReaderSection explains one reader section without topic-wide retrieval.
func (s *StudyService) ExplainReaderSection(sectionID string, question string) map[string]interface{} {
	sectionID = strings.TrimSpace(sectionID)
	question = strings.TrimSpace(question)
	if sectionID == "" {
		return map[string]interface{}{"error": "section ID is required"}
	}
	if s.fastLLMProvider == nil {
		return map[string]interface{}{"error": "FAST_LLM provider not initialized"}
	}

	section, err := db.GetParentSection(sectionID)
	if err != nil {
		return map[string]interface{}{"error": "failed to fetch reader section: " + err.Error()}
	}
	if question == "" {
		question = "Explain this section in clear study notes."
	}

	prompt := fmt.Sprintf(`You are an AI study companion.
Use ONLY the section below. Do not add outside knowledge.
If a question asks about details missing from the section, reply with: "This section does not contain that detail."

Section heading: %s
Section content:
%s

User request: %s

Return a response with:
1. Plain-language summary (2–3 sentences, main idea)
2. Key takeaway: why this matters or where it applies
3. Recall cue: one memorable phrase or question to test understanding
4. Example (if the section includes a concrete example or scenario, highlight it; otherwise, skip this)`, section["heading"], section["content"], question)

	answer, err := s.fastLLMProvider.GenerateAnswer(prompt)
	if err != nil {
		return map[string]interface{}{"error": "section explanation failed: " + err.Error()}
	}
	return map[string]interface{}{
		"answer":         answer,
		"cited_sections": []string{section["heading"]},
		"section_id":     section["id"],
	}
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
	if s.applyAssessmentReviewTx == nil {
		return map[string]interface{}{"error": "assessment review callback not initialized"}
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
	ratingLabel, _ := shortAnswerScoreToFSRSRating(score)
	fsrsResult, err := s.applyAssessmentReviewTx(tx, question.TopicID, "written_question", question.ID, ratingLabel)
	if err != nil {
		return map[string]interface{}{"error": "failed to update written-assessment FSRS: " + err.Error()}
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
		"fsrsRating":        fsrsResult["fsrs_rating"],
		"scheduled_days":    fsrsResult["scheduled_days"],
		"next_review_at":    fsrsResult["next_review_at"],
		"review_log_id":     fsrsResult["review_log_id"],
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
	start := strings.Index(raw, "{")
	end := strings.LastIndex(raw, "}")
	if start >= 0 && end > start {
		raw = raw[start : end+1]
	}
	var out flashcardLLMResponse
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil, err
	}
	if len(out.Cards) == 0 {
		return nil, fmt.Errorf("no cards in LLM response")
	}
	return &out, nil
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

func resolveCorrectOption(correctAnswer string, options []string) (string, bool) {
	canonical := strings.TrimSpace(strings.ToLower(correctAnswer))
	for _, opt := range options {
		if strings.TrimSpace(strings.ToLower(opt)) == canonical {
			return strings.TrimSpace(opt), true
		}
	}
	return "", false
}

func normalizeHeadingKey(heading string) string {
	if heading == "" {
		return ""
	}
	cleaned := strings.FieldsFunc(strings.TrimSpace(heading), func(r rune) bool {
		return (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') && (r < '0' || r > '9') && r != ' '
	})
	return strings.ToLower(strings.Join(cleaned, " "))
}

func semanticSnippet(content string, limit int) string {
	trimmed := strings.TrimSpace(content)
	if limit <= 0 || trimmed == "" {
		return ""
	}
	runes := []rune(trimmed)
	if len(runes) <= limit {
		return trimmed
	}
	cutRunes := runes[:limit]
	cut := string(cutRunes)
	best := strings.LastIndex(cut, ". ")
	if idx := strings.LastIndex(cut, "\n"); idx > best {
		best = idx
	}
	if idx := strings.LastIndex(cut, "? "); idx > best {
		best = idx
	}
	if idx := strings.LastIndex(cut, "! "); idx > best {
		best = idx
	}
	if best > limit/2 {
		candidate := strings.TrimSpace(cut[:best+1])
		if candidate != "" {
			candidateRunes := []rune(candidate)
			if len(candidateRunes) > limit-3 {
				candidateRunes = candidateRunes[:limit-3]
			}
			return string(candidateRunes) + "..."
		}
	}
	cut = strings.TrimSpace(cut)
	cutRunes = []rune(cut)
	if len(cutRunes) > limit-3 {
		cutRunes = cutRunes[:limit-3]
	}
	return string(cutRunes) + "..."
}

func semanticSnippetByTokens(content string, maxTokens int) (string, error) {
	trimmed := strings.TrimSpace(content)
	if maxTokens <= 0 || trimmed == "" {
		return "", nil
	}
	tokens, err := embeddings.CountTokens(trimmed)
	if err != nil {
		return "", fmt.Errorf("failed to count tokens: %w", err)
	}
	if tokens <= maxTokens {
		return trimmed, nil
	}
	truncated, err := embeddings.TruncateToTokens(trimmed, maxTokens)
	if err != nil {
		return "", fmt.Errorf("failed to truncate to tokens: %w", err)
	}
	truncatedTokens, err := embeddings.CountTokens(truncated)
	if err != nil {
		return "", fmt.Errorf("failed to count truncated tokens: %w", err)
	}
	if truncatedTokens > maxTokens {
		conservativeLimit := maxTokens - 10
		if conservativeLimit > 0 {
			conservative, err := embeddings.TruncateToTokens(trimmed, conservativeLimit)
			if err != nil {
				return "", fmt.Errorf("failed to truncate conservatively: %w", err)
			}
			conservativeTokens, verifyErr := embeddings.CountTokens(conservative)
			if verifyErr != nil {
				return "", fmt.Errorf("failed to count conservative tokens: %w", verifyErr)
			}
			if conservativeTokens <= maxTokens {
				truncated = conservative
				truncatedTokens = conservativeTokens
			}
		}
	}
	best := strings.LastIndex(truncated, ". ")
	if idx := strings.LastIndex(truncated, "\n"); idx > best {
		best = idx
	}
	if idx := strings.LastIndex(truncated, "? "); idx > best {
		best = idx
	}
	if idx := strings.LastIndex(truncated, "! "); idx > best {
		best = idx
	}
	if best > len(truncated)/2 {
		candidate := strings.TrimSpace(truncated[:best+1])
		if candidate != "" {
			candidateWithEllipsis := candidate + "..."
			candidateTokens, err := embeddings.CountTokens(candidateWithEllipsis)
			if err != nil {
				return "", fmt.Errorf("failed to count candidate tokens: %w", err)
			}
			if candidateTokens <= maxTokens {
				return candidateWithEllipsis, nil
			}
		}
	}
	if truncatedTokens <= maxTokens {
		return truncated, nil
	}
	return "", fmt.Errorf("truncated text exceeds maxTokens: %d > %d", truncatedTokens, maxTokens)
}

func shortAnswerScoreToFSRSRating(score int) (string, int) {
	switch {
	case score <= 3:
		return "again", scheduler.Again
	case score <= 5:
		return "hard", scheduler.Hard
	case score <= 8:
		return "good", scheduler.Good
	default:
		return "easy", scheduler.Easy
	}
}

// ---------- Env-configurable density thresholds ----------

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
func buildPageBoundedContext(notebookID string, startPage, endPage int) ([]models.ChunkWithContext, int, error) {
	chunks, err := db.GetChunksWithContextByNotebookPageRange(notebookID, startPage, endPage)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to load page-bounded context: %w", err)
	}
	if len(chunks) == 0 {
		// Return empty response instead of error for marathon mode compatibility
		return []models.ChunkWithContext{}, 0, nil
	}

	// Calculate tokens based on actual LLM prompt format, not just raw text
	tokenCount := calculatePromptTokenCount(chunks)

	return chunks, tokenCount, nil
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

	var contentTokens int
	for i := 0; i < limit; i++ {
		chunk := chunks[i]
		text := strings.TrimSpace(chunk.Text)
		if text == "" {
			continue
		}
		// Format: "- chunk_id: %s | page_num: %d | text: %s\n"
		chunkLine := fmt.Sprintf("- chunk_id: %s | page_num: %d | text: %s\n", chunk.ChunkID, chunk.PageNum, text)
		// Count tokens for this formatted chunk line
		if chunkTokens, err := embeddings.CountTokens(chunkLine); err == nil {
			contentTokens += chunkTokens
		} else {
			// Fallback to word count
			contentTokens += len(strings.Fields(chunkLine))
		}
	}

	// Add truncation notice if needed
	if len(chunks) > maxContextChunks {
		contentTokens += 10 // Tokens for "[...additional chunks truncated...]"
	}

	return baseOverhead + contentTokens
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
