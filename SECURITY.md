# Security Policy

## Reporting a vulnerability
Open a private security advisory on GitHub (Security > Advisories). Do not file public issues for vulnerabilities.

## Threat model summary
- **Untrusted specifications:** size and nesting limits, alias-bomb protection, no external `$ref`, bounded reference chains.
- **Outbound requests:** the test runner contacts only hosts on an operator-defined allowlist, blocks link-local addresses (cloud metadata) at connect time, re-checks redirects, ignores proxy variables and caps request count, timeouts and response size. Runs default to an in-process mock.
- **Accounts:** bcrypt hashes, HS256 tokens with pinned algorithm and expiry, admin/member roles, strict tenant isolation, rate limits (stricter on register/login), request-size limits.
- **Data:** parameterized SQL only (guarded by a static test); recordings mask tokens, passwords and personal data; internal errors are never returned to clients.
- **Secrets:** never committed. `.env` is git-ignored; `docker compose` refuses to start without `POSTGRES_PASSWORD` and `JWT_SECRET`.
- **Dependencies:** `govulncheck` and `npm audit` run in CI.

Known limitations are listed in `docs/reports/limitations.md`.
