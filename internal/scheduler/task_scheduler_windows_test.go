//go:build windows

package scheduler

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
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

	tmpDir := t.TempDir()
	t.Setenv("APPDATA", tmpDir)

	vbsDir := filepath.Join(tmpDir, "Studyloop")
	if err := os.MkdirAll(vbsDir, 0o755); err != nil {
		t.Fatalf("failed to create Studyloop dir: %v", err)
	}
	vbsPath := filepath.Join(vbsDir, "reminder.vbs")
	if err := os.WriteFile(vbsPath, []byte("WScript.Echo 1"), 0o644); err != nil {
		t.Fatalf("failed to create fixture reminder.vbs: %v", err)
	}

	err := SyncStudyStartTask("17:00", true)
	if err != nil {
		t.Fatalf("expected no error when syncing disabled scheduled task, got: %v", err)
	}

	if _, err := os.Stat(vbsPath); !os.IsNotExist(err) {
		t.Fatalf("expected reminder.vbs to be removed, but it still exists")
	}
}
