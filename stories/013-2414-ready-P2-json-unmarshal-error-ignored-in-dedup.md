---
id: "013-2414"
status: ready
priority: P2
created: "2026-04-16T00:00:00Z"
source: code-review
---
# json.Unmarshal error silently dropped in marshalFrom

## Description
`marshalFrom` calls `json.Unmarshal(bytes, destination)` and discards the error. If unmarshal fails, `destination` is left in a zero-value state causing all messages to hash identically — deduplication then incorrectly suppresses every unique message as a duplicate.

## Acceptance Criteria
- [ ] Unmarshal error is checked and returned wrapped with `%w`
- [ ] Caller `stripTimestampFromMessage` propagates the error

## Context Files
- `internal/client/dedupe/dedupe.go:60-74` — `marshalFrom` and `stripTimestampFromMessage`
