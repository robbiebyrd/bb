---
id: "034-5a83"
status: ready
priority: P2
created: "2026-04-16T00:00:00Z"
source: code-review
---
# Four filter types (Transmit, DataLength, Remote, Data) have zero tests

## Description
`CanTransmitFilter`, `CanDataLengthFilter`, `CanRemoteFilter`, and `CanDataFilter` each have multi-branch switch logic. None are tested. Given that `CanDataFilter` has a confirmed AND-logic bug (story 009), other filter types may have similar bugs.

## Acceptance Criteria
- [ ] Tests for all operators in `CanTransmitFilter` (TXOnly, RXOnly, TXAndRX)
- [ ] Tests for all operators in `CanDataLengthFilter` (GT, LT, EQ, NEQ)
- [ ] Tests for all operators in `CanRemoteFilter` (Include, Exclude, Only)
- [ ] Tests for `CanDataFilter` with both AND and OR operators, including multi-condition cases

## Context Files
- `internal/client/filter/types.go` — all four types
- `internal/client/filter/types_test.go` — add tests here
