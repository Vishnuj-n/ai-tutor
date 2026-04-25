package db

import (
	"fmt"
	"strings"
)

// validateID trims whitespace from an ID and returns an error if it's empty.
// This is a common validation helper used throughout the codebase.
// nolint:unused // Infrastructure helper for future input sanitization refactoring
func validateID(id string, fieldName string) (string, error) {
	trimmed := strings.TrimSpace(id)
	if trimmed == "" {
		return "", fmt.Errorf("%s is required", fieldName)
	}
	return trimmed, nil
}
