# Research: Multi-Agent Backend Support

**Date**: 2026-03-20
**Feature**: 002-multi-agent-backend

## R1: Backend CLI Invocation Patterns

**Decision**: Define each backend as a struct with binary name,
argument builder function, and default model. Three built-in
backends.

**Research findings** (from official docs):

### Claude Code
- Binary: `claude`
- Invocation: `claude -p "<prompt>" --model <model>`
- Output: text to stdout, may contain non-JSON wrapper text
- Default model: `haiku`

### OpenCode
- Binary: `opencode`
- Invocation: `opencode -p "<prompt>" -q`
- The `-q` (quiet) flag suppresses the spinner animation, important
  for scripted use
- Output: text to stdout
- Default model: (uses configured provider default)
- Source: https://opencode.ai/docs/cli/

### Codex (OpenAI)
- Binary: `codex`
- Invocation: `codex exec "<prompt>" -m <model>`
- Uses `exec` subcommand for non-interactive mode
- Streams progress to stderr, final answer to stdout
- Default model: `o4-mini`
- Source: https://developers.openai.com/codex/noninteractive

**Rationale**: Each backend has a unique arg pattern. An argument
builder function per backend is cleaner than a string template,
because some backends use subcommands (codex exec) while others
use flags (-p).

**Alternatives considered**:
- String template with `{prompt}` and `{model}` placeholders:
  Doesn't handle subcommands well. `codex exec "{prompt}" -m {model}`
  works as a template, but edge cases with quoting are fragile.
- Single universal flag pattern: Impossible — backends differ
  fundamentally in how they accept prompts.

---

## R2: Backend Resolution Strategy

**Decision**: Three-level resolution:
1. `--backend` flag (per-command override)
2. `MEM_BACKEND` env var (persistent preference)
3. Auto-detect: probe `exec.LookPath` for binaries in priority
   order: `claude` > `opencode` > `codex`

**Rationale**: Follows standard CLI convention (flag > env > default).
Auto-detect uses `exec.LookPath` which is stdlib and returns
immediately if the binary is in PATH.

**Priority order**: Claude first because `mem` was originally
built for Claude Code users. OpenCode second as the most popular
open-source alternative. Codex third.

---

## R3: Custom Backend Support

**Decision**: Users configure a custom backend via two env vars:
- `MEM_BACKEND=custom`
- `MEM_BACKEND_BINARY=<path>` — the binary to execute
- `MEM_BACKEND_ARGS=<template>` — argument template with
  `{prompt}` and `{model}` placeholders

Example:
```bash
MEM_BACKEND=custom
MEM_BACKEND_BINARY=my-agent
MEM_BACKEND_ARGS="-p {prompt} --model {model}"
```

The system splits `MEM_BACKEND_ARGS` by spaces, replaces `{prompt}`
and `{model}` with actual values, and executes.

**Rationale**: Simple, no config files needed. Environment variables
are the project's established configuration pattern (MEM_PATH,
MEM_SESSION_THRESHOLD, etc.). Template placeholders are intuitive.

**Alternatives considered**:
- YAML/JSON config file: Adds file I/O and parsing complexity.
  Overkill for 2 settings.
- Go plugin system: External dependency, platform-specific. Way
  too complex.

---

## R4: Backward Compatibility

**Decision**: Zero breaking changes. When `MEM_BACKEND` is not set
and Claude is installed, behavior is identical to current version.

- The `--model` flag defaults remain per-command (haiku for extract,
  sonnet for consolidate)
- If `MEM_BACKEND` is not set and Claude is found, Claude is used
  with exactly the same args as before
- The only visible change: when Claude is NOT found but another
  backend is, `mem` now works instead of failing

---

## R5: Error Message Design

**Decision**: When a backend binary is not found, show:
```
mem: extract: backend "opencode" not found
  Binary "opencode" is not installed or not in PATH.
  Supported backends: claude, opencode, codex
  Set MEM_BACKEND to choose a different backend.
```

When no backend is found at all:
```
mem: extract: no supported backend found
  Install one of: claude, opencode, codex
  Or configure a custom backend:
    MEM_BACKEND=custom
    MEM_BACKEND_BINARY=/path/to/binary
    MEM_BACKEND_ARGS="-p {prompt} --model {model}"
```
