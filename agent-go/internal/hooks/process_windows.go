//go:build windows

package hooks

import (
	"os"
	"os/exec"
	"syscall"
)

// setSysProcAttr is a no-op on Windows (no process groups via Setpgid).
func setSysProcAttr(_ *exec.Cmd) {}

// killProcessGroup kills the process on Windows (no negative-pid process group kill).
func killProcessGroup(pid int, _ syscall.Signal) error {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	return proc.Kill()
}
