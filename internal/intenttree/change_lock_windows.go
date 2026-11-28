package intenttree

import (
	"os"

	"golang.org/x/sys/windows"
)

func lockTreeChange(dir string) (func(), error) {
	lockDir := lockDirForTree(dir)
	if err := os.MkdirAll(lockDir, 0o755); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(lockPathForTree(dir), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	o := &windows.Overlapped{}
	if err := windows.LockFileEx(windows.Handle(f.Fd()), windows.LOCKFILE_EXCLUSIVE_LOCK, 0, 1, 0, o); err != nil {
		_ = f.Close()
		return nil, err
	}
	return func() { _ = windows.UnlockFileEx(windows.Handle(f.Fd()), 0, 1, 0, o); _ = f.Close() }, nil
}
