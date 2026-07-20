//go:build windows

package notebook

import (
	"os/exec"
	"syscall"
)

const _CREATE_NO_WINDOW = 0x08000000

// hideConsoleWindow prevents the child process from opening a visible console window on Windows.
func hideConsoleWindow(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.CreationFlags = _CREATE_NO_WINDOW
}
