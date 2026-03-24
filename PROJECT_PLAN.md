# PROJECT_PLAN.md

Implementation plan for sgard. See ARCHITECTURE.md for design details.

## Step 1: Project Scaffolding

Remove old C++ source files and set up the Go project.

- [x] Remove old files: `sgard.cc`, `proto/`, `CMakeLists.txt`, `scripts/`, `.clang-format`, `.clang-tidy`, `.idea/`, `.trunk/`
- [x] `go mod init github.com/kisom/sgard`
- [x] Add dependencies: `gopkg.in/yaml.v3`, `github.com/spf13/cobra`
- [x] Create directory structure: `cmd/sgard/`, `manifest/`, `store/`, `garden/`
- [x] Set up `cmd/sgard/main.go` with cobra root command and `--repo` persistent flag
- [x] Update CLAUDE.md to reflect Go project
- [x] Verify: `go build ./...` compiles clean

## Step 2: Manifest Package

*Can be done in parallel with Step 3.*

- [x] `manifest/manifest.go`: `Manifest` and `Entry` structs with YAML tags
  - Entry types: `file`, `directory`, `link`
  - Mode as string type to avoid YAML octal coercion
  - Per-file `updated` timestamp
- [x] `manifest/manifest.go`: `Load(path)` and `Save(path)` functions
  - Save uses atomic write (write to `.tmp`, rename)
- [x] `manifest/manifest_test.go`: round-trip marshal/unmarshal, atomic save, entry type validation

## Step 3: Store Package

*Can be done in parallel with Step 2.*

- [x] `store/store.go`: `Store` struct with `root` path
- [x] `store/store.go`: `Write(data) (hash, error)` — hash content, write to `blobs/XX/YY/<hash>`
- [x] `store/store.go`: `Read(hash) ([]byte, error)` — read blob by hash
- [x] `store/store.go`: `Exists(hash) bool` — check if blob exists
- [x] `store/store.go`: `Delete(hash) error` — remove a blob
- [x] `store/store_test.go`: write/read round-trip, integrity check, missing blob error

## Step 4: Garden Core — Init and Add

Depends on Steps 2 and 3.

- [x] `garden/hasher.go`: `HashFile(path) (string, error)` — SHA-256 of a file
- [x] `garden/garden.go`: `Garden` struct tying manifest + store + root path
- [x] `garden/garden.go`: `Open(root) (*Garden, error)` — load existing repo
- [x] `garden/garden.go`: `Init(root) (*Garden, error)` — create new repo (dirs + empty manifest)
- [x] `garden/garden.go`: `Add(paths []string) error` — hash files, store blobs, add manifest entries
- [x] `garden/garden_test.go`: init creates correct structure, add stores blob and updates manifest
- [x] Wire up CLI: `cmd/sgard/init.go`, `cmd/sgard/add.go`
- [x] Verify: `go build ./cmd/sgard && ./sgard init && ./sgard add ~/.bashrc`

## Step 5: Checkpoint and Status

Depends on Step 4.

- [x] `garden/garden.go`: `Checkpoint(message string) error` — re-hash all tracked files, store changed blobs, update manifest timestamps
- [x] `garden/garden.go`: `Status() ([]FileStatus, error)` — compare current hashes to manifest; report modified/missing/ok
- [x] `garden/garden_test.go`: checkpoint detects changed files, status reports correctly
- [x] Wire up CLI: `cmd/sgard/checkpoint.go`, `cmd/sgard/status.go`

## Step 6: Restore

Depends on Step 5.

- [x] `garden/garden.go`: `Restore(paths []string, force bool, confirm func) error`
  - Restore all files if paths is empty, otherwise just the specified paths
  - Timestamp comparison: skip prompt if manifest `updated` is newer than file mtime
  - Prompt user if file on disk is newer or times match (unless `--force`)
  - Create parent directories as needed
  - Recreate symlinks for `link` type entries
  - Set file permissions from manifest `mode`
- [x] `garden/garden_test.go`: restore writes correct content, respects permissions, handles symlinks
- [x] Wire up CLI: `cmd/sgard/restore.go`

## Step 7: Remaining Commands

*These can be done in parallel with each other.*

- [x] `garden/remove.go`: `Remove(paths []string) error` — remove manifest entries
- [x] `garden/verify.go`: `Verify() ([]VerifyResult, error)` — check blobs against manifest hashes
- [x] `garden/list.go`: `List() []Entry` — return all manifest entries
- [x] `garden/diff.go`: `Diff(path string) (string, error)` — diff stored blob vs current file
- [x] Wire up CLI: `cmd/sgard/remove.go`, `cmd/sgard/verify.go`, `cmd/sgard/list.go`, `cmd/sgard/diff.go`
- [x] Tests for each

## Step 8: Polish

- [x] Lint setup (golangci-lint config)
- [x] Clock abstraction: inject `jonboulle/clockwork` into Garden for deterministic timestamp tests
- [x] End-to-end test: init → add → checkpoint → modify file → status → restore → verify
- [x] Ensure `go vet ./...` and `go test ./...` pass clean
- [x] Update CLAUDE.md, ARCHITECTURE.md, PROGRESS.md

## Phase 2: gRPC Remote Sync

### Step 9: Proto Definitions + Code Gen

- [x] Write `proto/sgard/v1/sgard.proto` — 5 RPCs (PushManifest, PushBlobs, PullManifest, PullBlobs, Prune), all messages
- [x] Add Makefile target for protoc code generation
- [x] Add grpc, protobuf, x/crypto deps to go.mod
- [x] Update flake.nix devShell with protoc tools
- [x] Verify: `go build ./sgardpb` compiles

### Step 10: Garden Accessor Methods

*Can be done in parallel with Step 11.*

- [x] `garden/garden.go`: `GetManifest()`, `BlobExists()`, `ReadBlob()`, `WriteBlob()`, `ReplaceManifest()`
- [x] Tests for each accessor
- [x] Verify: `go test ./garden/...`

### Step 11: Proto-Manifest Conversion

*Can be done in parallel with Step 10.*

- [x] `server/convert.go`: `ManifestToProto`, `ProtoToManifest`, entry helpers
- [x] `server/convert_test.go`: round-trip test
- [x] Verify: `go test ./server/...`

### Step 12: Server Implementation (No Auth)

Depends on Steps 9, 10, 11.

- [x] `server/server.go`: Server struct with RWMutex, 5 RPC handlers (+ Prune)
- [x] PushManifest: timestamp compare, compute missing blobs
- [x] PushBlobs: receive stream, write to store, replace manifest
- [x] PullManifest: return manifest
- [x] PullBlobs: stream requested blobs (64 KiB chunks)
- [x] Prune: remove orphaned blobs (added store.List + garden.ListBlobs/DeleteBlob)
- [x] `server/server_test.go`: in-process test with bufconn, push+pull+prune

### Step 12b: Directory Recursion and Mirror Command

- [x] `garden/garden.go`: `Add` recurses directories — walk all files/symlinks, add each as its own entry
- [x] `garden/mirror.go`: `MirrorUp(paths []string) error` — walk directory, add new files, remove entries for files gone from disk, re-hash changed
- [x] `garden/mirror.go`: `MirrorDown(paths []string, force bool, confirm func(string) bool) error` — restore all tracked files under path, delete anything not in manifest
- [x] `garden/mirror_test.go`: tests for recursive add, mirror up (detects new/removed), mirror down (cleans extras)
- [x] `cmd/sgard/mirror.go`: `sgard mirror up <path>`, `sgard mirror down <path> [--force]`
- [x] Update existing add tests for directory recursion

### Step 13: Client Library (No Auth)

Depends on Step 12.

- [x] `client/client.go`: Client struct, `Push()`, `Pull()`, `Prune()` methods
- [x] `client/client_test.go`: integration tests (push+pull cycle, server newer, up-to-date, prune)

### Step 14: SSH Key Auth

- [x] `server/auth.go`: AuthInterceptor, parse authorized_keys, verify SSH signatures
- [x] `client/auth.go`: LoadSigner (ssh-agent or key file), SSHCredentials (PerRPCCredentials)
- [x] `server/auth_test.go`: valid key, reject unauthenticated, reject unauthorized key, reject expired timestamp
- [x] `client/auth_test.go`: metadata generation, no-transport-security
- [x] Integration tests: authenticated push/pull succeeds, unauthenticated is rejected

### Step 15: CLI Wiring + Prune

Depends on Steps 13, 14.

- [x] `garden/prune.go`: `Prune() (int, error)` — collect referenced hashes, delete orphaned blobs
- [x] `garden/prune_test.go`: prune removes orphaned, keeps referenced
- [x] `server/server.go`: Prune RPC (done in Step 12)
- [x] `proto/sgard/v1/sgard.proto`: Prune RPC (done in Step 9)
- [x] `client/client.go`: Prune() method (done in Step 13)
- [x] `cmd/sgard/prune.go`: local prune; with `--remote` prunes remote instead
- [x] `cmd/sgard/main.go`: add `--remote`, `--ssh-key` persistent flags, resolveRemote()
- [x] `cmd/sgard/push.go`, `cmd/sgard/pull.go`
- [x] `cmd/sgardd/main.go`: flags, garden open, auth interceptor, gRPC serve
- [x] Verify: both binaries compile

### Step 16: Polish + Release

- [x] Update ARCHITECTURE.md, README.md, CLAUDE.md, PROGRESS.md
- [x] Update flake.nix (add sgardd, updated vendorHash)
- [x] Update .goreleaser.yaml (add sgardd build)
- [x] E2e integration test: init two repos, push from one, pull into other (with auth)
- [x] Verify: all tests pass, full push/pull cycle works

## Phase 3: Encryption

### Step 17: Encryption Core (Passphrase Only)

- [x] `manifest/manifest.go`: add `Encrypted`, `PlaintextHash` fields to Entry; add `Encryption` section with `KekSlots` map to Manifest
- [x] `garden/encrypt.go`: `EncryptInit(passphrase string) error` — generate DEK, derive KEK via Argon2id, wrap DEK, store in manifest encryption section
- [x] `garden/encrypt.go`: `UnlockDEK(prompt) error` — read slots, try passphrase, unwrap DEK; cache in memory for command duration
- [x] `garden/encrypt.go`: encrypt/decrypt helpers using XChaCha20-Poly1305 (nonce + seal/open)
- [x] `garden/garden.go`: modify Add to accept encrypt flag — encrypt blob before storing, set `encrypted: true` and `plaintext_hash` on entry
- [x] `garden/garden.go`: modify Checkpoint to re-encrypt changed encrypted entries (compares plaintext_hash)
- [x] `garden/garden.go`: modify Restore to decrypt encrypted blobs before writing
- [x] `garden/diff.go`: modify Diff to decrypt stored blob before diffing
- [x] `garden/garden.go`: modify Status to use `plaintext_hash` for encrypted entries
- [x] Tests: 10 encryption tests (init, persist, unlock, add-encrypted, restore round-trip, checkpoint, status, diff, requires-DEK)
- [x] Verify: `go test ./... && go vet ./... && golangci-lint run ./...`

### Step 18: FIDO2 Support

Depends on Step 17.

- [x] `garden/encrypt_fido2.go`: FIDO2Device interface, AddFIDO2Slot, unlockFIDO2, defaultFIDO2Label
- [x] `garden/encrypt.go`: UnlockDEK tries fido2/* slots first (credential_id matching), falls back to passphrase
- [x] `garden/encrypt_fido2_test.go`: mock FIDO2 device, 6 tests (add slot, duplicate rejected, unlock via FIDO2, fallback to passphrase, persistence, encrypted round-trip with FIDO2)
- [x] Verify: `go test ./... && go vet ./... && golangci-lint run ./...`

### Step 19: Encryption CLI + Slot Management

Depends on Steps 17, 18.

- [x] `cmd/sgard/encrypt.go`: `sgard encrypt init [--fido2]`, `add-fido2 [--label]`, `remove-slot`, `list-slots`, `change-passphrase`
- [x] `garden/encrypt.go`: `RemoveSlot`, `ListSlots`, `ChangePassphrase` methods
- [x] `cmd/sgard/add.go`: add `--encrypt` flag with passphrase prompt
- [x] Update proto: add `encrypted`, `plaintext_hash` to ManifestEntry; add KekSlot, Encryption messages, encryption field on Manifest
- [x] Update `server/convert.go`: full encryption section conversion (Encryption, KekSlot)
- [x] Verify: both binaries compile, `go test ./...`, lint clean

### Step 20: Encryption Polish + Release

- [x] E2e test: full encryption lifecycle (init, add encrypted+plaintext, checkpoint, modify, status, restore, verify, diff, slot management, passphrase change)
- [x] Update ARCHITECTURE.md, README.md, CLAUDE.md
- [x] Update flake.nix vendorHash
- [x] Verify: all tests pass, lint clean

## Future Steps (Not Phase 3)

## Phase 4: Hardening + Completeness

### Step 21: Lock/Unlock Toggle Commands

- [x] `garden/lock.go`: `Lock(paths)`, `Unlock(paths)` — toggle locked flag on existing entries
- [x] `cmd/sgard/lock.go`: `sgard lock <path>...`, `sgard unlock <path>...`
- [x] Tests: lock/unlock existing entry, persist, error on untracked, checkpoint/status behavior changes (6 tests)

### Step 22: Shell Completion

- [ ] `cmd/sgard/completion.go`: cobra's built-in completion for bash, zsh, fish
- [ ] Update README with completion installation instructions

### Step 23: TLS Transport for sgardd

- [ ] `cmd/sgardd/main.go`: add `--tls-cert`, `--tls-key` flags
- [ ] Server uses `credentials.NewTLS()` when cert/key provided, insecure otherwise
- [ ] Client: add `--tls` flag and `--tls-ca` for custom CA
- [ ] Update `cmd/sgard/main.go` and `dialRemote()` for TLS
- [ ] Tests: TLS connection with self-signed cert
- [ ] Update ARCHITECTURE.md and README.md

### Step 24: DEK Rotation

- [ ] `garden/encrypt.go`: `RotateDEK(promptPassphrase func() (string, error)) error` — generate new DEK, re-encrypt all encrypted blobs, re-wrap with all existing KEK slots
- [ ] `cmd/sgard/encrypt.go`: `sgard encrypt rotate-dek`
- [ ] Tests: rotate DEK, verify all encrypted entries still decrypt correctly

### Step 25: Real FIDO2 Hardware Binding

- [ ] Evaluate approach: libfido2 CGo bindings vs subprocess (`fido2-token`/`fido2-cred`)
- [ ] Implement real `FIDO2Device` satisfying the existing interface
- [ ] `cmd/sgard/encrypt.go`: wire real device into `add-fido2` and unlock resolution
- [ ] Build tag or runtime detection for FIDO2 availability
- [ ] Tests: skip on CI without hardware, manual test instructions

### Step 26: Test Cleanup

- [ ] Standardize all test calls to use `AddOptions{}` struct (remove any legacy variadic patterns)
- [ ] Ensure all tests use `t.TempDir()` consistently
- [ ] Review lint config, tighten if possible
- [ ] Verify test coverage for lock/unlock, encrypted locked files, dir-only locked entries

### Step 27: Phase 4 Polish + Release

- [ ] Update all docs (ARCHITECTURE.md, README.md, CLAUDE.md, PROGRESS.md)
- [ ] Update flake.nix vendorHash if deps changed
- [ ] Update .goreleaser.yaml if needed
- [ ] E2e test covering TLS + encryption + locked files
- [ ] Verify: all tests pass, lint clean, both binaries compile

## Phase 5: Multi-Repo + Per-Machine Inclusion

(To be planned)

## Phase 6: Manifest Signing

(To be planned — requires trust model design)
