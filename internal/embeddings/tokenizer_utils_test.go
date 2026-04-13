package embeddings

import (
	"strings"
	"testing"
)

func TestCountTokensUsesTokenizer(t *testing.T) {
	if err := InitPromptTokenizer(tokenizerAssetPath(t)); err != nil {
		t.Fatalf("failed to init prompt tokenizer: %v", err)
	}

	if got := CountTokens("hello tokenizer"); got <= 0 {
		t.Fatalf("expected positive token count, got=%d", got)
	}
}

func TestTruncateToTokensPreservesSentenceBoundary(t *testing.T) {
	if err := InitPromptTokenizer(tokenizerAssetPath(t)); err != nil {
		t.Fatalf("failed to init prompt tokenizer: %v", err)
	}

	text := "First sentence. Second sentence. Third sentence."
	limit := CountTokens("First sentence. Second sentence.")
	trimmed := TruncateToTokens(text, limit)

	if strings.TrimSpace(strings.ToLower(trimmed)) != "first sentence. second sentence." {
		t.Fatalf("unexpected trimmed text: %q", trimmed)
	}

	if got := CountTokens(trimmed); got > limit {
		t.Fatalf("trimmed text exceeded token limit: got=%d limit=%d", got, limit)
	}
}
