CREATE TABLE consumer_contracts (
    id         TEXT PRIMARY KEY,
    project_id TEXT NOT NULL REFERENCES api_projects(id) ON DELETE CASCADE,
    payload    JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX consumer_contracts_project_idx ON consumer_contracts (project_id, created_at);
