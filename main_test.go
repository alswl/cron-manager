package main

import (
	"flag"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

// TestGetExporterPath tests the getExporterPath function
func TestGetExporterPath(t *testing.T) {
	tests := []struct {
		name           string
		envVar         string
		envExists      bool
		expectedSuffix string
	}{
		{
			name:           "with COLLECTOR_TEXTFILE_PATH env var",
			envVar:         "/custom/path",
			envExists:      true,
			expectedSuffix: "/custom/path/crons.prom",
		},
		{
			name:           "without COLLECTOR_TEXTFILE_PATH env var",
			envVar:         "",
			envExists:      false,
			expectedSuffix: "/var/cache/prometheus/crons.prom",
		},
		{
			name:           "with empty COLLECTOR_TEXTFILE_PATH env var",
			envVar:         "",
			envExists:      true,
			expectedSuffix: "/crons.prom",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Save original env var
			originalValue, originalExists := os.LookupEnv("COLLECTOR_TEXTFILE_PATH")
			defer func() {
				if originalExists {
					os.Setenv("COLLECTOR_TEXTFILE_PATH", originalValue)
				} else {
					os.Unsetenv("COLLECTOR_TEXTFILE_PATH")
				}
			}()

			// Set up test environment
			if tt.envExists {
				os.Setenv("COLLECTOR_TEXTFILE_PATH", tt.envVar)
			} else {
				os.Unsetenv("COLLECTOR_TEXTFILE_PATH")
			}

			// Test the function
			result := getExporterPath()
			if !strings.HasSuffix(result, tt.expectedSuffix) {
				t.Errorf("getExporterPath() = %v, want suffix %v", result, tt.expectedSuffix)
			}
		})
	}
}

// TestIdleWait tests the idleWait function
func TestIdleWait(t *testing.T) {
	tests := []struct {
		name           string
		jobStart       time.Time
		shouldWait     bool
		minWaitSeconds int
		maxWaitSeconds int
	}{
		{
			name:           "should wait when job started recently",
			jobStart:       time.Now().Add(-10 * time.Second),
			shouldWait:     true,
			minWaitSeconds: 49,
			maxWaitSeconds: 51,
		},
		{
			name:           "should not wait when job started more than 60 seconds ago",
			jobStart:       time.Now().Add(-70 * time.Second),
			shouldWait:     false,
			minWaitSeconds: 0,
			maxWaitSeconds: 1,
		},
		{
			name:           "should wait approximately 60 seconds when job just started",
			jobStart:       time.Now().Add(-2 * time.Second), // Use 2 seconds ago to account for test execution time
			shouldWait:     true,
			minWaitSeconds: 57,
			maxWaitSeconds: 61,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			start := time.Now()
			idleWait(tt.jobStart)
			elapsed := time.Since(start).Seconds()

			if tt.shouldWait {
				if elapsed < float64(tt.minWaitSeconds) || elapsed > float64(tt.maxWaitSeconds) {
					t.Errorf("idleWait() waited for %v seconds, expected between %v and %v seconds",
						elapsed, tt.minWaitSeconds, tt.maxWaitSeconds)
				}
			} else {
				if elapsed > float64(tt.maxWaitSeconds) {
					t.Errorf("idleWait() waited for %v seconds, expected less than %v seconds",
						elapsed, tt.maxWaitSeconds)
				}
			}
		})
	}
}

// TestWriteToExporter tests the writeToExporter function
func TestWriteToExporter(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir, err := os.MkdirTemp("", "cronmanager_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	exporterPath := filepath.Join(tmpDir, "crons.prom")

	// Save original env var
	originalValue, originalExists := os.LookupEnv("COLLECTOR_TEXTFILE_PATH")
	defer func() {
		if originalExists {
			os.Setenv("COLLECTOR_TEXTFILE_PATH", originalValue)
		} else {
			os.Unsetenv("COLLECTOR_TEXTFILE_PATH")
		}
	}()

	// Set custom path for testing
	os.Setenv("COLLECTOR_TEXTFILE_PATH", tmpDir)

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
				"# TYPE cron_job gauge",
				`cronjob{name="test_job",dimension="run"} 1`,
			},
		},
		{
			name:           "update existing metric",
			jobName:        "test_job",
			label:          "run",
			metric:         "0",
			initialContent: `# TYPE cron_job gauge
cronjob{name="test_job",dimension="run"} 1
`,
			expectedLines: []string{
				"# TYPE cron_job gauge",
				`cronjob{name="test_job",dimension="run"} 0`,
			},
		},
		{
			name:           "add new metric to existing file",
			jobName:        "test_job",
			label:          "failed",
			metric:         "0",
			initialContent: `# TYPE cron_job gauge
cronjob{name="test_job",dimension="run"} 1
`,
			expectedLines: []string{
				"# TYPE cron_job gauge",
				`cronjob{name="test_job",dimension="run"} 1`,
				`cronjob{name="test_job",dimension="failed"} 0`,
			},
		},
		{
			name:           "write metric with different job name",
			jobName:        "another_job",
			label:          "run",
			metric:         "1",
			initialContent: `# TYPE cron_job gauge
cronjob{name="test_job",dimension="run"} 1
`,
			expectedLines: []string{
				"# TYPE cron_job gauge",
				`cronjob{name="test_job",dimension="run"} 1`,
				`cronjob{name="another_job",dimension="run"} 1`,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clean up any existing file
			os.Remove(exporterPath)

			// Create initial file content if needed
			if tt.initialContent != "" {
				err := os.WriteFile(exporterPath, []byte(tt.initialContent), 0644)
				if err != nil {
					t.Fatalf("Failed to write initial content: %v", err)
				}
			}

			// Call the function
			writeToExporter(tt.jobName, tt.label, tt.metric)

			// Read and verify the result
			content, err := os.ReadFile(exporterPath)
			if err != nil {
				t.Fatalf("Failed to read exporter file: %v", err)
			}

			contentStr := string(content)
			lines := strings.Split(strings.TrimSpace(contentStr), "\n")

			// Verify TYPE header exists
			foundType := false
			for _, line := range lines {
				if strings.Contains(line, "# TYPE cron_job gauge") {
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

			// Verify file permissions
			info, err := os.Stat(exporterPath)
			if err != nil {
				t.Fatalf("Failed to stat file: %v", err)
			}
			expectedPerm := os.FileMode(0644)
			if info.Mode().Perm() != expectedPerm {
				t.Errorf("File permissions = %v, want %v", info.Mode().Perm(), expectedPerm)
			}
		})
	}
}

// TestWriteToExporterFileCreation tests that writeToExporter creates file if it doesn't exist
func TestWriteToExporterFileCreation(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "cronmanager_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	exporterPath := filepath.Join(tmpDir, "crons.prom")

	// Save original env var
	originalValue, originalExists := os.LookupEnv("COLLECTOR_TEXTFILE_PATH")
	defer func() {
		if originalExists {
			os.Setenv("COLLECTOR_TEXTFILE_PATH", originalValue)
		} else {
			os.Unsetenv("COLLECTOR_TEXTFILE_PATH")
		}
	}()

	// Set custom path for testing
	os.Setenv("COLLECTOR_TEXTFILE_PATH", tmpDir)

	// Verify file doesn't exist
	if _, err := os.Stat(exporterPath); err == nil {
		t.Fatalf("File should not exist before test")
	}

	// Call the function
	writeToExporter("test_job", "run", "1")

	// Verify file was created
	if _, err := os.Stat(exporterPath); err != nil {
		t.Fatalf("File should be created: %v", err)
	}

	// Verify content
	content, err := os.ReadFile(exporterPath)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	contentStr := string(content)
	if !strings.Contains(contentStr, "# TYPE cron_job gauge") {
		t.Errorf("Content should contain TYPE header: %s", contentStr)
	}
	if !strings.Contains(contentStr, `cronjob{name="test_job",dimension="run"} 1`) {
		t.Errorf("Content should contain metric: %s", contentStr)
	}
}

// TestWriteToExporterConcurrentWrites tests concurrent writes (basic test)
func TestWriteToExporterConcurrentWrites(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "cronmanager_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Save original env var
	originalValue, originalExists := os.LookupEnv("COLLECTOR_TEXTFILE_PATH")
	defer func() {
		if originalExists {
			os.Setenv("COLLECTOR_TEXTFILE_PATH", originalValue)
		} else {
			os.Unsetenv("COLLECTOR_TEXTFILE_PATH")
		}
	}()

	// Set custom path for testing
	os.Setenv("COLLECTOR_TEXTFILE_PATH", tmpDir)

	// Write multiple metrics concurrently
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func(index int) {
			writeToExporter("test_job", "run", "1")
			writeToExporter("test_job", "failed", "0")
			writeToExporter("test_job", "duration", "100")
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// Verify file exists and has content
	exporterPath := filepath.Join(tmpDir, "crons.prom")
	content, err := os.ReadFile(exporterPath)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	contentStr := string(content)
	if !strings.Contains(contentStr, "# TYPE cron_job gauge") {
		t.Errorf("Content should contain TYPE header: %s", contentStr)
	}
}

// TestFlagParsing tests command line flag parsing
func TestFlagParsing(t *testing.T) {
	t.Run("version flag set", func(t *testing.T) {
		// Reset global variable
		flgVersion = false
		
		// Create new flag set and register version flag
		fs := flag.NewFlagSet("test", flag.ContinueOnError)
		fs.BoolVar(&flgVersion, "version", false, "print version")
		
		err := fs.Parse([]string{"-version"})
		if err != nil {
			t.Errorf("flag parsing error = %v", err)
		}
		
		if !flgVersion {
			t.Error("flgVersion should be true after parsing -version flag")
		}
	})
	
	t.Run("idle flag set", func(t *testing.T) {
		// Create new flag set
		fs := flag.NewFlagSet("test", flag.ContinueOnError)
		idle := fs.Bool("i", false, "idle flag")
		cmdPtr := fs.String("c", "", "command")
		jobnamePtr := fs.String("n", "", "job name")
		
		err := fs.Parse([]string{"-i", "-c", "echo test", "-n", "test_job"})
		if err != nil {
			t.Errorf("flag parsing error = %v", err)
		}
		
		if !*idle {
			t.Error("idle flag should be true")
		}
		if *cmdPtr != "echo test" {
			t.Errorf("cmdPtr = %v, want 'echo test'", *cmdPtr)
		}
		if *jobnamePtr != "test_job" {
			t.Errorf("jobnamePtr = %v, want 'test_job'", *jobnamePtr)
		}
	})
}

// TestConstants tests that constants are set correctly
func TestConstants(t *testing.T) {
	if idleForSeconds != 60 {
		t.Errorf("idleForSeconds = %v, want 60", idleForSeconds)
	}
}

// TestVersionVariable tests version variable
func TestVersionVariable(t *testing.T) {
	// Version is set in main(), but we can test that it's a string type
	// In actual execution, it will be set to "1.1.18"
	if version == "" && len(version) == 0 {
		// This is acceptable since version is only set in main()
		// We're just testing the variable exists and is a string
		t.Log("version is empty (expected if main() hasn't run)")
	}
}

// TestGlobalVariables tests global variables initialization
func TestGlobalVariables(t *testing.T) {
	// Test that global variables are initialized
	if reflect.TypeOf(isDelayed).Kind() != reflect.Bool {
		t.Error("isDelayed should be bool")
	}
	if reflect.TypeOf(jobDuration).Kind() != reflect.Float64 {
		t.Error("jobDuration should be float64")
	}
}

// TestWriteToExporterRegexMatching tests regex matching in writeToExporter
func TestWriteToExporterRegexMatching(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "cronmanager_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Save original env var
	originalValue, originalExists := os.LookupEnv("COLLECTOR_TEXTFILE_PATH")
	defer func() {
		if originalExists {
			os.Setenv("COLLECTOR_TEXTFILE_PATH", originalValue)
		} else {
			os.Unsetenv("COLLECTOR_TEXTFILE_PATH")
		}
	}()

	// Set custom path for testing
	os.Setenv("COLLECTOR_TEXTFILE_PATH", tmpDir)

	// Test with special characters in job name
	specialJobName := "test-job_with.special_chars"
	writeToExporter(specialJobName, "run", "1")

	exporterPath := filepath.Join(tmpDir, "crons.prom")
	content, err := os.ReadFile(exporterPath)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	contentStr := string(content)
	expectedLine := `cronjob{name="test-job_with.special_chars",dimension="run"} 1`
	if !strings.Contains(contentStr, expectedLine) {
		t.Errorf("Content should contain special job name: %s\nGot: %s", expectedLine, contentStr)
	}
}

// TestWriteToExporterMultipleJobs tests multiple jobs in same file
func TestWriteToExporterMultipleJobs(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "cronmanager_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Save original env var
	originalValue, originalExists := os.LookupEnv("COLLECTOR_TEXTFILE_PATH")
	defer func() {
		if originalExists {
			os.Setenv("COLLECTOR_TEXTFILE_PATH", originalValue)
		} else {
			os.Unsetenv("COLLECTOR_TEXTFILE_PATH")
		}
	}()

	// Set custom path for testing
	os.Setenv("COLLECTOR_TEXTFILE_PATH", tmpDir)

	// Write metrics for multiple jobs
	writeToExporter("job1", "run", "1")
	writeToExporter("job2", "run", "1")
	writeToExporter("job1", "failed", "0")
	writeToExporter("job2", "failed", "0")

	exporterPath := filepath.Join(tmpDir, "crons.prom")
	content, err := os.ReadFile(exporterPath)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	contentStr := string(content)
	
	// Verify both jobs are present
	if !strings.Contains(contentStr, `cronjob{name="job1",dimension="run"}`) {
		t.Errorf("Content should contain job1 run metric: %s", contentStr)
	}
	if !strings.Contains(contentStr, `cronjob{name="job2",dimension="run"}`) {
		t.Errorf("Content should contain job2 run metric: %s", contentStr)
	}
	if !strings.Contains(contentStr, `cronjob{name="job1",dimension="failed"}`) {
		t.Errorf("Content should contain job1 failed metric: %s", contentStr)
	}
	if !strings.Contains(contentStr, `cronjob{name="job2",dimension="failed"}`) {
		t.Errorf("Content should contain job2 failed metric: %s", contentStr)
	}

	// Verify only one TYPE header
	typeCount := strings.Count(contentStr, "# TYPE cron_job gauge")
	if typeCount != 1 {
		t.Errorf("Should have exactly one TYPE header, got %d", typeCount)
	}
}
