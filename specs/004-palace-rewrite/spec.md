# Feature Specification: Palace Architecture Rewrite

**Feature Branch**: `004-palace-rewrite`
**Created**: 2026-04-07
**Status**: Draft
**Input**: Rewrite mem as a Go-based standalone memory palace — no LLM dependency, palace structure, knowledge graph, semantic search, MCP server

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Mine Project Files Into Palace (Priority: P1)

As a developer, I want to ingest my project files (code, docs,
notes, conversation exports) into a structured memory palace so
that everything I've ever worked on is organized and searchable.

I run `mem mine ~/projects/myapp` and the system reads all files,
detects topics (rooms), assigns them to the right wing, and stores
the verbatim content in drawers with compressed summaries in
closets. No LLM required — the system uses text analysis to
categorize automatically.

**Why this priority**: Without data ingestion, nothing else works.
Mining is the foundation that populates the palace.

**Independent Test**: Run `mem mine` on a project with 100 files.
Verify all files are indexed, rooms are auto-detected, and
`mem status` shows the correct drawer count.

**Acceptance Scenarios**:

1. **Given** a project directory with source code and docs,
   **When** `mem mine ~/project` is run, **Then** all text files
   are read, chunked, and stored as drawers with metadata (wing,
   hall, room, source file).
2. **Given** conversation exports (Claude, ChatGPT, Slack JSON),
   **When** `mem mine ~/chats --mode convos` is run, **Then**
   conversations are parsed, split by exchange, and each exchange
   becomes a drawer.
3. **Given** a wing name is specified, **When** `mem mine ~/project
   --wing myapp` is run, **Then** all drawers are tagged with
   wing "myapp".
4. **Given** the same file is mined twice, **When** checked,
   **Then** no duplicates are created (content-based dedup).

---

### User Story 2 - Search Memories (Priority: P2)

As a developer, I want to search across all my stored memories
using natural language queries so that I can find decisions,
discussions, and code patterns from months ago in seconds.

I run `mem search "why did we switch to GraphQL"` and get back
verbatim text from the original conversations, ranked by relevance,
showing which wing/room the result came from.

**Why this priority**: Search is the primary value proposition —
making stored knowledge findable. Without search, the palace is
just an archive.

**Independent Test**: Mine 1000 documents, run 10 diverse queries,
verify relevant results appear in the top 5 with correct wing/room
attribution.

**Acceptance Scenarios**:

1. **Given** a populated palace, **When** `mem search "query"` is
   run, **Then** the top N most relevant drawers are returned with
   verbatim content, similarity score, wing, room, and source file.
2. **Given** a wing filter, **When** `mem search "query" --wing
   myapp` is run, **Then** only results from wing "myapp" are
   returned.
3. **Given** a room filter, **When** `mem search "query" --room
   auth-migration` is run, **Then** results are scoped to that
   specific room.
4. **Given** palace structure (wing+room filtering), **When**
   compared to flat search (all drawers), **Then** retrieval
   accuracy improves by at least 20%.

---

### User Story 3 - Wake-Up Context for AI Sessions (Priority: P3)

As a developer starting a new AI coding session, I want to load
a compact context summary (~170 tokens) that tells the AI who I
am, what my projects are, and what happened recently so that the
AI has full context from token #1.

I run `mem wake-up` and get a compressed context block containing
my identity (L0), critical facts about my team and projects (L1),
ready to paste into a system prompt or inject via hook.

**Why this priority**: This is the "magic moment" — the AI
instantly knows your world. It's what makes the palace useful
every single day.

**Independent Test**: After mining 50 conversations, run
`mem wake-up`. Verify output is under 200 tokens and contains
key people, projects, and recent decisions.

**Acceptance Scenarios**:

1. **Given** a populated palace with identity configured, **When**
   `mem wake-up` is run, **Then** output contains L0 (identity,
   ~50 tokens) + L1 (critical facts, ~120 tokens).
2. **Given** a wing-specific wake-up, **When** `mem wake-up
   --wing myapp` is run, **Then** L1 facts are filtered to that
   project.
3. **Given** wake-up output, **When** token count is measured,
   **Then** total is under 300 tokens (compact enough for any
   context window).
4. **Given** compressed format is used, **When** the AI reads the
   wake-up text, **Then** it understands the content without
   any special decoder (readable by any LLM).

---

### User Story 4 - Knowledge Graph (Priority: P4)

As a developer, I want to track entities (people, projects, tools)
and their relationships over time so that I can ask "who works on
what" and "what changed about project X in January" and get
accurate, time-aware answers.

I add facts to the knowledge graph, and the system tracks when
each fact was true, detects contradictions, and answers temporal
queries.

**Why this priority**: The knowledge graph adds structured
reasoning on top of the unstructured palace. It's what enables
contradiction detection and temporal queries.

**Independent Test**: Add 20 entity facts with dates, query an
entity, verify temporal filtering works correctly.

**Acceptance Scenarios**:

1. **Given** a fact "Kai works on Orion since 2025-06-01", **When**
   `mem kg query Kai` is run, **Then** the fact appears as current.
2. **Given** the fact is invalidated as of 2026-03-01, **When**
   queried after that date, **Then** it appears as historical,
   not current.
3. **Given** two conflicting facts from different sources, **When**
   a new fact is added, **Then** the system flags the contradiction.
4. **Given** an entity timeline query, **When** `mem kg timeline
   Orion` is run, **Then** all facts about Orion are returned in
   chronological order.

---

### User Story 5 - MCP Server (Priority: P5)

As a developer using Claude Code, ChatGPT, or Cursor, I want
`mem` to expose its capabilities as an MCP server so that my AI
agent can search, add, and navigate the palace automatically
through tool calls.

I register `mem mcp` as an MCP server, and the AI gets access to
tools for searching, adding drawers, querying the knowledge graph,
and navigating the palace structure.

**Why this priority**: MCP integration makes the palace invisible
to the user — the AI handles it. But it requires all other features
to be working first.

**Independent Test**: Register MCP server, ask the AI "what did
we decide about auth", verify it calls `mem_search` and returns
correct results.

**Acceptance Scenarios**:

1. **Given** `mem mcp` is running, **When** a client connects,
   **Then** available tools are listed (search, add_drawer,
   kg_query, status, etc.).
2. **Given** the AI calls `mem_search`, **When** results are
   returned, **Then** they match CLI `mem search` output.
3. **Given** the AI calls `mem_add_drawer`, **When** content is
   filed, **Then** it appears in subsequent searches.

---

### User Story 6 - Conversation Mining with Format Detection (Priority: P6)

As a developer, I want to mine conversation exports from multiple
AI tools (Claude, ChatGPT, Slack) with automatic format detection
so that I don't need to convert files manually before mining.

**Why this priority**: Most valuable memories are in conversations.
Supporting multiple formats removes a major adoption barrier.

**Independent Test**: Export conversations from Claude and ChatGPT,
mine both, verify all exchanges are indexed.

**Acceptance Scenarios**:

1. **Given** a Claude JSONL export, **When** mined with
   `--mode convos`, **Then** each assistant/human exchange pair
   becomes a separate drawer.
2. **Given** a ChatGPT JSON export, **When** mined, **Then**
   format is auto-detected and parsed correctly.
3. **Given** a Slack export directory, **When** mined, **Then**
   messages are grouped by channel and thread.

---

### Edge Cases

- What happens when a file cannot be read (permissions, binary)?
  The system MUST skip it, log a warning, and continue mining
  other files.
- What happens when the palace database becomes corrupted? The
  system MUST detect corruption on startup and offer a repair
  command.
- What happens when search returns zero results? The system MUST
  display a helpful message suggesting broader search terms or
  different wing/room.
- What happens when two concurrent processes write to the palace?
  The system MUST use database-level locking to prevent corruption.
- What happens when the identity file is missing during wake-up?
  The system MUST output L1 only (no L0) without error.
- What happens when mining very large files (>10MB)? The system
  MUST chunk them into manageable pieces for indexing.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST organize memories in a hierarchical
  palace structure: Wing (person/project) → Hall (memory type:
  facts, events, discoveries, preferences, advice) → Room
  (specific topic) → Closet (compressed summary) → Drawer
  (verbatim content).
- **FR-002**: System MUST store verbatim original content in
  drawers. No summarization or lossy compression of source data.
  Closets contain compressed summaries that point to drawers.
- **FR-003**: System MUST support semantic search across all
  drawers using built-in text similarity (no external embedding
  service required). Search MUST support filtering by wing, room,
  and hall.
- **FR-004**: System MUST implement a 4-layer memory stack:
  L0 (Identity, ~50 tokens, always loaded), L1 (Critical facts,
  ~120 tokens, always loaded), L2 (On-demand retrieval by
  wing/room), L3 (Deep semantic search).
- **FR-005**: System MUST provide a wake-up command that outputs
  L0+L1 in a compact format (~170 tokens total) readable by any
  LLM without a decoder.
- **FR-006**: System MUST implement a temporal knowledge graph
  with entities, typed relationships, and validity windows
  (valid_from, valid_to). Facts MUST be queryable by entity,
  relationship type, and time period.
- **FR-007**: System MUST detect contradictions when new facts
  conflict with existing facts in the knowledge graph (e.g.,
  conflicting assignments, wrong dates, stale information).
- **FR-008**: System MUST mine project files (code, docs, text)
  and conversation exports (Claude, ChatGPT, Slack formats) into
  the palace structure.
- **FR-009**: System MUST auto-detect rooms (topics) from content
  during mining, without requiring manual topic assignment. When
  topic detection fails (no keywords extracted), the system MUST
  assign content to a "general" room as fallback.
- **FR-010**: System MUST provide an MCP server exposing palace
  capabilities as tools (search, add, navigate, knowledge graph)
  for integration with MCP-compatible AI agents.
- **FR-011**: System MUST deduplicate content during mining — the
  same file or conversation mined twice MUST NOT create duplicate
  drawers.
- **FR-012**: System MUST support cross-wing connections (tunnels)
  — when the same room appears in multiple wings, they are
  automatically linked.
- **FR-013**: System MUST produce a single static binary with no
  runtime dependencies beyond the binary itself.
- **FR-014**: System MUST work entirely offline — no internet
  connection required after installation.
- **FR-015**: System MUST support CLI commands: `init` (setup
  palace), `mine` (ingest data), `search` (find memories),
  `wake-up` (load context), `status` (overview), `kg` (knowledge
  graph operations), `mcp` (start MCP server).
- **FR-016**: System MUST store all structured data in a single
  database file for portability (easy backup, sync, transfer
  between machines). User-editable configuration files (e.g.,
  identity.txt) are separate plain text files.
- **FR-017**: System MUST chunk large files into segments suitable
  for indexing, preserving enough context per chunk to be
  independently understandable.

### Key Entities

- **Wing**: A top-level grouping (person, project, topic).
  Attributes: name, type (person/project/general), keywords.
- **Hall**: A memory-type corridor within a wing. Fixed set:
  facts, events, discoveries, preferences, advice.
- **Room**: A named topic within a wing. Attributes: name (slug),
  wing, halls containing it, drawer count.
- **Closet**: A compressed summary of a room's content. Attributes:
  room, compressed text, token count.
- **Drawer**: Verbatim stored content. Attributes: content text,
  wing, hall, room, source file, timestamp, content hash (for
  dedup).
- **Entity**: A node in the knowledge graph (person, project,
  tool, concept). Attributes: name, type, properties.
- **Triple**: A relationship in the knowledge graph. Attributes:
  subject, predicate, object, valid_from, valid_to, confidence,
  source reference.
- **Tunnel**: An automatic cross-wing connection created when the
  same room appears in multiple wings.

## Assumptions

- The built-in search uses TF-IDF or BM25 for text similarity
  (zero external dependencies). Users who want better accuracy
  can configure an external embedding API endpoint.
- Room auto-detection uses keyword extraction and clustering from
  document content — no LLM required.
- The compressed format (similar to AAAK) is a structured text
  abbreviation readable by any LLM — not a binary encoding.
- Conversation format detection supports at minimum: Claude
  (JSONL), ChatGPT (JSON), plain text transcripts, and Slack
  (JSON export).
- The MCP server implements the Model Context Protocol
  specification for tool-based interaction.
- All data is stored in a single SQLite database file at
  `~/.mempalace/palace.db` by default, overridable via
  `--palace` flag or `MEM_PALACE` environment variable.
- The binary embeds SQLite (via CGo or pure-Go driver) — the
  user does not need to install SQLite separately.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Mining 1000 files completes in under 30 seconds on
  a standard machine.
- **SC-002**: Search returns results in under 100ms for a palace
  with 10,000 drawers.
- **SC-003**: Wake-up context is under 300 tokens and contains
  the user's key people, projects, and recent events.
- **SC-004**: Palace structure (wing+room filtering) improves
  search recall by at least 20% compared to flat unstructured
  search across the same data.
- **SC-005**: The system runs entirely offline with zero internet
  dependency after installation.
- **SC-006**: The single binary is under 15 MB and starts in
  under 100ms.
- **SC-007**: Knowledge graph queries return results in under
  10ms for a graph with 1000 entities.
- **SC-008**: The MCP server handles tool calls with under 200ms
  latency.
- **SC-009**: Duplicate detection prevents re-indexing of already-
  mined content with 99%+ accuracy.
