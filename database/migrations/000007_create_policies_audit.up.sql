CREATE TABLE release_policies (
    id         TEXT PRIMARY KEY,
    project_id TEXT NOT NULL REFERENCES api_projects(id) ON DELETE CASCADE,
    payload    JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX release_policies_project_idx ON release_policies (project_id, created_at);

CREATE TABLE audit_events (
    id          TEXT PRIMARY KEY,
    org_id      TEXT NOT NULL,
    actor_id    TEXT NOT NULL,
    action      TEXT NOT NULL,
    resource    TEXT NOT NULL,
    resource_id TEXT NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX audit_events_org_idx ON audit_events (org_id, created_at);
