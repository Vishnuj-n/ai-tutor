//go:build !windows

package notebook

import "os/exec"

// hideConsoleWindow is a no-op on non-Windows platforms.
func hideConsoleWindow(cmd *exec.Cmd) {}
