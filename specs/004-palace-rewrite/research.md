# Research: Palace Architecture Rewrite

**Date**: 2026-04-07

## R1: Pure-Go SQLite Driver

**Decision**: `modernc.org/sqlite` — C-to-Go transpiled SQLite.

**Rationale**: No CGo required. Produces static binary. Battle-
tested (used by Gogs, Woodpecker CI, many others). Compatible
with `database/sql` interface.

**Alternatives**:
- `ncruces/go-sqlite3`: WASM-based, newer, fast. But larger
  binary and less mature ecosystem.
- `zombiezen.com/go/sqlite`: Wraps modernc, custom API. Faster
  but non-standard interface.
- `mattn/go-sqlite3`: CGo-based. Requires C toolchain, breaks
  static binary goal.

Source: [modernc.org/sqlite](https://pkg.go.dev/modernc.org/sqlite)

---

## R2: BM25 Search Implementation

**Decision**: Implement BM25 Okapi variant from scratch (~200
lines), using SQLite-backed inverted index.

**Rationale**: BM25 is the standard text retrieval algorithm
(used by Elasticsearch, Lucene). Simple to implement: tokenize
documents, build inverted index, score queries. The palace
structure (wing+room filtering) provides the retrieval boost —
mempalace showed +34% from filtering alone, regardless of
embedding quality.

**BM25 Formula**:
```
score(D,Q) = Σ IDF(qi) * (f(qi,D) * (k1+1)) / (f(qi,D) + k1 * (1 - b + b * |D|/avgDL))
```
Where k1=1.5, b=0.75 (standard parameters).

**Index structure** (SQLite tables):
- `search_terms`: unique terms with IDs
- `search_index`: (term_id, drawer_id, tf) — term frequency per doc
- `search_meta`: corpus stats (avg doc length, total docs)

**Why not vector embeddings**:
- Embeddings require a model (~100MB+ for sentence-transformers)
- Or external API calls (defeats "offline" goal)
- BM25 + palace filtering = comparable recall for text retrieval

Sources: [BM25 Go libs](https://pkg.go.dev/github.com/crawlab-team/bm25),
[Understanding BM25](https://news.ycombinator.com/item?id=42190650)

---

## R3: MCP Server SDK

**Decision**: `github.com/modelcontextprotocol/go-sdk` — official
MCP SDK maintained by Anthropic + Google.

**Rationale**: The official SDK follows the latest MCP spec.
Maintained by the protocol authors. Clean Go API. Handles
transport, serialization, tool registration.

**Usage pattern**:
```go
server := mcp.NewServer("mem", "0.2.0")
server.AddTool(mcp.Tool{Name: "mem_search", ...}, searchHandler)
server.AddTool(mcp.Tool{Name: "mem_kg_query", ...}, kgHandler)
transport := stdio.NewTransport()
server.Run(ctx, transport)
```

**Alternatives**:
- `mark3labs/mcp-go`: Community SDK, excellent but not official.
- Roll own: 1000+ lines for JSON-RPC transport + tool protocol.

Source: [Official MCP Go SDK](https://github.com/modelcontextprotocol/go-sdk)

---

## R4: Room Auto-Detection

**Decision**: Keyword extraction via TF-IDF + simple clustering.

**Algorithm**:
1. Tokenize each document (lowercase, strip stopwords)
2. Compute TF-IDF for each term per document
3. Extract top 3-5 keywords per document
4. Group documents by shared top keywords
5. Name each group (room) by its most frequent keyword combo

Example: Documents about "auth", "OAuth", "login" → room
"auth-login". Documents about "deploy", "CI", "pipeline" →
room "ci-deploy".

**No LLM needed**: Pure statistical text analysis. Works offline.
Less precise than LLM classification, but good enough for
organizing — and users can rename rooms manually.

---

## R5: AAAK-Like Compression

**Decision**: Structured abbreviation format for L1 context.

**Format**:
```
TEAM: KAI(backend,3yr) | SOR(frontend) | MAY(infra)
PROJ: DRIFTWOOD(saas.analytics) | SPRINT: auth→clerk
DECISION: KAI.rec:clerk>auth0(pricing+dx) | ★★★★
```

**Implementation**: Template-based compression. Extract key
entities from knowledge graph + top drawers. Format using
abbreviation rules:
- Names → 3-letter codes
- Relationships → compact notation
- Recent events → one-line summaries

Output: ~120 tokens covering all critical facts. Any LLM reads
it natively (it's just structured text).

---

## R6: Conversation Format Detection

**Decision**: Support 4 formats, auto-detect by file extension
and content structure:

| Format | Extension | Detection |
|--------|-----------|-----------|
| Claude | `.jsonl` | Each line is JSON with `role` field |
| ChatGPT | `.json` | Array with `mapping` or `messages` key |
| Slack | `.json` | Has `channels` or `messages` array with `user`+`ts` |
| Plain text | `.txt/.md` | Fallback — split by `Human:`/`Assistant:` patterns |

Each format is parsed into a common `Exchange` struct:
`{Speaker, Content, Timestamp}`.
