package embeddings

import (
	"strings"
)

// NormalizeWhitespace collapses repeated whitespace into single spaces.
func NormalizeWhitespace(input string) string {
	return strings.TrimSpace(strings.Join(strings.Fields(input), " "))
}
