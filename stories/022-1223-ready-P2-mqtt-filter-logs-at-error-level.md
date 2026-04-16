---
id: "022-1223"
status: ready
priority: P2
created: "2026-04-16T00:00:00Z"
source: code-review
---
# MQTT shouldFilterMessage logs normal dedup activity at ERROR level

## Description
`c.l.Error("skipping")` fires every time a message is deduplicated — normal, expected, high-frequency behavior. This floods error-level logs, masks real errors, and trains operators to ignore errors.

## Acceptance Criteria
- [ ] Remove `c.l.Error("skipping")` — the caller already logs at Debug level with full context
- [ ] No ERROR-level log emitted for normal filter/dedup activity

## Context Files
- `internal/output/mqtt/client.go:118` — the misclassified log
- `internal/output/mqtt/client.go:73-76` — the caller that already logs correctly at Debug
