package job

import (
	"testing"
	"time"
)

// TestIdleWait tests the IdleWait function
func TestIdleWait(t *testing.T) {
	t.Run("idle seconds is 0, should not wait", func(t *testing.T) {
		start := time.Now()
		IdleWait(start, 0)
		elapsed := time.Since(start)
		if elapsed > 100*time.Millisecond {
			t.Errorf("IdleWait with 0 seconds should not wait, but waited %v", elapsed)
		}
	})

	t.Run("idle seconds is negative, should not wait", func(t *testing.T) {
		start := time.Now()
		IdleWait(start, -10)
		elapsed := time.Since(start)
		if elapsed > 100*time.Millisecond {
			t.Errorf("IdleWait with negative seconds should not wait, but waited %v", elapsed)
		}
	})

	t.Run("idle seconds is positive, should wait remaining time", func(t *testing.T) {
		start := time.Now()
		// Wait a bit first
		time.Sleep(50 * time.Millisecond)
		IdleWait(start, 1) // 1 second total
		elapsed := time.Since(start)
		// Should have waited approximately 950ms (1 second - 50ms already elapsed)
		if elapsed < 900*time.Millisecond || elapsed > 1100*time.Millisecond {
			t.Errorf("IdleWait should wait approximately 950ms, but elapsed %v", elapsed)
		}
	})

	t.Run("job already ran longer than idle seconds, should not wait", func(t *testing.T) {
		start := time.Now().Add(-2 * time.Second) // Job started 2 seconds ago
		beforeWait := time.Now()
		IdleWait(start, 1) // Only need 1 second, but job already ran 2 seconds
		waitDuration := time.Since(beforeWait)
		// Should not wait since job already ran for 2 seconds (longer than 1 second required)
		if waitDuration > 100*time.Millisecond {
			t.Errorf("IdleWait should not wait if job already ran longer, but waited %v", waitDuration)
		}
	})
}
