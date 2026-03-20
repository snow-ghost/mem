# Data Model: Agent Memory Companion

**Date**: 2026-03-20
**Feature**: 001-agent-memory-companion

## Entities

### Episode

A single significant event captured from an agent work session.
Stored as one JSON line in `episodes.jsonl`.

| Field    | Type     | Required | Description |
|----------|----------|----------|-------------|
| ts       | string   | Yes      | ISO 8601 timestamp (UTC) of when the event occurred |
| session  | string   | Yes      | Session identifier (e.g., short git hash or UUID) |
| type     | string   | Yes      | One of: `decision`, `error`, `pattern`, `insight`, `rollback` |
| summary  | string   | Yes      | One-sentence description, understandable without session context |
| tags     | []string | Yes      | 1-3 keywords for grouping during consolidation |
| agent_id | string   | No       | Identifier of the source agent (for multi-agent coordination) |

**Validation rules**:
- `ts` MUST be valid ISO 8601
- `type` MUST be one of the five allowed values
- `summary` MUST be non-empty and <= 500 characters
- `tags` MUST contain 1-3 entries, each non-empty

**State transitions**: None. Episodes are immutable after creation.
Consolidation may delete episodes but never modifies them.

---

### Principle

An extracted, verifiable rule derived from recurring episode
patterns. Stored as a bullet line in `principles.md`, grouped
under topic headings.

| Field    | Type   | Required | Description |
|----------|--------|----------|-------------|
| topic    | string | Yes      | Category heading (e.g., "Architecture", "Testing") |
| rule     | string | Yes      | Concrete, verifiable action statement |

**Validation rules**:
- `topic` MUST be non-empty
- `rule` MUST be a concrete action, not a vague aspiration
- Total principle count MUST NOT exceed 100

**File format**:
```markdown
# Project Principles

## {topic}
- {rule}
- {rule}

## {topic}
- {rule}
```

**State transitions**:
- Created: Extracted from 3+ similar episodes during consolidation
- Updated: Merged with similar principle during consolidation
- Deleted: Contradicts current project state, or merged into another

---

### Skill

A reusable procedural recipe for multi-step tasks. One Markdown
file per skill in `.memory/skills/{slug}.md`.

| Field         | Type     | Required | Description |
|---------------|----------|----------|-------------|
| name          | string   | Yes      | Human-readable skill name (also the H1 heading) |
| slug          | string   | Yes      | URL-safe filename identifier (derived from name) |
| triggers      | []string | Yes      | Conditions that indicate this skill applies |
| prerequisites | []string | Yes      | What must be true before executing the skill |
| steps         | []string | Yes      | Ordered list of concrete actions |
| verification  | []string | Yes      | How to confirm the skill succeeded |
| antipatterns  | []string | Yes      | Common mistakes to avoid |
| created_at    | string   | No       | ISO 8601 date when the skill was first created |

**File format**:
```markdown
# {name}

## When to apply
- {trigger}

## Prerequisites
- {prerequisite}

## Steps
1. {step}
2. {step}

## Success verification
- {check}

## Anti-patterns
- {antipattern}
```

**State transitions**:
- Created: Detected from 3+ similar procedural episodes, or 1
  complex/critical procedure
- Flagged: Skill age > 6 months or underlying tooling changed
- Deleted: Superseded or no longer relevant

---

### ConsolidationLogEntry

A record of a single consolidation run. Appended to
`consolidation-log.md`.

| Field              | Type   | Required | Description |
|--------------------|--------|----------|-------------|
| date               | string | Yes      | ISO 8601 date of the consolidation run |
| number             | int    | Yes      | Sequential consolidation number |
| episodes_processed | int    | Yes      | Total episodes analyzed |
| principles_added   | int    | Yes      | New principles created |
| principles_updated | int    | Yes      | Existing principles modified |
| principles_removed | int    | Yes      | Outdated principles deleted |
| episodes_removed   | int    | Yes      | Duplicate/superseded episodes deleted |
| skills_created     | int    | Yes      | New skills generated |
| skill_candidates   | []string | No    | Names of potential skills not yet created |

**File format**:
```markdown
## 2026-03-20 — Consolidation #1
- Episodes processed: 12
- Principles added: 2 (architecture, testing)
- Principles updated: 1 (migrations)
- Principles removed: 0
- Episodes removed: 3 (duplicates)
- Skills created: 0
- Skill candidates: "database migration" (3 occurrences)
```

---

### MemoryStore

The collection of all memory files for a project. Not a persisted
entity itself — it is the directory structure.

| Component          | Path                       | Format  |
|--------------------|----------------------------|---------|
| Episodes           | .memory/episodes.jsonl     | JSONL   |
| Principles         | .memory/principles.md      | Markdown |
| Skills             | .memory/skills/{slug}.md   | Markdown |
| Consolidation log  | .memory/consolidation-log.md | Markdown |
| Extraction prompt  | .memory/prompts/extract.md | Markdown |
| Consolidation prompt | .memory/prompts/consolidate.md | Markdown |
| Session counter    | .memory/.session-count     | Plain integer |
| Lock file          | .memory/.lock              | Sentinel |

**Invariants**:
- episodes.jsonl <= 200 lines (enforced by consolidation)
- principles.md <= 100 rules (enforced by consolidation)
- 50 most recent episodes are never deleted by consolidation

## Relationships

```
Episode ---[3+ similar]---> Principle (extraction)
Episode ---[3+ procedural]--> Skill (detection)
ConsolidationLogEntry ---[records]--> changes to Episodes,
                                      Principles, Skills
MemoryStore ---[contains]--> all of the above
```

Episodes are the source data. Principles and skills are derived
from episodes through consolidation. The consolidation log tracks
all derivation operations.
