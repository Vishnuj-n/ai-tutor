package embeddings

import (
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"unicode"

	"github.com/sugarme/tokenizer"
	"github.com/sugarme/tokenizer/pretrained"
)

var (
	promptTokenizerMu sync.RWMutex
	promptTokenizer   *tokenizer.Tokenizer
)

// InitPromptTokenizer initializes shared tokenizer used for prompt budgeting.
func InitPromptTokenizer(tokenizerPath string) error {
	tok, err := pretrained.FromFile(tokenizerPath)
	if err != nil {
		return err
	}

	promptTokenizerMu.Lock()
	promptTokenizer = tok
	promptTokenizerMu.Unlock()

	return nil
}

// CountTokens counts tokens using configured tokenizer.
func CountTokens(text string) (int, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return 0, nil
	}

	tok := getPromptTokenizer()
	if tok == nil {
		return 0, fmt.Errorf("prompt tokenizer not initialized")
	}

	enc, err := tok.EncodeSingle(text, true)
	if err != nil {
		return 0, fmt.Errorf("tokenizer encode failed in CountTokens: %w", err)
	}

	return len(enc.Ids), nil
}

// TruncateToTokens trims text to token limit, preferring clean sentence boundaries.
func TruncateToTokens(text string, limit int) (string, error) {
	text = strings.TrimSpace(text)
	if text == "" || limit <= 0 {
		return "", nil
	}

	tok := getPromptTokenizer()
	if tok == nil {
		return "", fmt.Errorf("prompt tokenizer not initialized")
	}

	enc, err := tok.EncodeSingle(text, true)
	if err != nil {
		return "", fmt.Errorf("tokenizer encode failed in TruncateToTokens: %w", err)
	}

	if len(enc.Ids) <= limit {
		return text, nil
	}

	decoded := tok.Decode(enc.Ids[:limit], true)

	return trimToSentenceBoundary(decoded), nil
}

func getPromptTokenizer() *tokenizer.Tokenizer {
	promptTokenizerMu.RLock()
	tok := promptTokenizer
	promptTokenizerMu.RUnlock()
	if tok != nil {
		return tok
	}

	for _, candidate := range tokenizerPathCandidates() {
		if _, err := os.Stat(candidate); err != nil {
			continue
		}
		if err := InitPromptTokenizer(candidate); err != nil {
			log.Printf("failed to initialize prompt tokenizer from %s: %v", candidate, err)
			continue
		}

		promptTokenizerMu.RLock()
		loaded := promptTokenizer
		promptTokenizerMu.RUnlock()
		return loaded
	}

	return nil
}

func tokenizerPathCandidates() []string {
	candidates := make([]string, 0, 3)
	if fromEnv := strings.TrimSpace(os.Getenv("TOKENIZER_PATH")); fromEnv != "" {
		candidates = append(candidates, fromEnv)
	}
	candidates = append(candidates,
		"asset/tokenizer.json",
		"../asset/tokenizer.json",
	)
	return candidates
}

func trimToSentenceBoundary(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}

	lastEnd := -1
	for i, r := range text {
		if r == '.' || r == '!' || r == '?' {
			lastEnd = i
		}
	}
	if lastEnd >= 0 {
		return strings.TrimSpace(text[:lastEnd+1])
	}

	lastSpace := -1
	for i, r := range text {
		if unicode.IsSpace(r) {
			lastSpace = i
		}
	}
	if lastSpace > 0 {
		return strings.TrimSpace(text[:lastSpace])
	}

	return text
}
