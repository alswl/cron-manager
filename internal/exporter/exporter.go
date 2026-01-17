package exporter

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/afero"
)

// config holds configuration for Exporter (package-private)
type config struct {
	// exporterDir is the directory for Prometheus exporter files
	// If empty, uses COLLECTOR_TEXTFILE_PATH env var or default path
	exporterDir string
	// exporterFilename is the filename for the Prometheus exporter file
	// Default is "crons.prom"
	exporterFilename string
	// metricName is the base metric name prefix
	// Default is "crontab"
	metricName string
	// metricDisabled indicates whether metric writing is disabled
	metricDisabled bool
	// fs is the file system abstraction, defaults to real FS
	// Can be set for testing purposes
	fs afero.Fs
	// useOsLock indicates whether to use real file system locking
	// Default is true
	useOsLock bool
}

// defaultConfig returns a config with default values
func defaultConfig() config {
	return config{
		exporterDir:      "",
		exporterFilename: "crons.prom",
		metricName:       "crontab",
		metricDisabled:   false,
		fs:               afero.NewOsFs(),
		useOsLock:        true,
	}
}

// Option is a function that configures an Exporter
// This is the only public interface for configuring Exporter
type Option func(*config)

// WithExporterDir sets the exporter directory
func WithExporterDir(dir string) Option {
	return func(c *config) {
		c.exporterDir = dir
	}
}

// WithExporterFilename sets the exporter filename
func WithExporterFilename(filename string) Option {
	return func(c *config) {
		c.exporterFilename = filename
	}
}

// WithMetricName sets the metric name prefix
func WithMetricName(name string) Option {
	return func(c *config) {
		c.metricName = name
	}
}

// WithMetricDisabled disables metric writing
func WithMetricDisabled(disabled bool) Option {
	return func(c *config) {
		c.metricDisabled = disabled
	}
}

// WithFileSystem sets a custom file system (for testing)
func WithFileSystem(fs afero.Fs) Option {
	return func(c *config) {
		c.fs = fs
		c.useOsLock = false // Disable OS locking when using custom FS
	}
}

// Exporter manages Prometheus metric export configuration and operations
type Exporter struct {
	config       config        // Immutable configuration (package-private)
	metricWriter *MetricWriter // Writer for low-level metric operations
}

// NewExporter creates a new Exporter instance with default settings
// Options can be provided to customize the configuration
func NewExporter(opts ...Option) *Exporter {
	config := defaultConfig()
	for _, opt := range opts {
		opt(&config)
	}
	return &Exporter{
		config:       config,
		metricWriter: NewMetricWriter(config.fs, config.useOsLock),
	}
}

// IsMetricDisabled returns whether metric writing is disabled.
func (e *Exporter) IsMetricDisabled() bool {
	return e.config.metricDisabled
}

// GetExporterPath returns the path to the Prometheus exporter file.
// Priority for directory: config.exporterDir > COLLECTOR_TEXTFILE_PATH env var > default path
// Filename: config.exporterFilename (default: "crons.prom")
func (e *Exporter) GetExporterPath() string {
	var exporterDir string

	// Priority 1: Custom directory from config
	if e.config.exporterDir != "" {
		exporterDir = e.config.exporterDir
	} else {
		// Priority 2: Environment variable
		if envPath, exists := os.LookupEnv("COLLECTOR_TEXTFILE_PATH"); exists && envPath != "" {
			exporterDir = envPath
		} else {
			// Priority 3: Default path
			exporterDir = "/var/lib/prometheus/node-exporter"
		}
	}

	// Use filename from config (default is "crons.prom")
	filename := e.config.exporterFilename

	return filepath.Join(exporterDir, filename)
}

// writeMetric writes a metric to the Prometheus exporter file
// metricName: full metric name (e.g., "crontab_failed")
// metricType: type of metric (gauge or counter)
// jobName: name of the job
// labels: additional labels as key-value pairs (e.g., map[string]string{"status": "success"})
// value: metric value
// help: HELP comment for the metric
func (e *Exporter) writeMetric(metricName string, metricType MetricType, jobName string, labels map[string]string, value string, help string) {
	// If metric writing is disabled, return early
	if e.config.metricDisabled {
		return
	}

	// Get base metric prefix from config (default: "crontab")
	basePrefix := e.config.metricName

	// If metricName doesn't start with base prefix, prepend it
	fullMetricName := metricName
	if !strings.HasPrefix(metricName, basePrefix) {
		fullMetricName = basePrefix + "_" + metricName
	}

	exporterPath := e.GetExporterPath()
	e.metricWriter.WriteMetric(exporterPath, fullMetricName, metricType, jobName, labels, value, help)
}

// WriteToExporter writes a metric to the Prometheus exporter file (legacy API with dimension label)
// Deprecated: Use WriteGauge or WriteCounter instead
func (e *Exporter) WriteToExporter(jobName string, label string, metric string) {
	// Legacy API: map dimension label to new metric names
	metricMap := map[string]string{
		"failed":    "failed",
		"exit_code": "exit_code",
		"duration":  "duration_seconds",
		"run":       "running",
		"last":      "last_run_timestamp_seconds",
	}

	metricName, exists := metricMap[label]
	if !exists {
		// Fallback to dimension-based format for unknown labels
		metricName = "crontab"
		e.writeMetric(metricName, MetricTypeGauge, jobName, map[string]string{"dimension": label}, metric, "Cron job execution metrics")
		return
	}

	// Use new metric names
	helpText := map[string]string{
		"failed":                     "Whether the job failed (1 = failed, 0 = success)",
		"exit_code":                  "Exit code of the last job execution",
		"duration_seconds":           "Duration of the last job execution in seconds",
		"running":                    "Whether the job is currently running (1 = running, 0 = finished)",
		"last_run_timestamp_seconds": "Timestamp of the last job execution",
	}

	help := helpText[metricName]
	if help == "" {
		help = "Cron job execution metrics"
	}

	e.writeMetric(metricName, MetricTypeGauge, jobName, nil, metric, help)
}

// WriteGauge writes a gauge metric to the Prometheus exporter file
func (e *Exporter) WriteGauge(metricName string, jobName string, value string, help string) {
	e.writeMetric(metricName, MetricTypeGauge, jobName, nil, value, help)
}

// WriteGaugeWithLabels writes a gauge metric with additional labels
func (e *Exporter) WriteGaugeWithLabels(metricName string, jobName string, labels map[string]string, value string, help string) {
	e.writeMetric(metricName, MetricTypeGauge, jobName, labels, value, help)
}

// WriteCounter writes a counter metric to the Prometheus exporter file
// Note: Counter values should be incremented by the caller
func (e *Exporter) WriteCounter(metricName string, jobName string, labels map[string]string, value string, help string) {
	e.writeMetric(metricName, MetricTypeCounter, jobName, labels, value, help)
}

// IncrementCounter increments a counter metric by 1
func (e *Exporter) IncrementCounter(metricName string, jobName string, labels map[string]string, help string) {
	// If metric writing is disabled, return early
	if e.config.metricDisabled {
		return
	}

	// Get base metric prefix from config
	basePrefix := e.config.metricName
	fullMetricName := basePrefix + "_" + metricName

	exporterPath := e.GetExporterPath()
	e.metricWriter.IncrementCounter(exporterPath, fullMetricName, jobName, labels, help)
}
