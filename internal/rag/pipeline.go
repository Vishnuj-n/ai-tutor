package rag

import (
	"fmt"
	"strings"

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
	prompt := buildPrompt(
		content["title"].(string),
		userQuestion,
		ctx,
	)

	// Step 7: Call LLM
	answer, err := p.llm.GenerateAnswer(prompt)
	if err != nil {
		return nil, fmt.Errorf("LLM error: %w", err)
	}

	// Step 8: Build response with citations
	citedHeadings := []string{}
	for _, section := range ctx.Sections {
		lines := strings.Split(section, "\n")
		if len(lines) > 0 {
			heading := strings.TrimPrefix(lines[0], "**")
			heading = strings.TrimSuffix(heading, "**")
			citedHeadings = append(citedHeadings, heading)
		}
	}

	result := &Response{
		Answer:          answer,
		CitedSections:   citedHeadings,
		TopicID:         topicID,
		ChunksRetrieved: ctx.ChunkHits,
		SectionsUsed:    len(ctx.Sections),
	}

	if len(results) > 0 {
		result.SampleRetrievalText = results[0].Text
	}

	return result, nil
}

func buildPrompt(topicTitle, userQuestion string, ctx *RetrievalContext) string {
	sectionText := ""
	for _, section := range ctx.Sections {
		sectionText += section + "\n\n"
	}

	return fmt.Sprintf(`You are a helpful tutor assisting a student learn about: %s

Retrieved course material:
%s

Student's question: %s

Please provide a clear, concise answer based only on the material above.
If the material doesn't contain enough information to answer the question, say so.
Keep your response focused and educational.`, topicTitle, sectionText, userQuestion)
}
