# bb — Bertha (2024 Jeep Wrangler 4xe CAN Bus Logger)

Go service that reads CAN bus frames and fans them out to InfluxDB, MQTT, CSV, and Prometheus.

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
  → OutputClients: InfluxDB | MQTT (w/ dedupe) | CSV | Prometheus
```

Key packages: `internal/app`, `internal/connection`, `internal/client/broadcast`,
`internal/client/dedupe`, `internal/output/{influxdb,mqtt,csv,prometheus}`, `internal/parser/dbc`

## Config (env vars)

All config is environment-based via `caarlos0/env`.

| Prefix | Key vars |
|--------|----------|
| `INFLUX_` | `HOST` (required), `TOKEN`, `TOKEN_FILE`, `MESSAGE_DATABASE`, `SIGNAL_DATABASE` (empty = signals disabled), `TABLE`, `SIGNAL_TABLE`, `FLUSH_TIME`, `MAX_WRITE_LINES` |
| `MQTT_` | `HOST` (required), `CLIENT_ID` (required), `TOPIC`, `DEDUPE`, `DEDUPE_TIMEOUT_MS`, `DEDUPE_IDS`, `USERNAME`, `PASSWORD`, `TLS` |
| `CSV_` | `CAN_OUTPUT_FILE`, `SIGNAL_OUTPUT_FILE`, `OUTPUT_HEADERS` |
| `CRTD_` | `CAN_OUTPUT_FILE`, `SIGNAL_OUTPUT_FILE` |
| `MF4_` | `CAN_OUTPUT_FILE`, `SIGNAL_OUTPUT_FILE`, `FINALIZE` (default `true`) |
| `PROMETHEUS_` | `LISTEN_ADDR` (empty = disabled, e.g. `127.0.0.1:9091`), `PATH` (default `/metrics`) |
| `INTERFACE_` | `NAME` (required), `NET` (`can`\|`sim`), `URI` |
| — | `MSG_BUFFER_SIZE`, `LOG_LEVEL`, `SIM_RATE` (ns), `CAN_INTERFACE_SEPARATOR` |

## Gotchas

- **No `INTERFACE_*` env vars set?** Defaults silently to a single `sim` interface — useful for local dev, surprising in prod.
- **InfluxDB token**: loaded from `INFLUX_TOKEN` env var; if empty, falls back to `INFLUX_TOKEN_FILE` (default `./config/influxdb/token.json`). Docker compose writes the token there on first start.
- **Interface name format**: `{name}{sep}{network}{sep}{uri}` — separator defaults to `-`, configurable via `CAN_INTERFACE_SEPARATOR`.
- **Sim emit rate**: `SIM_RATE` is in **nanoseconds** between messages (e.g. `10000000` = 10ms = 100 msg/s).
- **MQTT dedupe**: filters by message ID within a time window; `DEDUPE_IDS` is a comma-separated list of IDs to deduplicate (empty = all IDs).
- **Prometheus frame counters**: the `can_frames_total` counter only populates when `LOG_CAN_MESSAGES=true` (default). `can_signal_value` gauges always populate when a DBC file is configured.
- **Prometheus listen address**: bind to loopback (`127.0.0.1:9091`) by default; the Prometheus server in docker-compose scrapes `host.docker.internal:9091`.
- **MF4 output**: produces ASAM MDF4 files readable by the playback parser. Files are written in unfinalized (streaming) form so a crash still leaves a valid file. On graceful shutdown the DT block length and ID block magic are rewritten to finalize — disable via `MF4_FINALIZE=false` if the process may be killed abruptly. Signal output uses a custom `Signal` channel group; CAN output uses a standard `CAN_DataFrame` CG with VLSD payloads.

@.claude/wiz-claude.md
