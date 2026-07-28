//go:build windows

package scheduler

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"ai-tutor/internal/utils"
)

const TaskName = "AiTutorStudyReminder"

var execCommandContext = exec.CommandContext

// RemoveStudyStartTask deletes any existing study reminder task from Windows Task Scheduler.
func RemoveStudyStartTask() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := execCommandContext(ctx, "schtasks", "/Delete", "/TN", TaskName, "/F")
	out, err := cmd.CombinedOutput()
	outStr := strings.TrimSpace(string(out))
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return fmt.Errorf("schtasks delete timed out: %w", ctx.Err())
		}
		outLower := strings.ToLower(outStr)
		if strings.Contains(outLower, "cannot find") || strings.Contains(outLower, "does not exist") || strings.Contains(outLower, "not found") {
			utils.Infof("schtasks remove info: %s", outStr)
			return nil
		}
		return fmt.Errorf("failed to remove scheduled task: %w (output: %s)", err, outStr)
	}
	utils.Infof("Removed existing Windows task %s: %s", TaskName, outStr)
	return nil
}

// SyncStudyStartTask creates or updates the Windows Task Scheduler task for the study start notification.
func SyncStudyStartTask(startTime string, enabled bool) error {
	trimmedTime := strings.TrimSpace(startTime)
	if !enabled || trimmedTime == "" {
		return RemoveStudyStartTask()
	}

	// Validate HH:MM format
	if _, err := time.Parse("15:04", trimmedTime); err != nil {
		return fmt.Errorf("invalid start time format: %s: %w", startTime, err)
	}

	// Purge existing task first
	if err := RemoveStudyStartTask(); err != nil {
		return err
	}

	// PowerShell script to trigger Windows native toast notification
	toastScript := `$Xml = '<toast><visual><binding template="ToastGeneric"><text>Study Time Started!</text><text>It is study time! Open AI Tutor to work on your queue.</text></binding></visual></toast>'; [Windows.UI.Notifications.ToastNotificationManager, Windows.UI.Notifications, ContentType = WindowsRuntime] | Out-Null; $ToastXml = [Windows.Data.Xml.Dom.XmlDocument, Windows.Data.Xml.Dom.XmlDocument, ContentType = WindowsRuntime]::new(); $ToastXml.LoadXml($Xml); $Notifier = [Windows.UI.Notifications.ToastNotificationManager]::CreateToastNotifier('AI Tutor'); $Notifier.Show([Windows.UI.Notifications.ToastNotification]::new($ToastXml))`

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := execCommandContext(ctx, "schtasks", "/Create",
		"/TN", TaskName,
		"/TR", fmt.Sprintf("powershell.exe -WindowStyle Hidden -NoProfile -ExecutionPolicy Bypass -Command %q", toastScript),
		"/SC", "DAILY",
		"/ST", trimmedTime,
		"/IT",
		"/F",
	)

	out, err := cmd.CombinedOutput()
	outStr := strings.TrimSpace(string(out))
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return fmt.Errorf("schtasks create timed out: %w", ctx.Err())
		}
		utils.Errorf("schtasks create failed: %v, output: %s", err, outStr)
		return fmt.Errorf("failed to create scheduled task: %w (output: %s)", err, outStr)
	}

	utils.Infof("Successfully scheduled Windows study reminder for %s: %s", trimmedTime, outStr)
	return nil
}
