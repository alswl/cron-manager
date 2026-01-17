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
	"strings"
	"syscall"
	"time"

	"github.com/abohmeed/cronmanager/pkg/exporter"
	"github.com/abohmeed/cronmanager/pkg/job"
	"github.com/abohmeed/cronmanager/pkg/version"
)

var (
	flgVersion bool
)

func main() {
	idle := flag.Bool("i", false, fmt.Sprintf("Idle for %d seconds at the beginning so Prometheus can notice it's actually running", job.IdleForSeconds))
	cmdPtr := flag.String("c", "", "[Required] The `cron job` command")
	jobnamePtr := flag.String("n", "", "[Required] The `job name` to appear in the alarm")
	logfilePtr := flag.String("l", "", "[Optional] The `log file` to store the cron output")
	flag.BoolVar(&flgVersion, "version", false, "if true print version and exit")
	flag.Parse()
	if flgVersion {
		fmt.Println("CronManager version " + version.Version)
		os.Exit(0)
	}
	flag.Usage = func() {
		fmt.Printf("Usage: cronmanager -c command  -n jobname  [ -l log file ]\nExample: cronmanager \"/usr/bin/php /var/www/app.zlien.com/console broadcast:entities:updated -e project -l 20000\" -n update_entitites_cron -t 3600 -l /path/to/log\n")
		flag.PrintDefaults()
	}
	if *cmdPtr == "" || *jobnamePtr == "" {
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

	// Parse the command by extracting the first token as the command and the rest as its args
	cmdArr := strings.Split(*cmdPtr, " ")
	cmdBin := cmdArr[0]
	cmdArgs := cmdArr[1:]
	cmd := exec.Command(cmdBin, cmdArgs...)

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
	err := cmd.Start()
	if err != nil {
		log.Fatal(err)
	}

	// Execute the command
	err = cmd.Wait()

	// wait if idle is active
	if *idle {
		job.IdleWait(jobStartTime)
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
