---
id: 078-2af8
title: BroadcastClient.AddMany() never called — YAGNI
status: ready
priority: P3
created: "2026-04-16T20:32:52.687Z"
updated: "2026-04-16T20:33:39.064Z"
dependencies: []
---

# BroadcastClient.AddMany() never called — YAGNI

## Problem Statement

AddMany is defined but has zero call sites in the entire codebase. Speculative convenience wrapper.

## Acceptance Criteria

- [ ] AddMany method deleted from BroadcastClient

## Files

- internal/client/broadcast/broadcast.go

## Work Log

