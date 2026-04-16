---
id: "028-0b5f"
status: ready
priority: P2
created: "2026-04-16T00:00:00Z"
source: code-review
---
# OutputClients slice never populated; RemoveOutput/RemoveOutputs always no-op

## Description
`AddOutput` registers goroutines and a broadcast listener but never appends to `b.OutputClients`. `RemoveOutput` and `RemoveOutputs` loop over an always-empty slice and do nothing. The `AppInterface` methods are dead code.

## Acceptance Criteria
- [ ] Either: append to `b.OutputClients` in `AddOutput` and fix Remove to actually remove broadcast listeners
- [ ] Or: delete `OutputClients` field, `RemoveOutput`, and `RemoveOutputs` from both `AppData` and `AppInterface` (YAGNI if not needed now)

## Context Files
- `internal/app/app.go:104-136` — AddOutput and Remove methods
- `internal/models/app.go:10-11` — interface declarations
