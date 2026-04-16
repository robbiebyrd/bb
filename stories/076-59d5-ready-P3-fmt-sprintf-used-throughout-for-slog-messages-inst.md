---
id: 076-59d5
title: fmt.Sprintf used throughout for slog messages instead of key-value args
status: ready
priority: P3
created: "2026-04-16T20:32:52.687Z"
updated: "2026-04-16T20:33:39.051Z"
dependencies: []
---

# fmt.Sprintf used throughout for slog messages instead of key-value args

## Problem Statement

c.l.Debug(fmt.Sprintf(worker %v started, i)) defeats structured logging — values embedded in message string cannot be queried by log aggregators. Should use slog key-value pairs.

## Acceptance Criteria

- [ ] All slog calls using fmt.Sprintf in the message converted to structured key-value args
- [ ] Affects: influxdb/client.go, app/app.go, mqtt/client.go, simulate/connection.go

## Files

- internal/output/influxdb/client.go
- internal/app/app.go
- internal/output/mqtt/client.go
- internal/connection/simulate/connection.go

## Work Log

