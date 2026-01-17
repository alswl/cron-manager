package exporter

import (
	"log"
	"os"
	"path/filepath"
	"regexp"

	"github.com/abohmeed/cronmanager/pkg/fslock"
	"github.com/spf13/afero"
)

var (
	// fs is the file system abstraction, defaults to real FS, can be replaced with memory FS in tests
	fs afero.Fs = afero.NewOsFs()
	// useOsLock indicates whether to use real file system locking, default true
	useOsLock bool = true
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
}

// GetExporterPath returns the path to the Prometheus exporter file
func GetExporterPath() string {
	exporterPath, exists := os.LookupEnv("COLLECTOR_TEXTFILE_PATH")
	exporterPath = exporterPath + "/crons.prom"
	if !exists {
		exporterPath = "/var/cache/prometheus/crons.prom"
	}
	return exporterPath
}

// WriteToExporter writes a metric to the Prometheus exporter file
func WriteToExporter(jobName string, label string, metric string) {
	jobNeedle := "cronjob{name=\"" + jobName + "\",dimension=\"" + label + "\"}"
	typeData := "# TYPE cron_job gauge"
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
