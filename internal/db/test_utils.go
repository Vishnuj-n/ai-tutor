package db

import (
	"strings"
	"testing"
)

// assertCountEquals asserts that a query returns exactly the expected count
func assertCountEquals(t *testing.T, query string, arg interface{}, want int) {
	t.Helper()

	if conn == nil {
		t.Fatalf("nil db connection")
	}

	var got int
	if err := conn.QueryRow(query, arg).Scan(&got); err != nil {
		t.Fatalf("query failed (%s): %v", sanitizeWhitespace(query), err)
	}
	if got != want {
		t.Fatalf("unexpected count for query (%s): got=%d want=%d", sanitizeWhitespace(query), got, want)
	}
}

// contains checks if a target string exists in a slice of strings
func contains(items []string, target string) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
}

func equalStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if strings.TrimSpace(a[i]) != strings.TrimSpace(b[i]) {
			return false
		}
	}
	return true
}

// sanitizeWhitespace normalizes whitespace in a string for consistent error messages
func sanitizeWhitespace(input string) string {
	return strings.Join(strings.Fields(input), " ")
}
