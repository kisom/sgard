# ARCHITECTURE.md

Design document for sgard (Shimmering Clarity Gardener), a dotfiles manager.

## Overview

sgard manages dotfiles by checkpointing them into a portable repository and
restoring them to their original locations. The repository is a single
directory that can live anywhere — local disk, USB drive, NFS mount — making
it portable between machines.

## Tech Stack

**Language: Go** (`github.com/kisom/sgard`)

- Static binaries by default, no runtime dependencies on target machines.
- First-class gRPC and protobuf support for the future remote mode.
- Standard library covers all core needs: file I/O (`os`, `path/filepath`),
  hashing (`crypto/sha256`), and cross-platform path handling.
- Trivial cross-compilation via `GOOS`/`GOARCH`.

**CLI framework: cobra**

**Manifest format: YAML** (via `gopkg.in/yaml.v3`)

- Human-readable and supports comments (unlike JSON).
- Natural syntax for lists of structured entries (unlike TOML's `[[array_of_tables]]`).
- File modes stored as quoted strings (`"0644"`) to avoid YAML's octal coercion.

## Repository Layout on Disk

A sgard repository is a single directory with this structure:

```
<repo>/
  manifest.yaml          # single manifest tracking all files
  .gitignore             # excludes blobs/ (created by sgard init)
  blobs/
    a1/b2/a1b2c3d4...   # content-addressable file storage
```

### Manifest Schema

```yaml
version: 1
created: "2026-03-23T12:00:00Z"
updated: "2026-03-23T14:30:00Z"
message: "pre-upgrade checkpoint"   # optional

files:
  - path: ~/.bashrc                 # original location (default restore target)
    hash: a1b2c3d4e5f6...          # SHA-256 of file contents
    type: file                      # file | directory | link
    mode: "0644"                    # permissions (quoted to avoid YAML coercion)
    updated: "2026-03-23T14:30:00Z" # last checkpoint time for this file

  - path: ~/.config/nvim
    type: directory
    mode: "0755"
    updated: "2026-03-23T14:30:00Z"
    # directories have no hash or blob — they're structural entries

  - path: ~/.vimrc
    type: link
    target: ~/.config/nvim/init.vim  # symlink target
    updated: "2026-03-23T14:30:00Z"
    # links have no hash or blob — just the target path

  - path: ~/.ssh/config
    hash: d4e5f6a1b2c3...
    type: file
    mode: "0600"
    updated: "2026-03-23T14:30:00Z"
```

### Blob Store

Files are stored by their SHA-256 hash in a two-level directory structure:

```
blobs/<first 2 hex chars>/<next 2 hex chars>/<full 64-char hash>
```

Example: a file with hash `a1b2c3d4e5...` is stored at `blobs/a1/b2/a1b2c3d4e5...`

Properties:
- **Deduplication**: identical files across different paths share one blob.
- **Rename-safe**: moving a dotfile to a new path updates only the manifest.
- **Integrity**: the filename *is* the expected hash — corruption is trivially detectable.
- **Directories and symlinks** are manifest-only entries. No blobs are stored for them.

## CLI Commands

All commands operate on a repository directory (default: `~/.sgard`, override with `--repo`).

### Phase 1 — Local

| Command | Description |
|---|---|
| `sgard init [--repo <path>]` | Create a new repository |
| `sgard add <path>...` | Track files; copies them into the blob store and adds manifest entries |
| `sgard remove <path>...` | Untrack files; removes manifest entries (blobs cleaned up on next checkpoint) |
| `sgard checkpoint [-m <message>]` | Re-hash all tracked files, store any changed blobs, update manifest |
| `sgard restore [<path>...] [--force]` | Restore files from manifest to their original locations |
| `sgard status` | Compare current files against manifest: modified, missing, ok |
| `sgard verify` | Check all blobs against manifest hashes (integrity check) |
| `sgard list` | List all tracked files |
| `sgard diff [<path>]` | Show content diff between current file and stored blob |

**Workflow example:**

```sh
# Initialize a repo on a USB drive
sgard init --repo /mnt/usb/dotfiles

# Track some files
sgard add ~/.bashrc ~/.gitconfig ~/.ssh/config --repo /mnt/usb/dotfiles

# Checkpoint current state
sgard checkpoint -m "initial" --repo /mnt/usb/dotfiles

# On a new machine, restore
sgard restore --repo /mnt/usb/dotfiles
```

### Phase 2 — Remote (Future)

| Command | Description |
|---|---|
| `sgard push` | Push checkpoint to remote gRPC server |
| `sgard pull` | Pull checkpoint from remote gRPC server |
| `sgard serve` | Run the gRPC daemon |

## Go Package Structure

```
sgard/
  cmd/sgard/              # CLI entry point — one file per command
    main.go               # cobra root command, --repo flag
    version.go            # sgard version (ldflags-injected)
    init.go add.go remove.go checkpoint.go
    restore.go status.go verify.go list.go diff.go

  garden/                 # Core business logic — one file per operation
    garden.go             # Garden struct, Init, Open, Add, Checkpoint, Status
    restore.go            # Restore with timestamp comparison and confirm callback
    remove.go verify.go list.go diff.go
    hasher.go             # SHA-256 file hashing
    e2e_test.go           # Full lifecycle integration test

  manifest/               # YAML manifest parsing
    manifest.go           # Manifest and Entry structs, Load/Save

  store/                  # Content-addressable blob storage
    store.go              # Store struct: Write/Read/Exists/Delete

  flake.nix               # Nix flake for building on NixOS
  .goreleaser.yaml        # GoReleaser config for releases
  .github/workflows/      # GitHub Actions release pipeline
```

### Key Architectural Rule

**The `garden` package contains all logic. The `cmd` package is pure CLI wiring.**

The `Garden` struct is the central coordinator:

```go
type Garden struct {
    manifest     *manifest.Manifest
    store        *store.Store
    root         string              // repository root directory
    manifestPath string
    clock        clockwork.Clock     // injectable for testing
}

func (g *Garden) Add(paths []string) error
func (g *Garden) Remove(paths []string) error
func (g *Garden) Checkpoint(message string) error
func (g *Garden) Restore(paths []string, force bool, confirm func(path string) bool) error
func (g *Garden) Status() ([]FileStatus, error)
func (g *Garden) Verify() ([]VerifyResult, error)
func (g *Garden) List() []manifest.Entry
func (g *Garden) Diff(path string) (string, error)
```

This separation means the future gRPC server calls the same `Garden` methods
as the CLI — no logic duplication.

## Design Decisions

**Paths in manifest use `~` unexpanded.** The `garden` package expands `~` to
`$HOME` at runtime. This makes the manifest portable across machines with
different usernames.

**No history.** Phase 1 stores only the latest checkpoint. For versioning,
place the repo under git — `sgard init` creates a `.gitignore` that excludes
`blobs/`. Blob durability (backup, replication) is deferred to a future phase.

**Per-file timestamps.** Each manifest entry records an `updated` timestamp
set at checkpoint time. On restore, if the manifest entry is newer than the
file on disk (by mtime), the restore proceeds without prompting. If the file
on disk is newer or the times match, sgard prompts for confirmation.
`--force` always skips the prompt.

**Atomic writes.** Checkpoint writes `manifest.yaml.tmp` then renames to
`manifest.yaml`. A crash cannot corrupt the manifest.

**Timestamp comparison truncates to seconds** for cross-platform filesystem
compatibility.
