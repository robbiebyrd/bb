---
id: "006-6643"
status: complete
priority: P1
created: "2026-04-16T00:00:00Z"
source: code-review
updated: 2026-04-16
---
# InfluxDB token serialized into debug log at startup

## Description
`config.ToJSON()` marshals the full `Config` struct including `InfluxDB.Token`. With `LOG_LEVEL=debug`, the bearer token appears in plaintext in any log aggregator. An attacker with read access to logs obtains the credential.

## Acceptance Criteria
- [ ] `InfluxDB.Token` is redacted (shown as `[REDACTED]`) in all log output
- [ ] A custom `MarshalJSON` on `InfluxDBConfig` or a sanitization pass before logging is in place
- [ ] Verify with `LOG_LEVEL=debug`: token must not appear in output

## Context Files
- `internal/config/args.go:26-41` — `ToJSON` that marshals the full config
- `internal/app/app.go:42-43` — where the JSON is logged
- `internal/models/config.go:3-12` — `InfluxDBConfig` struct with `Token` field
