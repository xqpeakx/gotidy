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
- Leaves hidden files, subdirectories, and symlinks alone.
- Never overwrites an existing file in the destination folder.
- Uses only the Go standard library.

## Install

### `go install`

```sh
go install github.com/xqpeakx/gotidy@latest
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

If you need to restore the last real run in that directory:

```sh
gotidy --undo ~/Downloads
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

It decides based on file extension. The full mapping lives in
[`categories.go`](./categories.go).

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
- It skips a move if the destination already contains a file with that name.

If you are organizing a folder you care about, start with `--dry-run`.

`--undo` restores only the last real run for that directory. If new files now
occupy the original location, `gotidy` will leave those conflicts alone rather
than overwrite them.

## Usage

```text
gotidy [flags] [directory]
```

Flags:

| Flag | Description |
| --- | --- |
| `-n`, `--dry-run` | Show what would move without changing anything |
| `-u`, `--undo` | Restore the last real run in this directory |
| `-v`, `--verbose` | Print each decision as gotidy works |
| `-V`, `--version` | Show the version and exit |
| `-h`, `--help` | Show the help text |

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
