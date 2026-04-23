# gotidy

A tiny command-line tool that sorts the files in a directory into subfolders
by type. Point it at a messy `Downloads` folder and it will neatly tuck every
file into `images/`, `documents/`, `code/`, and friends.

```
Downloads/                    Downloads/
├── photo.jpg                 ├── images/
├── report.pdf                │   └── photo.jpg
├── song.mp3         →        ├── documents/
├── script.go                 │   └── report.pdf
└── backup.zip                ├── audio/
                              │   └── song.mp3
                              ├── code/
                              │   └── script.go
                              └── archives/
                                  └── backup.zip
```

## Features

- Sorts files into nine categories: `images`, `documents`, `spreadsheets`,
  `presentations`, `videos`, `audio`, `archives`, `code`, and `other`.
- Safe by default — leaves subdirectories, hidden files, and symlinks alone,
  and never overwrites an existing file at the destination.
- `--dry-run` mode so you can preview every move before committing.
- Zero dependencies outside the Go standard library.

## Install

### From source (requires Go 1.21+)

```sh
git clone https://github.com/yourusername/gotidy.git
cd gotidy
go build -o gotidy .
```

Then move the `gotidy` binary somewhere on your `PATH` (for example
`/usr/local/bin` on macOS/Linux).

### With `go install`

```sh
go install github.com/yourusername/gotidy@latest
```

## Usage

```sh
gotidy [flags] [directory]
```

If no directory is given, the current working directory is used.

| Flag              | Description                                |
| ----------------- | ------------------------------------------ |
| `-n`, `--dry-run` | Preview what would move; don't touch files |
| `-v`, `--verbose` | Print every considered file                |
| `-V`, `--version` | Print the gotidy version and exit          |
| `-h`, `--help`    | Show the help message                      |

### Examples

Preview what would happen to your Downloads folder:

```sh
gotidy --dry-run ~/Downloads
```

Actually organize it, with verbose output:

```sh
gotidy -v ~/Downloads
```

Organize the current directory:

```sh
gotidy
```

## How it decides categories

Each file is categorised by its extension. The full mapping lives in
[`categories.go`](./categories.go); here are a few highlights:

| Category        | Example extensions                              |
| --------------- | ----------------------------------------------- |
| `images`        | `jpg`, `png`, `gif`, `svg`, `webp`              |
| `documents`     | `pdf`, `docx`, `md`, `txt`                      |
| `spreadsheets`  | `xlsx`, `csv`, `tsv`                            |
| `presentations` | `pptx`, `key`                                   |
| `videos`        | `mp4`, `mov`, `mkv`, `webm`                     |
| `audio`         | `mp3`, `wav`, `flac`, `m4a`                     |
| `archives`      | `zip`, `tar`, `gz`, `7z`                        |
| `code`          | `go`, `py`, `js`, `ts`, `html`, `json`, `yaml`  |
| `other`         | anything not listed above, and files with no ext |

Extension matching is case-insensitive, so `Photo.JPG` and `photo.jpg` land
in the same place.

## Safety

`gotidy` is deliberately conservative:

- It only moves top-level **regular files**. Subdirectories, symlinks, and
  device files are left in place.
- Hidden files (those whose name starts with a `.`) are never moved, so your
  `.env` and `.DS_Store` are safe.
- If a file with the same name already exists at the destination, `gotidy`
  skips it rather than overwriting.
- `--dry-run` lets you see exactly what would happen before committing.

That said, file moves are permanent — use `--dry-run` first on directories
whose contents you care about.

## Development

Run the test suite:

```sh
go test ./...
```

Run tests with coverage:

```sh
go test -cover ./...
```

Format and vet before committing:

```sh
go fmt ./...
go vet ./...
```

## Project layout

```
gotidy/
├── main.go             CLI entrypoint — flag parsing and summary output
├── organizer.go        Core logic — scanning a directory and moving files
├── categories.go       Mapping from file extensions to category folders
├── organizer_test.go   Unit tests for the organizer
├── categories_test.go  Unit tests for the categoriser
├── go.mod
├── LICENSE             MIT
└── README.md
```

## License

[MIT](./LICENSE)
