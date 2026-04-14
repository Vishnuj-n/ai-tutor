package embeddings

import (
	"regexp"
	"strings"
)

var nonWord = regexp.MustCompile(`[^a-z0-9]+`)

// NormalizeWhitespace collapses repeated whitespace into single spaces.
func NormalizeWhitespace(input string) string {
	return strings.TrimSpace(strings.Join(strings.Fields(input), " "))
}

// TokenizeSimple splits text into a normalized token set for lightweight matching.
func TokenizeSimple(text string) map[string]struct{} {
	lowered := strings.ToLower(text)
	clean := nonWord.ReplaceAllString(lowered, " ")
	parts := strings.Fields(clean)
	set := make(map[string]struct{}, len(parts))
	for _, part := range parts {
		if len(part) < 3 {
			continue
		}
		set[part] = struct{}{}
	}
	return set
}
