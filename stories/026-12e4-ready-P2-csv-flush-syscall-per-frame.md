---
id: "026-12e4"
status: ready
priority: P2
created: "2026-04-16T00:00:00Z"
source: code-review
---
# CSV writer.Flush() called on every message — one syscall per CAN frame

## Description
`c.w.Flush()` is called after every single row. The `csv.Writer` buffers internally; this per-message flush defeats that buffering entirely, issuing an OS write syscall on every frame (~100/sec per interface).

## Acceptance Criteria
- [ ] Per-message `Flush()` removed from `Handle()`
- [ ] Flush runs on a periodic ticker (e.g., 500ms) or on graceful shutdown
- [ ] CSV data is not lost on process termination (final flush on shutdown)

## Context Files
- `internal/output/csv/client.go:56-67` — Handle() with per-message flush
