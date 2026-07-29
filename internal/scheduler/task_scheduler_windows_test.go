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

func TestSyncStudyStartTaskRemoved(t *testing.T) {
	orig := execCommandContext
	execCommandContext = fakeExecCommandContext
	defer func() { execCommandContext = orig }()

	err := SyncStudyStartTask("17:00", true)
	if err != nil {
		t.Fatalf("expected no error when syncing disabled scheduled task, got: %v", err)
	}
}
