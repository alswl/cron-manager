package main

import (
	"flag"
	"testing"
)

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
