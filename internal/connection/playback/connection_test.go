package playback

import (
	"context"
	"io"
	"log/slog"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	canModels "github.com/robbiebyrd/bb/internal/models"
)

// capturingHandler collects slog records for test assertions.
type capturingHandler struct {
	mu      sync.Mutex
	records []slog.Record
}

func newCapturingHandler() *capturingHandler { return &capturingHandler{} }

func (h *capturingHandler) Enabled(_ context.Context, _ slog.Level) bool { return true }

func (h *capturingHandler) Handle(_ context.Context, r slog.Record) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.records = append(h.records, r)
	return nil
}

func (h *capturingHandler) WithAttrs(_ []slog.Attr) slog.Handler { return h }
func (h *capturingHandler) WithGroup(_ string) slog.Handler      { return h }

func (h *capturingHandler) recordsAt(level slog.Level) []slog.Record {
	h.mu.Lock()
	defer h.mu.Unlock()
	var out []slog.Record
	for _, r := range h.records {
		if r.Level == level {
			out = append(out, r)
		}
	}
	return out
}

// --- helpers -----------------------------------------------------------------

func testConfig() *canModels.Config {
	return &canModels.Config{CanInterfaceSeparator: "-"}
}

func testChannel() chan canModels.CanMessageTimestamped {
	return make(chan canModels.CanMessageTimestamped, 64)
}

func silentLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// writeTempLog creates a temp file with the given content and returns its path.
func writeTempLog(t *testing.T, content string) string {
	t.Helper()
	f, err := os.CreateTemp("", "playback_test_*.log")
	require.NoError(t, err)
	t.Cleanup(func() { os.Remove(f.Name()) })
	_, err = f.WriteString(content)
	require.NoError(t, err)
	require.NoError(t, f.Close())
	return f.Name()
}

// minimalBUSMASTERLog returns a minimal valid BUSMASTER log with the given data lines.
func minimalBUSMASTERLog(dataLines string) string {
	return "***BUSMASTER Ver 3.2.2***\n" +
		"***PROTOCOL CAN***\n" +
		"***[START LOGGING SESSION]***\n" +
		"***START DATE AND TIME 12:9:2021 10:45:44:000***\n" +
		"***HEX***\n" +
		"***ABSOLUTE MODE***\n" +
		"***<Time><Tx/Rx><Channel><CAN ID><Type><DLC><DataBytes>***\n" +
		dataLines
}

// twoFrameLog returns a path to a BUSMASTER log with two zero-delay frames.
func twoFrameLog(t *testing.T) string {
	t.Helper()
	return writeTempLog(t, minimalBUSMASTERLog(
		"0:0:0:0000 Rx 1 0x001 s 1 AA\n"+
			"0:0:0:0000 Tx 1 0x002 s 2 BB CC\n",
	))
}

// receiveAndWait starts Receive in a goroutine and waits for it to finish.
func receiveAndWait(conn *PlaybackCanClient) {
	var wg sync.WaitGroup
	conn.Receive(&wg)
	wg.Wait()
}

// --- BUSMASTERParser ---------------------------------------------------------

func TestBUSMASTERParser_Parse_SkipsHeaders(t *testing.T) {
	path := writeTempLog(t, minimalBUSMASTERLog(""))
	entries, err := (&BUSMASTERParser{}).Parse(path)
	require.NoError(t, err)
	assert.Empty(t, entries)
}

func TestBUSMASTERParser_Parse_SingleFrame(t *testing.T) {
	content := minimalBUSMASTERLog("0:0:0:0000 Rx 1 0x028 s 8 07 D0 03 FC 07 D0 90 34\n")
	entries, err := (&BUSMASTERParser{}).Parse(writeTempLog(t, content))
	require.NoError(t, err)
	require.Len(t, entries, 1)
	e := entries[0]
	assert.Equal(t, int64(0), e.OffsetNs)
	assert.Equal(t, uint32(0x028), e.ID)
	assert.False(t, e.Transmit)
	assert.False(t, e.Remote)
	assert.Equal(t, uint8(8), e.Length)
	assert.Equal(t, []byte{0x07, 0xD0, 0x03, 0xFC, 0x07, 0xD0, 0x90, 0x34}, e.Data)
}

func TestBUSMASTERParser_Parse_OffsetRelativeToFirstFrame(t *testing.T) {
	content := minimalBUSMASTERLog(
		"0:0:0:0000 Rx 1 0x028 s 1 AA\n" +
			"0:0:0:0050 Rx 1 0x029 s 1 BB\n" +
			"0:0:0:0150 Rx 1 0x02A s 1 CC\n",
	)
	entries, err := (&BUSMASTERParser{}).Parse(writeTempLog(t, content))
	require.NoError(t, err)
	require.Len(t, entries, 3)
	assert.Equal(t, int64(0), entries[0].OffsetNs)
	assert.Equal(t, int64(50*time.Millisecond), entries[1].OffsetNs)
	assert.Equal(t, int64(150*time.Millisecond), entries[2].OffsetNs)
}

func TestBUSMASTERParser_Parse_TxDirection(t *testing.T) {
	content := minimalBUSMASTERLog("0:0:0:0000 Tx 1 0x100 s 2 AB CD\n")
	entries, err := (&BUSMASTERParser{}).Parse(writeTempLog(t, content))
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.True(t, entries[0].Transmit)
}

func TestBUSMASTERParser_Parse_RemoteFrame(t *testing.T) {
	content := minimalBUSMASTERLog("0:0:0:0000 Rx 1 0x200 sr 0\n")
	entries, err := (&BUSMASTERParser{}).Parse(writeTempLog(t, content))
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.True(t, entries[0].Remote)
}

func TestBUSMASTERParser_Parse_NonZeroStartTimestamp(t *testing.T) {
	// First message at 1s, second at 1.1s → offset of second should be 100ms.
	content := minimalBUSMASTERLog(
		"0:0:1:0000 Rx 1 0x001 s 1 AA\n" +
			"0:0:1:0100 Rx 1 0x002 s 1 BB\n",
	)
	entries, err := (&BUSMASTERParser{}).Parse(writeTempLog(t, content))
	require.NoError(t, err)
	require.Len(t, entries, 2)
	assert.Equal(t, int64(0), entries[0].OffsetNs)
	assert.Equal(t, int64(100*time.Millisecond), entries[1].OffsetNs)
}

func TestBUSMASTERParser_Parse_HoursMinutesSeconds(t *testing.T) {
	// Only the *offset* between frames matters, not the absolute time.
	content := minimalBUSMASTERLog(
		"1:2:3:0456 Rx 1 0x001 s 1 AA\n" +
			"1:2:3:0556 Rx 1 0x002 s 1 BB\n",
	)
	entries, err := (&BUSMASTERParser{}).Parse(writeTempLog(t, content))
	require.NoError(t, err)
	require.Len(t, entries, 2)
	assert.Equal(t, int64(0), entries[0].OffsetNs)
	assert.Equal(t, int64(100*time.Millisecond), entries[1].OffsetNs)
}

func TestBUSMASTERParser_Parse_SkipsUnparseable(t *testing.T) {
	content := minimalBUSMASTERLog(
		"0:0:0:0000 Rx 1 0x001 s 1 AA\n" +
			"this is garbage\n" +
			"0:0:0:0010 Rx 1 0x002 s 1 BB\n",
	)
	entries, err := (&BUSMASTERParser{l: silentLogger()}).Parse(writeTempLog(t, content))
	require.NoError(t, err)
	assert.Len(t, entries, 2, "garbage line should be skipped")
}

func TestBUSMASTERParser_Parse_LogsSkippedLines(t *testing.T) {
	content := minimalBUSMASTERLog(
		"0:0:0:0000 Rx 1 0x001 s 1 AA\n" +
			"this is garbage\n" +
			"also garbage\n" +
			"0:0:0:0010 Rx 1 0x002 s 1 BB\n",
	)
	h := newCapturingHandler()
	logger := slog.New(h)
	p := &BUSMASTERParser{l: logger}
	entries, err := p.Parse(writeTempLog(t, content))
	require.NoError(t, err)
	assert.Len(t, entries, 2)

	debugRecords := h.recordsAt(slog.LevelDebug)
	assert.Len(t, debugRecords, 2, "expected one debug log per skipped line")

	warnRecords := h.recordsAt(slog.LevelWarn)
	require.Len(t, warnRecords, 1, "expected one warn summary after loop")
	assert.Contains(t, warnRecords[0].Message, "skipped")
}

// --- CandumpParser -----------------------------------------------------------

func TestCandumpParser_Parse_EmptyFile(t *testing.T) {
	path := writeTempLog(t, "")
	entries, err := (&CandumpParser{}).Parse(path)
	require.NoError(t, err)
	assert.Empty(t, entries)
}

func TestCandumpParser_Parse_SingleFrame(t *testing.T) {
	content := "(1638323019.096233) can1 09B#26FF00F9007500FF R\n"
	entries, err := (&CandumpParser{}).Parse(writeTempLog(t, content))
	require.NoError(t, err)
	require.Len(t, entries, 1)
	e := entries[0]
	assert.Equal(t, int64(0), e.OffsetNs)
	assert.Equal(t, uint32(0x09B), e.ID)
	assert.False(t, e.Transmit)
	assert.False(t, e.Remote)
	assert.Equal(t, uint8(8), e.Length)
	assert.Equal(t, []byte{0x26, 0xFF, 0x00, 0xF9, 0x00, 0x75, 0x00, 0xFF}, e.Data)
}

func TestCandumpParser_Parse_ShortFrame(t *testing.T) {
	content := "(1000.000000) can1 129#01000000 R\n"
	entries, err := (&CandumpParser{}).Parse(writeTempLog(t, content))
	require.NoError(t, err)
	require.Len(t, entries, 1)
	e := entries[0]
	assert.Equal(t, uint32(0x129), e.ID)
	assert.Equal(t, uint8(4), e.Length)
	assert.Equal(t, []byte{0x01, 0x00, 0x00, 0x00}, e.Data)
}

func TestCandumpParser_Parse_OffsetRelativeToFirstFrame(t *testing.T) {
	content := "" +
		"(1000.000000) can1 001#AA R\n" +
		"(1000.001000) can1 002#BB R\n" +
		"(1000.101000) can1 003#CC R\n"
	entries, err := (&CandumpParser{}).Parse(writeTempLog(t, content))
	require.NoError(t, err)
	require.Len(t, entries, 3)
	assert.Equal(t, int64(0), entries[0].OffsetNs)
	assert.Equal(t, int64(1*time.Millisecond), entries[1].OffsetNs)
	assert.Equal(t, int64(101*time.Millisecond), entries[2].OffsetNs)
}

func TestCandumpParser_Parse_TxDirection(t *testing.T) {
	content := "(1000.000000) can0 100#ABCD T\n"
	entries, err := (&CandumpParser{}).Parse(writeTempLog(t, content))
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.True(t, entries[0].Transmit)
}

func TestCandumpParser_Parse_NoDirectionField(t *testing.T) {
	content := "(1000.000000) can0 100#ABCD\n"
	entries, err := (&CandumpParser{}).Parse(writeTempLog(t, content))
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.False(t, entries[0].Transmit)
}

func TestCandumpParser_Parse_SkipsUnparseable(t *testing.T) {
	content := "" +
		"(1000.000000) can1 001#AA R\n" +
		"this is garbage\n" +
		"(1000.010000) can1 002#BB R\n"
	entries, err := (&CandumpParser{l: silentLogger()}).Parse(writeTempLog(t, content))
	require.NoError(t, err)
	assert.Len(t, entries, 2, "garbage line should be skipped")
}

func TestCandumpParser_Parse_LogsSkippedLines(t *testing.T) {
	content := "" +
		"(1000.000000) can1 001#AA R\n" +
		"this is garbage\n" +
		"also garbage\n" +
		"(1000.010000) can1 002#BB R\n"
	h := newCapturingHandler()
	logger := slog.New(h)
	p := &CandumpParser{l: logger}
	entries, err := p.Parse(writeTempLog(t, content))
	require.NoError(t, err)
	assert.Len(t, entries, 2)

	debugRecords := h.recordsAt(slog.LevelDebug)
	assert.Len(t, debugRecords, 2, "expected one debug log per skipped line")

	warnRecords := h.recordsAt(slog.LevelWarn)
	require.Len(t, warnRecords, 1, "expected one warn summary after loop")
	assert.Contains(t, warnRecords[0].Message, "skipped")
}

// --- DetectParser ------------------------------------------------------------

func TestDetectParser_Candump(t *testing.T) {
	path := writeTempLog(t, "(1638323019.096233) can1 09B#26FF00F9007500FF R\n")
	parser, err := DetectParser(path, silentLogger())
	require.NoError(t, err)
	assert.IsType(t, &CandumpParser{}, parser)
}

func TestDetectParser_BUSMASTER(t *testing.T) {
	path := writeTempLog(t, minimalBUSMASTERLog(""))
	parser, err := DetectParser(path, silentLogger())
	require.NoError(t, err)
	assert.IsType(t, &BUSMASTERParser{}, parser)
}

func TestDetectParser_UnknownFormat(t *testing.T) {
	path := writeTempLog(t, "this is not a known log format\n")
	_, err := DetectParser(path, silentLogger())
	assert.Error(t, err)
}

func TestDetectParser_MissingFile(t *testing.T) {
	_, err := DetectParser("/no/such/file.log", nil)
	assert.Error(t, err)
}

// --- NewPlaybackCanClient ----------------------------------------------------

func TestNewPlaybackCanClient_ValidParams(t *testing.T) {
	conn := NewPlaybackCanClient(context.Background(), testConfig(), "test", testChannel(), "/f.log", false, silentLogger(), nil, nil, nil)
	require.NotNil(t, conn)
}

func TestNewPlaybackCanClient_EmptyNamePanics(t *testing.T) {
	assert.Panics(t, func() {
		NewPlaybackCanClient(context.Background(), testConfig(), "", testChannel(), "/f.log", false, silentLogger(), nil, nil, nil)
	})
}

func TestNewPlaybackCanClient_NilChannelPanics(t *testing.T) {
	assert.Panics(t, func() {
		NewPlaybackCanClient(context.Background(), testConfig(), "test", nil, "/f.log", false, silentLogger(), nil, nil, nil)
	})
}

func TestNewPlaybackCanClient_DefaultsNetwork(t *testing.T) {
	conn := NewPlaybackCanClient(context.Background(), testConfig(), "test", testChannel(), "/f.log", false, silentLogger(), nil, nil, nil)
	assert.Equal(t, "playback", conn.GetNetwork())
}

func TestNewPlaybackCanClient_DefaultsURIToFilePath(t *testing.T) {
	conn := NewPlaybackCanClient(context.Background(), testConfig(), "test", testChannel(), "/some/file.log", false, silentLogger(), nil, nil, nil)
	assert.Equal(t, "/some/file.log", conn.GetURI())
}

func TestNewPlaybackCanClient_ExplicitNetworkAndURI(t *testing.T) {
	network := "mynet"
	uri := "/dev/my.log"
	conn := NewPlaybackCanClient(context.Background(), testConfig(), "test", testChannel(), "/f.log", false, silentLogger(), &network, &uri, nil)
	assert.Equal(t, "mynet", conn.GetNetwork())
	assert.Equal(t, "/dev/my.log", conn.GetURI())
}

// --- Getters / setters -------------------------------------------------------

func TestPlaybackCanClient_GettersAndSetters(t *testing.T) {
	conn := NewPlaybackCanClient(context.Background(), testConfig(), "test", testChannel(), "/f.log", false, silentLogger(), nil, nil, nil)

	conn.SetID(7)
	assert.Equal(t, 7, conn.GetID())

	conn.SetName("new-name")
	assert.Equal(t, "new-name", conn.GetName())

	conn.SetURI("/other.log")
	assert.Equal(t, "/other.log", conn.GetURI())

	conn.SetNetwork("logreplay")
	assert.Equal(t, "logreplay", conn.GetNetwork())

	fp := "/path/to/file.dbc"
	conn.SetDBCFilePath(&fp)
	assert.Equal(t, &fp, conn.GetDBCFilePath())

	assert.Nil(t, conn.GetConnection())
	conn.SetConnection(nil)

	assert.False(t, conn.IsOpen())
}

func TestPlaybackCanClient_GetInterfaceName(t *testing.T) {
	network := "playback"
	uri := "/logs/drive.log"
	conn := NewPlaybackCanClient(context.Background(), testConfig(), "jeep", testChannel(), "/logs/drive.log", false, silentLogger(), &network, &uri, nil)
	assert.Equal(t, "jeep-playback-/logs/drive.log", conn.GetInterfaceName())
}

// --- Interface compliance ----------------------------------------------------

func TestPlaybackCanClient_ImplementsCanConnection(t *testing.T) {
	conn := NewPlaybackCanClient(context.Background(), testConfig(), "test", testChannel(), "/f.log", false, silentLogger(), nil, nil, nil)
	var _ canModels.CanConnection = conn
}

// --- Receive -----------------------------------------------------------------

func TestPlaybackCanClient_Receive_PlaysInOrder(t *testing.T) {
	ch := testChannel()
	conn := NewPlaybackCanClient(context.Background(), testConfig(), "test", ch, twoFrameLog(t), false, silentLogger(), nil, nil, nil)
	receiveAndWait(conn)

	require.Equal(t, 2, len(ch))
	first := <-ch
	second := <-ch
	assert.Equal(t, uint32(0x001), first.ID)
	assert.Equal(t, uint32(0x002), second.ID)
	assert.True(t, second.Transmit)
}

func TestPlaybackCanClient_Receive_NoLoop_StopsAfterFile(t *testing.T) {
	ch := testChannel()
	conn := NewPlaybackCanClient(context.Background(), testConfig(), "test", ch, twoFrameLog(t), false, silentLogger(), nil, nil, nil)
	receiveAndWait(conn)
	assert.Equal(t, 2, len(ch))
}

func TestPlaybackCanClient_Receive_Loop_RepeatsFrames(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ch := make(chan canModels.CanMessageTimestamped, 256)
	conn := NewPlaybackCanClient(ctx, testConfig(), "test", ch, twoFrameLog(t), true, silentLogger(), nil, nil, nil)

	var wg sync.WaitGroup
	conn.Receive(&wg)

	// Wait until we receive at least 6 messages (3+ loops × 2 frames).
	deadline := time.After(2 * time.Second)
	for len(ch) < 6 {
		select {
		case <-deadline:
			t.Fatalf("timed out waiting for loop; got %d messages", len(ch))
		default:
			time.Sleep(5 * time.Millisecond)
		}
	}

	cancel()
	wg.Wait()
}

func TestPlaybackCanClient_Receive_ContextCancel_StopsPlayback(t *testing.T) {
	// 200ms gap between frames so the cancellation can interrupt the sleep.
	path := writeTempLog(t, minimalBUSMASTERLog(
		"0:0:0:0000 Rx 1 0x001 s 1 AA\n"+
			"0:0:0:0200 Rx 1 0x002 s 1 BB\n",
	))

	ctx, cancel := context.WithCancel(context.Background())
	ch := testChannel()
	conn := NewPlaybackCanClient(ctx, testConfig(), "test", ch, path, false, silentLogger(), nil, nil, nil)

	var wg sync.WaitGroup
	conn.Receive(&wg)

	// Let the first frame emit, then cancel before the 200ms delay fires.
	time.Sleep(10 * time.Millisecond)
	cancel()

	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()

	select {
	case <-done:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("goroutine did not exit after context cancel")
	}
}

func TestPlaybackCanClient_Receive_UnsupportedFormat_ExitsCleanly(t *testing.T) {
	path := writeTempLog(t, "not a known format\nsome data\n")
	ch := testChannel()
	conn := NewPlaybackCanClient(context.Background(), testConfig(), "test", ch, path, false, silentLogger(), nil, nil, nil)
	receiveAndWait(conn)
	assert.Equal(t, 0, len(ch))
}
