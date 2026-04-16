---
id: 074-36fc
title: CAN_MESSAGE_MAX_DATA_LENGTH should be const not var
status: ready
priority: P3
created: "2026-04-16T20:32:52.686Z"
updated: "2026-04-16T20:33:39.050Z"
dependencies: []
---

# CAN_MESSAGE_MAX_DATA_LENGTH should be const not var

## Problem Statement

var CAN_MESSAGE_MAX_DATA_LENGTH uint8 = 8 is an exported mutable variable for an immutable value. Go convention requires const and CamelCase naming.

## Acceptance Criteria

- [ ] Changed to const canMessageMaxDataLength uint8 = 8 (unexported)
- [ ] All usages within simulate package updated

## Files

- internal/connection/simulate/connection.go

## Work Log

