package study

import (
	"fmt"
	"strings"

	"ai-tutor/internal/db"
	"ai-tutor/internal/models"

	"github.com/google/uuid"
)

// GenerateComprehensiveExam generates a short-answer written assessment question
// from the raw text of a notebook's page range (no RAG / ONNX).
func (s *StudyService) GenerateComprehensiveExam(notebookID string, startPage, endPage int) map[string]interface{} {
	notebookID = strings.TrimSpace(notebookID)
	if notebookID == "" {
		return map[string]interface{}{"error": "notebook ID is required"}
	}
	if startPage <= 0 || endPage <= 0 || endPage < startPage {
		return map[string]interface{}{"error": fmt.Sprintf("invalid page range: start=%d end=%d", startPage, endPage)}
	}

	contextChunks, _, err := buildPageBoundedContext(notebookID, startPage, endPage)
	if err != nil {
		return map[string]interface{}{"error": err.Error()}
	}
	contextText := buildContextTextFromChunks(contextChunks)

	llm, tier := s.selectLLM(contextText)
	if llm == nil {
		return map[string]interface{}{"error": "no LLM provider available (tier: " + tier + ")"}
	}

	prompt := buildComprehensiveExamPrompt(notebookID, startPage, endPage, contextText)
	raw, err := llm.GenerateAnswer(prompt)
	if err != nil {
		return map[string]interface{}{"error": "exam generation failed: " + err.Error()}
	}
	parsed, err := parseShortAnswerPromptLLMResponse(raw)
	if err != nil {
		return map[string]interface{}{"error": "exam prompt parsing failed: " + err.Error()}
	}
	questionPrompt := strings.TrimSpace(parsed.Prompt)
	if questionPrompt == "" {
		return map[string]interface{}{"error": "exam prompt generation returned empty question"}
	}

	syntheticTopicID := fmt.Sprintf("comprehensive-%s-p%d-%d", notebookID, startPage, endPage)

	if err := db.EnsureTopicsBatch([]db.TopicBatchItem{{
		TopicID: syntheticTopicID,
		Title:   fmt.Sprintf("Comprehensive %s p%d-%d", notebookID, startPage, endPage),
	}}); err != nil {
		fmt.Printf("failed to create synthetic topic %s for comprehensive exam in notebook %s: %v\n", syntheticTopicID, notebookID, err)
		return map[string]interface{}{"error": "failed to create synthetic topic for comprehensive exam: " + err.Error()}
	}

	question := models.WrittenQuestion{
		ID:              uuid.NewString(),
		TopicID:         syntheticTopicID,
		Prompt:          questionPrompt,
		SourcePageStart: startPage,
		SourcePageEnd:   endPage,
		LLMModel:        providerModelName(llm),
		PromptVersion:   "comprehensive-exam-v1",
	}
	if err := db.CreateWrittenQuestion(question); err != nil {
		return map[string]interface{}{"error": "failed to persist comprehensive exam question: " + err.Error()}
	}

	return map[string]interface{}{
		"questionID":        question.ID,
		"prompt":            question.Prompt,
		"topicID":           syntheticTopicID,
		"notebook_id":       notebookID,
		"start_page":        startPage,
		"end_page":          endPage,
		"llm_tier":          tier,
		"source_page_start": startPage,
		"source_page_end":   endPage,
	}
}

func buildComprehensiveExamPrompt(notebookID string, startPage, endPage int, contextText string) string {
	var b strings.Builder
	b.WriteString("You are an AI tutor generating a short-answer assessment question.\n")
	fmt.Fprintf(&b, "Generate exactly one short-answer question grounded in pages %d-%d of notebook '%s'.\n",
		startPage, endPage, notebookID)
	b.WriteString(`Return STRICT JSON only in this shape: {"prompt":"..."}.` + "\n")
	b.WriteString("Rules:\n")
	b.WriteString("- Ask exactly one question.\n")
	b.WriteString("- Keep it concise (max 30 words).\n")
	b.WriteString("- Require understanding, not pure definition recall.\n")
	b.WriteString("- Do not include answer choices, rubric, preamble, or markdown.\n")
	b.WriteString("\n=== SOURCE MATERIAL ===\n")
	const maxContextRunes = 20000
	runes := []rune(contextText)
	if len(runes) > maxContextRunes {
		runes = runes[:maxContextRunes]
		contextText = string(runes) + "\n[...content truncated...]"
	}
	b.WriteString(contextText)
	return b.String()
}
