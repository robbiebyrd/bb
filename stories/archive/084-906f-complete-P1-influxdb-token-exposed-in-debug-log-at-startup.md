---
id: 084-906f
title: InfluxDB token exposed in debug log at startup
status: complete
priority: P1
created: "2026-04-16T20:36:34.551Z"
updated: "2026-04-16T20:41:40.327Z"
dependencies: []
---

# InfluxDB token exposed in debug log at startup

## Problem Statement

config.ToJSON() marshals the full Config struct including InfluxDB.Token. With LOG_LEVEL=debug, the bearer token appears in plaintext in any log aggregator. An attacker with read access to logs obtains the credential.

## Acceptance Criteria

- [ ] Token is redacted ([REDACTED]) in all log output
- [ ] Custom MarshalJSON on InfluxDBConfig or sanitization pass before logging
- [ ] Verify with LOG_LEVEL=debug: token does not appear in output

## Files

- internal/config/args.go
- internal/app/app.go
- internal/models/config.go

## Work Log

### 2026-04-16T20:41:40.285Z - Fixed: redact InfluxDB token in debug log via ToJSON sanitization in config/args.go

