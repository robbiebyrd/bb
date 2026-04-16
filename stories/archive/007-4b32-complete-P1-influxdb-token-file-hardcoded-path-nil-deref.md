---
id: 007-4b32
status: complete
priority: P1
created: "2026-04-16T00:00:00Z"
source: code-review
updated: "2026-04-16T20:41:40.444Z"
---
# Token file uses hardcoded path, ignores TokenFile config, panics on missing file

## Description
`os.Open("./config/influxdb/token.json")` hardcodes the path, ignoring `cfg.InfluxDB.TokenFile`. On `os.Open` failure: `fmt.Println(err)` is used (not the structured logger) and execution continues — `defer jsonFile.Close()` then panics on nil. Continuing with an empty token makes InfluxDB silently fail all writes.

## Acceptance Criteria
- [ ] Use `cfg.InfluxDB.TokenFile` as the actual path
- [ ] On open failure: log with structured logger and exit cleanly
- [ ] On decode failure: log and exit — do not continue with empty token
- [ ] `fmt.Println` calls replaced with `l.Error(...)`

## Context Files
- `internal/app/app.go:55-72` — the token file loading block
- `internal/models/config.go:6` — `TokenFile` field that should be used

## Work Log

### 2026-04-16T20:41:40.394Z - Fixed: token file path now uses cfg.InfluxDB.TokenFile; errors logged and surfaced

