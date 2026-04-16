---
id: 005-7986
status: complete
priority: P1
created: "2026-04-16T00:00:00Z"
source: code-review
updated: "2026-04-16T20:39:38.341Z"
---
# Broadcast blocking fan-out: slow consumer stalls all other outputs

## Description
`Broadcast()` sends to each output channel with a plain blocking send. All three outputs (InfluxDB, MQTT, CSV) are served serially from one goroutine. A slow MQTT broker or full InfluxDB queue blocks CSV and the incoming CAN channel.

## Acceptance Criteria
- [ ] Non-blocking send with drop-and-log, OR each output dispatched in its own goroutine
- [ ] A slow/blocked output does not delay messages to other outputs
- [ ] Dropped messages are logged with output name and count

## Context Files
- `internal/client/broadcast/broadcast.go:76-86` — the blocking fan-out loop

## Work Log

### 2026-04-16T20:39:38.298Z - Completed by parallel agent - see review file for details

