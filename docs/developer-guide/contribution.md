# Contributing Workflow

Branch from `main`, use Conventional Commits, keep commits small and meaningful. A change is ready when `./scripts/dev.ps1 check` and (for the dashboard) `npm test` pass, new behaviour has tests, and documentation or an ADR is updated when behaviour or architecture changes. Never commit `.env`, `node_modules`, build output or credentials.
