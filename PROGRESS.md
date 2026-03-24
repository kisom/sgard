# PROGRESS.md

Tracks implementation status. See PROJECT_PLAN.md for the full plan and
ARCHITECTURE.md for design details.

**If you are picking this up mid-implementation, read this file first.**

## Current Status

**Phase:** Pre-implementation — design complete, ready to begin Step 1.

**Last updated:** 2026-03-23

## Completed Steps

(none yet)

## In Progress

(none yet)

## Up Next

Step 1: Project Scaffolding — remove old C++ files, initialize Go module,
create directory structure, set up cobra root command.

## Known Issues / Decisions Deferred

- **Blob durability**: blobs are not stored in git. A strategy for backup or
  replication is deferred to a future phase.
- **gRPC remote mode**: Phase 2. Package structure is designed to accommodate
  it (garden core separates logic from CLI wiring).

## Change Log

| Date | Step | Summary |
|---|---|---|
| 2026-03-23 | — | Design phase complete. ARCHITECTURE.md and PROJECT_PLAN.md written. |
