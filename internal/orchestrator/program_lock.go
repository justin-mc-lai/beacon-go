//go:build !windows

package orchestrator

import (
	"os"
	"path/filepath"
	"syscall"
)

// Lock the project target, not an Engine instance: separate CLI processes
// must reload the host snapshot only AFTER acquiring the same lock.
func (e *Engine) lockProgram(runID string) (func(), error) {
	run, _, _, err := e.loadRun(runID)
	if err != nil {
		return nil, err
	}
	root, err := e.runProjectRoot(run)
	if err != nil {
		return nil, err
	}
	dir := filepath.Join(root, ".beacon")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(filepath.Join(dir, "program.lock"), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		f.Close()
		return nil, err
	}
	return func() { _ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN); _ = f.Close() }, nil
}
