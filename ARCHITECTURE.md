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

### Local

| Command | Description |
|---|---|
| `sgard init [--repo <path>]` | Create a new repository |
| `sgard add <path>...` | Track files, directories (recursed), or symlinks |
| `sgard remove <path>...` | Untrack files; run `prune` to clean orphaned blobs |
| `sgard checkpoint [-m <message>]` | Re-hash all tracked files, store changed blobs, update manifest |
| `sgard restore [<path>...] [--force]` | Restore files from manifest to their original locations |
| `sgard status` | Compare current files against manifest: modified, missing, ok |
| `sgard verify` | Check all blobs against manifest hashes (integrity check) |
| `sgard list` | List all tracked files |
| `sgard diff <path>` | Show content diff between current file and stored blob |
| `sgard prune` | Remove orphaned blobs not referenced by the manifest |
| `sgard mirror up <path>...` | Sync filesystem → manifest (add new, remove deleted, rehash) |
| `sgard mirror down <path>... [--force]` | Sync manifest → filesystem (restore + delete untracked) |

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

### Remote

| Command | Description |
|---|---|
| `sgard push` | Push checkpoint to remote gRPC server |
| `sgard pull` | Pull checkpoint from remote gRPC server |
| `sgard prune` | With `--remote`, prunes orphaned blobs on the server |
| `sgardd` | Run the gRPC sync daemon |

## gRPC Protocol

The GardenSync service uses four RPCs for sync plus one for maintenance:

```
service GardenSync {
  rpc PushManifest(PushManifestRequest) returns (PushManifestResponse);
  rpc PushBlobs(stream PushBlobsRequest) returns (PushBlobsResponse);
  rpc PullManifest(PullManifestRequest) returns (PullManifestResponse);
  rpc PullBlobs(PullBlobsRequest) returns (stream PullBlobsResponse);
  rpc Prune(PruneRequest) returns (PruneResponse);
}
```

**Push flow:** Client sends manifest → server compares `manifest.Updated`
timestamps → if client newer, server returns list of missing blob hashes →
client streams those blobs (64 KiB chunks) → server replaces its manifest.

**Pull flow:** Client requests server manifest → compares timestamps locally →
if server newer, requests missing blobs → server streams them → client
replaces its manifest.

**Last timestamp wins** for conflict resolution (single-user, personal sync).

## Authentication

Authentication is designed to be transparent — the user never explicitly
logs in or manages credentials. It uses SSH keys they already have.

### Overview

Two mechanisms, layered:

1. **SSH key signing** — used to obtain a token or when no valid token exists
2. **JWT token** — used for all subsequent requests, cached on disk

From the user's perspective, authentication is automatic. The client
handles token acquisition, caching, and renewal without prompting.

### Token-Based Auth (Primary Path)

The server issues signed JWT tokens valid for 30 days. The client caches
the token and attaches it as gRPC metadata on every call.

```
service GardenSync {
  rpc Authenticate(AuthenticateRequest) returns (AuthenticateResponse);
  // ... other RPCs
}
```

**Authenticate RPC:**
- Client sends an SSH-signed challenge (nonce + timestamp + public key)
- Server verifies the signature against its `authorized_keys` file
- Server returns a JWT signed with its own secret key
- JWT claims: public key fingerprint, issued-at, 30-day expiry

**Normal request flow:**
1. Client reads cached token from `$XDG_STATE_HOME/sgard/token`
   (falls back to `~/.local/state/sgard/token`)
2. Client attaches token as `x-sgard-auth-token` gRPC metadata
3. Server verifies JWT signature and expiry
4. If valid → request proceeds

**Token rejection — two cases:**

The server distinguishes between an expired-but-previously-valid token
and a completely invalid one:

- **Expired token** (valid signature, known fingerprint still in
  authorized_keys, but past expiry): server returns `Unauthenticated`
  with a `ReauthChallenge` — a server-generated nonce embedded in the
  error details. This is the fast path.

- **Invalid token** (bad signature, unknown fingerprint, corrupted):
  server returns a plain `Unauthenticated` with no challenge. The client
  falls back to the full Authenticate flow.

**Fast re-auth flow (expired token, transparent to user):**
1. Client sends request with expired token
2. Server returns `Unauthenticated` + `ReauthChallenge{nonce, timestamp}`
3. Client signs the server-provided nonce+timestamp with SSH key
4. Client calls `Authenticate` with the signature
5. Server verifies, issues new JWT
6. Client caches new token to disk
7. Client retries the original request with the new token

This saves a round trip compared to full re-auth — the server provides
the nonce, so the client doesn't need to generate one and hope it's
accepted. The server controls the challenge, which also prevents any
client-side nonce reuse.

**Full auth flow (no valid token, transparent to user):**
1. Client has no cached token or token is completely invalid
2. Client calls `Authenticate` with a self-generated nonce+timestamp,
   signed with SSH key
3. Server verifies, issues JWT
4. Client caches token, proceeds with original request

### SSH Key Signing

Used during the `Authenticate` RPC to prove possession of an authorized
SSH private key. The challenge can come from the server (re-auth fast
path) or be generated by the client (initial auth).

**Challenge payload:** `nonce (32 random bytes) || timestamp (big-endian int64)`

**Authenticate RPC request fields:**
- `nonce` — 32-byte nonce (from server's ReauthChallenge or client-generated)
- `timestamp` — Unix seconds
- `signature` — SSH signature over (nonce || timestamp)
- `public_key` — SSH public key in authorized_keys format

**Server verification:**
- Parse public key, check fingerprint against `authorized_keys` file
- Verify SSH signature over the payload
- Check timestamp is within 5-minute window (prevents replay)

### Server-Side Token Management

The server does not store tokens. JWTs are stateless — the server signs
them with a secret key and verifies its own signature on each request.

**Secret key:** Generated on first startup, stored at `<repo>/jwt.key`
(32 random bytes). If the key file is deleted, all outstanding tokens
become invalid and clients re-authenticate automatically.

**No revocation mechanism.** For a single-user personal sync tool,
revocation is unnecessary. Removing a key from `authorized_keys`
prevents new token issuance. Existing tokens expire naturally within
30 days. Deleting `jwt.key` invalidates all tokens immediately.

### Client-Side Token Storage

Token cached at `$XDG_STATE_HOME/sgard/token` (per XDG Base Directory
spec, state is "data that should persist between restarts but isn't
important enough to back up"). Falls back to `~/.local/state/sgard/token`.

The token file contains the raw JWT string. File permissions are set to
`0600`.

### Key Resolution

SSH key resolution order (for initial authentication):
1. `--ssh-key` flag (explicit path to private key)
2. `SGARD_SSH_KEY` environment variable
3. ssh-agent (if `SSH_AUTH_SOCK` is set, uses first available key)
4. Default paths: `~/.ssh/id_ed25519`, `~/.ssh/id_rsa`

## Go Package Structure

```
sgard/
  cmd/sgard/              # CLI entry point — one file per command
    main.go               # cobra root command, --repo/--remote/--ssh-key flags
    push.go pull.go prune.go mirror.go
    init.go add.go remove.go checkpoint.go
    restore.go status.go verify.go list.go diff.go version.go

  cmd/sgardd/             # gRPC server daemon
    main.go               # --listen, --repo, --authorized-keys flags

  garden/                 # Core business logic — one file per operation
    garden.go             # Garden struct, Init, Open, Add, Checkpoint, Status, accessors
    restore.go mirror.go prune.go remove.go verify.go list.go diff.go
    hasher.go             # SHA-256 file hashing

  manifest/               # YAML manifest parsing
    manifest.go           # Manifest and Entry structs, Load/Save

  store/                  # Content-addressable blob storage
    store.go              # Store struct: Write/Read/Exists/Delete/List

  server/                 # gRPC server implementation
    server.go             # GardenSync RPC handlers with RWMutex
    auth.go               # SSH key auth interceptor
    convert.go            # proto ↔ manifest type conversion

  client/                 # gRPC client library
    client.go             # Push, Pull, Prune methods
    auth.go               # SSHCredentials (PerRPCCredentials), LoadSigner

  sgardpb/                # Generated protobuf + gRPC Go code
  proto/sgard/v1/         # Proto source definitions

  flake.nix               # Nix flake (builds sgard + sgardd)
  .goreleaser.yaml        # GoReleaser (builds both binaries)
```

### Key Architectural Rule

**The `garden` package contains all logic. The `cmd` package is pure CLI
wiring. The `server` package wraps `Garden` methods as gRPC endpoints.**

```go
type Garden struct {
    manifest     *manifest.Manifest
    store        *store.Store
    root         string
    manifestPath string
    clock        clockwork.Clock
}

// Local operations
func (g *Garden) Add(paths []string) error
func (g *Garden) Remove(paths []string) error
func (g *Garden) Checkpoint(message string) error
func (g *Garden) Restore(paths []string, force bool, confirm func(string) bool) error
func (g *Garden) Status() ([]FileStatus, error)
func (g *Garden) Verify() ([]VerifyResult, error)
func (g *Garden) List() []manifest.Entry
func (g *Garden) Diff(path string) (string, error)
func (g *Garden) Prune() (int, error)
func (g *Garden) MirrorUp(paths []string) error
func (g *Garden) MirrorDown(paths []string, force bool, confirm func(string) bool) error

// Accessors (used by server package)
func (g *Garden) GetManifest() *manifest.Manifest
func (g *Garden) BlobExists(hash string) bool
func (g *Garden) ReadBlob(hash string) ([]byte, error)
func (g *Garden) WriteBlob(data []byte) (string, error)
func (g *Garden) ReplaceManifest(m *manifest.Manifest) error
func (g *Garden) ListBlobs() ([]string, error)
func (g *Garden) DeleteBlob(hash string) error
```

The gRPC server calls the same `Garden` methods as the CLI — no logic
duplication.

## Design Decisions

**Paths in manifest use `~` unexpanded.** The `garden` package expands `~` to
`$HOME` at runtime. This makes the manifest portable across machines with
different usernames.

**Adding a directory recurses.** `Add` walks directories and adds each
file/symlink individually. Directories are not tracked as entries — only
leaf files and symlinks.

**No history.** Only the latest checkpoint is stored. For versioning, place
the repo under git — `sgard init` creates a `.gitignore` that excludes
`blobs/`.

**Per-file timestamps.** Each manifest entry records an `updated` timestamp
set at checkpoint time. On restore, if the manifest entry is newer than the
file on disk (by mtime), the restore proceeds without prompting. If the file
on disk is newer or the times match, sgard prompts for confirmation.
`--force` always skips the prompt.

**Atomic writes.** Manifest saves write to a temp file then rename.

**Timestamp comparison truncates to seconds** for cross-platform filesystem
compatibility.

**Remote config resolution:** `--remote` flag > `SGARD_REMOTE` env >
`<repo>/remote` file.

**SSH key resolution:** `--ssh-key` flag > `SGARD_SSH_KEY` env > ssh-agent >
`~/.ssh/id_ed25519` > `~/.ssh/id_rsa`.
