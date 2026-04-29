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

	topicStartPage, topicEndPage, err := db.GetTopicPageBounds(topicID)
	if err != nil {
		return map[string]interface{}{"error": "failed to fetch topic page bounds: " + err.Error()}
	}
	if topicEndPage <= 0 {
		return map[string]interface{}{"error": "topic has no page bounds configured"}
	}

	// Adjust startPage before validation to prevent misleading errors when startPage <= 0
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
	// Convert token count to estimated word count for scaledQuizQuestionCount
	// scaledQuizQuestionCount expects word counts, but we have token counts
	estimatedWordCount := totalChunkTokens * 3 / 4 // Approximate 4 tokens per word ratio
	expectedQuestionCount := scaledQuizQuestionCount(estimatedWordCount)
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
	var failedQuestions []string
	var validationErrors []string

	for i, q := range parsed.Questions {
		if strings.TrimSpace(q.Prompt) == "" {
			failedQuestions = append(failedQuestions, fmt.Sprintf("question %d: empty prompt", i+1))
			validationErrors = append(validationErrors, fmt.Sprintf("question %d failed validation: empty prompt", i+1))
			continue
		}
		if len(q.Options) < 2 {
			failedQuestions = append(failedQuestions, fmt.Sprintf("question %d: insufficient options (%d)", i+1, len(q.Options)))
			validationErrors = append(validationErrors, fmt.Sprintf("question %d failed validation: insufficient options (%d)", i+1, len(q.Options)))
			continue
		}
		if strings.TrimSpace(q.CorrectAnswer) == "" {
			failedQuestions = append(failedQuestions, fmt.Sprintf("question %d: empty correct answer", i+1))
			validationErrors = append(validationErrors, fmt.Sprintf("question %d failed validation: empty correct answer", i+1))
			continue
		}
		matchedOption, ok := resolveCorrectOption(q.CorrectAnswer, q.Options)
		if !ok {
			failedQuestions = append(failedQuestions, fmt.Sprintf("question %d: correct answer '%s' not found in options", i+1, q.CorrectAnswer))
			validationErrors = append(validationErrors, fmt.Sprintf("question %d failed validation: correct answer '%s' not found in options", i+1, q.CorrectAnswer))
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

	// Log validation failures if any
	if len(failedQuestions) > 0 {
		utils.Warnf("topic %s: %d questions failed validation: %s", topicID, len(failedQuestions), strings.Join(failedQuestions, "; "))
	}

	// Check question count with tolerance for small shortfall
	minAcceptableCount := expectedQuestionCount - 1
	if minAcceptableCount < 1 {
		minAcceptableCount = 1 // Always require at least 1 valid question
	}

	if len(questions) < minAcceptableCount {
		// Try retry if we have some questions but not enough
		if len(questions) > 0 {
			utils.Warnf("topic %s: retrying quiz generation - got %d valid questions, need at least %d (expected %d)",
				topicID, len(questions), minAcceptableCount, expectedQuestionCount)

			// Build retry prompt with stronger validation hints
			retryPrompt, err := buildReaderCompletionRetryPrompt(topicID, startPage, targetPage, contextEndPage, parentPassages, totalChunkTokens, expectedQuestionCount, validationErrors)
			if err != nil {
				return map[string]interface{}{"error": "completion quiz retry prompt generation failed: " + err.Error()}
			}

			retryRaw, err := s.fastLLMProvider.GenerateAnswer(retryPrompt)
			if err != nil {
				return map[string]interface{}{"error": "completion quiz retry generation failed: " + err.Error()}
			}

			retryParsed, err := parseQuizLLMResponse(retryRaw)
			if err != nil {
				return map[string]interface{}{"error": "completion quiz retry parsing failed: " + err.Error()}
			}

			// Process retry questions
			retryQuestions := make([]models.QuizQuestion, 0, len(retryParsed.Questions))
			var retryFailedQuestions []string

			for i, q := range retryParsed.Questions {
				if strings.TrimSpace(q.Prompt) == "" || len(q.Options) < 2 || strings.TrimSpace(q.CorrectAnswer) == "" {
					retryFailedQuestions = append(retryFailedQuestions, fmt.Sprintf("retry question %d: basic validation failed", i+1))
					continue
				}
				matchedOption, ok := resolveCorrectOption(q.CorrectAnswer, q.Options)
				if !ok {
					retryFailedQuestions = append(retryFailedQuestions, fmt.Sprintf("retry question %d: correct answer not found", i+1))
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
				retryQuestions = append(retryQuestions, models.QuizQuestion{
					ID: uuid.NewString(), TopicID: topicID,
					Prompt: strings.TrimSpace(q.Prompt), Options: q.Options,
					CorrectAnswer: matchedOption, Explanation: strings.TrimSpace(q.Explanation),
					Hint: strings.TrimSpace(q.Hint), SourceHeading: strings.TrimSpace(q.SourceHeading),
					SourceSnippet:   strings.TrimSpace(q.SourceSnippet),
					SourcePageStart: srcStart, SourcePageEnd: srcEnd,
					LLMModel: modelName, PromptVersion: "reader-complete-v2-density-retry",
				})
			}

			if len(retryFailedQuestions) > 0 {
				utils.Warnf("topic %s retry: %d questions failed validation: %s", topicID, len(retryFailedQuestions), strings.Join(retryFailedQuestions, "; "))
			}

			// Use retry results if better, otherwise keep original
			if len(retryQuestions) >= len(questions) {
				questions = retryQuestions
				utils.Infof("topic %s: retry succeeded - got %d valid questions", topicID, len(questions))
			} else {
				utils.Warnf("topic %s: retry produced fewer valid questions (%d) than original (%d), keeping original", topicID, len(retryQuestions), len(questions))
			}
		}

		// Final check after retry
		if len(questions) < minAcceptableCount {
			return map[string]interface{}{"error": fmt.Sprintf("completion quiz produced %d valid questions; minimum acceptable %d (expected %d)", len(questions), minAcceptableCount, expectedQuestionCount)}
		}
	}

	utils.Infof("topic %s: quiz generation completed with %d valid questions (expected %d)", topicID, len(questions), expectedQuestionCount)

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
				bufferEmpty = false
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
		return "", fmt.Errorf("no context passages could be included for topic %s", topicID)
	}
	return b.String(), nil
}

func buildReaderCompletionRetryPrompt(topicID string, startPage, targetPage, contextEndPage int, parentPassages []string, totalChunkTokens, targetCount int, validationErrors []string) (string, error) {
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

	// Add validation error feedback
	if len(validationErrors) > 0 {
		b.WriteString("\n\nPREVIOUS ATTEMPT VALIDATION ERRORS (FIX THESE IN YOUR RESPONSE):\n")
		for _, err := range validationErrors {
			fmt.Fprintf(&b, "- %s\n", err)
		}
		b.WriteString("\nCRITICAL: Ensure all questions have:\n")
		b.WriteString("- Non-empty prompt text\n")
		b.WriteString("- Exactly 4 distinct options\n")
		b.WriteString("- A correct_answer that EXACTLY MATCHES one of the options\n")
		b.WriteString("- Proper source_page_start and source_page_end numbers\n")
	}

	b.WriteString("\nJSON format: {\"questions\":[{\"prompt\":string,\"options\":[string,string,string,string],\"correct_answer\":string,\"explanation\":string,\"hint\":string,\"source_heading\":string,\"source_snippet\":string,\"source_page_start\":number,\"source_page_end\":number}]}\n")
	fmt.Fprintf(&b, "- Return exactly %d questions.\n", targetCount)
	b.WriteString("- correct_answer must match one option EXACTLY (case-sensitive).\n")
	b.WriteString("- All options must be distinct and non-empty.\n")
	b.WriteString("- Keep all questions grounded in the context below.\n")
	b.WriteString("- Double-check that correct_answer appears in options array.\n")
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
				bufferEmpty = false
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
	// Return error if no context was added (consistent with quiz prompt behavior)
	if bufferEmpty {
		return "", fmt.Errorf("no context passages for retry prompt (topicID: %s)", topicID)
	}
	return b.String(), nil
}
