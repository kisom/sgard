# PROGRESS.md

Tracks implementation status. See PROJECT_PLAN.md for the full plan and
ARCHITECTURE.md for design details.

**If you are picking this up mid-implementation, read this file first.**

## Current Status

**Phase:** Phase 5 complete. File exclusion feature added.

**Last updated:** 2026-03-27

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

Phase 6: Manifest Signing (to be planned).

## Standalone Additions

- **Deployment to rift**: sgardd deployed as Podman container on rift behind
  mc-proxy (L4 SNI passthrough on :9443, multiplexed with metacrypt gRPC).
  TLS cert issued by Metacrypt, SSH-key auth. DNS at
  `sgard.svc.mcp.metacircular.net`.
- **Default remote config**: `sgard remote set/show` commands. Saves addr,
  TLS, and CA path to `<repo>/remote.yaml`. `dialRemote` merges saved config
  with CLI flags (flags win). Removes need for `--remote`/`--tls` on every
  push/pull.

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
| 2026-03-24 | 19 | Encryption CLI: encrypt init/add-fido2/remove-slot/list-slots/change-passphrase, --encrypt on add, proto + convert updates. |
| 2026-03-24 | 20 | Polish: encryption e2e test, all docs updated, flake vendorHash updated. |
| 2026-03-24 | — | Locked files + dir-only entries. v2.0.0 released. |
| 2026-03-24 | — | Phase 4 planned (Steps 21–27): lock/unlock, shell completion, TLS, DEK rotation, real FIDO2, test cleanup. |
| 2026-03-24 | 21 | Lock/unlock toggle commands. garden/lock.go, cmd/sgard/lock.go, 6 tests. |
| 2026-03-24 | 22 | Shell completion: cobra built-in, README docs for bash/zsh/fish. |
| 2026-03-24 | 23 | TLS transport: sgardd --tls-cert/--tls-key, sgard --tls/--tls-ca, 2 integration tests. |
| 2026-03-24 | 24 | DEK rotation: RotateDEK re-encrypts all blobs, re-wraps all slots, CLI command, 4 tests. |
| 2026-03-24 | 25 | Real FIDO2: go-libfido2 bindings, build tag gating, CLI wiring, nix sgard-fido2 package. |
| 2026-03-24 | 26 | Test cleanup: tightened lint, 3 combo tests (encrypted+locked, dir-only+locked, toggle), stale doc fixes. |
| 2026-03-24 | 27 | Phase 4 polish: e2e test (TLS+encryption+locked+push/pull), final doc review. Phase 4 complete. |
| 2026-03-24 | — | Phase 5 planned (Steps 28–32): machine identity, targeting, tags, proto update, polish. |
| 2026-03-24 | 28 | Machine identity + targeting core: Entry Only/Never, Identity(), EntryApplies(), tags file. 13 tests. |
| 2026-03-24 | 29 | Operations respect targeting: checkpoint/restore/status skip non-matching. 6 tests. |
| 2026-03-24 | 30 | Targeting CLI: tag add/remove/list, identity, --only/--never on add, target command. |
| 2026-03-24 | 31 | Proto + sync: only/never fields on ManifestEntry, conversion, round-trip test. |
| 2026-03-24 | 32 | Phase 5 polish: e2e test (targeting + push/pull + restore), docs updated. Phase 5 complete. |
| 2026-03-25 | — | `sgard info` command: shows detailed file information (status, hash, timestamps, mode, encryption, targeting). 5 tests. |
| 2026-03-25 | — | Deploy sgardd to rift: Dockerfile, docker-compose, mc-proxy L4 route on :9443, Metacrypt TLS cert, DNS. |
| 2026-03-25 | — | `sgard remote set/show`: persistent remote config in `<repo>/remote.yaml` (addr, tls, tls_ca). |
| 2026-03-26 | — | `sgard list` remote support: uses `resolveRemoteConfig()` to list server manifest via `PullManifest` RPC. Client `List()` method added. |
| 2026-03-26 | — | Version derived from git tags via `VERSION` file. flake.nix reads `VERSION`; Makefile `version` target syncs from latest tag, `build` injects via ldflags. |
| 2026-03-27 | — | File exclusion: `sgard exclude`/`include` commands, `Manifest.Exclude` field, Add/MirrorUp/MirrorDown respect exclusions, directory exclusion support. 8 tests. |
