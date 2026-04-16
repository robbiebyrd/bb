---
id: "030-3f0c"
status: ready
priority: P2
created: "2026-04-16T00:00:00Z"
source: code-review
---
# CSVCanMessage struct never used — delete models.go in csv package

## Description
`CSVCanMessage` is the only type in `internal/output/csv/models.go`. The CSV client builds `[]string` rows directly in `Handle()` and never uses this struct. The file is dead weight with mismatched struct tags (`lp:` InfluxDB tags on a CSV type).

## Acceptance Criteria
- [ ] `internal/output/csv/models.go` deleted
- [ ] No compilation errors after deletion

## Context Files
- `internal/output/csv/models.go` — the entire file to delete
- `internal/output/csv/client.go:56-66` — Handle() that builds rows without using the struct
