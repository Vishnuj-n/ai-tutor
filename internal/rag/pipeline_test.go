package rag

import (
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"ai-tutor/internal/db"
	"ai-tutor/internal/embeddings"
)

func initPromptTokenizerForTests(t *testing.T) {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatalf("failed to resolve caller path")
	}

	path := filepath.Join(filepath.Dir(file), "..", "embeddings", "testdata", "tokenizer.json")
	absPath, err := filepath.Abs(path)
	if err != nil {
		t.Fatalf("failed to resolve tokenizer path: %v", err)
	}

	if _, err := os.Stat(absPath); err != nil {
		t.Fatalf("tokenizer test asset missing: %v", err)
	}

	if err := embeddings.InitPromptTokenizer(absPath); err != nil {
		t.Fatalf("failed to initialize prompt tokenizer: %v", err)
	}
}

func initRagTestDB(t *testing.T) {
	t.Helper()
	tempDB := filepath.Join(t.TempDir(), "rag-test.db")
	if err := db.Init(tempDB, ""); err != nil {
		t.Fatalf("failed to init rag test db: %v", err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Fatalf("failed to close rag test db: %v", err)
		}
	})
}

func TestBuildContextParentOrderIsDeterministic(t *testing.T) {
	initRagTestDB(t)

	results := []RetrievalResult{
		{ParentID: "parent-2"},
		{ParentID: "parent-1"},
		{ParentID: "parent-2"},
	}

	ctx, err := BuildContext(results, "os-scheduling")
	if err != nil {
		t.Fatalf("BuildContext failed: %v", err)
	}

	wantOrder := []string{"parent-2", "parent-1"}
	if !reflect.DeepEqual(ctx.ParentIDs, wantOrder) {
		t.Fatalf("unexpected parent order: got=%v want=%v", ctx.ParentIDs, wantOrder)
	}
}

func TestBuildPromptUsesParentOrder(t *testing.T) {
	initPromptTokenizerForTests(t)

	ctx := &RetrievalContext{
		TopicID: "os-scheduling",
		Sections: map[string]string{
			"a": "**A**\nSection A",
			"b": "**B**\nSection B",
		},
		ParentIDs: []string{"b", "a"},
	}

	prompt, _ := buildPrompt("Operating Systems", "Explain scheduling", *ctx)
	bIdx := strings.Index(prompt, "**B**")
	aIdx := strings.Index(prompt, "**A**")
	if bIdx == -1 || aIdx == -1 || bIdx > aIdx {
		t.Fatalf("prompt did not preserve section order: %s", prompt)
	}
}

func TestBuildPromptContainsInsufficientContextGuardrail(t *testing.T) {
	initPromptTokenizerForTests(t)

	ctx := &RetrievalContext{
		TopicID: "os-scheduling",
		Sections: map[string]string{
			"a": "**A**\nSection A",
		},
		ParentIDs: []string{"a"},
	}

	prompt, _ := buildPrompt("Operating Systems", "Unknown question", *ctx)
	needle := "I don't have enough information in the provided material to answer that confidently."
	if !strings.Contains(prompt, needle) {
		t.Fatalf("prompt missing guardrail phrase, prompt=%s", prompt)
	}
}
