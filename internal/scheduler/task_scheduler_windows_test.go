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
	toastScript := `[Windows.UI.Notifications.ToastNotificationManager,Windows.UI.Notifications,ContentType=WindowsRuntime]|Out-Null;$x=[Windows.Data.Xml.Dom.XmlDocument,Windows.Data.Xml.Dom.XmlDocument,ContentType=WindowsRuntime]::new();$x.LoadXml('<toast><visual><binding template="ToastGeneric"><text>Study Time Started!</text><text>Open AI Tutor to work on your queue.</text></binding></visual></toast>');[Windows.UI.Notifications.ToastNotificationManager]::CreateToastNotifier('AI Tutor').Show([Windows.UI.Notifications.ToastNotification]::new($x))`
	encodedScript := encodePowerShellCommand(toastScript)
	trValue := "powershell -W Hidden -NoP -Enc " + encodedScript
	if len(trValue) > 261 {
		t.Fatalf("/TR value length %d exceeds 261 character limit", len(trValue))
	}
}
