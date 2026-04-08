package main

import (
	"fmt"
	"strings"
)

// RAGPipeline orchestrates retrieval and generation
type RAGPipeline struct {
	embedStore *EmbeddingStore
	llm        *LLMProvider
}

// NewRAGPipeline creates a new RAG pipeline
func NewRAGPipeline(embedStore *EmbeddingStore, llm *LLMProvider) *RAGPipeline {
	return &RAGPipeline{
		embedStore: embedStore,
		llm:        llm,
	}
}

// RAGResponse contains the final response with citations
type RAGResponse struct {
	Answer              string
	CitedSections       []string
	TopicID             string
	ChunksRetrieved     int
	SectionsUsed        int
	SampleRetrievalText string
}

// ProcessQuery runs the full RAG pipeline
func (r *RAGPipeline) ProcessQuery(topicID, userQuestion string) (*RAGResponse, error) {
	// Step 1: Validate topic exists
	content, err := GetTopicContent(topicID)
	if err != nil {
		return nil, fmt.Errorf("topic not found: %w", err)
	}

	// Step 2: Retrieve chunks
	chunks, err := GetChunksForTopic(topicID)
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
	results := r.embedStore.SearchTopK(userQuestion, chunks, topK)

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
	prompt := BuildRAGPrompt(
		content["title"].(string),
		userQuestion,
		ctx,
	)

	// Step 7: Call LLM
	answer, err := r.llm.GenerateAnswer(prompt)
	if err != nil {
		return nil, fmt.Errorf("LLM error: %w", err)
	}

	// Step 8: Build response with citations
	citedHeadings := []string{}
	for _, section := range ctx.Sections {
		// Extract heading from the formatted section
		lines := strings.Split(section, "\n")
		if len(lines) > 0 {
			heading := strings.TrimPrefix(lines[0], "**")
			heading = strings.TrimSuffix(heading, "**")
			citedHeadings = append(citedHeadings, heading)
		}
	}

	result := &RAGResponse{
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

// ApplyHeuristicScoring is an explicit retrieval-stage hook for reranking.
// V1 behavior is pass-through to preserve existing ranking.
func ApplyHeuristicScoring(results []RetrievalResult) []RetrievalResult {
	return results
}
