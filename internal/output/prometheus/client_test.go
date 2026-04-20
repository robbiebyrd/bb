package prometheus

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	canModels "github.com/robbiebyrd/bb/internal/models"
)

// mockCanConn implements canModels.CanConnection for testing.
type mockCanConn struct {
	interfaceName string
}

func (m *mockCanConn) SetID(_ int)               {}
func (m *mockCanConn) GetName() string           { return "" }
func (m *mockCanConn) GetInterfaceName() string  { return m.interfaceName }
func (m *mockCanConn) Open() error               { return nil }
func (m *mockCanConn) Close() error              { return nil }
func (m *mockCanConn) Receive(_ *sync.WaitGroup) {}

// mockResolver implements canModels.InterfaceResolver for testing.
type mockResolver struct {
	conns map[int]*mockCanConn
}

func (r *mockResolver) ConnectionByID(id int) canModels.CanConnection {
	if c, ok := r.conns[id]; ok {
		return c
	}
	return nil
}

// mockFilterAlwaysTrue drops every message.
type mockFilterAlwaysTrue struct{}

func (m *mockFilterAlwaysTrue) Filter(_ canModels.CanMessageTimestamped) bool { return true }

func silentLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func testResolver() *mockResolver {
	return &mockResolver{
		conns: map[int]*mockCanConn{
			0: {interfaceName: "test-if"},
		},
	}
}

func newTestClient(listenAddr string) *PrometheusClient {
	cfg := &canModels.Config{
		MessageBufferSize: 32,
		Prometheus: canModels.PrometheusConfig{
			ListenAddr: listenAddr,
			Path:       "/metrics",
		},
	}
	client, err := NewClient(context.Background(), cfg, silentLogger(), testResolver())
	if err != nil {
		panic(fmt.Sprintf("newTestClient: %v", err))
	}
	return client.(*PrometheusClient)
}

// gatherCounterValue reads the float64 value for a CounterVec label combination.
func gatherCounterValue(t *testing.T, c *PrometheusClient, labels map[string]string) float64 {
	t.Helper()
	mfs, err := c.registry.Gather()
	require.NoError(t, err)
	for _, mf := range mfs {
		if mf.GetName() != "can_frames_total" {
			continue
		}
		for _, m := range mf.GetMetric() {
			if matchLabels(m.GetLabel(), labels) {
				return m.GetCounter().GetValue()
			}
		}
	}
	return 0
}

// gatherGaugeValue reads the float64 value for a GaugeVec label combination.
func gatherGaugeValue(t *testing.T, c *PrometheusClient, labels map[string]string) (float64, bool) {
	t.Helper()
	mfs, err := c.registry.Gather()
	require.NoError(t, err)
	for _, mf := range mfs {
		if mf.GetName() != "can_signal_value" {
			continue
		}
		for _, m := range mf.GetMetric() {
			if matchLabels(m.GetLabel(), labels) {
				return m.GetGauge().GetValue(), true
			}
		}
	}
	return 0, false
}

func matchLabels(pairs []*dto.LabelPair, want map[string]string) bool {
	got := make(map[string]string, len(pairs))
	for _, lp := range pairs {
		got[lp.GetName()] = lp.GetValue()
	}
	for k, v := range want {
		if got[k] != v {
			return false
		}
	}
	return true
}

// --- Step 3: Constructor / skeleton ---

func TestNewClient_returnsCorrectName(t *testing.T) {
	c := newTestClient("")
	assert.Equal(t, "output-prometheus", c.GetName())
}

func TestNewClient_returnsBufferedChannel(t *testing.T) {
	c := newTestClient("")
	assert.Equal(t, 32, cap(c.GetChannel()))
}

func TestNewClient_registersMetrics(t *testing.T) {
	c := newTestClient("")
	// Gather must succeed without error, confirming the registry is valid.
	_, err := c.registry.Gather()
	require.NoError(t, err)
}

func TestNewClient_filtersApplied(t *testing.T) {
	cfg := &canModels.Config{
		MessageBufferSize: 32,
		Prometheus:        canModels.PrometheusConfig{Path: "/metrics"},
	}
	filter := canModels.FilterInput{Name: "drop-all", Filter: &mockFilterAlwaysTrue{}}
	client, err := NewClient(context.Background(), cfg, silentLogger(), testResolver(), filter)
	require.NoError(t, err)
	c := client.(*PrometheusClient)
	assert.Len(t, c.filters, 1)
}

// --- Step 4: HandleCanMessageChannel ---

func TestHandleCanMessageChannel_incrementsCounter(t *testing.T) {
	c := newTestClient("")

	c.canChannel <- canModels.CanMessageTimestamped{
		Interface: 0,
		ID:        0x200,
		Length:    2,
		Data:      []byte{0x01, 0x02},
	}
	close(c.canChannel)

	require.NoError(t, c.HandleCanMessageChannel())

	v := gatherCounterValue(t, c, map[string]string{"interface": "test-if", "id": "0x200"})
	assert.Equal(t, float64(1), v)
}

func TestHandleCanMessageChannel_filterDropped(t *testing.T) {
	c := newTestClient("")
	require.NoError(t, c.AddFilter("drop-all", &mockFilterAlwaysTrue{}))

	c.canChannel <- canModels.CanMessageTimestamped{Interface: 0, ID: 0x200, Data: []byte{0x01}}
	close(c.canChannel)

	require.NoError(t, c.HandleCanMessageChannel())

	v := gatherCounterValue(t, c, map[string]string{"interface": "test-if", "id": "0x200"})
	assert.Equal(t, float64(0), v)
}

func TestHandleCanMessageChannel_returnsOnClose(t *testing.T) {
	c := newTestClient("")
	close(c.canChannel)

	done := make(chan error, 1)
	go func() { done <- c.HandleCanMessageChannel() }()

	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(200 * time.Millisecond):
		t.Fatal("HandleCanMessageChannel did not return after channel close")
	}
}

func TestHandleCanMessageChannel_unknownInterfaceUsesEmpty(t *testing.T) {
	c := newTestClient("")

	c.canChannel <- canModels.CanMessageTimestamped{
		Interface: 99, // not in resolver
		ID:        0x100,
		Data:      []byte{0x00},
	}
	close(c.canChannel)

	require.NoError(t, c.HandleCanMessageChannel())

	v := gatherCounterValue(t, c, map[string]string{"interface": "", "id": "0x100"})
	assert.Equal(t, float64(1), v)
}

// --- Step 5: HandleSignalChannel ---

func TestHandleSignalChannel_setsGauge(t *testing.T) {
	c := newTestClient("")

	c.signalChannel <- canModels.CanSignalTimestamped{
		Interface: 0,
		Message:   "EngineData",
		Signal:    "EngineSpeed",
		Value:     3200.5,
		Unit:      "rpm",
	}
	close(c.signalChannel)

	require.NoError(t, c.HandleSignalChannel())

	v, found := gatherGaugeValue(t, c, map[string]string{
		"interface": "test-if",
		"message":   "EngineData",
		"signal":    "EngineSpeed",
		"unit":      "rpm",
	})
	require.True(t, found, "gauge not found")
	assert.InDelta(t, 3200.5, v, 0.001)
}

func TestHandleSignalChannel_returnsOnClose(t *testing.T) {
	c := newTestClient("")
	close(c.signalChannel)

	done := make(chan error, 1)
	go func() { done <- c.HandleSignalChannel() }()

	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(200 * time.Millisecond):
		t.Fatal("HandleSignalChannel did not return after channel close")
	}
}

func TestGetSignalChannel_returnsChannel(t *testing.T) {
	c := newTestClient("")
	assert.Equal(t, 32, cap(c.GetSignalChannel()))
}

func TestHandleSignal_isNoop(t *testing.T) {
	// HandleSignal is a no-op; must not panic.
	c := newTestClient("")
	c.HandleSignal(canModels.CanSignalTimestamped{Signal: "RPM", Value: 1000})
}

func TestAddFilter_duplicateReturnsError(t *testing.T) {
	c := newTestClient("")
	require.NoError(t, c.AddFilter("f", &mockFilterAlwaysTrue{}))
	err := c.AddFilter("f", &mockFilterAlwaysTrue{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "filter group already exists")
}

// --- Step 6: Run() HTTP server ---

func TestRun_servesMetricsEndpoint(t *testing.T) {
	c := newTestClient("127.0.0.1:0") // port :0 picks a free port
	// We need a known port for this test — use a fixed high port instead.
	ctx, cancel := context.WithCancel(context.Background())
	c.ctx = ctx
	c.listenAddr = "127.0.0.1:19091"

	runDone := make(chan error, 1)
	go func() { runDone <- c.Run() }()

	// Wait for the server to be ready (retry for up to 1s).
	var resp *http.Response
	var err error
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		resp, err = http.Get("http://127.0.0.1:19091/metrics") //nolint:noctx
		if err == nil {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	require.NoError(t, err, "metrics endpoint never came up")
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	body, _ := io.ReadAll(resp.Body)
	assert.True(t, strings.Contains(string(body), "go_"), "expected Go runtime metrics in output")

	cancel()

	select {
	case err := <-runDone:
		require.NoError(t, err)
	case <-time.After(500 * time.Millisecond):
		t.Fatal("Run() did not return after context cancel")
	}
}

func TestRun_respectsContextCancel(t *testing.T) {
	c := newTestClient("")
	ctx, cancel := context.WithCancel(context.Background())
	c.ctx = ctx
	c.listenAddr = "127.0.0.1:19092"

	runDone := make(chan error, 1)
	go func() { runDone <- c.Run() }()

	// Give the server a moment to start, then cancel immediately.
	time.Sleep(20 * time.Millisecond)
	cancel()

	select {
	case err := <-runDone:
		require.NoError(t, err)
	case <-time.After(500 * time.Millisecond):
		t.Fatal("Run() did not return within 500ms of context cancel")
	}
}
