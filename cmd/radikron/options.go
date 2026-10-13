package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
)

type options struct {
	configFile    string
	debug         bool
	version       bool
	checkUpdate   bool
	selfUpdate    bool
	noUpdateCheck bool
}

const helpText = `Radikron automatically saves radiko.jp timeshift programs
matching your configured rules.

Usage:
  radikron [options]

Options:
  -h, --help                 Show help and exit
  -c, --config FILE          Read configuration from FILE (default: config.yml)
  -d, --debug                Include source locations and update-check errors in logs
  -v, --version              Print version and exit
      --check-update         Check for a newer GitHub release and exit
      --self-update          Verify and install the latest release, then exit
      --no-update-check      Skip the startup update check

Examples:
  radikron
  radikron --config config.yml
  radikron --check-update
`

// parseOptions keeps Go flag's existing spellings and parsing semantics while
// adding named aliases. Help never starts the scheduler or update client.
func parseOptions(args []string, stdout, stderr io.Writer) (options, error) {
	var opts options
	flags := flag.NewFlagSet("radikron", flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.Usage = func() {} // Render help below, choosing the appropriate stream.
	for _, name := range []string{"c", "config"} {
		flags.StringVar(&opts.configFile, name, "config.yml", "Read configuration from FILE")
	}
	for _, name := range []string{"d", "debug"} {
		flags.BoolVar(&opts.debug, name, false, "Include source locations and update-check errors in logs")
	}
	for _, name := range []string{"v", "version"} {
		flags.BoolVar(&opts.version, name, false, "Print version and exit")
	}
	flags.BoolVar(&opts.checkUpdate, "check-update", false, "Check for a newer GitHub release and exit")
	flags.BoolVar(&opts.selfUpdate, "self-update", false, "Verify and install the latest release, then exit")
	flags.BoolVar(&opts.noUpdateCheck, "no-update-check", false, "Skip the startup update check")
	err := flags.Parse(args)
	if errors.Is(err, flag.ErrHelp) {
		fmt.Fprint(stdout, helpText)
	} else if err != nil {
		fmt.Fprint(stderr, helpText)
	}
	return opts, err
}
