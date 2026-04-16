---
id: "020-2c94"
status: ready
priority: P2
created: "2026-04-16T00:00:00Z"
source: code-review
---
# *context.Context (pointer to interface) anti-pattern used throughout codebase

## Description
`*context.Context` is passed everywhere: `AppInterface.GetContext()`, every output client constructor, connection manager, broadcast client. Pointers to interfaces add indirection with no benefit and make nil checks ambiguous. `context.Context` is already an interface and should be passed by value.

## Acceptance Criteria
- [ ] All `*context.Context` parameters and fields changed to `context.Context`
- [ ] `AppInterface.GetContext()` returns `context.Context`
- [ ] All constructors accept `context.Context` by value

## Context Files
- `internal/models/app.go:16` — interface method
- `internal/app/app.go:154-156` — GetContext implementation
- Every output client constructor
