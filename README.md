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

| Flag | Description |
| --- | --- |
| `--backup` | Create a zip backup before organizing |
| `--by-date` | Add `YYYY/MM` subdirectories under each destination |
| `--by-size` | Move large files under `large_files/` |
| `--classify` | Classify the provided filenames without moving anything |
| `--config PATH` | Load a custom config file explicitly |
| `--exclude PATTERNS` | Skip matching filenames |
| `--ignore-file PATH` | Load ignore patterns from a custom path |
| `--include PATTERNS` | Only move matching filenames |
| `--interactive` | Prompt before moving or overwriting files |
| `--json` | Print machine-readable JSON output |
| `--large-files-over SIZE` | Threshold used with `--by-size` |
| `--list-categories` | Show the active category and extension mapping |
| `-n`, `--dry-run` | Show what would move without changing anything |
| `--overwrite` | Overwrite colliding destinations after interactive confirmation |
| `--rename` | Rename colliding destinations with `_N` suffixes |
| `--rename-on-collision` | Alias for `--rename` |
| `--skip` | Skip colliding destinations |
| `--stats` | Print extra count and size statistics |
| `-u`, `--undo` | Restore the last real run in this directory |
| `--update` | Install the newest `main` branch build with `go install` |
| `-v`, `--verbose` | Print each decision as gotidy works |
| `-V`, `--version` | Show the version and exit |
| `-h`, `--help` | Show the help text |

`--json` works with normal organize runs, `--dry-run`, `--undo`,
`--classify`, `--list-categories`, and `--version`.

`--overwrite` requires `--interactive`, so every replacement is confirmed.

## Config format

`gotidy` looks for these files in the target directory unless you pass
`--config`:

- `.gotidy.yaml`
- `.gotidy.yml`
- `.gotidy.json`

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
