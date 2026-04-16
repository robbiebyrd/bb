---
id: 081-ec4a
title: FrameInterface and ReceiverInterface in models have no implementations
status: ready
priority: P3
created: "2026-04-16T20:32:52.689Z"
updated: "2026-04-16T20:33:39.067Z"
dependencies: []
---

# FrameInterface and ReceiverInterface in models have no implementations

## Problem Statement

Both FrameInterface and ReceiverInterface in internal/models/can.go have zero implementations and zero usage anywhere. Speculative interfaces with no value.

## Acceptance Criteria

- [ ] Both interface declarations deleted from internal/models/can.go

## Files

- internal/models/can.go

## Work Log

