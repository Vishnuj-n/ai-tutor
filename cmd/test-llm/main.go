package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"ai-tutor/internal/llm"

	"github.com/joho/godotenv"
)

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

// TestLLMConnection verifies that the configured API endpoint is reachable.
func main() {
	fmt.Println("=== AI Tutor LLM API Connection Test ===")
	fmt.Println()

	// Load .env from project root
	_ = godotenv.Load(".env")

	// Load config like the app does
	config := llm.LoadConfigFromEnv()

	fmt.Printf("Loaded LLM Configuration:\n")
	fmt.Printf("  Base URL: %s\n", config.BaseURL)
	fmt.Printf("  Model: %s\n", config.Model)
	fmt.Printf("  API Key: %s...\n", maskSecret(config.APIKey))
	fmt.Printf("  Timeout: %dms\n\n", config.TimeoutMs)

	// Build test request identical to GenerateAnswer
	testPrompt := "Respond with exactly: SUCCESS"
	requestBody := OpenAIRequest{
		Model: config.Model,
		Messages: []OpenAIMessage{
			{
				Role:    "user",
				Content: testPrompt,
			},
		},
	}

	body, err := json.Marshal(requestBody)
	if err != nil {
		fmt.Printf("❌ Error marshaling request: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Sending test request to API...")
	fmt.Printf("  Endpoint: %s/v1/chat/completions\n", config.BaseURL)
	fmt.Printf("  Prompt: '%s'\n\n", testPrompt)

	// Build HTTP request
	url := strings.TrimSuffix(config.BaseURL, "/") + "/v1/chat/completions"
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(body))
	if err != nil {
		fmt.Printf("❌ Error creating HTTP request: %v\n", err)
		os.Exit(1)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+config.APIKey)

	// Execute request with timeout
	client := &http.Client{
		Timeout: time.Duration(config.TimeoutMs) * time.Millisecond,
	}

	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("❌ Error calling API: %v\n", err)
		os.Exit(1)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			fmt.Printf("Warning: failed to close response body: %v\n", err)
		}
	}()

	// Check response status
	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		fmt.Printf("❌ API returned status %d\n", resp.StatusCode)
		fmt.Printf("   Response: %s\n", string(bodyBytes))
		os.Exit(1)
	}

	// Parse response
	var apiResp OpenAIResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		fmt.Printf("❌ Error parsing response: %v\n", err)
		os.Exit(1)
	}

	if len(apiResp.Choices) == 0 {
		fmt.Printf("❌ No choices in API response\n")
		os.Exit(1)
	}

	answer := apiResp.Choices[0].Message.Content

	// Success
	fmt.Printf("✅ Connection successful!\n")
	fmt.Printf("   API Response: %s\n\n", answer)
	fmt.Println("=== Test Passed ===")
}

func maskSecret(s string) string {
	if len(s) <= 4 {
		return "***"
	}
	return s[:4] + "..." + s[len(s)-4:]
}
