---
id: "017-7e36"
status: ready
priority: P2
created: "2026-04-16T00:00:00Z"
source: code-review
---
# SocketCAN receiver busy-spins at 100% CPU on connection error

## Description
When `receiver.Receive()` returns false (error/EOF), the outer `for {}` immediately re-spins without checking `receiver.Err()`. This burns 100% CPU when the CAN interface goes down and hides the root cause error.

## Acceptance Criteria
- [ ] After inner loop exits, check `scc.receiver.Err()`
- [ ] On error: log with structured logger and return (or add backoff before retry)
- [ ] CPU usage drops to near-zero when CAN interface is down

## Context Files
- `internal/connection/socketcan/receiver.go:13-34` — the Receive loop
