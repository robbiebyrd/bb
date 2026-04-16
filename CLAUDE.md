# bb — Bertha (2024 Jeep Wrangler 4xe CAN Bus Logger)

Go service that reads CAN bus frames and fans them out to InfluxDB, MQTT, and CSV.

## Commands

```bash
go build ./...                  # Build
go test ./...                   # Run all tests
go run ./cmd/server/main.go     # Run (needs env vars or defaults to sim mode)
docker compose up -d            # Start InfluxDB3 + Mosquitto locally
```

## Architecture

```
CanInterfaces (socketcan | sim)
  → buffered channel (MSG_BUFFER_SIZE, default 81920)
  → BroadcastClient (fan-out)
  → OutputClients: InfluxDB | MQTT (w/ dedupe) | CSV
```

Key packages: `internal/app`, `internal/connection`, `internal/client/broadcast`,
`internal/client/dedupe`, `internal/output/{influxdb,mqtt,csv}`, `internal/parser/dbc`

## Config (env vars)

All config is environment-based via `caarlos0/env`.

| Prefix | Key vars |
|--------|----------|
| `INFLUX_` | `HOST` (required), `TOKEN`, `TOKEN_FILE`, `DATABASE`, `TABLE`, `FLUSH_TIME`, `MAX_WRITE_LINES` |
| `MQTT_` | `HOST` (required), `CLIENT_ID` (required), `TOPIC`, `DEDUPE`, `DEDUPE_TIMEOUT_MS`, `DEDUPE_IDS`, `USERNAME`, `PASSWORD`, `TLS` |
| `CSV_` | `OUTPUT_FILE` (required) |
| `INTERFACE_` | `NAME` (required), `NET` (`can`\|`sim`), `URI` |
| — | `MSG_BUFFER_SIZE`, `LOG_LEVEL`, `SIM_RATE` (ms), `CAN_INTERFACE_SEPARATOR` |

## Gotchas

- **No `INTERFACE_*` env vars set?** Defaults silently to a single `sim` interface — useful for local dev, surprising in prod.
- **InfluxDB token**: loaded from `INFLUX_TOKEN` env var; if empty, falls back to `INFLUX_TOKEN_FILE` (default `./config/influxdb/token.json`). Docker compose writes the token there on first start.
- **Interface name format**: `{name}{sep}{network}{sep}{uri}` — separator defaults to `-`, configurable via `CAN_INTERFACE_SEPARATOR`.
- **Sim emit rate**: `SIM_RATE` is in **milliseconds** between messages (default 10ms = 100 msg/s).
- **MQTT dedupe**: filters by message ID within a time window; `DEDUPE_IDS` is a comma-separated list of IDs to deduplicate (empty = all IDs).

@.claude/wiz-claude.md
