package logwriter

import (
	"bufio"
	"io"
	"log"
	"os"
	"os/exec"
	"sync"
)

// LogWriter handles concurrent writing of stdout and stderr to a log file
type LogWriter struct {
	file       *os.File
	writer     *bufio.Writer
	mu         sync.Mutex
	wg         sync.WaitGroup
	stdoutPipe io.ReadCloser
	stderrPipe io.ReadCloser
}

// NewLogWriter creates a new LogWriter that writes to the specified log file
func NewLogWriter(logPath string) (*LogWriter, error) {
	file, err := os.Create(logPath)
	if err != nil {
		return nil, err
	}

	return &LogWriter{
		file:   file,
		writer: bufio.NewWriter(file),
	}, nil
}

// SetupPipes sets up stdout and stderr pipes for the command
func (lw *LogWriter) SetupPipes(cmd *exec.Cmd) error {
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}

	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return err
	}

	lw.stdoutPipe = stdoutPipe
	lw.stderrPipe = stderrPipe
	return nil
}

// Start begins copying stdout and stderr to the log file concurrently
func (lw *LogWriter) Start() {
	// Copy stdout to log file
	lw.wg.Add(1)
	go func() {
		defer lw.wg.Done()
		if _, err := io.Copy(lw, lw.stdoutPipe); err != nil {
			log.Printf("Error copying stdout: %v", err)
		}
	}()

	// Copy stderr to log file
	lw.wg.Add(1)
	go func() {
		defer lw.wg.Done()
		if _, err := io.Copy(lw, lw.stderrPipe); err != nil {
			log.Printf("Error copying stderr: %v", err)
		}
	}()
}

// Wait waits for all copying operations to complete and flushes the buffer
func (lw *LogWriter) Wait() error {
	lw.wg.Wait()

	lw.mu.Lock()
	defer lw.mu.Unlock()

	return lw.writer.Flush()
}

// Close closes the log file
func (lw *LogWriter) Close() error {
	return lw.file.Close()
}

// Write implements io.Writer interface with thread-safe access
func (lw *LogWriter) Write(p []byte) (n int, err error) {
	lw.mu.Lock()
	defer lw.mu.Unlock()
	return lw.writer.Write(p)
}
