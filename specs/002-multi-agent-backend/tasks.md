# Tasks: Multi-Agent Backend Support

**Input**: Design documents from `/specs/002-multi-agent-backend/`
**Prerequisites**: plan.md (required), spec.md (required), research.md, data-model.md

**Tests**: BDD is MANDATORY per plan.md. Every implementation task follows Red-Green-Refactor: write BDD test FIRST, verify it FAILS, then implement until tests pass.

**Organization**: Tasks are grouped by user story. This is a focused refactor (~200 lines) of existing code — no new packages, only modifications to `internal/agent/`, `internal/config/`, and `cmd/mem/main.go`.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)

---

## Phase 1: Setup

**Purpose**: Extend config and define the Backend abstraction

- [x] T001 BDD test + add `MEM_BACKEND`, `MEM_BACKEND_BINARY`, `MEM_BACKEND_ARGS` fields to Config struct and `Load()` in `internal/config/config.go` and `internal/config/config_test.go`. Scenarios: Given no MEM_BACKEND set, When loaded, Then Backend is empty; Given MEM_BACKEND=opencode, When loaded, Then Backend is "opencode"; Given MEM_BACKEND=custom with MEM_BACKEND_BINARY=my-agent and MEM_BACKEND_ARGS="-p {prompt}", When loaded, Then all three fields populated
- [x] T002 BDD test + implement Backend struct (Name, Binary, SupportsModel bool, BuildArgs func(prompt, model string) []string) and three built-in backend definitions (claude, opencode, codex) in `internal/agent/backend.go` and `internal/agent/backend_test.go`. Scenarios: Given claude backend, When BuildArgs("hello","haiku"), Then args are ["-p","hello","--model","haiku"]; Given opencode backend, When BuildArgs("hello",""), Then args are ["-p","hello","-q"]; Given codex backend, When BuildArgs("hello","o4-mini"), Then args are ["exec","hello","-m","o4-mini"]

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Backend registry, resolution, and invoker refactor — blocks all user stories

- [x] T003 BDD test + implement `Resolve(cfg, backendFlag string) (*Backend, error)` in `internal/agent/backend.go` and `internal/agent/backend_test.go`. Three-level resolution: (1) backendFlag if non-empty, (2) cfg.Backend env var, (3) auto-detect via exec.LookPath in order claude > opencode > codex. Scenarios: Given backendFlag="opencode", When resolved, Then opencode backend returned; Given cfg.Backend="codex" and no flag, When resolved, Then codex returned; Given no flag and no env and claude in PATH, When resolved, Then claude returned (auto-detect); Given no flag, no env, no binaries in PATH, When resolved, Then error "no supported backend found" with list of backends; Given cfg.Backend="gemini", When resolved, Then error "invalid backend" with valid values
- [x] T004 BDD test + implement custom backend construction from config (MEM_BACKEND_BINARY + MEM_BACKEND_ARGS template with {prompt}/{model} replacement) in `internal/agent/backend.go` and `internal/agent/backend_test.go`. Scenarios: Given MEM_BACKEND=custom, BINARY=my-agent, ARGS="-p {prompt} --model {model}", When BuildArgs("hello","sonnet"), Then args are ["-p","hello","--model","sonnet"]; Given ARGS="{prompt}" only (no model), When BuildArgs("hello","sonnet"), Then args are ["hello"] (model ignored); Given MEM_BACKEND=custom but BINARY empty, When resolved, Then error "MEM_BACKEND_BINARY is required"; Given ARGS not set, When resolved, Then default template "{prompt}" used
- [x] T005 BDD test + refactor `CLIInvoker` to use Backend struct — replace hardcoded `claude` binary and args with `Backend.Binary` and `Backend.BuildArgs()` in `internal/agent/agent.go` and `internal/agent/agent_test.go`. The Invoker interface stays unchanged. `NewInvoker(backend *Backend) Invoker` creates a CLIInvoker from a resolved backend. Scenarios: Given backend=opencode, When Invoke("","hello prompt"), Then exec.Command uses "opencode" binary with ["-p","hello prompt","-q"]; Given backend with non-existent binary, When Invoke called, Then error includes binary name; Read stdout only for response, capture stderr separately for error reporting per FR-013

**Checkpoint**: Backend struct defined, 3 built-ins registered, resolution works (flag > env > auto-detect), custom backend works, CLIInvoker uses Backend. All BDD tests pass.

---

## Phase 3: User Story 1 — OpenCode Support (Priority: P1) MVP

**Goal**: `MEM_BACKEND=opencode mem extract` works natively

**Independent Test**: Set `MEM_BACKEND=opencode`, run `mem extract`, verify OpenCode is invoked with `-p prompt -q`.

### Implementation for User Story 1

- [x] T006 [US1] Wire backend resolution into `runExtract` in `cmd/mem/main.go` — add `--backend` flag, call `agent.Resolve(cfg, backendFlag)`, pass resolved backend to `runner.RunExtract` via `agent.NewInvoker(backend)`. Update `runExtract` to accept the resolved invoker instead of hardcoding `&agent.CLIInvoker{}`
- [x] T007 [US1] Wire backend resolution into `runConsolidate` in `cmd/mem/main.go` — same pattern as T006, add `--backend` flag, resolve backend, pass to runner
- [x] T008 [US1] BDD test + add backend info to `mem status` output — show `Backend: <name> (<source>)` where source is "flag", "env", or "auto-detected" in `cmd/mem/main.go`. Scenarios: Given MEM_BACKEND=opencode, When status run, Then output includes "Backend: opencode (env)"; Given no config and claude auto-detected, When status run, Then output includes "Backend: claude (auto-detected)"; Given --json flag, When status run, Then JSON includes backend field

**Checkpoint**: `MEM_BACKEND=opencode mem extract` invokes OpenCode. `mem status` shows active backend. All BDD tests pass.

---

## Phase 4: User Story 2 — Codex Support (Priority: P2)

**Goal**: `MEM_BACKEND=codex mem extract` works with Codex's `exec` subcommand

**Independent Test**: Set `MEM_BACKEND=codex`, verify `codex exec "<prompt>" -m <model>` invocation pattern.

### Implementation for User Story 2

- [x] T009 [US2] BDD test + implement stdout/stderr separation in CLIInvoker — use `cmd.Stdout` pipe for response and `cmd.Stderr` pipe for error capture (instead of `CombinedOutput`) in `internal/agent/agent.go` and `internal/agent/agent_test.go`. Scenarios: Given backend writes "result" to stdout and "progress" to stderr, When invoked successfully, Then only stdout returned as response; Given backend exits non-zero with stderr "auth failed", When invoked, Then error includes "auth failed" from stderr

**Checkpoint**: Codex backend works with `exec` subcommand, stdout/stderr properly separated. All BDD tests pass.

---

## Phase 5: User Story 3 — Auto-Detection (Priority: P3)

**Goal**: `mem extract` with no config auto-detects the first available backend

**Independent Test**: With only one backend installed, run `mem extract` with no MEM_BACKEND — verify it's auto-detected.

### Implementation for User Story 3

- [x] T010 [US3] BDD test + implement auto-detection with exec.LookPath probing in `internal/agent/backend.go` and `internal/agent/backend_test.go`. Scenarios: Given only "opencode" in PATH (mock via custom PATH), When auto-detect runs, Then opencode backend returned; Given "claude" and "opencode" both in PATH, When auto-detect runs, Then claude returned (higher priority); Given nothing in PATH, When auto-detect runs, Then error with full list of supported backends and custom config instructions per FR-007

**Checkpoint**: Auto-detection works in priority order. Clear error when nothing found. All BDD tests pass.

---

## Phase 6: User Story 4 — Custom Backend (Priority: P4)

**Goal**: `MEM_BACKEND=custom` with BINARY and ARGS env vars works

**Independent Test**: Create a script that echoes a JSON array, configure as custom backend, run `mem extract`.

### Implementation for User Story 4

- [x] T011 [US4] BDD test + implement error messages for all FR-007 failure modes in `internal/agent/backend.go` and `internal/agent/backend_test.go`: invalid MEM_BACKEND value (list valid values), custom without BINARY (specific error), binary not found (include name + PATH hint), binary not executable (surface OS error). Scenarios: Given MEM_BACKEND="gemini", When resolved, Then error lists valid values; Given custom without BINARY, When resolved, Then error says "MEM_BACKEND_BINARY is required"; Given custom with non-existent binary, When invoked, Then error includes binary path

**Checkpoint**: All custom backend and error scenarios covered. All BDD tests pass.

---

## Phase 7: Polish & Cross-Cutting

**Purpose**: Backward compatibility verification, documentation, cleanup

- [x] T012 [P] BDD test backward compatibility — verify that with no MEM_BACKEND set and claude in PATH, the resolved backend produces identical args to the old hardcoded `CLIInvoker{Binary:"claude"}` in `internal/agent/backend_test.go`. Scenarios: Given no env/flag and claude available, When resolved and BuildArgs("prompt","haiku"), Then args identical to ["-p","prompt","--model","haiku"]
- [x] T013 [P] Update README.md — add "Multi-Backend Support" section documenting MEM_BACKEND, --backend flag, auto-detection, custom backend, and supported backends table in `README.md`
- [x] T014 [P] Update `mem init` hook config output — show generic hook (not Claude-specific) that works with any backend in `cmd/mem/main.go`
- [x] T015 Run `go test -race -shuffle=on ./...` and `go vet ./...` — verify all existing + new tests pass, no regressions

---

## Dependencies & Execution Order

### Phase Dependencies

- **Phase 1 (Setup)**: No dependencies — start immediately
- **Phase 2 (Foundational)**: Depends on Phase 1 — BLOCKS all user stories
- **US1 (Phase 3)**: Depends on Phase 2 — wires resolution into CLI
- **US2 (Phase 4)**: Depends on Phase 2 — stdout/stderr separation
- **US3 (Phase 5)**: Depends on Phase 2 — auto-detection logic (most already in T003)
- **US4 (Phase 6)**: Depends on Phase 2 — error messages (most already in T004)
- **Polish (Phase 7)**: Depends on all user stories

### User Story Dependencies

- **US1 (OpenCode)**: Can start after Phase 2. MVP.
- **US2 (Codex)**: Can start after Phase 2, parallel with US1.
- **US3 (Auto-detect)**: Most logic in T003 (Phase 2). Phase 5 is additional testing.
- **US4 (Custom)**: Most logic in T004 (Phase 2). Phase 6 is error message polish.

### Parallel Opportunities

- Phase 1: T001, T002 can run in parallel (different files)
- Phase 2: T003, T004 after T002 (depend on Backend struct)
- Phase 3-5: US1, US2, US3 can proceed in parallel after Phase 2
- Phase 7: T012, T013, T014 can run in parallel

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Phase 1: Config + Backend struct (T001-T002)
2. Phase 2: Resolution + Custom + Refactor CLIInvoker (T003-T005)
3. Phase 3: Wire into CLI + Status output (T006-T008)
4. **STOP and VALIDATE**: `MEM_BACKEND=opencode mem extract` works

### Incremental Delivery

1. Phase 1-2 → Backend abstraction ready
2. US1 → OpenCode works (MVP!)
3. US2 → Codex works (stdout/stderr separation)
4. US3 → Auto-detection works
5. US4 → Custom backend + error messages polished
6. Polish → Backward compat verified, README updated

---

## Notes

- This is a **focused refactor** of ~200 lines, not a greenfield feature
- The `Invoker` interface does NOT change — only its implementations
- All existing runner tests continue to work (they use `StubInvoker`)
- BDD mandatory: tests FIRST, then implement
- `go test -race -shuffle=on ./...` must pass after every task
- Zero new external dependencies
