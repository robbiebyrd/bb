---
id: 012-4ef6
status: complete
priority: P1
created: "2026-04-16T00:00:00Z"
source: code-review
updated: "2026-04-16T20:41:40.892Z"
---
# Warn-level log with fmt.Sprintf allocation on every CAN frame in InfluxDB Handle()

## Description
`c.l.Warn(fmt.Sprintf("current count %v", len(c.messageBlock)))` fires on every incoming CAN frame. At 100 msg/sec this is 100 formatted string allocations per second at Warn level (which is always active in production). This is leftover debug scaffolding.

## Acceptance Criteria
- [ ] Line removed entirely (or demoted to `c.l.Debug("message block count", "count", len(c.messageBlock))`)
- [ ] No Warn-level logging in the hot message-handling path

## Context Files
- `internal/output/influxdb/client.go:71` — the offending log line

## Work Log

### 2026-04-16T20:41:40.850Z - Fixed: replaced Warn+fmt.Sprintf per frame with Debug using structured key-value args

