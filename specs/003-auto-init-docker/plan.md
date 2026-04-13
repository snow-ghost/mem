# Implementation Plan: Auto-Init & Docker Support

**Branch**: `003-auto-init-docker` | **Date**: 2026-03-21 | **Spec**: [spec.md](spec.md)

## Summary

Two independent features: (1) auto-initialize `.memory/` when any
command is run and the store doesn't exist yet, and (2) a minimal
Dockerfile for running `mem` in containers. BDD mandatory, stdlib
only, no new dependencies.

## Technical Context

**Language/Version**: Go 1.26.0 (existing project)
**Primary Dependencies**: Go stdlib only. No new deps.
**Testing**: `go test -race -shuffle=on ./...` — BDD, tests FIRST.
**Target Platform**: Linux/macOS + Docker (linux/amd64, linux/arm64)
**Constraints**: Zero external deps. Docker image < 10 MB.

## Changes Required

### Auto-Init (~30 lines)

Add `EnsureInit()` method to `MemoryStore` that creates the store
if it doesn't exist, without erroring if it does. Unlike `Init()`,
it is idempotent and only creates missing files/dirs.

Call `EnsureInit()` at the start of every command except `init`
(which keeps its current behavior). Print notice to stderr.

### Dockerfile (~15 lines)

Multi-stage build: Go builder stage → scratch/alpine final stage
with just the binary. Entrypoint `mem`, workdir `/project`.

### GitHub Actions (~30 lines)

Workflow file for building and pushing Docker image on release tags.

## Project Structure (changes only)

```text
internal/store/
├── store.go           # MODIFY: add EnsureInit()
└── store_test.go      # MODIFY: BDD tests for EnsureInit

cmd/mem/
└── main.go            # MODIFY: call EnsureInit() in each command

Dockerfile             # NEW: multi-stage build
.github/workflows/
└── docker.yml         # NEW: build + push on release
```

## Constitution Check

All applicable principles pass. No new deviations.

## Complexity Tracking

No new deviations.
