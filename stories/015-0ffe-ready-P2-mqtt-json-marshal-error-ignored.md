---
id: "015-0ffe"
status: ready
priority: P2
created: "2026-04-16T00:00:00Z"
source: code-review
---
# MQTT ToJSON silently ignores json.Marshal error

## Description
`jsonBytes, _ := json.Marshal(a)` discards the marshal error. On failure `jsonBytes` is nil and an empty string `""` is published to the MQTT broker — consumers receive a valid but empty payload with no indication of the failure.

## Acceptance Criteria
- [ ] Marshal error is checked and logged; return empty string with log entry on failure
- [ ] Or change signature to `(string, error)` so callers can skip the publish

## Context Files
- `internal/output/mqtt/models.go:31` — the blank-identifier assignment
