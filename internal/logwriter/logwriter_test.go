package logwriter

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestLogWriter(t *testing.T) {
	// Create a temporary log file
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test.log")

	// Create LogWriter
	lw, err := NewLogWriter(logPath)
	if err != nil {
		t.Fatalf("Failed to create LogWriter: %v", err)
	}
	defer func() { _ = lw.Close() }()

	// Create a test command that outputs to both stdout and stderr
	cmd := exec.Command("sh", "-c", "echo 'stdout message'; echo 'stderr message' >&2")

	// Setup pipes
	if err := lw.SetupPipes(cmd); err != nil {
		t.Fatalf("Failed to setup pipes: %v", err)
	}

	// Start the command
	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start command: %v", err)
	}

	// Start copying
	lw.Start()

	// Wait for all copying to complete BEFORE cmd.Wait() — avoids race where
	// cmd.Wait() closes pipe read ends while goroutines are still reading.
	if err := lw.Wait(); err != nil {
		t.Fatalf("Failed to wait for log writer: %v", err)
	}

	// Wait for command to complete and get exit status
	if err := cmd.Wait(); err != nil {
		t.Fatalf("Command failed: %v", err)
	}

	// Read and verify log file content
	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	contentStr := string(content)
	if !strings.Contains(contentStr, "stdout message") {
		t.Errorf("Log file should contain stdout message, got: %s", contentStr)
	}
	if !strings.Contains(contentStr, "stderr message") {
		t.Errorf("Log file should contain stderr message, got: %s", contentStr)
	}
}

// TestLogWriterNoRace runs many iterations with large output to stress-test
// the Wait ordering. The bug would manifest as:
//
//	Error copying stdout: read |0: file already closed
func TestLogWriterNoRace(t *testing.T) {
	// Generate a shell command that produces substantial output quickly
	script := "i=0; while [ $i -lt 1000 ]; do echo \"stdout line $i\"; i=$((i+1)); done; echo 'stderr output' >&2"

	for i := range 100 {
		tmpDir := t.TempDir()
		logPath := filepath.Join(tmpDir, "test.log")

		lw, err := NewLogWriter(logPath)
		if err != nil {
			t.Fatalf("iteration %d: failed to create LogWriter: %v", i, err)
		}

		cmd := exec.Command("sh", "-c", script)

		if err := lw.SetupPipes(cmd); err != nil {
			lw.Close()
			t.Fatalf("iteration %d: failed to setup pipes: %v", i, err)
		}

		if err := cmd.Start(); err != nil {
			lw.Close()
			t.Fatalf("iteration %d: failed to start command: %v", i, err)
		}

		lw.Start()

		// The fix: wait for copies BEFORE cmd.Wait()
		if err := lw.Wait(); err != nil {
			lw.Close()
			t.Fatalf("iteration %d: log writer wait failed: %v", i, err)
		}

		if err := cmd.Wait(); err != nil {
			lw.Close()
			t.Fatalf("iteration %d: command failed: %v", i, err)
		}

		// Verify output is complete (no data loss)
		content, err := os.ReadFile(logPath)
		lw.Close()
		if err != nil {
			t.Fatalf("iteration %d: failed to read log file: %v", i, err)
		}

		contentStr := string(content)
		if !strings.Contains(contentStr, "stdout line 0") {
			t.Errorf("iteration %d: missing stdout start", i)
		}
		if !strings.Contains(contentStr, "stdout line 999") {
			t.Errorf("iteration %d: missing stdout end", i)
		}
		if !strings.Contains(contentStr, "stderr output") {
			t.Errorf("iteration %d: missing stderr", i)
		}
	}
}
