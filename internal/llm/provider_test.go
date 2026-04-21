package llm

import "testing"

func TestLoadConfigFromEnvForPrefixUsesPrefixedValues(t *testing.T) {
	t.Setenv("FAST_LLM_BASE_URL", "https://fast.example.com")
	t.Setenv("FAST_LLM_API_KEY", "fast-key")
	t.Setenv("FAST_LLM_MODEL", "fast-model")
	t.Setenv("FAST_LLM_TIMEOUT_MS", "1234")

	config := LoadConfigFromEnvForPrefix("FAST_LLM")

	if config.BaseURL != "https://fast.example.com" {
		t.Fatalf("unexpected BaseURL: %s", config.BaseURL)
	}
	if config.APIKey != "fast-key" {
		t.Fatalf("unexpected APIKey: %s", config.APIKey)
	}
	if config.Model != "fast-model" {
		t.Fatalf("unexpected Model: %s", config.Model)
	}
	if config.TimeoutMs != 1234 {
		t.Fatalf("unexpected TimeoutMs: %d", config.TimeoutMs)
	}
}

func TestLoadConfigFromEnvForPrefixFallsBackToLegacyVars(t *testing.T) {
	t.Setenv("LLM_BASE_URL", "https://legacy.example.com")
	t.Setenv("LLM_API_KEY", "legacy-key")
	t.Setenv("LLM_MODEL", "legacy-model")
	t.Setenv("LLM_TIMEOUT_MS", "2345")

	config := LoadConfigFromEnvForPrefix("HEAVY_LLM")

	if config.BaseURL != "https://legacy.example.com" {
		t.Fatalf("unexpected BaseURL: %s", config.BaseURL)
	}
	if config.APIKey != "legacy-key" {
		t.Fatalf("unexpected APIKey: %s", config.APIKey)
	}
	if config.Model != "legacy-model" {
		t.Fatalf("unexpected Model: %s", config.Model)
	}
	if config.TimeoutMs != 2345 {
		t.Fatalf("unexpected TimeoutMs: %d", config.TimeoutMs)
	}
}
