# Implementation Plan: Multi-Agent Backend Support

**Branch**: `002-multi-agent-backend` | **Date**: 2026-03-20 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `/specs/002-multi-agent-backend/spec.md`

## Summary

Make `mem` agent-agnostic by replacing the hardcoded Claude Code
CLI invocation with a backend registry. Each backend (Claude Code,
OpenCode, Codex, or custom) is defined by its binary name, argument
template, and default model. Users select a backend via
`MEM_BACKEND` env var or `--backend` flag. When unconfigured,
auto-detection probes for installed binaries in priority order.
Zero external dependencies — stdlib only. BDD mandatory.

## Technical Context

**Language/Version**: Go 1.26.0 (existing project)
**Primary Dependencies**: Go stdlib only. No new dependencies.
**Storage**: N/A (this feature modifies the agent invocation layer,
not storage)
**Testing**: `go test -race -shuffle=on ./...` — BDD table-driven
tests. Tests FIRST.
**Target Platform**: Linux/macOS
**Project Type**: CLI tool (existing)
**Performance Goals**: Auto-detection <500ms. Backend selection
adds <10ms to startup.
**Constraints**: Zero external deps. Backward compatible —
existing Claude-only users see no change in behavior.
**Scale/Scope**: Modify `internal/agent/` package, `internal/config/`,
`cmd/mem/main.go`. ~200 lines changed/added.

## Constitution Check

| Principle | Applicable? | Status | Notes |
|-----------|-------------|--------|-------|
| I. Code Quality — error wrapping | Yes | PASS | All backend errors wrapped with context |
| I. Code Quality — single responsibility | Yes | PASS | Backend registry is a single package |
| II. Testing — BDD coverage | Yes | PASS | All backend logic has BDD tests |
| Arch — Go 1.26.0 | Yes | PASS | No version change |
| Arch — Zero external deps | Yes | PASS | Stdlib only |
| Dev Workflow — Testing gates | Yes | PASS | `go test -race -shuffle=on ./...` |

**Post-Phase 1 re-check**: All gates pass.

## Project Structure

### Documentation (this feature)

```text
specs/002-multi-agent-backend/
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
└── tasks.md
```

### Source Code (changes only)

```text
internal/
├── agent/
│   ├── agent.go           # MODIFY: Replace CLIInvoker with Backend + Registry
│   ├── agent_test.go      # MODIFY: BDD tests for backend resolution
│   ├── backend.go          # NEW: Backend struct, built-in registry, auto-detect
│   └── backend_test.go     # NEW: BDD tests for backends
└── config/
    ├── config.go           # MODIFY: Add MEM_BACKEND, MEM_BACKEND_BINARY, MEM_BACKEND_ARGS
    └── config_test.go      # MODIFY: BDD tests for new config fields

cmd/mem/
└── main.go                 # MODIFY: Pass --backend flag to runners, resolve backend
```

**Structure Decision**: This is a focused refactor of the existing
`internal/agent/` package. No new packages needed — the backend
registry lives in the agent package alongside the Invoker interface.

## Testing Strategy: BDD

Same approach as feature 001. BDD mandatory, table-driven tests,
Given/When/Then naming, StubInvoker for runner tests.

## Complexity Tracking

No new deviations required. All existing deviations from feature
001 (no DDD layers, exported fields, go vet instead of golangci-lint)
carry forward unchanged.
