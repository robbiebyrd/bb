---
id: "029-225b"
status: ready
priority: P2
created: "2026-04-16T00:00:00Z"
source: code-review
---
# Deduper interface has no implementation and no callers — delete it

## Description
`models.Deduper` defines `Enabled()`, `Enable()`, `Disable()`, `Filter()`. Nothing implements it; `DedupeFilterClient` implements `FilterInterface` instead. The interface exists as speculative API surface.

## Acceptance Criteria
- [ ] `internal/models/dedupe.go` deleted
- [ ] No compilation errors after deletion

## Context Files
- `internal/models/dedupe.go` — the entire file to delete
- `internal/client/dedupe/dedupe.go` — implements FilterInterface, not Deduper
