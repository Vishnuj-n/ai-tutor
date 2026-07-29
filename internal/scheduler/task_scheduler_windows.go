//go:build windows

package scheduler

import (
	"context"
	"encoding/base64"
	"fmt"
	"os/exec"
	"strings"
	"time"
	"unicode/utf16"

	"ai-tutor/internal/utils"
)

func encodePowerShellCommand(cmd string) string {
	encoded := utf16.Encode([]rune(cmd))
	b := make([]byte, len(encoded)*2)
	for i, u := range encoded {
		b[i*2] = byte(u)
		b[i*2+1] = byte(u >> 8)
	}
	return base64.StdEncoding.EncodeToString(b)
}

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

	// PowerShell script to trigger Windows native toast notification.
	// PowerShell script using simplified BurntToast / Windows notification payload.
	// Kept ultra concise so Base64 UTF-16LE -EncodedCommand + command wrapper easily fits within schtasks 261 char limit.
	toastScript := `[Windows.UI.Notifications.ToastNotificationManager,Windows.UI.Notifications,ContentType=WindowsRuntime]|Out-Null;$x=[Windows.Data.Xml.Dom.XmlDocument,Windows.Data.Xml.Dom.XmlDocument,ContentType=WindowsRuntime]::new();$x.LoadXml('<toast><visual><binding template="ToastGeneric"><text>Study Time Started!</text><text>Open AI Tutor to work on your queue.</text></binding></visual></toast>');[Windows.UI.Notifications.ToastNotificationManager]::CreateToastNotifier('AI Tutor').Show([Windows.UI.Notifications.ToastNotification]::new($x))`

	// Encode toastScript as UTF-16LE base64 string
	encodedScript := encodePowerShellCommand(toastScript)
	trValue := fmt.Sprintf("powershell -W Hidden -NoP -Enc %s", encodedScript)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := execCommandContext(ctx, "schtasks", "/Create",
		"/TN", TaskName,
		"/TR", trValue,
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
