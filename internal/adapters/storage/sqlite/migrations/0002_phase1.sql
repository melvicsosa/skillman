-- Phase 1: skills get a stable text identity (sha1 of agent|scope|path) so
-- rows survive rescans, and a read_only flag for agent-managed skills such
-- as Claude plugins. The 0001 table was never populated (Phase 0 wrote no
-- skills) and its UNIQUE(agent_id, project_id, path) did not constrain
-- global rows because project_id is NULL there, so it is recreated.

DROP INDEX IF EXISTS idx_skills_agent_scope;
DROP INDEX IF EXISTS idx_skills_name;
DROP TABLE IF EXISTS skills;

CREATE TABLE skills (
    id              TEXT PRIMARY KEY,
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
    read_only       INTEGER NOT NULL DEFAULT 0,
    last_seen_at    TEXT NOT NULL,
    UNIQUE (agent_id, scope, path)
);

CREATE INDEX idx_skills_agent_scope ON skills(agent_id, scope);
CREATE INDEX idx_skills_name ON skills(name);
