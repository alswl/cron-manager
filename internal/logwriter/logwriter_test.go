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

	// Wait for command to complete
	if err := cmd.Wait(); err != nil {
		t.Fatalf("Command failed: %v", err)
	}

	// Wait for all copying to complete
	if err := lw.Wait(); err != nil {
		t.Fatalf("Failed to wait for log writer: %v", err)
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
