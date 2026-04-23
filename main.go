package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
)

// version is set at build time via -ldflags "-X main.version=...".
var version = "dev"

const usage = `gotidy - sort files in a directory into subfolders by type

Usage:
  gotidy [flags] [directory]

If no directory is given, the current working directory is used.
Subdirectories, hidden files, and symlinks are left untouched.

Flags:
  -n, --dry-run    Preview the moves without touching any files
  -v, --verbose    Print every considered file
  -V, --version    Print the gotidy version and exit
  -h, --help       Show this help message

Examples:
  gotidy ~/Downloads
  gotidy --dry-run ~/Downloads
  gotidy -v .
`

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("gotidy", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() { fmt.Fprint(stderr, usage) }

	var (
		dryRun      bool
		verbose     bool
		showVersion bool
	)
	fs.BoolVar(&dryRun, "dry-run", false, "preview moves without touching files")
	fs.BoolVar(&dryRun, "n", false, "preview moves without touching files (shorthand)")
	fs.BoolVar(&verbose, "verbose", false, "print every considered file")
	fs.BoolVar(&verbose, "v", false, "print every considered file (shorthand)")
	fs.BoolVar(&showVersion, "version", false, "print the gotidy version")
	fs.BoolVar(&showVersion, "V", false, "print the gotidy version (shorthand)")

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}

	if showVersion {
		fmt.Fprintf(stdout, "gotidy %s\n", version)
		return 0
	}

	dir := "."
	switch fs.NArg() {
	case 0:
	case 1:
		dir = fs.Arg(0)
	default:
		fmt.Fprintln(stderr, "error: gotidy accepts at most one directory argument")
		fmt.Fprint(stderr, usage)
		return 2
	}

	opts := Options{
		DryRun:  dryRun,
		Verbose: verbose,
		Logf: func(format string, args ...any) {
			fmt.Fprintf(stdout, format+"\n", args...)
		},
	}

	summary, err := Organize(dir, opts)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}

	printSummary(stdout, dir, summary, dryRun)
	return 0
}

func printSummary(out io.Writer, dir string, s Summary, dryRun bool) {
	verb := "Moved"
	if dryRun {
		verb = "Would move"
	}

	fmt.Fprintf(out, "%s %d file(s) in %s (%d skipped).\n", verb, s.Moved, dir, s.Skipped)

	if len(s.ByCategory) == 0 {
		return
	}

	categories := make([]string, 0, len(s.ByCategory))
	for c := range s.ByCategory {
		categories = append(categories, c)
	}
	sort.Strings(categories)

	for _, c := range categories {
		fmt.Fprintf(out, "  %-14s %d\n", c+":", s.ByCategory[c])
	}
}
