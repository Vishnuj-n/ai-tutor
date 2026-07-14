package study

import "testing"

func TestComputeCorrectnessFlags(t *testing.T) {
	quizPayload := `{
		"questions": [
			{"id":"q1","prompt":"P1","options":["A","B","C","D"],"correct_answer":"A"},
			{"id":"q2","prompt":"P2","options":["A","B","C","D"],"correct_answer":"B"},
			{"id":"q3","prompt":"P3","options":["A","B","C","D"],"correct_answer":"C"}
		],
		"passing_score": 70
	}`
	answersJSON := `[
		{"question_id":"q1","selected":"A"},
		{"question_id":"q2","selected":"D"},
		{"question_id":"q3","selected":"C"}
	]`

	flags, err := ComputeCorrectnessFlags(quizPayload, answersJSON)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(flags) != 3 {
		t.Fatalf("expected 3 flags, got %d", len(flags))
	}
	expected := []int{1, 0, 1}
	for i := range expected {
		if flags[i] != expected[i] {
			t.Fatalf("expected flags %v, got %v", expected, flags)
		}
	}
}

func TestComputeCorrectnessFlagsReturnsNilForInvalidJSON(t *testing.T) {
	flags, err := ComputeCorrectnessFlags("{", "[]")
	if err == nil {
		t.Fatalf("expected error for invalid payload JSON, got nil")
	}
	if flags != nil {
		t.Fatalf("expected nil flags for invalid payload JSON, got %v", flags)
	}
}
