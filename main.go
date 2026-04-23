package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
)

var version = "alpha_0.0.5"

const usage = `gotidy - sort files in a directory into subfolders by type

Usage:
  gotidy [flags] [directory]
  gotidy --classify [flags] file [file ...]
  gotidy --list-categories [flags] [directory]

If you do not pass a directory, gotidy uses the current one.

What it does:
  - moves top-level files into folders like images/, documents/, code/, and more
  - can load custom category rules from .gotidy.yaml, .gotidy.yml, or .gotidy.json
  - can learn extension and filename patterns over time with --learn
  - can use adaptive heuristics and optional content hints with --adaptive
  - can filter files with --include, --exclude, and .gotidyignore
  - can bucket files by date or large-file size
  - can back up the target directory before moving anything
  - can undo the last real run with --undo

Flags:
  --adaptive              Use learned preferences and directory heuristics
  --backup                Create a zip backup before organizing
  --by-date               Add YYYY/MM subdirectories under each destination
  --by-size               Prefix large files with large_files/
  --classify              Classify filenames passed as arguments without moving anything
  --config PATH           Load a custom gotidy config from PATH
  --content-hints         Inspect ambiguous text-like files for safer content-based hints
  --exclude PATTERNS      Skip matching filenames (comma-separated globs)
  --ignore-file PATH      Load ignore patterns from PATH instead of .gotidyignore
  --include PATTERNS      Only organize matching filenames (comma-separated globs)
  --interactive           Prompt before moving or overwriting files
  --json                  Print structured JSON output
  --large-files-over SIZE Treat files at or above SIZE as large (default: 100MB)
  --learn                 Update local learning data from successful real runs
  --list-categories       Show the active category-to-extension mapping
  -n, --dry-run           Show what would move without changing anything
  --overwrite             Overwrite colliding destinations (interactive only)
  --rename                Rename colliding destinations with _N suffixes
  --rename-on-collision   Alias for --rename
  --skip                  Skip colliding destinations (default)
  --stats                 Print extended size/count stats
  -u, --undo              Undo the last organize run in this directory
  --update                Install the newest gotidy from main
  -v, --verbose           Print each decision as gotidy works
  -V, --version           Show the gotidy version and exit
  -h, --help              Show this help text

Examples:
  gotidy ~/Downloads
  gotidy --adaptive --classify budget.txt notes.txt
  gotidy --learn --config ~/.config/gotidy/work.yaml ~/Downloads
  gotidy --config ~/.config/gotidy/work.yaml ~/Downloads
  gotidy --include "*.pdf,*.docx" --by-date ~/Downloads
  gotidy --adaptive --content-hints ~/Downloads
  gotidy --rename --by-size --large-files-over 250MB ~/Downloads
  gotidy --interactive --overwrite ~/Downloads
  gotidy --backup --stats ~/Downloads
  gotidy --classify photo.jpg report.pdf Dockerfile
  gotidy --list-categories ~/Downloads
  gotidy --undo ~/Downloads
`

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	return runWithIO(args, os.Stdin, stdout, stderr)
}

func runWithIO(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("gotidy", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() { fmt.Fprint(stderr, usage) }

	var (
		adaptive          bool
		backup            bool
		byDate            bool
		bySize            bool
		classify          bool
		contentHints      bool
		configPath        string
		dryRun            bool
		excludeRaw        string
		ignoreFilePath    string
		includeRaw        string
		interactive       bool
		jsonOutput        bool
		largeFilesOverRaw string
		learn             bool
		listCategories    bool
		overwrite         bool
		rename            bool
		renameAlias       bool
		skip              bool
		stats             bool
		undo              bool
		update            bool
		verbose           bool
		showVersion       bool
	)

	fs.BoolVar(&adaptive, "adaptive", false, "use learned preferences and directory heuristics")
	fs.BoolVar(&backup, "backup", false, "create a zip backup before organizing")
	fs.BoolVar(&byDate, "by-date", false, "add YYYY/MM subdirectories under each destination")
	fs.BoolVar(&bySize, "by-size", false, "prefix large files with large_files/")
	fs.BoolVar(&classify, "classify", false, "classify filenames without moving anything")
	fs.BoolVar(&contentHints, "content-hints", false, "inspect ambiguous text-like files for content hints")
	fs.StringVar(&configPath, "config", "", "load a custom gotidy config from PATH")
	fs.BoolVar(&dryRun, "dry-run", false, "preview moves without touching files")
	fs.BoolVar(&dryRun, "n", false, "preview moves without touching files (shorthand)")
	fs.StringVar(&excludeRaw, "exclude", "", "skip matching filenames (comma-separated globs)")
	fs.StringVar(&ignoreFilePath, "ignore-file", "", "load ignore patterns from PATH instead of .gotidyignore")
	fs.StringVar(&includeRaw, "include", "", "only organize matching filenames (comma-separated globs)")
	fs.BoolVar(&interactive, "interactive", false, "prompt before moving files")
	fs.BoolVar(&jsonOutput, "json", false, "print structured JSON output")
	fs.StringVar(&largeFilesOverRaw, "large-files-over", "100MB", "threshold used by --by-size")
	fs.BoolVar(&learn, "learn", false, "update local learning data from successful real runs")
	fs.BoolVar(&listCategories, "list-categories", false, "show the active category mapping")
	fs.BoolVar(&overwrite, "overwrite", false, "overwrite colliding destinations")
	fs.BoolVar(&rename, "rename", false, "rename colliding destinations with _N suffixes")
	fs.BoolVar(&renameAlias, "rename-on-collision", false, "alias for --rename")
	fs.BoolVar(&skip, "skip", false, "skip colliding destinations")
	fs.BoolVar(&stats, "stats", false, "print extended size/count stats")
	fs.BoolVar(&undo, "undo", false, "undo the last organize run in this directory")
	fs.BoolVar(&undo, "u", false, "undo the last organize run in this directory (shorthand)")
	fs.BoolVar(&update, "update", false, "install the newest gotidy from main")
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

	if jsonOutput && verbose {
		return usageError(stderr, "--json cannot be combined with --verbose")
	}
	if jsonOutput && interactive {
		return usageError(stderr, "--json cannot be combined with --interactive")
	}

	duplicateStrategy, err := resolveDuplicateStrategy(skip, rename, renameAlias, overwrite)
	if err != nil {
		return usageError(stderr, err.Error())
	}
	if duplicateStrategy == DuplicateOverwrite && !interactive {
		return usageError(stderr, "--overwrite requires --interactive so each replacement is confirmed")
	}

	includePatterns := parsePatternList(includeRaw)
	excludePatterns := parsePatternList(excludeRaw)
	largeFileThreshold, err := parseSize(largeFilesOverRaw)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 2
	}

	if showVersion {
		if adaptive || backup || byDate || bySize || classify || contentHints || configPath != "" || dryRun || excludeRaw != "" || ignoreFilePath != "" || includeRaw != "" || interactive || learn || listCategories || overwrite || rename || renameAlias || skip || stats || undo || update || verbose || fs.NArg() > 0 {
			return usageError(stderr, "--version cannot be combined with other flags or arguments")
		}
		if jsonOutput {
			if err := writeJSON(stdout, versionOutput{Mode: "version", Name: "gotidy", Version: version}); err != nil {
				fmt.Fprintf(stderr, "error: %v\n", err)
				return 1
			}
			return 0
		}
		fmt.Fprintf(stdout, "gotidy %s\n", version)
		return 0
	}

	if update {
		if adaptive || backup || byDate || bySize || classify || contentHints || configPath != "" || dryRun || excludeRaw != "" || ignoreFilePath != "" || includeRaw != "" || interactive || jsonOutput || learn || listCategories || overwrite || rename || renameAlias || skip || stats || undo || verbose || fs.NArg() > 0 {
			return usageError(stderr, "--update cannot be combined with other flags or arguments")
		}
		if err := selfUpdate(stdout, stderr); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		return 0
	}

	if classify {
		if backup || byDate || bySize || dryRun || excludeRaw != "" || ignoreFilePath != "" || includeRaw != "" || interactive || listCategories || overwrite || rename || renameAlias || skip || stats || undo || verbose {
			return usageError(stderr, "--classify cannot be combined with organize or undo flags")
		}
		if fs.NArg() == 0 {
			return usageError(stderr, "--classify needs at least one filename")
		}

		resolver, loadedConfig, err := loadResolver(".", configPath)
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		categorizer, err := NewCategorizer(".", resolver, adaptive, learn, contentHints)
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}

		results := make([]classificationResult, 0, fs.NArg())
		for _, name := range fs.Args() {
			decision, err := categorizer.Classify(name)
			if err != nil {
				fmt.Fprintf(stderr, "error: %v\n", err)
				return 1
			}
			results = append(results, classificationResult{
				Input:       name,
				Category:    decision.Rule.Name,
				Destination: decision.Rule.Destination,
				Source:      decision.Source,
				Reason:      decision.Reason,
				Confidence:  decision.Confidence,
			})
		}

		if jsonOutput {
			payload := classifyOutput{
				Mode:         "classify",
				Results:      results,
				LearningPath: categorizer.LearningPath(),
			}
			if loadedConfig != nil {
				payload.ConfigPath = loadedConfig.Path
			}
			if err := writeJSON(stdout, payload); err != nil {
				fmt.Fprintf(stderr, "error: %v\n", err)
				return 1
			}
			return 0
		}

		printClassifications(stdout, results, loadedConfig, categorizer)
		return 0
	}

	if listCategories {
		if backup || byDate || bySize || dryRun || excludeRaw != "" || ignoreFilePath != "" || includeRaw != "" || interactive || overwrite || rename || renameAlias || skip || stats || undo || verbose {
			return usageError(stderr, "--list-categories cannot be combined with organize or undo flags")
		}
		if fs.NArg() > 1 {
			return usageError(stderr, "--list-categories accepts at most one directory argument")
		}

		root := "."
		if fs.NArg() == 1 {
			root = fs.Arg(0)
		}

		resolver, loadedConfig, err := loadResolver(root, configPath)
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		categorizer, err := NewCategorizer(root, resolver, adaptive, learn, contentHints)
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		definitions := resolver.Definitions()
		learnedDefinitions := categorizer.LearningDefinitions()

		if jsonOutput {
			payload := categoryListOutput{
				Mode:              "list-categories",
				Categories:        definitions,
				LearnedCategories: learnedDefinitions,
				LearningPath:      categorizer.LearningPath(),
				AdaptiveRequested: adaptive || learn || contentHints,
			}
			if loadedConfig != nil {
				payload.ConfigPath = loadedConfig.Path
			}
			if err := writeJSON(stdout, payload); err != nil {
				fmt.Fprintf(stderr, "error: %v\n", err)
				return 1
			}
			return 0
		}

		printCategoryList(stdout, definitions, learnedDefinitions, loadedConfig, categorizer)
		return 0
	}

	if undo {
		if adaptive || backup || byDate || bySize || contentHints || excludeRaw != "" || ignoreFilePath != "" || includeRaw != "" || interactive || learn || overwrite || rename || renameAlias || skip {
			return usageError(stderr, "--undo cannot be combined with organize-only flags")
		}
	}

	dir := "."
	switch fs.NArg() {
	case 0:
	case 1:
		dir = fs.Arg(0)
	default:
		return usageError(stderr, "gotidy accepts at most one directory argument")
	}

	opts := Options{
		DryRun:             dryRun,
		Verbose:            verbose,
		Stats:              stats,
		Interactive:        interactive,
		Backup:             backup,
		ByDate:             byDate,
		BySize:             bySize,
		CollisionStrategy:  duplicateStrategy,
		LargeFileThreshold: largeFileThreshold,
		Logf: func(format string, args ...any) {
			fmt.Fprintf(stdout, format+"\n", args...)
		},
	}

	if undo {
		opts.Resolver = DefaultCategoryResolver()
	} else {
		resolver, loadedConfig, err := loadResolver(dir, configPath)
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}

		ignorePatterns, loadedIgnoreFile, err := loadIgnorePatterns(dir, ignoreFilePath)
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}

		opts.IncludePatterns = includePatterns
		opts.ExcludePatterns = excludePatterns
		opts.IgnorePatterns = ignorePatterns
		opts.Resolver = resolver
		categorizer, err := NewCategorizer(dir, resolver, adaptive, learn, contentHints)
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		opts.Categorizer = categorizer
		if loadedConfig != nil {
			opts.ConfigPath = loadedConfig.Path
		}
		if loadedIgnoreFile != "" {
			opts.IgnoreFilePath = loadedIgnoreFile
		}
	}
	if interactive {
		opts.Prompt = makePrompter(stdin, stdout)
	}

	var summary Summary
	if undo {
		summary, err = Undo(dir, opts)
	} else {
		summary, err = Organize(dir, opts)
	}
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}

	if jsonOutput {
		mode := "organize"
		if undo {
			mode = "undo"
		}
		if err := writeJSON(stdout, summaryOutput{
			Mode:            mode,
			Directory:       dir,
			DryRun:          dryRun,
			Examined:        summary.Examined,
			Moved:           summary.Moved,
			AdaptiveMatches: summary.AdaptiveMatches,
			Renamed:         summary.Renamed,
			Overwritten:     summary.Overwritten,
			Skipped:         summary.Skipped,
			Filtered:        summary.Filtered,
			TotalSizeBytes:  summary.TotalBytes,
			ByCategory:      summary.ByCategory,
			ByCategoryBytes: summary.ByCategoryBytes,
			BackupPath:      summary.BackupPath,
			ConfigPath:      summary.ConfigPath,
			IgnoreFilePath:  summary.IgnoreFilePath,
			LearningPath:    summary.LearningPath,
		}); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		return 0
	}

	if undo {
		printUndoSummary(stdout, dir, summary, dryRun, stats)
	} else {
		printSummary(stdout, dir, summary, dryRun, stats)
	}
	return 0
}

func resolveDuplicateStrategy(skip, rename, renameAlias, overwrite bool) (DuplicateStrategy, error) {
	selected := make([]DuplicateStrategy, 0, 3)
	if skip {
		selected = append(selected, DuplicateSkip)
	}
	if rename || renameAlias {
		selected = append(selected, DuplicateRename)
	}
	if overwrite {
		selected = append(selected, DuplicateOverwrite)
	}
	if len(selected) > 1 {
		return "", fmt.Errorf("choose only one of --skip, --rename, or --overwrite")
	}
	if len(selected) == 0 {
		return DuplicateSkip, nil
	}
	return selected[0], nil
}

func loadResolver(root, configPath string) (CategoryResolver, *LoadedConfig, error) {
	loadedConfig, err := LoadConfig(root, configPath)
	if err != nil {
		return CategoryResolver{}, nil, err
	}

	if loadedConfig == nil {
		return DefaultCategoryResolver(), nil, nil
	}

	resolver, err := NewCategoryResolver(&loadedConfig.Config)
	if err != nil {
		return CategoryResolver{}, nil, fmt.Errorf("cannot build category resolver from %q: %w", loadedConfig.Path, err)
	}
	return resolver, loadedConfig, nil
}

func usageError(stderr io.Writer, message string) int {
	fmt.Fprintf(stderr, "error: %s\n", message)
	fmt.Fprint(stderr, usage)
	return 2
}

func printSummary(out io.Writer, dir string, s Summary, dryRun, stats bool) {
	verb := "Organized"
	if dryRun {
		verb = "Dry run: would move"
	}

	if s.Moved == 0 {
		prefix := "Nothing to do in"
		if dryRun {
			prefix = "Dry run: nothing to move in"
		}
		if s.Skipped > 0 {
			prefix = "No files were moved in"
			if dryRun {
				prefix = "Dry run: no files would be moved in"
			}
		}
		fmt.Fprintf(out, "%s %s.\n", prefix, dir)
		if s.Skipped > 0 {
			fmt.Fprintf(out, "Skipped %s.\n", countLabel(s.Skipped, "file", "files"))
		}
		if s.BackupPath != "" {
			fmt.Fprintf(out, "Created backup at %s.\n", s.BackupPath)
		}
		if stats {
			printStats(out, s)
		}
		printContext(out, s)
		return
	}

	fmt.Fprintf(out, "%s %s in %s.\n", verb, countLabel(s.Moved, "file", "files"), dir)
	if s.Skipped > 0 {
		fmt.Fprintf(out, "Skipped %s.\n", countLabel(s.Skipped, "file", "files"))
	}
	if s.Renamed > 0 {
		printRenameSummary(out, s.Renamed, dryRun)
	}
	if s.AdaptiveMatches > 0 {
		printAdaptiveSummary(out, s.AdaptiveMatches, dryRun)
	}
	if s.Overwritten > 0 {
		printOverwriteSummary(out, s.Overwritten, dryRun)
	}
	if s.BackupPath != "" {
		fmt.Fprintf(out, "Created backup at %s.\n", s.BackupPath)
	}
	if stats {
		printStats(out, s)
	}
	if len(s.ByCategory) > 0 {
		printCategoryBreakdown(out, s.ByCategory, s.ByCategoryBytes, stats)
	}
	printContext(out, s)
}

func printUndoSummary(out io.Writer, dir string, s Summary, dryRun, stats bool) {
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
		if stats {
			printStats(out, s)
		}
		return
	}

	fmt.Fprintf(out, "%s %s in %s.\n", verb, countLabel(s.Moved, "file", "files"), dir)
	if s.Skipped > 0 {
		fmt.Fprintf(out, "Could not restore %s.\n", countLabel(s.Skipped, "file", "files"))
	}
	if stats {
		printStats(out, s)
	}
	if len(s.ByCategory) > 0 {
		printCategoryBreakdown(out, s.ByCategory, s.ByCategoryBytes, stats)
	}
}

func printStats(out io.Writer, s Summary) {
	if s.Examined > 0 {
		fmt.Fprintf(out, "Examined %s.\n", countLabel(s.Examined, "entry", "entries"))
	}
	if s.Filtered > 0 {
		fmt.Fprintf(out, "Filtered %s.\n", countLabel(s.Filtered, "entry", "entries"))
	}
	if s.AdaptiveMatches > 0 {
		fmt.Fprintf(out, "Adaptive matches: %s.\n", countLabel(s.AdaptiveMatches, "file", "files"))
	}
	if s.TotalBytes > 0 {
		fmt.Fprintf(out, "Total size organized: %s.\n", formatBytes(s.TotalBytes))
	}
}

func printContext(out io.Writer, s Summary) {
	if s.ConfigPath != "" {
		fmt.Fprintf(out, "Loaded config from %s.\n", s.ConfigPath)
	}
	if s.IgnoreFilePath != "" {
		fmt.Fprintf(out, "Loaded ignore rules from %s.\n", s.IgnoreFilePath)
	}
	if s.LearningPath != "" {
		fmt.Fprintf(out, "Learning data at %s.\n", s.LearningPath)
	}
}

func printCategoryBreakdown(out io.Writer, byCategory map[string]int, byCategoryBytes map[string]int64, showBytes bool) {
	fmt.Fprintln(out, "By category:")

	categories := make([]string, 0, len(byCategory))
	for category := range byCategory {
		categories = append(categories, category)
	}
	sort.Strings(categories)

	for _, category := range categories {
		if showBytes {
			fmt.Fprintf(out, "  %-14s %d (%s)\n", category+":", byCategory[category], formatBytes(byCategoryBytes[category]))
			continue
		}
		fmt.Fprintf(out, "  %-14s %d\n", category+":", byCategory[category])
	}
}

func printRenameSummary(out io.Writer, renamed int, dryRun bool) {
	verb := "Renamed"
	if dryRun {
		verb = "Dry run: would rename"
	}
	fmt.Fprintf(out, "%s %s to avoid collisions.\n", verb, countLabel(renamed, "file", "files"))
}

func printOverwriteSummary(out io.Writer, overwritten int, dryRun bool) {
	verb := "Overwrote"
	if dryRun {
		verb = "Dry run: would overwrite"
	}
	fmt.Fprintf(out, "%s %s after collision checks.\n", verb, countLabel(overwritten, "file", "files"))
}

func printAdaptiveSummary(out io.Writer, adaptiveMatches int, dryRun bool) {
	verb := "Applied adaptive categorization to"
	if dryRun {
		verb = "Dry run: would apply adaptive categorization to"
	}
	fmt.Fprintf(out, "%s %s.\n", verb, countLabel(adaptiveMatches, "file", "files"))
}

func printCategoryList(out io.Writer, categories, learnedCategories []CategoryDefinition, loadedConfig *LoadedConfig, categorizer *Categorizer) {
	fmt.Fprintln(out, "Categories:")
	for _, category := range categories {
		label := category.Name + ":"
		if category.Destination != "" && category.Destination != category.Name {
			label = category.Name + " -> " + category.Destination + ":"
		}
		if len(category.Extensions) == 0 {
			fmt.Fprintf(out, "  %-28s no mapped extensions\n", label)
			continue
		}
		fmt.Fprintf(out, "  %-28s %s\n", label, strings.Join(category.Extensions, ", "))
	}
	if len(learnedCategories) > 0 {
		fmt.Fprintln(out, "Learned preferences:")
		for _, category := range learnedCategories {
			fmt.Fprintf(out, "  %-28s %s\n", category.Name+" -> "+category.Destination+":", strings.Join(category.Extensions, ", "))
		}
	}
	if loadedConfig != nil {
		fmt.Fprintf(out, "Loaded config from %s.\n", loadedConfig.Path)
	}
	if categorizer != nil && categorizer.LearningPath() != "" {
		fmt.Fprintf(out, "Learning data at %s.\n", categorizer.LearningPath())
	}
}

func printClassifications(out io.Writer, results []classificationResult, loadedConfig *LoadedConfig, categorizer *Categorizer) {
	for _, result := range results {
		line := fmt.Sprintf("%s: %s", result.Input, result.Category)
		if result.Destination != "" && result.Destination != result.Category {
			line = fmt.Sprintf("%s -> %s", line, result.Destination)
		}
		if result.Source != "" && result.Source != "builtin" {
			line = fmt.Sprintf("%s (%s", line, result.Source)
			if result.Reason != "" {
				line = fmt.Sprintf("%s: %s", line, result.Reason)
			}
			line += ")"
		}
		fmt.Fprintln(out, line)
	}
	if loadedConfig != nil {
		fmt.Fprintf(out, "Loaded config from %s.\n", loadedConfig.Path)
	}
	if categorizer != nil && categorizer.LearningPath() != "" {
		fmt.Fprintf(out, "Learning data at %s.\n", categorizer.LearningPath())
	}
}

func countLabel(n int, singular, plural string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, singular)
	}
	return fmt.Sprintf("%d %s", n, plural)
}

func writeJSON(out io.Writer, v any) error {
	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func makePrompter(in io.Reader, out io.Writer) func(prompt string) (string, error) {
	reader := bufio.NewReader(in)
	return func(prompt string) (string, error) {
		if _, err := fmt.Fprint(out, prompt); err != nil {
			return "", err
		}
		line, err := reader.ReadString('\n')
		if err != nil && !errors.Is(err, io.EOF) {
			return "", err
		}
		return strings.TrimSpace(line), nil
	}
}

type summaryOutput struct {
	Mode            string           `json:"mode"`
	Directory       string           `json:"directory"`
	DryRun          bool             `json:"dry_run"`
	Examined        int              `json:"examined"`
	Moved           int              `json:"moved"`
	AdaptiveMatches int              `json:"adaptive_matches"`
	Renamed         int              `json:"renamed"`
	Overwritten     int              `json:"overwritten"`
	Skipped         int              `json:"skipped"`
	Filtered        int              `json:"filtered"`
	TotalSizeBytes  int64            `json:"total_size_bytes"`
	ByCategory      map[string]int   `json:"by_category"`
	ByCategoryBytes map[string]int64 `json:"by_category_bytes"`
	BackupPath      string           `json:"backup_path,omitempty"`
	ConfigPath      string           `json:"config_path,omitempty"`
	IgnoreFilePath  string           `json:"ignore_file_path,omitempty"`
	LearningPath    string           `json:"learning_path,omitempty"`
}

type categoryListOutput struct {
	Mode              string               `json:"mode"`
	ConfigPath        string               `json:"config_path,omitempty"`
	LearningPath      string               `json:"learning_path,omitempty"`
	AdaptiveRequested bool                 `json:"adaptive_requested"`
	Categories        []CategoryDefinition `json:"categories"`
	LearnedCategories []CategoryDefinition `json:"learned_categories,omitempty"`
}

type classificationResult struct {
	Input       string  `json:"input"`
	Category    string  `json:"category"`
	Destination string  `json:"destination"`
	Source      string  `json:"source,omitempty"`
	Reason      string  `json:"reason,omitempty"`
	Confidence  float64 `json:"confidence,omitempty"`
}

type classifyOutput struct {
	Mode         string                 `json:"mode"`
	ConfigPath   string                 `json:"config_path,omitempty"`
	LearningPath string                 `json:"learning_path,omitempty"`
	Results      []classificationResult `json:"results"`
}

type versionOutput struct {
	Mode    string `json:"mode"`
	Name    string `json:"name"`
	Version string `json:"version"`
}
