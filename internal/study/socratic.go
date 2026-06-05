package study

// socratic.go — ONLY file that imports internal/retrieval.
// The GenerateShortAnswerPrompt method is the sole consumer of the
// SemanticSearch engine. All other study flows use page-bounded SQL injection.

import (
	"fmt"
	"strings"

	"ai-tutor/internal/db"
	"ai-tutor/internal/embeddings"
	llmpkg "ai-tutor/internal/llm"
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
	parentIDs := make([]string, 0, len(results))
	for _, r := range results {
		contextBuilder.WriteString(r.Text)
		contextBuilder.WriteByte('\n')
		if r.ParentID != "" {
			parentIDs = append(parentIDs, r.ParentID)
		}
	}
	contextText := strings.TrimSpace(contextBuilder.String())

	prompt := fmt.Sprintf(`You are an AI tutor generating a short-answer assessment question.
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

	// Resolve lineage from cited chunk parents.
	sourceHeading, sourcePageStart, sourcePageEnd := resolveSocraticLineage(topicID, parentIDs)

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
	if err := db.CreateWrittenQuestion(question); err != nil {
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

// resolveSocraticLineage resolves the heading / page range from parent section IDs.
func resolveSocraticLineage(topicID string, parentIDs []string) (string, int, int) {
	if len(parentIDs) == 0 {
		return "", 0, 0
	}
	headingPageRanges, err := db.GetTopicHeadingPageRanges(topicID)
	if err != nil {
		utils.Warnf("could not resolve socratic lineage for topic %s: %v", topicID, err)
		return "", 0, 0
	}
	sourceHeading, sourcePageStart, sourcePageEnd := "", 0, 0
	maxSpan := 0
	// Pick the heading with the widest page span to ensure sourceHeading covers the computed range
	for _, pid := range parentIDs {
		pageRange, ok := headingPageRanges[pid]
		if !ok {
			continue
		}
		span := pageRange[1] - pageRange[0]
		if span > maxSpan {
			maxSpan = span
			sourceHeading = pid
		}
		if sourcePageStart == 0 || pageRange[0] < sourcePageStart {
			sourcePageStart = pageRange[0]
		}
		if pageRange[1] > sourcePageEnd {
			sourcePageEnd = pageRange[1]
		}
	}
	return sourceHeading, sourcePageStart, sourcePageEnd
}

// AskSocratic processes a conversational query in Socratic Tutor mode, completely statelessly.
func (s *StudyService) AskSocratic(topicID string, question string) (map[string]interface{}, error) {
	topicID = strings.TrimSpace(topicID)
	question = strings.TrimSpace(question)
	if topicID == "" {
		return nil, fmt.Errorf("topic ID is required")
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

	// 1. Semantic search for relevant chunks
	const topK = 5
	results, err := s.retrievalEngine.SemanticSearch(topicID, question, topK, 0, 0)
	if err != nil {
		return nil, fmt.Errorf("retrieval failed: %w", err)
	}

	// 2. Build retrieved material context blocks and citations
	blocks, citations := buildReaderContextBlocks(results)
	// Join blocks to form initial context text
	contextText := strings.TrimSpace(strings.Join(blocks, "\n\n"))

	// 3. Prepend Socratic instructions (moved from frontend)
	socraticPrompt := strings.Join([]string{
		"You are a Socratic tutor.",
		"- Begin with a short, probing question that helps the student analyze the topic.",
		"- Follow with a concise hint that is grounded only in the selected material and retrieval scope.",
		"- Do not provide the final answer unless the student explicitly requests it.",
		"- Keep responses clear, calm, and focused on guiding thinking rather than giving solutions.",
		"",
		"Retrieved material:",
		contextText,
		"",
		"Student question: " + question,
		"",
		"Response:",
	}, "\n")

	// 4. Generate answer using heavy LLM provider (to ensure high quality guiding responses)
	llm := s.heavyLLMProvider
	if llm == nil {
		llm = s.fastLLMProvider
	}

	// Enforce token budget when possible. If the underlying provider exposes
	// model limits, compute available input tokens and truncate the retrieved
	// blocks to fit while preserving Socratic instructions and the student
	// question. Use tokenizer fallback when accurate counts are unavailable.
	if limiter, ok := llm.(interface{ GetLimits() llmpkg.ModelLimits }); ok {
		limits := limiter.GetLimits()

		// Compute tokens for prompt overhead (instructions + student question + fixed labels)
		overheadText := strings.Join([]string{
			"You are a Socratic tutor.",
			"- Begin with a short, probing question that helps the student analyze the topic.",
			"- Follow with a concise hint that is grounded only in the selected material and retrieval scope.",
			"- Do not provide the final answer unless the student explicitly requests it.",
			"- Keep responses clear, calm, and focused on guiding thinking rather than giving solutions.",
			"",
			"Student question: " + question,
			"",
			"Response:",
		}, "\n")

		overheadTokens := embeddings.CountTokensFallback(overheadText)
		// Reserve a small safety margin for formatting and LLM internals
		reserved := 50
		available := limits.MaxInputTokens - overheadTokens - reserved
		if available < 0 {
			available = 0
		}

		// Include as many blocks as will fit into available tokens, truncating
		// the final block if necessary. Keep citations aligned to included blocks.
		newBlocks := make([]string, 0, len(blocks))
		newCitations := make([]string, 0, len(citations))
		usedTokens := 0
		for i, blk := range blocks {
			blkTokens := embeddings.CountTokensFallback(blk)
			if usedTokens+blkTokens <= available {
				newBlocks = append(newBlocks, blk)
				newCitations = append(newCitations, citations[i])
				usedTokens += blkTokens
				continue
			}
			remaining := available - usedTokens
			if remaining > 8 {
				// Try tokenizer-based truncation for the final chunk
				if truncated, err := embeddings.TruncateToTokens(blk, remaining); err == nil && strings.TrimSpace(truncated) != "" {
					newBlocks = append(newBlocks, truncated)
					newCitations = append(newCitations, citations[i])
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
				}
			}
		}

		contextText = strings.TrimSpace(strings.Join(newBlocks, "\n\n"))
		citations = newCitations
	}

	// Rebuild the final prompt now that contextText may have been truncated
	socraticPrompt = strings.Join([]string{
		"You are a Socratic tutor.",
		"- Begin with a short, probing question that helps the student analyze the topic.",
		"- Follow with a concise hint that is grounded only in the selected material and retrieval scope.",
		"- Do not provide the final answer unless the student explicitly requests it.",
		"- Keep responses clear, calm, and focused on guiding thinking rather than giving solutions.",
		"",
		"Retrieved material:",
		contextText,
		"",
		"Student question: " + question,
		"",
		"Response:",
	}, "\n")

	answer, err := llm.GenerateAnswer(socraticPrompt)
	if err != nil {
		return nil, fmt.Errorf("Socratic response generation failed: %w", err)
	}

	return map[string]interface{}{
		"answer":         answer,
		"cited_sections": citations,
	}, nil
}

// Ensure retrieval import is used (the Engine type lives there).
var _ *retrieval.Engine
