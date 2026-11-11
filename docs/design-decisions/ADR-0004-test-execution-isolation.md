# ADR-0004: Test execution isolation

- Status: Accepted
- Date: 2026-11-04

## Decision
The test engine may only send requests to explicitly allowed targets (allowlist), with per-request timeouts, request-count limits and blocking of internal or link-local addresses unless explicitly permitted. Mock servers are the default target. Replay data has tokens and personal data masked.

## Consequences
CompatLab cannot be used as a scanner against arbitrary systems. Some legitimate internal targets require explicit configuration.
