# Quickstart: Agent Memory Companion

## Prerequisites

- Go 1.26.0+
- Claude Code CLI (`claude`) installed and authenticated
- Git repository (for session diff analysis)

## Build

```bash
go build -o mem ./cmd/mem
```

## Initialize Memory Store

```bash
cd /path/to/your/project
mem init
```

This creates the `.memory/` directory with all required files.

## Manual Usage

### After a work session — extract events

```bash
mem extract
```

The tool reads the latest git diff, invokes Claude (Haiku) to
identify significant events, and appends them to the episode log.

### Check memory status

```bash
mem status
```

### Run consolidation

```bash
mem consolidate
```

Groups episodes, extracts principles, detects skills, cleans up
duplicates. Uses Claude (Sonnet) for analysis.

### Before a new session — inject context

```bash
mem inject
```

Outputs formatted memory context (principles, recent episodes,
relevant skills) to stdout.

## Automatic Usage (Claude Code Hooks)

### Post-session extraction hook

Add to your Claude Code `settings.json`:

```json
{
  "hooks": {
    "PostToolUse": [
      {
        "matcher": "Stop",
        "command": "/path/to/mem extract"
      }
    ]
  }
}
```

### Verify it works

1. Run a Claude Code session that makes some decisions or fixes
   a bug.
2. After the session ends, check:
   ```bash
   mem status
   ```
3. You should see 1-5 new episodes.

## Validation Checklist

- [ ] `mem init` creates `.memory/` with correct structure
- [ ] `mem extract` produces 1-5 episodes after a real session
- [ ] `mem status` shows accurate counts
- [ ] `mem consolidate --force` runs without error
- [ ] `mem inject` outputs formatted memory context
- [ ] File locking prevents corruption under concurrent writes
