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

If you do not pass a directory, gotidy uses the current one.

What it does:
  - moves top-level files into folders like images/, documents/, code/, and more
  - leaves subdirectories, hidden files, and symlinks alone
  - skips files when the destination already has the same name

Flags:
  -n, --dry-run    Show what would move without changing anything
  -v, --verbose    Print each decision as gotidy works
  -V, --version    Show the gotidy version and exit
  -h, --help       Show this help text

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
	verb := "Organized"
	if dryRun {
		verb = "Dry run: would move"
	}

	if s.Moved == 0 {
		prefix := "Nothing to do in"
		if dryRun {
			prefix = "Dry run: nothing to move in"
		}
		fmt.Fprintf(out, "%s %s.\n", prefix, dir)
		if s.Skipped > 0 {
			fmt.Fprintf(out, "Skipped %s.\n", countLabel(s.Skipped, "file", "files"))
		}
		return
	}

	fmt.Fprintf(out, "%s %s in %s.\n", verb, countLabel(s.Moved, "file", "files"), dir)
	if s.Skipped > 0 {
		fmt.Fprintf(out, "Skipped %s.\n", countLabel(s.Skipped, "file", "files"))
	}

	if len(s.ByCategory) == 0 {
		return
	}

	fmt.Fprintln(out, "By category:")

	categories := make([]string, 0, len(s.ByCategory))
	for c := range s.ByCategory {
		categories = append(categories, c)
	}
	sort.Strings(categories)

	for _, c := range categories {
		fmt.Fprintf(out, "  %-14s %d\n", c+":", s.ByCategory[c])
	}
}

func countLabel(n int, singular, plural string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, singular)
	}
	return fmt.Sprintf("%d %s", n, plural)
}
