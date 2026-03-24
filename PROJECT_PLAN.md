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

- [ ] `manifest/manifest.go`: `Manifest` and `Entry` structs with YAML tags
  - Entry types: `file`, `directory`, `link`
  - Mode as string type to avoid YAML octal coercion
  - Per-file `updated` timestamp
- [ ] `manifest/manifest.go`: `Load(path)` and `Save(path)` functions
  - Save uses atomic write (write to `.tmp`, rename)
- [ ] `manifest/manifest_test.go`: round-trip marshal/unmarshal, atomic save, entry type validation

## Step 3: Store Package

*Can be done in parallel with Step 2.*

- [ ] `store/store.go`: `Store` struct with `root` path
- [ ] `store/store.go`: `Write(data) (hash, error)` — hash content, write to `blobs/XX/YY/<hash>`
- [ ] `store/store.go`: `Read(hash) ([]byte, error)` — read blob by hash
- [ ] `store/store.go`: `Exists(hash) bool` — check if blob exists
- [ ] `store/store.go`: `Delete(hash) error` — remove a blob
- [ ] `store/store_test.go`: write/read round-trip, integrity check, missing blob error

## Step 4: Garden Core — Init and Add

Depends on Steps 2 and 3.

- [ ] `garden/hasher.go`: `HashFile(path) (string, error)` — SHA-256 of a file
- [ ] `garden/garden.go`: `Garden` struct tying manifest + store + root path
- [ ] `garden/garden.go`: `Open(root) (*Garden, error)` — load existing repo
- [ ] `garden/garden.go`: `Init(root) (*Garden, error)` — create new repo (dirs + empty manifest)
- [ ] `garden/garden.go`: `Add(paths []string) error` — hash files, store blobs, add manifest entries
- [ ] `garden/garden_test.go`: init creates correct structure, add stores blob and updates manifest
- [ ] Wire up CLI: `cmd/sgard/init.go`, `cmd/sgard/add.go`
- [ ] Verify: `go build ./cmd/sgard && ./sgard init && ./sgard add ~/.bashrc`

## Step 5: Checkpoint and Status

Depends on Step 4.

- [ ] `garden/garden.go`: `Checkpoint(message string) error` — re-hash all tracked files, store changed blobs, update manifest timestamps
- [ ] `garden/garden.go`: `Status() ([]FileStatus, error)` — compare current hashes to manifest; report modified/missing/ok
- [ ] `garden/garden_test.go`: checkpoint detects changed files, status reports correctly
- [ ] Wire up CLI: `cmd/sgard/checkpoint.go`, `cmd/sgard/status.go`

## Step 6: Restore

Depends on Step 5.

- [ ] `garden/garden.go`: `Restore(paths []string, force bool) error`
  - Restore all files if paths is empty, otherwise just the specified paths
  - Timestamp comparison: skip prompt if manifest `updated` is newer than file mtime
  - Prompt user if file on disk is newer or times match (unless `--force`)
  - Create parent directories as needed
  - Recreate symlinks for `link` type entries
  - Set file permissions from manifest `mode`
- [ ] `garden/garden_test.go`: restore writes correct content, respects permissions, handles symlinks
- [ ] Wire up CLI: `cmd/sgard/restore.go`

## Step 7: Remaining Commands

*These can be done in parallel with each other.*

- [ ] `garden/garden.go`: `Remove(paths []string) error` — remove manifest entries
- [ ] `garden/garden.go`: `Verify() ([]VerifyResult, error)` — check blobs against manifest hashes
- [ ] `garden/garden.go`: `List() []Entry` — return all manifest entries
- [ ] `garden/diff.go`: `Diff(path string) (string, error)` — diff stored blob vs current file
- [ ] Wire up CLI: `cmd/sgard/remove.go`, `cmd/sgard/verify.go`, `cmd/sgard/list.go`, `cmd/sgard/diff.go`
- [ ] Tests for each

## Step 8: Polish

- [ ] Lint setup (golangci-lint config)
- [ ] End-to-end test: init → add → checkpoint → modify file → status → restore → verify
- [ ] Ensure `go vet ./...` and `go test ./...` pass clean
- [ ] Update CLAUDE.md, ARCHITECTURE.md, PROGRESS.md

## Future Steps (Not Phase 1)

- Blob durability (backup/replication strategy)
- gRPC remote mode (push/pull/serve)
- Proto definitions for wire format
- Shell completion via cobra
