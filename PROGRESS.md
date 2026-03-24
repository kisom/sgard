# PROGRESS.md

Tracks implementation status. See PROJECT_PLAN.md for the full plan and
ARCHITECTURE.md for design details.

**If you are picking this up mid-implementation, read this file first.**

## Current Status

**Phase:** Steps 2 & 3 complete. Ready for Step 4 (Garden Core).

**Last updated:** 2026-03-23

## Completed Steps

- **Step 1: Project Scaffolding** — removed old C++ files and `.trunk/` config,
  initialized Go module, added cobra + yaml.v3 deps, created package dirs,
  set up cobra root command with `--repo` flag.
- **Step 2: Manifest Package** — `Manifest` and `Entry` structs with YAML tags,
  `New()`, `Load(path)`, and `Save(path)` with atomic write. 5 tests.
- **Step 3: Store Package** — content-addressable blob store with SHA-256 keying.
  `New()`, `Write()`, `Read()`, `Exists()`, `Delete()` with atomic writes,
  hash validation, and two-level directory layout. 11 tests.

## In Progress

(none)

## Up Next

Step 4: Garden Core — Init and Add. Ties manifest + store together.

## Known Issues / Decisions Deferred

- **Blob durability**: blobs are not stored in git. A strategy for backup or
  replication is deferred to a future phase.
- **gRPC remote mode**: Phase 2. Package structure is designed to accommodate
  it (garden core separates logic from CLI wiring).

## Change Log

| Date | Step | Summary |
|---|---|---|
| 2026-03-23 | — | Design phase complete. ARCHITECTURE.md and PROJECT_PLAN.md written. |
| 2026-03-23 | 1 | Scaffolding complete. Old C++ removed, Go module initialized, cobra root command. |
| 2026-03-23 | 2 | Manifest package complete. Structs, Load/Save with atomic write, full test suite. |
| 2026-03-23 | 3 | Store package complete. Content-addressable blob store, 11 tests. |
