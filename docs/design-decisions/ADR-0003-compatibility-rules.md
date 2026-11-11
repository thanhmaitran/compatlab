# ADR-0003: Rule-based compatibility semantics with separate directions

- Status: Accepted
- Date: 2026-11-04

## Decision
Compatibility is decided by a rule registry, not scattered conditionals. Each rule has an ID, trigger, direction, classification, severity, explanation, remediation and tests. Backward (can the new provider serve old consumers?) and forward (can new consumers use the old provider?) are classified separately. Results are BREAKING, NON_BREAKING or CONDITIONAL. A score, if shown, is CompatLab-defined and never hides a breaking change.

## Consequences
Rules are testable in isolation and extensible. Reports are richer than a single score. Rules and Schemathesis/Pact-style concepts are implemented in-house; those tools are references, not runtime dependencies.
