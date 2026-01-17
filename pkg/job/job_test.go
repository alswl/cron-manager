package job

import (
	"testing"
)

// TestConstants tests that constants are set correctly
func TestConstants(t *testing.T) {
	if IdleForSeconds != 60 {
		t.Errorf("IdleForSeconds = %v, want 60", IdleForSeconds)
	}
}
