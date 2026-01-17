package fslock

import (
	"github.com/gofrs/flock"
)

// Locker is the file locking interface
type Locker interface {
	Lock() error
	Unlock() error
}

// fsLocker uses real file system locking
type fsLocker struct {
	lock *flock.Flock
}

func (f *fsLocker) Lock() error {
	_, err := f.lock.TryLock()
	if err != nil {
		return err
	}
	// If TryLock fails to acquire, use blocking Lock
	if !f.lock.Locked() {
		return f.lock.Lock()
	}
	return nil
}

func (f *fsLocker) Unlock() error {
	return f.lock.Unlock()
}

// NewLocker creates a locker. osLock true uses file system lock, false uses memory lock (for testing)
func NewLocker(path string, osLock bool) Locker {
	if osLock {
		return &fsLocker{lock: flock.New(path + ".lock")}
	}
	return newMemLocker(path)
}
