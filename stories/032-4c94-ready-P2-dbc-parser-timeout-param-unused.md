---
id: "032-4c94"
status: ready
priority: P2
created: "2026-04-16T00:00:00Z"
source: code-review
---
# NewDBCParserClient timeout parameter is accepted but never used

## Description
The `timeout int` parameter in `NewDBCParserClient` is discarded immediately — not stored, not passed anywhere. All call sites pass `0`. It creates false expectations about retry or deadline behavior.

## Acceptance Criteria
- [ ] `timeout int` parameter removed from `NewDBCParserClient` signature
- [ ] All four call sites in `dbc_test.go` updated to remove the `0` argument

## Context Files
- `internal/parser/dbc/dbc.go:24-34` — function signature and body
- `internal/parser/dbc/dbc_test.go` — four call sites to update
