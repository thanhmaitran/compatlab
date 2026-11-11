# Contract Format (v1.0)

A consumer contract is a JSON document. Unknown fields are rejected.

```json
{
  "contract_version": "1.0",
  "consumer": "mobile-app",
  "provider": "users-api",
  "interactions": [
    {
      "description": "Retrieve user",
      "request": { "method": "GET", "path": "/users/u-123" },
      "response": { "status": 200, "body": { "id": "u-123", "email": "user@example.com" } }
    }
  ]
}
```

## Verification
Each interaction passes only if the provider API (a) defines a matching endpoint (the most specific path template wins), (b) documents the expected status (exact, `NXX` or `default`), and (c) defines every field of the expected body with a compatible type. Fields not mentioned in the contract are ignored, so a consumer that reads only `name` is not affected by changes to `email`.

## Consumer impact
`contracts.Impact` links backward BREAKING and CONDITIONAL changes to consumers: an interaction must target the changed operation and, for response changes, the same status code and a field the contract references. Levels: `AFFECTED` (breaking) and `REVIEW` (conditional). Consumers without a contract are unknown and are not reported.

Contract bodies are examples, not production data; do not put real credentials or personal data in contracts.
