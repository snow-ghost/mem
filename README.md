# mem — Persistent Memory for AI Coding Agents

`mem` gives your AI coding agent long-term memory. It captures what happened across sessions, extracts reusable principles, builds a library of proven procedures, and feeds relevant context back at the start of each new session.

Works with **Claude Code**, **OpenCode**, and **Codex** out of the box. Auto-detected, no wrappers needed.

Three types of memory, all stored as human-readable files:

- **Episodic** (`episodes.jsonl`) — significant events: decisions, bugs, patterns, insights, rollbacks
- **Semantic** (`principles.md`) — extracted rules grouped by topic
- **Procedural** (`skills/*.md`) — step-by-step recipes for recurring tasks

## Install

### Pre-built binaries (recommended)

Download from [GitHub Releases](https://github.com/snow-ghost/mem/releases/latest):

```bash
# macOS (Apple Silicon)
curl -fsSL https://github.com/snow-ghost/mem/releases/latest/download/mem-darwin-arm64.tar.gz | tar xz
sudo mv mem-darwin-arm64 /usr/local/bin/mem

# macOS (Intel)
curl -fsSL https://github.com/snow-ghost/mem/releases/latest/download/mem-darwin-amd64.tar.gz | tar xz
sudo mv mem-darwin-amd64 /usr/local/bin/mem

# Linux (x86_64)
curl -fsSL https://github.com/snow-ghost/mem/releases/latest/download/mem-linux-amd64.tar.gz | tar xz
sudo mv mem-linux-amd64 /usr/local/bin/mem

# Linux (ARM64)
curl -fsSL https://github.com/snow-ghost/mem/releases/latest/download/mem-linux-arm64.tar.gz | tar xz
sudo mv mem-linux-arm64 /usr/local/bin/mem
```

### Via Go

```bash
go install github.com/snow-ghost/mem/cmd/mem@latest
```

### Docker

```bash
docker run --rm -v $(pwd):/project ghcr.io/snow-ghost/mem status
```

Or build locally:

```bash
docker build -t mem .
docker run --rm -v $(pwd):/project mem status
```

### From source

```bash
git clone https://github.com/snow-ghost/mem.git
cd mem
go build -o mem ./cmd/mem
sudo mv mem /usr/local/bin/
```

### Requirements

At least one supported AI coding agent:

| Backend | Binary | Install |
|---------|--------|---------|
| Claude Code | `claude` | [docs.anthropic.com](https://docs.anthropic.com/en/docs/claude-code) |
| OpenCode | `opencode` | [opencode.ai](https://opencode.ai) |
| Codex (OpenAI) | `codex` | [developers.openai.com](https://developers.openai.com/codex/cli) |

`mem` auto-detects which one is installed. No configuration needed.

## Quick Start

### 1. Start using mem

```bash
cd your-project
mem status
```

That's it. The memory store is auto-initialized on first use — no `mem init` needed (though `mem init` still works for explicit setup).

The `.memory/` directory:

```
.memory/
├── episodes.jsonl          # event log (append-only)
├── principles.md           # extracted rules
├── skills/                 # one file per reusable procedure
├── consolidation-log.md    # consolidation history
└── prompts/                # LLM prompt templates (editable)
    ├── extract.md
    └── consolidate.md
```

### 2. Set up automatic extraction

#### Claude Code

Add to your Claude Code `settings.json`:

```json
{
  "hooks": {
    "PostToolUse": [
      {
        "matcher": "Stop",
        "command": "mem extract"
      }
    ]
  }
}
```

#### OpenCode

Copy the plugin to your project:

```bash
mkdir -p .opencode/plugins
curl -fsSL https://raw.githubusercontent.com/snow-ghost/mem/main/.opencode/plugins/mem.ts \
  -o .opencode/plugins/mem.ts
```

The plugin automatically injects memory context at session start and extracts events when the session goes idle.

#### Codex / Other agents

Run after each session manually or via your agent's hook system:

```bash
mem extract
```

### 3. Check what's stored

```bash
mem status
```

```
Memory Store: /home/user/project/.memory
  Episodes:       12 / 200
  Principles:     3 / 100
  Skills:         1
  Session count:  4 / 10 (next consolidation at 10)
  Store size:     4820 bytes
  Backend:        claude (auto-detected)
```

### 4. Consolidate when prompted

After enough sessions (default: 10) or episodes (default: 100), `mem` recommends consolidation:

```bash
mem consolidate
```

This groups similar episodes into principles, creates skill files for repeated procedures, and cleans up duplicates.

### 5. Inject context into new sessions

```bash
mem inject
```

Outputs relevant memory (principles, recent events, matching skills) for the agent to read at session start.

## Commands

### `mem init`

Creates the `.memory/` directory with all required files.

```bash
mem init [--path <dir>]
```

### `mem extract`

Captures significant events from the last session. Invokes the configured backend to analyze the git diff and identify decisions, errors, patterns, insights, and rollbacks.

```bash
mem extract [--session <id>] [--model <model>] [--backend <name>] [--dry-run]
```

| Flag | Default | Description |
|------|---------|-------------|
| `--session` | git short hash | Session identifier |
| `--model` | `haiku` | LLM model for analysis |
| `--backend` | auto-detect | Backend: `claude`, `opencode`, `codex`, `custom` |
| `--dry-run` | `false` | Print episodes without writing |

### `mem consolidate`

Analyzes accumulated episodes, extracts principles, detects skill candidates, and cleans up the store.

```bash
mem consolidate [--model <model>] [--backend <name>] [--dry-run] [--force]
```

| Flag | Default | Description |
|------|---------|-------------|
| `--model` | `sonnet` | LLM model for analysis |
| `--backend` | auto-detect | Backend override |
| `--dry-run` | `false` | Show changes without applying |
| `--force` | `false` | Run even if thresholds not reached |

Exits with code 3 if thresholds are not met (use `--force` to override).

### `mem inject`

Assembles relevant memory context for a new session. No LLM call — pure file assembly.

```bash
mem inject [--episodes <n>] [--format <fmt>]
```

| Flag | Default | Description |
|------|---------|-------------|
| `--episodes` | `10` | Number of recent episodes |
| `--format` | `markdown` | Output format: `markdown` or `json` |

Skills are matched against recent episode tags. If no skills match, all skills are listed.

### `mem status`

Shows memory store statistics, including detected backend.

```bash
mem status [--json] [--backend <name>]
```

All commands accept `--path <dir>` to override the default `.memory` location.

## Multi-Backend Support

`mem` works with multiple AI coding agents out of the box:

| Backend | Binary | Invocation Pattern | Model Flag |
|---------|--------|--------------------|------------|
| Claude Code | `claude` | `claude -p "<prompt>" --model <model>` | Supported |
| OpenCode | `opencode` | `opencode run "<prompt>"` | Not passed (uses provider default) |
| Codex | `codex` | `codex exec "<prompt>" -m <model>` | Supported |

### Backend Selection

1. **Auto-detect** (default): checks which CLI is installed in order: `claude` > `opencode` > `codex`
2. **Environment variable**: `export MEM_BACKEND=opencode`
3. **Per-command flag**: `mem extract --backend codex`

Priority: `--backend` flag > `MEM_BACKEND` env > auto-detect.

### Custom Backend

Use any CLI tool that accepts a prompt and returns text:

```bash
export MEM_BACKEND=custom
export MEM_BACKEND_BINARY=/path/to/my-agent
export MEM_BACKEND_ARGS="-p {prompt} --model {model}"
mem extract
```

`{prompt}` and `{model}` are replaced with actual values. If `{model}` is absent from the template, the `--model` flag is silently ignored.

## Configuration

All settings via environment variables:

| Variable | Default | Description |
|----------|---------|-------------|
| `MEM_BACKEND` | (auto-detect) | Backend: `claude`, `opencode`, `codex`, `custom` |
| `MEM_BACKEND_BINARY` | | Binary path for custom backend |
| `MEM_BACKEND_ARGS` | `{prompt}` | Argument template for custom backend |
| `MEM_PATH` | `.memory` | Memory store directory |
| `MEM_SESSION_THRESHOLD` | `10` | Sessions before consolidation is recommended |
| `MEM_EPISODE_THRESHOLD` | `100` | Episodes before consolidation is recommended |
| `MEM_PRINCIPLES_MAX` | `100` | Maximum number of principles |
| `MEM_EPISODES_MAX` | `200` | Maximum stored episodes |
| `MEM_EPISODES_KEEP` | `50` | Recent episodes protected from cleanup |
| `MEM_AGENT_ID` | `hostname-PID` | Agent identifier for multi-agent setups |

## How It Works

### Extraction (after each session)

1. Reads the latest `git diff`
2. Reads the last 20 episodes and current principles for context
3. Sends everything to the configured backend with a prompt asking to identify significant events
4. Deduplicates against existing episodes (exact match on type + summary)
5. Appends new episodes to `episodes.jsonl` under file lock
6. Increments the session counter and checks consolidation thresholds

### Consolidation (periodic)

1. Reads all episodes, principles, and skill list
2. Sends to the configured backend for analysis and synthesis
3. Merges new principles, deduplicates, enforces the 100-principle limit
4. Removes flagged episodes, enforces the 200-episode limit (newest 50 protected)
5. Creates skill files for procedures detected 3+ times
6. Flags skills older than 6 months for review
7. Detects conflicting decisions between different agents
8. Writes a consolidation log entry and resets the session counter

### Injection (before each session)

1. Reads all principles
2. Reads the N most recent episodes
3. Loads all skills, matches against recent episode tags
4. Outputs formatted context (Markdown or JSON) to stdout

No LLM call — injection is a local file operation.

## Multi-Agent Support

When multiple agents work on the same project concurrently:

- Each agent is identified by `MEM_AGENT_ID` (or auto-generated `hostname-PID`)
- File locking (`flock`) prevents concurrent write corruption
- Consolidation detects conflicting decisions from different agents and flags them for review

```bash
# Agent A
MEM_AGENT_ID=agent-a mem extract

# Agent B
MEM_AGENT_ID=agent-b mem extract
```

## Customizing Prompts

The LLM prompts used for extraction and consolidation are stored in `.memory/prompts/`. Edit them to tune what counts as a "significant event" or how principles are extracted:

- `.memory/prompts/extract.md` — extraction prompt
- `.memory/prompts/consolidate.md` — consolidation prompt

If deleted, the built-in defaults are used.

## File Formats

### episodes.jsonl

One JSON object per line:

```json
{"ts":"2026-03-20T14:32:00Z","session":"abc123","type":"decision","summary":"Chose JSONL over SQLite for event storage","tags":["architecture","storage"],"agent_id":"dev-laptop-1234"}
```

**Event types:** `decision`, `error`, `pattern`, `insight`, `rollback`

### principles.md

```markdown
# Project Principles

## Architecture
- Use JSONL for append-only logs — simpler git diffs, no driver dependency
- Memory files must stay under 150 lines

## Testing
- Always use file locks for concurrent writes — prevents race conditions
```

### skills/{slug}.md

```markdown
# Database Migration

## When to apply
- Need to change DB schema

## Prerequisites
- Database access
- Backup of current schema

## Steps
1. Create migration file
2. Write SQL (Up and Down)
3. Apply migration
4. Verify status
5. Regenerate models

## Success verification
- Migration status shows Applied
- Tests pass

## Anti-patterns
- Do not edit generated model files
```

## Development

```bash
go test -race -shuffle=on ./...    # 87 tests
go build -o mem ./cmd/mem          # build
go vet ./...                       # lint
```

Zero external dependencies — stdlib only.

## License

MIT
