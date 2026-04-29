package study

import (
	"fmt"
	"strings"

	"ai-tutor/internal/db"
	"ai-tutor/internal/embeddings"
	"ai-tutor/internal/models"
	"ai-tutor/internal/utils"

	"github.com/google/uuid"
)

// CompleteReadingSession generates quiz questions for a locked page window and
// advances the topic's page cursor.  Serves the Reader flow exclusively.
func (s *StudyService) CompleteReadingSession(topicID string, startPage int, targetPage int) map[string]interface{} {
	topicID = strings.TrimSpace(topicID)
	if topicID == "" {
		return map[string]interface{}{"error": "topic ID is required"}
	}
	if targetPage <= 0 {
		return map[string]interface{}{"error": "target page must be positive"}
	}
	if s.fastLLMProvider == nil {
		return map[string]interface{}{"error": "FAST_LLM provider not initialized"}
	}

	currentCursor, err := db.GetTopicCurrentPageCursor(topicID)
	if err != nil {
		return map[string]interface{}{"error": "failed to fetch current page cursor: " + err.Error()}
	}
	if startPage != currentCursor {
		return map[string]interface{}{"error": fmt.Sprintf("invalid completion window: start page %d does not match current cursor %d", startPage, currentCursor)}
	}
	if targetPage <= currentCursor {
		return map[string]interface{}{"error": fmt.Sprintf("invalid completion window: target page %d must be greater than current cursor %d", targetPage, currentCursor)}
	}

	topicStartPage, topicEndPage, err := db.GetTopicPageBounds(topicID)
	if err != nil {
		return map[string]interface{}{"error": "failed to fetch topic page bounds: " + err.Error()}
	}
	if topicEndPage <= 0 {
		return map[string]interface{}{"error": "topic has no page bounds configured"}
	}
	if startPage <= 0 {
		if topicStartPage > 0 {
			startPage = topicStartPage
		} else {
			startPage = 1
		}
	}
	if topicStartPage > 0 && startPage < topicStartPage {
		startPage = topicStartPage
	}
	if targetPage > topicEndPage {
		targetPage = topicEndPage
	}
	// Re-validate after adjusting targetPage to ensure it's still >= startPage
	if targetPage < startPage {
		return map[string]interface{}{"error": "invalid completion window: target page must be >= start page"}
	}

	contextEndPage := targetPage + 1
	if contextEndPage > topicEndPage {
		contextEndPage = topicEndPage
	}
	parentPassages, err := db.GetParentPassagesForTopicPageRange(topicID, startPage, contextEndPage)
	if err != nil {
		return map[string]interface{}{"error": "failed to fetch completion passages: " + err.Error()}
	}
	if len(parentPassages) == 0 {
		return map[string]interface{}{"error": "no passage content found for completion window"}
	}

	totalChunkTokens, err := db.GetTotalChunkTokensForPageRange(topicID, startPage, contextEndPage)
	if err != nil {
		return map[string]interface{}{"error": "failed to calculate completion quiz density: " + err.Error()}
	}
	expectedQuestionCount := scaledQuizQuestionCount(totalChunkTokens)
	prompt, err := buildReaderCompletionQuizPrompt(topicID, startPage, targetPage, contextEndPage, parentPassages, totalChunkTokens, expectedQuestionCount)
	if err != nil {
		return map[string]interface{}{"error": "completion quiz prompt generation failed: " + err.Error()}
	}
	raw, err := s.fastLLMProvider.GenerateAnswer(prompt)
	if err != nil {
		return map[string]interface{}{"error": "completion quiz generation failed: " + err.Error()}
	}
	parsed, err := parseQuizLLMResponse(raw)
	if err != nil {
		return map[string]interface{}{"error": "completion quiz parsing failed: " + err.Error()}
	}

	modelName := providerModelName(s.fastLLMProvider)
	questions := make([]models.QuizQuestion, 0, len(parsed.Questions))
	for _, q := range parsed.Questions {
		if strings.TrimSpace(q.Prompt) == "" || len(q.Options) < 2 || strings.TrimSpace(q.CorrectAnswer) == "" {
			continue
		}
		matchedOption, ok := resolveCorrectOption(q.CorrectAnswer, q.Options)
		if !ok {
			continue
		}
		srcStart, srcEnd := q.SourcePageStart, q.SourcePageEnd
		if srcStart <= 0 || srcEnd <= 0 || srcEnd < srcStart {
			srcStart, srcEnd = startPage, contextEndPage
		}
		if srcStart < startPage {
			srcStart = startPage
		}
		if srcEnd > contextEndPage {
			srcEnd = contextEndPage
		}
		if srcEnd < srcStart {
			srcEnd = srcStart
		}
		questions = append(questions, models.QuizQuestion{
			ID: uuid.NewString(), TopicID: topicID,
			Prompt: strings.TrimSpace(q.Prompt), Options: q.Options,
			CorrectAnswer: matchedOption, Explanation: strings.TrimSpace(q.Explanation),
			Hint: strings.TrimSpace(q.Hint), SourceHeading: strings.TrimSpace(q.SourceHeading),
			SourceSnippet:   strings.TrimSpace(q.SourceSnippet),
			SourcePageStart: srcStart, SourcePageEnd: srcEnd,
			LLMModel: modelName, PromptVersion: "reader-complete-v2-density",
		})
	}
	if len(questions) != expectedQuestionCount {
		return map[string]interface{}{"error": fmt.Sprintf("completion quiz produced %d valid questions; expected exactly %d", len(questions), expectedQuestionCount)}
	}

	nextCursor := targetPage + 1
	markLearned := targetPage >= topicEndPage
	if err := db.AppendQuestionsAndAdvanceCursor(topicID, questions, nextCursor, markLearned); err != nil {
		return map[string]interface{}{"error": "failed to append completion questions and update cursor: " + err.Error()}
	}
	status := "reading"
	if markLearned {
		status = "learned"
	}
	return map[string]interface{}{
		"ok": true, "topic_id": topicID,
		"source_page_start": startPage, "source_page_end": contextEndPage,
		"target_page": targetPage, "questions_generated": len(questions),
		"prompt_version":      "reader-complete-v2-density",
		"current_page_cursor": nextCursor, "topic_status": status,
	}
}

func buildReaderCompletionQuizPrompt(topicID string, startPage, targetPage, contextEndPage int, parentPassages []string, totalChunkTokens, targetCount int) (string, error) {
	var b strings.Builder
	b.WriteString("You are an AI tutor quiz generator. Return STRICT JSON only. No markdown.\n")
	fmt.Fprintf(&b, "Generate exactly %d multiple-choice questions for this completed reading session.\n", targetCount)
	b.WriteString("Topic ID: ")
	b.WriteString(topicID)
	fmt.Fprintf(&b, "\nMaterial density estimate: %d chunk tokens.", totalChunkTokens)
	fmt.Fprintf(&b, "\nLocked completion window: pages %d-%d", startPage, targetPage)
	fmt.Fprintf(&b, "\nAssessment context window: pages %d-%d", startPage, contextEndPage)
	if contextEndPage > targetPage {
		fmt.Fprintf(&b, "\nGenerate questions only from pages %d-%d. Page %d is buffer/supporting context only.", startPage, targetPage, contextEndPage)
	}
	b.WriteString("\nJSON format: {\"questions\":[{\"prompt\":string,\"options\":[string,string,string,string],\"correct_answer\":string,\"explanation\":string,\"hint\":string,\"source_heading\":string,\"source_snippet\":string,\"source_page_start\":number,\"source_page_end\":number}]}\n")
	fmt.Fprintf(&b, "- Return exactly %d questions.\n", targetCount)
	b.WriteString("- correct_answer must match one option exactly.\n")
	b.WriteString("- Keep all questions grounded in the context below.\n")
	b.WriteString("\nContext chunks (ordered):\n")

	const systemPromptTokens = 300
	const outputStructureTokens = 800
	const maxModelTokens = 4096
	availableContextTokens := maxModelTokens - systemPromptTokens - outputStructureTokens
	currentTokens := 0
	bufferEmpty := true
	for _, text := range parentPassages {
		passageTokens, err := embeddings.CountTokens(text)
		if err != nil {
			// Fallback to approximation if tokenizer fails
			passageTokens = len(text) / 4
		}
		if currentTokens+passageTokens > availableContextTokens {
			remainingTokens := availableContextTokens - currentTokens
			if remainingTokens > 0 {
				truncatedSnippet, err := semanticSnippetByTokens(text, remainingTokens)
				if err != nil {
					if bufferEmpty {
						return "", err
					}
					break
				}
				b.WriteString("- ")
				b.WriteString(truncatedSnippet)
				b.WriteString("\n")
			}
			break
		}
		snippet, err := semanticSnippetByTokens(text, passageTokens)
		if err != nil {
			if bufferEmpty {
				return "", err
			}
			break
		}
		b.WriteString("- ")
		b.WriteString(snippet)
		b.WriteString("\n")
		bufferEmpty = false
		currentTokens += passageTokens
	}
	if bufferEmpty {
		utils.Warnf("completion quiz prompt for topic %s has no context passages", topicID)
	}
	return b.String(), nil
}
