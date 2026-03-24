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

- [ ] `client/client.go`: Client struct, `Push()`, `Pull()` methods
- [ ] `client/client_test.go`: integration test against in-process server

### Step 14: SSH Key Auth

- [ ] `server/auth.go`: AuthInterceptor, parse authorized_keys, verify SSH signatures
- [ ] `client/auth.go`: LoadSigner (ssh-agent or key file), PerRPCCredentials
- [ ] `server/auth_test.go`: in-memory ed25519 key pair, reject unauthenticated
- [ ] `client/auth_test.go`: metadata generation test

### Step 15: CLI Wiring + Prune

Depends on Steps 13, 14.

- [ ] `garden/prune.go`: `Prune() (int, error)` — collect referenced hashes from manifest, delete orphaned blobs, return count removed
- [ ] `garden/prune_test.go`: add file, remove it, prune removes orphaned blob
- [ ] `server/server.go`: add `Prune` RPC — server-side prune, returns count
- [ ] `proto/sgard/v1/sgard.proto`: add `rpc Prune(PruneRequest) returns (PruneResponse)`
- [ ] `client/client.go`: add `Prune()` method
- [ ] `cmd/sgard/prune.go`: local prune; with `--remote` flag prunes remote instead
- [ ] `cmd/sgard/main.go`: add `--remote`, `--ssh-key` persistent flags
- [ ] `cmd/sgard/push.go`, `cmd/sgard/pull.go`
- [ ] `cmd/sgardd/main.go`: flags, garden open, auth interceptor, gRPC serve
- [ ] Verify: both binaries compile

### Step 16: Polish + Release

- [ ] Update ARCHITECTURE.md, README.md, CLAUDE.md, PROGRESS.md
- [ ] Update flake.nix (add sgardd, protoc to devShell)
- [ ] Update .goreleaser.yaml (add sgardd build)
- [ ] E2e integration test: init two repos, push from one, pull into other
- [ ] Verify: all tests pass, full push/pull cycle works

## Future Steps (Not Phase 2)

- Shell completion via cobra
- TLS transport (optional --tls-cert/--tls-key on sgardd)
- Multiple repo support on server
