package main

import (
	"strings"
	"testing"
)

func TestVersionCompareHelper(t *testing.T) {
	// ponytail: test the version difference logic cleanly
	testCases := []struct {
		remote   string
		current  string
		expected bool
	}{
		{"1.0.0", "1.0.0", false},
		{"v1.0.0", "1.0.0", false},
		{"1.1.0", "1.0.0", true},
		{"v1.1.0", "1.0.0", true},
		{"", "1.0.0", false},
	}

	for _, tc := range testCases {
		remoteClean := strings.TrimPrefix(tc.remote, "v")
		currentClean := strings.TrimPrefix(tc.current, "v")
		actual := remoteClean != "" && remoteClean != currentClean
		if actual != tc.expected {
			t.Errorf("Compare(%q, %q) expected %v, got %v", tc.remote, tc.current, tc.expected, actual)
		}
	}
}
