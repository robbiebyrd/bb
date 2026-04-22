# CANtou

<picture>
  <source media="(prefers-color-scheme: light)" srcset="docs/img/cantou-dark.svg">
  <source media="(prefers-color-scheme: dark)" srcset="docs/img/cantou-light.svg">
  <img alt="Cantou" src="docs/img/cantou-dark.svg">
</picture>

An easy-to-use CAN bus logger for any vehicle or test rig that speaks CAN.

CANtou reads frames from one or more CAN interfaces, optionally decodes them with DBC files, and fans the results out to a configurable mix of sinks: time-series databases, message brokers, flat files, and metrics endpoints.

## Features

- **CAN inputs:** Linux SocketCAN, serial SLCAN, offline log playback (`.log`, `.asc`, `.trc`, `.mf4`), and a built-in simulator for local development.
- **Signal decoding:** multi-DBC support per interface, plus a built-in OBD-II DBC that auto-attaches unless disabled.
- **Signal filtering:** per-interface rules to exclude noise (`UNUSED`, `UNKNOWN_*`) or include only signals you care about, with configurable AND/OR logic.
- **Outputs:** InfluxDB 3, MQTT (with optional per-ID deduplication), CSV, CRTD, ASAM MDF 4.11, and Prometheus.
- **Config:** env-var-first via `caarlos0/env`, every knob overridable by a CLI flag.
- **Dev ergonomics:** `docker compose up -d` brings up InfluxDB 3, Mosquitto, Prometheus, and MQTT Explorer locally.

## Quickstart

```bash
# Build
go build ./...

# Run with the built-in simulator (no hardware needed)
go run ./cmd/server/main.go

# Run against a SocketCAN interface on Linux
INTERFACE_0_NAME=can0 \
INTERFACE_0_NET=can \
INTERFACE_0_URI=can0 \
go run ./cmd/server/main.go

# Bring up the local dev stack (InfluxDB + Mosquitto + Prometheus + MQTT Explorer)
docker compose up -d
```

When no `INTERFACE_*` variables are set, CANtou starts a single `sim` interface emitting synthetic frames — useful for wiring things up before real hardware is available.

## Architecture

```
CanInterfaces (socketcan | slcan | sim | playback)
  → buffered channel (MSG_BUFFER_SIZE, default 81920)
  → BroadcastClient (fan-out)
  → OutputClients: InfluxDB | MQTT | CSV | CRTD | MF4 | Prometheus
```

Key packages:

| Package | Role |
|---|---|
| `cmd/server` | Binary entrypoint and CLI wiring |
| `internal/app` | Lifecycle coordinator (context, signals, shutdown) |
| `internal/connection/{socketcan,slcan,simulate,playback}` | Frame sources |
| `internal/client/message-dispatch` | Fan-out hub between inputs and outputs |
| `internal/client/signal-dispatch` | DBC decoding and signal fan-out pipeline |
| `internal/client/signal-filter` | Per-interface signal filter rules |
| `internal/client/dedupe` | Time-windowed per-ID deduplication filter |
| `internal/output/{influxdb,mqtt,csv,crtd,mf4,prometheus}` | Sinks |
| `internal/parser/{dbc,mf4,obd2}` | Format parsers and writers |

## Output clients

Each sink is independent — enable any combination by setting the relevant env vars or flags.

| Sink | Activated when… | Notes |
|---|---|---|
| **InfluxDB 3** | `INFLUX_HOST` is set | Writes to `can_message` and optionally a signals table. Token from env or JSON file. |
| **MQTT** | `MQTT_HOST` is set | Publishes per frame/signal. Optional dedupe window. TLS supported. |
| **CSV** | `CSV_CAN_OUTPUT_FILE` or `CSV_SIGNAL_OUTPUT_FILE` is set | Two independent files; headers optional. |
| **CRTD** | `CRTD_CAN_OUTPUT_FILE` or `CRTD_SIGNAL_OUTPUT_FILE` is set | Plain-text log format. |
| **MF4** | `MF4_CAN_OUTPUT_FILE` or `MF4_SIGNAL_OUTPUT_FILE` is set | ASAM MDF 4.11. Files are written unfinalized so a crash still leaves a valid stream; finalized on graceful shutdown when `MF4_FINALIZE=true`. |
| **Prometheus** | `PROMETHEUS_LISTEN_ADDR` is set | Exposes `/metrics`. Bind to loopback in production. |

MF4 output is round-trippable through the built-in playback parser — see `internal/connection/playback/mf4_roundtrip_test.go`.

## Configuration

All config is environment-based and every field is also exposed as a CLI flag. See `CLAUDE.md` for the full reference table; the most commonly used knobs are:

| Prefix | Selected keys |
|---|---|
| `INTERFACE_N_` | `NAME`, `NET` (`can`\|`sim`\|`slcan`\|`playback`), `URI`, `DBC` (comma-separated paths), `LOOP`, `SIGNAL_FILTER` (comma-separated rules), `SIGNAL_FILTER_OP` (`and`\|`or`, default `and`), `SIGNAL_FILTER_MODE` (`exclude`\|`include`, default `exclude`) |
| `INFLUX_` | `HOST`, `TOKEN`, `TOKEN_FILE`, `MESSAGE_DATABASE`, `SIGNAL_DATABASE`, `TABLE`, `FLUSH_TIME` |
| `MQTT_` | `HOST`, `CLIENT_ID`, `TOPIC`, `DEDUPE`, `DEDUPE_TIMEOUT_MS`, `DEDUPE_IDS`, `USERNAME`, `PASSWORD`, `TLS` |
| `CSV_` | `CAN_OUTPUT_FILE`, `SIGNAL_OUTPUT_FILE`, `OUTPUT_HEADERS` |
| `CRTD_` | `CAN_OUTPUT_FILE`, `SIGNAL_OUTPUT_FILE` |
| `MF4_` | `CAN_OUTPUT_FILE`, `SIGNAL_OUTPUT_FILE`, `FINALIZE` |
| `PROMETHEUS_` | `LISTEN_ADDR`, `PATH` |
| — | `MSG_BUFFER_SIZE`, `LOG_LEVEL`, `SIM_RATE` (ms), `SIM_RATE_MIN`, `SIM_RATE_MAX`, `DISABLE_OBD2` |

Interfaces are also configurable via the repeatable `--interface name:net:uri[:dbcfiles[:loop]]` flag.

### Signal filtering

Per-interface rules to suppress noise or allow-list signals of interest. Each rule is `field:op:value` where `field` is `signal` or `message` and `op` is one of `eq`, `neq`, `contains`, `notcontains`, `startswith`, `notstartswith`, `endswith`, `notendswith`.

**Env vars** (set per interface, e.g. `INTERFACE_0_`):

```bash
# Drop signals whose name contains "UNUSED" or whose message starts with "UNKNOWN_"
INTERFACE_0_SIGNAL_FILTER=signal:contains:UNUSED,message:startswith:UNKNOWN_
INTERFACE_0_SIGNAL_FILTER_OP=or        # any rule matching causes a drop (default: and)
INTERFACE_0_SIGNAL_FILTER_MODE=exclude # drop matching signals (default)

# Only keep RPM and Speed signals
INTERFACE_0_SIGNAL_FILTER=signal:eq:RPM,signal:eq:Speed
INTERFACE_0_SIGNAL_FILTER_OP=or
INTERFACE_0_SIGNAL_FILTER_MODE=include
```

**CLI flags** (reference interface by name, repeatable):

```bash
--signal-filter "can0:signal:contains:UNUSED" \
--signal-filter "can0:message:startswith:UNKNOWN_" \
--signal-filter-op "can0:or" \
--signal-filter-mode "can0:exclude"
```

## Development

```bash
go test ./...                      # Run the test suite
go vet ./...                       # Static checks
go build ./cmd/server              # Build the binary
```

The docker-compose stack is intended for local development only — Mosquitto listens on loopback, and the InfluxDB 3 admin token is written to `./config/influxdb/token.json` on first start. CANtou reads the token from `INFLUX_TOKEN` or, failing that, from `INFLUX_TOKEN_FILE`.

## License

See repository for license information.
