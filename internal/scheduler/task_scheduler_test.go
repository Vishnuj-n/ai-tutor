package scheduler

import (
	"testing"
)

func TestSyncStudyStartTaskDisabled(t *testing.T) {
	err := SyncStudyStartTask("17:00", false)
	if err != nil {
		t.Fatalf("expected no error when sync disabled, got: %v", err)
	}
}

func TestSyncStudyStartTaskInvalidTime(t *testing.T) {
	err := SyncStudyStartTask("invalid", true)
	if err == nil {
		t.Fatal("expected error for invalid start time format")
	}
}
