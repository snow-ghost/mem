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

# Optional: semantic search with an OpenAI-compatible embeddings API
export MEM_EMBEDDINGS_URL=https://api.openai.com/v1/embeddings
export MEM_EMBEDDINGS_MODEL=text-embedding-3-small
export MEM_EMBEDDINGS_API_KEY=sk-...
# `mem mine` now auto-embeds new drawers; `mem reindex` covers older ones.
mem mine ~/projects/myapp --wing myapp        # auto-embeds new drawers
mem reindex                                    # one-shot for older drawers
mem search "auth decision" --mode hybrid       # BM25 + cosine via RRF
# (use --no-embed on `mem mine` to skip the embedding step)

# Optional: cross-encoder reranking on top of hybrid for stronger top-1
export MEM_RERANK_URL=https://your-endpoint/v1/rerank
export MEM_RERANK_MODEL=BAAI/bge-reranker-v2-m3

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

No vector embeddings required for the default mode — **everything works offline**.

#### Optional: semantic embeddings (hybrid search)

Set `MEM_EMBEDDINGS_URL` + `MEM_EMBEDDINGS_MODEL` (+ `MEM_EMBEDDINGS_API_KEY`)
pointing at any OpenAI-compatible `/v1/embeddings` endpoint — OpenAI, Voyage AI,
Cohere (compat mode), Together, Ollama, LM Studio, LocalAI, llama.cpp server.
Once set, `mem mine` automatically embeds new drawers as it ingests them
(opt out with `--no-embed`). `mem reindex` covers any older drawers that
predate the embeddings provider. `mem search --mode hybrid` fuses BM25 +
cosine similarity via weighted Reciprocal Rank Fusion (k=60). Pure vector
search (`--mode vector`) is also available. Embeddings are stored as BLOBs
in the same SQLite file — no second database. The entire feature is
optional; unset vars = BM25-only behavior unchanged.

For stronger top-1 results, also set `MEM_RERANK_URL` + `MEM_RERANK_MODEL`
(Cohere-compatible `/v1/rerank` endpoint, e.g. `BAAI/bge-reranker-v2-m3`).
The MCP `mem_search` tool accepts a `mode` argument that selects between
bm25, vector, and hybrid retrieval at call time.

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

| Metric | BM25 + stemming | Hybrid (RRF 0.7) | Hybrid + rerank |
|---|---:|---:|---:|
| **Recall@1** | 44.4% | 48.0% | **52.6%** |
| **Recall@5** | 71.0% | **74.6%** | 74.6% |
| **Recall@10** | 77.2% | 79.2% | **80.8%** |
| Avg query latency | 8.6 ms | 67 ms | 137 ms |

Tokenizer applies Porter step 1a/1b stemming — `+1.6 R@5` on BM25 alone
without any external dependency. Hybrid mode adds another `+3.6 R@5` via
weighted Reciprocal Rank Fusion (0.7 BM25 / 0.3 vector). Cross-encoder
reranking (`BAAI/bge-reranker-v2-m3`) on top of hybrid adds **+4.6 R@1**
and **+1.6 R@10** — useful for top-1-driven workflows like LLM
retrieval-augmented answering.

### LoCoMo (Snap Research) — 10 long-form conversations, 1986 QAs

| Metric | BM25 (offline) | Hybrid (BM25 + bge-m3) |
|---|---:|---:|
| Recall@1 | **60.0%** | 59.0% |
| **Recall@5** | 88.2% | **88.6%** |
| Recall@10 | 93.7% | **95.6%** |
| Avg query latency | 1.7 ms | 4.5 ms |

Hybrid's win is concentrated where it matters most: **multi-hop +7.3 pp**
(hardest category, 59.4 → 66.7) and **single-hop +5.7 pp** (80.5 → 86.2).
R@10 jumps +1.9 pp — embeddings rescue evidence that fell out of BM25 top-5.

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
  search/              BM25 + vector + hybrid (RRF) search, Porter stemmer
  embeddings/          Optional OpenAI-compatible client + blob serializer
  rerank/              Optional Cohere-compatible cross-encoder client
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
