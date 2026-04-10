package db

const schema = `
CREATE TABLE IF NOT EXISTS wings (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT UNIQUE NOT NULL,
    type TEXT DEFAULT 'general',
    keywords TEXT DEFAULT '',
    created_at TEXT DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS rooms (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    wing_id INTEGER NOT NULL REFERENCES wings(id),
    created_at TEXT DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(name, wing_id)
);

CREATE TABLE IF NOT EXISTS drawers (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    content TEXT NOT NULL,
    content_hash TEXT NOT NULL UNIQUE,
    wing_id INTEGER NOT NULL REFERENCES wings(id),
    room_id INTEGER NOT NULL REFERENCES rooms(id),
    hall TEXT DEFAULT 'facts',
    source_file TEXT DEFAULT '',
    source_type TEXT DEFAULT 'file',
    created_at TEXT DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS closets (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    room_id INTEGER NOT NULL REFERENCES rooms(id),
    compressed_text TEXT NOT NULL,
    token_count INTEGER DEFAULT 0,
    updated_at TEXT DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS search_terms (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    term TEXT UNIQUE NOT NULL
);

CREATE TABLE IF NOT EXISTS search_index (
    term_id INTEGER NOT NULL REFERENCES search_terms(id),
    drawer_id INTEGER NOT NULL REFERENCES drawers(id),
    tf REAL NOT NULL,
    PRIMARY KEY (term_id, drawer_id)
);

CREATE TABLE IF NOT EXISTS search_meta (
    key TEXT PRIMARY KEY,
    value TEXT
);

CREATE TABLE IF NOT EXISTS entities (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    type TEXT DEFAULT 'unknown',
    properties TEXT DEFAULT '{}',
    created_at TEXT DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS triples (
    id TEXT PRIMARY KEY,
    subject TEXT NOT NULL REFERENCES entities(id),
    predicate TEXT NOT NULL,
    object TEXT NOT NULL REFERENCES entities(id),
    valid_from TEXT,
    valid_to TEXT,
    confidence REAL DEFAULT 1.0,
    source_drawer_id INTEGER REFERENCES drawers(id),
    created_at TEXT DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_triples_subject ON triples(subject);
CREATE INDEX IF NOT EXISTS idx_triples_object ON triples(object);
CREATE INDEX IF NOT EXISTS idx_triples_valid ON triples(valid_from, valid_to);
CREATE INDEX IF NOT EXISTS idx_drawers_wing ON drawers(wing_id);
CREATE INDEX IF NOT EXISTS idx_drawers_room ON drawers(room_id);
CREATE INDEX IF NOT EXISTS idx_drawers_hash ON drawers(content_hash);
CREATE INDEX IF NOT EXISTS idx_search_term ON search_index(term_id);
`

func InitSchema(d *DB) error {
	_, err := d.Exec(schema)
	return err
}
