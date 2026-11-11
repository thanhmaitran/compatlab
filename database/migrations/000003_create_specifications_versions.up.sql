CREATE TABLE api_specifications (
    id         TEXT PRIMARY KEY,
    project_id TEXT NOT NULL REFERENCES api_projects(id) ON DELETE CASCADE,
    filename   TEXT NOT NULL,
    format     TEXT NOT NULL CHECK (format IN ('yaml', 'json')),
    hash       TEXT NOT NULL,
    status     TEXT NOT NULL,
    content    BYTEA NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX api_specifications_project_idx ON api_specifications (project_id);

CREATE TABLE api_versions (
    id               TEXT PRIMARY KEY,
    project_id       TEXT NOT NULL REFERENCES api_projects(id) ON DELETE CASCADE,
    label            TEXT NOT NULL,
    specification_id TEXT NOT NULL REFERENCES api_specifications(id) ON DELETE CASCADE,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (project_id, label)
);
