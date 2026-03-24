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
  - path: ~/.bashrc                 # plaintext file
    hash: a1b2c3d4e5f6...          # SHA-256 of file contents
    type: file
    mode: "0644"
    updated: "2026-03-23T14:30:00Z"

  - path: ~/.vimrc
    type: link
    target: ~/.config/nvim/init.vim
    updated: "2026-03-23T14:30:00Z"

  - path: ~/.ssh/config             # encrypted file
    hash: f8e9d0c1...              # SHA-256 of encrypted blob
    plaintext_hash: e5f6a7...      # SHA-256 of plaintext
    encrypted: true
    type: file
    mode: "0600"
    updated: "2026-03-23T14:30:00Z"

# Encryption config — only present if sgard encrypt init has been run.
# Travels with the manifest so a new machine can decrypt after pull.
# KEK slots are a map keyed by user-chosen label.
encryption:
  algorithm: xchacha20-poly1305
  kek_slots:
    passphrase:
      type: passphrase
      argon2_time: 3
      argon2_memory: 65536
      argon2_threads: 4
      salt: "base64..."
      wrapped_dek: "base64..."
    fido2/workstation:
      type: fido2
      credential_id: "base64..."
      salt: "base64..."
      wrapped_dek: "base64..."
    fido2/laptop:
      type: fido2
      credential_id: "base64..."
      salt: "base64..."
      wrapped_dek: "base64..."
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

## Encryption

sgard supports optional at-rest encryption for individual files.
Encryption is per-file, not per-repo — any file can be marked as
encrypted at add time. A repo may contain a mix of encrypted and
plaintext blobs.

### Key Hierarchy

A two-layer key hierarchy separates the encryption key from the user's
secret (passphrase or FIDO2 key):

```
User Secret (passphrase or FIDO2 hmac-secret)
    │
    ▼
KEK (Key Encryption Key) — derived from user secret
    │
    ▼
DEK (Data Encryption Key) — random, encrypts/decrypts file blobs
```

**DEK (Data Encryption Key):**
- 256-bit random key, generated once when encryption is first enabled
- Used with XChaCha20-Poly1305 (AEAD) to encrypt file blobs
- Never stored in plaintext — always wrapped by the KEK
- Each KEK source stores its own wrapped copy in the manifest
  (`encryption.kek_sources[].wrapped_dek`, base64-encoded)

**KEK (Key Encryption Key):**
- Derived from the user's secret
- Used only to wrap/unwrap the DEK, never to encrypt data directly
- Never stored on disk — derived on demand

This separation means changing a passphrase or adding a FIDO2 key only
requires re-wrapping the DEK, not re-encrypting every blob.

### KEK Derivation

Two slot types. A repo has one `passphrase` slot and zero or more
`fido2/<label>` slots:

**Passphrase slot** (at most one per repo):
- KEK = Argon2id(passphrase, salt, time=3, memory=64MB, threads=4)
- Salt and Argon2id parameters stored in the slot entry
- Slot key: `passphrase`

**FIDO2 slots** (one per device, labeled):
- KEK = HMAC-SHA256 output from the FIDO2 authenticator
- The authenticator computes `HMAC(device_secret, salt)` using the
  credential registered for this slot
- `credential_id` in the slot entry ties it to a specific FIDO2
  registration, allowing sgard to skip non-matching devices
- Slot key: `fido2/<label>` (defaults to hostname, overridable)

### Blob Encryption

**Algorithm:** XChaCha20-Poly1305 (from `golang.org/x/crypto/chacha20poly1305`)
- 24-byte nonce (random per blob), 16-byte auth tag
- AEAD — provides both confidentiality and integrity
- XChaCha20 variant chosen for its 24-byte nonce, which is safe to
  generate randomly without collision risk

**Encrypted blob format:**
```
[24-byte nonce][ciphertext + 16-byte Poly1305 tag]
```

**Encryption flow (during Add/Checkpoint):**
1. Read file plaintext
2. Generate random 24-byte nonce
3. Encrypt: `ciphertext = XChaCha20-Poly1305.Seal(nonce, DEK, plaintext)`
4. Compute SHA-256 hash of the encrypted blob (nonce + ciphertext)
5. Store the encrypted blob in the content-addressable store

**Decryption flow (during Restore/Diff):**
1. Read encrypted blob from store
2. Extract 24-byte nonce prefix
3. Decrypt: `plaintext = XChaCha20-Poly1305.Open(nonce, DEK, ciphertext)`
4. Write plaintext to disk

### Hashing: Post-Encryption

The manifest hash is the SHA-256 of the **ciphertext**, not the plaintext.

Rationale:
- `verify` checks blob integrity without needing the DEK
- The hash matches what's actually stored on disk
- The server never needs the DEK — it handles only encrypted blobs
- `status` needs the DEK to compare against the current file (hash
  the plaintext, encrypt it, compare encrypted hash — or keep a
  plaintext hash in the manifest)

**Manifest changes for encryption:**

Encrypted entries gain two fields: `encrypted: true` and
`plaintext_hash` (SHA-256 of the plaintext, for efficient `status`
checks without decryption):

```yaml
files:
  - path: ~/.bashrc
    hash: a1b2c3d4...        # SHA-256 of plaintext — not encrypted
    type: file
    mode: "0644"
    updated: "2026-03-24T..."

  - path: ~/.ssh/config
    hash: f8e9d0c1...        # SHA-256 of encrypted blob (post-encryption)
    plaintext_hash: e5f6a7... # SHA-256 of plaintext (pre-encryption)
    encrypted: true
    type: file
    mode: "0600"
    updated: "2026-03-24T..."
```

For unencrypted entries, `hash` is the SHA-256 of the plaintext (current
behavior), and `plaintext_hash` and `encrypted` are omitted.

`status` hashes the current file on disk and compares against
`plaintext_hash` (for encrypted entries) or `hash` (for plaintext).
`verify` always uses `hash` to check store integrity without the DEK.

### DEK Storage

Each slot wraps the DEK independently using XChaCha20-Poly1305,
stored as base64 in the slot's `wrapped_dek` field:

```
wrapped_dek = base64([24-byte nonce][encrypted DEK + 16-byte tag])
```

The manifest is fully self-contained — pulling it to a new machine
gives you everything needed to decrypt (given the user's secret).

### Unlock Resolution

When sgard needs the DEK, it reads `encryption.kek_slots` from the
manifest and tries slots automatically:

1. **FIDO2 slots** (all `fido2/*` slots, in map order):
   - For each: check if a connected FIDO2 device matches the
     slot's `credential_id`
   - If match found → prompt for touch, derive KEK, unwrap DEK
   - If no device matches or touch times out → try next slot

2. **Passphrase slot** (if `passphrase` slot exists):
   - Prompt for passphrase on stdin
   - Derive KEK via Argon2id, unwrap DEK

3. **No slots succeed** → error

FIDO2 is tried first because it requires no typing — just a touch.
The `credential_id` check avoids prompting for touch on a device that
can't unwrap the slot, which matters when multiple FIDO2 keys are
connected. The passphrase slot is the universal fallback.

The user never specifies which slot to use. The presence of the
`encryption` section indicates the repo has encryption capability.
Individual files opt in via `--encrypt` at add time.

### CLI Integration

**Setting up encryption (creates DEK, adds `encryption` to manifest):**
```sh
sgard encrypt init                          # passphrase slot only
sgard encrypt init --fido2                  # fido2/<hostname> + passphrase slots
```

When `--fido2` is specified, sgard creates both slots: the FIDO2 slot
(named `fido2/<hostname>` by default) and immediately prompts for a
passphrase to create the fallback slot. This ensures the user is never
locked out if they lose the FIDO2 key.

Without `--fido2`, only the `passphrase` slot is created.

**Adding encrypted files:**
```sh
sgard add --encrypt ~/.ssh/config ~/.aws/credentials
sgard add ~/.bashrc               # not encrypted
```

**Managing slots:**
```sh
sgard encrypt add-fido2                     # adds fido2/<hostname>
sgard encrypt add-fido2 --label yubikey-5   # adds fido2/yubikey-5
sgard encrypt remove-slot fido2/old-laptop  # removes a slot
sgard encrypt list-slots                    # shows all slot names and types
sgard encrypt change-passphrase             # prompts for old and new
```

Adding a slot auto-unlocks the DEK via an existing slot first (e.g.,
`add-fido2` will prompt for the passphrase to unwrap the DEK, then
re-wrap it with the new FIDO2 key).

**Unlocking:**
Operations that touch encrypted entries (add --encrypt, checkpoint,
restore, diff, mirror on encrypted files) trigger automatic unlock
via the resolution order above. The DEK is cached in memory for the
duration of the command.

Operations that only touch plaintext entries never prompt — they work
exactly as before, even if the repo has encryption configured.

There is no long-lived unlock state — each command invocation that needs
the DEK obtains it fresh. This is intentional: dotfile operations are
infrequent, and caching the DEK across invocations would require a
daemon or on-disk secret, both of which expand the attack surface.

### Security Properties

- **Selective confidentiality:** Only files marked `--encrypt` are
  encrypted. The manifest contains paths and hashes but not file
  contents for encrypted entries.
- **Server ignorance:** The server never has the DEK. Push/pull
  transfers encrypted blobs opaquely. The server cannot read encrypted
  file contents.
- **Key rotation:** Changing the passphrase re-wraps the DEK without
  re-encrypting blobs.
- **Compromise recovery:** If the DEK is compromised, all encrypted
  blobs must be re-encrypted (not just re-wrapped). This is an
  explicit `sgard encrypt rotate-dek` operation.
- **No plaintext leaks:** `diff` decrypts in memory, never writes
  decrypted blobs to disk.
- **Graceful degradation:** Commands that don't touch encrypted entries
  work without the DEK. A `status` on a mixed repo can check plaintext
  entries without prompting.

### Repos Without Encryption

A manifest with no `encryption` section has no DEK and cannot have
encrypted entries. The `--encrypt` flag on `add` will error, prompting
the user to run `sgard encrypt init` first. All existing behavior is
unchanged.

### Encryption and Remote Sync

The server never has the DEK. Push/pull transfers the manifest
(including the `encryption` section with wrapped DEKs and salts) and
encrypted blobs as opaque bytes. The server cannot decrypt file
contents.

When pulling to a new machine:
1. The manifest arrives with all `kek_slots` intact
2. The user provides their passphrase (universal fallback)
3. sgard derives the KEK, unwraps the DEK, decrypts blobs on restore

No additional setup is needed beyond having the passphrase.

**Adding FIDO2 on a new machine:** FIDO2 hmac-secret is device-bound —
a different physical key produces a different KEK. After pulling to a
new machine, the user runs `sgard encrypt add-fido2` which:
1. Unlocks the DEK via the passphrase slot
2. Registers a new FIDO2 credential on the local device
3. Wraps the DEK with the new FIDO2 KEK
4. Adds a `fido2/<hostname>` slot to the manifest

On next push, the new slot propagates to the server and other machines.
Each machine accumulates its own FIDO2 slot over time.

### TLS Transport

sgardd supports optional TLS via `--tls-cert` and `--tls-key` flags.
When provided, the server uses `credentials.NewTLS()` with a minimum
of TLS 1.2. Without them, it runs insecure (for local/trusted networks).

The client gains `--tls` and `--tls-ca` flags:
- `--tls` — enables TLS transport (uses system CA pool by default)
- `--tls-ca <path>` — custom CA certificate for self-signed server certs

Both flags must be specified together on the server side; on the client
side `--tls` alone uses the system trust store, and `--tls-ca` adds a
custom root.

### FIDO2 Hardware Support

Real FIDO2 hardware support uses `go-libfido2` (CGo bindings to
Yubico's libfido2 C library). It is gated behind the `fido2` build
tag to avoid requiring CGo and libfido2 for users who don't need it:

- `go build ./...` — default build, no FIDO2 hardware support
- `go build -tags fido2 ./...` — links against libfido2 for real keys

The implementation (`garden/fido2_hardware.go`) wraps
`libfido2.Device.MakeCredential` and `Assertion` with the
`HMACSecretExtension` to derive 32-byte HMAC secrets from hardware
keys. A `--fido2-pin` flag is available for PIN-protected devices.

The Nix flake provides two packages: `sgard` (default, no CGo) and
`sgard-fido2` (links libfido2).

### DEK Rotation

`sgard encrypt rotate-dek` generates a new DEK, re-encrypts all
encrypted blobs with the new key, and re-wraps the new DEK with all
existing KEK slots. Required when the DEK is suspected compromised
(re-wrapping alone is insufficient since the old DEK could decrypt
the existing blobs).

The rotation process:
1. Generate a new random 256-bit DEK
2. For each encrypted entry: decrypt with old DEK, re-encrypt with new DEK,
   write new blob to store, update manifest hash (plaintext hash unchanged)
3. Re-derive each KEK (passphrase via Argon2id, FIDO2 via device) and
   re-wrap the new DEK. FIDO2 slots without a matching connected device
   are dropped during rotation.
4. Save updated manifest

Plaintext entries are untouched.

### Planned: Multi-Repo + Per-Machine Inclusion (Phase 5)

Support for multiple repos on a single server, and per-machine
inclusion rules (e.g., "this file only applies to Linux machines" or
"this directory is only for the workstation"). Design TBD.

### Future: Manifest Signing (Phase 6)

Manifest signing (to detect tampering) is deferred. The challenge is
the trust model: which key signs, and how does a pulling client verify
the signature when multiple machines with different SSH keys push to
the same server? This requires a proper trust/key-authority design.

## Go Package Structure

```
sgard/
  cmd/sgard/              # CLI entry point — one file per command
    main.go               # cobra root command, --repo/--remote/--ssh-key/--tls/--tls-ca flags
    encrypt.go            # sgard encrypt init/add-fido2/remove-slot/list-slots/change-passphrase
    push.go pull.go prune.go mirror.go
    init.go add.go remove.go checkpoint.go
    restore.go status.go verify.go list.go diff.go version.go

  cmd/sgardd/             # gRPC server daemon
    main.go               # --listen, --repo, --authorized-keys, --tls-cert, --tls-key flags

  garden/                 # Core business logic — one file per operation
    garden.go             # Garden struct, Init, Open, Add, Checkpoint, Status, accessors
    encrypt.go            # EncryptInit, UnlockDEK, RotateDEK, encrypt/decrypt blobs, slot mgmt
    encrypt_fido2.go      # FIDO2Device interface, AddFIDO2Slot, unlock resolution
    fido2_hardware.go     # Real FIDO2 via go-libfido2 (//go:build fido2)
    fido2_nohardware.go   # Stub returning nil (//go:build !fido2)
    restore.go mirror.go prune.go remove.go verify.go list.go diff.go
    hasher.go             # SHA-256 file hashing

  manifest/               # YAML manifest parsing
    manifest.go           # Manifest and Entry structs, Load/Save

  store/                  # Content-addressable blob storage
    store.go              # Store struct: Write/Read/Exists/Delete/List

  server/                 # gRPC server implementation
    server.go             # GardenSync RPC handlers with RWMutex
    auth.go               # JWT token + SSH key auth interceptor, Authenticate RPC
    convert.go            # proto ↔ manifest type conversion (incl. encryption)

  client/                 # gRPC client library
    client.go             # Push, Pull, Prune with auto-auth retry
    auth.go               # TokenCredentials, LoadSigner, Authenticate, token caching

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
    dek          []byte  // unlocked data encryption key
}

// Local operations
func (g *Garden) Add(paths []string, opts ...AddOptions) error
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
func (g *Garden) Lock(paths []string) error
func (g *Garden) Unlock(paths []string) error

// Encryption
func (g *Garden) EncryptInit(passphrase string) error
func (g *Garden) UnlockDEK(prompt func() (string, error), fido2 ...FIDO2Device) error
func (g *Garden) HasEncryption() bool
func (g *Garden) NeedsDEK(entries []manifest.Entry) bool
func (g *Garden) RotateDEK(prompt func() (string, error), fido2 ...FIDO2Device) error
func (g *Garden) AddFIDO2Slot(device FIDO2Device, label string) error
func (g *Garden) RemoveSlot(name string) error
func (g *Garden) ListSlots() map[string]string
func (g *Garden) ChangePassphrase(newPassphrase string) error

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

**Locked files (`--lock`).** A locked entry is repo-authoritative — the
on-disk copy is treated as potentially corrupted by the system, not as
a user edit. Semantics:
- **`add --lock`** — tracks the file normally, marks it as locked
- **`checkpoint`** — skips locked files entirely (preserves the repo version)
- **`status`** — reports locked files with changed hashes as `drifted`
  (distinct from `modified`, which implies a user edit)
- **`restore`** — always restores locked files if the hash differs,
  regardless of timestamp, without prompting. Skips if hash matches.
- **`add`** (without `--lock`) — can be used to explicitly update a locked
  file in the repo when the on-disk version is intentionally new

Use case: system-managed files like `~/.config/user-dirs.dirs` that get
overwritten by the OS but should be kept at a known-good state.

**Directory-only entries (`--dir`).** `add --dir <path>` tracks the
directory itself as a structural entry without recursing into its
contents. On restore, sgard ensures the directory exists with the
correct permissions. Use case: directories that must exist for other
software to function, but whose contents are managed elsewhere.

**Remote config resolution:** `--remote` flag > `SGARD_REMOTE` env >
`<repo>/remote` file.

**SSH key resolution:** `--ssh-key` flag > `SGARD_SSH_KEY` env > ssh-agent >
`~/.ssh/id_ed25519` > `~/.ssh/id_rsa`.
