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
	if err := db.SeedDemoDataForTests(); err != nil {
		t.Fatalf("failed to seed rag test db: %v", err)
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

	prompt, _, err := buildPrompt("Operating Systems", "Explain scheduling", *ctx)
	if err != nil {
		t.Fatalf("buildPrompt failed: %v", err)
	}
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

	prompt, _, err := buildPrompt("Operating Systems", "Unknown question", *ctx)
	if err != nil {
		t.Fatalf("buildPrompt failed: %v", err)
	}
	needle := "I don't have enough information in the provided material to answer that confidently."
	if !strings.Contains(prompt, needle) {
		t.Fatalf("prompt missing guardrail phrase, prompt=%s", prompt)
	}
}

func TestTrimToTokenBudgetNoSentencePunctuationStillKeepsContent(t *testing.T) {
	initPromptTokenizerForTests(t)

	topic := "Operating Systems"
	question := "Explain scheduling"
	existing := ""
	candidate := "Heading One\n- bullet alpha\n- bullet beta\n- bullet gamma"

	baseTokens, err := countPromptTokens(formatPrompt(topic, existing, question))
	if err != nil {
		t.Fatalf("count base prompt tokens failed: %v", err)
	}

	tokenLimit := baseTokens + 12
	trimmed, err := trimToTokenBudget(topic, question, existing, candidate, tokenLimit)
	if err != nil {
		t.Fatalf("trimToTokenBudget failed: %v", err)
	}
	if strings.TrimSpace(trimmed) == "" {
		t.Fatalf("expected non-empty trimmed content for punctuation-free candidate")
	}

	finalTokens, err := countPromptTokens(formatPrompt(topic, existing+trimmed, question))
	if err != nil {
		t.Fatalf("count final prompt tokens failed: %v", err)
	}
	if finalTokens > tokenLimit {
		t.Fatalf("trimmed prompt exceeded token limit: got=%d limit=%d", finalTokens, tokenLimit)
	}
}

func TestTrimToTokenBudgetSmallRemainingBudgetReturnsEmpty(t *testing.T) {
	initPromptTokenizerForTests(t)

	topic := "Operating Systems"
	question := "Explain scheduling"
	existing := ""
	candidate := "Some candidate context that should not fit"

	baseTokens, err := countPromptTokens(formatPrompt(topic, existing, question))
	if err != nil {
		t.Fatalf("count base prompt tokens failed: %v", err)
	}

	trimmed, err := trimToTokenBudget(topic, question, existing, candidate, baseTokens)
	if err != nil {
		t.Fatalf("trimToTokenBudget failed: %v", err)
	}
	if trimmed != "" {
		t.Fatalf("expected empty trim when no remaining budget, got=%q", trimmed)
	}
}
