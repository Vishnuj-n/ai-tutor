package llm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
)

// Config holds LLM provider configuration.
type Config struct {
	BaseURL   string
	APIKey    string
	Model     string
	TimeoutMs int
}

// LoadConfigFromEnv loads provider config from environment variables.
// Supports multiple key aliases so different providers can share one config path.
func LoadConfigFromEnv() *Config {
	config := &Config{
		BaseURL: firstEnv(
			"LLM_BASE_URL",
			"OPENAI_ENDPOINT",
			"OPENAI_BASE_URL",
			"BASE_URL",
		),
		APIKey: firstEnv(
			"LLM_API_KEY",
			"OPENAI_API_KEY",
			"API_KEY",
		),
		Model: firstEnv(
			"LLM_MODEL",
			"BASE_MODEL",
			"OPENAI_MODEL",
			"MODEL",
		),
		TimeoutMs: firstEnvInt(30000,
			"LLM_TIMEOUT_MS",
			"OPENAI_TIMEOUT_MS",
			"TIMEOUT_MS",
		),
	}

	if config.BaseURL == "" {
		config.BaseURL = "http://localhost:8000"
	}
	if config.Model == "" {
		config.Model = "openai/gpt-oss-120b"
	}
	if config.APIKey == "" {
		config.APIKey = "sk-test"
	}

	return config
}

func firstEnv(keys ...string) string {
	for _, key := range keys {
		value := strings.TrimSpace(os.Getenv(key))
		if value != "" {
			return value
		}
	}
	return ""
}

func firstEnvInt(defaultValue int, keys ...string) int {
	for _, key := range keys {
		raw := strings.TrimSpace(os.Getenv(key))
		if raw == "" {
			continue
		}
		value, err := strconv.Atoi(raw)
		if err == nil {
			return value
		}
	}
	return defaultValue
}

// Provider handles communication with OpenAI-compatible APIs.
type Provider struct {
	config *Config
}

// NewProvider creates a new LLM provider.
func NewProvider(config *Config) *Provider {
	return &Provider{config: config}
}

// openAIRequest follows the OpenAI API format.
type openAIRequest struct {
	Model    string          `json:"model"`
	Messages []openAIMessage `json:"messages"`
}

// openAIMessage represents a message in the OpenAI API.
type openAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// openAIResponse follows the OpenAI API response format.
type openAIResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

// GenerateAnswer calls the LLM to generate an answer.
func (p *Provider) GenerateAnswer(prompt string) (string, error) {
	if p.config == nil || p.config.BaseURL == "" {
		return "", fmt.Errorf("LLM config not configured")
	}

	requestBody := openAIRequest{
		Model: p.config.Model,
		Messages: []openAIMessage{
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

	url := strings.TrimSuffix(p.config.BaseURL, "/") + "/v1/chat/completions"
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(body))
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.config.APIKey)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("LLM API error (status %d): %s", resp.StatusCode, string(bodyBytes))
	}

	var apiResp openAIResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return "", err
	}

	if len(apiResp.Choices) == 0 {
		return "", fmt.Errorf("no response from LLM")
	}

	return apiResp.Choices[0].Message.Content, nil
}
