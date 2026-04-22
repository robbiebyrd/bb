# CANtou — Bertha (2024 Jeep Wrangler 4xe CAN Bus Logger)

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
CanInterfaces (socketcan | slcan | sim | playback)
  → buffered channel (MSG_BUFFER_SIZE, default 81920)
  → MessageDispatcher (fan-out)  [internal/client/message-dispatch]
  → OutputClients: InfluxDB | MQTT (w/ dedupe) | CSV | CRTD | MF4 | Prometheus
  → SignalDispatcher (per interface, DBC decode + signal filter + fan-out)  [internal/client/signal-dispatch]
```

Key packages: `internal/app`, `internal/connection`, `internal/client/message-dispatch`,
`internal/client/signal-dispatch`, `internal/client/signal-filter`,
`internal/client/dedupe`, `internal/output/{influxdb,mqtt,csv,crtd,mf4,prometheus}`, `internal/parser/dbc`

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
| `INTERFACE_` | `NAME` (required), `NET` (`can`\|`sim`\|`slcan`\|`playback`), `URI`, `DBC` (comma-separated paths), `LOOP`, `SIGNAL_FILTER` (comma-separated `field:op:value` rules), `SIGNAL_FILTER_OP` (`and`\|`or`, default `and`), `SIGNAL_FILTER_MODE` (`exclude`\|`include`, default `exclude`) |
| — | `MSG_BUFFER_SIZE`, `LOG_LEVEL`, `SIM_RATE` (ms, 0=unset), `SIM_RATE_MIN`, `SIM_RATE_MAX`, `CAN_INTERFACE_SEPARATOR` |

## Gotchas

- **No `INTERFACE_*` env vars set?** Defaults silently to a single `sim` interface — useful for local dev, surprising in prod.
- **InfluxDB token**: loaded from `INFLUX_TOKEN` env var; if empty, falls back to `INFLUX_TOKEN_FILE` (default `./config/influxdb/token.json`). Docker compose writes the token there on first start.
- **Interface name format**: `{name}{sep}{network}{sep}{uri}` — separator defaults to `-`, configurable via `CAN_INTERFACE_SEPARATOR`.
- **Sim emit rate**: `SIM_RATE` is in **milliseconds** (default 10ms = ~100 msg/s). Set `SIM_RATE_MIN` and `SIM_RATE_MAX` (both required together) for a random interval per frame. `SIM_RATE` takes priority if set alongside min/max. Setting only one of min/max is an error at startup.
- **MQTT dedupe**: filters by message ID within a time window; `DEDUPE_IDS` is a comma-separated list of IDs to deduplicate (empty = all IDs).
- **Prometheus frame counters**: the `can_frames_total` counter only populates when `LOG_CAN_MESSAGES=true` (default). `can_signal_value` gauges always populate when a DBC file is configured.
- **Prometheus listen address**: bind to loopback (`127.0.0.1:9091`) by default; the Prometheus server in docker-compose scrapes `host.docker.internal:9091`.
- **MF4 output**: produces ASAM MDF4 files readable by the playback parser. Files are written in unfinalized (streaming) form so a crash still leaves a valid file. On graceful shutdown the DT block length and ID block magic are rewritten to finalize — disable via `MF4_FINALIZE=false` if the process may be killed abruptly. Signal output uses a custom `Signal` channel group; CAN output uses a standard `CAN_DataFrame` CG with VLSD payloads.
- **Signal filtering**: per-interface rules applied after DBC decode and OBD-II PID expansion, before fan-out to listeners. Rule format `field:op:value` (field: `signal`|`message`; op: `eq`, `neq`, `contains`, `notcontains`, `startswith`, `notstartswith`, `endswith`, `notendswith`). `SIGNAL_FILTER_MODE=exclude` drops matching signals; `include` keeps only matching. `SIGNAL_FILTER_OP` controls AND/OR across rules. CLI: `--signal-filter name:field:op:value`, `--signal-filter-op name:and|or`, `--signal-filter-mode name:exclude|include`.
- **Renamed packages**: `internal/client/broadcast` → `internal/client/message-dispatch` (pkg `messagedispatch`), `internal/client/signaldispatch` → `internal/client/signal-dispatch` (pkg `signaldispatch`). Listener types `MessageDispatcherListener` and `SignalDispatcherListener` live in `internal/models`.

@.claude/wiz-claude.md
