package fslock

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestFsLockerBasic tests basic file locking with flock
func TestFsLockerBasic(t *testing.T) {
	tmpDir := t.TempDir()
	lockPath := filepath.Join(tmpDir, "test.lock")

	locker := NewLocker(lockPath, true)

	// Test Lock
	err := locker.Lock()
	if err != nil {
		t.Fatalf("Failed to acquire lock: %v", err)
	}

	// Test Unlock
	err = locker.Unlock()
	if err != nil {
		t.Fatalf("Failed to release lock: %v", err)
	}
}

// TestFsLockerConcurrent tests concurrent locking
func TestFsLockerConcurrent(t *testing.T) {
	tmpDir := t.TempDir()
	lockPath := filepath.Join(tmpDir, "concurrent.lock")

	// First locker acquires the lock
	locker1 := NewLocker(lockPath, true)
	err := locker1.Lock()
	if err != nil {
		t.Fatalf("Locker1 failed to acquire lock: %v", err)
	}

	// Second locker tries to acquire (should be blocked)
	locker2 := NewLocker(lockPath, true)

	locked := false
	done := make(chan bool)

	go func() {
		err := locker2.Lock()
		if err != nil {
			t.Errorf("Locker2 failed to acquire lock: %v", err)
		}
		locked = true
		_ = locker2.Unlock()
		done <- true
	}()

	// Give it a moment to try locking
	time.Sleep(100 * time.Millisecond)

	// Should not be locked yet
	if locked {
		t.Error("Locker2 should not acquire lock while locker1 holds it")
	}

	// Release first lock
	err = locker1.Unlock()
	if err != nil {
		t.Fatalf("Locker1 failed to release lock: %v", err)
	}

	// Wait for second locker to complete
	select {
	case <-done:
		if !locked {
			t.Error("Locker2 should have acquired lock after locker1 released")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Timeout waiting for locker2 to acquire lock")
	}
}

// TestFsLockerWithRealFile tests locking on actual file operations
func TestFsLockerWithRealFile(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "data.txt")

	locker := NewLocker(filePath, true)

	// Lock before writing
	err := locker.Lock()
	if err != nil {
		t.Fatalf("Failed to lock: %v", err)
	}

	// Write to file
	err = os.WriteFile(filePath, []byte("test data"), 0644)
	if err != nil {
		t.Fatalf("Failed to write file: %v", err)
	}

	// Unlock
	err = locker.Unlock()
	if err != nil {
		t.Fatalf("Failed to unlock: %v", err)
	}

	// Verify lock file was created
	lockFile := filePath + ".lock"
	if _, err := os.Stat(lockFile); err != nil {
		t.Fatalf("Expected lock file to exist: %v", err)
	}
}

// TestMemLockerVsFsLocker tests that memory locker and fs locker work differently
func TestMemLockerVsFsLocker(t *testing.T) {
	tmpDir := t.TempDir()
	lockPath := filepath.Join(tmpDir, "test.lock")

	// Test memory locker (useOsLock = false)
	memLocker := NewLocker(lockPath, false)
	err := memLocker.Lock()
	if err != nil {
		t.Fatalf("Memory locker failed to lock: %v", err)
	}
	err = memLocker.Unlock()
	if err != nil {
		t.Fatalf("Memory locker failed to unlock: %v", err)
	}

	// Test fs locker (useOsLock = true)
	fsLocker := NewLocker(lockPath, true)
	err = fsLocker.Lock()
	if err != nil {
		t.Fatalf("FS locker failed to lock: %v", err)
	}
	err = fsLocker.Unlock()
	if err != nil {
		t.Fatalf("FS locker failed to unlock: %v", err)
	}
}
