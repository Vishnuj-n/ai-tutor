package notebook

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"ai-tutor/internal/models"
)

// extractPDFCPUBookmarkDraft extracts chapter drafts from PDF bookmarks using pdfcpu.
func extractPDFCPUBookmarkDraft(filePath string, pageCount int, uploadDir string) []models.SyllabusChapterDraft {
	jsonOutput, err := runPDFCPUBookmarksExport(filePath, uploadDir)
	if err != nil || strings.TrimSpace(string(jsonOutput)) == "" {
		return nil
	}

	return ParsePDFCPUBookmarkDraftFromJSON(jsonOutput, pageCount)
}

// runPDFCPUBookmarksExport exports PDF bookmarks to JSON using pdfcpu.
func runPDFCPUBookmarksExport(filePath string, uploadDir string) ([]byte, error) {
	absFilePath, err := validatePDFCPUInputFilePath(filePath, uploadDir)
	if err != nil {
		return nil, err
	}

	pdfcpuPath, err := findPDFCPUExecutable()
	if err != nil {
		return nil, err
	}

	tmpFile, err := os.CreateTemp("", "pdfcpu-bookmarks-*.json")
	if err != nil {
		return nil, err
	}
	tmpPath := tmpFile.Name()
	_ = tmpFile.Close()
	defer func() {
		_ = os.Remove(tmpPath)
	}()

	cmd := exec.Command(pdfcpuPath, "bookmarks", "export", absFilePath, tmpPath)
	if _, runErr := cmd.Output(); runErr != nil {
		return nil, runErr
	}

	content, readErr := os.ReadFile(tmpPath)
	if readErr != nil {
		return nil, readErr
	}
	return content, nil
}

// validatePDFCPUInputFilePath validates that the file path is safe and within allowed directory.
func validatePDFCPUInputFilePath(filePath string, uploadDir string) (string, error) {
	trimmed := strings.TrimSpace(filePath)
	if trimmed == "" {
		return "", fmt.Errorf("file path is required")
	}
	if strings.Contains(trimmed, "\x00") {
		return "", fmt.Errorf("invalid file path")
	}
	if strings.Contains(trimmed, "..\\") || strings.Contains(trimmed, "../") {
		return "", fmt.Errorf("file path traversal is not allowed")
	}

	cleaned := filepath.Clean(trimmed)
	absPath, err := filepath.Abs(cleaned)
	if err != nil {
		return "", fmt.Errorf("invalid file path: %w", err)
	}
	uploadRoot, err := filepath.Abs(uploadDir)
	if err != nil {
		return "", fmt.Errorf("failed to resolve upload directory: %w", err)
	}
	relToUploadRoot, err := filepath.Rel(uploadRoot, absPath)
	if err != nil {
		return "", fmt.Errorf("invalid file path relation: %w", err)
	}
	if relToUploadRoot == ".." || strings.HasPrefix(relToUploadRoot, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("file path is outside allowed upload directory")
	}

	info, err := os.Stat(absPath)
	if err != nil {
		return "", fmt.Errorf("failed to stat file path: %w", err)
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("file path must point to a regular file")
	}
	return absPath, nil
}

// findPDFCPUExecutable locates the pdfcpu binary in common installation paths.
func findPDFCPUExecutable() (string, error) {
	pdfcpuPath, err := exec.LookPath("pdfcpu")
	if err == nil {
		return pdfcpuPath, nil
	}

	binary := "pdfcpu"
	if runtime.GOOS == "windows" {
		binary = "pdfcpu.exe"
	}

	candidateDirs := make([]string, 0, 8)
	if gobin := strings.TrimSpace(os.Getenv("GOBIN")); gobin != "" {
		candidateDirs = append(candidateDirs, gobin)
	}
	if gopath := strings.TrimSpace(os.Getenv("GOPATH")); gopath != "" {
		candidateDirs = append(candidateDirs, filepath.Join(gopath, "bin"))
	} else if home, homeErr := os.UserHomeDir(); homeErr == nil && home != "" {
		candidateDirs = append(candidateDirs, filepath.Join(home, "go", "bin"))
	}

	switch runtime.GOOS {
	case "windows":
		candidateDirs = append(candidateDirs, `C:\Program Files\pdfcpu`, `C:\Program Files (x86)\pdfcpu`)
	case "darwin":
		candidateDirs = append(candidateDirs, "/usr/local/bin", "/opt/homebrew/bin")
	default:
		candidateDirs = append(candidateDirs, "/usr/local/bin", "/usr/bin")
	}

	for _, dir := range candidateDirs {
		if strings.TrimSpace(dir) == "" {
			continue
		}
		candidate := filepath.Join(dir, binary)
		info, statErr := os.Stat(candidate)
		if statErr == nil && !info.IsDir() {
			return candidate, nil
		}
	}

	return "", fmt.Errorf("pdfcpu binary not found; install pdfcpu and ensure it is available on PATH, GOBIN, or GOPATH/bin")
}

// ParsePDFCPUBookmarkDraftFromJSON parses pdfcpu bookmark JSON output into chapter drafts.
func ParsePDFCPUBookmarkDraftFromJSON(raw []byte, pageCount int) []models.SyllabusChapterDraft {
	var payload interface{}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil
	}

	type bookmarkNode struct {
		title string
		page  int
	}

	collected := make([]bookmarkNode, 0)
	var walk func(node interface{})
	walk = func(node interface{}) {
		switch typed := node.(type) {
		case map[string]interface{}:
			title := strings.TrimSpace(firstString(typed, "title", "Title", "name", "Name"))
			page := firstInt(typed, "page", "Page", "pageNr", "PageNr", "p", "PageFrom", "from")
			if title != "" && page > 0 {
				collected = append(collected, bookmarkNode{title: title, page: page})
			}
			for _, key := range []string{"children", "Children", "bookmarks", "Bookmarks", "items", "Items", "nodes", "Nodes", "sub", "Sub"} {
				if child, ok := typed[key]; ok {
					walk(child)
				}
			}
		case []interface{}:
			for _, child := range typed {
				walk(child)
			}
		}
	}

	walk(payload)
	if len(collected) == 0 {
		return nil
	}

	draft := make([]models.SyllabusChapterDraft, 0, len(collected))
	for _, item := range collected {
		draft = append(draft, models.SyllabusChapterDraft{Title: item.title, StartPage: item.page, EndPage: item.page})
	}

	return NormalizeSyllabusChapters(draft, pageCount)
}

// firstString returns the first non-empty string value for the given keys.
func firstString(node map[string]interface{}, keys ...string) string {
	for _, key := range keys {
		if value, ok := node[key]; ok {
			switch typed := value.(type) {
			case string:
				if strings.TrimSpace(typed) != "" {
					return typed
				}
			}
		}
	}
	return ""
}

// firstInt returns the first integer value for the given keys.
func firstInt(node map[string]interface{}, keys ...string) int {
	for _, key := range keys {
		if value, ok := node[key]; ok {
			switch typed := value.(type) {
			case float64:
				return int(typed)
			case int:
				return typed
			case string:
				var parsed int
				if _, err := fmt.Sscanf(strings.TrimSpace(typed), "%d", &parsed); err == nil {
					return parsed
				}
			}
		}
	}
	return 0
}
