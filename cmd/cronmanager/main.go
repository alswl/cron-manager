package main

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"strconv"
	"syscall"
	"time"

	"github.com/abohmeed/cronmanager/pkg/exporter"
	"github.com/abohmeed/cronmanager/pkg/job"
	"github.com/abohmeed/cronmanager/pkg/version"
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
	jobnamePtr := flag.String("n", "", "[Required] The `job name` to appear in the alarm")
	logfilePtr := flag.String("l", "", "[Optional] The `log file` to store the cron output")
	idleSeconds := flag.Int("i", 0, "Idle for specified seconds (default: 0, disabled). If set, will wait to ensure the job runs for at least this duration so Prometheus can detect it")
	flag.BoolVar(&flgVersion, "version", false, "if true print version and exit")
	flag.Parse()
	if flgVersion {
		fmt.Println("CronManager version " + version.Version)
		os.Exit(0)
	}
	flag.Usage = func() {
		fmt.Printf(`Usage: cronmanager -n jobname [options] -- command [args...]
Example: cronmanager -n update_entities_cron -l /path/to/log -- /usr/bin/php /var/www/app/console broadcast:entities:updated -e project -l 20000
`)
		flag.PrintDefaults()
	}

	if *jobnamePtr == "" {
		flag.Usage()
		os.Exit(1)
	}

	// Parse command and arguments from -- separator
	// Note: flag.Parse() stops parsing flags when it encounters "--",
	// so flag.Args() returns all arguments after "--" (without "--" itself)
	// We need to check os.Args to verify "--" was provided, then use flag.Args()
	// which contains everything after "--"
	hasSeparator := false
	for _, arg := range os.Args {
		if arg == "--" {
			hasSeparator = true
			break
		}
	}

	if !hasSeparator {
		fmt.Fprintf(os.Stderr, "Error: command separator '--' not found\n")
		flag.Usage()
		os.Exit(1)
	}

	// flag.Args() contains all arguments after "--", so we can treat them as command args
	// We need to reconstruct the full args list with "--" to use extractCommandAfterSeparator
	args := append([]string{"--"}, flag.Args()...)
	cmdBin, cmdArgsOnly, err := extractCommandAfterSeparator(args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		flag.Usage()
		os.Exit(1)
	}

	//Record the start time of the job
	jobStartTime := time.Now()
	//Start a ticker in a goroutine that will write an alarm metric if the job exceeds the time
	go func() {
		for range time.Tick(time.Second) {
			jobDuration := time.Since(jobStartTime).Seconds()
			// Log current duration counter
			exporter.WriteToExporter(*jobnamePtr, "duration", strconv.FormatFloat(jobDuration, 'f', 0, 64))
			// Store last timestamp
			exporter.WriteToExporter(*jobnamePtr, "last", fmt.Sprintf("%d", time.Now().Unix()))
		}
	}()

	// Job started
	exporter.WriteToExporter(*jobnamePtr, "run", "1")

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

	if err != nil {
		if exiterr, ok := err.(*exec.ExitError); ok {
			if _, ok := exiterr.Sys().(syscall.WaitStatus); ok {
				exporter.WriteToExporter(*jobnamePtr, "failed", "1")
				// Job is no longer running
				exporter.WriteToExporter(*jobnamePtr, "run", "0")
			}
		} else {
			log.Fatalf("cmd.Wait: %v", err)
		}
	} else {
		// The job had no errors
		exporter.WriteToExporter(*jobnamePtr, "failed", "0")
		// Job is no longer running
		exporter.WriteToExporter(*jobnamePtr, "run", "0")
		// In all cases, unlock the file
	}

	// Store last timestamp
	exporter.WriteToExporter(*jobnamePtr, "last", fmt.Sprintf("%d", time.Now().Unix()))
}
