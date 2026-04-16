---
id: "019-7249"
status: ready
priority: P2
created: "2026-04-16T00:00:00Z"
source: code-review
---
# panic() used for recoverable I/O errors across multiple components

## Description
`panic(err)` is used instead of returning errors in: `socketcan/connection.go` (Open/Close), `influxdb/client.go` (client init, worker write failure), `mqtt/client.go` (connect), `csv/client.go` (file open). Transient network or filesystem errors crash the entire process.

## Acceptance Criteria
- [ ] `socketcan.Open()` and `Close()` return `fmt.Errorf(...)` instead of panicking
- [ ] `influxdb.NewClient()` returns `(canModels.OutputClient, error)`
- [ ] `mqtt.NewClient()` returns `(canModels.OutputClient, error)`
- [ ] `csv.NewClient()` returns `(canModels.OutputClient, error)`
- [ ] Worker logs write errors instead of panicking
- [ ] `OutputClient` interface updated if constructor signatures change
- [ ] `main.go` updated to handle errors from constructors

## Context Files
- `internal/connection/socketcan/connection.go:96-113`
- `internal/output/influxdb/client.go:34-62`
- `internal/output/mqtt/client.go:24-55`
- `internal/output/csv/client.go:24-44`
- `internal/models/repo.go` — OutputClient interface
