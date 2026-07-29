//go:build windows

package scheduler

import (
	"context"
	"os"
	"os/exec"
	"testing"
)

func fakeExecCommandContext(ctx context.Context, command string, args ...string) *exec.Cmd {
	cs := []string{"-test.run=TestHelperProcess", "--", command}
	cs = append(cs, args...)
	cmd := exec.Command(os.Args[0], cs...)
	cmd.Env = []string{"GO_WANT_HELPER_PROCESS=1"}
	return cmd
}

func TestHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}
	os.Exit(0)
}

func TestSyncStudyStartTaskInvalidTime(t *testing.T) {
	orig := execCommandContext
	execCommandContext = fakeExecCommandContext
	defer func() { execCommandContext = orig }()

	err := SyncStudyStartTask("invalid", true)
	if err == nil {
		t.Fatal("expected error for invalid start time format")
	}
}

func TestSyncStudyStartTaskDisabledWindows(t *testing.T) {
	orig := execCommandContext
	execCommandContext = fakeExecCommandContext
	defer func() { execCommandContext = orig }()

	err := SyncStudyStartTask("17:00", false)
	if err != nil {
		t.Fatalf("expected no error when sync disabled, got: %v", err)
	}
}

func TestSyncStudyStartTaskTRLength(t *testing.T) {
	trValue := `powershell -W Hidden -NoP -C "msg * /TIME:10 Study Time! Time to review your queue in AI Tutor."`
	t.Logf("/TR value length: %d", len(trValue))
	if len(trValue) > 261 {
		t.Fatalf("/TR value length %d exceeds Windows schtasks limit of 261 chars", len(trValue))
	}
}
