package orchestrator

import (
	"os"
	"path/filepath"

	"golang.org/x/sys/windows"
)

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
	o := &windows.Overlapped{}
	if err := windows.LockFileEx(windows.Handle(f.Fd()), windows.LOCKFILE_EXCLUSIVE_LOCK, 0, 1, 0, o); err != nil {
		f.Close()
		return nil, err
	}
	return func() { _ = windows.UnlockFileEx(windows.Handle(f.Fd()), 0, 1, 0, o); _ = f.Close() }, nil
}
