# gotidy

`gotidy` is a small command-line tool for cleaning up a messy folder.
Point it at something like `~/Downloads`, and it will sort the loose files
into folders such as `images/`, `documents/`, `audio/`, and `code/`.

It is meant to be boring in the best way: fast, predictable, and careful
about not touching things it should leave alone.

```text
Before                        After
Downloads/                    Downloads/
├── photo.jpg                 ├── images/
├── report.pdf                │   └── photo.jpg
├── song.mp3         ->       ├── documents/
├── script.go                 │   └── report.pdf
└── backup.zip                ├── audio/
                              │   └── song.mp3
                              ├── code/
                              │   └── script.go
                              └── archives/
                                  └── backup.zip
```

## Why use it

- Cleans up a folder in one command.
- Gives you a `--dry-run` mode before it changes anything.
- Lets you undo the last real run if you need to put things back.
- Can load custom categories from `.gotidy.yaml`, `.gotidy.yml`, or `.gotidy.json`.
- Can switch named profiles from `.gotidy.profiles.yaml` for work, creative, family, or team setups.
- Can learn local preferences over time with `--learn`.
- Can reuse learned preferences and directory heuristics with `--adaptive`.
- Can inspect ambiguous text-like files with `--content-hints`.
- Can skip, rename, or interactively overwrite duplicate destinations.
- Can filter with `--include`, `--exclude`, and `.gotidyignore`.
- Can bucket files by date and large-file size.
- Can create a zip backup before reorganizing.
- Can print JSON for scripting with `--json`.
- Can inspect active categories with `--list-categories` and `--classify`.
- Leaves hidden files, subdirectories, and symlinks alone.
- Uses only the Go standard library.

## Install

### `go install`

```sh
go install github.com/xqpeakx/gotidy@latest
```

This installs from GitHub and the Go module proxy, not from your local clone.
If you just changed code in a checkout and want to install that exact local
state, run this inside the repo instead:

```sh
go install .
```

By default, Go installs the binary to `$(go env GOPATH)/bin` unless you have
set `GOBIN`.

If that directory is not already on your `PATH`, add it:

```sh
echo 'export PATH="$PATH:$(go env GOPATH)/bin"' >> ~/.zshrc
source ~/.zshrc
```

If you use `bash`, add the same line to `~/.bashrc` instead.

### Build from source

```sh
git clone https://github.com/xqpeakx/gotidy.git
cd gotidy
go build -o gotidy .
```

Then move the binary to a directory on your `PATH`. For example:

```sh
sudo mv gotidy /usr/local/bin/gotidy
```

If `/usr/local/bin` is not on your `PATH`, add it:

```sh
echo 'export PATH="$PATH:/usr/local/bin"' >> ~/.zshrc
source ~/.zshrc
```

Check that it worked:

```sh
gotidy --version
```

## Quick start

See what would happen first:

```sh
gotidy --dry-run ~/Downloads
```

If the preview looks right, run it for real:

```sh
gotidy ~/Downloads
```

If you want to see every decision as it happens:

```sh
gotidy --verbose ~/Downloads
```

If you want custom categories and destinations:

```sh
cat > ~/Downloads/.gotidy.yaml <<'EOF'
categories:
  photos:
    extensions: [jpg, png, raw, dng]
    destination: Projects/Photography
  work_docs:
    extensions: [pdf, docx]
    destination: Work/Documents
EOF

gotidy ~/Downloads
```

If you want quick profile switching with shared defaults:

```sh
cat > ~/Projects/.gotidy.profiles.yaml <<'EOF'
profiles:
  work:
    backup: true
    duplicate_strategy: rename
    include: [*.pdf, *.docx, *.csv]
    categories:
      reports:
        extensions: [pdf]
        destination: Work/Reports
  creative:
    by_date: true
    categories:
      renders:
        extensions: [blend, psd, png]
        destination: Creative/Renders
EOF

gotidy --profile work ~/Projects
gotidy --profile creative --list-categories ~/Projects
```

If you want gotidy to learn your local preferences and reuse them later:

```sh
gotidy --learn --config ~/.config/gotidy/work.yaml ~/Downloads
gotidy --adaptive ~/Downloads
```

If you want cautious content-based hints for ambiguous text files:

```sh
gotidy --adaptive --content-hints ~/Downloads
gotidy --adaptive --classify budget.txt notes.txt
```

If you want smarter duplicate handling:

```sh
gotidy --rename ~/Downloads
gotidy --interactive --overwrite ~/Downloads
```

If you want date and size bucketing:

```sh
gotidy --by-date ~/Downloads
gotidy --by-size --large-files-over 250MB ~/Downloads
```

If you want finer control over what gets moved:

```sh
gotidy --include "*.pdf,*.docx" ~/Downloads
gotidy --exclude "*.tmp,*.crdownload" ~/Downloads
```

If you want a backup and richer reporting:

```sh
gotidy --backup --stats ~/Downloads
gotidy --dry-run --json ~/Downloads
```

If you need to restore the last real run:

```sh
gotidy --undo ~/Downloads
```

If you want to inspect classification without moving anything:

```sh
gotidy --classify photo.jpg report.pdf archive.tar.gz
gotidy --list-categories ~/Downloads
```

If you do not pass a directory, `gotidy` uses the current one.

## What it sorts

`gotidy` groups files into these folders:

- `images`
- `documents`
- `spreadsheets`
- `presentations`
- `videos`
- `audio`
- `archives`
- `code`
- `other`

It decides based on file extension. The built-in mapping lives in
[`categories.go`](./categories.go), and custom configs can override or extend
it per directory.

The built-in mapping covers a broader set of common formats now, including
things like `heic`, `avif`, `epub`, `numbers`, `m4v`, `opus`, `dmg`, `iso`,
`swift`, `kt`, `sql`, `vue`, and `svelte`.

You can inspect the exact live mapping from the binary itself, including any
custom config in the target directory:

```sh
gotidy --list-categories ~/Downloads
gotidy --classify holiday.heic book.epub app.swift
```

Some examples:

| Folder | Example extensions |
| --- | --- |
| `images` | `jpg`, `png`, `gif`, `svg`, `webp` |
| `documents` | `pdf`, `docx`, `md`, `txt` |
| `spreadsheets` | `xlsx`, `csv`, `tsv` |
| `presentations` | `pptx`, `key` |
| `videos` | `mp4`, `mov`, `mkv`, `webm` |
| `audio` | `mp3`, `wav`, `flac`, `m4a` |
| `archives` | `zip`, `tar`, `gz`, `7z` |
| `code` | `go`, `py`, `js`, `ts`, `html`, `json`, `yaml` |
| `other` | anything not listed above, plus files with no extension |

Extension matching is case-insensitive, so `Photo.JPG` and `photo.jpg` end up
in the same place.

## What it will not do

`gotidy` is intentionally conservative.

- It only moves top-level regular files.
- It leaves nested folders exactly as they are.
- It leaves hidden files alone, including things like `.env` and `.DS_Store`.
- It skips symlinks and other special files.
- It defaults to skipping duplicate destination names.
  Use `--rename` or `--interactive --overwrite` if you want a different policy.

If you are organizing a folder you care about, start with `--dry-run`.

`--undo` restores only the last real run for that directory. If new files now
occupy the original location, `gotidy` will leave those conflicts alone rather
than overwrite them.

`--backup` creates a hidden zip file like `.gotidy-backup-20260423-153000.zip`
in the target directory before any real move starts.

`.gotidyignore` is optional. It uses one pattern per line and supports simple
glob-style matches against top-level filenames:

```text
*.tmp
*.crdownload
Important Project/
Thumbs.db
```

## Usage

```text
gotidy [flags] [directory]
gotidy --classify [flags] file [file ...]
gotidy --list-categories [flags] [directory]
```

Flags:

| Flag | What it does | Example |
| --- | --- | --- |
| `--adaptive` | Use learned preferences plus directory/file heuristics before falling back to built-ins | `gotidy --adaptive ~/Downloads` |
| `--backup` | Create a zip snapshot before any real moves happen | `gotidy --backup ~/Downloads` |
| `--by-date` | Add `YYYY/MM` folders under the chosen destination | `gotidy --by-date ~/Downloads` |
| `--by-size` | Route large files under `large_files/` before their category path | `gotidy --by-size ~/Downloads` |
| `--classify` | Show how names would be categorized without moving anything | `gotidy --classify photo.jpg report.pdf` |
| `--config PATH` | Load a specific config file instead of auto-detecting one in the target directory | `gotidy --config ~/.config/gotidy/work.yaml ~/Downloads` |
| `--content-hints` | Inspect ambiguous text-like files for tabular/code/document hints | `gotidy --adaptive --content-hints ~/Downloads` |
| `--exclude PATTERNS` | Skip matching top-level filenames | `gotidy --exclude "*.tmp,*.crdownload" ~/Downloads` |
| `--ignore-file PATH` | Read ignore rules from a custom file instead of `.gotidyignore` | `gotidy --ignore-file ~/.config/gotidy/ignore ~/Downloads` |
| `--include PATTERNS` | Only move matching top-level filenames | `gotidy --include "*.pdf,*.docx" ~/Downloads` |
| `--interactive` | Prompt before moves and on collisions | `gotidy --interactive ~/Downloads` |
| `--json` | Emit machine-readable output for scripts and automation | `gotidy --dry-run --json ~/Downloads` |
| `--large-files-over SIZE` | Set the size threshold used by `--by-size` | `gotidy --by-size --large-files-over 250MB ~/Downloads` |
| `--learn` | Update `.gotidy-learning.json` from successful real runs | `gotidy --learn ~/Downloads` |
| `--list-categories` | Show the active extension map, including custom config overrides | `gotidy --list-categories ~/Downloads` |
| `--profile NAME` | Activate a named profile from `.gotidy.profiles.yaml` or `.json` | `gotidy --profile work ~/Downloads` |
| `-n`, `--dry-run` | Preview the move plan without touching the filesystem | `gotidy --dry-run ~/Downloads` |
| `--overwrite` | Replace an existing destination file after interactive confirmation | `gotidy --interactive --overwrite ~/Downloads` |
| `--rename` | Keep moving colliding files by adding `_1`, `_2`, and so on | `gotidy --rename ~/Downloads` |
| `--rename-on-collision` | Alias for `--rename` | `gotidy --rename-on-collision ~/Downloads` |
| `--skip` | Keep the default conservative collision policy | `gotidy --skip ~/Downloads` |
| `--stats` | Print extra counts and total size information | `gotidy --stats ~/Downloads` |
| `-u`, `--undo` | Restore the most recent real run in this directory | `gotidy --undo ~/Downloads` |
| `--update` | Install the newest `main` branch build with `go install` | `gotidy --update` |
| `-v`, `--verbose` | Print every decision as gotidy works | `gotidy --verbose ~/Downloads` |
| `-V`, `--version` | Show the version and exit | `gotidy --version` |
| `-h`, `--help` | Show the help text | `gotidy --help` |

`--json` works with normal organize runs, `--dry-run`, `--undo`,
`--classify`, `--list-categories`, and `--version`.

`--overwrite` requires `--interactive`, so every replacement is confirmed.

## Flag guide

### Preview and safety

Use these first when you are working on a directory you care about.

| Flag | When to use it | Typical result |
| --- | --- | --- |
| `--dry-run` | You want to preview the plan before moving anything | Prints the same summary and category breakdown you would get from a real run, but changes nothing |
| `--backup` | You want a filesystem snapshot beyond undo metadata | Creates a hidden zip file in the target directory before moving files |
| `--undo` | You want to reverse the last real organize run in that directory | Moves files back to their original top-level locations when those paths are still free |
| `--interactive` | You want to approve moves manually | Prompts for each move and for each collision decision |

### Adaptive categorization

These flags add cautious local intelligence without external services.

| Flag | Behavior | Example use |
| --- | --- | --- |
| `--learn` | Persist successful category and destination choices into `.gotidy-learning.json` | Train gotidy on a custom work folder layout over a few runs |
| `--adaptive` | Apply learned extension/token preferences plus directory/file relationship heuristics | Reuse those learned choices without keeping a config file around |
| `--content-hints` | Inspect ambiguous text-like files such as `.txt`, `.log`, or extensionless files | Turn `budget.txt` with CSV-like headers into `spreadsheets` instead of plain `documents` |

Adaptive matching is still conservative:

- explicit config wins first
- learned preferences come next
- then directory/file relationship heuristics
- built-in extension rules remain the fallback

### Duplicate handling

These flags decide what happens when the destination path already exists.

| Flag | Behavior | Example use |
| --- | --- | --- |
| `--skip` | Leave the source file in place | Safe default for mixed or unknown folders |
| `--rename` | Move the file with a suffix like `_1` | Good when you run gotidy repeatedly on the same directory |
| `--overwrite` | Replace the destination file after prompt confirmation | Useful when the newer top-level file should win |

### Filtering and scope

These flags narrow what gotidy touches.

| Flag | Behavior | Example use |
| --- | --- | --- |
| `--include` | Only move files that match one of the patterns | `gotidy --include "*.pdf,*.docx" ~/Downloads` |
| `--exclude` | Skip files that match one of the patterns | `gotidy --exclude "*.tmp,*.crdownload" ~/Downloads` |
| `.gotidyignore` / `--ignore-file` | Centralize skip rules you want to keep reusing | Good for things like `Thumbs.db`, `*.part`, or project folders |

### Layout control

These flags change where files land after category resolution.

| Flag | Behavior | Example use |
| --- | --- | --- |
| `--config` | Override built-in category mapping and destinations from a specific file | Load a central `~/.config/gotidy/work.yaml` instead of directory-local config |
| `--profile` | Activate a named preset of categories, filters, backup, and duplicate rules | Switch between `work` and `creative` folder layouts with one flag |
| `--by-date` | Add `YYYY/MM` subfolders under the destination | Split years of downloads into smaller monthly buckets |
| `--by-size` | Prefix large files with `large_files/` | Separate space-heavy media from normal category folders |
| `--large-files-over` | Tune the threshold used by `--by-size` | Treat anything above `250MB` as a large file |

### Profiles and shared defaults

Use profiles when you want multiple organization modes without rewriting flags
every time.

Typical uses:

- `gotidy --profile work ~/Downloads` for stricter filters, backups, and business-specific categories
- `gotidy --profile creative ~/Projects` for date bucketing and media-heavy destinations
- commit `.gotidy.profiles.yaml` into a repo so a family or team shares the same folder rules

Profile values act as defaults. Direct CLI flags still win. For example, if the
`work` profile enables backups but you run `gotidy --profile work --dry-run`,
you still get a dry run with no archive created.

### Reporting and inspection

These flags help when scripting, auditing, or just learning what gotidy would do.

| Flag | Behavior | Example use |
| --- | --- | --- |
| `--stats` | Print counts plus total organized size | Measure how much a cleanup run actually moved |
| `--json` | Emit structured output | Pipe into `jq`, shell scripts, or automation |
| `--classify` | Explain category resolution for sample names | Debug config rules without moving files |
| `--list-categories` | Print the active extension map | Verify what a directory-local config is overriding |
| `--verbose` | Log every keep/skip/move decision | Debug why a particular file did or did not move |

## Config format

`gotidy` looks for these files in the target directory unless you pass
`--config`:

- `.gotidy.yaml`
- `.gotidy.yml`
- `.gotidy.json`
- `.gotidy.profiles.yaml`
- `.gotidy.profiles.yml`
- `.gotidy.profiles.json`

If more than one of these exists, `gotidy` merges them. That lets you keep
basic categories in `.gotidy.yaml` and shared named profiles in
`.gotidy.profiles.yaml`.

YAML example:

```yaml
categories:
  photos:
    extensions: [jpg, png, raw, dng]
    destination: Projects/Photography
  work_docs:
    extensions:
      - pdf
      - docx
    destination: Work/Documents
```

JSON example:

```json
{
  "categories": {
    "photos": {
      "extensions": ["jpg", "png", "raw", "dng"],
      "destination": "Projects/Photography"
    },
    "work_docs": {
      "extensions": ["pdf", "docx"],
      "destination": "Work/Documents"
    }
  }
}
```

Profile YAML example:

```yaml
profiles:
  work:
    backup: true
    duplicate_strategy: rename
    include: [*.pdf, *.docx, *.csv]
    ignore_patterns:
      - keep-*
    categories:
      reports:
        extensions: [pdf]
        destination: Work/Reports
  creative:
    by_date: true
    by_size: true
    large_files_over: 250MB
    categories:
      renders:
        extensions: [blend, psd, png]
        destination: Creative/Renders
```

The profile file is a good candidate to commit into a repo when you want a
shared default layout for a team or household.

## Example output

Preview a run:

```sh
$ gotidy --dry-run ~/Downloads
Dry run: would move 4 files in /Users/alex/Downloads.
By category:
  archives:      1
  documents:     1
  images:        1
  videos:        1
```

Run with stats and a backup:

```sh
$ gotidy --backup --stats ~/Downloads
Organized 42 files in /Users/alex/Downloads.
Created backup at /Users/alex/Downloads/.gotidy-backup-20260423-153000.zip.
Examined 57 entries.
Filtered 8 entries.
Total size organized: 2.30 GB.
By category:
  documents:     15 (220 MB)
  images:        20 (1.40 GB)
  other:         2 (1.20 MB)
  videos:        5 (680 MB)
```

Run with a shared profile:

```sh
$ gotidy --profile work ~/Projects/Shared
Organized 3 files in /Users/alex/Projects/Shared.
Renamed 1 file to avoid collisions.
Created backup at /Users/alex/Projects/Shared/.gotidy-backup-20260423-153000.zip.
Loaded config from /Users/alex/Projects/Shared/.gotidy.profiles.yaml.
Active profile: work.
```

Classify filenames without moving them:

```sh
$ gotidy --classify photo.jpg report.pdf Dockerfile
photo.jpg: images
report.pdf: documents
Dockerfile: other
```

Inspect the active category mapping when a custom config is present:

```sh
$ gotidy --list-categories ~/Downloads
Categories:
  photos -> Projects/Photography: dng, jpg, png, raw
  work_docs -> Work/Documents:    docx, pdf
  images:                        avif, bmp, gif, heic, jpeg, jpg, png, webp
  other:                         no mapped extensions
Loaded config from /Users/alex/Downloads/.gotidy.yaml.
```

Inspect the active category mapping for a specific profile:

```sh
$ gotidy --profile creative --list-categories ~/Projects
Categories:
  renders -> Creative/Renders: blend, png, psd
  images:                     avif, bmp, gif, heic, jpeg, jpg, png, webp
  other:                      no mapped extensions
Loaded config from /Users/alex/Projects/.gotidy.profiles.yaml.
Active profile: creative.
```

Classify with adaptive content hints:

```sh
$ gotidy --adaptive --content-hints --classify budget.txt notes.txt
budget.txt: spreadsheets (content-hint: content looks like a delimited table)
notes.txt: documents
Learning data at /Users/alex/.gotidy-learning.json.
```

Teach gotidy a preference and reuse it later:

```sh
$ gotidy --learn --config ~/.config/gotidy/work.yaml ~/Downloads
Organized 6 files in /Users/alex/Downloads.
Learning data at /Users/alex/Downloads/.gotidy-learning.json.

$ gotidy --adaptive --classify sales.csv
sales.csv: data_files -> Work/Data (learned-extension: learned .csv -> Work/Data from prior runs)
Learning data at /Users/alex/Downloads/.gotidy-learning.json.
```

Use interactive overwrite mode on a collision:

```sh
$ gotidy --interactive --overwrite ~/Downloads
collision for report.pdf at documents/report.pdf [s]kip/[r]ename/[o]verwrite/[q]uit (default: overwrite): o
Organized 1 file in /Users/alex/Downloads.
Overwrote 1 file after collision checks.
```

Consume JSON from a script:

```sh
$ gotidy --dry-run --json ~/Downloads
{
  "mode": "organize",
  "directory": "/Users/alex/Downloads",
  "dry_run": true,
  "examined": 7,
  "moved": 4,
  "renamed": 0,
  "overwritten": 0,
  "skipped": 0,
  "filtered": 1,
  "total_size_bytes": 734003200,
  "by_category": {
    "documents": 1,
    "images": 1,
    "videos": 2
  },
  "by_category_bytes": {
    "documents": 84512,
    "images": 420331,
    "videos": 733498357
  }
}
```

## Updating

If you installed `gotidy` with Go, you can update it from the tool itself:

```sh
gotidy --update
```

This runs `go install github.com/xqpeakx/gotidy@main` with `GOPROXY=direct`,
so it tracks the newest pushed commit on `main` instead of waiting for the Go
module proxy to catch up.

It replaces the current `gotidy` executable you are running. That means if
your shell is using `/usr/local/bin/gotidy`, `--update` replaces that exact
binary instead of only updating `$(go env GOPATH)/bin/gotidy`.

`--update` still requires:

- `go` to be installed
- network access to GitHub
- write permission to the current `gotidy` binary path

## Development

Run the test suite:

```sh
go test ./...
```

Run with coverage:

```sh
go test -cover ./...
```

Format and vet before committing:

```sh
go fmt ./...
go vet ./...
```

## License

[MIT](./LICENSE)
