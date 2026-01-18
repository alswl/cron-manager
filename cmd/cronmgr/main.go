package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"strconv"
	"syscall"
	"time"

	"github.com/alswl/cron-manager/internal/exporter"
	"github.com/alswl/cron-manager/internal/job"
	"github.com/alswl/cron-manager/internal/version"
	"github.com/spf13/pflag"
)

var (
	flgVersion bool
)

// extractCommandAfterSeparator extracts the command and its arguments from args
// after the "--" separator. It returns the command path, its arguments, and an error
// if the separator is missing or no command is provided after the separator.
func extractCommandAfterSeparator(args []string) (command string, arguments []string, err error) {
	const separator = "--"

	// Find the separator index
	separatorIndex := -1
	for i, arg := range args {
		if arg == separator {
			separatorIndex = i
			break
		}
	}

	if separatorIndex == -1 {
		return "", nil, fmt.Errorf("command separator '%s' not found", separator)
	}

	// Extract command and arguments after the separator
	commandArgs := args[separatorIndex+1:]
	if len(commandArgs) == 0 {
		return "", nil, fmt.Errorf("command is required after '%s' separator", separator)
	}

	// First element is the command, rest are arguments
	command = commandArgs[0]
	arguments = commandArgs[1:]
	return command, arguments, nil
}

func main() {
	// Define flags with both short and long options
	jobnamePtr := pflag.StringP("name", "n", "", "Job name (required, will appear in alerts)")
	logfilePtr := pflag.StringP("log", "l", "", "Log file path to store the cron job output")
	idleSeconds := pflag.IntP("idle", "i", 0, "Idle wait duration in seconds (0 = disabled). Ensures job runs for at least this duration for Prometheus detection")
	exporterDirPtr := pflag.StringP("dir", "d", "", "Directory for Prometheus exporter file (default: /var/lib/prometheus/node-exporter or COLLECTOR_TEXTFILE_PATH env var)")
	textfilePtr := pflag.String("textfile", "crons.prom", "Filename for Prometheus exporter file")
	metricNamePtr := pflag.String("metric", "crontab", "Metric name for Prometheus metrics")
	noMetricPtr := pflag.Bool("no-metric", false, "Disable metric writing to Prometheus exporter file")
	pflag.BoolVarP(&flgVersion, "version", "v", false, "Display version information and exit")

	// Set usage function
	pflag.Usage = func() {
		fmt.Fprintf(os.Stderr, `Usage: cronmgr --name <jobname> [options] -- <command> [args...]

Execute and monitor a cron job, publishing metrics to Prometheus.

Options:
`)
		pflag.PrintDefaults()
		fmt.Fprintf(os.Stderr, `
Examples:
  cronmgr --name update_entities_cron -- /usr/bin/php /var/www/app/console task:run
  cronmgr -n job_cron --log /var/log/cron.log -- /usr/bin/python3 script.py
  cronmgr -n job_cron --idle 60 --metric my_metric -- /usr/bin/command arg1 arg2
  cronmgr -n job_cron --no-metric -- /usr/bin/command

For more information, visit: https://github.com/alswl/cron-manager
`)
	}

	// Sort flags for better help output
	pflag.CommandLine.SortFlags = false

	pflag.Parse()

	if flgVersion {
		fmt.Println("cronmgr version " + version.Version)
		os.Exit(0)
	}

	if *jobnamePtr == "" {
		fmt.Fprintf(os.Stderr, "Error: --name is required\n\n")
		pflag.Usage()
		os.Exit(1)
	}

	// Build exporter options
	var opts []exporter.Option
	if *exporterDirPtr != "" {
		opts = append(opts, exporter.WithExporterDir(*exporterDirPtr))
	}
	if *textfilePtr != "" {
		opts = append(opts, exporter.WithExporterFilename(*textfilePtr))
	}
	if *metricNamePtr != "" {
		opts = append(opts, exporter.WithMetricName(*metricNamePtr))
	}
	if *noMetricPtr {
		opts = append(opts, exporter.WithMetricDisabled(true))
	}

	// Create exporter instance with options
	exp := exporter.NewExporter(opts...)

	// Parse command and arguments from -- separator
	// Note: pflag.Parse() stops parsing flags when it encounters "--",
	// so pflag.Args() returns all arguments after "--" (without "--" itself)
	// We need to check os.Args to verify "--" was provided, then use pflag.Args()
	// which contains everything after "--"
	hasSeparator := false
	for _, arg := range os.Args {
		if arg == "--" {
			hasSeparator = true
			break
		}
	}

	if !hasSeparator {
		fmt.Fprintf(os.Stderr, "Error: command separator '--' not found\n\n")
		pflag.Usage()
		os.Exit(1)
	}

	// pflag.Args() contains all arguments after "--", so we can treat them as command args
	// We need to reconstruct the full args list with "--" to use extractCommandAfterSeparator
	args := append([]string{"--"}, pflag.Args()...)
	cmdBin, cmdArgsOnly, err := extractCommandAfterSeparator(args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n\n", err)
		pflag.Usage()
		os.Exit(1)
	}

	//Record the start time of the job
	jobStartTime := time.Now()
	//Start a ticker in a goroutine that will write an alarm metric if the job exceeds the time
	go func() {
		for range time.Tick(time.Second) {
			jobDuration := time.Since(jobStartTime).Seconds()
			// Log current duration with 2 decimal precision
			exp.WriteGauge("duration_seconds", *jobnamePtr, strconv.FormatFloat(jobDuration, 'f', 2, 64), "Duration of the last job execution in seconds")
			// Store last timestamp
			exp.WriteGauge("last_run_timestamp_seconds", *jobnamePtr, fmt.Sprintf("%d", time.Now().Unix()), "Timestamp of the last job execution")
		}
	}()

	// Job started - increment run counter and set running status
	exp.IncrementCounter("runs_total", *jobnamePtr, map[string]string{"status": "started"}, "Total number of job runs")
	exp.WriteGauge("running", *jobnamePtr, "1", "Whether the job is currently running (1 = running, 0 = finished)")

	// Execute the command with arguments
	cmd := exec.Command(cmdBin, cmdArgsOnly...)

	var buf bytes.Buffer

	// If we have a log file specified, use it
	if *logfilePtr != "" {
		outfile, err := os.Create(*logfilePtr)
		if err != nil {
			panic(err)
		}
		defer func() { _ = outfile.Close() }()
		stdoutPipe, err := cmd.StdoutPipe()
		if err != nil {
			panic(err)
		}
		writer := bufio.NewWriter(outfile)
		defer func() { _ = writer.Flush() }()
		go func() {
			if _, err := io.Copy(writer, stdoutPipe); err != nil {
				log.Printf("Error copying stdout: %v", err)
			}
		}()
	} else {
		cmd.Stdout = &buf
	}

	err = cmd.Start()
	if err != nil {
		log.Fatal(err)
	}

	// Execute the command
	err = cmd.Wait()

	// wait if idle is active
	if *idleSeconds > 0 {
		job.IdleWait(jobStartTime, *idleSeconds)
	}

	// Calculate final duration
	finalDuration := time.Since(jobStartTime).Seconds()

	if err != nil {
		if exiterr, ok := err.(*exec.ExitError); ok {
			if waitStatus, ok := exiterr.Sys().(syscall.WaitStatus); ok {
				exitCode := waitStatus.ExitStatus()
				// Job failed
				exp.WriteGauge("failed", *jobnamePtr, "1", "Whether the job failed (1 = failed, 0 = success)")
				exp.WriteGauge("exit_code", *jobnamePtr, strconv.Itoa(exitCode), "Exit code of the last job execution")
				// Increment failed counter
				exp.IncrementCounter("runs_total", *jobnamePtr, map[string]string{"status": "failed"}, "Total number of job runs")
			}
		} else {
			log.Fatalf("cmd.Wait: %v", err)
		}
	} else {
		// The job succeeded
		exp.WriteGauge("failed", *jobnamePtr, "0", "Whether the job failed (1 = failed, 0 = success)")
		exp.WriteGauge("exit_code", *jobnamePtr, "0", "Exit code of the last job execution")
		// Increment success counter
		exp.IncrementCounter("runs_total", *jobnamePtr, map[string]string{"status": "success"}, "Total number of job runs")
	}

	// Job is no longer running
	exp.WriteGauge("running", *jobnamePtr, "0", "Whether the job is currently running (1 = running, 0 = finished)")
	// Store final duration and last timestamp
	exp.WriteGauge("duration_seconds", *jobnamePtr, strconv.FormatFloat(finalDuration, 'f', 2, 64), "Duration of the last job execution in seconds")
	exp.WriteGauge("last_run_timestamp_seconds", *jobnamePtr, fmt.Sprintf("%d", time.Now().Unix()), "Timestamp of the last job execution")
}
