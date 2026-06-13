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
- **ELM327 vs STN adapters**: both speak the same line-oriented ASCII protocol (`internal/obd`: write `cmd\r`, read to `>` prompt; monitor streams frames until a stop byte). `elm327` uses the base AT command set (single `ATCF`/`ATCM` filter, `ATMA` monitor); `stn` (STN11xx, e.g. OBDLink MX+ = STN2120) adds the ST command set (multiple hardware pass filters via `STFPA`/`STFPC`, raw `STM` monitor — `STMA` is avoided since it reassembles CAN as ISO 15765 and overrides filters — and firmware periodic polling via `STPPMA`). The protocol library is transport-agnostic (`obd.Transport` = `io.ReadWriteCloser`) and unit-tested against an in-memory `scriptedTransport` — no hardware needed. Shared CanConnection boilerplate + monitor/poll/hybrid orchestration + auto-reconnect live in `internal/connection/obdconn`; the `elm327`/`stn` packages are thin `Driver` adapters.
- **OBD transports & URI**: `obd.Open(uri)` (in `internal/obd/transport.go`) dispatches on the URI: a serial device path → `go.bug.st/serial` (cross-platform); an `rfcomm://<MAC>` or bare-MAC URI → a native Linux RFCOMM socket (`transport_rfcomm_linux.go`, `AF_BLUETOOTH`) with **no manual `rfcomm bind`**. Channel auto-discovers via SDP-over-L2CAP (`sdp.go`, falls back to channel 1) unless given as `rfcomm://<MAC>/<ch>`. The SDP PDU codec and URI/MAC parsing are unit-tested; the sockets themselves are Linux-only and hardware-untested.
- **OBD auto-reconnect**: a dropped link (engine off, out of range) is not fatal — `Device.Monitor` returns `io.EOF`, and the `obdconn` receive loop reconnects with exponential backoff (`reconnectMinBackoff`=1s … `reconnectMaxBackoff`=30s), re-running init/filters/periodic. A clean shutdown (`Close` → `Discontinue` closes `quit`) returns `nil` and does **not** reconnect. The `driver`/`transport` pointers are mutex-guarded since the loop swaps them on reconnect while `Close` may read them.
- **OBD modes**: `monitor` = passive sniffing (`ATMA`/`STM`); `poll` = active OBD-II PID request/response on an interval (requires `OBD_PIDS`); `hybrid` = monitor **and** poll concurrently, **STN only** — on `elm327` it logs a warning and falls back (`poll` if PIDs set, else `monitor`). STN hybrid is gap-free: it offloads polling to device firmware (`STCMM 1` to enable transmit-while-monitoring, one `STPPMA period,7DF,<pci+pid>` periodic message per PID, then `STM`); the poll responses appear in the monitor stream, so monitoring is never interrupted. `StopPeriodic` clears them (`STPPMC` + `STCMM 0`) on shutdown. A plain ELM327 has no periodic-message feature, which is why hybrid is STN-only. Hardware pass filters (`HW_FILTER`) matter most on STN over Bluetooth — they prune frames before the slow rfcomm link; if you filter in hybrid mode, include the OBD response IDs (`7E8`–`7EF`) so poll replies pass.
- **OBD over Bluetooth**: pair/trust once (`bluetoothctl pair <MAC>` then `trust <MAC>`), then on **Linux** set `INTERFACE_URI=rfcomm://<MAC>` — cantou opens the RFCOMM socket itself (no `rfcomm bind`, no `/dev/rfcomm0`). A connect that fails because the device isn't paired returns an actionable error pointing at `bluetoothctl`. The legacy bound-path form (`rfcomm bind /dev/rfcomm0 <MAC> 1` + `INTERFACE_URI=/dev/rfcomm0`) still works. `PORT_BAUD` is nominal over SPP/rfcomm (the Bluetooth link governs throughput).
- **Cross-OS support**: the OBD adapters (`elm327`/`stn`) work on **Linux, macOS, and Windows** because they ride a serial/`io.ReadWriteCloser` transport. The native `rfcomm://<MAC>` socket is **Linux-only**; on macOS/Windows the OS exposes a paired adapter as a serial device, so use that path directly (macOS `/dev/cu.*`, Windows `COMx`) — `openRFCOMM` returns a clear error pointing there (`transport_rfcomm_other.go`). `socketcan` is Linux-only and `slcan` is Linux-gated via gocan; `sim`/`playback`/`elm327`/`stn` are portable. The whole module cross-compiles for darwin/windows/linux (amd64/arm64/arm).

@.claude/wiz-claude.md
