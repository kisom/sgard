# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Critical: Keep Project Docs Updated

Any change to the codebase MUST be reflected in these files:

- **ARCHITECTURE.md** — design decisions, data model, package structure
- **PROJECT_PLAN.md** — implementation steps; check off completed items
- **PROGRESS.md** — current status, change log; update after completing any step

If another agent or engineer picks this up later, these files are how they
resume. Keeping them accurate is not optional.

## Project

sgard (Shimmering Clarity Gardener) — a dotfiles manager.
Module: `github.com/kisom/sgard`. Author: K. Isom <kyle@imap.cc>.

## Build

```bash
go build ./...                   # both sgard and sgardd
go build -tags fido2 ./...       # with real FIDO2 hardware support (requires libfido2)
```

Nix:
```bash
nix build .#sgard                # builds both binaries (no CGo)
nix build .#sgard-fido2          # with FIDO2 hardware support (links libfido2)
```

Run tests:
```bash
go test ./...
```

Lint:
```bash
golangci-lint run ./...
```

Regenerate proto (requires protoc toolchain):
```bash
make proto
```

## Dependencies

- `gopkg.in/yaml.v3` — manifest serialization
- `github.com/spf13/cobra` — CLI framework
- `github.com/jonboulle/clockwork` — injectable clock for deterministic tests
- `google.golang.org/grpc` — gRPC runtime
- `google.golang.org/protobuf` — protobuf runtime
- `golang.org/x/crypto` — SSH key auth (ssh, ssh/agent), Argon2id, XChaCha20-Poly1305
- `github.com/golang-jwt/jwt/v5` — JWT token auth
- `github.com/keys-pub/go-libfido2` — FIDO2 hardware key support (build tag `fido2`, requires libfido2)

## Package Structure

```
cmd/sgard/    CLI entry point (cobra commands, pure wiring)
cmd/sgardd/   gRPC server daemon
garden/       Core business logic (Garden struct, encryption, FIDO2 hardware via build tags)
manifest/     YAML manifest parsing (Manifest/Entry structs, Load/Save)
store/        Content-addressable blob storage (SHA-256 keyed)
server/       gRPC server (RPC handlers, JWT/SSH auth interceptor, proto conversion)
client/       gRPC client library (Push, Pull, Prune, token auth with auto-renewal)
sgardpb/      Generated protobuf + gRPC Go code
```

Key rule: all logic lives in `garden/`. The `cmd/` layer only parses flags
and calls `Garden` methods. The `server` wraps `Garden` as gRPC endpoints.
No logic duplication.

Each garden operation lives in its own file (`garden/<op>.go`) to minimize
merge conflicts during parallel development.
