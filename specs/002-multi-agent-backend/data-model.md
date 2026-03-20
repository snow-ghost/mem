# Data Model: Multi-Agent Backend Support

**Date**: 2026-03-20
**Feature**: 002-multi-agent-backend

## Entities

### Backend

A supported LLM CLI tool that `mem` can invoke for extraction
and consolidation.

| Field        | Type     | Required | Description |
|--------------|----------|----------|-------------|
| Name         | string   | Yes      | Unique identifier: "claude", "opencode", "codex", "custom" |
| Binary       | string   | Yes      | Executable name or path (e.g., "claude", "/usr/local/bin/opencode") |
| DefaultModel | string   | Yes      | Default model name for this backend (e.g., "haiku", "o4-mini") |
| BuildArgs    | function | Yes      | Function that takes (prompt, model) and returns []string args |

**Built-in backends**:

| Name     | Binary     | Default Model | Args Pattern |
|----------|------------|---------------|--------------|
| claude   | claude     | (per-command)  | `-p`, prompt, `--model`, model |
| opencode | opencode   | (per-command)  | `-p`, prompt, `-q` |
| codex    | codex      | (per-command)  | `exec`, prompt, `-m`, model |

Note: Default model is not set at the backend level — it remains
per-command (haiku for extract, sonnet for consolidate). Backends
that don't support model selection (like opencode when using
provider defaults) simply ignore the model arg.

### Backend Registry

The collection of all known backends. Not persisted — built in
memory at startup from hardcoded definitions + env var overrides.

**Resolution order**: `--backend` flag > `MEM_BACKEND` env > auto-detect

**Auto-detect order**: claude > opencode > codex
