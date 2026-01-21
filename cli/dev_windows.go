//go:build windows

package cli

import (
	"os/exec"
)

// setProcAttr sets Windows-specific process attributes (no-op on Windows)
func setProcAttr(cmd *exec.Cmd) {
	// No process group handling on Windows
}

// killProcess kills the process on Windows
func killProcess(cmd *exec.Cmd) {
	if cmd.Process != nil {
		cmd.Process.Kill()
	}
}
