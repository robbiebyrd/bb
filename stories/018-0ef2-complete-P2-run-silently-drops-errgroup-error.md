---
id: 018-0ef2
status: complete
priority: P2
created: "2026-04-16T00:00:00Z"
source: code-review
updated: "2026-04-16T21:07:12.140Z"
---
# AppInterface.Run() returns nothing, silently drops errgroup errors

## Description
`Run()` returns nothing. `b.wgClients.Wait()` returns an error that is discarded. Any goroutine failure (broadcast, connections, output clients) is invisible. `main()` cannot detect that the system exited due to an error.

## Acceptance Criteria
- [ ] `AppInterface.Run()` changed to `Run() error`
- [ ] `AppData.Run()` returns `b.wgClients.Wait()`
- [ ] `main.go` handles the returned error (log and exit with non-zero code)

## Context Files
- `internal/models/app.go:13` — interface declaration
- `internal/app/app.go:139-148` — concrete Run()
- `cmd/server/main.go` — caller to update

## Work Log

### 2026-04-16T21:07:12.091Z - Fixed: cross-cutting refactor in wave 2

