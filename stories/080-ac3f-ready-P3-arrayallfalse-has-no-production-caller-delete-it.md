---
id: 080-ac3f
title: ArrayAllFalse has no production caller — delete it
status: ready
priority: P3
created: "2026-04-16T20:32:52.688Z"
updated: "2026-04-16T20:33:39.066Z"
dependencies: []
---

# ArrayAllFalse has no production caller — delete it

## Problem Statement

ArrayAllFalse is called only from its own unit test. No production code uses it.

## Acceptance Criteria

- [ ] ArrayAllFalse removed from utils.go
- [ ] TestArrayAllFalse removed from utils_test.go

## Files

- internal/client/common/utils.go
- internal/client/common/utils_test.go

## Work Log

