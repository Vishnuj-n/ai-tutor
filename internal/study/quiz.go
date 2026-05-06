package study

import (
	"fmt"
	"strings"
	"time"

	"ai-tutor/internal/db"
	"ai-tutor/internal/models"
	"ai-tutor/internal/utils"

	"github.com/google/uuid"
)

// GenerateMarathonQuiz generates multiple-choice questions from the raw text
// of a notebook's page range, injecting context directly into the prompt
// (no RAG / ONNX vectors).  LLM tier is auto-selected by word count.
func (s *StudyService) GenerateMarathonQuiz(notebookID string, startPage, endPage int) map[string]interface{} {
	notebookID = strings.TrimSpace(notebookID)
	if notebookID == "" {
		return map[string]interface{}{"error": "notebook ID is required"}
	}
	if startPage <= 0 || endPage <= 0 || endPage < startPage {
		return map[string]interface{}{"error": fmt.Sprintf("invalid page range: start=%d end=%d", startPage, endPage)}
	}

	contextChunks, tokenCount, err := BuildPageBoundedContext(notebookID, startPage, endPage)
	if err != nil {
		return map[string]interface{}{"error": err.Error()}
	}
	contextText := buildContextTextFromChunks(contextChunks)

	llm, tier := s.selectLLM(contextText)
	if llm == nil {
		return map[string]interface{}{"error": "no LLM provider available (tier: " + tier + ")"}
	}

	targetCount := scaledQuizQuestionCount(tokenCount)
	prompt := buildMarathonQuizPrompt(notebookID, startPage, endPage, contextChunks, tokenCount, targetCount)

	raw, err := llm.GenerateAnswer(prompt)
	if err != nil {
		return map[string]interface{}{"error": "quiz generation failed: " + err.Error()}
	}
	parsed, err := parseQuizLLMResponse(raw)
	if err != nil {
		return map[string]interface{}{"error": "quiz parsing failed: " + err.Error()}
	}

	modelName := providerModelName(llm)
	// Build a synthetic topic reference so existing ScoreAnswer / FSRS can work.
	syntheticTopicID := fmt.Sprintf("marathon-%s-p%d-%d", notebookID, startPage, endPage)

	questions := make([]models.QuizQuestion, 0, len(parsed.Questions))
	allowedChunkIDs := make(map[string]struct{}, len(contextChunks))
	for _, chunk := range contextChunks {
		allowedChunkIDs[chunk.ChunkID] = struct{}{}
	}
	for _, q := range parsed.Questions {
		sourceChunkID := strings.TrimSpace(q.SourceChunkID)
		if strings.TrimSpace(q.Prompt) == "" || len(q.Options) < 2 || strings.TrimSpace(q.CorrectAnswer) == "" || sourceChunkID == "" {
			if strings.TrimSpace(q.Prompt) == "" {
				utils.Warnf("Skipping quiz question: empty prompt")
			}
			if len(q.Options) < 2 {
				utils.Warnf("Skipping quiz question: insufficient options (%d)", len(q.Options))
			}
			if strings.TrimSpace(q.CorrectAnswer) == "" {
				utils.Warnf("Skipping quiz question: empty correct_answer")
			}
			if sourceChunkID == "" {
				utils.Warnf("Skipping quiz question: empty source_chunk_id")
			}
			continue
		}
		if _, ok := allowedChunkIDs[sourceChunkID]; !ok {
			utils.Warnf("Skipping quiz question: source_chunk_id '%s' not found in allowed chunks (total allowed: %d)", sourceChunkID, len(allowedChunkIDs))
			continue
		}
		matchedOption, ok := resolveCorrectOption(q.CorrectAnswer, q.Options)
		if !ok {
			continue
		}
		srcStart := q.SourcePageStart
		srcEnd := q.SourcePageEnd
		if srcStart <= 0 {
			srcStart = startPage
		}
		if srcEnd <= 0 || srcEnd < srcStart {
			srcEnd = endPage
		}
		questions = append(questions, models.QuizQuestion{
			ID:              uuid.NewString(),
			TopicID:         syntheticTopicID,
			SourceChunkID:   sourceChunkID,
			Prompt:          strings.TrimSpace(q.Prompt),
			Options:         q.Options,
			CorrectAnswer:   matchedOption,
			Explanation:     strings.TrimSpace(q.Explanation),
			Hint:            strings.TrimSpace(q.Hint),
			SourceHeading:   strings.TrimSpace(q.SourceHeading),
			SourceSnippet:   strings.TrimSpace(q.SourceSnippet),
			SourcePageStart: srcStart,
			SourcePageEnd:   srcEnd,
			LLMModel:        modelName,
			PromptVersion:   "marathon-quiz-v1",
		})
	}
	if len(questions) == 0 {
		return map[string]interface{}{"error": "no valid questions generated from page range"}
	}

	// Ensure the synthetic topic row exists for FK constraints on questions table.
	if err := db.EnsureTopicsBatch([]db.TopicBatchItem{{TopicID: syntheticTopicID, Title: fmt.Sprintf("Marathon %s p%d-%d", notebookID, startPage, endPage)}}); err != nil {
		fmt.Printf("failed to create synthetic topic %s for marathon quiz: %v\n", syntheticTopicID, err)
		return map[string]interface{}{"error": "failed to create synthetic topic for marathon quiz: " + err.Error()}
	}
	if err := db.ReplaceQuestionsForTopic(syntheticTopicID, questions); err != nil {
		return map[string]interface{}{"error": "failed to persist marathon quiz: " + err.Error()}
	}

	return map[string]interface{}{
		"notebook_id":       notebookID,
		"start_page":        startPage,
		"end_page":          endPage,
		"topic_id":          syntheticTopicID,
		"questions":         questions,
		"question_count":    len(questions),
		"llm_tier":          tier,
		"generated_at_unix": time.Now().Unix(),
	}
}

// buildMarathonQuizPrompt constructs the chunk-anchored prompt for quiz generation.
func buildMarathonQuizPrompt(notebookID string, startPage, endPage int, contextChunks []models.ChunkWithContext, tokenCount, targetCount int) string {
	var b strings.Builder
	b.WriteString("You are an AI tutor quiz generator. Return STRICT JSON only. No markdown.\n")
	fmt.Fprintf(&b, "Generate exactly %d multiple-choice questions covering pages %d-%d of notebook '%s'.\n",
		targetCount, startPage, endPage, notebookID)
	fmt.Fprintf(&b, "Content token count: %d\n", tokenCount)
	b.WriteString(`JSON format: {"questions":[{"source_chunk_id":string,"prompt":string,"options":[string,string,string,string],"correct_answer":string,"explanation":string,"hint":string,"source_heading":string,"source_snippet":string,"source_page_start":number,"source_page_end":number}]}` + "\n")
	b.WriteString("\n=== QUESTION DIVERSITY (CRITICAL) ===\n")
	b.WriteString("Cover different concepts: recall, application/analysis, and one misconception when count allows.\n")
	b.WriteString("\n=== RULES ===\n")
	b.WriteString("- correct_answer must match one option exactly.\n")
	b.WriteString("- Keep each option short (< 15 words).\n")
	b.WriteString("- Explanations grounded in source text.\n")
	b.WriteString("- Each question must require understanding, not just recall.\n")
	b.WriteString("- source_chunk_id must exactly match one chunk_id from the provided chunk list.\n")
	b.WriteString("\n=== SOURCE CHUNKS ===\n")
	const maxContextChunks = 120
	limit := len(contextChunks)
	if limit > maxContextChunks {
		limit = maxContextChunks
	}
	for i := 0; i < limit; i++ {
		chunk := contextChunks[i]
		text := strings.TrimSpace(chunk.Text)
		if text == "" {
			continue
		}
		fmt.Fprintf(&b, "- chunk_id: %s | page_num: %d | text: %s\n", chunk.ChunkID, chunk.PageNum, text)
	}
	if len(contextChunks) > maxContextChunks {
		b.WriteString("[...additional chunks truncated...]\n")
	}
	return b.String()
}
