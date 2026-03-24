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
go build ./cmd/sgard ./cmd/sgardd
```

Or install into `$GOBIN`:

```
go install github.com/kisom/sgard/cmd/sgard@latest
go install github.com/kisom/sgard/cmd/sgardd@latest
```

NixOS (flake):

```
nix profile install github:kisom/sgard
```

Or add to your flake inputs and include `sgard.packages.${system}.default`
in your packages.

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

### Local

| Command | Description |
|---|---|
| `init` | Create a new repository |
| `add <path>...` | Track files, directories (recursed), or symlinks |
| `remove <path>...` | Stop tracking files |
| `checkpoint [-m msg]` | Re-hash tracked files and update the manifest |
| `restore [path...] [-f]` | Restore files to their original locations |
| `status` | Show which tracked files have changed |
| `diff <path>` | Show content diff between stored and current file |
| `list` | List all tracked files |
| `verify` | Check blob store integrity against manifest hashes |
| `prune` | Remove orphaned blobs not referenced by the manifest |
| `mirror up <path>` | Sync filesystem → manifest (add new, remove deleted) |
| `mirror down <path> [-f]` | Sync manifest → filesystem (restore + delete untracked) |
| `version` | Print the version |

### Remote sync

| Command | Description |
|---|---|
| `push` | Push checkpoint to remote gRPC server |
| `pull` | Pull checkpoint from remote gRPC server |
| `prune` | With `--remote`, prunes orphaned blobs on the server |

Remote commands require `--remote host:port` (or `SGARD_REMOTE` env, or a
`<repo>/remote` config file) and authenticate via SSH keys.

The server daemon `sgardd` is a separate binary (included in releases and
Nix builds).

## Remote sync

Start the daemon on your server:

```sh
sgard init --repo /srv/sgard
sgardd --authorized-keys ~/.ssh/authorized_keys
```

Push and pull from client machines:

```sh
sgard push --remote myserver:9473
sgard pull --remote myserver:9473
```

Authentication uses your existing SSH keys (ssh-agent, `~/.ssh/id_ed25519`,
or `--ssh-key`). No passwords or certificates to manage.

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
portable across machines with different usernames. Adding a directory
recursively tracks all files and symlinks inside.

See [ARCHITECTURE.md](ARCHITECTURE.md) for full design details.