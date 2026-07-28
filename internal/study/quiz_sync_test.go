package study

import (
	"testing"

	"ai-tutor/internal/models"
)

func TestNormalizeChunkIDs(t *testing.T) {
	tests := []struct {
		name      string
		input     []string
		wantLen   int
		expectErr bool
	}{
		{
			name:      "empty input",
			input:     []string{},
			expectErr: true,
		},
		{
			name:      "whitespace only input",
			input:     []string{"   ", "\t"},
			expectErr: true,
		},
		{
			name:      "deduplicates and trims",
			input:     []string{" chunk-1 ", "chunk-2", "chunk-1"},
			wantLen:   2,
			expectErr: false,
		},
		{
			name:      "caps at maxChunks 24",
			input:     makeChunkIDList(30),
			wantLen:   24,
			expectErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := normalizeChunkIDs(tt.input)
			if tt.expectErr && err == nil {
				t.Fatalf("expected error, got nil")
			}
			if !tt.expectErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !tt.expectErr && len(res) != tt.wantLen {
				t.Errorf("expected len %d, got %d", tt.wantLen, len(res))
			}
		})
	}
}

func makeChunkIDList(n int) []string {
	res := make([]string, n)
	for i := 0; i < n; i++ {
		res[i] = "chunk_" + string(rune('a'+i))
	}
	return res
}

func TestBuildQuizContext(t *testing.T) {
	chunkText := map[string]string{
		"c1": "This is chunk one with some words for testing.",
		"c2": "This is chunk two with more words for prompt context.",
	}

	// Budget that easily fits c1 and c2
	res, err := buildQuizContext([]string{"c1", "c2"}, chunkText, 1000, 300)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(res.contextParts) != 2 {
		t.Errorf("expected 2 context parts, got %d", len(res.contextParts))
	}
	if res.truncatedCount != 0 {
		t.Errorf("expected 0 truncated, got %d", res.truncatedCount)
	}

	// Budget that fits c1 but not c2 (e.g. availableBudget = 15)
	resSmall, err := buildQuizContext([]string{"c1", "c2"}, chunkText, 15, 300)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resSmall.contextParts) != 1 {
		t.Errorf("expected 1 context part, got %d", len(resSmall.contextParts))
	}
	if resSmall.truncatedCount != 1 {
		t.Errorf("expected 1 truncated, got %d", resSmall.truncatedCount)
	}
}

func TestCalculateQuizScore(t *testing.T) {
	questions := []models.QuizTaskQuestion{
		{ID: "q1", Prompt: "P1", Options: []string{"A", "B", "C", "D"}, CorrectAnswer: "A"},
		{ID: "q2", Prompt: "P2", Options: []string{"A", "B", "C", "D"}, CorrectAnswer: "B"},
	}

	tests := []struct {
		name         string
		answers      []models.QuizAnswer
		passingScore int
		wantScore    int
		wantPassed   bool
		wantFailed   int
	}{
		{
			name: "all correct",
			answers: []models.QuizAnswer{
				{QuestionID: "q1", Selected: "A"},
				{QuestionID: "q2", Selected: "B"},
			},
			passingScore: 70,
			wantScore:    100,
			wantPassed:   true,
			wantFailed:   0,
		},
		{
			name: "half correct",
			answers: []models.QuizAnswer{
				{QuestionID: "q1", Selected: "A"},
				{QuestionID: "q2", Selected: "WRONG"},
			},
			passingScore: 70,
			wantScore:    50,
			wantPassed:   false,
			wantFailed:   1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := calculateQuizScore(questions, tt.answers, tt.passingScore)
			if res.score != tt.wantScore {
				t.Errorf("expected score %d, got %d", tt.wantScore, res.score)
			}
			if res.passed != tt.wantPassed {
				t.Errorf("expected passed %v, got %v", tt.wantPassed, res.passed)
			}
			if len(res.failedQuestions) != tt.wantFailed {
				t.Errorf("expected %d failed questions, got %d", tt.wantFailed, len(res.failedQuestions))
			}
		})
	}
}

func TestValidateAndConvertQuestions(t *testing.T) {
	resp := &quizLLMResponse{
		Questions: []quizLLMQuestion{
			{
				Prompt:        "Valid prompt?",
				Options:       []string{"Opt A", "Opt B", "Opt C", "Opt D"},
				CorrectAnswer: "Opt A",
				SourceChunkID: "c1",
			},
			{
				Prompt:        "Invalid - 3 options",
				Options:       []string{"Opt A", "Opt B", "Opt C"},
				CorrectAnswer: "Opt A",
				SourceChunkID: "c2",
			},
			{
				Prompt:        "Invalid - correct answer mismatch",
				Options:       []string{"Opt A", "Opt B", "Opt C", "Opt D"},
				CorrectAnswer: "Nonexistent",
				SourceChunkID: "c3",
			},
		},
	}

	questions := validateAndConvertQuestions(resp)
	if len(questions) != 1 {
		t.Fatalf("expected 1 valid question, got %d", len(questions))
	}
	if questions[0].Prompt != "Valid prompt?" {
		t.Errorf("expected prompt 'Valid prompt?', got '%s'", questions[0].Prompt)
	}
}
