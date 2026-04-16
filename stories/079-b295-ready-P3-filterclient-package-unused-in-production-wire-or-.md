---
id: 079-b295
title: FilterClient package unused in production — wire or delete
status: ready
priority: P3
created: "2026-04-16T20:32:52.688Z"
updated: "2026-04-16T20:33:39.065Z"
dependencies: []
---

# FilterClient package unused in production — wire or delete

## Problem Statement

NewFilterClient and all concrete filter types in internal/client/filter/ are only referenced from their own test files. No production wiring exists. The ctx field in FilterClient is stored but never read.

## Acceptance Criteria

- [ ] Either wire FilterClient into a real use case in main.go
- [ ] Or delete internal/client/filter/filter.go and remove unused ctx field

## Files

- internal/client/filter/filter.go
- cmd/server/main.go

## Work Log

