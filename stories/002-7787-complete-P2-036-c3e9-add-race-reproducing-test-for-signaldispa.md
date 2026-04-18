---
id: 002-7787
title: "036-c3e9: Add race-reproducing test for SignalDispatcher concurrent AddListener/Dispatch"
status: complete
priority: P2
created: "2026-04-17T19:34:39.795Z"
updated: "2026-04-17T19:35:14.323Z"
dependencies: []
---

# 036-c3e9: Add race-reproducing test for SignalDispatcher concurrent AddListener/Dispatch

## Problem Statement

Add TestDispatch_ConcurrentAddListenerNoRace to dispatcher_test.go documenting that AddListener before Dispatch is safe.

## Acceptance Criteria

- [ ] Implement as described

## QA

None

## Work Log

### 2026-04-17T19:35:14.261Z - Completed

