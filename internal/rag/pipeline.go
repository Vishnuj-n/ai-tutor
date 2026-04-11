package rag

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"ai-tutor/internal/db"
	"ai-tutor/internal/llm"
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
func (p *Pipeline) ProcessQuery(topicID, userQuestion string) (*Response, error) {
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
	results := p.embedStore.SearchTopK(userQuestion, chunks, topK)

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

	prompt, promptParentIDs := buildPrompt(
		topicTitle,
		userQuestion,
		ctx,
	)

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

func buildPrompt(topicTitle, userQuestion string, ctx *RetrievalContext) (string, []string) {
	const maxContextRunes = 6000

	sectionText := ""
	usedParentIDs := make([]string, 0, len(ctx.ParentIDs))
	for _, parentID := range ctx.ParentIDs {
		section, ok := ctx.Sections[parentID]
		if !ok {
			continue
		}

		candidate := section + "\n\n"
		remaining := maxContextRunes - utf8.RuneCountInString(sectionText)
		if remaining <= 0 {
			break
		}

		candidateRunes := utf8.RuneCountInString(candidate)
		if candidateRunes <= remaining {
			sectionText += candidate
			usedParentIDs = append(usedParentIDs, parentID)
			continue
		}

		sectionText += trimToRunes(candidate, remaining)
		usedParentIDs = append(usedParentIDs, parentID)
		if utf8.RuneCountInString(sectionText) >= maxContextRunes {
			break
		}
	}

	prompt := fmt.Sprintf(`You are an AI tutor.

Topic: %s

Rules:
- Use only the retrieved course material below.
- If the material is insufficient, reply exactly: "I don't have enough information in the provided material to answer that confidently."
- Do not use outside facts.
- Keep the answer concise and instructional.

Retrieved course material:
%s

Student's question: %s

Answer:`, topicTitle, sectionText, userQuestion)

	return prompt, usedParentIDs
}

func trimToRunes(input string, maxRunes int) string {
	if maxRunes <= 0 {
		return ""
	}
	runes := []rune(input)
	if len(runes) <= maxRunes {
		return input
	}
	return string(runes[:maxRunes])
}
