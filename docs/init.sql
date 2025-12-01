-- GoFile / fGo — SQLite schema (Milestone 1, alpha)
-- Notes:
-- - Linear history on `main` by default; armes are strings on enacts+refs
-- - Visibility enforced at box level
-- - Namespaces deferred; keep a column for future use with a default
-- - All timestamps stored as ISO8601 text (UTC)

PRAGMA foreign_keys = ON;
PRAGMA journal_mode = WAL;
PRAGMA synchronous = NORMAL; -- adjust to FULL for stricter durability

-- BOXES
CREATE TABLE IF NOT EXISTS boxes (
  id            TEXT PRIMARY KEY,            -- ULID
  namespace_id  TEXT NOT NULL DEFAULT 'global',
  name          TEXT NOT NULL,
  visibility    TEXT NOT NULL CHECK (visibility IN ('public','unlisted','private')),
  default_arm TEXT NOT NULL DEFAULT 'main',
  created_at    TEXT NOT NULL,
  updated_at    TEXT NOT NULL,
  UNIQUE(namespace_id, name)
) STRICT;

CREATE INDEX IF NOT EXISTS idx_boxes_visibility ON boxes(visibility);

-- enactS (immutable)
CREATE TABLE IF NOT EXISTS enacts (
  id         TEXT PRIMARY KEY,               -- ULID
  box_id     TEXT NOT NULL REFERENCES boxes(id) ON DELETE CASCADE,
  arm     TEXT NOT NULL,
  parent_id  TEXT REFERENCES enacts(id) ON DELETE SET NULL,
  message    TEXT,
  author     TEXT,
  timestamp  TEXT NOT NULL
) STRICT;

CREATE INDEX IF NOT EXISTS idx_enacts_box_arm ON enacts(box_id, arm);
CREATE INDEX IF NOT EXISTS idx_enacts_parent ON enacts(parent_id);

-- ENTRIES (manifest entries per enact)
CREATE TABLE IF NOT EXISTS entries (
  enact_id  TEXT NOT NULL REFERENCES enacts(id) ON DELETE CASCADE,
  path       TEXT NOT NULL,                  -- POSIX path
  sha256     TEXT NOT NULL CHECK (length(sha256) = 64),
  size       INTEGER NOT NULL CHECK (size >= 0),
  mode       INTEGER NOT NULL,
  PRIMARY KEY (enact_id, path)
) STRICT;

CREATE INDEX IF NOT EXISTS idx_entries_sha ON entries(sha256);

-- REFS (arm heads per box)
CREATE TABLE IF NOT EXISTS refs (
  box_id    TEXT NOT NULL REFERENCES boxes(id) ON DELETE CASCADE,
  arm    TEXT NOT NULL,
  enact_id TEXT NOT NULL REFERENCES enacts(id) ON DELETE CASCADE,
  PRIMARY KEY (box_id, arm)
) STRICT;

-- TOKENS (alpha auth: bearer tokens hashed with Argon2id)
CREATE TABLE IF NOT EXISTS tokens (
  id            TEXT PRIMARY KEY,            -- ULID
  name          TEXT NOT NULL,
  namespace_id  TEXT NOT NULL DEFAULT 'global',
  hash          TEXT NOT NULL,               -- Argon2id result
  scope         TEXT NOT NULL CHECK (scope IN ('read','write','admin')),
  created_at    TEXT NOT NULL,
  revoked_at    TEXT
) STRICT;

CREATE INDEX IF NOT EXISTS idx_tokens_namespace ON tokens(namespace_id);
CREATE INDEX IF NOT EXISTS idx_tokens_scope ON tokens(scope);

-- TOMBSTONES (optional in M1; harmless to include)
CREATE TABLE IF NOT EXISTS tombstones (
  object_type TEXT NOT NULL CHECK (object_type IN ('box','enact')),
  object_id   TEXT NOT NULL,
  expires_at  TEXT NOT NULL,
  PRIMARY KEY (object_type, object_id)
) STRICT;

-- TRIGGERS: update boxes.updated_at on change
CREATE TRIGGER IF NOT EXISTS trg_boxes_updated_at
AFTER UPDATE ON boxes
FOR EACH ROW
BEGIN
  UPDATE boxes SET updated_at = strftime('%Y-%m-%dT%H:%M:%fZ') WHERE id = NEW.id;
END;

-- SEED: none (create an admin token out-of-band and insert its Argon2id hash).
-- Example (placeholder):
-- INSERT INTO tokens(id,name,hash,scope,created_at)
-- VALUES('01JABC…','admin','argon2id$…','admin',strftime('%Y-%m-%dT%H:%M:%fZ'));
