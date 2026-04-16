---
id: 001-058a
status: complete
priority: P1
created: "2026-04-16T00:00:00Z"
source: code-review
updated: "2026-04-16T20:41:40.561Z"
---
# Data race: InfluxDB workers write shared fields without synchronization

## Description
`workerLastRan` and `count` in `InfluxDBClient` are written by every worker goroutine concurrently with no mutex or atomic protection. `Handle()` also reads `workerLastRan` from a different goroutine. This is a confirmed data race that will be flagged by `-race`.

## Acceptance Criteria
- [ ] `workerLastRan` and `count` are protected by a mutex or replaced with `atomic.Int64`
- [ ] `go test -race ./...` passes with no data race warnings

## Context Files
- `internal/output/influxdb/client.go:111-112` — the racy writes in `worker()`
- `internal/output/influxdb/client.go:72` — the racy read in `Handle()`

## Work Log

### 2026-04-16T20:41:40.517Z - Fixed: added sync.Mutex to InfluxDBClient guarding workerLastRan and count accesses

