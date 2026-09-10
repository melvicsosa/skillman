-- Initial schema. Filesystem is the source of truth; these tables hold cache
-- and intent only (see PLAN.md section 2).

CREATE TABLE IF NOT EXISTS agents (
    id               TEXT PRIMARY KEY,
    name             TEXT NOT NULL,
    enabled          INTEGER NOT NULL DEFAULT 1,
    global_dirs      TEXT NOT NULL DEFAULT '[]',  -- JSON array of absolute paths
    project_dirs     TEXT NOT NULL DEFAULT '[]',  -- JSON array of relative paths
    supports_symlink INTEGER NOT NULL DEFAULT 1,
    created_at       TEXT NOT NULL,
    updated_at       TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS projects (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    root          TEXT NOT NULL UNIQUE,
    name          TEXT NOT NULL,
    registered_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS skills (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    agent_id        TEXT NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    project_id      INTEGER REFERENCES projects(id) ON DELETE CASCADE,
    scope           TEXT NOT NULL,                 -- 'global' or 'project:<root>'
    path            TEXT NOT NULL,
    name            TEXT NOT NULL,
    description     TEXT NOT NULL DEFAULT '',
    version         TEXT NOT NULL DEFAULT '',
    content_hash    TEXT NOT NULL DEFAULT '',
    is_symlink      INTEGER NOT NULL DEFAULT 0,
    link_target     TEXT NOT NULL DEFAULT '',
    state           TEXT NOT NULL DEFAULT 'enabled', -- 'enabled' | 'disabled'
    quarantine_path TEXT NOT NULL DEFAULT '',
    vault_ref       TEXT NOT NULL DEFAULT '',
    last_seen_at    TEXT NOT NULL,
    UNIQUE (agent_id, project_id, path)
);

CREATE INDEX IF NOT EXISTS idx_skills_agent_scope ON skills(agent_id, scope);
CREATE INDEX IF NOT EXISTS idx_skills_name ON skills(name);

CREATE TABLE IF NOT EXISTS vault (
    name           TEXT PRIMARY KEY,
    path           TEXT NOT NULL,
    content_hash   TEXT NOT NULL DEFAULT '',
    source_type    TEXT NOT NULL DEFAULT '',
    source_ref     TEXT NOT NULL DEFAULT '',
    source_url     TEXT NOT NULL DEFAULT '',
    source_subpath TEXT NOT NULL DEFAULT '',
    source_hash    TEXT NOT NULL DEFAULT '',
    spec_valid     INTEGER NOT NULL DEFAULT 1,
    spec_issues    TEXT NOT NULL DEFAULT '[]',     -- JSON array of strings
    installed_at   TEXT NOT NULL,
    updated_at     TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS scans (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    started_at   TEXT NOT NULL,
    finished_at  TEXT,
    agent_id     TEXT,
    project_id   INTEGER,
    skills_found INTEGER NOT NULL DEFAULT 0,
    errors       TEXT NOT NULL DEFAULT '[]'        -- JSON array of strings
);

CREATE TABLE IF NOT EXISTS settings (
    key        TEXT PRIMARY KEY,
    value      TEXT NOT NULL,
    updated_at TEXT NOT NULL
);
