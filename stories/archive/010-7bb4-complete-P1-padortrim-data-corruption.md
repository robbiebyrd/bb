---
id: 010-7bb4
status: complete
priority: P1
created: "2026-04-16T00:00:00Z"
source: code-review
updated: "2026-04-16T20:39:38.553Z"
---
# PadOrTrim silently truncates data when input length > size/2

## Description
`copy(tmp[:size-l], bb)` copies only `size-l` bytes. For `PadOrTrim([]byte{1,2,3,4,5}, 8)`, only 3 bytes are copied; bytes 4-5 are silently lost. The current call sites happen not to trigger this (socketcan always passes 8 bytes; sim passes ≤ size), but the function's contract is broken for any padding case where `l > size/2`.

## Acceptance Criteria
- [ ] Change `copy(tmp[:size-l], bb)` to `copy(tmp, bb)`
- [ ] Tests added for PadOrTrim: exact-fit, trim (l > size), pad-where-l<size/2, pad-where-l>size/2

## Context Files
- `internal/client/common/utils.go:5-13` — the PadOrTrim function
- `internal/client/common/utils_test.go` — add tests here

## Work Log

### 2026-04-16T20:39:38.512Z - Completed by parallel agent - see review file for details

