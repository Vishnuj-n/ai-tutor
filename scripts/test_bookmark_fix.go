package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type SyllabusChapterDraft struct {
	Title     string `json:"title"`
	StartPage int    `json:"start_page"`
	EndPage   int    `json:"end_page"`
}

func firstString(node map[string]interface{}, keys ...string) string {
	for _, key := range keys {
		if value, ok := node[key]; ok {
			if typed, ok := value.(string); ok && strings.TrimSpace(typed) != "" {
				return typed
			}
		}
	}
	return ""
}

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

func parseBookmarks(raw []byte) []SyllabusChapterDraft {
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
			for _, key := range []string{"kids", "Kids", "children", "Children", "bookmarks", "Bookmarks", "items", "Items", "nodes", "Nodes", "sub", "Sub"} {
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

	draft := make([]SyllabusChapterDraft, 0, len(collected))
	for _, item := range collected {
		draft = append(draft, SyllabusChapterDraft{Title: item.title, StartPage: item.page, EndPage: item.page})
	}
	return draft
}

func main() {
	pdfPath := "Designing Data Intensive Applications by Martin Kleppmann.pdf"
	if len(os.Args) > 1 {
		pdfPath = os.Args[1]
	}

	absPath, err := filepath.Abs(pdfPath)
	if err != nil {
		fmt.Printf("Error abs path: %v\n", err)
		return
	}

	tmpFile, err := os.CreateTemp("", "pdfcpu-bookmarks-*.json")
	if err != nil {
		fmt.Printf("Error create temp: %v\n", err)
		return
	}
	tmpPath := tmpFile.Name()
	tmpFile.Close()
	defer os.Remove(tmpPath)

	ctx := context.Background()
	cmd := exec.CommandContext(ctx, "pdfcpu", "bookmarks", "export", absPath, tmpPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Printf("Error running pdfcpu: %v, output: %s\n", err, string(output))
		return
	}

	rawJSON, err := os.ReadFile(tmpPath)
	if err != nil {
		fmt.Printf("Error reading tmp json: %v\n", err)
		return
	}

	chapters := parseBookmarks(rawJSON)
	compactJSON, _ := json.Marshal(chapters)

	fmt.Printf("=== BOOKMARK EXTRACTOR VERIFICATION ===\n")
	fmt.Printf("PDF File: %s\n", filepath.Base(absPath))
	fmt.Printf("Raw pdfcpu output: %d bytes (~%d tokens)\n", len(rawJSON), len(rawJSON)/4)
	fmt.Printf("Extracted items: %d nodes\n", len(chapters))
	fmt.Printf("Compact JSON sent to LLM: %d bytes (~%d tokens)\n", len(compactJSON), len(compactJSON)/4)
	fmt.Printf("Prompt Token Reduction: %.1f%%\n\n", (1.0-float64(len(compactJSON))/float64(len(rawJSON)))*100)

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Extracted %d Bookmark Nodes from %s:\n", len(chapters), filepath.Base(absPath)))
	sb.WriteString(strings.Repeat("-", 60) + "\n")

	fmt.Println("Extracted Bookmark Nodes:")
	for i, ch := range chapters {
		line := fmt.Sprintf("  %3d: p.%-4d %s\n", i+1, ch.StartPage, ch.Title)
		fmt.Print(line)
		sb.WriteString(line)
	}

	txtFileName := "bookmark_nodes_output.txt"
	if err := os.WriteFile(txtFileName, []byte(sb.String()), 0644); err == nil {
		fmt.Printf("\nSaved full list of %d nodes to %s\n", len(chapters), txtFileName)
	} else {
		fmt.Printf("\nFailed to save output to %s: %v\n", txtFileName, err)
	}
}
