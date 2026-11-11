-- Reference queries behind the analytics endpoints (see internal/persistence/postgres.go
-- and internal/analytics). Parameters: $1 = project id.

-- Endpoints that change most often (backward direction).
SELECT method, path, COUNT(*) AS changes
FROM change_records
WHERE project_id = $1 AND direction = 'BACKWARD'
GROUP BY method, path
ORDER BY changes DESC, path, method
LIMIT 10;

-- Breaking changes per month (backward direction).
SELECT date_trunc('month', a.created_at) AS month, COUNT(*) AS breaking_changes
FROM change_records c
JOIN analysis_runs a ON a.id = c.analysis_id
WHERE c.project_id = $1 AND c.direction = 'BACKWARD' AND c.classification = 'BREAKING'
GROUP BY month
ORDER BY month;

-- Classification distribution (backward direction).
SELECT classification, COUNT(*)
FROM change_records
WHERE project_id = $1 AND direction = 'BACKWARD'
GROUP BY classification;

-- Rules that fire most often.
SELECT rule_id, COUNT(*)
FROM change_records
WHERE project_id = $1
GROUP BY rule_id
ORDER BY COUNT(*) DESC;
