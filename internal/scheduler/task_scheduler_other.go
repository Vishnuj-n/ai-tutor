//go:build !windows

package scheduler

// RemoveStudyStartTask is a no-op on non-Windows platforms.
func RemoveStudyStartTask() error {
	return nil
}

// SyncStudyStartTask is a no-op on non-Windows platforms.
func SyncStudyStartTask(startTime string, enabled bool) error {
	return nil
}
