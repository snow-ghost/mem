# Quickstart: mem (Palace Architecture)

## Install

```bash
# Pre-built binary
curl -fsSL https://github.com/snow-ghost/mem/releases/latest/download/mem-linux-amd64.tar.gz | tar xz
sudo mv mem-linux-amd64 /usr/local/bin/mem

# Or via Go
go install github.com/snow-ghost/mem/cmd/mem@latest
```

## Set Up Your Palace

```bash
# Initialize with guided setup
mem init ~/projects/myapp

# Mine your project files
mem mine ~/projects/myapp

# Mine conversation exports
mem mine ~/chats/ --mode convos

# Mine with a specific wing name
mem mine ~/projects/api --wing api-backend
```

## Search Anything

```bash
# Search across everything
mem search "why did we switch to GraphQL"

# Filter by wing (project)
mem search "auth decision" --wing myapp

# Filter by room (topic)
mem search "database choice" --room data-layer
```

## Wake Up Your AI

```bash
# Get compact context (~170 tokens)
mem wake-up

# Project-specific context
mem wake-up --wing myapp

# Paste into your AI's system prompt, or pipe:
mem wake-up | pbcopy
```

## Knowledge Graph

```bash
# Add a fact
mem kg add "Kai" "works_on" "Orion" --from 2025-06-01

# Query an entity
mem kg query Kai

# Timeline
mem kg timeline Orion

# Invalidate a fact
mem kg invalidate "Kai" "works_on" "Orion" --ended 2026-03-01
```

## MCP Integration

```bash
# Register with Claude Code
claude mcp add mem -- mem mcp

# Now the AI has search, add, navigate tools automatically
```

## Check Status

```bash
mem status
```

## Validation Checklist

- [ ] `mem init` creates palace database
- [ ] `mem mine` indexes files into drawers
- [ ] `mem search` returns relevant results with wing/room info
- [ ] `mem wake-up` outputs compact context under 300 tokens
- [ ] `mem kg add/query` manages entity facts
- [ ] `mem mcp` starts MCP server with tools listed
- [ ] `mem status` shows palace overview
