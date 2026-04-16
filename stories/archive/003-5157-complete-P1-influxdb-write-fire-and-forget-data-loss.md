---
id: 003-5157
status: complete
priority: P1
created: "2026-04-16T00:00:00Z"
source: code-review
updated: "2026-04-16T20:41:40.786Z"
---
# InfluxDB write() fire-and-forget goroutine silently discards all write errors

## Description
`write()` calls `go c.client.WriteData(...)` — the goroutine's error return is dropped and `write()` always returns `nil`. InfluxDB write failures (network errors, auth errors, schema mismatches) are completely invisible. The `panic(err)` in `worker()` that checks `write()`'s return is dead code.

## Acceptance Criteria
- [ ] Remove the `go` keyword from `write()` — run `WriteData` synchronously and return its error
- [ ] Worker logs the error on failure rather than panicking
- [ ] Write errors are visible in logs

## Context Files
- `internal/output/influxdb/client.go:161-170` — the `write()` function
- `internal/output/influxdb/client.go:107-109` — the `panic(err)` in worker that becomes meaningful after fix

## Work Log

### 2026-04-16T20:41:40.740Z - Fixed: write is now synchronous; removed fire-and-forget goroutine; errors logged

