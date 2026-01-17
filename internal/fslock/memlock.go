package fslock

import (
	"sync"
)

// memLocker uses memory lock for testing
type memLocker struct {
	mu *sync.Mutex
}

func (m *memLocker) Lock() error {
	m.mu.Lock()
	return nil
}

func (m *memLocker) Unlock() error {
	m.mu.Unlock()
	return nil
}

var (
	// memLockers stores mutexes for testing, ensuring same path shares the same mutex
	memLockers   map[string]*sync.Mutex
	memLockersMu sync.Mutex
)

func init() {
	memLockers = make(map[string]*sync.Mutex)
}

// newMemLocker creates a memory locker (internal use)
func newMemLocker(path string) Locker {
	memLockersMu.Lock()
	defer memLockersMu.Unlock()
	if lock, exists := memLockers[path]; exists {
		return &memLocker{mu: lock}
	}
	lock := &sync.Mutex{}
	memLockers[path] = lock
	return &memLocker{mu: lock}
}

// ResetMemLockers resets memory locks (testing only)
func ResetMemLockers() {
	memLockersMu.Lock()
	defer memLockersMu.Unlock()
	memLockers = make(map[string]*sync.Mutex)
}
