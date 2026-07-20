package utils

import (
	"crypto/md5"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"strings"
)

// MD5Hex returns the lowercase hex MD5 digest for the provided text.
func MD5Hex(text string) string {
	hash := md5.Sum([]byte(text))
	return fmt.Sprintf("%x", hash)
}

// FileSHA256 returns the lowercase hex SHA-256 digest of a file's contents.
func FileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

// CleanTopicTitle formats a topic ID or raw string (like nb-uuid-ch-01-chapter-1)
// into a clean user-facing title (like "Chapter 1").
func CleanTopicTitle(title string) string {
	title = strings.TrimSpace(title)
	if !strings.HasPrefix(title, "nb-") || !strings.Contains(title, "-ch-") {
		return title
	}
	parts := strings.SplitN(title, "-ch-", 2)
	if len(parts) < 2 {
		return title
	}
	subParts := strings.SplitN(parts[1], "-", 2)
	chNumStr := subParts[0]
	chNumStr = strings.TrimLeft(chNumStr, "0")
	if chNumStr == "" {
		chNumStr = "0"
	}

	if len(subParts) < 2 {
		return "Chapter " + chNumStr
	}

	suffix := subParts[1]
	suffix = strings.ReplaceAll(suffix, "-", " ")
	suffix = strings.TrimSpace(suffix)

	// Capitalize words
	words := strings.Fields(suffix)
	for i, w := range words {
		if len(w) > 0 {
			words[i] = strings.ToUpper(string(w[0])) + strings.ToLower(w[1:])
		}
	}
	formattedSuffix := strings.Join(words, " ")

	// Avoid redundant "Chapter 1: Chapter 1" or "Chapter 1: Chapter-1"
	redundantPrefix := "Chapter " + chNumStr
	if strings.EqualFold(formattedSuffix, redundantPrefix) || strings.EqualFold(formattedSuffix, "Chapter "+subParts[0]) {
		return redundantPrefix
	}

	return "Chapter " + chNumStr + ": " + formattedSuffix
}
