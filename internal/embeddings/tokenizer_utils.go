package embeddings

import (
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
func CountTokens(text string) int {
	text = strings.TrimSpace(text)
	if text == "" {
		return 0
	}

	tok := getPromptTokenizer()
	if tok == nil {
		return len([]byte(text))
	}

	enc, err := tok.EncodeSingle(text, true)
	if err != nil {
		log.Printf("tokenizer encode failed in CountTokens: %v", err)
		return len([]byte(text))
	}

	return len(enc.Ids)
}

// TruncateToTokens trims text to token limit, preferring clean sentence boundaries.
func TruncateToTokens(text string, limit int) string {
	text = strings.TrimSpace(text)
	if text == "" || limit <= 0 {
		return ""
	}

	tok := getPromptTokenizer()
	if tok == nil {
		return truncateFallback(text, limit)
	}

	enc, err := tok.EncodeSingle(text, true)
	if err != nil {
		log.Printf("tokenizer encode failed in TruncateToTokens: %v", err)
		return truncateFallback(text, limit)
	}

	if len(enc.Ids) <= limit {
		return text
	}

	decoded := tok.Decode(enc.Ids[:limit], true)

	return trimToSentenceBoundary(decoded)
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

func truncateFallback(text string, limit int) string {
	if limit <= 0 {
		return ""
	}

	runes := []rune(text)
	if len(runes) <= limit {
		return strings.TrimSpace(text)
	}

	return trimToSentenceBoundary(string(runes[:limit]))
}
