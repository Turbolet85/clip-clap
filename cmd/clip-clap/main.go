// Package main is the clip-clap entry point.
//
// The version flag is processed via the run() seam (testable from
// main_test.go) before any subsystem is initialized.
//
// The goversioninfo directive below produces cmd/clip-clap/resource.syso
// at `go generate` time. Paths are relative to this file's directory
// (cmd/clip-clap/), so they reach back two levels to the project root
// where assets/ and goversioninfo.json live.
//
//go:generate goversioninfo -64 -icon=../../assets/app.ico -manifest=../../assets/app.manifest ../../goversioninfo.json
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
)

const versionString = "clip-clap v0.0.1"

// run is the testable seam that main() wraps. Tests in main_test.go invoke
// run directly with custom args + stdout buffer so --version can be exercised
// without spawning a subprocess.
//
// We use fs.Bool("version", ...) on a local FlagSet instead of flag.Bool("version", ...)
// on the global FlagSet — the local FlagSet can be re-parsed many times in tests
// without polluting global flag state (the global flag.Bool would panic on the
// second test run with "flag redefined: version").
func run(args []string, stdout io.Writer) int {
	fs := flag.NewFlagSet("clip-clap", flag.ContinueOnError)
	// Writer target for flag package output. Routing to stdout (not io.Discard)
	// means `--help` prints the flag listing to the provided writer instead of
	// being silently swallowed — required by /implement Final 5 scaffolding
	// CLI smoke (`{binary} --help` must return 0 with flag listing).
	fs.SetOutput(stdout)
	versionFlag := fs.Bool("version", false, "print version and exit")
	if err := fs.Parse(args); err != nil {
		// flag.ErrHelp: the user passed -h or --help. flag.Parse already
		// printed the usage to stdout (via SetOutput above) before returning
		// ErrHelp. We return 0 to match standard CLI convention.
		if err == flag.ErrHelp {
			return 0
		}
		// Any other parse error (unknown flag, malformed value) → exit 2.
		// Error message was written to stdout by fs.Parse before returning.
		return 2
	}
	if *versionFlag {
		fmt.Fprintln(stdout, versionString)
		return 0
	}
	// Phase 0 skeleton: no-args is a no-op exit. Phase 2+ replaces this with
	// the message-pump entry point (config load → tray + hotkey + status).
	return 0
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout))
}
