//go:build windows

package harness

import "os/exec"

// setProcessGroup is a no-op on Windows: syscall.SysProcAttr has no
// Setpgid field there and there is no process group to signal.
func setProcessGroup(cmd *exec.Cmd) {}

// killProcessGroup kills the direct child only; grandchildren are not
// addressed because Windows has no group signal here.
func killProcessGroup(cmd *exec.Cmd) error {
	if cmd.Process == nil {
		return nil
	}
	return cmd.Process.Kill()
}
