# Implementation Plan: Palace Architecture Rewrite

**Branch**: `004-palace-rewrite` | **Date**: 2026-04-07 | **Spec**: [spec.md](spec.md)

## Summary

Rewrite `mem` as a standalone memory palace system. No LLM
dependency for core features. Palace structure (Wing→Hall→Room→
Closet→Drawer) for organized, searchable memory. Knowledge graph
with temporal validity. Built-in BM25 search. MCP server. Single
static binary. Pure Go, minimal dependencies.

## Technical Context

**Language/Version**: Go 1.26.0
**Dependencies** (minimal):
- `modernc.org/sqlite` — pure-Go SQLite (no CGo, static binary)
- `github.com/modelcontextprotocol/go-sdk` — official MCP SDK
- Go stdlib for everything else
**Storage**: Single SQLite database (`~/.mempalace/palace.db`)
**Testing**: `go test -race -shuffle=on ./...` — BDD mandatory
**Target Platform**: Linux/macOS/Windows (pure Go, no CGo)
**Project Type**: CLI tool + MCP server
**Performance Goals**: Search <100ms at 10K drawers. Mining 1K
files <30s. Wake-up <10ms.
**Constraints**: Single static binary <15 MB. Zero internet
dependency after install. No LLM required for any core feature.

## Constitution Check

| Principle | Applicable? | Status | Notes |
|-----------|-------------|--------|-------|
| I. Code Quality — clean architecture | Yes | PASS | Clean package separation by domain |
| I. Code Quality — error wrapping | Yes | PASS | `fmt.Errorf("op: %w", err)` |
| II. Testing — BDD, race detection | Yes | PASS | BDD table-driven tests |
| Arch — minimal deps | Yes | PASS | 2 deps only: sqlite + mcp-sdk |
| Dev Workflow — test gates | Yes | PASS | `go test -race -shuffle=on` |

## Project Structure

```text
cmd/
└── mem/
    └── main.go                  # CLI entry + subcommand dispatch

internal/
├── palace/
│   ├── palace.go                # Palace struct, Init, path helpers
│   ├── palace_test.go
│   ├── wing.go                  # Wing CRUD
│   ├── room.go                  # Room auto-detection + CRUD
│   ├── drawer.go                # Drawer storage (verbatim content)
│   ├── drawer_test.go
│   ├── closet.go                # Compressed summaries
│   └── tunnel.go                # Cross-wing connections
├── search/
│   ├── bm25.go                  # BM25 scoring engine
│   ├── bm25_test.go
│   ├── tokenizer.go             # Text tokenization + stemming
│   ├── tokenizer_test.go
│   ├── index.go                 # Inverted index (SQLite-backed)
│   └── search.go                # Search orchestrator (filter+rank)
├── kg/
│   ├── graph.go                 # Knowledge graph (entities+triples)
│   ├── graph_test.go
│   ├── contradiction.go         # Contradiction detection
│   └── contradiction_test.go
├── layers/
│   ├── stack.go                 # 4-layer memory stack
│   ├── stack_test.go
│   ├── identity.go              # L0: identity.txt
│   ├── compress.go              # AAAK-like compression for L1
│   └── compress_test.go
├── miner/
│   ├── miner.go                 # File mining orchestrator
│   ├── miner_test.go
│   ├── chunker.go               # Text chunking
│   ├── convo.go                 # Conversation format parser
│   ├── convo_test.go
│   ├── room_detector.go         # Auto-detect rooms from content
│   └── dedup.go                 # Content-hash dedup
├── mcp/
│   ├── server.go                # MCP server (tools registration)
│   └── server_test.go
├── config/
│   ├── config.go                # Configuration
│   └── config_test.go
└── db/
    ├── schema.go                # SQLite schema + migrations
    └── db.go                    # Database connection helpers

Dockerfile
```

**Structure Decision**: Domain-driven flat packages. Each package
owns one concern: `palace` (structure), `search` (retrieval),
`kg` (knowledge graph), `layers` (memory stack), `miner` (data
ingestion), `mcp` (protocol server). The `db` package provides
shared database access.

## Key Design Decisions

### SQLite as Single Storage Backend

Everything in one file: drawers (verbatim content), closets
(summaries), search index (inverted index for BM25), knowledge
graph (entities + triples), metadata (wings, rooms, halls). Single
`palace.db` file = easy backup, sync, transfer.

Using `modernc.org/sqlite` for pure-Go, no-CGo static linking.

### BM25 Built-In Search

Implement BM25 (Okapi variant) using SQLite-backed inverted index.
~200 lines of Go. No external search engine.

**Algorithm**: For each document, store tokenized terms + TF in
SQLite. On query, compute IDF from term frequency across corpus,
then score each matching document with BM25 formula. Wing/room
filtering applied as SQL WHERE clause before scoring.

### Room Auto-Detection

Extract keywords from content using TF-IDF on the document.
Group documents by keyword similarity into rooms. Room names
derived from the top keywords of each cluster. No LLM needed.

### MCP Server

Use official `github.com/modelcontextprotocol/go-sdk`. Register
tools: `mem_search`, `mem_add_drawer`, `mem_kg_query`,
`mem_kg_add`, `mem_status`, `mem_wake_up`, `mem_list_wings`,
`mem_list_rooms`, `mem_traverse`. Runs as `mem mcp`.

## SQLite Schema

```sql
-- Palace structure
CREATE TABLE wings (
    id INTEGER PRIMARY KEY,
    name TEXT UNIQUE NOT NULL,
    type TEXT DEFAULT 'general',  -- person/project/general
    keywords TEXT DEFAULT '',
    created_at TEXT DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE rooms (
    id INTEGER PRIMARY KEY,
    name TEXT NOT NULL,           -- slug: "auth-migration"
    wing_id INTEGER REFERENCES wings(id),
    created_at TEXT DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(name, wing_id)
);

CREATE TABLE drawers (
    id INTEGER PRIMARY KEY,
    content TEXT NOT NULL,         -- verbatim text
    content_hash TEXT NOT NULL,    -- SHA256 for dedup
    wing_id INTEGER REFERENCES wings(id),
    room_id INTEGER REFERENCES rooms(id),
    hall TEXT DEFAULT 'facts',     -- facts/events/discoveries/preferences/advice
    source_file TEXT,
    source_type TEXT,              -- file/conversation/manual
    created_at TEXT DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(content_hash)
);

CREATE TABLE closets (
    id INTEGER PRIMARY KEY,
    room_id INTEGER REFERENCES rooms(id),
    compressed_text TEXT NOT NULL,
    token_count INTEGER DEFAULT 0,
    updated_at TEXT DEFAULT CURRENT_TIMESTAMP
);

-- Search index (BM25)
CREATE TABLE search_terms (
    id INTEGER PRIMARY KEY,
    term TEXT UNIQUE NOT NULL
);

CREATE TABLE search_index (
    term_id INTEGER REFERENCES search_terms(id),
    drawer_id INTEGER REFERENCES drawers(id),
    tf REAL NOT NULL,              -- term frequency in document
    PRIMARY KEY (term_id, drawer_id)
);

CREATE TABLE search_meta (
    key TEXT PRIMARY KEY,
    value TEXT
);  -- stores avg_doc_len, total_docs for BM25

-- Knowledge graph
CREATE TABLE entities (
    id TEXT PRIMARY KEY,           -- normalized name
    name TEXT NOT NULL,
    type TEXT DEFAULT 'unknown',
    properties TEXT DEFAULT '{}',
    created_at TEXT DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE triples (
    id TEXT PRIMARY KEY,
    subject TEXT REFERENCES entities(id),
    predicate TEXT NOT NULL,
    object TEXT REFERENCES entities(id),
    valid_from TEXT,
    valid_to TEXT,
    confidence REAL DEFAULT 1.0,
    source_drawer_id INTEGER REFERENCES drawers(id),
    created_at TEXT DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_triples_subject ON triples(subject);
CREATE INDEX idx_triples_object ON triples(object);
CREATE INDEX idx_triples_valid ON triples(valid_from, valid_to);
CREATE INDEX idx_drawers_wing ON drawers(wing_id);
CREATE INDEX idx_drawers_room ON drawers(room_id);
CREATE INDEX idx_drawers_hash ON drawers(content_hash);
CREATE INDEX idx_search_term ON search_index(term_id);
```

## Complexity Tracking

| Deviation | Why | Alternative Rejected |
|-----------|-----|---------------------|
| `modernc.org/sqlite` dep | Need embedded SQL for structured storage + search index + knowledge graph | File-based (previous): doesn't scale to 10K drawers |
| `modelcontextprotocol/go-sdk` dep | MCP protocol is complex; SDK maintained by protocol authors | Roll-own: 1000+ lines of protocol code for an evolving spec |
| BM25 instead of vector embeddings | Zero-dependency search. Vectors need embedding model (~100MB+) | ChromaDB: Python-only, external process, huge dep |
