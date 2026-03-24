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
go build ./cmd/sgard
```

Run tests:
```bash
go test ./...
```

## Dependencies

- `gopkg.in/yaml.v3` — manifest serialization
- `github.com/spf13/cobra` — CLI framework

## Package Structure

```
cmd/sgard/    CLI entry point (cobra commands, pure wiring)
garden/       Core business logic (Garden struct orchestrating everything)
manifest/     YAML manifest parsing (Manifest/Entry structs, Load/Save)
store/        Content-addressable blob storage (SHA-256 keyed)
```

Key rule: all logic lives in `garden/`. The `cmd/` layer only parses flags
and calls `Garden` methods. This enables the future gRPC server to reuse
the same logic with zero duplication.

## Legacy Files

Old C++ and proto source files may still be present. They are retained in
git history for reference and should be removed as part of the Go rewrite
(see PROJECT_PLAN.md Step 1).
