# CLI Interface Contract: mem

**Binary**: `mem`
**Invocation**: `mem <command> [flags]`

## Commands

### `mem init`

Initialize the `.memory/` directory structure in the current project.

```
mem init [--path <dir>]
```

| Flag    | Type   | Default | Description |
|---------|--------|---------|-------------|
| --path  | string | .       | Project root directory |

**Output** (stdout):
```
Initialized memory store at /path/to/project/.memory/
Created: episodes.jsonl, principles.md, skills/,
         consolidation-log.md, prompts/
```

**Exit codes**: 0 = success, 1 = already initialized, 2 = error

---

### `mem extract`

Run post-session extraction. Analyzes the last session's changes,
invokes Claude CLI to identify significant events, and appends
episodes to the store.

```
mem extract [--session <id>] [--model <model>] [--dry-run]
```

| Flag      | Type   | Default | Description |
|-----------|--------|---------|-------------|
| --session | string | auto    | Session ID. Auto-generates from git short hash if omitted |
| --model   | string | haiku   | Claude model for extraction |
| --dry-run | bool   | false   | Print extracted episodes without writing |

**Output** (stdout):
```
Extracted 3 episodes from session abc1234
  [decision] Chose JSONL over SQLite — simpler git diffs
  [error]    Race condition in concurrent file writes
  [pattern]  Third manual DB migration this week
Session count: 7/10 (consolidation threshold)
```

**Exit codes**: 0 = success (episodes written or none found),
1 = error

**Side effects**:
- Appends to `.memory/episodes.jsonl`
- May update `.memory/principles.md` (if pattern detected)
- Increments `.memory/.session-count`
- If session count or episode count exceeds threshold, prints
  consolidation recommendation

---

### `mem consolidate`

Run periodic consolidation. Groups episodes, extracts principles,
detects skills, and cleans up duplicates/outdated records.

```
mem consolidate [--model <model>] [--dry-run] [--force]
```

| Flag      | Type   | Default | Description |
|-----------|--------|---------|-------------|
| --model   | string | sonnet  | Claude model for consolidation |
| --dry-run | bool   | false   | Show proposed changes without applying |
| --force   | bool   | false   | Run even if thresholds not reached |

**Output** (stdout):
```
Consolidation #3 — 2026-03-20
  Episodes processed: 45
  Principles added: 2 (architecture, testing)
  Principles updated: 1 (migrations)
  Principles removed: 0
  Episodes removed: 8 (5 duplicates, 3 superseded)
  Skills created: 1 (database-migration)
  Skill candidates: "deploy-to-staging" (2 occurrences)
```

**Exit codes**: 0 = success, 1 = error, 3 = skipped (thresholds
not reached, use --force to override)

**Side effects**:
- Rewrites `.memory/episodes.jsonl` (removes duplicates/old)
- Updates `.memory/principles.md`
- May create files in `.memory/skills/`
- Appends to `.memory/consolidation-log.md`
- Resets `.memory/.session-count` to 0

---

### `mem inject`

Assemble relevant memory context for a new agent session. Outputs
formatted context to stdout.

```
mem inject [--episodes <n>] [--format <fmt>]
```

| Flag       | Type   | Default  | Description |
|------------|--------|----------|-------------|
| --episodes | int    | 10       | Number of recent episodes to include |
| --format   | string | markdown | Output format: `markdown` or `json` |

**Output** (stdout, markdown format):
```markdown
# Project Memory

## Principles
- Use JSONL for append-only logs — git-diffable
- Always add file lock for concurrent writes

## Recent Events
- [2026-03-20] [decision] Chose JSONL over SQLite
- [2026-03-19] [error] Race condition in concurrent writes

## Relevant Skills
(included when skill triggers match recent context)
```

**Exit codes**: 0 = success (may output empty if no memory),
1 = error

**Side effects**: None (read-only)

---

### `mem status`

Display current memory store statistics.

```
mem status [--json]
```

| Flag   | Type | Default | Description |
|--------|------|---------|-------------|
| --json | bool | false   | Output as JSON instead of human-readable |

**Output** (stdout, human-readable):
```
Memory Store: /path/to/project/.memory/
  Episodes:       47 / 200
  Principles:     12 / 100
  Skills:         3
  Session count:  7 / 10 (next consolidation at 10)
  Last consolidation: 2026-03-18
  Store size:     24 KB
```

**Output** (stdout, JSON):
```json
{
  "path": "/path/to/project/.memory/",
  "episodes": {"count": 47, "max": 200},
  "principles": {"count": 12, "max": 100},
  "skills": 3,
  "session_count": {"current": 7, "threshold": 10},
  "last_consolidation": "2026-03-18",
  "store_size_bytes": 24576
}
```

**Exit codes**: 0 = success, 1 = not initialized, 2 = error

**Side effects**: None (read-only)

## Environment Variables

| Variable                | Default | Description |
|-------------------------|---------|-------------|
| MEM_PATH                | .memory | Path to memory store directory |
| MEM_SESSION_THRESHOLD   | 10      | Sessions before auto-consolidation trigger |
| MEM_EPISODE_THRESHOLD   | 100     | Episodes before auto-consolidation trigger |
| MEM_PRINCIPLES_MAX      | 100     | Maximum principles allowed |
| MEM_EPISODES_MAX        | 200     | Maximum episodes allowed |
| MEM_EPISODES_KEEP       | 50      | Recent episodes protected from cleanup |

## Error Output

All errors go to stderr. Format:
```
mem: <command>: <message>
```

Example:
```
mem: extract: memory store not initialized — run "mem init" first
mem: consolidate: failed to acquire lock — another process holds it
```
