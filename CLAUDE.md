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
CanInterfaces (socketcan | slcan | sim | playback | elm327 | stn)
  → buffered channel (MSG_BUFFER_SIZE, default 81920)
  → MessageDispatcher (fan-out)  [internal/client/message-dispatch]
  → OutputClients: InfluxDB | MQTT (w/ dedupe) | CSV | CRTD | MF4 | Prometheus
  → SignalDispatcher (per interface, DBC decode + signal filter + fan-out)  [internal/client/signal-dispatch]
```

Key packages: `internal/app`, `internal/connection`, `internal/client/message-dispatch`,
`internal/client/signal-dispatch`, `internal/client/signal-filter`,
`internal/client/dedupe`, `internal/output/{influxdb,mqtt,csv,crtd,mf4,prometheus}`, `internal/parser/dbc`,
`internal/obd` (ELM327/STN protocol library), `internal/connection/{elm327,stn,obdconn}`

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
| `INTERFACE_` | `NAME` (required), `NET` (`can`\|`sim`\|`slcan`\|`playback`\|`elm327`\|`stn`), `URI`, `DBC` (comma-separated paths), `LOOP`, `SIGNAL_FILTER` (comma-separated `field:op:value` rules), `SIGNAL_FILTER_OP` (`and`\|`or`, default `and`), `SIGNAL_FILTER_MODE` (`exclude`\|`include`, default `exclude`) |
| `INTERFACE_` (OBD: `elm327`\|`stn`) | `OBD_MODE` (`monitor`\|`poll`\|`hybrid`, default `monitor`), `OBD_PROTOCOL` (ELM327 `ATSP` selector; `6`=ISO15765 11/500 default, `7`=29/500, `0`=auto), `HW_FILTER` (comma-separated CAN IDs to pass in hardware), `OBD_PIDS` (comma-separated requests, e.g. `010C,010D`), `OBD_POLL_MS` (poll interval, default `1000`), `PORT_BAUD` (serial speed, default `115200`) |
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
- **ELM327 vs STN adapters**: both speak the same line-oriented ASCII protocol (`internal/obd`: write `cmd\r`, read to `>` prompt; monitor streams frames until a stop byte). `elm327` uses the base AT command set (single `ATCF`/`ATCM` filter, `ATMA` monitor); `stn` (STN11xx, e.g. OBDLink MX+ = STN2120) adds the ST command set (multiple hardware pass filters via `STFAP`, raw `STM` monitor — `STMA` is avoided since it reassembles CAN as ISO 15765 and overrides filters). The protocol library is transport-agnostic (`go.bug.st/serial`) and unit-tested against an in-memory `scriptedTransport` — no hardware needed. Shared CanConnection boilerplate + monitor/poll/hybrid orchestration live in `internal/connection/obdconn`; the `elm327`/`stn` packages are thin `Driver` adapters.
- **OBD modes**: `monitor` = passive sniffing (`ATMA`/`STM`); `poll` = active OBD-II PID request/response on an interval (requires `OBD_PIDS`); `hybrid` = interleave monitor+poll, **STN only** — on `elm327` it logs a warning and falls back (`poll` if PIDs set, else `monitor`). Monitor and active polling are mutually exclusive on a single adapter at any instant, so hybrid time-slices: monitor for one `OBD_POLL_MS` window, then send the PID batch, repeat. Hardware pass filters (`HW_FILTER`) matter most on STN over Bluetooth — they prune frames before the slow rfcomm link.
- **OBD over Bluetooth**: `URI` is a bound serial device path; for the OBDLink MX+ bind rfcomm first (`rfcomm bind /dev/rfcomm0 <MAC> 1`) then set `INTERFACE_URI=/dev/rfcomm0`. `PORT_BAUD` is nominal over SPP/rfcomm (the Bluetooth link governs throughput). On macOS use the `/dev/cu.*` path.

@.claude/wiz-claude.md
