# Tasks: Palace Architecture Rewrite

**Input**: Design documents from `/specs/004-palace-rewrite/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md, quickstart.md

**Tests**: BDD mandatory. Tests FIRST per plan.md.

**Organization**: Grouped by user story (6 stories). This is a full rewrite — clean the old code first, then build from scratch.

## Format: `[ID] [P?] [Story] Description`

---

## Phase 1: Setup

**Purpose**: Initialize new project structure, deps, SQLite schema

- [x] T001 Remove old `internal/` code (episode, principle, skill, consolidation, store, filelock, runner, agent, config packages) and `cmd/mem/main.go`. Clean slate for rewrite. Keep `.opencode/`, `.github/`, `Dockerfile`, `README.md`, `go.mod`.
- [x] T002 Update `go.mod` — add `modernc.org/sqlite` dependency. Run `go mod tidy`.
- [x] T003 [P] Create directory structure per plan.md: `internal/palace/`, `internal/search/`, `internal/kg/`, `internal/layers/`, `internal/miner/`, `internal/mcp/`, `internal/config/`, `internal/db/`
- [x] T004 [P] BDD test + implement SQLite schema creation and database connection helpers in `internal/db/schema.go` and `internal/db/db.go` and `internal/db/db_test.go`. Schema includes all tables from plan.md (wings, rooms, drawers, closets, search_terms, search_index, search_meta, entities, triples) with indexes. `Open(path)` returns `*sql.DB`, `InitSchema(db)` creates all tables if not exist.
- [x] T005 [P] BDD test + implement config loading in `internal/config/config.go` and `internal/config/config_test.go`. Fields: PalacePath (default `~/.mempalace`), DbFile (default `palace.db`). Env vars: `MEM_PALACE`. Config resolves absolute path.

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Palace core + search engine — needed by ALL user stories

- [x] T006 BDD test + implement Wing CRUD in `internal/palace/wing.go` and `internal/palace/wing_test.go`. Functions: `CreateWing(db, name, type, keywords)`, `GetWing(db, name)`, `ListWings(db)`. BDD: Given no wings, When created, Then listed; Given existing wing, When created again, Then returns existing (idempotent).
- [x] T007 [P] BDD test + implement Room CRUD in `internal/palace/room.go` and `internal/palace/room_test.go`. Functions: `CreateRoom(db, name, wingID)`, `GetRoom(db, name, wingID)`, `ListRooms(db, wingID)`. Unique constraint on (name, wing_id).
- [x] T008 [P] BDD test + implement Drawer CRUD in `internal/palace/drawer.go` and `internal/palace/drawer_test.go`. Functions: `AddDrawer(db, content, wingID, roomID, hall, sourceFile, sourceType)` — computes SHA-256 hash, dedup via UNIQUE(content_hash). `GetDrawer(db, id)`, `ListDrawers(db, wingID, roomID)`, `CountDrawers(db)`. BDD: Given same content added twice, When checked, Then only one drawer exists.
- [x] T009 [P] BDD test + implement text tokenizer in `internal/search/tokenizer.go` and `internal/search/tokenizer_test.go`. Functions: `Tokenize(text) []string` — lowercase, split by whitespace/punctuation, strip English stopwords (hardcoded ~150 words), optional Porter stemming. BDD: Given "The quick brown fox", When tokenized, Then ["quick", "brown", "fox"].
- [x] T010 BDD test + implement BM25 search engine in `internal/search/bm25.go`, `internal/search/index.go`, `internal/search/search.go` and tests. `IndexDrawer(db, drawerID, content)` — tokenize, insert into search_terms/search_index, update search_meta. `Search(db, query, wingID, roomID, limit) []SearchResult` — tokenize query, compute BM25 scores, return ranked results with drawer content, wing, room, similarity. BDD: Given 10 indexed drawers about different topics, When searching for a specific topic, Then relevant drawers rank highest.
- [x] T011 BDD test + implement Palace init/status in `internal/palace/palace.go` and `internal/palace/palace_test.go`. `Init(palacePath)` — create dir, open DB, init schema. `Status(db)` — return wing count, room count, drawer count, DB size. BDD: Given empty dir, When Init, Then DB created with schema; When Status, Then all zeros.
- [x] T012 BDD test + implement CLI skeleton with subcommand dispatch in `cmd/mem/main.go`. Commands: init, mine, search, wake-up, status, kg, mcp. Each as stub that prints "not yet implemented" except init/status which wire to palace.Init/Status. BDD: build binary, run `mem init`, verify palace created; run `mem status`, verify counts shown.

**Checkpoint**: Palace database works, drawers stored/searched via BM25, CLI skeleton functional.

---

## Phase 3: User Story 1 — Mine Project Files (Priority: P1) MVP

**Goal**: `mem mine ~/project` indexes files into palace

- [x] T013 [P] [US1] BDD test + implement text file chunker in `internal/miner/chunker.go` and `internal/miner/chunker_test.go`. `ChunkFile(path, maxChunkSize) []Chunk` — read file, split into chunks of ~500 words preserving paragraph boundaries. Each chunk includes source file path. BDD: Given a 2000-word file, When chunked at 500, Then 4 chunks with correct boundaries.
- [x] T014 [P] [US1] BDD test + implement room auto-detection in `internal/miner/room_detector.go` and `internal/miner/room_detector_test.go`. `DetectRoom(content) string` — extract top 2-3 keywords via TF-IDF on the chunk, combine into slug (e.g., "auth-migration"). Fallback: return "general" when no meaningful keywords extracted. BDD: Given text about OAuth and login, When detected, Then room slug contains "auth" or "oauth"; Given very short or stopword-only text, When detected, Then room slug is "general".
- [x] T015 [P] [US1] BDD test + implement content deduplication in `internal/miner/dedup.go` and `internal/miner/dedup_test.go`. `IsDuplicate(db, contentHash) bool` — check drawers table for existing hash. BDD: Given content already mined, When same content hashed, Then returns true.
- [x] T016 [US1] BDD test + implement file mining orchestrator in `internal/miner/miner.go` and `internal/miner/miner_test.go`. `MineDirectory(db, dir, wing, mode)` — walk directory, filter text files (skip binary), chunk each file, detect room, dedup, add drawers, index for search. Report: files processed, drawers created, duplicates skipped. BDD: Given a dir with 5 text files, When mined, Then 5+ drawers created and searchable.
- [x] T017 [US1] Wire `mem mine` command in `cmd/mem/main.go` — flags: `--wing`, `--mode` (files/convos), positional arg for directory. Calls miner.MineDirectory, prints summary.
- [x] T018 [US1] Wire `mem search` command (basic) in `cmd/mem/main.go` — positional query arg, `--wing`, `--room`, `--limit` flags. Calls search.Search, prints results with wing/room/source/similarity.

**Checkpoint**: `mem mine ~/project && mem search "query"` works end-to-end.

---

## Phase 4: User Story 2 — Search with Palace Filtering (Priority: P2)

**Goal**: Search accuracy improves with wing/room filtering

- [x] T019 [US2] BDD test + implement Tunnel detection in `internal/palace/tunnel.go` and `internal/palace/tunnel_test.go`. `FindTunnels(db, wingA, wingB) []Tunnel` — rooms that appear in multiple wings. `ListTunnels(db) []Tunnel`. BDD: Given room "auth" in wing A and wing B, When tunnels listed, Then "auth" appears as tunnel.
- [x] T020 [US2] BDD test + implement palace graph traversal in `internal/search/search.go` — `Traverse(db, startRoom, maxHops)` — BFS from a room through shared wings. BDD: Given rooms connected across wings, When traversing from room A, Then connected rooms found.
- [x] T021 [US2] Enhance `mem search` output — show palace structure path (wing/hall/room) for each result, similarity score, source file. Add `--verbose` flag to show full drawer content.

**Checkpoint**: Search results show palace structure, tunnel navigation works.

---

## Phase 5: User Story 3 — Wake-Up Context (Priority: P3)

**Goal**: `mem wake-up` outputs ~170 token compact context

- [x] T022 [P] [US3] BDD test + implement L0 Identity loader in `internal/layers/identity.go` and `internal/layers/identity_test.go`. Read `~/.mempalace/identity.txt` (plain text, ~50 tokens). Return text or default message if missing. BDD: Given identity.txt exists, When loaded, Then content returned; Given missing, Then default returned.
- [x] T023 [P] [US3] BDD test + implement AAAK-like L1 compressor in `internal/layers/compress.go` and `internal/layers/compress_test.go`. `CompressL1(db, wing) string` — pull top drawers by recency/importance, extract key entities, format as compact abbreviations (~120 tokens). BDD: Given 50 drawers, When compressed, Then output under 150 tokens and contains key entities.
- [x] T024 [US3] BDD test + implement MemoryStack in `internal/layers/stack.go` and `internal/layers/stack_test.go`. `WakeUp(db, wing) string` — combine L0+L1. `Recall(db, wing, room) string` — L2 filtered retrieval. `Search(db, query, wing, room) string` — L3 deep search. BDD: Given populated palace, When WakeUp, Then output contains identity + key facts.
- [x] T025 [US3] Wire `mem wake-up` command in `cmd/mem/main.go` — `--wing` flag. Print L0+L1 to stdout.

**Checkpoint**: `mem wake-up` outputs compact context under 300 tokens.

---

## Phase 6: User Story 4 — Knowledge Graph (Priority: P4)

**Goal**: Temporal entity-relationship graph with contradiction detection

- [x] T026 [P] [US4] BDD test + implement KG entity/triple CRUD in `internal/kg/graph.go` and `internal/kg/graph_test.go`. `AddEntity(db, name, type, props)`, `AddTriple(db, subj, pred, obj, validFrom, validTo)`, `Invalidate(db, subj, pred, obj, ended)`, `QueryEntity(db, name, asOf, direction)`, `Timeline(db, entity)`, `Stats(db)`. BDD: Given triple added with date, When queried at that date, Then returned; When queried after valid_to, Then not returned.
- [x] T027 [US4] BDD test + implement contradiction detection in `internal/kg/contradiction.go` and `internal/kg/contradiction_test.go`. `CheckContradiction(db, subj, pred, obj) []Conflict` — check if new triple conflicts with existing current triples (same subject+predicate, different object). BDD: Given "Kai works_on Orion" exists, When adding "Kai works_on Nova", Then contradiction flagged.
- [x] T028 [US4] Wire `mem kg` subcommands in `cmd/mem/main.go`: `mem kg add <subj> <pred> <obj> [--from DATE] [--to DATE]`, `mem kg query <entity> [--as-of DATE]`, `mem kg timeline <entity>`, `mem kg invalidate <subj> <pred> <obj> --ended DATE`, `mem kg stats`.

**Checkpoint**: Knowledge graph with temporal queries and contradiction detection.

---

## Phase 7: User Story 5 — MCP Server (Priority: P5)

**Goal**: `mem mcp` exposes palace as MCP tools

- [x] T029 Update `go.mod` — add `github.com/modelcontextprotocol/go-sdk` dependency.
- [x] T030 [US5] BDD test + implement MCP server with tool registration in `internal/mcp/server.go` and `internal/mcp/server_test.go`. Register tools: `mem_search` (query, wing, room), `mem_add_drawer` (content, wing, room, hall), `mem_status`, `mem_wake_up` (wing), `mem_kg_query` (entity, as_of), `mem_kg_add` (subj, pred, obj, from), `mem_list_wings`, `mem_list_rooms` (wing). Each tool handler calls the corresponding internal function. BDD: Given server created, When tools listed, Then all 8+ tools present with correct schemas.
- [x] T031 [US5] Wire `mem mcp` command in `cmd/mem/main.go` — start MCP server on stdio transport.

**Checkpoint**: `claude mcp add mem -- mem mcp` works, AI can search and add memories.

---

## Phase 8: User Story 6 — Conversation Mining (Priority: P6)

**Goal**: Mine Claude/ChatGPT/Slack exports

- [x] T032 [P] [US6] BDD test + implement conversation format parser in `internal/miner/convo.go` and `internal/miner/convo_test.go`. `ParseConversation(path) []Exchange` — auto-detect format (Claude JSONL, ChatGPT JSON, Slack JSON, plain text), parse into `Exchange{Speaker, Content, Timestamp}` structs. BDD: Given Claude JSONL file, When parsed, Then exchanges extracted with correct speakers; Given ChatGPT JSON, When parsed, Then same Exchange format.
- [x] T033 [US6] Integrate conversation mining into `MineDirectory` — when `--mode convos`, use convo parser instead of file chunker. Each exchange pair (human+assistant) becomes one drawer.

**Checkpoint**: `mem mine ~/chats --mode convos` indexes conversations.

---

## Phase 9: Polish & Cross-Cutting

- [x] T034 [P] BDD test + implement Closet generation in `internal/palace/closet.go` and `internal/palace/closet_test.go`. `GenerateCloset(db, roomID)` — take top drawers for a room, compress into a summary closet. Updated after each mine operation. BDD: Given room with 10 drawers, When closet generated, Then compressed text under 200 tokens.
- [x] T035 [P] Update `Dockerfile` — multi-stage build with `modernc.org/sqlite` dep. Verify image builds and `mem status` works inside container.
- [x] T036 [P] Update `README.md` — rewrite for palace architecture: new install, new commands, palace concept explanation, MCP setup, knowledge graph usage.
- [x] T037 Run `go test -race -shuffle=on ./...` and `go vet ./...` — verify all tests pass, no regressions.
- [x] T038 Verify static binary — build with `go build -o mem ./cmd/mem/`, run `ldd mem` (Linux) or `otool -L mem` (macOS), confirm no dynamic library dependencies. Binary must be self-contained per FR-013.

---

## Dependencies & Execution Order

### Phase Dependencies

- **Phase 1 (Setup)**: No deps — start immediately
- **Phase 2 (Foundational)**: Depends on Phase 1 — BLOCKS all user stories
- **US1 (Phase 3)**: Depends on Phase 2 — mining needs palace+search
- **US2 (Phase 4)**: Depends on US1 — needs drawers to search
- **US3 (Phase 5)**: Depends on Phase 2 — needs palace data
- **US4 (Phase 6)**: Depends on Phase 2 — uses DB schema
- **US5 (Phase 7)**: Depends on US1+US4 — wraps existing features
- **US6 (Phase 8)**: Depends on US1 — extends miner
- **Polish (Phase 9)**: After all user stories

### Parallel Opportunities

- Phase 1: T003, T004, T005 in parallel
- Phase 2: T006, T007, T008, T009 in parallel (different tables/packages)
- Phase 3: T013, T014, T015 in parallel
- Phase 5: T022, T023 in parallel
- Phase 9: T034, T035, T036 in parallel

### User Story Independence

- **US1** (mine): MVP — can be used standalone
- **US2** (search enhance): Extends US1 with tunnels/traversal
- **US3** (wake-up): Independent of US1 (reads DB directly)
- **US4** (kg): Independent (separate tables, separate CLI)
- **US5** (mcp): Depends on US1+US4 (wraps them)
- **US6** (convos): Extends US1 miner

---

## Implementation Strategy

### MVP: Phase 1 + 2 + 3 (T001-T018)

Working `mem init && mem mine && mem search`. Core palace with BM25 search.

### Incremental Delivery

1. Setup + Foundational → Palace DB, search engine
2. US1 (mine+search) → `mem mine && mem search` works (MVP!)
3. US3 (wake-up) → `mem wake-up` for AI context
4. US4 (kg) → Knowledge graph with temporal queries
5. US2 (tunnels) → Cross-wing navigation
6. US6 (convos) → Conversation mining
7. US5 (mcp) → MCP server for AI integration
8. Polish → Closets, Docker, README

---

## Notes

- BDD mandatory: tests FIRST, then implement
- `go test -race -shuffle=on ./...` must pass after every task
- Two external deps: `modernc.org/sqlite` + `modelcontextprotocol/go-sdk`
- Single SQLite DB file for everything
- Old mem-agent code preserved at github.com/snow-ghost/mem-agent
- 38 tasks total across 9 phases
