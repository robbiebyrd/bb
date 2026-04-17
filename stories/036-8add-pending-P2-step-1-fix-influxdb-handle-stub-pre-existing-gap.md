---
id: "036-8add"
title: "Step 1: Fix InfluxDB Handle() stub (pre-existing gap)"
status: pending
priority: P2
created: 2026-04-17T17:16:35.449Z
updated: 2026-04-17T17:16:35.449Z
dependencies: []
---

# Step 1: Fix InfluxDB Handle() stub (pre-existing gap)

## Problem Statement

Add no-op HandleCanMessage() stub to InfluxDBClient in internal/output/influxdb/client.go. The InfluxDB client intentionally bypasses per-message handling (uses ticker-based batching in HandleCanMessageChannel instead). The stub satisfies the OutputClient interface after the rename in Step 2.

## Acceptance Criteria

- [ ] Implement as described

## Work Log

