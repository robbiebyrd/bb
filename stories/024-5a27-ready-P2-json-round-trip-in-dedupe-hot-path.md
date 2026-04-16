---
id: "024-5a27"
status: ready
priority: P2
created: "2026-04-16T00:00:00Z"
source: code-review
---
# JSON round-trip (marshal + unmarshal) to strip timestamp on every dedupe check

## Description
`hashCanMessageData` calls `stripTimestampFromMessage` which JSON-marshals then JSON-unmarshals purely to exclude the `Timestamp` field before hashing. Two heap allocations plus reflection-based encoding on every deduplicated message. A direct struct copy is orders of magnitude cheaper.

## Acceptance Criteria
- [ ] `stripTimestampFromMessage` constructs `CanMessageData` by directly copying fields from `CanMessageTimestamped`
- [ ] `marshalFrom` function can be deleted
- [ ] No JSON marshal/unmarshal in the dedup hot path

## Context Files
- `internal/client/dedupe/dedupe.go:60-88` — marshalFrom, stripTimestampFromMessage, hashCanMessageData
- `internal/models/can.go:18-26` — CanMessageData fields to copy
