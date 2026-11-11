# Compatibility Model

CompatLab compares two normalized API versions in three stages: **diff** (raw changes), **classification** (rules) and **scoring**.

## Directions
- **BACKWARD**: can existing consumers keep working against the new provider?
- **FORWARD**: can consumers written for the new API work against the old provider?

Every change is classified once per direction and the directions are never merged into one verdict.

## Classifications
- `BREAKING`: likely to break consumers.
- `NON_BREAKING`: safe within the scope of the rule.
- `CONDITIONAL`: depends on how the consumer uses the API (for example strict response validation).

## Change types
`ENDPOINT_ADDED`, `ENDPOINT_REMOVED`, `METHOD_REMOVED`, `PARAMETER_ADDED`, `PARAMETER_REMOVED`, `REQUIRED_CHANGED`, `TYPE_CHANGED`, `FIELD_ADDED`, `FIELD_REMOVED`, `ENUM_CHANGED`, `STATUS_CODE_ADDED`, `STATUS_CODE_REMOVED`, `CONSTRAINT_CHANGED`, `AUTH_CHANGED`.

## Rule registry (backward direction)

| Rule | Change | Backward result |
|---|---|---|
| EP-001 | Endpoint removed | BREAKING (high) |
| EP-002 | Endpoint added | NON_BREAKING |
| EP-003 | HTTP method removed | BREAKING (high) |
| PAR-001 | Parameter removed | CONDITIONAL |
| PAR-002 | Parameter added | BREAKING if required, else NON_BREAKING |
| PAR-003 | Required flag changed | BREAKING when becoming required, else NON_BREAKING |
| REQ-001 | Request field added | BREAKING if required, else NON_BREAKING |
| REQ-002 | Request field removed | CONDITIONAL |
| REQ-004 | Request type changed | BREAKING (integer to number is NON_BREAKING) |
| REQ-005 | Request enum changed | BREAKING if values removed, else NON_BREAKING |
| REQ-006 | Request constraint changed | BREAKING if tightened, NON_BREAKING if relaxed |
| RES-001 | Response field removed | BREAKING |
| RES-002 | Response field added | CONDITIONAL |
| RES-003 | Response required flag changed | BREAKING when becoming optional |
| RES-004 | Response type changed | BREAKING (integer to number is CONDITIONAL) |
| RES-005 | Response enum changed | CONDITIONAL if values added |
| RES-006 | Response constraint changed | BREAKING if nullable is added; CONDITIONAL if relaxed |
| RES-007 | Response status removed | BREAKING for 2xx, else CONDITIONAL |
| RES-008 | Response status added | CONDITIONAL |
| SEC-001 | Authentication changed | BREAKING if added; CONDITIONAL if schemes or scopes are added |

Forward verdicts are defined in the same rule files. Changes with no matching rule are reported as `GEN-000` (CONDITIONAL, HIGH) so nothing is silently ignored.

## Path matching
Routes are compared by structure: `/users/{id}` and `/users/{userId}` are the same endpoint, and path parameters are matched by position.

## Enums
A missing enum means "any value". Adding an enum counts as removing every other value (`removed="*"`), and dropping one counts as adding any value.

## Score
`CompatibilityScores` (backward and forward, 0-100) is a CompatLab-defined indicator, not an industry standard. Penalty per change: severity weight (LOW 2, MEDIUM 5, HIGH 10, CRITICAL 20), halved for CONDITIONAL, zero for NON_BREAKING. Any BREAKING change caps the score of its direction at 89, and a BREAKING change of HIGH or CRITICAL severity caps it at 69, so a serious change cannot be averaged away.

## Determinism
Specifications are normalized (sorted collections), change IDs are hashes of change coordinates, results are sorted, and analyses carry no timestamps. The same inputs always produce byte-identical JSON.

## Limitations
See [ADR-0002](../design-decisions/ADR-0002-openapi-parser.md) for unsupported OpenAPI features. Rules reflect common consumer behaviour; real consumers may differ, which is why contracts matter.
