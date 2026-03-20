# Feature Specification: Agent Memory Companion

**Feature Branch**: `001-agent-memory-companion`
**Created**: 2026-03-20
**Status**: Draft
**Input**: User description: "Build an application that can provide memory for agents and use memory-companion-spec.md for it"

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Automatic Event Capture After Sessions (Priority: P1)

As a developer using an AI coding agent, I want the system to
automatically capture significant events (architectural decisions,
discovered bugs, recurring patterns, insights, and rollbacks) from
each completed work session so that important context is not lost
between sessions.

After every session the agent completes, the memory companion
reviews what happened, identifies noteworthy events, and persists
them as structured episode records. Only meaningful events are
recorded — routine operations (successful test runs, standard
commits) are filtered out.

**Why this priority**: Without event capture, no other memory
capability is possible. This is the foundational data-gathering
mechanism that feeds all downstream features (consolidation,
skill extraction, context injection).

**Independent Test**: Run 5 work sessions that include at least
one architectural decision, one bug fix, and one repeated task.
Verify that 1-5 relevant episode records appear after each
session, with no duplicates and no routine noise.

**Acceptance Scenarios**:

1. **Given** a completed work session where the agent chose JSONL
   over SQLite for a storage decision, **When** the memory
   companion runs extraction, **Then** an episode record of type
   "decision" is created with a one-sentence summary and relevant
   tags.
2. **Given** a completed session where tests failed due to a race
   condition, **When** the memory companion runs extraction,
   **Then** an episode record of type "error" is created capturing
   the root cause.
3. **Given** a session with only routine file edits and passing
   tests, **When** the memory companion runs extraction, **Then**
   no new episode records are created.
4. **Given** a session that produced an event already recorded in
   a previous session, **When** the memory companion runs
   extraction, **Then** the duplicate event is not recorded again.

---

### User Story 2 - Memory Consolidation and Principle Extraction (Priority: P2)

As a developer, I want the system to periodically analyze
accumulated episode records, extract reusable principles (rules),
detect skill candidates, and clean up outdated or duplicate
records so that the memory remains compact, accurate, and
actionable.

When a consolidation threshold is reached (configurable number of
sessions or episode count), the memory companion groups episodes
by theme, promotes recurring patterns into explicit principles,
flags repeated multi-step procedures as skill candidates, and
removes duplicates and superseded records.

**Why this priority**: Raw episodes become noisy over time.
Consolidation transforms event data into structured knowledge
that the agent can act on, and keeps storage size manageable.

**Independent Test**: Seed the system with 30+ episode records
including 3+ similar events about database migrations. Run
consolidation. Verify a new principle about migrations appears,
duplicates are removed, and a consolidation log entry is created.

**Acceptance Scenarios**:

1. **Given** 30 accumulated episode records with 4 episodes
   tagged "migration" describing similar manual steps, **When**
   consolidation runs, **Then** a principle is extracted and added
   to the principles store (e.g., "Before migration, always
   snapshot current schema").
2. **Given** episode records containing 3 duplicate entries for
   the same event, **When** consolidation runs, **Then** the
   duplicates are removed and only one copy remains.
3. **Given** an existing principle that contradicts the current
   state of the project, **When** consolidation runs, **Then**
   the outdated principle is removed.
4. **Given** consolidation completes, **When** the consolidation
   log is checked, **Then** it contains a dated entry with counts
   of episodes processed, principles added/updated/removed, and
   skills identified.

---

### User Story 3 - Context Injection at Session Start (Priority: P3)

As a developer, I want the agent to automatically receive
relevant memory context (recent events, extracted principles, and
applicable skills) at the beginning of each new work session so
that the agent avoids repeating past mistakes and follows
established project rules from the first interaction.

When a new agent session starts, the system surfaces the most
relevant principles, recent episode records, and any skills
matching the upcoming task context.

**Why this priority**: Memory only delivers value when it is
surfaced at the right time. Without injection, the agent must be
manually reminded of prior context every session.

**Independent Test**: After accumulating principles and episodes
across 10 sessions, start a new session. Verify the agent
receives relevant principles and recent episodes without the user
needing to request them.

**Acceptance Scenarios**:

1. **Given** 5 principles and 20 episode records exist in memory,
   **When** a new agent session begins, **Then** the agent
   receives all current principles and the 10 most recent
   episodes as context.
2. **Given** a skill "Database Migration" exists and the user's
   first message mentions schema changes, **When** the session
   starts, **Then** the relevant skill is surfaced to the agent.
3. **Given** memory storage is empty (first-ever session),
   **When** a session begins, **Then** the system starts normally
   with no errors and no memory context injected.

---

### User Story 4 - Procedural Skill Library (Priority: P4)

As a developer, I want the system to automatically detect repeated
multi-step procedures and capture them as reusable skill recipes
(with trigger conditions, prerequisites, steps, success checks,
and anti-patterns) so that the agent can follow proven procedures
instead of reinventing them each time.

When the same sequence of actions is performed 3 or more times
(or once for complex/critical procedures), the system creates a
skill entry. Each skill is a self-contained recipe that the agent
can reference when encountering a matching task.

**Why this priority**: Procedural memory reduces errors in
complex, multi-step workflows and ensures consistency. However, it
depends on having sufficient episode data (P1) and consolidation
(P2) to detect patterns.

**Independent Test**: Seed 5+ episodes describing the same
multi-step database migration procedure. Run consolidation.
Verify a skill entry is created with the correct structure
(trigger, prerequisites, steps, success check, anti-patterns).

**Acceptance Scenarios**:

1. **Given** 3 episodes describing similar database migration
   steps, **When** consolidation runs, **Then** a skill record
   "Database Migration" is created with all required sections.
2. **Given** a skill "Database Migration" exists, **When** the
   user asks the agent to change a database schema, **Then** the
   agent references the skill's steps rather than improvising.
3. **Given** a skill was created 6 months ago and the project's
   migration tooling has changed, **When** consolidation runs,
   **Then** the outdated skill is flagged for review.

---

### User Story 5 - Multi-Agent Memory Coordination (Priority: P5)

As a developer running multiple parallel agents on the same
project, I want them to share a single memory store without
conflicts so that all agents benefit from each other's
discoveries and do not duplicate work or produce contradictory
decisions.

When multiple agents operate concurrently, the system ensures
safe concurrent writes to the memory store, includes agent
identification in episode records, and detects conflicting
decisions between agents.

**Why this priority**: This is an advanced capability needed only
when multiple agents work in parallel. It builds on all previous
stories and introduces concurrency concerns.

**Independent Test**: Run two agents concurrently that both
attempt to write episode records. Verify no records are lost or
corrupted, each record identifies its source agent, and
conflicting decisions are flagged.

**Acceptance Scenarios**:

1. **Given** two agents completing sessions simultaneously,
   **When** both write episode records concurrently, **Then** all
   records are persisted without data loss or corruption.
2. **Given** Agent A records a decision "Use approach X" and
   Agent B records "Use approach Y" for the same problem, **When**
   consolidation runs, **Then** the conflicting decisions are
   flagged for human review.
3. **Given** multiple agents share the memory store, **When**
   episode records are inspected, **Then** each record includes an
   identifier for the agent that created it.

---

### Edge Cases

- What happens when memory storage becomes corrupted (malformed
  records)? The system MUST skip corrupt records, log a warning,
  and continue processing valid records.
- What happens when the episode store exceeds the maximum record
  limit (200 records)? The system MUST trigger consolidation and
  remove the oldest records that have been fully covered by
  extracted principles.
- What happens when consolidation removes a principle that is
  still valid? All consolidation changes MUST be reviewable and
  reversible (via version history).
- What happens when extraction runs but the session produced no
  meaningful events? The system MUST complete silently without
  creating empty or placeholder records.
- What happens when two consolidations are triggered
  simultaneously? The system MUST serialize consolidation runs to
  prevent conflicting updates.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST capture significant events from
  completed agent sessions and persist them as structured episode
  records.
- **FR-002**: Each episode record MUST contain a timestamp,
  session identifier, event type, one-sentence summary, and 1-3
  tags.
- **FR-003**: System MUST support five event types: decision,
  error, pattern, insight, and rollback.
- **FR-004**: System MUST filter out routine operations and only
  record events that carry architectural, debugging, or procedural
  significance.
- **FR-005**: System MUST deduplicate episode records — an event
  already present in the store MUST NOT be recorded again.
- **FR-006**: System MUST periodically consolidate episodes by
  grouping related events, extracting principles from clusters of
  3+ similar events, and removing duplicates and superseded
  records.
- **FR-007**: Principles MUST be expressed as concrete, verifiable
  actions (not vague guidance), grouped by topic.
- **FR-008**: System MUST enforce a maximum size for the
  principles store (100 entries). When exceeded, consolidation
  MUST merge similar principles and remove outdated ones.
- **FR-009**: System MUST enforce a maximum size for the episode
  store (200 records). The 50 most recent records MUST always be
  preserved during cleanup.
- **FR-010**: System MUST detect repeated multi-step procedures
  (3+ occurrences) and create skill records containing: trigger
  conditions, prerequisites, ordered steps, success verification,
  and anti-patterns.
- **FR-011**: System MUST inject relevant memory context
  (principles, recent episodes, matching skills) at the start of
  each agent session.
- **FR-012**: System MUST maintain a consolidation log recording
  each consolidation's date, counts of episodes processed,
  principles added/updated/removed, and skills created.
- **FR-013**: System MUST store all memory data in human-readable,
  inspectable files — no opaque binary formats or databases.
- **FR-014**: System MUST support both automatic (hook-triggered)
  and manual invocation of extraction and consolidation.
- **FR-015**: System MUST support configurable consolidation
  triggers: by session count (default: every 10 sessions), by
  episode count (default: when exceeding 100 records), or manual.
- **FR-016**: System MUST identify the source agent in each
  episode record when multiple agents share the memory store.
- **FR-017**: System MUST detect and flag conflicting decisions
  recorded by different agents for human review.
- **FR-018**: System MUST handle concurrent writes to the memory
  store without data loss or corruption.

### Key Entities

- **Episode**: A single significant event captured from an agent
  session. Attributes: timestamp, session identifier, event type,
  summary text, tags, and optionally source agent identifier.
- **Principle**: An extracted, verifiable rule derived from
  recurring patterns in episodes. Attributes: topic/category,
  rule statement. Grouped by topic.
- **Skill**: A reusable procedural recipe for multi-step tasks.
  Attributes: name, trigger conditions, prerequisites, ordered
  steps, success verification criteria, anti-patterns.
- **Consolidation Log Entry**: A record of a consolidation run.
  Attributes: date, episodes processed count, principles
  added/updated/removed counts, skills created, skill candidates
  identified.
- **Memory Store**: The collection of all episodes, principles,
  skills, and consolidation logs for a project. Scoped per
  project.

## Assumptions

- The system targets AI coding agents (primarily Claude Code) as
  its users. "User" in this spec refers to the developer who
  operates the agent.
- Memory is scoped per project — each project has its own
  independent memory store.
- Memory files are version-controlled alongside the project
  (committed to the repository) to enable review and rollback.
- The extraction subagent can access the session's change history
  (e.g., git diff) to identify significant events.
- Cost efficiency matters: extraction uses a lightweight model
  (cost-effective), while consolidation uses a more capable model
  for analysis and synthesis tasks.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Zero repeated errors — once an error is recorded in
  memory, the agent MUST NOT make the same mistake in subsequent
  sessions (measured by comparing episode records against agent
  behavior over 20 sessions).
- **SC-002**: Principles store contains 20-80 entries after the
  first month of use, indicating active extraction without bloat.
- **SC-003**: 5-15 skills are created within the first month of
  use from real detected patterns.
- **SC-004**: Agent references an existing skill at least once per
  week when encountering a matching task.
- **SC-005**: Memory companion cost is less than 5% of the main
  agent's cost per session (measured by token usage comparison).
- **SC-006**: Greater than 80% of recorded episodes are judged as
  genuinely significant upon manual review (sampled bi-weekly).
- **SC-007**: New agent sessions receive relevant context within
  the first interaction, without the developer needing to manually
  remind the agent of prior decisions.
