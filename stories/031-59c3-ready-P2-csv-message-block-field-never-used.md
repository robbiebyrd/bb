---
id: "031-59c3"
status: ready
priority: P2
created: "2026-04-16T00:00:00Z"
source: code-review
---
# CSVClient.messageBlock field initialized but never used

## Description
`messageBlock []canModels.CanMessageTimestamped` is allocated in the constructor and never appended to or read. It appears to be a copy-paste artifact from `InfluxDBClient`. It misleads readers into thinking CSV output is buffered.

## Acceptance Criteria
- [ ] `messageBlock` field removed from `CSVClient` struct
- [ ] `messageBlock: []canModels.CanMessageTimestamped{}` removed from constructor

## Context Files
- `internal/output/csv/client.go:15-45` — struct and constructor
