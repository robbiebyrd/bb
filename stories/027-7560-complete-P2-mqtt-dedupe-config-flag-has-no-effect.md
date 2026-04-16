---
id: 027-7560
status: complete
priority: P2
created: "2026-04-16T00:00:00Z"
source: code-review
updated: "2026-04-16T21:07:12.495Z"
---
# MQTT_DEDUPE=false config flag has no effect — dedup is always active

## Description
`MQTTConfig.Dedupe bool` (default `true`) is never read. `main.go` unconditionally constructs `DedupeFilterClient`. Setting `MQTT_DEDUPE=false` does nothing.

## Acceptance Criteria
- [ ] `DedupeFilterClient` construction in `main.go` is gated behind `if cfg.MQTTConfig.Dedupe`
- [ ] Setting `MQTT_DEDUPE=false` disables deduplication for MQTT output

## Context Files
- `cmd/server/main.go:19-22` — unconditional dedupe construction
- `internal/models/config.go:20` — the unused `Dedupe` field

## Work Log

### 2026-04-16T21:07:12.450Z - Fixed: cross-cutting refactor in wave 2

