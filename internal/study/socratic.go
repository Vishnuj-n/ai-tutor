package study

// socratic.go — ONLY file that imports internal/retrieval.
// The GenerateShortAnswerPrompt method is the sole consumer of the
// SemanticSearch engine. All other study flows use page-bounded SQL injection.

import (
	"fmt"
	"strings"

	"ai-tutor/internal/embeddings"
	"ai-tutor/internal/models"
	"ai-tutor/internal/retrieval"
	"ai-tutor/internal/utils"

	"github.com/google/uuid"
)

// GenerateShortAnswerPrompt creates, persists, and returns one grounded short-answer
// question for the Socratic mode.  It is the only method in the study package
// that calls the vector retrieval engine.
func (s *StudyService) GenerateShortAnswerPrompt(topicID string) map[string]interface{} {
	topicID = strings.TrimSpace(topicID)
	if topicID == "" {
		return map[string]interface{}{"error": "topic ID is required"}
	}
	if s.fastLLMProvider == nil {
		return map[string]interface{}{"error": "FAST_LLM provider not initialized"}
	}
	if s.retrievalEngine == nil {
		return map[string]interface{}{"error": "retrieval engine not initialized"}
	}

	// Semantic search for the most relevant chunks in this topic.
	results, err := s.retrievalEngine.SemanticSearch(
		topicID,
		"Generate exactly one short-answer assessment question grounded in the retrieved material.",
		5, 0, 0,
	)
	if err != nil {
		return map[string]interface{}{"error": "retrieval failed: " + err.Error()}
	}
	if len(results) == 0 {
		return map[string]interface{}{"error": "no relevant content found for Socratic question"}
	}

	// Build context from top results.
	var contextBuilder strings.Builder
	chunkIDs := make([]string, 0, len(results))
	for _, r := range results {
		contextBuilder.WriteString(r.Text)
		contextBuilder.WriteByte('\n')
		chunkIDs = append(chunkIDs, r.ChunkID)
	}
	contextText := strings.TrimSpace(contextBuilder.String())

	prompt := fmt.Sprintf(`You are an AI tutor generating a short-answer assessment question.
Act like a human tutor talking to a confused student.
Prefer concrete examples over abstract analysis.
Start from the student's likely confusion.
Use ONLY the material below. Return STRICT JSON only in this shape: {"prompt":"..."}.
Rules:
- Ask exactly one question.
- Keep it concise (max 30 words).
- Require understanding, not pure definition recall.
- Do not include answer choices, rubric, preamble, or markdown.

Retrieved material:
%s`, contextText)

	raw, err := s.fastLLMProvider.GenerateAnswer(prompt)
	if err != nil {
		return map[string]interface{}{"error": "short-answer prompt generation failed: " + err.Error()}
	}
	parsed, err := parseShortAnswerPromptLLMResponse(raw)
	if err != nil {
		return map[string]interface{}{"error": "short-answer prompt parsing failed: " + err.Error()}
	}
	questionPrompt := strings.TrimSpace(parsed.Prompt)
	if questionPrompt == "" {
		return map[string]interface{}{"error": "short-answer prompt generation returned empty prompt"}
	}

	// Resolve lineage from cited chunks.
	sourceHeading, sourcePageStart, sourcePageEnd := s.resolveSocraticLineage(topicID, chunkIDs)

	question := models.WrittenQuestion{
		ID:              uuid.NewString(),
		TopicID:         topicID,
		Prompt:          questionPrompt,
		SourceHeading:   sourceHeading,
		SourcePageStart: sourcePageStart,
		SourcePageEnd:   sourcePageEnd,
		LLMModel:        providerModelName(s.fastLLMProvider),
		PromptVersion:   "written-v1-persisted",
	}
	if err := s.repo.CreateWrittenQuestion(question); err != nil {
		return map[string]interface{}{"error": "failed to persist short-answer prompt: " + err.Error()}
	}
	return map[string]interface{}{
		"questionID":        question.ID,
		"prompt":            question.Prompt,
		"topicID":           topicID,
		"source_heading":    question.SourceHeading,
		"source_page_start": question.SourcePageStart,
		"source_page_end":   question.SourcePageEnd,
	}
}

// resolveSocraticLineage resolves the heading / page range from chunk IDs.
func (s *StudyService) resolveSocraticLineage(topicID string, chunkIDs []string) (string, int, int) {
	if len(chunkIDs) == 0 {
		return "", 0, 0
	}
	headingPageRanges, err := s.repo.GetTopicHeadingPageRanges(topicID)
	if err != nil {
		utils.Warnf("could not resolve socratic lineage for topic %s: %v", topicID, err)
		return "", 0, 0
	}
	sourcePageStart, sourcePageEnd := 0, 0
	for _, cid := range chunkIDs {
		pageRange, ok := headingPageRanges[cid]
		if !ok {
			continue
		}
		if sourcePageStart == 0 || pageRange[0] < sourcePageStart {
			sourcePageStart = pageRange[0]
		}
		if pageRange[1] > sourcePageEnd {
			sourcePageEnd = pageRange[1]
		}
	}
	sourceHeading := ""
	if sourcePageStart > 0 {
		sourceHeading = fmt.Sprintf("Page %d", sourcePageStart)
	}
	return sourceHeading, sourcePageStart, sourcePageEnd
}

func (s *StudyService) AskSocratic(notebookID string, topicID string, question string, conversationHistory []map[string]string) (map[string]interface{}, error) {
	notebookID = strings.TrimSpace(notebookID)
	topicID = strings.TrimSpace(topicID)
	question = strings.TrimSpace(question)
	if notebookID == "" {
		return nil, retrieval.ErrInvalidNotebookContext
	}
	if question == "" {
		return nil, fmt.Errorf("question is required")
	}
	if s.fastLLMProvider == nil {
		return nil, fmt.Errorf("FAST_LLM provider not initialized")
	}
	if s.retrievalEngine == nil {
		return nil, fmt.Errorf("retrieval engine not initialized")
	}

	// 1. Semantic search for relevant chunks inside the notebook scope
	const topK = 5
	results, err := s.retrievalEngine.SemanticSearchNotebook(notebookID, topicID, question, topK)
	if err != nil {
		return nil, fmt.Errorf("retrieval failed: %w", err)
	}

	// 2. Build retrieved material context blocks, citations, and chunk texts
	blocks, citations, chunkTexts := buildReaderContextBlocksWithText(results)

	// 3. Generate answer using heavy LLM provider (to ensure high quality guiding responses)
	llm := s.heavyLLMProvider
	if llm == nil {
		llm = s.fastLLMProvider
	}

	// Enforce token budget — compute available input tokens and truncate
	// the retrieved blocks to fit while preserving Socratic instructions
	// and the student question.
	limits := llm.GetLimits()

	// Build conversation history block for the prompt
	historyBlock := ""
	if len(conversationHistory) > 0 {
		var histBuilder strings.Builder
		histBuilder.WriteString("Previous conversation:\n")
		for _, msg := range conversationHistory {
			role := "Student"
			if msg["role"] == "assistant" {
				role = "Tutor"
			}
			fmt.Fprintf(&histBuilder, "%s: %s\n", role, msg["content"])
		}
		historyBlock = histBuilder.String()
	}

	// Compute tokens for prompt overhead (instructions + history + student question + fixed labels)
	overheadText := strings.Join([]string{
		"You are an adaptive Socratic tutor helping a student understand material from the retrieved content.",
		"Act like a human tutor talking to a confused student.",
		"Prefer concrete examples over abstract analysis.",
		"Start from the student's likely confusion.",
		"",
		"Goal:",
		"Help the student discover the answer through guided thinking, not answer substitution.",
		"",
		"Rules:",
		"- Stay within the retrieved material.",
		"- The student cannot see the retrieved material. Do NOT refer to \"retrieved material\", \"provided text\", \"context\", \"document\", or \"source\". Talk to the student naturally as if you both know the subject matter.",
		"- First identify what the student is being asked to do (theme identification, concept understanding, comparison, argument analysis, application, etc.).",
		"- Stay at the same level of abstraction as the question.",
		"- Guide using questions and hints before explanations.",
		"- Build on the student's current understanding.",
		"- Help the student notice evidence, patterns, contrasts, causes, and assumptions.",
		"- Do not create study plans, teaching plans, summaries, or new tasks unless requested.",
		"- Do not provide the final answer unless asked or the student is clearly stuck.",
		"- Keep responses concise and focused.",
		"- Continue the conversation naturally. Reference what the student said before.",
		"",
		"Hint Progression:",
		"Observation → Pattern → Concept → Near Answer → Full Explanation",
		"",
		"Response Format Guidelines:",
		"- Respond in a natural, conversational manner.",
		"- Directly respond to the student's input: validate if they are correct, partially correct, or incorrect, and explain why briefly using the retrieved material. If they ask a question, answer it directly and clearly.",
		"- End your response with exactly one short probing question to guide them further. If helpful, you may add a hint below the question labeled 'Hint:'.",
		"",
		historyBlock,
		"Student question: " + question,
	}, "\n")

	overheadTokens := embeddings.CountTokensFallback(overheadText)
	// Reserve a small safety margin for formatting and LLM internals
	reserved := 100
	available := limits.MaxInputTokens - overheadTokens - reserved
	if available < 0 {
		available = 0
	}

	// Include as many blocks as will fit into available tokens, truncating
	// the final block if necessary. Keep citations aligned to included blocks.
	newBlocks := make([]string, 0, len(blocks))
	newCitations := make([]string, 0, len(citations))
	newChunkTexts := make([]string, 0, len(chunkTexts))
	usedTokens := 0
	for i, blk := range blocks {
		blkTokens := embeddings.CountTokensFallback(blk)
		if usedTokens+blkTokens <= available {
			newBlocks = append(newBlocks, blk)
			newCitations = append(newCitations, citations[i])
			newChunkTexts = append(newChunkTexts, chunkTexts[i])
			usedTokens += blkTokens
			continue
		}
		remaining := available - usedTokens
		if remaining > 8 {
			// Try tokenizer-based truncation for the final chunk
			if truncated, err := embeddings.TruncateToTokens(blk, remaining); err == nil && strings.TrimSpace(truncated) != "" {
				newBlocks = append(newBlocks, truncated)
				newCitations = append(newCitations, citations[i])
				newChunkTexts = append(newChunkTexts, chunkTexts[i])
			}
		}
		break
	}

	// If everything was truncated away, only fall back within remaining budget.
	if len(newBlocks) == 0 && len(blocks) > 0 {
		safeLimit := available
		if safeLimit > 128 {
			safeLimit = 128
		}
		if safeLimit > 0 {
			if truncated, err := embeddings.TruncateToTokens(blocks[0], safeLimit); err == nil && strings.TrimSpace(truncated) != "" {
				newBlocks = append(newBlocks, truncated)
				newCitations = append(newCitations, citations[0])
				newChunkTexts = append(newChunkTexts, chunkTexts[0])
			}
		}
	}

	contextText := strings.TrimSpace(strings.Join(newBlocks, "\n\n"))
	citations = newCitations
	chunkTexts = newChunkTexts

	// Rebuild the final prompt now that contextText may have been truncated
	socraticPrompt := strings.Join([]string{
		"You are an adaptive Socratic tutor helping a student understand material from the retrieved content.",
		"Act like a human tutor talking to a confused student.",
		"Prefer concrete examples over abstract analysis.",
		"Start from the student's likely confusion.",
		"",
		"Goal:",
		"Help the student discover the answer through guided thinking, not answer substitution.",
		"",
		"Rules:",
		"- Stay within the retrieved material.",
		"- The student cannot see the retrieved material. Do NOT refer to \"retrieved material\", \"provided text\", \"context\", \"document\", or \"source\". Talk to the student naturally as if you both know the subject matter.",
		"- First identify what the student is being asked to do (theme identification, concept understanding, comparison, argument analysis, application, etc.).",
		"- Stay at the same level of abstraction as the question.",
		"- Guide using questions and hints before explanations.",
		"- Build on the student's current understanding.",
		"- Help the student notice evidence, patterns, contrasts, causes, and assumptions.",
		"- Do not create study plans, teaching plans, summaries, or new tasks unless requested.",
		"- Do not provide the final answer unless asked or the student is clearly stuck.",
		"- Keep responses concise and focused.",
		"- Continue the conversation naturally. Reference what the student said before.",
		"",
		"Hint Progression:",
		"Observation → Pattern → Concept → Near Answer → Full Explanation",
		"",
		"Response Format Guidelines:",
		"- Respond in a natural, conversational manner.",
		"- Directly respond to the student's input: validate if they are correct, partially correct, or incorrect, and explain why briefly using the retrieved material. If they ask a question, answer it directly and clearly.",
		"- End your response with exactly one short probing question to guide them further. If helpful, you may add a hint below the question labeled 'Hint:'.",
		"",
		historyBlock,
		"Retrieved material:",
		contextText,
		"",
		"Student question: " + question,
	}, "\n")

	answer, err := llm.GenerateAnswer(socraticPrompt)
	if err != nil {
		return nil, fmt.Errorf("socratic response generation failed: %w", err)
	}

	return map[string]interface{}{
		"answer":         answer,
		"cited_sections": citations,
		"chunk_texts":    chunkTexts,
	}, nil
}

// Ensure retrieval import is used (the Engine type lives there).
var _ *retrieval.Engine
