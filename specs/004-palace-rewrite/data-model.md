# Data Model: Palace Architecture

**Date**: 2026-04-07

## Palace Hierarchy

```
Wing (person/project)
 └── Hall (memory type: facts/events/discoveries/preferences/advice)
      └── Room (named topic: "auth-migration", "ci-pipeline")
           ├── Closet (compressed summary of room content)
           └── Drawer (verbatim original text)

Tunnel = automatic link when same Room appears in multiple Wings
```

## Entities

### Wing

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| id | int | auto | Primary key |
| name | string | Yes | Unique name (e.g., "kai", "driftwood") |
| type | string | Yes | "person", "project", or "general" |
| keywords | string | No | Comma-separated keywords for auto-assignment |
| created_at | datetime | auto | Creation timestamp |

### Room

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| id | int | auto | Primary key |
| name | string | Yes | Topic slug (e.g., "auth-migration") |
| wing_id | int | Yes | Parent wing |
| created_at | datetime | auto | Creation timestamp |

Unique constraint: (name, wing_id) — same room can exist in
different wings (creating a tunnel).

### Drawer

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| id | int | auto | Primary key |
| content | text | Yes | Verbatim original text |
| content_hash | string | Yes | SHA-256 for dedup (unique) |
| wing_id | int | Yes | Parent wing |
| room_id | int | Yes | Parent room |
| hall | string | Yes | Memory type: facts/events/discoveries/preferences/advice |
| source_file | string | No | Original file path |
| source_type | string | No | "file", "conversation", "manual" |
| created_at | datetime | auto | Timestamp |

### Closet

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| id | int | auto | Primary key |
| room_id | int | Yes | Parent room |
| compressed_text | text | Yes | AAAK-like compressed summary |
| token_count | int | No | Estimated token count |
| updated_at | datetime | auto | Last update |

### Entity (Knowledge Graph)

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| id | string | Yes | Normalized name (lowercase, underscored) |
| name | string | Yes | Display name |
| type | string | Yes | "person", "project", "tool", "concept" |
| properties | json | No | Additional key-value properties |
| created_at | datetime | auto | Timestamp |

### Triple (Knowledge Graph)

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| id | string | auto | Generated unique ID |
| subject | string | Yes | Entity ID |
| predicate | string | Yes | Relationship type (e.g., "works_on") |
| object | string | Yes | Entity ID |
| valid_from | date | No | When this fact became true |
| valid_to | date | No | When this fact stopped being true (null=current) |
| confidence | float | No | 0.0-1.0, default 1.0 |
| source_drawer_id | int | No | Reference to supporting drawer |
| created_at | datetime | auto | When the triple was recorded |

## Relationships

```
Wing 1──N Room
Room 1──N Drawer
Room 1──1 Closet
Wing 1──N Drawer (via room)
Drawer N──1 Room
Entity N──N Entity (via Triple)
Triple N──1 Drawer (source reference)
```

## Search Index (BM25)

| Table | Fields | Purpose |
|-------|--------|---------|
| search_terms | id, term | Unique vocabulary |
| search_index | term_id, drawer_id, tf | Term frequency per drawer |
| search_meta | key, value | Corpus stats (avg_doc_len, total_docs) |

Updated on each drawer insert/delete.
