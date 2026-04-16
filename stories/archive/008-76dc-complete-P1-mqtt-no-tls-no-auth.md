---
id: 008-76dc
status: complete
priority: P1
created: "2026-04-16T00:00:00Z"
source: code-review
updated: "2026-04-16T20:39:38.446Z"
---
# MQTT broker connection has no TLS and no authentication

## Description
`mqtt.NewClientOptions()` is created with no `SetTLSConfig`, `SetUsername`, or `SetPassword`. All vehicle CAN bus telemetry transmits in cleartext. Any host on the same network can subscribe to `#` and receive all data, or inject messages.

## Acceptance Criteria
- [ ] Add `MQTT_USERNAME` and `MQTT_PASSWORD` env vars (optional, used when set)
- [ ] Add `MQTT_TLS` bool env var; when true, configure TLS with system cert pool
- [ ] Document in CLAUDE.md

## Context Files
- `internal/output/mqtt/client.go:27-35` — client construction
- `internal/models/config.go:14-23` — `MQTTConfig` struct to extend

## Work Log

### 2026-04-16T20:39:38.406Z - Completed by parallel agent - see review file for details

