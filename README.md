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

### Shell completion

```sh
# Bash (add to ~/.bashrc)
source <(sgard completion bash)

# Zsh (add to ~/.zshrc)
source <(sgard completion zsh)

# Fish
sgard completion fish | source
# To load on startup:
sgard completion fish > ~/.config/fish/completions/sgard.fish
```

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

### Locked files

Some files get overwritten by the system (desktop environments,
package managers, etc.) but you want to keep them at a known-good
state. Locked files are repo-authoritative — `restore` always
overwrites them, and `checkpoint` never picks up the system's changes:

```sh
# XDG user-dirs.dirs gets reset by the desktop environment on login
sgard add --lock ~/.config/user-dirs.dirs

# The system overwrites it — status reports "drifted", not "modified"
sgard status
# drifted    ~/.config/user-dirs.dirs

# Restore puts it back without prompting
sgard restore
```

Use `add` (without `--lock`) when you intentionally want to update the
repo with a new version of a locked file.

### Directory-only entries

Sometimes a directory must exist for software to work, but its
contents are managed elsewhere. `--dir` tracks the directory itself
without recursing:

```sh
# Ensure ~/.local/share/applications exists (some apps break without it)
sgard add --dir ~/.local/share/applications
```

On `restore`, sgard creates the directory with the correct permissions
but doesn't touch its contents.

## Commands

### Local

| Command | Description |
|---|---|
| `init` | Create a new repository |
| `add <path>...` | Track files, directories (recursed), or symlinks |
| `add --lock <path>...` | Track as locked (repo-authoritative, auto-restores on drift) |
| `add --dir <path>` | Track directory itself without recursing into contents |
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

### Encryption

| Command | Description |
|---|---|
| `encrypt init` | Set up encryption (creates DEK + passphrase slot) |
| `encrypt add-fido2 [--label]` | Add a FIDO2 KEK slot |
| `encrypt remove-slot <name>` | Remove a KEK slot |
| `encrypt list-slots` | List all KEK slots |
| `encrypt change-passphrase` | Change the passphrase |
| `add --encrypt <path>...` | Track files with encryption |

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

### TLS

To encrypt the connection with TLS:

```sh
# Server: provide cert and key
sgardd --tls-cert server.crt --tls-key server.key --authorized-keys ~/.ssh/authorized_keys

# Client: enable TLS (uses system CA pool)
sgard push --remote myserver:9473 --tls

# Client: with a custom/self-signed CA
sgard push --remote myserver:9473 --tls --tls-ca ca.crt
```

Without `--tls-cert`/`--tls-key`, sgardd runs without TLS (suitable for
localhost or trusted networks).

## Encryption

Sensitive files can be encrypted individually:

```sh
# Set up encryption (once per repo)
sgard encrypt init

# Add encrypted files
sgard add --encrypt ~/.ssh/config ~/.aws/credentials

# Plaintext files work as before
sgard add ~/.bashrc
```

Encrypted blobs use XChaCha20-Poly1305. The data encryption key (DEK)
is wrapped by a passphrase-derived key (Argon2id). FIDO2 hardware keys
are also supported as an alternative KEK source — sgard tries FIDO2
first and falls back to passphrase automatically.

The encryption config (wrapped DEKs, salts) lives in the manifest, so
it syncs with push/pull. The server never has the DEK.

See [ARCHITECTURE.md](ARCHITECTURE.md) for the full encryption design.

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