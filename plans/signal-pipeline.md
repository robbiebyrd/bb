# Implementation Plan: Signal Pipeline — OutputClient Rename + Signal Interface + Dispatcher

**Created:** 2026-04-17
**Status:** Draft
**Estimated Effort:** M

---

## Summary

Rename the two existing `OutputClient` methods for clarity (`Handle` → `HandleCanMessage`, `HandleChannel` → `HandleCanMessageChannel`), introduce a new `CanSignalTimestamped` type representing a decoded DBC signal, and add a second opt-in interface `SignalOutputClient` that output clients implement when they want to receive decoded signals.

Signal delivery is wired via a new `SignalDispatcher` component that sits downstream of the `BroadcastClient`: it receives raw `CanMessageTimestamped` values, runs the DBC parser once per frame, and fans out one `CanSignalTimestamped` per decoded signal to all registered signal-capable output clients. `app.go` detects `SignalOutputClient` at wiring time via a type assertion — non-signal clients (CSV, CRTD) need no changes beyond the rename.

---

## Research Findings

### Repository Patterns

- `OutputClient` interface: `internal/models/repo.go`
- Existing broadcast pattern: `BroadcastClient` → per-client buffered channel → `HandleChannel()` loop → `Handle()` per message
- `app.go` `AddOutput()`: starts `Run()` + `HandleChannel()` goroutines; registers `GetChannel()` with `BroadcastClient`
- DBC parser: `internal/parser/dbc/dbc.go` — `Parse(CanMessageData) any` returns a JSON string; `sig.Unit` is available in the parser but not currently emitted
- DBC output client: `internal/output/dbc/client.go` — the only existing signal consumer; calls parser, logs result; **not wired in `main.go`**
- **Pre-existing gap:** `InfluxDBClient` has no `Handle()` method — it violates the current `OutputClient` interface and will become a hard compile error after the rename

### Gaps Resolved Before This Plan Was Written

| Gap | Resolution |
|-----|------------|
| Who produces `CanSignalTimestamped`? | Central `SignalDispatcher` (new package) |
| All clients or opt-in interface? | Separate `SignalOutputClient` interface |
| `Value` type: `[]byte` vs `float64`? | `float64` — matches DBC parser output; `Data []byte` dropped |
| `GetSignalChannel()` missing from spec | Added to `SignalOutputClient` interface |

---

## Final Type Definitions

```go
// internal/models/can.go — new type
type CanSignalTimestamped struct {
    Timestamp int64
    Interface int
    ID        uint32
    Signal    string
    Value     float64
    Unit      string
}

// internal/models/repo.go — renamed interface
type OutputClient interface {
    Run() error
    HandleCanMessage(canMsg CanMessageTimestamped)
    HandleCanMessageChannel() error
    GetChannel() chan CanMessageTimestamped
    GetName() string
    AddFilter(name string, filter FilterInterface) error
}

// internal/models/repo.go — new interface
type SignalOutputClient interface {
    OutputClient
    HandleSignal(signal CanSignalTimestamped)
    HandleSignalChannel() error
    GetSignalChannel() chan CanSignalTimestamped
}
```

---

## Implementation Order (TDD)

Each step: write failing test → implement → `go test ./...` green before moving on.

---

### Step 1: Fix InfluxDB interface gap (pre-existing bug)

Add a no-op `HandleCanMessage()` stub to `InfluxDBClient`. This surfaces the pre-existing gap as a compile error after the rename in Step 2, so fix it now — in the same commit as the rename.

- **Implement:** `internal/output/influxdb/client.go` — add `func (c *InfluxDBClient) HandleCanMessage(_ canModels.CanMessageTimestamped) {}`
- **Note:** No test needed — InfluxDB's channel-based batching intentionally bypasses per-message handling; the stub is intentional. Add a comment explaining this.
- **Validation:** `go build ./...` passes

---

### Step 2: Rename Handle → HandleCanMessage, HandleChannel → HandleCanMessageChannel

Mechanical rename across the whole codebase. Do in one commit so the repo is never in a partially-renamed state.

- **Interface:** `internal/models/repo.go:3` — rename both method signatures
- **Implementations (all 5):**
  - `internal/output/mqtt/client.go` — rename `Handle` + `HandleChannel`
  - `internal/output/csv/client.go` — rename both; `HandleCanMessageChannel` body calls `HandleCanMessage`
  - `internal/output/crtd/client.go` — same pattern as CSV
  - `internal/output/dbc/client.go` — same pattern
  - `internal/output/influxdb/client.go` — rename `HandleChannel` → `HandleCanMessageChannel` (stub from Step 1 already named correctly)
- **Wiring:** `internal/app/app.go:62` — `c.HandleChannel` → `c.HandleCanMessageChannel`
- **Tests (14 call sites across 3 files):**
  - `internal/output/csv/client_test.go` — rename `Handle` → `HandleCanMessage`
  - `internal/output/crtd/client_test.go` — rename `Handle` → `HandleCanMessage`
  - `internal/output/influxdb/client_test.go` — rename `HandleChannel` → `HandleCanMessageChannel`
- **Validation:** `go test ./...` all green; `go build ./...` passes

---

### Step 3: Add `CanSignalTimestamped` type

- **Test:** `internal/models/can_test.go` (new) — verify struct field names and types compile and zero-value correctly
- **Implement:** `internal/models/can.go` — add `CanSignalTimestamped` struct:
  ```go
  type CanSignalTimestamped struct {
      Timestamp int64
      Interface int
      ID        uint32
      Signal    string
      Value     float64
      Unit      string
  }
  ```
- **Validation:** `go test ./internal/models/...` passes

---

### Step 4: Add `SignalOutputClient` interface

- **Test:** `internal/models/repo_test.go` (new) — compile-time assertion that `SignalOutputClient` embeds `OutputClient`; verify method set via reflection
- **Implement:** `internal/models/repo.go` — add `SignalOutputClient` interface (see type definition above)
- **Validation:** `go build ./...` passes

---

### Step 5: Update DBC parser — add `ParseSignals` method

The existing `Parse()` returns a JSON `any`. A new typed method is needed to produce `[]CanSignalTimestamped`.

- **Test:** `internal/parser/dbc/dbc_test.go` — `TestParseSignals`: load a minimal DBC file, call `ParseSignals`, assert returned slice contains expected `Signal`, `Value`, `Unit`, correct `Timestamp` and `Interface` propagation; test unknown message ID returns empty slice
- **Update `ParserInterface`:** `internal/models/parser.go` — add:
  ```go
  ParseSignals(msg CanMessageData, timestamp int64, iface int) []CanSignalTimestamped
  ```
  Keep existing `Parse()` to avoid breaking the DBC output client before it is updated in Step 7.
- **Implement:** `internal/parser/dbc/dbc.go` — new `ParseSignals` method:
  - Look up `msg.ID` in database (same as `Parse`)
  - For each signal: decode value + get `sig.Unit`
  - Return one `CanSignalTimestamped{Timestamp: timestamp, Interface: iface, ID: msg.ID, Signal: sig.Name, Value: physicalValue, Unit: sig.Unit}` per signal
  - Return `nil` if message ID not found
- **Validation:** `go test ./internal/parser/...` passes

---

### Step 6: Create `SignalDispatcher`

New package `internal/client/signaldispatch/` — receives `CanMessageTimestamped`, parses each frame into signals, fans out to registered signal listeners (non-blocking).

- **Test:** `internal/client/signaldispatch/dispatcher_test.go`
  - `TestDispatch_FansOutSignals`: mock parser returns two signals for a message; assert both listener channels receive both signals
  - `TestDispatch_UnknownMessage`: mock parser returns nil; assert no messages sent to listeners
  - `TestDispatch_DropsWhenListenerFull`: listener channel at capacity; assert dispatch does not block; drop is logged at Warn
  - `TestDispatch_ExitsOnContextCancel` (or channel close): goroutine exits cleanly when input channel is closed
- **Implement:** `internal/client/signaldispatch/dispatcher.go`
  ```go
  type SignalDispatcher struct {
      parser    canModels.ParserInterface
      inChannel chan canModels.CanMessageTimestamped
      listeners []chan canModels.CanSignalTimestamped
      l         *slog.Logger
  }

  func NewSignalDispatcher(parser canModels.ParserInterface, bufSize int, logger *slog.Logger) *SignalDispatcher

  // AddListener registers a signal output channel. Must be called before Dispatch().
  func (d *SignalDispatcher) AddListener(ch chan canModels.CanSignalTimestamped)

  // GetChannel returns the input channel to register with BroadcastClient.
  func (d *SignalDispatcher) GetChannel() chan canModels.CanMessageTimestamped

  // Dispatch reads from inChannel, parses signals, and fans out. Runs until channel closed.
  func (d *SignalDispatcher) Dispatch() error
  ```
- **Validation:** `go test ./internal/client/signaldispatch/...` passes

---

### Step 7: Update `app.go` — wire signal-capable clients

`AppData` gains an optional `*signaldispatch.SignalDispatcher`. `AddOutput` detects `SignalOutputClient` via type assertion and wires signal channels when the dispatcher is present.

- **Test:** `internal/app/` has no test file today. Add `internal/app/app_test.go`:
  - `TestAddOutput_SignalClientWired`: mock `SignalOutputClient` + mock dispatcher; assert `HandleSignalChannel` goroutine started and `GetSignalChannel()` registered
  - `TestAddOutput_NonSignalClientSkipped`: plain `OutputClient` mock; assert no signal wiring attempted
- **Implement:** `internal/app/app.go`
  - Add `signalDispatcher *signaldispatch.SignalDispatcher` field to `AppData`
  - Add `SetSignalDispatcher(d *signaldispatch.SignalDispatcher)` method to `AppInterface` and `AppData`
  - Update `AddOutput`:
    ```go
    func (b *AppData) AddOutput(c canModels.OutputClient) {
        b.wgClients.Go(c.Run)
        b.wgClients.Go(c.HandleCanMessageChannel)
        b.broadcastClient.Add(broadcast.BroadcastClientListener{
            Name:    c.GetName(),
            Channel: c.GetChannel(),
        })
        if sc, ok := c.(canModels.SignalOutputClient); ok && b.signalDispatcher != nil {
            b.wgClients.Go(sc.HandleSignalChannel)
            b.signalDispatcher.AddListener(sc.GetSignalChannel())
        }
    }
    ```
- **Update `AppInterface`:** `internal/models/app.go` — add `SetSignalDispatcher` signature
- **Validation:** `go test ./internal/app/...` passes; `go test ./...` all green

---

### Step 8: Wire `SignalDispatcher` in `main.go` + add config

Add a `SignalConfig` to `Config` with a single `DBCFile` path. When set, `main.go` creates a `SignalDispatcher`, registers it with the `BroadcastClient` (as a regular listener), starts its `Dispatch()` goroutine, and calls `app.SetSignalDispatcher`.

- **Test:** `internal/config/args_test.go` — `TestSignalConfig_OptionalWhenDBCFileAbsent`: parse with no `SIGNAL_DBC_FILE` set; no error, empty string
- **Config:** `internal/models/config.go` — add:
  ```go
  type SignalConfig struct {
      DBCFile string `env:"DBC_FILE" envDefault:""`
  }
  // in Config struct:
  Signal SignalConfig `envPrefix:"SIGNAL_"`
  ```
- **Flags:** `internal/config/flags.go` — add `--signal-dbc-file` flag bound to `cfg.Signal.DBCFile`
- **`main.go`** — after creating app, before `AddOutputs`:
  ```go
  if cfg.Signal.DBCFile != "" {
      parser, err := dbc.NewDBCParserClient(cfg.Signal.DBCFile)
      if err != nil {
          logger.Error("failed to load signal DBC file", "error", err)
          os.Exit(1)
      }
      dispatcher := signaldispatch.NewSignalDispatcher(parser, cfg.MessageBufferSize, logger)
      // Register dispatcher as a CAN message listener in the broadcast client
      b.GetBroadcastClient().Add(broadcast.BroadcastClientListener{
          Name:    "signal-dispatcher",
          Channel: dispatcher.GetChannel(),
      })
      b.SetSignalDispatcher(dispatcher)
      b.GetWaitGroup().Go(dispatcher.Dispatch)
  }
  ```
  > **Note:** `GetBroadcastClient()` and `GetWaitGroup()` may need to be added to `AppInterface` if not already present, or the dispatcher wiring can be done inside a new `app.go` helper. Prefer the helper to avoid leaking internals.
- **Validation:** `go test ./...` all green; `go build ./...` passes

---

### Step 9: Update existing DBC output client (use renamed methods)

The DBC output client currently implements `Handle`/`HandleChannel`. After the rename in Step 2 it already uses the new names. Verify it still compiles and its tests pass. No logic changes needed — it is not a `SignalOutputClient` and continues to log decoded signals via its own internal parser call.

- **Validation:** `go test ./internal/output/dbc/...` passes

---

### Final: Cleanup

- [ ] Remove any leftover `Parse() any` usage if the old DBC output client can be migrated to `ParseSignals`
- [ ] Verify `go vet ./...` and `go build ./...` are clean
- [ ] Check `CLAUDE.md` gotchas section — add note about `SIGNAL_DBC_FILE` optional config

---

## Acceptance Criteria

- [ ] `OutputClient` interface has `HandleCanMessage` and `HandleCanMessageChannel` (renamed)
- [ ] `SignalOutputClient` interface exists with `HandleSignal`, `HandleSignalChannel`, `GetSignalChannel`
- [ ] `CanSignalTimestamped` type exists with `Timestamp`, `Interface`, `ID`, `Signal`, `Value float64`, `Unit`
- [ ] All existing output clients (mqtt, csv, crtd, influxdb, dbc) compile with renamed methods
- [ ] `InfluxDBClient` has a `HandleCanMessage` stub with explanatory comment
- [ ] `SignalDispatcher` fans out signals to registered listeners; drops non-blocking with Warn log
- [ ] `app.go` wires `HandleSignalChannel` goroutine only for clients implementing `SignalOutputClient`
- [ ] Signal pipeline is opt-in via `SIGNAL_DBC_FILE` / `--signal-dbc-file`; absent = no dispatcher created
- [ ] `go test ./...` all green
- [ ] `go build ./...` clean

---

## Performance Considerations

- `SignalDispatcher` uses non-blocking sends (same pattern as `BroadcastClient`) — a full signal listener channel drops the signal batch with a Warn log, matching the existing drop behaviour for CAN messages
- The DBC parser is called once per frame by the `SignalDispatcher`, not once per output client — O(1) parse cost regardless of how many signal-capable clients are registered

---

## Related Files

| File | Change |
|------|--------|
| `internal/models/repo.go` | Rename interface methods; add `SignalOutputClient` |
| `internal/models/can.go` | Add `CanSignalTimestamped` |
| `internal/models/app.go` | Add `SetSignalDispatcher` to `AppInterface` |
| `internal/models/config.go` | Add `SignalConfig` |
| `internal/models/parser.go` | Add `ParseSignals` to `ParserInterface` |
| `internal/parser/dbc/dbc.go` | Implement `ParseSignals` |
| `internal/output/influxdb/client.go` | Add `HandleCanMessage` stub; rename `HandleChannel` |
| `internal/output/mqtt/client.go` | Rename `Handle` → `HandleCanMessage`, `HandleChannel` → `HandleCanMessageChannel` |
| `internal/output/csv/client.go` | Same renames |
| `internal/output/crtd/client.go` | Same renames |
| `internal/output/dbc/client.go` | Same renames |
| `internal/app/app.go` | Rename call site; add signal dispatcher wiring |
| `internal/client/signaldispatch/dispatcher.go` | New file |
| `internal/config/flags.go` | Add `--signal-dbc-file` flag |
| `cmd/server/main.go` | Wire signal dispatcher when `SIGNAL_DBC_FILE` set |
| `internal/output/*/client_test.go` | Rename call sites in tests |

---

## Next Steps

```
/wiz:work plans/signal-pipeline.md   # execute the plan step by step
```
