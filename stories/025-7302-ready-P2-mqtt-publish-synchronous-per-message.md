---
id: "025-7302"
status: ready
priority: P2
created: "2026-04-16T00:00:00Z"
source: code-review
---
# MQTT token.Wait() blocks synchronously per message, bottlenecks throughput

## Description
`token.Wait()` blocks for a full broker round-trip on every single published message. At 100 frames/sec with any broker latency, the MQTT goroutine becomes the bottleneck and its full channel (81920) starts filling.

## Acceptance Criteria
- [ ] For QoS 0: remove `token.Wait()` (no acknowledgment needed)
- [ ] For QoS > 0: use async callbacks or batched publish
- [ ] MQTT throughput matches incoming frame rate under normal conditions

## Context Files
- `internal/output/mqtt/client.go:80-85` — the blocking publish
- `internal/models/config.go:17` — Qos field
