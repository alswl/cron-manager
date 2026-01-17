package exporter

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/afero"
)

// newTestExporter creates a new Exporter instance with test filesystem
func newTestExporter(fs afero.Fs) *Exporter {
	return NewExporter(WithFileSystem(fs))
}

// TestGetExporterPath tests the GetExporterPath function
func TestGetExporterPath(t *testing.T) {
	tests := []struct {
		name           string
		customDir      string
		customFilename string
		envVar         string
		envExists      bool
		expectedSuffix string
	}{
		{
			name:           "with custom directory and filename",
			customDir:      "/custom/dir",
			customFilename: "custom.prom",
			envVar:         "/env/path",
			envExists:      true,
			expectedSuffix: "/custom/dir/custom.prom",
		},
		{
			name:           "with custom directory (highest priority)",
			customDir:      "/custom/dir",
			customFilename: "",
			envVar:         "/env/path",
			envExists:      true,
			expectedSuffix: "/custom/dir/crons.prom",
		},
		{
			name:           "with custom filename only",
			customDir:      "",
			customFilename: "my-metrics.prom",
			envVar:         "/custom/path",
			envExists:      true,
			expectedSuffix: "/custom/path/my-metrics.prom",
		},
		{
			name:           "with COLLECTOR_TEXTFILE_PATH env var",
			customDir:      "",
			customFilename: "",
			envVar:         "/custom/path",
			envExists:      true,
			expectedSuffix: "/custom/path/crons.prom",
		},
		{
			name:           "without COLLECTOR_TEXTFILE_PATH env var",
			customDir:      "",
			customFilename: "",
			envVar:         "",
			envExists:      false,
			expectedSuffix: "/var/lib/prometheus/node-exporter/crons.prom",
		},
		{
			name:           "with empty COLLECTOR_TEXTFILE_PATH env var (should use default)",
			customDir:      "",
			customFilename: "",
			envVar:         "",
			envExists:      true,
			expectedSuffix: "/var/lib/prometheus/node-exporter/crons.prom",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Save original env var
			originalValue, originalExists := os.LookupEnv("COLLECTOR_TEXTFILE_PATH")
			defer func() {
				if originalExists {
					_ = os.Setenv("COLLECTOR_TEXTFILE_PATH", originalValue)
				} else {
					_ = os.Unsetenv("COLLECTOR_TEXTFILE_PATH")
				}
			}()

			// Build exporter options
			var opts []Option
			if tt.customDir != "" {
				opts = append(opts, WithExporterDir(tt.customDir))
			}
			if tt.customFilename != "" {
				opts = append(opts, WithExporterFilename(tt.customFilename))
			}

			// Create exporter instance
			exp := NewExporter(opts...)
			// First unset the env var to ensure clean state
			_ = os.Unsetenv("COLLECTOR_TEXTFILE_PATH")

			// Then set it according to test case
			if tt.envExists {
				if tt.envVar != "" {
					_ = os.Setenv("COLLECTOR_TEXTFILE_PATH", tt.envVar)
				} else {
					// For empty env var test, we need to set it to empty string
					// but GetExporterPath checks for exists && != "", so empty string should use default
					_ = os.Setenv("COLLECTOR_TEXTFILE_PATH", "")
				}
			}

			// Test the function
			result := exp.GetExporterPath()
			if !strings.HasSuffix(result, tt.expectedSuffix) {
				t.Errorf("GetExporterPath() = %v, want suffix %v", result, tt.expectedSuffix)
			}
		})
	}
}

// TestSetMetricName tests the SetMetricName function
func TestSetMetricName(t *testing.T) {
	memFs := afero.NewMemMapFs()
	exp := NewExporter(
		WithFileSystem(memFs),
		WithExporterDir("/tmp/test_exporter"),
		WithMetricName("custom_metric"),
	)

	exp.WriteToExporter("test_job", "run", "1")

	exporterPath := exp.GetExporterPath()
	content, err := afero.ReadFile(memFs, exporterPath)
	if err != nil {
		t.Fatalf("Failed to read exporter file: %v", err)
	}

	contentStr := string(content)
	if !strings.Contains(contentStr, "# TYPE custom_metric gauge") {
		t.Errorf("Expected metric type 'custom_metric', got: %s", contentStr)
	}
	if !strings.Contains(contentStr, `custom_metric{name="test_job",dimension="run"} 1`) {
		t.Errorf("Expected metric name 'custom_metric', got: %s", contentStr)
	}
}

// TestDisableMetric tests the DisableMetric function
func TestDisableMetric(t *testing.T) {
	memFs := afero.NewMemMapFs()
	exp := NewExporter(
		WithFileSystem(memFs),
		WithExporterDir("/tmp/test_exporter"),
		WithMetricDisabled(true),
	)

	exp.WriteToExporter("test_job", "run", "1")

	exporterPath := exp.GetExporterPath()
	exists, _ := afero.Exists(memFs, exporterPath)
	if exists {
		content, _ := afero.ReadFile(memFs, exporterPath)
		if len(content) > 0 {
			t.Errorf("Metric writing should be disabled, but file contains: %s", string(content))
		}
	}
}

// TestWriteToExporter tests the WriteToExporter function
func TestWriteToExporter(t *testing.T) {
	// Use in-memory filesystem for testing
	memFs := afero.NewMemMapFs()
	exp := newTestExporter(memFs)

	// Use virtual path for testing
	tmpDir := "/test/path"
	originalEnv, envExists := os.LookupEnv("COLLECTOR_TEXTFILE_PATH")
	_ = os.Setenv("COLLECTOR_TEXTFILE_PATH", tmpDir)
	defer func() {
		if envExists {
			_ = os.Setenv("COLLECTOR_TEXTFILE_PATH", originalEnv)
		} else {
			_ = os.Unsetenv("COLLECTOR_TEXTFILE_PATH")
		}
	}()

	exporterPath := filepath.Join(tmpDir, "crons.prom")

	tests := []struct {
		name           string
		jobName        string
		label          string
		metric         string
		initialContent string
		expectedLines  []string
	}{
		{
			name:           "write first metric to empty file",
			jobName:        "test_job",
			label:          "run",
			metric:         "1",
			initialContent: "",
			expectedLines: []string{
				"# TYPE crontab gauge",
				`crontab{name="test_job",dimension="run"} 1`,
			},
		},
		{
			name:    "update existing metric",
			jobName: "test_job",
			label:   "run",
			metric:  "0",
			initialContent: `# TYPE crontab gauge
crontab{name="test_job",dimension="run"} 1
`,
			expectedLines: []string{
				"# TYPE crontab gauge",
				`crontab{name="test_job",dimension="run"} 0`,
			},
		},
		{
			name:    "add new metric to existing file",
			jobName: "test_job",
			label:   "failed",
			metric:  "0",
			initialContent: `# TYPE crontab gauge
crontab{name="test_job",dimension="run"} 1
`,
			expectedLines: []string{
				"# TYPE crontab gauge",
				`crontab{name="test_job",dimension="run"} 1`,
				`crontab{name="test_job",dimension="failed"} 0`,
			},
		},
		{
			name:    "write metric with different job name",
			jobName: "another_job",
			label:   "run",
			metric:  "1",
			initialContent: `# TYPE crontab gauge
crontab{name="test_job",dimension="run"} 1
`,
			expectedLines: []string{
				"# TYPE crontab gauge",
				`crontab{name="test_job",dimension="run"} 1`,
				`crontab{name="another_job",dimension="run"} 1`,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clean up any existing file
			_ = memFs.Remove(exporterPath)

			// Create initial file content if needed
			if tt.initialContent != "" {
				err := afero.WriteFile(memFs, exporterPath, []byte(tt.initialContent), 0644)
				if err != nil {
					t.Fatalf("Failed to write initial content: %v", err)
				}
			}

			// Call the function
			exp.WriteToExporter(tt.jobName, tt.label, tt.metric)

			// Read and verify the result
			content, err := afero.ReadFile(memFs, exporterPath)
			if err != nil {
				t.Fatalf("Failed to read exporter file: %v", err)
			}

			contentStr := string(content)
			lines := strings.Split(strings.TrimSpace(contentStr), "\n")

			// Verify TYPE header exists
			foundType := false
			for _, line := range lines {
				if strings.Contains(line, "# TYPE crontab gauge") {
					foundType = true
					break
				}
			}
			if !foundType {
				t.Errorf("Expected TYPE header not found in content: %s", contentStr)
			}

			// Verify expected lines exist
			for _, expectedLine := range tt.expectedLines {
				found := false
				for _, line := range lines {
					if strings.TrimSpace(line) == expectedLine {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected line not found: %s\nContent was:\n%s", expectedLine, contentStr)
				}
			}

			// Verify file exists (permissions check not applicable for MemMapFs)
			exists, err := afero.Exists(memFs, exporterPath)
			if err != nil || !exists {
				t.Errorf("File should exist at %s", exporterPath)
			}
		})
	}
}

// TestWriteToExporterFileCreation tests that WriteToExporter creates file if it doesn't exist
func TestWriteToExporterFileCreation(t *testing.T) {
	// Use in-memory filesystem for testing
	memFs := afero.NewMemMapFs()
	exp := newTestExporter(memFs)

	// Use virtual path for testing
	tmpDir := "/test/path"
	exporterPath := filepath.Join(tmpDir, "crons.prom")

	_ = os.Setenv("COLLECTOR_TEXTFILE_PATH", tmpDir)
	defer func() { _ = os.Unsetenv("COLLECTOR_TEXTFILE_PATH") }()

	// Verify file doesn't exist
	exists, _ := afero.Exists(memFs, exporterPath)
	if exists {
		t.Fatalf("File should not exist before test")
	}

	// Call the function
	exp.WriteToExporter("test_job", "run", "1")

	// Verify file was created
	exists, err := afero.Exists(memFs, exporterPath)
	if err != nil || !exists {
		t.Fatalf("File should be created: %v", err)
	}

	// Verify content
	content, err := afero.ReadFile(memFs, exporterPath)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	contentStr := string(content)
	if !strings.Contains(contentStr, "# TYPE crontab gauge") {
		t.Errorf("Content should contain TYPE header: %s", contentStr)
	}
	if !strings.Contains(contentStr, `crontab{name="test_job",dimension="run"} 1`) {
		t.Errorf("Content should contain metric: %s", contentStr)
	}
}

// TestWriteToExporterConcurrentWrites tests concurrent writes (basic test)
func TestWriteToExporterConcurrentWrites(t *testing.T) {
	// Use in-memory filesystem for testing
	memFs := afero.NewMemMapFs()
	exp := newTestExporter(memFs)

	// Use virtual path for testing
	tmpDir := "/test/path"
	exporterPath := filepath.Join(tmpDir, "crons.prom")

	_ = os.Setenv("COLLECTOR_TEXTFILE_PATH", tmpDir)
	defer func() { _ = os.Unsetenv("COLLECTOR_TEXTFILE_PATH") }()

	// Write multiple metrics concurrently
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func() {
			exp.WriteToExporter("test_job", "run", "1")
			exp.WriteToExporter("test_job", "failed", "0")
			exp.WriteToExporter("test_job", "duration", "100")
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// Verify file exists and has content
	content, err := afero.ReadFile(memFs, exporterPath)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	contentStr := string(content)
	if !strings.Contains(contentStr, "# TYPE crontab gauge") {
		t.Errorf("Content should contain TYPE header: %s", contentStr)
	}
}

// TestWriteToExporterRegexMatching tests regex matching in WriteToExporter
func TestWriteToExporterRegexMatching(t *testing.T) {
	// Use in-memory filesystem for testing
	memFs := afero.NewMemMapFs()
	exp := newTestExporter(memFs)

	// Use virtual path for testing
	tmpDir := "/test/path"
	exporterPath := filepath.Join(tmpDir, "crons.prom")

	_ = os.Setenv("COLLECTOR_TEXTFILE_PATH", tmpDir)
	defer func() { _ = os.Unsetenv("COLLECTOR_TEXTFILE_PATH") }()

	// Test with special characters in job name
	specialJobName := "test-job_with.special_chars"
	exp.WriteToExporter(specialJobName, "run", "1")

	content, err := afero.ReadFile(memFs, exporterPath)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	contentStr := string(content)
	expectedLine := `crontab{name="test-job_with.special_chars",dimension="run"} 1`
	if !strings.Contains(contentStr, expectedLine) {
		t.Errorf("Content should contain special job name: %s\nGot: %s", expectedLine, contentStr)
	}
}

// TestWriteToExporterMultipleJobs tests multiple jobs in same file
func TestWriteToExporterMultipleJobs(t *testing.T) {
	// Use in-memory filesystem for testing
	memFs := afero.NewMemMapFs()
	exp := newTestExporter(memFs)

	// Use virtual path for testing
	tmpDir := "/test/path"
	exporterPath := filepath.Join(tmpDir, "crons.prom")

	_ = os.Setenv("COLLECTOR_TEXTFILE_PATH", tmpDir)
	defer func() { _ = os.Unsetenv("COLLECTOR_TEXTFILE_PATH") }()

	// Write metrics for multiple jobs
	exp.WriteToExporter("job1", "run", "1")
	exp.WriteToExporter("job2", "run", "1")
	exp.WriteToExporter("job1", "failed", "0")
	exp.WriteToExporter("job2", "failed", "0")

	content, err := afero.ReadFile(memFs, exporterPath)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	contentStr := string(content)

	// Verify both jobs are present
	if !strings.Contains(contentStr, `crontab{name="job1",dimension="run"}`) {
		t.Errorf("Content should contain job1 run metric: %s", contentStr)
	}
	if !strings.Contains(contentStr, `crontab{name="job2",dimension="run"}`) {
		t.Errorf("Content should contain job2 run metric: %s", contentStr)
	}
	if !strings.Contains(contentStr, `crontab{name="job1",dimension="failed"}`) {
		t.Errorf("Content should contain job1 failed metric: %s", contentStr)
	}
	if !strings.Contains(contentStr, `crontab{name="job2",dimension="failed"}`) {
		t.Errorf("Content should contain job2 failed metric: %s", contentStr)
	}

	// Verify only one TYPE header
	typeCount := strings.Count(contentStr, "# TYPE crontab gauge")
	if typeCount != 1 {
		t.Errorf("Should have exactly one TYPE header, got %d", typeCount)
	}
}
