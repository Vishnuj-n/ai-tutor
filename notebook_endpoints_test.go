package main

import (
	"testing"

	"ai-tutor/internal/notebook"
)

func TestParsePDFCPUBookmarkDraftFromJSON_Level1Only(t *testing.T) {
	raw := []byte(`{
		"bookmarks": [
			{"title":"Chapter 1","page":1},
			{"title":"Chapter 2","page":5,"children":[
				{"title":"Subtopic 2.1","page":7}
			]}
		]
	}`)

	draft := notebook.ParsePDFCPUBookmarkDraftFromJSON(raw, 12)
	if len(draft) != 2 {
		t.Fatalf("expected 2 level-1 draft entries, got %d (%#v)", len(draft), draft)
	}

	if draft[0].Title != "Chapter 1" || draft[0].StartPage != 1 || draft[0].EndPage != 4 {
		t.Fatalf("unexpected first chapter: %#v", draft[0])
	}
	if draft[1].Title != "Chapter 2" || draft[1].StartPage != 5 || draft[1].EndPage != 12 {
		t.Fatalf("unexpected second chapter: %#v", draft[1])
	}
}

func TestExtractFullPDFCPUBookmarkNodes_AllLevels(t *testing.T) {
	raw := []byte(`{
		"bookmarks": [
			{"title":"Chapter 1","page":1},
			{"title":"Chapter 2","page":5,"children":[
				{"title":"Subtopic 2.1","page":7}
			]}
		]
	}`)

	nodes := notebook.ExtractFullPDFCPUBookmarkNodes(raw)
	if len(nodes) != 3 {
		t.Fatalf("expected 3 extracted bookmark nodes across all levels, got %d (%#v)", len(nodes), nodes)
	}
	if nodes[2].Title != "Subtopic 2.1" || nodes[2].StartPage != 7 {
		t.Fatalf("unexpected third node: %#v", nodes[2])
	}
}

func TestParsePDFCPUBookmarkDraftFromJSON_EmptyPayload(t *testing.T) {
	raw := []byte(`{"bookmarks":[]}`)
	draft := notebook.ParsePDFCPUBookmarkDraftFromJSON(raw, 10)
	if len(draft) != 0 {
		t.Fatalf("expected empty draft, got %#v", draft)
	}
}
