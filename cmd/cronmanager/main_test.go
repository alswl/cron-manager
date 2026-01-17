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
		idleSeconds := fs.Int("i", 0, "idle seconds")
		jobnamePtr := fs.String("n", "", "job name")

		err := fs.Parse([]string{"-i", "60", "-n", "test_job", "--", "echo", "test"})
		if err != nil {
			t.Errorf("flag parsing error = %v", err)
		}

		if *idleSeconds != 60 {
			t.Errorf("idleSeconds = %v, want 60", *idleSeconds)
		}
		if *jobnamePtr != "test_job" {
			t.Errorf("jobnamePtr = %v, want 'test_job'", *jobnamePtr)
		}
	})

	t.Run("idle flag not set", func(t *testing.T) {
		// Create new flag set
		fs := flag.NewFlagSet("test", flag.ContinueOnError)
		idleSeconds := fs.Int("i", 0, "idle seconds")
		jobnamePtr := fs.String("n", "", "job name")

		err := fs.Parse([]string{"-n", "test_job", "--", "echo", "test"})
		if err != nil {
			t.Errorf("flag parsing error = %v", err)
		}

		if *idleSeconds != 0 {
			t.Errorf("idleSeconds = %v, want 0", *idleSeconds)
		}
		if *jobnamePtr != "test_job" {
			t.Errorf("jobnamePtr = %v, want 'test_job'", *jobnamePtr)
		}
	})
}

// TestExtractCommandAfterSeparator tests the extractCommandAfterSeparator function
func TestExtractCommandAfterSeparator(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		wantCmd   string
		wantArgs  []string
		wantError bool
		errorMsg  string
	}{
		{
			name:      "normal case with command and arguments",
			args:      []string{"--", "/usr/bin/php", "script.php", "arg1", "arg2"},
			wantCmd:   "/usr/bin/php",
			wantArgs:  []string{"script.php", "arg1", "arg2"},
			wantError: false,
		},
		{
			name:      "command without arguments",
			args:      []string{"--", "/usr/bin/echo"},
			wantCmd:   "/usr/bin/echo",
			wantArgs:  []string{},
			wantError: false,
		},
		{
			name:      "command with single argument",
			args:      []string{"--", "/usr/bin/python3", "script.py"},
			wantCmd:   "/usr/bin/python3",
			wantArgs:  []string{"script.py"},
			wantError: false,
		},
		{
			name:      "command with complex arguments",
			args:      []string{"--", "/usr/bin/php", "/var/www/app/console", "task:run", "-e", "project", "-l", "20000"},
			wantCmd:   "/usr/bin/php",
			wantArgs:  []string{"/var/www/app/console", "task:run", "-e", "project", "-l", "20000"},
			wantError: false,
		},
		{
			name:      "missing separator",
			args:      []string{"/usr/bin/echo", "test"},
			wantCmd:   "",
			wantArgs:  nil,
			wantError: true,
			errorMsg:  "command separator '--' not found",
		},
		{
			name:      "separator at end",
			args:      []string{"--"},
			wantCmd:   "",
			wantArgs:  nil,
			wantError: true,
			errorMsg:  "command is required after '--' separator",
		},
		{
			name:      "empty args",
			args:      []string{},
			wantCmd:   "",
			wantArgs:  nil,
			wantError: true,
			errorMsg:  "command separator '--' not found",
		},
		{
			name:      "separator with flags before it",
			args:      []string{"-n", "job_name", "--", "/usr/bin/command", "arg1"},
			wantCmd:   "/usr/bin/command",
			wantArgs:  []string{"arg1"},
			wantError: false,
		},
		{
			name:      "multiple separators (first one is used)",
			args:      []string{"--", "cmd1", "--", "cmd2"},
			wantCmd:   "cmd1",
			wantArgs:  []string{"--", "cmd2"},
			wantError: false,
		},
		{
			name:      "command with spaces in path",
			args:      []string{"--", "/usr/bin/my script", "arg1"},
			wantCmd:   "/usr/bin/my script",
			wantArgs:  []string{"arg1"},
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotCmd, gotArgs, err := extractCommandAfterSeparator(tt.args)

			if tt.wantError {
				if err == nil {
					t.Errorf("extractCommandAfterSeparator() expected error but got none")
					return
				}
				if err.Error() != tt.errorMsg {
					t.Errorf("extractCommandAfterSeparator() error = %v, want %v", err.Error(), tt.errorMsg)
				}
			} else {
				if err != nil {
					t.Errorf("extractCommandAfterSeparator() unexpected error = %v", err)
					return
				}
				if gotCmd != tt.wantCmd {
					t.Errorf("extractCommandAfterSeparator() command = %v, want %v", gotCmd, tt.wantCmd)
				}
				if len(gotArgs) != len(tt.wantArgs) {
					t.Errorf("extractCommandAfterSeparator() args length = %v, want %v", len(gotArgs), len(tt.wantArgs))
					return
				}
				for i, arg := range gotArgs {
					if arg != tt.wantArgs[i] {
						t.Errorf("extractCommandAfterSeparator() args[%d] = %v, want %v", i, arg, tt.wantArgs[i])
					}
				}
			}
		})
	}
}
