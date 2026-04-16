---
id: "014-54f8"
status: ready
priority: P2
created: "2026-04-16T00:00:00Z"
source: code-review
---
# CSV Write and Flush errors silently ignored on every message

## Description
Both `c.w.Write(row)` and `c.w.Flush()` return errors that are never checked. Disk-full or broken file-descriptor conditions cause silent data loss with no indication in logs.

## Acceptance Criteria
- [ ] `Write` error is checked and logged
- [ ] `w.Error()` is checked after `Flush` and logged
- [ ] The header write in `NewClient` also checks and logs its error

## Context Files
- `internal/output/csv/client.go:56-66` — `Handle()` with unchecked Write/Flush
