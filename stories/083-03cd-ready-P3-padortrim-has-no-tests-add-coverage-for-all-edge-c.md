---
id: 083-03cd
title: PadOrTrim has no tests — add coverage for all edge cases
status: ready
priority: P3
created: "2026-04-16T20:32:52.689Z"
updated: "2026-04-16T20:33:39.068Z"
dependencies: []
---

# PadOrTrim has no tests — add coverage for all edge cases

## Problem Statement

PadOrTrim has zero tests. The padding branch has a confirmed data corruption bug. Tests would have caught this. All edge cases need coverage.

## Acceptance Criteria

- [ ] Tests for: exact-fit, trim (l>size), pad where l<size/2, pad where l>size/2
- [ ] Note: fix story 045-0007 (PadOrTrim data corruption) first or combine into one PR

## Files

- internal/client/common/utils.go
- internal/client/common/utils_test.go

## Work Log

