//go:build !windows

package harness

import (
	"os/exec"
	"syscall"
)

// setProcessGroup makes the child its own process group so a cancel can
// kill grandchildren too, instead of leaving them holding the stdout pipe.
func setProcessGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

// killProcessGroup kills the whole group; the negative pid is what
// signals the group rather than the single child.
func killProcessGroup(cmd *exec.Cmd) error {
	if cmd.Process == nil {
		return nil
	}
	return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
}
