package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// LLMConfig holds LLM provider configuration
type LLMConfig struct {
	BaseURL   string
	APIKey    string
	Model     string
	TimeoutMs int
}

// LLMProvider handles communication with OpenAI-compatible APIs
type LLMProvider struct {
	config *LLMConfig
}

// NewLLMProvider creates a new LLM provider
func NewLLMProvider(config *LLMConfig) *LLMProvider {
	return &LLMProvider{config: config}
}

// OpenAIRequest follows the OpenAI API format
type OpenAIRequest struct {
	Model    string          `json:"model"`
	Messages []OpenAIMessage `json:"messages"`
}

// OpenAIMessage represents a message in the OpenAI API
type OpenAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// OpenAIResponse follows the OpenAI API response format
type OpenAIResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

// GenerateAnswer calls the LLM to generate an answer
func (l *LLMProvider) GenerateAnswer(prompt string) (string, error) {
	if l.config == nil || l.config.BaseURL == "" {
		return "", fmt.Errorf("LLM config not configured")
	}

	requestBody := OpenAIRequest{
		Model: l.config.Model,
		Messages: []OpenAIMessage{
			{
				Role:    "user",
				Content: prompt,
			},
		},
	}

	body, err := json.Marshal(requestBody)
	if err != nil {
		return "", err
	}

	url := strings.TrimSuffix(l.config.BaseURL, "/") + "/v1/chat/completions"
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(body))
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+l.config.APIKey)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("LLM API error (status %d): %s", resp.StatusCode, string(bodyBytes))
	}

	var apiResp OpenAIResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return "", err
	}

	if len(apiResp.Choices) == 0 {
		return "", fmt.Errorf("no response from LLM")
	}

	return apiResp.Choices[0].Message.Content, nil
}

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

	// Step 4: Build context by expanding to parent sections
	ctx, err := BuildContext(results, topicID)
	if err != nil {
		return nil, fmt.Errorf("could not build context: %w", err)
	}

	// Step 5: Assemble prompt
	prompt := assemblePrompt(
		content["title"].(string),
		userQuestion,
		ctx,
	)

	// Step 6: Call LLM
	answer, err := r.llm.GenerateAnswer(prompt)
	if err != nil {
		return nil, fmt.Errorf("LLM error: %w", err)
	}

	// Step 7: Build response with citations
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

// assemblePrompt builds the final prompt for the LLM
func assemblePrompt(topicTitle, userQuestion string, ctx *RetrievalContext) string {
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
