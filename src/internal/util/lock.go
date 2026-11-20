package util

import (
	"fmt"
	"os"
	"syscall"
)

// Lock is an exclusive advisory file lock (flock)
type Lock struct {
	file *os.File
}

// AcquireLock takes an exclusive lock on path, waiting for other holders.
// It serializes operations on the same app (e.g. two concurrent git pushes).
func AcquireLock(path string) (*Lock, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, fmt.Errorf("error opening lock file: %w", err)
	}
	_ = FixFileOwnership(path)

	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		fmt.Println("Waiting for another Portico operation on this app to finish...")
		if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
			_ = f.Close()
			return nil, fmt.Errorf("error acquiring lock: %w", err)
		}
	}
	return &Lock{file: f}, nil
}

// Release releases the lock
func (l *Lock) Release() {
	if l == nil || l.file == nil {
		return
	}
	_ = syscall.Flock(int(l.file.Fd()), syscall.LOCK_UN)
	_ = l.file.Close()
}
