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
	"time"
)

const minTimeoutMs = 1

// ModelLimits defines token limits for specific models.
type ModelLimits struct {
	MaxInputTokens  int
	MaxOutputTokens int
}

// Config holds LLM provider configuration.
type Config struct {
	BaseURL   string
	APIKey    string
	Model     string
	TimeoutMs int
	Limits    ModelLimits
}

// LoadConfigFromEnv loads the legacy single-provider config from environment variables.
func LoadConfigFromEnv() *Config {
	return LoadConfigFromEnvForPrefix("")
}

// LoadConfigFromEnvForPrefix loads provider config for a named tier.
// Prefix examples: FAST_LLM
func LoadConfigFromEnvForPrefix(prefix string) *Config {
	prefix = strings.TrimSpace(prefix)
	if prefix != "" {
		prefix = strings.TrimSuffix(prefix, "_")
	}

	baseURLKeys := prefixedKeys(prefix, "LLM_BASE_URL", "OPENAI_ENDPOINT", "OPENAI_BASE_URL", "BASE_URL")
	apiKeyKeys := prefixedKeys(prefix, "LLM_API_KEY", "OPENAI_API_KEY", "API_KEY")
	modelKeys := prefixedKeys(prefix, "LLM_MODEL", "BASE_MODEL", "OPENAI_MODEL", "MODEL")
	timeoutKeys := prefixedKeys(prefix, "LLM_TIMEOUT_MS", "OPENAI_TIMEOUT_MS", "TIMEOUT_MS")

	config := &Config{
		BaseURL:   firstEnv(baseURLKeys...),
		APIKey:    firstEnv(apiKeyKeys...),
		Model:     firstEnv(modelKeys...),
		TimeoutMs: firstEnvInt(30000, timeoutKeys...),
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

	config.Limits = getModelLimits(config.Model)

	return config
}

// getModelLimits returns model-specific token limits with safety margins.
// Uses 60% of model's max context as safe input limit, reserves 15-20% margin.
func getModelLimits(model string) ModelLimits {
	model = strings.TrimSpace(strings.ToLower(model))

	switch {
	case strings.Contains(model, "llama-3.1-8b"):
		return ModelLimits{
			MaxInputTokens:  40000, // ~60% of 128k context
			MaxOutputTokens: 3000,
		}
	case strings.Contains(model, "llama-3.3-70b"):
		return ModelLimits{
			MaxInputTokens:  50000, // ~60% of 128k context
			MaxOutputTokens: 4000,
		}
	case strings.Contains(model, "gpt-oss-120b"):
		return ModelLimits{
			MaxInputTokens:  50000, // ~60% of 128k context
			MaxOutputTokens: 4000,
		}
	case strings.Contains(model, "gpt-oss-20b"):
		return ModelLimits{
			MaxInputTokens:  35000, // ~60% of 128k context
			MaxOutputTokens: 2500,
		}
	default:
		// Conservative defaults for unknown models
		return ModelLimits{
			MaxInputTokens:  30000,
			MaxOutputTokens: 3000,
		}
	}
}

func prefixedKeys(prefix string, keys ...string) []string {
	if prefix == "" {
		return keys
	}

	result := make([]string, 0, len(keys)*2)
	for _, key := range keys {
		result = append(result, prefix+"_"+key)
	}
	result = append(result, keys...)
	return result
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

// ModelName returns the configured model identifier sent to the provider API.
func (p *Provider) ModelName() string {
	if p == nil || p.config == nil {
		return ""
	}
	return strings.TrimSpace(p.config.Model)
}

// GetLimits returns the token limits for the configured model.
func (p *Provider) GetLimits() ModelLimits {
	if p == nil || p.config == nil {
		return ModelLimits{
			MaxInputTokens:  30000,
			MaxOutputTokens: 3000,
		}
	}
	return p.config.Limits
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

	effectiveTimeoutMs := firstEnvInt(30000, "LLM_TIMEOUT_MS", "OPENAI_TIMEOUT_MS", "TIMEOUT_MS")
	if p.config.TimeoutMs > 0 {
		effectiveTimeoutMs = p.config.TimeoutMs
	}
	if effectiveTimeoutMs <= 0 {
		effectiveTimeoutMs = minTimeoutMs
	}
	client := &http.Client{
		Timeout: time.Duration(effectiveTimeoutMs) * time.Millisecond,
	}
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
