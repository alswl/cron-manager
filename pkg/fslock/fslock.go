package fslock

import (
	"github.com/juju/fslock"
)

// Locker is the file locking interface
type Locker interface {
	Lock() error
	Unlock() error
}

// fsLocker uses real file system locking
type fsLocker struct {
	lock *fslock.Lock
}

func (f *fsLocker) Lock() error {
	return f.lock.Lock()
}

func (f *fsLocker) Unlock() error {
	return f.lock.Unlock()
}

// NewLocker creates a locker. osLock true uses file system lock, false uses memory lock (for testing)
func NewLocker(path string, osLock bool) Locker {
	if osLock {
		return &fsLocker{lock: fslock.New(path)}
	}
	return newMemLocker(path)
}
