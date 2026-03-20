# Tasks: Agent Memory Companion

**Input**: Design documents from `/specs/001-agent-memory-companion/`
**Prerequisites**: plan.md (required), spec.md (required), research.md, data-model.md, contracts/cli-interface.md

**Tests**: BDD is MANDATORY per plan.md. Every implementation task follows Red-Green-Refactor: write BDD test FIRST (`_test.go` with Given/When/Then naming), verify it FAILS, then implement until tests pass. Tests and implementation are a single task — not separate.

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

## Path Conventions

- **CLI tool**: `cmd/mem/` for entry point, `internal/` for packages
- `.memory/` for runtime data (prompt templates embedded as Go defaults)
- BDD: Each `.go` file has a colocated `_test.go` with Given/When/Then scenarios

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Initialize Go module, project structure, and cross-cutting concerns

- [ ] T001 Initialize Go 1.26 module (`go mod init`) and create all directories per plan.md: `cmd/mem/`, `internal/episode/`, `internal/principle/`, `internal/skill/`, `internal/consolidation/`, `internal/store/`, `internal/filelock/`, `internal/runner/`, `internal/agent/`, `internal/config/`
- [ ] T002 [P] BDD test + implement configuration loading from environment variables with defaults in `internal/config/config.go` and `internal/config/config_test.go`. Scenarios: Given no env vars set, When config loaded, Then defaults applied (MEM_PATH=.memory, thresholds=10/100/100/200/50); Given MEM_SESSION_THRESHOLD=20, When config loaded, Then threshold is 20; Given MEM_AGENT_ID=agent-1, When config loaded, Then agent ID is "agent-1"
- [ ] T003 [P] BDD test + implement file locking via `syscall.Flock` with `WithLock(lockPath, fn)` helper in `internal/filelock/filelock.go` and `internal/filelock/filelock_test.go`. Scenarios: Given lock file does not exist, When WithLock called, Then lock acquired and fn executed; Given lock held by another goroutine, When WithLock called, Then blocks until released; Given fn returns error, When WithLock called, Then error propagated and lock released
- [ ] T004 [P] Set up `log/slog` structured logger writing JSON to stderr in `cmd/mem/main.go`. Configure default handler at startup. All packages use `slog.Warn`/`slog.Error` for operational messages (not debug noise)

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Core data packages and store initialization that ALL user stories depend on

**CRITICAL**: No user story work can begin until this phase is complete

- [ ] T005 [P] BDD test + implement Episode struct with JSON tags, Validate() method (type enum check, non-empty summary <=500 chars, 1-3 non-empty tags, valid ISO 8601 ts), ReadAll (bufio.Scanner, skip corrupt lines with slog.Warn), and Append (O_APPEND under file lock) in `internal/episode/episode.go` and `internal/episode/episode_test.go`. Scenarios: Given valid fields, When validated, Then no error; Given unknown type "foo", When validated, Then error; Given corrupt JSONL line among valid ones, When ReadAll, Then corrupt line skipped and valid episodes returned; Given valid episode, When appended, Then file contains new line
- [ ] T006 [P] BDD test + implement Principle struct and Markdown read/write in `internal/principle/principle.go` and `internal/principle/principle_test.go`. Parse: read `## topic` headings + `- rule` lines into map[string][]string. Write: render map back to Markdown. Count: return total rules. Scenarios: Given markdown with 2 topics and 5 rules, When parsed, Then map has 2 keys with correct rules; Given empty file, When parsed, Then empty map; Given map with 3 topics, When written, Then valid Markdown with correct headings
- [ ] T007 [P] BDD test + implement Skill struct and Markdown read/write/list in `internal/skill/skill.go` and `internal/skill/skill_test.go`. Parse: read skill sections (name, triggers, prerequisites, steps, verification, antipatterns). Write: render to Markdown. List: return all .md filenames in skills/ dir. Scenarios: Given valid skill markdown, When parsed, Then all sections populated; Given skill struct, When written, Then file matches expected format; Given skills/ dir with 3 files, When listed, Then 3 slugs returned
- [ ] T008 [P] BDD test + implement ConsolidationLogEntry struct and Append/ReadLast in `internal/consolidation/consolidation.go` and `internal/consolidation/consolidation_test.go`. Scenarios: Given empty log file, When entry appended, Then file contains formatted section with correct counts; Given log with 3 entries, When ReadLast called, Then most recent entry returned with correct date and number
- [ ] T009 [P] BDD test + implement Invoker interface and CLIInvoker (wraps `os/exec.Command("claude", "-p", prompt, "--model", model)`) in `internal/agent/agent.go` and `internal/agent/agent_test.go`. Also implement StubInvoker for test use. Scenarios: Given StubInvoker with canned response, When Invoke called, Then canned response returned; Given CLIInvoker with non-existent binary, When Invoke called, Then error wrapped with context
- [ ] T010 BDD test + implement MemoryStore with Init (create .memory/ directory tree: episodes.jsonl, principles.md, skills/, consolidation-log.md, prompts/) and path resolution helpers (EpisodesPath, PrinciplesPath, SkillsDir, etc.) in `internal/store/store.go` and `internal/store/store_test.go`. Scenarios: Given empty directory, When Init called, Then all files and dirs created; Given already initialized store, When Init called, Then returns "already initialized" error; Given initialized store, When paths resolved, Then correct absolute paths returned
- [ ] T011 BDD test + implement CLI entry point with subcommand dispatch (init, extract, consolidate, inject, status) and `mem init` wiring in `cmd/mem/main.go`. Scenarios: Given no arguments, When run, Then usage printed to stderr and exit 1; Given "init" argument, When run in empty dir, Then .memory/ created and success message printed; Given unknown command, When run, Then usage printed and exit 1
- [ ] T012 BDD test + implement `mem status` command showing episode/principle/skill counts, session counter, last consolidation date, store size with `--json` flag in `cmd/mem/main.go`. Scenarios: Given initialized store with 5 episodes and 3 principles, When status run, Then correct counts displayed; Given --json flag, When status run, Then valid JSON output with all fields

**Checkpoint**: `mem init` creates valid store, `mem status` reports correct counts, all entity packages read/write correctly, all BDD tests pass with `go test -race -shuffle=on ./...`

---

## Phase 3: User Story 1 — Automatic Event Capture (Priority: P1) MVP

**Goal**: After a work session, capture significant events as episode records

**Independent Test**: Run `mem extract` after a session with at least one decision and one bug fix. Verify 1-5 relevant episodes appear in `.memory/episodes.jsonl` with no duplicates.

### Implementation for User Story 1

- [ ] T013 [US1] BDD test + implement episode deduplication with exact match on `(type + strings.ToLower(strings.TrimSpace(summary)))` in `internal/episode/dedup.go` and `internal/episode/dedup_test.go`. Scenarios: Given existing episode "decision: chose JSONL", When IsDuplicate checked with same type+summary, Then returns true; Given existing "decision: chose JSONL", When checked with "error: chose JSONL", Then returns false (different type); Given existing "decision: Chose JSONL", When checked with "decision: chose jsonl", Then returns true (case-insensitive); Given empty episode store, When any episode checked, Then returns false
- [ ] T014 [US1] BDD test + implement session counter read/increment/reset in `internal/store/store.go` and `internal/store/store_test.go`. Scenarios: Given no .session-count file, When ReadSessionCount, Then returns 0; Given count=7, When IncrementSessionCount, Then file contains 8; Given count=10, When ResetSessionCount, Then file contains 0
- [ ] T015 [US1] Create default extraction prompt template as embedded Go string constant in `internal/runner/extract.go`. Prompt instructs LLM to: analyze provided git diff context, identify significant events (decisions, errors, patterns, insights, rollbacks), filter routine operations, output JSON array of episode objects `[{"type":"...", "summary":"...", "tags":["..."]}]`
- [ ] T016 [US1] BDD test + implement extraction runner `RunExtract(cfg, store, invoker)` in `internal/runner/extract.go` and `internal/runner/extract_test.go`. Orchestration: gather git diff via `os/exec`, read last 20 episodes for context, read principles, build prompt from template + context, invoke agent with haiku model, parse JSON response into []Episode, deduplicate against existing, append new episodes under file lock, increment session count. Scenarios (using StubInvoker): Given stub returns JSON with 2 episodes, When RunExtract, Then 2 episodes appended to file; Given stub returns JSON with 1 episode matching existing, When RunExtract, Then 0 new episodes appended (dedup); Given stub returns empty array, When RunExtract, Then no episodes appended and no error; Given stub returns malformed response, When RunExtract, Then error returned with context
- [ ] T017 [US1] BDD test + implement threshold check in extraction runner — after extraction, compare session count and episode count against config thresholds, return threshold status in `internal/runner/extract.go` and `internal/runner/extract_test.go`. Scenarios: Given session count 9 with threshold 10, When checked, Then no recommendation; Given session count 10, When checked, Then consolidation recommended; Given episode count 101 with threshold 100, When checked, Then consolidation recommended
- [ ] T018 [US1] BDD test + wire `mem extract` command with `--session`, `--model`, `--dry-run` flags in `cmd/mem/main.go`. Scenarios: Given --dry-run flag, When extract run, Then episodes printed but not written to file; Given no flags, When extract run, Then default session=git-hash and model=haiku used

**Checkpoint**: `mem extract` captures events via LLM, deduplicates, tracks session count, recommends consolidation. All BDD tests pass.

---

## Phase 4: User Story 2 — Memory Consolidation (Priority: P2)

**Goal**: Periodically analyze episodes, extract principles, detect skill candidates, and clean up store

**Independent Test**: Seed 30+ episodes with 3+ similar migration events. Run `mem consolidate --force`. Verify migration principle extracted, duplicates removed, consolidation log updated.

### Implementation for User Story 2

- [ ] T019 [US2] BDD test + implement principle merge, dedup, and limit enforcement (max from config, default 100) in `internal/principle/merge.go` and `internal/principle/merge_test.go`. Scenarios: Given two principle maps with overlapping topics, When merged, Then combined under same topic headings without duplicates; Given 105 principles, When limit enforced at 100, Then oldest/least-specific 5 removed; Given duplicate rules under same topic, When deduped, Then only one copy remains
- [ ] T020 [US2] Create default consolidation prompt template as embedded Go string constant in `internal/runner/consolidate.go`. Prompt instructs LLM to: group episodes by tags, extract principles from 3+ clusters, identify skill candidates, flag duplicates/outdated episodes, output structured JSON: `{"new_principles":[{"topic":"...","rule":"..."}], "episodes_to_remove":[indices], "skill_candidates":[{"name":"...","occurrences":N}]}`
- [ ] T021 [US2] BDD test + implement consolidation runner `RunConsolidate(cfg, store, invoker)` in `internal/runner/consolidate.go` and `internal/runner/consolidate_test.go`. Orchestration: check thresholds (skip if not met and not force), read all episodes + principles + skill list, build prompt, invoke agent with sonnet model, parse response, merge new principles, remove flagged episodes, write consolidation log entry, reset session counter. Scenarios (using StubInvoker): Given stub returns 2 new principles and 3 episodes to remove, When RunConsolidate with --force, Then principles updated and 3 episodes removed; Given thresholds not met and no --force, When RunConsolidate, Then skipped with exit code 3
- [ ] T022 [US2] BDD test + implement episode cleanup with atomic rewrite (write to temp file + rename) under file lock in `internal/runner/consolidate.go` and `internal/runner/consolidate_test.go`. Enforce max 200 episodes, keep newest 50 protected. Scenarios: Given 210 episodes, When cleanup runs, Then oldest episodes beyond 200 removed but newest 50 untouched; Given episodes marked for removal by indices, When cleanup runs, Then those episodes absent from rewritten file; Given crash during write, When temp file examined, Then original file unchanged (atomic)
- [ ] T023 [US2] Wire consolidation log writing and session counter reset into consolidation runner in `internal/runner/consolidate.go`. After successful consolidation: call consolidation.Append with all counts, call store.ResetSessionCount
- [ ] T024 [US2] BDD test + wire `mem consolidate` command with `--model`, `--dry-run`, `--force` flags in `cmd/mem/main.go`. Scenarios: Given --dry-run flag, When consolidate run, Then proposed changes printed but no files modified; Given --force flag, When thresholds not met, Then consolidation runs anyway; Given no flags and thresholds not met, When run, Then exit code 3 with message

**Checkpoint**: `mem consolidate` extracts principles, removes duplicates, logs operations, enforces limits. All BDD tests pass.

---

## Phase 5: User Story 3 — Context Injection (Priority: P3)

**Goal**: Assemble and output relevant memory context at the start of each new agent session

**Independent Test**: Populate memory with 5 principles, 20 episodes, and 2 skills (one matching recent tags). Run `mem inject`. Verify output contains all principles, 10 most recent episodes, and only the matching skill.

### Implementation for User Story 3

- [ ] T025 [P] [US3] BDD test + implement keyword-based skill trigger matching in `internal/skill/match.go` and `internal/skill/match_test.go`. Compare skill trigger keywords against recent episode tags. Scenarios: Given skill with trigger "database migration" and recent tags ["migration","schema"], When matched, Then skill included; Given skill with trigger "deploy staging" and recent tags ["migration","schema"], When matched, Then skill not included; Given no skills match, When matched, Then empty list returned (caller falls back to listing all)
- [ ] T026 [US3] BDD test + implement injection context assembly `RunInject(cfg, store)` in `internal/runner/inject.go` and `internal/runner/inject_test.go`. Read all principles, read last N episodes (configurable, default 10), load all skills, match skills against recent episode tags, assemble context. Scenarios: Given 5 principles and 20 episodes, When RunInject with episodes=10, Then context contains 5 principles and 10 most recent episodes; Given empty memory store, When RunInject, Then empty context returned without error
- [ ] T027 [P] [US3] BDD test + implement Markdown output formatter in `internal/runner/inject.go` and `internal/runner/inject_test.go`. Format: `# Project Memory` / `## Principles` (bullet list) / `## Recent Events` (`- [date] [type] summary`) / `## Relevant Skills` (name + triggers, or "## Available Skills" with all if no matches). Scenarios: Given 3 principles and 5 episodes and 1 matched skill, When formatted as markdown, Then output has correct headings and content
- [ ] T028 [P] [US3] BDD test + implement JSON output formatter in `internal/runner/inject.go` and `internal/runner/inject_test.go`. Output: `{"principles":[...], "episodes":[...], "skills":[...]}`. Scenarios: Given context with data, When formatted as JSON, Then valid JSON with all fields; Given empty context, When formatted, Then JSON with empty arrays
- [ ] T029 [US3] BDD test + wire `mem inject` command with `--episodes` and `--format` flags in `cmd/mem/main.go`. Scenarios: Given --format json, When inject run, Then JSON output to stdout; Given --episodes 5, When inject run, Then only 5 most recent episodes in output; Given empty store, When inject run, Then exit 0 with empty output

**Checkpoint**: `mem inject` outputs relevant, filtered memory context in Markdown or JSON. Skill trigger matching surfaces relevant skills. All BDD tests pass.

---

## Phase 6: User Story 4 — Procedural Skill Library (Priority: P4)

**Goal**: Automatically detect and create reusable skill recipes from repeated procedural patterns

**Independent Test**: Seed 5+ episodes describing similar DB migration steps. Run `mem consolidate --force`. Verify skill file created in `.memory/skills/` with all required sections.

### Implementation for User Story 4

- [ ] T030 [US4] BDD test + implement slug generation from skill name (lowercase, spaces to hyphens, strip non-alphanumeric, collapse consecutive hyphens) in `internal/skill/skill.go` and `internal/skill/skill_test.go`. Scenarios: Given "Database Migration", When slugified, Then "database-migration"; Given "Fix: Race Condition!!", When slugified, Then "fix-race-condition"
- [ ] T031 [US4] BDD test + implement skill creation from consolidation results in `internal/runner/consolidate.go` and `internal/runner/consolidate_test.go`. When LLM identifies a skill candidate with 3+ occurrences and returns full skill sections, create skill file via skill.Write in `.memory/skills/{slug}.md`. Scenarios: Given consolidation response with skill candidate "database-migration" having 3+ occurrences and full sections, When processed, Then skill file created with correct content; Given candidate with only 2 occurrences, When processed, Then no skill created (logged as candidate)
- [ ] T032 [US4] BDD test + implement skill staleness detection — flag skills with created_at older than 6 months during consolidation, include in output as warnings in `internal/runner/consolidate.go` and `internal/runner/consolidate_test.go`. Scenarios: Given skill created 7 months ago, When staleness checked, Then flagged as stale; Given skill created 2 months ago, When checked, Then not flagged
- [ ] T033 [US4] Update consolidation prompt template to include skill detection instructions: when 3+ episodes describe similar multi-step procedures, output skill sections (triggers, prerequisites, steps, verification, antipatterns) in consolidation JSON response in `internal/runner/consolidate.go`

**Checkpoint**: Consolidation creates well-structured skill files from detected patterns. Stale skills flagged. All BDD tests pass.

---

## Phase 7: User Story 5 — Multi-Agent Coordination (Priority: P5)

**Goal**: Enable multiple concurrent agents to share memory without conflicts

**Independent Test**: Run two `mem extract` processes concurrently writing to the same store. Verify no data loss, each episode has agent_id, conflicting decisions flagged.

### Implementation for User Story 5

- [ ] T034 [US5] BDD test + implement agent_id field support: add MEM_AGENT_ID to config (fallback: hostname-PID), set agent_id on Episode during creation in `internal/config/config.go`, `internal/episode/episode.go` and their `_test.go` files. Scenarios: Given MEM_AGENT_ID="agent-1", When episode created, Then agent_id is "agent-1"; Given no env var, When agent ID resolved, Then format is "hostname-PID"
- [ ] T035 [US5] Set agent_id on all new episodes during extraction — update RunExtract to read agent ID from config and apply to each parsed episode in `internal/runner/extract.go`
- [ ] T036 [US5] BDD test + implement conflict detection during consolidation in `internal/runner/consolidate.go` and `internal/runner/consolidate_test.go`. Identify episodes of type "decision" from different agent_ids with overlapping tags. Scenarios: Given 2 "decision" episodes from agents A and B with same tag "storage", When conflicts detected, Then conflict reported with both summaries; Given 2 "decision" episodes from same agent, When checked, Then no conflict; Given "error" episodes from different agents with same tag, When checked, Then no conflict (only decisions conflict)
- [ ] T037 [US5] BDD test + add conflict section to consolidation log entry and stdout output when conflicts detected in `internal/runner/consolidate.go` and `internal/runner/consolidate_test.go`. Scenarios: Given 2 detected conflicts, When consolidation log written, Then conflicts section present with agent IDs and summaries

**Checkpoint**: Concurrent agents safely write episodes with identity. Conflicting decisions detected and reported. All BDD tests pass.

---

## Phase 8: Polish & Cross-Cutting Concerns

**Purpose**: Improvements that affect multiple user stories

- [ ] T038 [P] Add `--path` flag support to all commands (override MEM_PATH default) in `cmd/mem/main.go`
- [ ] T039 [P] BDD test + implement default prompt template file creation during `mem init` — write embedded extraction and consolidation prompts to `.memory/prompts/extract.md` and `.memory/prompts/consolidate.md`. At runtime, if file exists read it instead of embedded default (allows user customization) in `internal/store/store.go` and `internal/store/store_test.go`. Scenarios: Given init run, When prompts/ checked, Then extract.md and consolidate.md exist with default content; Given custom extract.md exists, When extraction loads prompt, Then custom content used
- [ ] T040 [P] Add sample Claude Code hook configuration output to `mem init` — after creating .memory/, print suggested `settings.json` PostToolUse hook config to stdout in `cmd/mem/main.go`
- [ ] T041 Ensure consistent error message format (`mem: <command>: <message>`) across all commands per CLI contract in `cmd/mem/main.go`
- [ ] T042 [P] Run quickstart.md validation — verify all commands work end-to-end per validation checklist

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies — can start immediately
- **Foundational (Phase 2)**: Depends on Setup completion — BLOCKS all user stories
- **User Stories (Phase 3+)**: All depend on Foundational phase completion
  - US1 (extraction) can proceed independently
  - US2 (consolidation) can proceed independently (seed test data manually)
  - US3 (injection) can proceed independently
  - US4 (skill library) depends on US2 consolidation runner existing
  - US5 (multi-agent) depends on US1 extraction runner existing
- **Polish (Phase 8)**: Depends on all desired user stories being complete

### User Story Dependencies

- **User Story 1 (P1)**: Can start after Foundational (Phase 2) — No dependencies on other stories
- **User Story 2 (P2)**: Can start after Foundational (Phase 2) — Independent of US1 (seed test data)
- **User Story 3 (P3)**: Can start after Foundational (Phase 2) — Read-only, no write dependency
- **User Story 4 (P4)**: Depends on US2 (extends consolidation runner with skill creation)
- **User Story 5 (P5)**: Depends on US1 (extends extraction runner with agent_id)

### Within Each User Story (BDD Workflow)

1. Write BDD tests FIRST — verify they FAIL (Red)
2. Implement minimum code to pass tests (Green)
3. Refactor while keeping tests green
4. Entity/data packages before runner orchestration
5. Runner orchestration before CLI command wiring

### Parallel Opportunities

- Phase 1: T002, T003, T004 can run in parallel
- Phase 2: T005, T006, T007, T008, T009 can all run in parallel (different packages)
- Phase 2: T010, T011, T012 depend on entity packages
- Phase 3-5: US1, US2, US3 can all start in parallel after Phase 2
- Phase 5: T025, T027, T028 can run in parallel
- Phase 8: T038, T039, T040, T042 can run in parallel

---

## Parallel Example: Phase 2 Foundational

```bash
# Launch all entity packages in parallel (each includes BDD tests):
Task: "BDD test + Episode struct/JSONL in internal/episode/"
Task: "BDD test + Principle struct/Markdown in internal/principle/"
Task: "BDD test + Skill struct/Markdown in internal/skill/"
Task: "BDD test + ConsolidationLog in internal/consolidation/"
Task: "BDD test + Invoker interface in internal/agent/"

# Then (after above complete):
Task: "BDD test + MemoryStore init in internal/store/"
Task: "BDD test + CLI dispatch + init in cmd/mem/"
Task: "BDD test + status command in cmd/mem/"
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup
2. Complete Phase 2: Foundational (CRITICAL — blocks all stories)
3. Complete Phase 3: User Story 1 (extraction)
4. **STOP and VALIDATE**: Run `go test -race -shuffle=on ./...` — all green
5. Manual validation: run 5 real sessions, verify 1-5 episodes each
6. Usable as post-session hook immediately

### Incremental Delivery

1. Setup + Foundational → `mem init` and `mem status` work (all BDD tests green)
2. US1 (extraction) → `mem extract` captures events (MVP!)
3. US2 (consolidation) → `mem consolidate` cleans and synthesizes
4. US3 (injection) → `mem inject` surfaces context with skill matching
5. US4 (skills) → Consolidation auto-creates skill files
6. US5 (multi-agent) → Safe concurrent access with conflict detection
7. Polish → Consistent CLI, prompt file overrides, hook config, validation

---

## Notes

- [P] tasks = different files, no dependencies
- [Story] label maps task to specific user story for traceability
- BDD MANDATORY: Every task includes writing tests FIRST, then implementing
- Test naming: `TestX_GivenY_WhenZ_ThenW` (table-driven where appropriate)
- Runner tests use StubInvoker to avoid real Claude CLI calls
- `go test -race -shuffle=on ./...` must pass after every task
- Commit after each task or logical group
- Stop at any checkpoint to validate story independently
- Zero external Go dependencies — all stdlib
