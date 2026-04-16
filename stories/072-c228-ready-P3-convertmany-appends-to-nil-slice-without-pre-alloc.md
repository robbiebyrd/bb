---
id: 072-c228
title: convertMany appends to nil slice without pre-allocation
status: ready
priority: P3
created: "2026-04-16T20:32:52.684Z"
updated: "2026-04-16T20:33:39.049Z"
dependencies: []
---

# convertMany appends to nil slice without pre-allocation

## Problem Statement

var convertedMessages []InfluxDBCanMessage grows via repeated reallocation for 1000-message blocks, causing ~10 allocations per batch flush.

## Acceptance Criteria

- [ ] convertedMessages pre-allocated with make([]InfluxDBCanMessage, 0, len(msgs))

## Files

- internal/output/influxdb/client.go

## Work Log

