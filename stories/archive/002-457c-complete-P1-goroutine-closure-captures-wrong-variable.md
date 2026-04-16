---
id: 002-457c
status: complete
priority: P1
created: "2026-04-16T00:00:00Z"
source: code-review
updated: "2026-04-16T20:41:40.670Z"
---
# Goroutine closure captures loop variable `i` instead of parameter `id`

## Description
In `InfluxDBClient.Run()`, workers are launched with `go func(id int) { c.worker(i) }(i)`. The closure uses the outer loop variable `i` instead of the parameter `id`. All workers call `c.worker()` with the same stale value of `i`.

## Acceptance Criteria
- [ ] Change `c.worker(i)` to `c.worker(id)` inside the goroutine closure
- [ ] Add `defer c.wg.Done()` inside the goroutine so the WaitGroup is properly balanced

## Context Files
- `internal/output/influxdb/client.go:124-128` — the broken goroutine launch loop

## Work Log

### 2026-04-16T20:41:40.629Z - Fixed: worker goroutine now captures stable id parameter instead of loop variable i

