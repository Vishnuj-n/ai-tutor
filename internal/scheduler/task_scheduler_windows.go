//go:build windows

package scheduler

import (
	"fmt"
	"os/exec"
	"strings"

	"ai-tutor/internal/utils"
)

const TaskName = "AiTutorStudyReminder"

// RemoveStudyStartTask deletes any existing study reminder task from Windows Task Scheduler.
func RemoveStudyStartTask() error {
	cmd := exec.Command("schtasks", "/Delete", "/TN", TaskName, "/F")
	out, err := cmd.CombinedOutput()
	if err != nil {
		// If task didn't exist, schtasks returns non-zero error output, which is safely ignored
		utils.Infof("schtasks remove info: %s", strings.TrimSpace(string(out)))
		return nil
	}
	utils.Infof("Removed existing Windows task %s: %s", TaskName, strings.TrimSpace(string(out)))
	return nil
}

// SyncStudyStartTask creates or updates the Windows Task Scheduler task for the study start notification.
// It explicitly purges any existing task first to ensure old schedules are removed.
func SyncStudyStartTask(startTime string, enabled bool) error {
	// Always remove old task first to guarantee stale schedules are cleared
	_ = RemoveStudyStartTask()

	if !enabled || strings.TrimSpace(startTime) == "" {
		return nil
	}

	// Validate HH:MM format
	parts := strings.Split(strings.TrimSpace(startTime), ":")
	if len(parts) != 2 {
		return fmt.Errorf("invalid start time format: %s", startTime)
	}

	// PowerShell script to trigger Windows native toast notification
	toastScript := `$Xml = '<toast><visual><binding template="ToastGeneric"><text>Study Time Started!</text><text>It is study time! Open AI Tutor to work on your queue.</text></binding></visual></toast>'; [Windows.UI.Notifications.ToastNotificationManager, Windows.UI.Notifications, ContentType = WindowsRuntime] | Out-Null; $ToastXml = [Windows.Data.Xml.Dom.XmlDocument, Windows.Data.Xml.Dom.XmlDocument, ContentType = WindowsRuntime]::new(); $ToastXml.LoadXml($Xml); $Notifier = [Windows.UI.Notifications.ToastNotificationManager]::CreateToastNotifier('AI Tutor'); $Notifier.Show([Windows.UI.Notifications.ToastNotification]::new($ToastXml))`

	cmd := exec.Command("schtasks", "/Create",
		"/TN", TaskName,
		"/TR", fmt.Sprintf("powershell.exe -WindowStyle Hidden -NoProfile -ExecutionPolicy Bypass -Command %q", toastScript),
		"/SC", "DAILY",
		"/ST", strings.TrimSpace(startTime),
		"/F",
	)

	out, err := cmd.CombinedOutput()
	if err != nil {
		utils.Errorf("schtasks create failed: %v, output: %s", err, string(out))
		return fmt.Errorf("failed to create scheduled task: %w (output: %s)", err, strings.TrimSpace(string(out)))
	}

	utils.Infof("Successfully scheduled Windows study reminder for %s: %s", startTime, strings.TrimSpace(string(out)))
	return nil
}
