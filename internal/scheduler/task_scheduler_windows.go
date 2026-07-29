//go:build windows

package scheduler

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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

// SyncStudyStartTask removes any scheduled task if it exists (feature removed).
func SyncStudyStartTask(startTime string, enabled bool) error {
	vbsPath := filepath.Join(os.Getenv("APPDATA"), "Studyloop", "reminder.vbs")
	var vbsErr error
	if err := os.Remove(vbsPath); err != nil && !os.IsNotExist(err) {
		vbsErr = fmt.Errorf("failed to remove reminder.vbs: %w", err)
	}
	taskErr := RemoveStudyStartTask()
	if vbsErr != nil {
		return vbsErr
	}
	return taskErr
}
