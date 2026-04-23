package rag

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"strings"
	"unicode"

	"ai-tutor/internal/db"
	"ai-tutor/internal/embeddings"
	"ai-tutor/internal/llm"
)

const (
	maxPromptTokens     = 3000
	minUsedSectionRatio = 0.5
)

// Pipeline orchestrates retrieval and generation
type Pipeline struct {
	embedStore *EmbeddingStore
	llm        *llm.Provider
}

// NewPipeline creates a new RAG pipeline
func NewPipeline(embedStore *EmbeddingStore, llmProvider *llm.Provider) *Pipeline {
	return &Pipeline{
		embedStore: embedStore,
		llm:        llmProvider,
	}
}

// Response contains the final response with citations
type Response struct {
	Answer              string
	CitedSections       []string
	TopicID             string
	ChunksRetrieved     int
	SectionsUsed        int
	SampleRetrievalText string
}

// ProcessQuery runs the full RAG pipeline
func (p *Pipeline) ProcessQuery(topicID, userQuestion string, startPage, endPage int) (*Response, error) {
	// Step 1: Validate topic exists
	content, err := db.GetTopicContent(topicID)
	if err != nil {
		return nil, fmt.Errorf("topic not found: %w", err)
	}

	// Step 2: Retrieve chunks
	chunks, err := db.GetChunksForTopic(topicID)
	if err != nil {
		return nil, fmt.Errorf("could not retrieve chunks: %w", err)
	}

	if len(chunks) == 0 {
		return nil, fmt.Errorf("no content available for this topic")
	}

	// Step 3: Search for relevant chunks (top-5)
	topK := 5
	if len(chunks) < topK {
		topK = len(chunks)
	}

	// Validate page bounds - treat non-positive bounds as "no filter"
	searchStartPage := startPage
	searchEndPage := endPage
	if searchStartPage <= 0 || searchEndPage <= 0 {
		searchStartPage = 0
		searchEndPage = 0
	}

	results := p.embedStore.SearchTopK(userQuestion, chunks, topK, searchStartPage, searchEndPage)

	if len(results) == 0 {
		return nil, fmt.Errorf("no relevant content found for your question")
	}

	// Step 4: Apply heuristic scoring (V1 no-op, V2 weak-area boost)
	results = ApplyHeuristicScoring(results)

	// Step 5: Build context by expanding to parent sections
	ctx, err := BuildContext(results, topicID)
	if err != nil {
		return nil, fmt.Errorf("could not build context: %w", err)
	}

	// Step 6: Assemble prompt
	topicTitle, _ := content["title"].(string)
	if topicTitle == "" {
		topicTitle = "Topic"
	}

	prompt, promptParentIDs, err := buildPrompt(
		topicTitle,
		userQuestion,
		*ctx,
	)
	if err != nil {
		return nil, fmt.Errorf("could not assemble prompt: %w", err)
	}

	tokens, err := countPromptTokens(prompt)
	if err != nil {
		return nil, fmt.Errorf("could not count prompt tokens: %w", err)
	}
	log.Printf("RAG prompt prepared topic_id=%s tokens=%d parent_sections=%d id=%s", topicID, tokens, len(promptParentIDs), shortPromptID(prompt))

	// Step 7: Call LLM
	answer, err := p.llm.GenerateAnswer(prompt)
	if err != nil {
		return nil, fmt.Errorf("LLM error: %w", err)
	}

	// Step 8: Build response with citations in deterministic parent order.
	citedHeadings := make([]string, 0, len(promptParentIDs))
	for _, parentID := range promptParentIDs {
		section, ok := ctx.Sections[parentID]
		if !ok {
			continue
		}

		lines := strings.Split(section, "\n")
		if len(lines) == 0 {
			continue
		}

		heading := strings.TrimPrefix(lines[0], "**")
		heading = strings.TrimSuffix(heading, "**")
		citedHeadings = append(citedHeadings, heading)
	}

	result := &Response{
		Answer:          answer,
		CitedSections:   citedHeadings,
		TopicID:         topicID,
		ChunksRetrieved: ctx.ChunkHits,
		SectionsUsed:    len(promptParentIDs),
	}

	if len(results) > 0 {
		result.SampleRetrievalText = results[0].Text
	}

	return result, nil
}

func buildPrompt(topicTitle, userQuestion string, ctx RetrievalContext) (string, []string, error) {
	sectionText := ""
	usedParentIDs := make([]string, 0, len(ctx.ParentIDs))
	for _, parentID := range ctx.ParentIDs {
		section, ok := ctx.Sections[parentID]
		if !ok {
			continue
		}

		candidate := section + "\n\n"
		candidatePrompt := formatPrompt(topicTitle, sectionText+candidate, userQuestion)
		candidateTokens, err := countPromptTokens(candidatePrompt)
		if err != nil {
			return "", nil, err
		}
		if candidateTokens <= maxPromptTokens {
			sectionText += candidate
			usedParentIDs = append(usedParentIDs, parentID)
			continue
		}

		trimmed, err := trimToTokenBudget(topicTitle, userQuestion, sectionText, candidate, maxPromptTokens)
		if err != nil {
			return "", nil, err
		}
		if trimmed == "" {
			currentTokens, err := countPromptTokens(formatPrompt(topicTitle, sectionText, userQuestion))
			if err != nil {
				return "", nil, err
			}
			if maxPromptTokens-currentTokens <= 0 {
				break
			}
			continue
		}

		originalTokens, err := countPromptTokens(candidate)
		if err != nil {
			return "", nil, err
		}
		trimmedTokens, err := countPromptTokens(trimmed)
		if err != nil {
			return "", nil, err
		}

		if trimmedTokens == originalTokens || (originalTokens > 0 && float64(trimmedTokens)/float64(originalTokens) >= minUsedSectionRatio) {
			sectionText += strings.TrimRight(trimmed, "\n") + "\n\n"
			usedParentIDs = append(usedParentIDs, parentID)
		}

		currentTokens, err := countPromptTokens(formatPrompt(topicTitle, sectionText, userQuestion))
		if err != nil {
			return "", nil, err
		}
		if currentTokens >= maxPromptTokens {
			break
		}
	}

	prompt := formatPrompt(topicTitle, sectionText, userQuestion)

	return prompt, usedParentIDs, nil
}

func formatPrompt(topicTitle, sectionText, userQuestion string) string {
	return fmt.Sprintf(`You are an expert AI tutor. Your role is to teach clearly, not comprehensively.

Topic: %s

=== GROUNDING RULES (CRITICAL) ===
- Use ONLY the retrieved course material below.
- If asked about concepts not in the material, reply exactly: "I don't have enough information in the provided material to answer that confidently."
- Do NOT use outside facts, examples, or knowledge.
- Do NOT apologize for limitations; reframe using the material provided.

=== ANSWER FORMAT ===
Structure your response as:
1. **Direct Answer** (1–2 sentences, directly addressing the question)
2. **Key Concepts** (bullet list of fundamental ideas)
3. **Application or Example** (if material includes concrete examples, cite them)
4. **Why This Matters** (1 sentence on relevance)

If the question involves multiple steps or concepts, number them clearly.

=== CONTENT REQUIREMENTS ===
- Keep explanations concise and instructional.
- Prioritize clarity over exhaustiveness.
- Quote source material when defining terms.
- Avoid rambling or over-explaining.

Retrieved course material:
%s

Student's question: %s

Answer:`, topicTitle, sectionText, userQuestion)
}

func trimToTokenBudget(topicTitle, userQuestion, existingSections, candidate string, tokenLimit int) (string, error) {
	if tokenLimit <= 0 || candidate == "" {
		return "", nil
	}

	basePrompt := formatPrompt(topicTitle, existingSections, userQuestion)
	baseTokens, err := countPromptTokens(basePrompt)
	if err != nil {
		return "", err
	}
	remaining := tokenLimit - baseTokens
	if remaining <= 0 {
		return "", nil
	}

	trimmed, err := embeddings.TruncateToTokens(candidate, remaining)
	if err != nil {
		return "", err
	}
	if trimmed == "" {
		return "", nil
	}

	for {
		trialTokens, err := countPromptTokens(formatPrompt(topicTitle, existingSections+trimmed, userQuestion))
		if err != nil {
			return "", err
		}
		if trialTokens <= tokenLimit {
			break
		}

		next := trimBySentenceOrWhitespace(trimmed)
		if next == "" {
			return "", nil
		}
		trimmed = next
	}

	if !endsWithSentenceBoundary(trimmed) {
		next := dropLastSentence(trimmed)
		if next == "" {
			next = trimByWhitespace(trimmed)
		}
		if next != "" {
			trimmed = next
		}
	}

	return strings.TrimSpace(trimmed), nil
}

func countPromptTokens(prompt string) (int, error) {
	return embeddings.CountTokens(prompt)
}

func shortPromptID(prompt string) string {
	sum := sha256.Sum256([]byte(prompt))
	return hex.EncodeToString(sum[:8])
}

func dropLastSentence(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}

	searchStart := len(text) - 1
	if endsWithSentenceBoundary(text) {
		searchStart = len(text) - 2
	}

	for i := searchStart; i >= 0; i-- {
		switch text[i] {
		case '.', '!', '?':
			return strings.TrimSpace(text[:i+1])
		}
	}

	return ""
}

func trimByWhitespace(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}

	lastSpace := -1
	for i, r := range text {
		if unicode.IsSpace(r) {
			lastSpace = i
		}
	}
	if lastSpace <= 0 {
		return ""
	}

	trimmed := strings.TrimSpace(text[:lastSpace])
	if trimmed == text {
		return ""
	}

	return trimmed
}

func trimBySentenceOrWhitespace(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}

	if endsWithSentenceBoundary(text) {
		if sentenceTrim := dropLastSentence(text); sentenceTrim != "" && sentenceTrim != text {
			return sentenceTrim
		}
	}

	if wsTrim := trimByWhitespace(text); wsTrim != "" && wsTrim != text {
		return wsTrim
	}

	if sentenceTrim := dropLastSentence(text); sentenceTrim != "" && sentenceTrim != text {
		return sentenceTrim
	}

	return ""
}

func endsWithSentenceBoundary(text string) bool {
	text = strings.TrimSpace(text)
	if text == "" {
		return false
	}

	last := text[len(text)-1]
	return last == '.' || last == '!' || last == '?'
}
