---
id: "033-48dd"
status: ready
priority: P2
created: "2026-04-16T00:00:00Z"
source: code-review
---
# DedupeFilterClient missing tests for timeout expiry and non-watched IDs

## Description
Even after fixing the empty-ids bug, two critical behaviors are untested: (1) after `timeout` ms, an identical message should not be suppressed; (2) a message whose ID is not in the watched list is never suppressed. The timeout expiry path is the primary mechanism preventing permanent suppression of valid repeated messages.

## Acceptance Criteria
- [ ] Test: message with unwatched ID is never suppressed (even on repeated calls)
- [ ] Test: after timeout elapses, same message is allowed through again
- [ ] Test: within timeout window, duplicate is suppressed

## Context Files
- `internal/client/dedupe/dedupe_test.go` — add tests here
- `internal/client/dedupe/dedupe.go:31-57` — the code under test
