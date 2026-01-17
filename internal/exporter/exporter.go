package exporter

import (
	"log"
	"os"
	"path/filepath"
	"regexp"

	"github.com/alswl/cron-manager/internal/fslock"
	"github.com/spf13/afero"
)

var (
	// fs is the file system abstraction, defaults to real FS, can be replaced with memory FS in tests
	fs afero.Fs = afero.NewOsFs()
	// useOsLock indicates whether to use real file system locking, default true
	useOsLock bool = true
	// customExporterDir is set by SetExporterDir to override default or env var
	customExporterDir string
	// customExporterFilename is set by SetExporterFilename to override default filename
	customExporterFilename string
	// customMetricName is set by SetMetricName to override default metric name
	customMetricName string
	// metricDisabled is set by DisableMetric to disable metric writing
	metricDisabled bool
)

// UseTestFileSystem sets file system for testing (testing only)
func UseTestFileSystem(newFs afero.Fs) {
	fs = newFs
	useOsLock = false
}

// ResetFs restores real file system (cleanup after testing)
func ResetFs() {
	fs = afero.NewOsFs()
	useOsLock = true
	fslock.ResetMemLockers()
	customExporterDir = ""
	customExporterFilename = ""
	customMetricName = ""
	metricDisabled = false
}

// SetExporterDir sets a custom directory for Prometheus exporter files.
// This takes precedence over environment variable and default path.
func SetExporterDir(dir string) {
	customExporterDir = dir
}

// SetExporterFilename sets a custom filename for the Prometheus exporter file.
// Default is "crons.prom".
func SetExporterFilename(filename string) {
	customExporterFilename = filename
}

// SetMetricName sets a custom metric name for Prometheus metrics.
// Default is "crontab".
func SetMetricName(name string) {
	customMetricName = name
}

// DisableMetric disables metric writing to the exporter file.
func DisableMetric() {
	metricDisabled = true
}

// IsMetricDisabled returns whether metric writing is disabled.
func IsMetricDisabled() bool {
	return metricDisabled
}

// GetExporterPath returns the path to the Prometheus exporter file.
// Priority for directory: customExporterDir (set via SetExporterDir) > COLLECTOR_TEXTFILE_PATH env var > default path
// Filename: customExporterFilename (set via SetExporterFilename) > default "crons.prom"
func GetExporterPath() string {
	var exporterDir string

	// Priority 1: Custom directory set via SetExporterDir
	if customExporterDir != "" {
		exporterDir = customExporterDir
	} else {
		// Priority 2: Environment variable
		if envPath, exists := os.LookupEnv("COLLECTOR_TEXTFILE_PATH"); exists && envPath != "" {
			exporterDir = envPath
		} else {
			// Priority 3: Default path
			exporterDir = "/var/cache/prometheus"
		}
	}

	// Determine filename
	filename := "crons.prom"
	if customExporterFilename != "" {
		filename = customExporterFilename
	}

	return filepath.Join(exporterDir, filename)
}

// WriteToExporter writes a metric to the Prometheus exporter file
func WriteToExporter(jobName string, label string, metric string) {
	// If metric writing is disabled, return early
	if metricDisabled {
		return
	}

	// Determine metric name (default: crontab)
	metricName := "crontab"
	if customMetricName != "" {
		metricName = customMetricName
	}

	jobNeedle := metricName + "{name=\"" + jobName + "\",dimension=\"" + label + "\"}"
	typeData := "# TYPE " + metricName + " gauge"
	jobData := jobNeedle + " " + metric

	exporterPath := GetExporterPath()

	// Lock filepath to prevent race conditions
	locker := fslock.NewLocker(exporterPath, useOsLock)
	if err := locker.Lock(); err != nil {
		log.Println("Error locking file " + exporterPath)
	}
	defer func() { _ = locker.Unlock() }()

	// Ensure directory exists
	dir := filepath.Dir(exporterPath)
	if dir != "" && dir != "." {
		if err := fs.MkdirAll(dir, 0755); err != nil {
			log.Fatal("Couldn't create directory: " + err.Error())
		}
	}

	// Read existing content
	input, err := afero.ReadFile(fs, exporterPath)
	if err != nil {
		// File doesn't exist, create empty file
		if err := afero.WriteFile(fs, exporterPath, []byte{}, 0644); err != nil {
			log.Fatal("Couldn't read or write to the exporter file. Check parent directory permissions")
		}
		input = []byte{}
	}

	re := regexp.MustCompile(jobNeedle + `.*\n`)
	// If we have the job data already, just replace it and that's it
	if re.Match(input) {
		input = re.ReplaceAll(input, []byte(jobData+"\n"))
	} else {
		// If the job is not there then either there is no TYPE header at all and this is the first job
		if re := regexp.MustCompile(typeData); !re.Match(input) {
			// Add the TYPE and the job data
			input = append(input, typeData+"\n"...)
			input = append(input, jobData+"\n"...)
		} else {
			// Or there is a TYPE header with one or more other jobs. Just append the job to the TYPE header
			input = re.ReplaceAll(input, []byte(typeData+"\n"+jobData))
		}
	}

	// Write to file
	if err := afero.WriteFile(fs, exporterPath, input, 0644); err != nil {
		log.Fatal(err)
	}
}
