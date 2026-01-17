package exporter

import (
	"fmt"
	"log"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/alswl/cron-manager/internal/fslock"
	"github.com/spf13/afero"
)

// MetricType represents the type of Prometheus metric
type MetricType string

const (
	MetricTypeGauge   MetricType = "gauge"
	MetricTypeCounter MetricType = "counter"
)

// MetricWriter handles low-level metric writing operations
type MetricWriter struct {
	fs        afero.Fs
	useOsLock bool
}

// NewMetricWriter creates a new MetricWriter
func NewMetricWriter(fs afero.Fs, useOsLock bool) *MetricWriter {
	return &MetricWriter{
		fs:        fs,
		useOsLock: useOsLock,
	}
}

// escapeLabelValue escapes special characters in Prometheus label values
// According to Prometheus spec, we need to escape: \ -> \\, " -> \", \n -> \n
func escapeLabelValue(s string) string {
	// Replace backslash first, then quotes, then newlines
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "\"", "\\\"")
	s = strings.ReplaceAll(s, "\n", "\\n")
	return s
}

// buildLabelString constructs a Prometheus label string from job name and additional labels
func buildLabelString(jobName string, labels map[string]string) string {
	escapedJobName := escapeLabelValue(jobName)
	labelPairs := []string{fmt.Sprintf(`name="%s"`, escapedJobName)}
	for k, v := range labels {
		escapedKey := escapeLabelValue(k)
		escapedValue := escapeLabelValue(v)
		labelPairs = append(labelPairs, fmt.Sprintf(`%s="%s"`, escapedKey, escapedValue))
	}
	return strings.Join(labelPairs, ",")
}

// ensureDirectoryExists ensures that the directory for the given path exists
func (w *MetricWriter) ensureDirectoryExists(path string) {
	dir := filepath.Dir(path)
	if dir != "" && dir != "." {
		if err := w.fs.MkdirAll(dir, 0755); err != nil {
			log.Fatal("Couldn't create directory: " + err.Error())
		}
	}
}

// addMetricHeaders adds HELP and TYPE headers to the content if they don't exist
func addMetricHeaders(content []byte, fullMetricName string, metricType MetricType, help string) []byte {
	helpData := fmt.Sprintf("# HELP %s %s", fullMetricName, help)
	typeData := fmt.Sprintf("# TYPE %s %s", fullMetricName, metricType)

	helpRe := regexp.MustCompile(regexp.QuoteMeta(helpData))
	typeRe := regexp.MustCompile(regexp.QuoteMeta(typeData))

	if !helpRe.Match(content) {
		content = append(content, []byte(helpData+"\n")...)
	}
	if !typeRe.Match(content) {
		content = append(content, []byte(typeData+"\n")...)
	}

	return content
}

// readOrCreateFile reads the file content or creates an empty file if it doesn't exist
func (w *MetricWriter) readOrCreateFile(path string) ([]byte, error) {
	input, err := afero.ReadFile(w.fs, path)
	if err != nil {
		// File doesn't exist, create empty file
		if err := afero.WriteFile(w.fs, path, []byte{}, 0644); err != nil {
			return nil, fmt.Errorf("couldn't read or write to the exporter file: %w", err)
		}
		return []byte{}, nil
	}
	return input, nil
}

// writeMetricNoLock writes a metric without acquiring a lock (internal use)
// Caller must hold the lock before calling this function
func (w *MetricWriter) writeMetricNoLock(exporterPath, fullMetricName string, metricType MetricType, jobName string, labels map[string]string, value string, help string) error {
	// Build label string
	labelStr := buildLabelString(jobName, labels)

	// Build metric line
	metricLine := fmt.Sprintf(`%s{%s} %s`, fullMetricName, labelStr, value)

	// Ensure directory exists
	w.ensureDirectoryExists(exporterPath)

	// Read existing content
	input, err := w.readOrCreateFile(exporterPath)
	if err != nil {
		return err
	}

	// For both gauge and counter types, replace existing value or add new one
	// (Counter increment logic is handled separately in IncrementCounter function)
	exactPattern := regexp.MustCompile(regexp.QuoteMeta(fullMetricName) + `\{` + regexp.QuoteMeta(labelStr) + `\}.*\n`)

	if exactPattern.Match(input) {
		// Replace existing metric
		input = exactPattern.ReplaceAll(input, []byte(metricLine+"\n"))
	} else {
		// Metric doesn't exist, add it
		input = addMetricHeaders(input, fullMetricName, metricType, help)
		input = append(input, []byte(metricLine+"\n")...)
	}

	// Write to file
	return afero.WriteFile(w.fs, exporterPath, input, 0644)
}

// WriteMetric writes a metric to the Prometheus exporter file
// exporterPath: full path to the exporter file
// fullMetricName: full metric name (e.g., "crontab_failed")
// metricType: type of metric (gauge or counter)
// jobName: name of the job
// labels: additional labels as key-value pairs (e.g., map[string]string{"status": "success"})
// value: metric value
// help: HELP comment for the metric
func (w *MetricWriter) WriteMetric(exporterPath, fullMetricName string, metricType MetricType, jobName string, labels map[string]string, value string, help string) {
	// Lock filepath to prevent race conditions
	locker := fslock.NewLocker(exporterPath, w.useOsLock)
	if err := locker.Lock(); err != nil {
		log.Println("Error locking file " + exporterPath)
	}
	defer func() { _ = locker.Unlock() }()

	// Call internal function with lock held
	if err := w.writeMetricNoLock(exporterPath, fullMetricName, metricType, jobName, labels, value, help); err != nil {
		log.Fatal(err)
	}
}

// IncrementCounter increments a counter metric by 1
// exporterPath: full path to the exporter file
// fullMetricName: full metric name with prefix (e.g., "crontab_executions_total")
// jobName: name of the job
// labels: additional labels as key-value pairs
// help: HELP comment for the metric
func (w *MetricWriter) IncrementCounter(exporterPath, fullMetricName, jobName string, labels map[string]string, help string) {
	// Lock filepath to prevent race conditions
	locker := fslock.NewLocker(exporterPath, w.useOsLock)
	if err := locker.Lock(); err != nil {
		log.Println("Error locking file " + exporterPath)
		return
	}
	defer func() { _ = locker.Unlock() }()

	// Read existing content
	input, err := afero.ReadFile(w.fs, exporterPath)
	if err != nil {
		// File doesn't exist, start with 1
		// Call internal function without lock (we already hold the lock)
		if err := w.writeMetricNoLock(exporterPath, fullMetricName, MetricTypeCounter, jobName, labels, "1", help); err != nil {
			log.Fatal(err)
		}
		return
	}

	// Build label string
	labelStr := buildLabelString(jobName, labels)

	// Find existing counter value
	counterPattern := regexp.MustCompile(fmt.Sprintf(`%s\{%s\} (\d+(?:\.\d+)?)`, regexp.QuoteMeta(fullMetricName), regexp.QuoteMeta(labelStr)))
	matches := counterPattern.FindSubmatch(input)

	var newValue string
	if matches != nil {
		// Parse current value and increment
		currentStr := string(matches[1])
		newValue = w.incrementValue(currentStr)

		// Replace existing line
		oldLine := fmt.Sprintf(`%s{%s} %s`, fullMetricName, labelStr, currentStr)
		newLine := fmt.Sprintf(`%s{%s} %s`, fullMetricName, labelStr, newValue)
		re := regexp.MustCompile(regexp.QuoteMeta(oldLine) + `.*\n`)
		input = re.ReplaceAll(input, []byte(newLine+"\n"))
	} else {
		// Counter doesn't exist, start with 1
		newValue = "1"
		metricLine := fmt.Sprintf(`%s{%s} %s`, fullMetricName, labelStr, newValue)

		// Ensure directory exists
		w.ensureDirectoryExists(exporterPath)

		// Add headers and metric line
		input = addMetricHeaders(input, fullMetricName, MetricTypeCounter, help)
		input = append(input, []byte(metricLine+"\n")...)
	}

	// Write to file
	if err := afero.WriteFile(w.fs, exporterPath, input, 0644); err != nil {
		log.Fatal(err)
	}
}

// incrementValue increments a numeric string value by 1
func (w *MetricWriter) incrementValue(currentStr string) string {
	if strings.Contains(currentStr, ".") {
		// Float value
		current, err := strconv.ParseFloat(currentStr, 64)
		if err == nil {
			return fmt.Sprintf("%.2f", current+1.0)
		}
	} else {
		// Integer value
		current, err := strconv.ParseInt(currentStr, 10, 64)
		if err == nil {
			return fmt.Sprintf("%d", current+1)
		}
	}
	// Fallback to 1 if parsing fails
	return "1"
}
