CREATE TABLE test_suites (
    id         TEXT PRIMARY KEY,
    project_id TEXT NOT NULL REFERENCES api_projects(id) ON DELETE CASCADE,
    payload    JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX test_suites_project_idx ON test_suites (project_id, created_at);

CREATE TABLE test_runs (
    id         TEXT PRIMARY KEY,
    project_id TEXT NOT NULL REFERENCES api_projects(id) ON DELETE CASCADE,
    payload    JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX test_runs_project_idx ON test_runs (project_id, created_at);
