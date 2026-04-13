package embeddings

import (
	"strings"
	"testing"
)

func TestCountTokensUsesTokenizer(t *testing.T) {
	if err := InitPromptTokenizer(tokenizerAssetPath(t)); err != nil {
		t.Fatalf("failed to init prompt tokenizer: %v", err)
	}

	got, err := CountTokens("hello tokenizer")
	if err != nil {
		t.Fatalf("count tokens failed: %v", err)
	}
	if got <= 0 {
		t.Fatalf("expected positive token count, got=%d", got)
	}
}

func TestTruncateToTokensPreservesSentenceBoundary(t *testing.T) {
	if err := InitPromptTokenizer(tokenizerAssetPath(t)); err != nil {
		t.Fatalf("failed to init prompt tokenizer: %v", err)
	}

	text := "First sentence. Second sentence. Third sentence."
	limit, err := CountTokens("First sentence. Second sentence.")
	if err != nil {
		t.Fatalf("count tokens failed: %v", err)
	}
	trimmed, err := TruncateToTokens(text, limit)
	if err != nil {
		t.Fatalf("truncate failed: %v", err)
	}

	if strings.TrimSpace(strings.ToLower(trimmed)) != "first sentence. second sentence." {
		t.Fatalf("unexpected trimmed text: %q", trimmed)
	}

	got, err := CountTokens(trimmed)
	if err != nil {
		t.Fatalf("count tokens failed on trimmed text: %v", err)
	}
	if got > limit {
		t.Fatalf("trimmed text exceeded token limit: got=%d limit=%d", got, limit)
	}
}
