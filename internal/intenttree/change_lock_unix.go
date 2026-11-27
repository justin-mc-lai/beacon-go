//go:build !windows

package intenttree

import (
	"os"
	"syscall"
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
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		_ = f.Close()
		return nil, err
	}
	return func() { _ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN); _ = f.Close() }, nil
}
