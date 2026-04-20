# Implementation Plan: Prometheus Output Client

**Created:** 2026-04-20
**Status:** Draft
**Estimated Effort:** M

## Summary

Add a `prometheus` output client that exposes CAN bus frames and decoded DBC signals as Prometheus metrics over an HTTP `/metrics` endpoint. The `bb` service acts as a Prometheus exporter — it listens on a configurable address and Prometheus scrapes it on its configured interval.

This follows the pull model (not Pushgateway), which is correct for a long-running daemon. The InfluxDB output client is the primary structural template. A Prometheus server service is added to `docker-compose.yml` to scrape the exporter.

---

## Research Findings

### Repository Patterns

- All output clients live under `internal/output/{name}/client.go`
- Clients implement `models.OutputClient` (mandatory) and optionally `models.RunnerClient` + `models.SignalOutputClient`
- Constructor signature: `NewClient(ctx, *cfg, logger, resolver, ...filters) (OutputClient, error)`
- Config struct added to `internal/models/config.go` with `envPrefix` tag; field added to `Config` struct
- Cobra flag bindings in `internal/config/flags.go`
- Client instantiation gated in `cmd/server/main.go` on a config field being non-empty
- CAN message channel handler wired only when `cfg.LogCanMessages` is true (via `app.AddOutput`)
- Signal channel wired for any client that implements `SignalOutputClient`, regardless of `LogCanMessages`
- Throughput reporting via `common.StartThroughputReporter`; filter application via `common.ShouldFilter`
- Tests use a dedicated `newTestClient` / `newHandleClient` helper, `mockResolver`, `silentLogger()`; table-driven subtests with `testify`

### Best Practices

- Library: `github.com/prometheus/client_golang` (packages: `prometheus`, `promhttp`)
- Pull model: `bb` exposes `/metrics`; Prometheus server scrapes it. Never use Pushgateway for long-running services.
- Signal values → `GaugeVec` (they go up and down)
- Frame counts → `CounterVec` with `_total` suffix (monotonically increasing)
- Use a **custom `prometheus.Registry`** (not `prometheus.DefaultRegisterer`) in the client to avoid test-time global state collisions
- Labels for signals: `interface`, `message`, `signal`, `unit` — all bounded by DBC schema
- Labels for frames: `interface`, `id` (hex string, bounded by CAN bus)
- `GaugeVec.With(labels).Set(value)` is goroutine-safe; no extra locking needed
- Register metrics **once at construction time**, not per-message

### Gaps Identified

- No `HOST` to connect to — gate condition differs from existing clients: use `ListenAddr` (empty = disabled)
- `LogCanMessages` controls CAN frame channel wiring; the Prometheus frame counter only works when it's enabled (document this in CLAUDE.md gotchas)
- `unit` label source: `CanSignal.Unit` field from the DBC parser — confirm field name before coding

---

## Questions to Resolve

### Critical (P1 — Blockers)

None. Research is sufficient to proceed.

### Important (P2 — Affects implementation)

1. **`CanSignal.Unit` field name** — verify the exact field on the `CanSignal`/`CanSignalTimestamped` model before writing the signal handler. Default assumption: `Signal.Unit string`.
2. **Prometheus server version to pin in docker-compose** — use `prom/prometheus:v3.2.1` unless a newer stable exists. Check at implementation time.

---

## Implementation Order (TDD)

Each step: write failing test → implement → verify green → next step.

### Step 1: Config struct

- **Test:** None required for the struct itself; the flags test in Step 2 covers config loading.
- **Implement:** `internal/models/config.go`
  - Add `PrometheusConfig` struct with:
    - `ListenAddr string` — `env:"LISTEN_ADDR" envDefault:""` — empty = disabled; doubles as the `http.ListenAndServe` address (e.g. `:9091`)
    - `Path string` — `env:"PATH" envDefault:"/metrics"`
  - Add `Prometheus PrometheusConfig \`envPrefix:"PROMETHEUS_"\`` field to `Config` struct, after `MQTTConfig`
- **Validation:** `go build ./...` passes

### Step 2: Config flags

- **Test:** `internal/config/flags_test.go` — add test asserting `--prometheus-listen-addr` and `--prometheus-path` flags bind correctly (follow the existing flag test pattern)
- **Implement:** `internal/config/flags.go`
  - Add a Prometheus section following the existing block structure:
    ```go
    f.StringVar(&cfg.Prometheus.ListenAddr, "prometheus-listen-addr", cfg.Prometheus.ListenAddr, "Prometheus metrics listener address (empty = disabled)")
    f.StringVar(&cfg.Prometheus.Path, "prometheus-path", cfg.Prometheus.Path, "Prometheus metrics HTTP path")
    ```
- **Validation:** `go test ./internal/config/...` passes

### Step 3: Client skeleton — constructor, GetName, GetChannel

- **Test:** `internal/output/prometheus/client_test.go`
  - `TestNewClient_returnsCorrectName` — `client.GetName() == "output-prometheus"`
  - `TestNewClient_returnsBufferedChannel` — channel capacity == `cfg.MessageBufferSize`
  - `TestNewClient_registersMetrics` — after construction, gather metrics from the test registry; expect `can_frames_total` and `can_signal_value` families to be registered
- **Implement:** `internal/output/prometheus/client.go`
  - `PrometheusClient` struct with: `ctx`, `l *slog.Logger`, `canChannel chan models.CanMessageTimestamped`, `signalChannel chan models.CanSignalTimestamped`, `resolver models.InterfaceResolver`, `filters map[string]models.FilterInterface`, `canMsgCount atomic.Uint64`, `signalMsgCount atomic.Uint64`, `registry *prometheus.Registry`, `frames *prometheus.CounterVec`, `signals *prometheus.GaugeVec`, `listenAddr string`, `path string`
  - `NewClient(ctx, *cfg, logger, resolver, ...filters) (models.OutputClient, error)`:
    - Creates `prometheus.NewRegistry()`
    - Registers `frames` CounterVec: `can_frames_total`, labels `["interface", "id"]`
    - Registers `signals` GaugeVec: `can_signal_value`, labels `["interface", "message", "signal", "unit"]`
    - Buffers channels at `cfg.MessageBufferSize`
    - Returns `*PrometheusClient`
  - `GetName() string` → `"output-prometheus"`
  - `GetChannel() chan models.CanMessageTimestamped` → returns `canChannel`
  - No-op `HandleCanMessage(canMsg models.CanMessageTimestamped)`
  - `AddFilter(name string, filter models.FilterInterface) error`
- **Validation:** `go test ./internal/output/prometheus/...` passes

### Step 4: HandleCanMessageChannel (frame counter)

- **Test:** `internal/output/prometheus/client_test.go`
  - `TestHandleCanMessageChannel_incrementsCounter` — send a `CanMessageTimestamped` into the channel, close the channel, call `HandleCanMessageChannel()`, gather metrics from registry, assert `can_frames_total{interface="test-if",id="0x200"}` == 1
  - `TestHandleCanMessageChannel_filterDropped` — configure a filter that blocks the message, assert counter stays at 0
  - `TestHandleCanMessageChannel_returnsOnClose` — close channel immediately, verify `HandleCanMessageChannel()` returns `nil`
- **Implement:** `internal/output/prometheus/client.go`
  - `HandleCanMessageChannel() error`:
    - Start `common.StartThroughputReporter`
    - Range over `canChannel`; on close, return `nil`
    - For each message: `common.ShouldFilter` check; resolve interface name via `c.resolver`; `c.frames.With(...).Inc()`; `c.canMsgCount.Add(1)`
- **Validation:** `go test ./internal/output/prometheus/...` passes

### Step 5: HandleSignalChannel (signal gauge)

- **Test:** `internal/output/prometheus/client_test.go`
  - `TestHandleSignalChannel_setsGauge` — send a `CanSignalTimestamped` into the signal channel, close it, call `HandleSignalChannel()`, gather from registry, assert `can_signal_value{interface="test-if",message="EngineData",signal="EngineSpeed",unit="rpm"}` == 3200.5
  - `TestHandleSignalChannel_returnsOnClose` — close channel immediately, verify returns `nil`
- **Implement:** `internal/output/prometheus/client.go`
  - `GetSignalChannel() chan models.CanSignalTimestamped` → returns `signalChannel`
  - `HandleSignal(signal models.CanSignalTimestamped)` — no-op (processing in channel handler)
  - `HandleSignalChannel() error`:
    - Start throughput reporter for signals
    - Range over `signalChannel`; on close, return `nil`
    - For each signal: resolve interface name; `c.signals.With(...).Set(signal.Value)` (confirm field names); `c.signalMsgCount.Add(1)`
- **Validation:** `go test ./internal/output/prometheus/...` passes

### Step 6: Run() — HTTP metrics server

- **Test:** `internal/output/prometheus/client_test.go`
  - `TestRun_servesMetricsEndpoint` — call `Run()` in a goroutine with a `context.WithCancel`; wait for the server to be up (retry GET loop, max 1s); GET `/metrics` → expect 200 and body containing `can_signal_value` or `can_frames_total`; cancel context; verify `Run()` returns `nil`
  - `TestRun_respectsContextCancel` — cancel immediately; `Run()` returns within 100ms
- **Implement:** `internal/output/prometheus/client.go`
  - `Run() error`:
    - Build dedicated `http.ServeMux`; register `promhttp.HandlerFor(c.registry, promhttp.HandlerOpts{})` at `c.path`
    - `srv := &http.Server{Addr: c.listenAddr, Handler: mux}`
    - Launch `srv.ListenAndServe()` in a goroutine; on `ctx.Done()`, call `srv.Shutdown(context.Background())`; return `nil`
    - Use `errgroup` or channel to propagate non-`http.ErrServerClosed` errors
- **Validation:** `go test ./internal/output/prometheus/...` passes

### Step 7: Wire into app

- **Test:** `internal/app/app_test.go` — add a test that a `PrometheusClient` (constructed with a test listen addr) is accepted by `AddOutput` without error and is started as a `RunnerClient` and `SignalOutputClient`
- **Implement:** `cmd/server/main.go`
  - Add import `"github.com/robbiebyrd/bb/internal/output/prometheus"`
  - Add block after the MQTT block:
    ```go
    if cfg.Prometheus.ListenAddr != "" {
        prometheusClient, err := prometheus.NewClient(ctx, &cfg, logger, connections)
        if err != nil {
            logger.Error("failed to create Prometheus client", "error", err)
            os.Exit(1)
        }
        outputs = append(outputs, prometheusClient)
    }
    ```
- **Validation:** `go build ./...` and `go test ./...` both pass

### Step 8: Docker Compose — Prometheus server

- **Test:** None (infra config; validated by running `docker compose config`)
- **Implement:**
  1. `docker-compose.yml` — add `prometheus` service:
     ```yaml
     prometheus:
       image: prom/prometheus:v3.2.1   # pin version at implementation time
       container_name: prometheus
       ports:
         - "127.0.0.1:9090:9090"
       volumes:
         - ./config/prometheus/prometheus.yml:/etc/prometheus/prometheus.yml:ro
         - prometheus-data:/prometheus
       restart: unless-stopped
     ```
     Add `prometheus-data:` to the `volumes:` section.
  2. `config/prometheus/prometheus.yml` — create:
     ```yaml
     global:
       scrape_interval: 15s

     scrape_configs:
       - job_name: bb
         static_configs:
           # Use host.docker.internal when bb runs on the host (not in compose).
           # Replace with the service name if bb is containerised.
           - targets: ['host.docker.internal:9091']
     ```
  3. Update `.env.example` with `PROMETHEUS_LISTEN_ADDR=:9091`
- **Validation:** `docker compose config` exits 0; `docker compose up prometheus` starts the Prometheus server

### Final: Documentation & Cleanup

- [ ] Update `CLAUDE.md` Gotchas section: "Frame counters in the Prometheus client only populate when `LOG_CAN_MESSAGES=true` (default). Signal gauges always populate when a DBC file is configured."
- [ ] Update `CLAUDE.md` Config table: add `PROMETHEUS_` prefix row
- [ ] Remove any TODOs introduced during implementation
- **Validation:** `go test ./...` clean, `go vet ./...` clean, `go build ./...` clean

---

## Acceptance Criteria

- [ ] `go test ./internal/output/prometheus/...` passes with full coverage of frame counter and signal gauge paths
- [ ] `go test ./...` passes (no regressions)
- [ ] `PROMETHEUS_LISTEN_ADDR=:9091` starts the HTTP server; `curl localhost:9091/metrics` returns 200 with Prometheus text format
- [ ] Signal gauges (`can_signal_value`) are populated after DBC-decoded signals flow through
- [ ] Frame counters (`can_frames_total`) are populated when `LogCanMessages=true`
- [ ] Empty `PROMETHEUS_LISTEN_ADDR` (default) starts the app without a Prometheus listener (no port opened)
- [ ] `docker compose up prometheus` starts the Prometheus server on `localhost:9090`
- [ ] `docker compose config` exits 0

---

## Security Considerations

- Bind the metrics endpoint to `127.0.0.1` by default (e.g. `PROMETHEUS_LISTEN_ADDR=127.0.0.1:9091`) to avoid accidental exposure on public interfaces. Document this in the config table.
- No authentication on `/metrics` — acceptable for a local dev tool; note this is consistent with the existing InfluxDB/MQTT setup which also binds to loopback.

## Performance Considerations

- `GaugeVec.With(labels).Set(value)` and `CounterVec.With(labels).Inc()` are goroutine-safe. No extra locking needed.
- Register metrics once at construction time (not per-message). Calling `prometheus.MustRegister` per frame is a resource leak.
- The Prometheus scrape happens on a separate goroutine managed by `net/http`; the CAN frame ingestion path is unaffected by scrape latency.
- High-frequency CAN buses (1000+ frames/s) are fine — the gauge simply holds the most-recent value, and Prometheus samples at 15s intervals. InfluxDB handles high-fidelity recording.

---

## Related Files

| File | Change |
|---|---|
| `internal/models/config.go` | Add `PrometheusConfig`, add `Prometheus` field to `Config` |
| `internal/config/flags.go` | Add Prometheus flag bindings |
| `internal/config/flags_test.go` | Add flag binding tests |
| `internal/output/prometheus/client.go` | New file — full client implementation |
| `internal/output/prometheus/client_test.go` | New file — full test suite |
| `cmd/server/main.go` | Add Prometheus client instantiation block |
| `internal/app/app_test.go` | Add wiring smoke test |
| `docker-compose.yml` | Add `prometheus` service + `prometheus-data` volume |
| `config/prometheus/prometheus.yml` | New file — Prometheus scrape config |
| `.env.example` | Add `PROMETHEUS_LISTEN_ADDR` |
| `CLAUDE.md` | Update Gotchas + Config table |

---

## Next Steps

When ready to implement, run:
- `/wiz:work plans/prometheus-output-client.md` — execute the plan step by step
