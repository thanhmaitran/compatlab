CREATE TABLE analysis_runs (
    id         TEXT PRIMARY KEY,
    project_id TEXT NOT NULL REFERENCES api_projects(id) ON DELETE CASCADE,
    payload    JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX analysis_runs_project_idx ON analysis_runs (project_id, created_at);

CREATE TABLE change_records (
    id             BIGSERIAL PRIMARY KEY,
    analysis_id    TEXT NOT NULL REFERENCES analysis_runs(id) ON DELETE CASCADE,
    project_id     TEXT NOT NULL REFERENCES api_projects(id) ON DELETE CASCADE,
    change_id      TEXT NOT NULL,
    rule_id        TEXT NOT NULL,
    change_type    TEXT NOT NULL,
    classification TEXT NOT NULL,
    severity       TEXT NOT NULL,
    direction      TEXT NOT NULL,
    method         TEXT NOT NULL,
    path           TEXT NOT NULL,
    location       TEXT NOT NULL
);
CREATE INDEX change_records_project_idx ON change_records (project_id, direction, path, method);

CREATE TABLE compatibility_reports (
    id         TEXT PRIMARY KEY,
    project_id TEXT NOT NULL REFERENCES api_projects(id) ON DELETE CASCADE,
    payload    JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX compatibility_reports_project_idx ON compatibility_reports (project_id, created_at);
