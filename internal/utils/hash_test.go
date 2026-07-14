package utils

import (
	"testing"
)

func TestCleanTopicTitle(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{
			input:    "nb-e9a116d8-8488-4fc6-b7b8-2a56fb0aaee9-ch-01-chapter-1",
			expected: "Chapter 1",
		},
		{
			input:    "nb-e9a116d8-8488-4fc6-b7b8-2a56fb0aaee9-ch-02-introduction-to-os",
			expected: "Chapter 2: Introduction To Os",
		},
		{
			input:    "Normal Chapter Title",
			expected: "Normal Chapter Title",
		},
		{
			input:    "nb-123-ch-10",
			expected: "Chapter 10",
		},
		{
			input:    "nb-uuid-ch-00-introduction",
			expected: "Chapter 0: Introduction",
		},
		{
			input:    "nb-uuid-ch-01-chapter-01",
			expected: "Chapter 1",
		},
		{
			input:    "   nb-uuid-ch-01-intro   ",
			expected: "Chapter 1: Intro",
		},
	}

	for _, tc := range tests {
		actual := CleanTopicTitle(tc.input)
		if actual != tc.expected {
			t.Errorf("CleanTopicTitle(%q) = %q; expected %q", tc.input, actual, tc.expected)
		}
	}
}
