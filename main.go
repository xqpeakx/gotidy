package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"runtime/debug"
	"sort"
	"strings"
)

// version is set at build time via -ldflags "-X main.version=...".
var version = "alpha_0.0.1"

const usage = `gotidy - sort files in a directory into subfolders by type

Usage:
  gotidy [flags] [directory]

If you do not pass a directory, gotidy uses the current one.

What it does:
  - moves top-level files into folders like images/, documents/, code/, and more
  - can undo the last real run with --undo
  - leaves subdirectories, hidden files, and symlinks alone
  - skips files when the destination already has the same name

Flags:
  -n, --dry-run    Show what would move without changing anything
  -u, --undo       Undo the last organize run in this directory
  -v, --verbose    Print each decision as gotidy works
  -V, --version    Show the gotidy version and exit
  -h, --help       Show this help text

Examples:
  gotidy ~/Downloads
  gotidy --dry-run ~/Downloads
  gotidy --undo ~/Downloads
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
		undo        bool
		verbose     bool
		showVersion bool
	)
	fs.BoolVar(&dryRun, "dry-run", false, "preview moves without touching files")
	fs.BoolVar(&dryRun, "n", false, "preview moves without touching files (shorthand)")
	fs.BoolVar(&undo, "undo", false, "undo the last organize run in this directory")
	fs.BoolVar(&undo, "u", false, "undo the last organize run in this directory (shorthand)")
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
		fmt.Fprintf(stdout, "gotidy %s\n", resolvedVersion())
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

	var (
		summary Summary
		err     error
	)
	if undo {
		summary, err = Undo(dir, opts)
	} else {
		summary, err = Organize(dir, opts)
	}
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}

	if undo {
		printUndoSummary(stdout, dir, summary, dryRun)
	} else {
		printSummary(stdout, dir, summary, dryRun)
	}
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

	printCategoryBreakdown(out, s.ByCategory)
}

func printUndoSummary(out io.Writer, dir string, s Summary, dryRun bool) {
	verb := "Restored"
	if dryRun {
		verb = "Dry run: would restore"
	}

	if s.Moved == 0 {
		prefix := "Nothing to undo in"
		if dryRun {
			prefix = "Dry run: nothing to undo in"
		}
		if s.Skipped > 0 {
			prefix = "No files were restored in"
			if dryRun {
				prefix = "Dry run: no files would be restored in"
			}
		}
		fmt.Fprintf(out, "%s %s.\n", prefix, dir)
		if s.Skipped > 0 {
			fmt.Fprintf(out, "Could not restore %s.\n", countLabel(s.Skipped, "file", "files"))
		}
		return
	}

	fmt.Fprintf(out, "%s %s in %s.\n", verb, countLabel(s.Moved, "file", "files"), dir)
	if s.Skipped > 0 {
		fmt.Fprintf(out, "Could not restore %s.\n", countLabel(s.Skipped, "file", "files"))
	}

	if len(s.ByCategory) == 0 {
		return
	}

	printCategoryBreakdown(out, s.ByCategory)
}

func printCategoryBreakdown(out io.Writer, byCategory map[string]int) {
	fmt.Fprintln(out, "By category:")

	categories := make([]string, 0, len(byCategory))
	for c := range byCategory {
		categories = append(categories, c)
	}
	sort.Strings(categories)

	for _, c := range categories {
		fmt.Fprintf(out, "  %-14s %d\n", c+":", byCategory[c])
	}
}

func countLabel(n int, singular, plural string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, singular)
	}
	return fmt.Sprintf("%d %s", n, plural)
}

func resolvedVersion() string {
	if version != "" && version != "dev" {
		return version
	}

	info, ok := debug.ReadBuildInfo()
	if !ok {
		return version
	}

	if info.Main.Version != "" && info.Main.Version != "(devel)" {
		return info.Main.Version
	}

	revision := buildSetting(info, "vcs.revision")
	if revision == "" {
		return version
	}

	shortRevision := revision
	if len(shortRevision) > 7 {
		shortRevision = shortRevision[:7]
	}

	suffix := shortRevision
	if buildSetting(info, "vcs.modified") == "true" {
		suffix += "-dirty"
	}

	if version == "" {
		return suffix
	}

	return strings.Join([]string{version, suffix}, "+")
}

func buildSetting(info *debug.BuildInfo, key string) string {
	for _, setting := range info.Settings {
		if setting.Key == key {
			return setting.Value
		}
	}
	return ""
}
