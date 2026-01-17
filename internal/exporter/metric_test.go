package exporter

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/spf13/afero"
)

// TestEscapeLabelValue tests the escapeLabelValue function
func TestEscapeLabelValue(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "no special characters",
			input:    "simple_value",
			expected: "simple_value",
		},
		{
			name:     "with backslash",
			input:    `path\to\file`,
			expected: `path\\to\\file`,
		},
		{
			name:     "with quotes",
			input:    `value "with" quotes`,
			expected: `value \"with\" quotes`,
		},
		{
			name:     "with newline",
			input:    "line1\nline2",
			expected: `line1\nline2`,
		},
		{
			name:     "with all special characters",
			input:    "test\\path\n\"quoted\"",
			expected: `test\\\\path\n\"quoted\"`,
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := escapeLabelValue(tt.input)
			if result != tt.expected {
				t.Errorf("escapeLabelValue(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

// TestBuildLabelString tests the buildLabelString function
func TestBuildLabelString(t *testing.T) {
	tests := []struct {
		name     string
		jobName  string
		labels   map[string]string
		expected string
	}{
		{
			name:     "only job name",
			jobName:  "test_job",
			labels:   nil,
			expected: `name="test_job"`,
		},
		{
			name:    "job name with single label",
			jobName: "test_job",
			labels: map[string]string{
				"status": "success",
			},
			expected: `name="test_job",status="success"`,
		},
		{
			name:    "job name with multiple labels",
			jobName: "test_job",
			labels: map[string]string{
				"status": "success",
				"env":    "production",
			},
			// Note: map iteration order is not guaranteed, but we can check both are present
		},
		{
			name:    "job name with special characters",
			jobName: `job"with\quotes`,
			labels: map[string]string{
				"key": `value"test`,
			},
			expected: `name="job\"with\\quotes",key="value\"test"`,
		},
		{
			name:     "empty job name",
			jobName:  "",
			labels:   nil,
			expected: `name=""`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildLabelString(tt.jobName, tt.labels)

			// For tests with multiple labels, we need to check differently
			// since map iteration order is not guaranteed
			if tt.name == "job name with multiple labels" {
				if !strings.Contains(result, `name="test_job"`) {
					t.Errorf("buildLabelString() missing job name, got %q", result)
				}
				if !strings.Contains(result, `status="success"`) {
					t.Errorf("buildLabelString() missing status label, got %q", result)
				}
				if !strings.Contains(result, `env="production"`) {
					t.Errorf("buildLabelString() missing env label, got %q", result)
				}
			} else {
				if result != tt.expected {
					t.Errorf("buildLabelString(%q, %v) = %q, want %q", tt.jobName, tt.labels, result, tt.expected)
				}
			}
		})
	}
}

// TestEnsureDirectoryExists tests the ensureDirectoryExists function
func TestEnsureDirectoryExists(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		wantDir string
	}{
		{
			name:    "create nested directory",
			path:    "/test/path/to/file.txt",
			wantDir: "/test/path/to",
		},
		{
			name:    "create single directory",
			path:    "/test/file.txt",
			wantDir: "/test",
		},
		{
			name:    "file in current directory",
			path:    "file.txt",
			wantDir: ".",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			memFs := afero.NewMemMapFs()
			writer := NewMetricWriter(memFs, false)

			writer.ensureDirectoryExists(tt.path)

			// Check if directory exists (unless it's ".")
			if tt.wantDir != "." {
				exists, err := afero.DirExists(memFs, tt.wantDir)
				if err != nil {
					t.Fatalf("Failed to check directory existence: %v", err)
				}
				if !exists {
					t.Errorf("Directory %q should exist after ensureDirectoryExists()", tt.wantDir)
				}
			}
		})
	}
}

// TestAddMetricHeaders tests the addMetricHeaders function
func TestAddMetricHeaders(t *testing.T) {
	tests := []struct {
		name           string
		content        []byte
		fullMetricName string
		metricType     MetricType
		help           string
		wantHelp       bool
		wantType       bool
	}{
		{
			name:           "add headers to empty content",
			content:        []byte{},
			fullMetricName: "test_metric",
			metricType:     MetricTypeGauge,
			help:           "Test metric help",
			wantHelp:       true,
			wantType:       true,
		},
		{
			name:           "add headers to existing content",
			content:        []byte("existing content\n"),
			fullMetricName: "test_metric",
			metricType:     MetricTypeCounter,
			help:           "Test counter help",
			wantHelp:       true,
			wantType:       true,
		},
		{
			name:           "skip existing HELP header",
			content:        []byte("# HELP test_metric Test metric help\n"),
			fullMetricName: "test_metric",
			metricType:     MetricTypeGauge,
			help:           "Test metric help",
			wantHelp:       false, // Should not duplicate
			wantType:       true,
		},
		{
			name:           "skip existing TYPE header",
			content:        []byte("# TYPE test_metric gauge\n"),
			fullMetricName: "test_metric",
			metricType:     MetricTypeGauge,
			help:           "Test metric help",
			wantHelp:       true,
			wantType:       false, // Should not duplicate
		},
		{
			name:           "skip both existing headers",
			content:        []byte("# HELP test_metric Test metric help\n# TYPE test_metric gauge\n"),
			fullMetricName: "test_metric",
			metricType:     MetricTypeGauge,
			help:           "Test metric help",
			wantHelp:       false,
			wantType:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			originalLen := len(tt.content)
			result := addMetricHeaders(tt.content, tt.fullMetricName, tt.metricType, tt.help)

			resultStr := string(result)
			expectedHelp := "# HELP " + tt.fullMetricName + " " + tt.help
			expectedType := "# TYPE " + tt.fullMetricName + " " + string(tt.metricType)

			// Check HELP header
			helpCount := strings.Count(resultStr, expectedHelp)
			if tt.wantHelp {
				if helpCount != 1 {
					t.Errorf("Expected exactly 1 HELP header, got %d in:\n%s", helpCount, resultStr)
				}
			} else {
				// If we don't want to add, it should already exist (count should be 1 from original)
				if helpCount > 1 {
					t.Errorf("HELP header duplicated, got %d occurrences", helpCount)
				}
			}

			// Check TYPE header
			typeCount := strings.Count(resultStr, expectedType)
			if tt.wantType {
				if typeCount != 1 {
					t.Errorf("Expected exactly 1 TYPE header, got %d in:\n%s", typeCount, resultStr)
				}
			} else {
				// If we don't want to add, it should already exist
				if typeCount > 1 {
					t.Errorf("TYPE header duplicated, got %d occurrences", typeCount)
				}
			}

			// Result should be at least as long as original
			if len(result) < originalLen {
				t.Errorf("Result length %d is less than original %d", len(result), originalLen)
			}
		})
	}
}

// TestReadOrCreateFile tests the readOrCreateFile function
func TestReadOrCreateFile(t *testing.T) {
	tests := []struct {
		name            string
		existingFile    bool
		existingContent string
		expectedContent string
		expectError     bool
	}{
		{
			name:            "read existing file",
			existingFile:    true,
			existingContent: "existing content\n",
			expectedContent: "existing content\n",
			expectError:     false,
		},
		{
			name:            "create new file",
			existingFile:    false,
			existingContent: "",
			expectedContent: "",
			expectError:     false,
		},
		{
			name:            "read empty existing file",
			existingFile:    true,
			existingContent: "",
			expectedContent: "",
			expectError:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			memFs := afero.NewMemMapFs()
			writer := NewMetricWriter(memFs, false)
			testPath := "/test/file.txt"

			// Create directory
			_ = memFs.MkdirAll(filepath.Dir(testPath), 0755)

			// Create existing file if needed
			if tt.existingFile {
				err := afero.WriteFile(memFs, testPath, []byte(tt.existingContent), 0644)
				if err != nil {
					t.Fatalf("Failed to create test file: %v", err)
				}
			}

			// Test the function
			content, err := writer.readOrCreateFile(testPath)

			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			if string(content) != tt.expectedContent {
				t.Errorf("readOrCreateFile() = %q, want %q", string(content), tt.expectedContent)
			}

			// Verify file exists after operation
			exists, _ := afero.Exists(memFs, testPath)
			if !exists {
				t.Error("File should exist after readOrCreateFile()")
			}
		})
	}
}

// TestIncrementValue tests the incrementValue function
func TestIncrementValue(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "increment integer",
			input:    "42",
			expected: "43",
		},
		{
			name:     "increment zero",
			input:    "0",
			expected: "1",
		},
		{
			name:     "increment float",
			input:    "3.14",
			expected: "4.14",
		},
		{
			name:     "increment float with trailing zeros",
			input:    "10.00",
			expected: "11.00",
		},
		{
			name:     "increment large integer",
			input:    "999999",
			expected: "1000000",
		},
		{
			name:     "invalid input returns 1",
			input:    "invalid",
			expected: "1",
		},
		{
			name:     "empty string returns 1",
			input:    "",
			expected: "1",
		},
		{
			name:     "negative integer",
			input:    "-5",
			expected: "-4",
		},
		{
			name:     "negative float",
			input:    "-2.50",
			expected: "-1.50",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			memFs := afero.NewMemMapFs()
			writer := NewMetricWriter(memFs, false)

			result := writer.incrementValue(tt.input)
			if result != tt.expected {
				t.Errorf("incrementValue(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

// TestMetricWriterWriteMetric tests the WriteMetric function with helper functions
func TestMetricWriterWriteMetric(t *testing.T) {
	memFs := afero.NewMemMapFs()
	writer := NewMetricWriter(memFs, false)
	testPath := "/test/metrics.prom"

	// Write first metric
	writer.WriteMetric(testPath, "test_metric", MetricTypeGauge, "job1", map[string]string{"status": "success"}, "100", "Test metric")

	// Verify file was created
	exists, err := afero.Exists(memFs, testPath)
	if err != nil || !exists {
		t.Fatalf("File should exist after WriteMetric()")
	}

	// Read and verify content
	content, err := afero.ReadFile(memFs, testPath)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	contentStr := string(content)

	// Check for HELP header
	if !strings.Contains(contentStr, "# HELP test_metric Test metric") {
		t.Errorf("Missing HELP header in:\n%s", contentStr)
	}

	// Check for TYPE header
	if !strings.Contains(contentStr, "# TYPE test_metric gauge") {
		t.Errorf("Missing TYPE header in:\n%s", contentStr)
	}

	// Check for metric line
	if !strings.Contains(contentStr, `test_metric{name="job1",status="success"} 100`) {
		t.Errorf("Missing metric line in:\n%s", contentStr)
	}
}

// TestMetricWriterIncrementCounter tests the IncrementCounter function
func TestMetricWriterIncrementCounter(t *testing.T) {
	memFs := afero.NewMemMapFs()
	writer := NewMetricWriter(memFs, false)
	testPath := "/test/metrics.prom"

	// Increment counter (file doesn't exist yet)
	writer.IncrementCounter(testPath, "test_counter", "job1", map[string]string{"type": "success"}, "Test counter")

	// Read and verify initial value is 1
	content, err := afero.ReadFile(memFs, testPath)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	contentStr := string(content)
	if !strings.Contains(contentStr, `test_counter{name="job1",type="success"} 1`) {
		t.Errorf("Initial counter value should be 1, got:\n%s", contentStr)
	}

	// Increment again
	writer.IncrementCounter(testPath, "test_counter", "job1", map[string]string{"type": "success"}, "Test counter")

	// Read and verify value is now 2
	content, err = afero.ReadFile(memFs, testPath)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	contentStr = string(content)
	if !strings.Contains(contentStr, `test_counter{name="job1",type="success"} 2`) {
		t.Errorf("Counter value should be 2 after second increment, got:\n%s", contentStr)
	}

	// Check for counter TYPE header
	if !strings.Contains(contentStr, "# TYPE test_counter counter") {
		t.Errorf("Missing TYPE counter header in:\n%s", contentStr)
	}
}

// TestMetricWriterIncrementCounterNoDeadlock tests that IncrementCounter doesn't deadlock when file doesn't exist
func TestMetricWriterIncrementCounterNoDeadlock(t *testing.T) {
	memFs := afero.NewMemMapFs()
	writer := NewMetricWriter(memFs, false) // Use memory lock (non-reentrant)
	testPath := "/test/metrics.prom"

	// This should complete without deadlock
	done := make(chan bool, 1)
	go func() {
		writer.IncrementCounter(testPath, "test_counter", "job1", nil, "Test counter")
		done <- true
	}()

	// Wait with timeout
	select {
	case <-done:
		// Success - no deadlock
	case <-time.After(2 * time.Second):
		t.Fatal("IncrementCounter deadlocked when file doesn't exist")
	}

	// Verify the counter was created
	content, err := afero.ReadFile(memFs, testPath)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	contentStr := string(content)
	if !strings.Contains(contentStr, `test_counter{name="job1"} 1`) {
		t.Errorf("Counter should be created with value 1, got:\n%s", contentStr)
	}
}

// TestMetricWriterConcurrentWrites tests concurrent writes using helper functions
func TestMetricWriterConcurrentWrites(t *testing.T) {
	memFs := afero.NewMemMapFs()
	writer := NewMetricWriter(memFs, false)
	testPath := "/test/metrics.prom"

	// Set COLLECTOR_TEXTFILE_PATH for testing
	tmpDir := "/test"
	_ = os.Setenv("COLLECTOR_TEXTFILE_PATH", tmpDir)
	defer func() { _ = os.Unsetenv("COLLECTOR_TEXTFILE_PATH") }()

	// Write multiple metrics concurrently
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func() {
			writer.WriteMetric(testPath, "concurrent_metric", MetricTypeGauge, "job1", nil, "100", "Concurrent test")
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// Verify file exists and has content
	content, err := afero.ReadFile(memFs, testPath)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	contentStr := string(content)
	if !strings.Contains(contentStr, "# TYPE concurrent_metric gauge") {
		t.Errorf("Content should contain TYPE header: %s", contentStr)
	}

	// Should have exactly one HELP and one TYPE header (no duplicates)
	helpCount := strings.Count(contentStr, "# HELP concurrent_metric")
	if helpCount != 1 {
		t.Errorf("Expected 1 HELP header, got %d", helpCount)
	}

	typeCount := strings.Count(contentStr, "# TYPE concurrent_metric")
	if typeCount != 1 {
		t.Errorf("Expected 1 TYPE header, got %d", typeCount)
	}
}
