# PROGRESS.md

Tracks implementation status. See PROJECT_PLAN.md for the full plan and
ARCHITECTURE.md for design details.

**If you are picking this up mid-implementation, read this file first.**

## Current Status

**Phase:** Phase 3 in progress. Steps 17–18 complete, ready for Step 19.

**Last updated:** 2026-03-24

## Completed Steps

- **Step 1: Project Scaffolding** — removed old C++ files and `.trunk/` config,
  initialized Go module, added cobra + yaml.v3 deps, created package dirs,
  set up cobra root command with `--repo` flag.
- **Step 2: Manifest Package** — `Manifest` and `Entry` structs with YAML tags,
  `New()`, `Load(path)`, and `Save(path)` with atomic write. 5 tests.
- **Step 3: Store Package** — content-addressable blob store with SHA-256 keying.
  `New()`, `Write()`, `Read()`, `Exists()`, `Delete()` with atomic writes,
  hash validation, and two-level directory layout. 11 tests.
- **Step 4: Garden Core — Init and Add** — `Garden` struct tying manifest +
  store, `Init()`, `Open()`, `Add()` handling files/dirs/symlinks, `HashFile()`,
  tilde path conversion, CLI `init` and `add` commands. 8 tests.
- **Step 5: Checkpoint and Status** — `Checkpoint()` re-hashes all tracked files,
  stores changed blobs, updates timestamps. `Status()` reports ok/modified/missing
  per entry. CLI `checkpoint` (with `-m` flag) and `status` commands. 4 tests.
- **Step 6: Restore** — `Restore()` with selective paths, force mode, confirm
  callback, timestamp-based auto-restore, parent dir creation, symlink support,
  file permission restoration. CLI `restore` with `--force` flag. 6 tests.
- **Step 7: Remaining Commands** — Remove (2 tests), Verify (3 tests), List
  (2 tests), Diff (3 tests). Each in its own file to enable parallel
  development. All CLI commands wired up.
- **Step 8: Polish** — golangci-lint config, all lint issues fixed, clockwork
  clock abstraction injected into Garden, e2e lifecycle test, docs updated.

## In Progress

(none)

## Up Next

Step 19: Encryption CLI + Slot Management.

## Known Issues / Decisions Deferred

- **Manifest signing**: deferred — trust model (which key signs, how do
  pulling clients verify) needs design.
- **DEK rotation**: `sgard encrypt rotate-dek` (re-encrypt all blobs)
  deferred to future work.
- **FIDO2 testing**: hardware-dependent, may need mocks or CI skip.

## Change Log

| Date | Step | Summary |
|---|---|---|
| 2026-03-23 | — | Design phase complete. ARCHITECTURE.md and PROJECT_PLAN.md written. |
| 2026-03-23 | 1 | Scaffolding complete. Old C++ removed, Go module initialized, cobra root command. |
| 2026-03-23 | 2 | Manifest package complete. Structs, Load/Save with atomic write, full test suite. |
| 2026-03-23 | 3 | Store package complete. Content-addressable blob store, 11 tests. |
| 2026-03-23 | 4 | Garden core complete. Init, Open, Add with file/dir/symlink support, CLI commands. 8 tests. |
| 2026-03-23 | 5 | Checkpoint and Status complete. Re-hash, store changed blobs, status reporting. 4 tests. |
| 2026-03-23 | 6 | Restore complete. Selective paths, force/confirm, timestamp logic, symlinks, permissions. 6 tests. |
| 2026-03-23 | 7 | Remaining commands complete. Remove, Verify, List, Diff — 10 tests across 4 parallel units. |
| 2026-03-23 | 8 | Polish complete. golangci-lint, clockwork, e2e test, doc updates. |
| 2026-03-23 | — | README, goreleaser config, version command, Nix flake, homebrew formula, release pipeline validated (v0.1.0–v0.1.2). |
| 2026-03-23 | — | v1.0.0 released. Docs updated for release. |
| 2026-03-23 | 9 | Proto definitions: 5 RPCs (Push/Pull manifest+blobs, Prune), generated sgardpb, Makefile, deps added. |
| 2026-03-23 | 10 | Garden accessor methods: GetManifest, BlobExists, ReadBlob, WriteBlob, ReplaceManifest. 5 tests. |
| 2026-03-23 | 11 | Proto-manifest conversion: ManifestToProto/ProtoToManifest with round-trip tests. |
| 2026-03-23 | 12 | gRPC server: 5 RPC handlers (push/pull manifest+blobs, prune), bufconn tests, store.List. |
| 2026-03-23 | 12b | Directory recursion in Add, mirror up/down commands, 7 tests. |
| 2026-03-23 | 13 | Client library: Push, Pull, Prune with chunked blob streaming. 6 integration tests. |
| 2026-03-23 | 14 | SSH key auth: server interceptor (authorized_keys, signature verification), client PerRPCCredentials (ssh-agent/key file). 8 tests including auth integration. |
| 2026-03-24 | 15 | CLI wiring: push, pull, prune commands, sgardd daemon binary, --remote/--ssh-key flags, local prune with 2 tests. |
| 2026-03-24 | 16 | Polish: updated all docs, flake.nix (sgardd + vendorHash), goreleaser (both binaries), e2e push/pull test with auth. |
| 2026-03-24 | — | JWT token auth implemented (transparent auto-renewal, XDG token cache, ReauthChallenge fast path). |
| 2026-03-24 | — | Phase 3 encryption design: selective per-file encryption, KEK slots (passphrase + fido2/label), manifest-embedded config. |
| 2026-03-24 | 17 | Encryption core: Argon2id KEK, XChaCha20 DEK wrap/unwrap, selective per-file encrypt in Add/Checkpoint/Restore/Diff/Status. 10 tests. |
| 2026-03-24 | 18 | FIDO2: FIDO2Device interface, AddFIDO2Slot, unlock resolution (fido2 first → passphrase fallback), mock device, 6 tests. |
