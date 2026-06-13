# CANtou

<picture>
  <source media="(prefers-color-scheme: light)" srcset="docs/img/cantou-dark.svg">
  <source media="(prefers-color-scheme: dark)" srcset="docs/img/cantou-light.svg">
  <img alt="Cantou" src="docs/img/cantou-dark.svg">
</picture>

An easy-to-use CAN bus logger for any vehicle or test rig that speaks CAN.

CANtou reads frames from one or more CAN interfaces, optionally decodes them with DBC files, and fans the results out to a configurable mix of sinks: time-series databases, message brokers, flat files, and metrics endpoints.

## Features

- **CAN inputs:** Linux SocketCAN, serial SLCAN, ELM327 & STN (STN11xx, e.g. OBDLink MX+) OBD-II adapters over USB or Bluetooth, offline log playback (`.log`, `.asc`, `.trc`, `.mf4`), and a built-in simulator for local development.
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
CanInterfaces (socketcan | slcan | sim | playback | elm327 | stn)
  → buffered channel (MSG_BUFFER_SIZE, default 81920)
  → BroadcastClient (fan-out)
  → OutputClients: InfluxDB | MQTT | CSV | CRTD | MF4 | Prometheus
```

Key packages:

| Package | Role |
|---|---|
| `cmd/server` | Binary entrypoint and CLI wiring |
| `internal/app` | Lifecycle coordinator (context, signals, shutdown) |
| `internal/connection/{socketcan,slcan,simulate,playback,elm327,stn}` | Frame sources |
| `internal/obd` | Transport-agnostic ELM327/STN (AT/ST) protocol library |
| `internal/connection/obdconn` | Shared OBD connection: monitor/poll/hybrid + auto-reconnect |
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
| `INTERFACE_N_` | `NAME`, `NET` (`can`\|`sim`\|`slcan`\|`playback`\|`elm327`\|`stn`), `URI`, `DBC` (comma-separated paths), `LOOP`, `SIGNAL_FILTER` (comma-separated rules), `SIGNAL_FILTER_OP` (`and`\|`or`, default `and`), `SIGNAL_FILTER_MODE` (`exclude`\|`include`, default `exclude`) |
| `INTERFACE_N_` (OBD: `elm327`\|`stn`) | `OBD_MODE` (`monitor`\|`poll`\|`hybrid`), `OBD_PROTOCOL` (ATSP selector; `6`=ISO15765 11/500 default), `OBD_HW_FILTER` (hex CAN IDs to pass), `OBD_PIDS` (e.g. `010C,010D`), `OBD_POLL_MS`, `OBD_PORT_BAUD` |
| `INFLUX_` | `HOST`, `TOKEN`, `TOKEN_FILE`, `MESSAGE_DATABASE`, `SIGNAL_DATABASE`, `TABLE`, `FLUSH_TIME` |
| `MQTT_` | `HOST`, `CLIENT_ID`, `TOPIC`, `DEDUPE`, `DEDUPE_TIMEOUT_MS`, `DEDUPE_IDS`, `USERNAME`, `PASSWORD`, `TLS` |
| `CSV_` | `CAN_OUTPUT_FILE`, `SIGNAL_OUTPUT_FILE`, `OUTPUT_HEADERS` |
| `CRTD_` | `CAN_OUTPUT_FILE`, `SIGNAL_OUTPUT_FILE` |
| `MF4_` | `CAN_OUTPUT_FILE`, `SIGNAL_OUTPUT_FILE`, `FINALIZE` |
| `PROMETHEUS_` | `LISTEN_ADDR`, `PATH` |
| — | `MSG_BUFFER_SIZE`, `LOG_LEVEL`, `SIM_RATE` (ms), `SIM_RATE_MIN`, `SIM_RATE_MAX`, `DISABLE_OBD2` |

Interfaces are also configurable via the repeatable `--interface name:net:uri[:dbcfiles[:loop]]` flag.

### JSON config file

For anything beyond a couple of interfaces, a JSON file is easier to author than indexed `INTERFACE_0_…` env vars — interfaces become a plain array. Point `CONFIG_FILE` at a JSON file whose tree mirrors the env groups, with multiword leaves as `camelCase` keys:

```jsonc
{
  "logLevel": "info",
  "interfaces": [
    {
      "name": "bertha",
      "net": "stn",
      "uri": "rfcomm://AA:BB:CC:DD:EE:FF",
      "dbc": ["./dbc/bertha.dbc"],
      "obd": { "mode": "hybrid", "pids": ["010C","010D"], "hwFilter": ["7E8","7E9","244"] }
    }
  ],
  "influx":     { "host": "http://localhost:8181" },
  "prometheus": { "listenAddr": "127.0.0.1:9091" }
}
```

```bash
CONFIG_FILE=./config.json go run ./cmd/server/main.go
```

The file is **overlaid on top of env vars**: only keys the file contains are applied, so env vars and defaults remain in effect for everything it omits — keep secrets like `INFLUX_TOKEN` in the environment and structure in the file. Mapping is 1:1 with env (`obd.mode` ↔ `INTERFACE_0_OBD_MODE`, `mqtt.host` ↔ `MQTT_HOST`). A full template lives in [`config.example.json`](config.example.json).

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

### OBD-II adapters (ELM327 / STN) and Bluetooth

CANtou talks to ELM327 and STN11xx (ST command set, e.g. the OBDLink MX+ = STN2120) adapters over USB or Bluetooth. Pick the adapter family with `NET=elm327` or `NET=stn`, and the operating mode with `OBD_MODE`:

| Mode | What it does | Required keys |
|---|---|---|
| `monitor` (default) | Passively sniffs CAN frames (`ATMA`/`STM`) | `OBD_HW_FILTER` optional (hex CAN IDs to pass) |
| `poll` | Actively requests OBD-II PIDs on an interval | `OBD_PIDS`, optional `OBD_POLL_MS` |
| `hybrid` | Sniffs **and** firmware-polls concurrently, gap-free — **STN only** (ELM327 falls back) | `OBD_PIDS` **and** `OBD_HW_FILTER` |

`OBD_HW_FILTER` and `OBD_PIDS` are the two things you pass to choose *what* to capture: `OBD_HW_FILTER` is a comma-separated list of **hex** CAN identifiers (`7E8,1C4,244`) the adapter passes in hardware (empty = all); `OBD_PIDS` is a comma-separated list of OBD-II requests as mode+PID hex (`010C` = engine RPM).

**Bluetooth (Linux):** pair and trust once, then connect by MAC — no `rfcomm bind`, no `/dev/rfcomm0`:

```bash
bluetoothctl pair AA:BB:CC:DD:EE:FF
bluetoothctl trust AA:BB:CC:DD:EE:FF

INTERFACE_0_NAME=bertha \
INTERFACE_0_NET=stn \
INTERFACE_0_URI=rfcomm://AA:BB:CC:DD:EE:FF \
INTERFACE_0_OBD_MODE=hybrid \
INTERFACE_0_OBD_PIDS=010C,010D \
INTERFACE_0_OBD_HW_FILTER=7E8,7E9,7EA,1C4,244 \
go run ./cmd/server/main.go
```

A dropped link (engine off, out of range) reconnects automatically with exponential backoff. On **macOS/Windows**, pair via the OS and point `URI` at the serial device instead (`/dev/cu.*` / `COMx`). See `.env.example` for monitor/poll/hybrid templates.

> **Tip:** in `hybrid` mode, include the OBD response IDs `7E8`–`7EF` in `OBD_HW_FILTER` or the hardware filter will drop your poll replies.

## Development

```bash
go test ./...                      # Run the test suite
go vet ./...                       # Static checks
go build ./cmd/server              # Build the binary
```

The docker-compose stack is intended for local development only — Mosquitto listens on loopback, and the InfluxDB 3 admin token is written to `./config/influxdb/token.json` on first start. CANtou reads the token from `INFLUX_TOKEN` or, failing that, from `INFLUX_TOKEN_FILE`.

## License

See repository for license information.
