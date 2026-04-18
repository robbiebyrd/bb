---
id: 001-f989
title: "018-9280: Add ctx.Done() exit path to SocketCAN receiver retry loop"
status: complete
priority: P2
created: "2026-04-17T19:34:34.244Z"
updated: "2026-04-17T19:35:00.665Z"
dependencies: []
---

# 018-9280: Add ctx.Done() exit path to SocketCAN receiver retry loop

## Problem Statement

Replace time.Sleep(100ms) in socketcan/receiver.go with a select that also checks scc.ctx.Done(). Also add ctx.Done() guard around the channel send.

## Acceptance Criteria

- [ ] Implement as described

## QA

None

## Work Log

### 2026-04-17T19:35:00.593Z - Completed

