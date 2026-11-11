# Policy Engine

Policies are version-controlled YAML (`configs/release-policy.yaml`):

```yaml
compatlab:
  policy:
    fail_on:
      - BREAKING
    require_contract_tests: false
    minimum_pass_rate: 0
    include_forward: false
```

| Condition | Effect |
|---|---|
| A classification in `fail_on` has changes | BLOCK |
| Other BREAKING or CONDITIONAL changes exist | WARN |
| `require_contract_tests` and no contracts verified, or a contract fails | BLOCK (WARN for failures when not required) |
| `minimum_pass_rate` > 0 and the test pass rate is lower | BLOCK (WARN if no test run exists) |

Only backward-direction changes are evaluated unless `include_forward` is true. Decisions: `PASS`, `WARN`, `BLOCK`.
