# mem — Standalone Memory Palace for AI Agents

A **pure Go** memory palace system inspired by [MemPalace](https://github.com/milla-jovovich/mempalace),
rewritten from scratch with zero Python dependencies. Single static binary, single SQLite file,
no LLM required for core features.

## What it is

`mem` organizes knowledge into a navigable palace structure:

```
Wing (person/project) → Hall (facts/events/...) → Room (topic) → Drawer (verbatim content)
```

Features:

- **Palace structure** — wings, halls, rooms, drawers, tunnels (cross-wing links)
- **BM25 semantic search** — pure Go implementation, no embeddings needed
- **Temporal knowledge graph** — entity-relationship triples with validity windows + contradiction detection
- **4-layer memory stack** — L0 Identity → L1 Critical Facts → L2 On-demand → L3 Deep Search
- **Wake-up context** — compact (~120 token) AAAK-like compression for AI session starts
- **MCP server** — 8 tools for Claude Code / ChatGPT / Cursor integration
- **Mining** — files (code, docs) and conversations (Claude, ChatGPT, Slack, plain text)
- **Auto-init** — any command bootstraps the palace on first use
- **Zero LLM dependency** for core features (everything works offline)

## Install

### Pre-built binaries

Download from [Releases](https://github.com/snow-ghost/mem/releases/latest):

```bash
curl -fsSL https://github.com/snow-ghost/mem/releases/latest/download/mem-linux-amd64.tar.gz | tar xz
sudo mv mem-linux-amd64 /usr/local/bin/mem
```

### From source

```bash
go install github.com/snow-ghost/mem/cmd/mem@latest
```

### Docker

```bash
docker run --rm -v $(pwd):/project ghcr.io/snow-ghost/mem status
```

Requires Go 1.26+ only for building. At runtime, **zero runtime dependencies**.

## Quick Start

```bash
# Initialize the palace (auto-creates ~/.mempalace/palace.db)
mem init

# Mine a project into the palace
mem mine ~/projects/myapp --wing myapp

# Mine conversation exports
mem mine ~/chats --mode convos --wing conversations

# Search across all memories
mem search "why did we switch to GraphQL"

# Filter by wing and room
mem search "auth decision" --wing myapp --room auth

# Compact context for AI session start
mem wake-up

# Knowledge graph operations
mem kg add Kai works_on Orion --from 2025-06-01
mem kg query Kai
mem kg timeline Orion
mem kg invalidate Kai works_on Orion --ended 2026-03-01

# Status overview
mem status

# Start MCP server (for Claude Code integration)
mem mcp
```

## How it works

### Storage

Everything lives in a **single SQLite database** at `~/.mempalace/palace.db`
(override with `MEM_PALACE` env var). Schema includes:

- `wings`, `rooms`, `drawers`, `closets` — palace hierarchy
- `search_terms`, `search_index`, `search_meta` — BM25 inverted index
- `entities`, `triples` — temporal knowledge graph

### Search

Built-in **BM25 Okapi** implementation with our own inverted index:

- Tokenization with stopword removal
- TF computation with batch indexing (transactional)
- Classic BM25 scoring (k1=1.5, b=0.75)
- Filter by wing / room before scoring (for palace structure boost)

No vector embeddings, no external services, no LLM calls.

### Knowledge Graph

Entity-relationship triples with temporal validity:

- `add_triple(subject, predicate, object, valid_from, valid_to)`
- `invalidate` facts when they stop being true
- `query_entity` with `as_of` date filtering
- `timeline` for chronological entity story
- Contradiction detection — flags conflicts when adding facts

### 4-Layer Memory Stack

- **L0 (Identity)** — read from `~/.mempalace/identity.txt` if present
- **L1 (Critical Facts)** — auto-compressed AAAK-like summary from top drawers
- **L2 (On-demand)** — filtered retrieval by wing/room
- **L3 (Deep Search)** — full BM25 search across palace

`mem wake-up` outputs L0+L1 (~120-170 tokens) for AI session bootstrap.

## MCP Integration

Register as an MCP server for Claude Code / ChatGPT / Cursor:

```bash
# Claude Code
claude mcp add mem -- mem mcp

# Available tools:
#   mem_search       — BM25 search with wing/room filters
#   mem_add_drawer   — Store content in the palace
#   mem_status       — Palace overview
#   mem_wake_up      — Compact context for AI
#   mem_kg_query     — Query the knowledge graph
#   mem_kg_add       — Add fact to the graph
#   mem_list_wings   — Enumerate wings
#   mem_list_rooms   — Enumerate rooms in a wing
```

## Benchmarks

Evaluated on three public memory benchmarks. See [`benchmarks/README.md`](benchmarks/README.md)
for reproduction steps.

### LongMemEval (ICLR 2025) — 500 questions, 6 question types

| Metric | Value |
|---|---|
| **Recall@5** | **69.4%** |
| Recall@10 | 78.4% |
| Full run | ~31s |
| Avg query latency | 7.1 ms |

Matches the published BM25 baseline (~70%), confirming correct implementation.
Semantic-embedding systems (MemPalace at 96.6%, Mem0/Zep at ~85%) need a model
in the loop; `mem` achieves this with pure BM25.

### LoCoMo (Snap Research) — 10 long-form conversations, 1986 QAs

| Metric | Value |
|---|---|
| Recall@1 | 60.0% |
| **Recall@5** | **88.2%** |
| Recall@10 | 93.7% |
| Full run | ~11s |

Per-category R@5: open-domain 92.7%, temporal 85.0%, single-hop 80.5%,
multi-hop 59.4%. Non-adversarial R@5: 86.8%.

### ConvoMem (Salesforce, arXiv 2511.10523) — 7,021 test cases, sizes 1–6

| Metric | Value |
|---|---|
| Recall@1 | **100.0%** |
| Recall@5 | **100.0%** |
| Avg query latency | 1.4 ms |

Confirms the ConvoMem paper's thesis: *"your first 150 conversations don't
need RAG"*. BM25 alone is sufficient at small haystacks. The harder regime
(50–300 conversations with value-change tracking) is left as future work.

## Architecture

```
cmd/mem/               CLI entry
internal/
  config/              Configuration (env vars, paths)
  db/                  SQLite schema + connection
  palace/              Wings, rooms, drawers, tunnels
  search/              BM25 tokenizer + indexer + search
  kg/                  Temporal knowledge graph + contradiction detection
  layers/              4-layer memory stack (L0 identity, L1 compression, wake-up)
  miner/               File and conversation mining (Claude JSONL, ChatGPT, Slack, plain text)
  mcp/                 MCP server with 8 tools
benchmarks/
  longmemeval/         LongMemEval harness
  locomo/              LoCoMo harness
  convomem/            ConvoMem harness
```

## Dependencies

**Only 2 external dependencies** (pure Go, no CGo):

- `modernc.org/sqlite` — pure-Go SQLite driver (no CGo = static binary)
- `github.com/modelcontextprotocol/go-sdk` — official MCP SDK

Everything else is Go stdlib.

## Previous code (LLM-dependent memory companion)

The previous version of `mem` (LLM-dependent extraction/consolidation with Claude/OpenCode/Codex backends)
lives at [github.com/snow-ghost/mem-agent](https://github.com/snow-ghost/mem-agent).

## License

MIT
