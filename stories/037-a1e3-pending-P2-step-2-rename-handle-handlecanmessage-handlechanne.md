---
id: "037-a1e3"
title: "Step 2: Rename Handle→HandleCanMessage, HandleChannel→HandleCanMessageChannel"
status: pending
priority: P2
created: 2026-04-17T17:16:40.548Z
updated: 2026-04-17T17:16:40.548Z
dependencies: []
---

# Step 2: Rename Handle→HandleCanMessage, HandleChannel→HandleCanMessageChannel

## Problem Statement

Rename both methods across all 5 output clients (mqtt, csv, crtd, dbc, influxdb), the OutputClient interface in models/repo.go, app.go wiring call site, and all test files (csv_test, crtd_test, influxdb_test). Single atomic commit so repo is never partially renamed.

## Acceptance Criteria

- [ ] Implement as described

## Work Log

