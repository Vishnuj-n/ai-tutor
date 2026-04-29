package study

// socratic.go — ONLY file that imports internal/retrieval.
// The GenerateShortAnswerPrompt method is the sole consumer of the
// SemanticSearch engine. All other study flows use page-bounded SQL injection.

import (
	"fmt"
	"strings"

	"ai-tutor/internal/db"
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

// Ensure retrieval import is used (the Engine type lives there).
var _ *retrieval.Engine
