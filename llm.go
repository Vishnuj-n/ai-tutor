package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// LLMConfig holds LLM provider configuration.
type LLMConfig struct {
	BaseURL   string
	APIKey    string
	Model     string
	TimeoutMs int
}

// LLMProvider handles communication with OpenAI-compatible APIs.
type LLMProvider struct {
	config *LLMConfig
}

// NewLLMProvider creates a new LLM provider.
func NewLLMProvider(config *LLMConfig) *LLMProvider {
	return &LLMProvider{config: config}
}

// OpenAIRequest follows the OpenAI API format.
type OpenAIRequest struct {
	Model    string          `json:"model"`
	Messages []OpenAIMessage `json:"messages"`
}

// OpenAIMessage represents a message in the OpenAI API.
type OpenAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// OpenAIResponse follows the OpenAI API response format.
type OpenAIResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

// GenerateAnswer calls the LLM to generate an answer.
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

// BuildRAGPrompt builds the final RAG prompt for the LLM.
func BuildRAGPrompt(topicTitle, userQuestion string, ctx *RetrievalContext) string {
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
