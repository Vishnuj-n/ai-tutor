package main

import (
	"testing"
)

func TestParsePDFCPUBookmarkDraftFromJSON_NestedPayload(t *testing.T) {
	raw := []byte(`{
		"bookmarks": [
			{"title":"Chapter 1","page":1},
			{"title":"Chapter 2","page":5,"children":[
				{"title":"Subtopic 2.1","page":7}
			]}
		]
	}`)

	draft := parsePDFCPUBookmarkDraftFromJSON(raw, 12)
	if len(draft) != 3 {
		t.Fatalf("expected 3 draft entries, got %d (%#v)", len(draft), draft)
	}

	if draft[0].Title != "Chapter 1" || draft[0].StartPage != 1 {
		t.Fatalf("unexpected first chapter: %#v", draft[0])
	}
	if draft[1].Title != "Chapter 2" || draft[1].StartPage != 5 {
		t.Fatalf("unexpected second chapter: %#v", draft[1])
	}
	if draft[2].StartPage != 7 {
		t.Fatalf("unexpected third chapter start page: %#v", draft[2])
	}
	if draft[2].EndPage != 12 {
		t.Fatalf("expected last chapter to extend to page count, got %#v", draft[2])
	}
}

func TestParsePDFCPUBookmarkDraftFromJSON_EmptyPayload(t *testing.T) {
	raw := []byte(`{"bookmarks":[]}`)
	draft := parsePDFCPUBookmarkDraftFromJSON(raw, 10)
	if len(draft) != 0 {
		t.Fatalf("expected empty draft, got %#v", draft)
	}
}
