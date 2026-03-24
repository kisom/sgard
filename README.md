# sgard — Shimmering Clarity Gardener

A dotfiles manager that checkpoints files into a portable repository and
restores them on demand.

The repository is a single directory that can live anywhere — local disk,
USB drive, NFS mount — making it portable between machines.

## Installation

Homebrew:

```
brew tap kisom/homebrew-tap
brew install sgard
```

From source:

```
git clone https://github.com/kisom/sgard && cd sgard
go build -o sgard ./cmd/sgard
```

Or install into `$GOBIN`:

```
go install github.com/kisom/sgard/cmd/sgard@latest
```

Binaries are also available on the
[releases page](https://github.com/kisom/sgard/releases).

## Quick start

```sh
# Initialize a repo (default: ~/.sgard)
sgard init

# Track some dotfiles
sgard add ~/.bashrc ~/.gitconfig ~/.ssh/config

# Checkpoint current state
sgard checkpoint -m "initial"

# Check what's changed
sgard status

# Restore from the repo
sgard restore
```

Use `--repo` to put the repository somewhere else, like a USB drive:

```sh
sgard init --repo /mnt/usb/dotfiles
sgard add ~/.bashrc --repo /mnt/usb/dotfiles
sgard restore --repo /mnt/usb/dotfiles
```

## Commands

| Command | Description |
|---|---|
| `init` | Create a new repository |
| `add <path>...` | Track files, directories, or symlinks |
| `remove <path>...` | Stop tracking files |
| `checkpoint [-m msg]` | Re-hash tracked files and update the manifest |
| `restore [path...] [-f]` | Restore files to their original locations |
| `status` | Show which tracked files have changed |
| `diff <path>` | Show content diff between stored and current file |
| `list` | List all tracked files |
| `verify` | Check blob store integrity against manifest hashes |
| `version` | Print the version |

## How it works

sgard stores files in a content-addressable blob store keyed by SHA-256.
A YAML manifest tracks each file's original path, hash, type, permissions,
and timestamp.

```
~/.sgard/
  manifest.yaml        # human-readable manifest
  blobs/
    a1/b2/a1b2c3d4...  # file contents stored by hash
```

On `restore`, sgard compares the manifest timestamp against the file's
mtime. If the manifest is newer, the file is restored without prompting.
Otherwise, sgard asks for confirmation (`--force` skips the prompt).

Paths under `$HOME` are stored as `~/...` in the manifest, making it
portable across machines with different usernames.

See [ARCHITECTURE.md](ARCHITECTURE.md) for full design details.