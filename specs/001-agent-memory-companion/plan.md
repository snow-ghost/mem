# Implementation Plan: Agent Memory Companion

**Branch**: `001-agent-memory-companion` | **Date**: 2026-03-20 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `/specs/001-agent-memory-companion/spec.md`

## Summary

Build a Go CLI tool (`mem`) that provides persistent memory for AI
coding agents. The tool manages a file-based memory store
(`.memory/` directory) with three memory types: episodic (JSONL event
log), semantic (Markdown principles), and procedural (Markdown skill
recipes). Three operational modes — extraction (post-session event
capture), consolidation (periodic cleanup and knowledge synthesis),
and injection (pre-session context assembly) — are orchestrated by
invoking Claude CLI as a subagent for LLM-powered analysis. The
application uses minimal external dependencies, relying on Go stdlib
for file I/O, JSON handling, and process execution.

**Testing approach: BDD (mandatory)**. All features MUST be developed
using Behavior-Driven Development. Tests are written FIRST as
Given/When/Then scenarios derived from spec.md acceptance criteria,
verified to FAIL, then implementation proceeds until tests pass.
Tests use Go stdlib `testing` package with `_test.go` files colocated
with implementation. BDD scenarios are expressed as Go table-driven
tests with descriptive names matching the spec's Given/When/Then
format.

## Technical Context

**Language/Version**: Go 1.26.0
**Primary Dependencies**: Go stdlib only (`encoding/json`, `os`,
`os/exec`, `bufio`, `flag`, `time`, `path/filepath`, `syscall`,
`log/slog`). No external frameworks.
**Storage**: File-based — JSONL for episodes, Markdown for
principles/skills/consolidation log. No database.
**Testing**: `go test -race -shuffle=on ./...` — BDD-style
table-driven tests with Given/When/Then naming. Tests FIRST.
**Target Platform**: Linux/macOS (CLI tool, runs where Claude Code
runs)
**Project Type**: CLI tool
**Performance Goals**: File operations complete in <100ms for stores
up to 200 episodes. Subagent invocation latency is bounded by LLM
response time (not controlled by this tool).
**Constraints**: Zero external Go module dependencies beyond stdlib.
File locking via `syscall.Flock` (Unix). Memory files must remain
human-readable and git-diffable.
**Scale/Scope**: Single project memory store, up to 200 episodes,
100 principles, ~15 skills. Multi-agent writes via file locking.

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1
design.*

| Principle | Applicable? | Status | Notes |
|-----------|-------------|--------|-------|
| I. Code Quality — single responsibility, error wrapping | Yes | PASS | Applied: flat packages by domain concept, `fmt.Errorf("op: %w", err)` wrapping |
| I. Code Quality — unexported entity fields | Partially | PASS with deviation | Episode struct uses exported fields for JSON serialization. Justified in Complexity Tracking. |
| I. Code Quality — 4-layer DDD | No | N/A (deviation) | CLI tool, not web service. Justified in Complexity Tracking. |
| I. Code Quality — golangci-lint | Partially | PASS with deviation | `go vet ./...` as lint gate. Full golangci-lint deferred — no `.golangci.yml` in this standalone project. Justified in Complexity Tracking. |
| II. Testing Standards — test coverage | Yes | PASS | BDD mandatory. All packages have `_test.go` files. Tests written FIRST per acceptance scenarios. `go test -race -shuffle=on ./...` |
| II. Testing Standards — testcontainers | No | N/A | No database. File-based integration tests use temp dirs. |
| II. Testing Standards — Mockery | No | N/A (deviation) | Few interfaces. Justified in Complexity Tracking. |
| III. API Consistency | No | N/A | No HTTP API. CLI consistency via subcommand conventions. |
| IV. Performance — file I/O | Yes | PASS | <100ms target for file ops up to 200 episodes. |
| Arch Constraints — Go version | Yes | PASS | Go 1.26.0 |
| Arch Constraints — Dependencies | Yes | PASS | Zero external deps. Stdlib only. |
| Arch Constraints — Observability | Partially | PASS with deviation | `log/slog` structured logging to stderr (stdlib). zerolog not used — external dep. Justified in Complexity Tracking. |
| Dev Workflow — Testing gates | Yes | PASS | `go test -race -shuffle=on ./...` and `go vet ./...` before merge. |

**Post-Phase 1 re-check**: All gates pass with documented deviations.

## Project Structure

### Documentation (this feature)

```text
specs/001-agent-memory-companion/
├── plan.md              # This file
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output
├── quickstart.md        # Phase 1 output
├── contracts/           # Phase 1 output (CLI interface)
└── tasks.md             # Phase 2 output (/speckit.tasks)
```

### Source Code (repository root)

```text
cmd/
└── mem/
    └── main.go              # CLI entry point, subcommand dispatch

internal/
├── episode/
│   ├── episode.go           # Episode struct, JSONL read/write/append
│   ├── episode_test.go      # BDD tests for episode CRUD
│   ├── dedup.go             # Deduplication: exact match on (type + normalized summary)
│   └── dedup_test.go        # BDD tests for dedup scenarios
├── principle/
│   ├── principle.go         # Principle struct, Markdown read/write
│   ├── principle_test.go    # BDD tests for principle CRUD
│   ├── merge.go             # Merge/dedup principles, enforce limits
│   └── merge_test.go        # BDD tests for merge scenarios
├── skill/
│   ├── skill.go             # Skill struct, Markdown read/write, slug gen
│   ├── skill_test.go        # BDD tests for skill CRUD
│   └── match.go             # Keyword-based skill trigger matching
│   └── match_test.go        # BDD tests for trigger matching
├── consolidation/
│   ├── consolidation.go     # Consolidation log read/write
│   └── consolidation_test.go # BDD tests for log operations
├── store/
│   ├── store.go             # MemoryStore: path resolution, init, session counter
│   └── store_test.go        # BDD tests for init and store operations
├── filelock/
│   ├── filelock.go          # File locking via syscall.Flock
│   └── filelock_test.go     # BDD tests for concurrent lock scenarios
├── runner/
│   ├── extract.go           # Extraction mode orchestrator
│   ├── extract_test.go      # BDD tests for extraction flow
│   ├── consolidate.go       # Consolidation mode orchestrator
│   ├── consolidate_test.go  # BDD tests for consolidation flow
│   ├── inject.go            # Injection mode orchestrator
│   └── inject_test.go       # BDD tests for injection flow
├── agent/
│   ├── agent.go             # Claude CLI invocation wrapper
│   └── agent_test.go        # BDD tests (mock via interface)
└── config/
    ├── config.go            # Configuration (thresholds, paths)
    └── config_test.go       # BDD tests for config loading

.memory/                     # Runtime data directory (per-project)
├── episodes.jsonl           # Append-only episode log
├── principles.md            # Extracted rules
├── skills/                  # One .md per skill
│   └── {slug}.md
├── consolidation-log.md     # Consolidation history
└── prompts/                 # LLM prompt templates
    ├── extract.md           # Extraction prompt
    └── consolidate.md       # Consolidation prompt
```

**Structure Decision**: Single-project CLI tool. Flat `internal/`
packages organized by domain concept (episode, principle, skill).
No layered architecture — a CLI tool does not warrant DDD layers.
The `runner/` package orchestrates the three modes (extract,
consolidate, inject) by composing the domain packages. The `agent/`
package wraps Claude CLI invocation via `os/exec`. The `skill/`
package includes `match.go` for keyword-based trigger matching
(remediates analysis finding H3).

## Testing Strategy: BDD

**Mandatory**: All code MUST be developed using BDD.

**Workflow per task**:
1. Write test file (`_test.go`) with Given/When/Then scenarios
   derived from spec.md acceptance criteria
2. Run tests — verify they FAIL (Red)
3. Implement the minimum code to make tests pass (Green)
4. Refactor if needed while keeping tests green

**Test naming convention**:
```go
func TestEpisode_GivenValidFields_WhenCreated_ThenAllFieldsSet(t *testing.T) { ... }
func TestDedup_GivenDuplicateSummary_WhenChecked_ThenReturnsDuplicate(t *testing.T) { ... }
func TestExtract_GivenRoutineSession_WhenRun_ThenNoEpisodesCreated(t *testing.T) { ... }
```

**Table-driven BDD pattern**:
```go
func TestEpisodeValidation(t *testing.T) {
    tests := []struct {
        name    string // "Given X, When Y, Then Z"
        episode Episode
        wantErr bool
    }{
        {
            name:    "Given valid episode, When validated, Then no error",
            episode: Episode{Type: "decision", Summary: "chose X", Tags: []string{"arch"}},
            wantErr: false,
        },
        {
            name:    "Given empty summary, When validated, Then error",
            episode: Episode{Type: "decision", Summary: "", Tags: []string{"arch"}},
            wantErr: true,
        },
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) { ... })
    }
}
```

**Agent package testability**: The `agent` package defines an
`Invoker` interface so runners can be tested with a stub:
```go
type Invoker interface {
    Invoke(model, prompt string) (string, error)
}
```
Runners accept `Invoker` — production uses `CLIInvoker` (real
claude CLI), tests use `StubInvoker` returning canned responses.

## Analysis Remediations

Findings from `/speckit.analyze` and their resolutions:

| Finding | Severity | Resolution |
|---------|----------|------------|
| C1: No test tasks | CRITICAL | Resolved: BDD is now mandatory. Every implementation task includes writing tests FIRST. Test files listed in project structure. |
| H1: Reversible consolidation | HIGH | Resolved: Episode cleanup uses atomic rewrite (write temp + rename). Files are git-tracked, so all changes are diffable and revertible via `git checkout`. Documented in research R3. |
| H2: Dedup algorithm underspecified | HIGH | Resolved: Exact match on `(type + strings.ToLower(strings.TrimSpace(summary)))`. Specified in research R2 addendum and project structure (dedup.go). |
| H3: Skill trigger matching missing in US3 | HIGH | Resolved: Added `internal/skill/match.go` with keyword-based trigger matching. Injection runner uses it to filter skills by relevance. |
| H4: Hook config task missing for FR-014 | HIGH | Resolved: `mem init` outputs sample hook configuration. Added to quickstart.md and will be a task in Phase 8. |
| M1: No structured logging | MEDIUM | Resolved: Use `log/slog` (stdlib, Go 1.21+). Structured JSON logging to stderr. No external dep needed. Deviation from zerolog documented in Complexity Tracking. |
| M2: Plan lists test files but tasks omit them | MEDIUM | Resolved: BDD mandate means tests are part of every implementation task, not separate tasks. Plan and tasks now consistent. |
| M3: No golangci-lint | MEDIUM | Resolved: `go vet ./...` as lint gate. Documented as deviation in Complexity Tracking. |
| M4: Exported episode fields | MEDIUM | Resolved: Documented in Complexity Tracking. JSON serialization requires exported fields; no domain invariants to protect in flat CLI tool. |
| M5: SC-005 untestable | MEDIUM | Acknowledged: Post-launch observational metric. Not instrumentable without Claude CLI token reporting API. |

## Complexity Tracking

| Deviation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| No DDD 4-layer architecture | CLI tool with file I/O, not a web service. Flat packages by domain concept are sufficient. | 4-layer would add unnecessary indirection for a tool with no HTTP handlers, no database, and no DI needs. |
| No Mockery / generated mocks | Single interface (`agent.Invoker`). Hand-written stub is 5 lines of code. | Generated mocks add tooling overhead disproportionate to the test surface. |
| No HTTP / Swagger | Tool is CLI-only, invoked by hooks or manually. No API consumers. | Adding HTTP would violate the companion spec's design. |
| Exported Episode struct fields | `encoding/json` requires exported fields for marshal/unmarshal. Episode has no complex invariants beyond validation. | Unexported fields + custom MarshalJSON/UnmarshalJSON adds ~40 lines of boilerplate with no safety benefit for a flat data struct. |
| `log/slog` instead of zerolog | `log/slog` is stdlib (Go 1.21+), provides structured logging without external dependency. | zerolog would add an external module, violating the minimal-deps constraint. |
| `go vet` instead of golangci-lint | This is a standalone CLI tool, not the warehouse API. `go vet` catches the most impactful issues. | Full golangci-lint requires `.golangci.yml` config and the golangci-lint binary. Can be added later if project grows. |
