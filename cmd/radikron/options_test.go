package main

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func TestParseOptions(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want options
	}{
		{"defaults", nil, options{configFile: "config.yml"}},
		{"short", []string{"-c", "custom.yml", "-d", "-v"}, options{configFile: "custom.yml", debug: true, version: true}},
		{"long", []string{"--config", "custom.yml", "--debug", "--version"}, options{configFile: "custom.yml", debug: true, version: true}},
		{"assignments", []string{"--config=custom.yml", "--debug=true", "--version=false"}, options{configFile: "custom.yml", debug: true}},
		{"legacy assignments", []string{"-c=custom.yml", "-d=true", "-v=false"}, options{configFile: "custom.yml", debug: true}},
		{"legacy double dash", []string{"--c", "custom.yml", "--d", "--v"}, options{configFile: "custom.yml", debug: true, version: true}},
		{"single dash named aliases", []string{"-config", "custom.yml", "-debug", "-version"}, options{configFile: "custom.yml", debug: true, version: true}},
		{"legacy updates", []string{"-check-update", "-self-update", "-no-update-check"}, options{configFile: "config.yml", checkUpdate: true, selfUpdate: true, noUpdateCheck: true}},
		{"long updates", []string{"--check-update", "--self-update", "--no-update-check"}, options{configFile: "config.yml", checkUpdate: true, selfUpdate: true, noUpdateCheck: true}},
		{"last alias wins", []string{"-c", "first.yml", "--config", "last.yml", "-d", "--debug=false"}, options{configFile: "last.yml"}},
		{"positional stops parsing", []string{"unused", "--debug"}, options{configFile: "config.yml"}},
		{"terminator", []string{"--", "--debug"}, options{configFile: "config.yml"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			got, err := parseOptions(tt.args, &stdout, &stderr)
			if err != nil || got != tt.want {
				t.Fatalf("parseOptions = %+v, %v; want %+v, nil", got, err, tt.want)
			}
			if stdout.Len() != 0 || stderr.Len() != 0 {
				t.Fatalf("unexpected output: stdout=%q stderr=%q", stdout.String(), stderr.String())
			}
		})
	}
}

func TestParseOptionsHelp(t *testing.T) {
	for _, arg := range []string{"-h", "--help", "-help", "--h"} {
		t.Run(arg, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			_, err := parseOptions([]string{arg}, &stdout, &stderr)
			if !errors.Is(err, flag.ErrHelp) || stdout.String() != helpText || stderr.Len() != 0 {
				t.Fatalf("help: err=%v stdout=%q stderr=%q", err, stdout.String(), stderr.String())
			}
		})
	}
}

func TestParseOptionsErrors(t *testing.T) {
	for _, args := range [][]string{{"--unknown"}, {"--config"}, {"--debug=invalid"}} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			_, err := parseOptions(args, &stdout, &stderr)
			if err == nil || errors.Is(err, flag.ErrHelp) || stdout.Len() != 0 || !strings.Contains(stderr.String(), helpText) {
				t.Fatalf("invalid flags: err=%v stdout=%q stderr=%q", err, stdout.String(), stderr.String())
			}
		})
	}
}

// Run the real entry point in an isolated process to check exit codes and
// prove help/version do not load the deliberately missing configuration.
func TestCLIExit(t *testing.T) {
	if os.Getenv("RADIKRON_TEST_CLI") == "1" {
		for i, arg := range os.Args {
			if arg == "--" {
				os.Args = append([]string{"radikron"}, os.Args[i+1:]...)
				break
			}
		}
		version = "v1.2.3-test"
		main()
		os.Exit(0)
	}
	tests := []struct {
		arg  string
		code int
		want string
	}{
		{"-h", 0, helpText},
		{"--help", 0, helpText},
		{"-v", 0, "v1.2.3-test\n"},
		{"--version", 0, "v1.2.3-test\n"},
		{"--unknown", 2, ""},
	}
	for _, tt := range tests {
		t.Run(tt.arg, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestCLIExit$", "--", "--config", "missing-config.yml", tt.arg)
			cmd.Env = append(os.Environ(), "RADIKRON_TEST_CLI=1")
			cmd.Dir = t.TempDir()
			var stdout, stderr bytes.Buffer
			cmd.Stdout, cmd.Stderr = &stdout, &stderr
			err := cmd.Run()
			if ctx.Err() != nil {
				t.Fatalf("CLI did not exit: %v", ctx.Err())
			}
			code := 0
			if err != nil {
				var exitErr *exec.ExitError
				if !errors.As(err, &exitErr) {
					t.Fatal(err)
				}
				code = exitErr.ExitCode()
			}
			if code != tt.code || stdout.String() != tt.want {
				t.Fatalf("exit=%d stdout=%q stderr=%q; want exit=%d stdout=%q", code, stdout.String(), stderr.String(), tt.code, tt.want)
			}
			if tt.code == 0 && stderr.Len() != 0 {
				t.Fatalf("unexpected startup activity: %q", stderr.String())
			}
			if tt.code != 0 && !strings.Contains(stderr.String(), "flag provided but not defined: -unknown") {
				t.Fatalf("missing diagnostic: %q", stderr.String())
			}
		})
	}
}
