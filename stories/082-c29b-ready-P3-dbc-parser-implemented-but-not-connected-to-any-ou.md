---
id: 082-c29b
title: DBC parser implemented but not connected to any output pipeline
status: ready
priority: P3
created: "2026-04-16T20:32:52.689Z"
updated: "2026-04-16T20:33:39.067Z"
dependencies: []
---

# DBC parser implemented but not connected to any output pipeline

## Problem Statement

DBCParserClient is fully implemented with tests but never constructed or wired in main.go or any output client. Architectural question of where parsing belongs is unresolved.

## Acceptance Criteria

- [ ] Design decision documented: where in the pipeline does DBC decoding happen?
- [ ] If wiring deferred: note as intentionally incomplete in CLAUDE.md
- [ ] If wiring implemented: parser integrated into at least one output path

## Files

- internal/parser/dbc/dbc.go
- internal/models/parser.go
- cmd/server/main.go

## Work Log

