# Quickstart: Multi-Agent Backend Support

## Default behavior (no changes needed)

If you already use `mem` with Claude Code, nothing changes.
Claude is auto-detected and used as before.

## Using mem with OpenCode

```bash
# Option 1: set environment variable
export MEM_BACKEND=opencode
mem extract

# Option 2: per-command flag
mem extract --backend opencode

# Option 3: auto-detect (if opencode is installed and claude is not)
mem extract
```

## Using mem with Codex

```bash
export MEM_BACKEND=codex
mem extract
```

## Using a custom backend

```bash
export MEM_BACKEND=custom
export MEM_BACKEND_BINARY=/path/to/my-agent
export MEM_BACKEND_ARGS="-p {prompt} --model {model}"
mem extract
```

## Check which backend is active

```bash
mem status
```

The status output shows which backend is configured/detected.

## Validation Checklist

- [ ] `MEM_BACKEND=opencode mem extract` invokes opencode
- [ ] `MEM_BACKEND=codex mem extract` invokes codex
- [ ] `mem extract --backend opencode` works as override
- [ ] Without MEM_BACKEND set, auto-detection finds installed backend
- [ ] Missing backend binary shows clear error with suggestions
- [ ] `mem inject` works without any backend (no LLM call)
- [ ] Switching backends between sessions — episodes remain valid
