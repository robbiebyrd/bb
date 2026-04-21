package mf4

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"strings"
)

// MaxCANDataLen is the largest payload length the writer accepts for a CAN
// record. Classic CAN tops out at 8 bytes and CAN FD at 64 bytes; MDF4 has
// no frame type larger than FD, so anything above 64 is rejected.
const MaxCANDataLen = 64

// Sentinel errors returned by AppendCAN / AppendSignal. Callers can use
// errors.Is to distinguish invalid input from I/O failures.
var (
	ErrCANDataTooLong     = errors.New("mf4: CAN data exceeds 64 bytes")
	ErrSignalLabelHasNUL  = errors.New("mf4: signal label string contains NUL byte")
	ErrSignalLabelTooLong = errors.New("mf4: signal label exceeds uint32 length")
)

// LayoutScheme selects the record schema written by the MDF4 writer. CAN
// schemes produce a CG named "CAN_DataFrame" so that the output can be
// consumed by the bb playback parser or by any MDF4-aware tool that
// recognises that bus channel group. The Signal scheme records decoded DBC
// signals with fixed fields plus a VLSD label string.
type LayoutScheme int

const (
	// LayoutCAN emits CAN_DataFrame records as produced by CANedge loggers.
	LayoutCAN LayoutScheme = iota
	// LayoutSignal emits a custom "Signal" channel group recording the
	// fields of CanSignalTimestamped. Records are 24 bytes of numeric
	// fields plus a VLSD offset into a string buffer.
	LayoutSignal
)

// CAN_DataFrame record size (see EncodeCANFrameRecord).
// Signal record size: 8 ts + 8 value + 4 iface + 4 canID + 8 VLSD offset = 32 bytes.
const SignalRecordSize = 32

// Record/VLSD group IDs used by the writer. The CAN schema and the Signal
// schema both use a pair of channel groups: a fixed-length CG holding the
// primary record, plus a VLSD CG carrying variable-length payloads
// (CAN data bytes or concatenated signal label strings).
const (
	RecordIDPrimary uint64 = 1
	RecordIDVLSD    uint64 = 2
)

// Writer appends records to an MDF4 file. It writes all required metadata
// blocks (ID/HD/TX/CG/CN/DG/DT-header) on Open, then appends record bodies
// as AppendCAN / AppendSignal are called. Files are written as
// "unfinalized" (streaming) so that they remain valid even if the process
// is killed mid-write; Close optionally finalizes the file by updating the
// DT block length and ID block flag.
type Writer struct {
	f          *os.File
	scheme     LayoutScheme
	dtHeaderAt int64 // file offset of the ##DT block header
	dataStart  int64 // file offset where record bytes begin
	vlsdOffset uint64
}

// NewCANWriter opens path for writing and emits the metadata blocks for a
// CAN_DataFrame MDF4 file. The file is left positioned at the end of the
// DT header so that subsequent AppendCAN calls append records in place.
func NewCANWriter(path string) (*Writer, error) {
	return newWriter(path, LayoutCAN)
}

// NewSignalWriter opens path for writing and emits the metadata blocks for
// a signal MDF4 file.
func NewSignalWriter(path string) (*Writer, error) {
	return newWriter(path, LayoutSignal)
}

func newWriter(path string, scheme LayoutScheme) (*Writer, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_RDWR, 0644)
	if err != nil {
		return nil, fmt.Errorf("opening MF4 output file: %w", err)
	}

	w := &Writer{f: f, scheme: scheme}
	if err := w.writeMetadata(); err != nil {
		f.Close()
		return nil, err
	}
	return w, nil
}

// writeMetadata lays down the ID / HD / TX / CG / CN / DG / DT-header
// blocks for the configured schema. Block sizes are computed up-front so
// that inter-block links can be filled in with absolute file offsets
// before anything is written.
func (w *Writer) writeMetadata() error {
	var (
		primaryAcqName string
		primaryRecSize uint32
	)
	switch w.scheme {
	case LayoutCAN:
		primaryAcqName = AcqNameCANDataFrame
		primaryRecSize = CANRecordSize
	case LayoutSignal:
		primaryAcqName = "Signal"
		primaryRecSize = SignalRecordSize
	default:
		return fmt.Errorf("unknown MF4 layout scheme: %d", w.scheme)
	}

	// Stage every block in memory so block sizes come from Block.Size() and
	// the serialized TX payloads. Link offsets are filled in after all
	// sizes are known, and the on-disk write order follows the computed
	// addresses below.
	hd := NewBlock(BlockIDHD, 6, 24)
	cg1 := NewBlock(BlockIDCG, 6, 32)
	cg2 := NewBlock(BlockIDCG, 6, 32)
	cnTs := NewBlock(BlockIDCN, 8, 48)
	cnPayload := NewBlock(BlockIDCN, 8, 48)
	dg := NewBlock(BlockIDDG, 4, 8)

	txPrimary := BuildTXBlock(primaryAcqName)
	txTimestamp := BuildTXBlock(AcqNameTimestamp)
	txPayload := BuildTXBlock(primaryAcqName)

	// The DT block header is emitted as a bare header (no links, no data)
	// so its on-disk length is simply HeaderSize.
	dtHeaderLen := int64(HeaderSize)

	cur := int64(IDBlockSize)
	cur += hd.Size()

	txPrimaryAddr := cur
	cur += int64(len(txPrimary))
	txTimestampAddr := cur
	cur += int64(len(txTimestamp))
	txPayloadAddr := cur
	cur += int64(len(txPayload))

	cg1Addr := cur
	cur += cg1.Size()
	cg2Addr := cur
	cur += cg2.Size()

	cnTsAddr := cur
	cur += cnTs.Size()
	cnDataAddr := cur
	cur += cnPayload.Size()

	dgAddr := cur
	cur += dg.Size()

	dtHeaderAddr := cur
	cur += dtHeaderLen

	// HD: wall-clock StartTimeNs is left 0; downstream tools fall back to
	// the measurement-relative timestamps inside the records.
	hd.Links[0] = dgAddr

	cg1.Links[0] = cg2Addr       // next CG -> VLSD
	cg1.Links[1] = cnTsAddr      // first CN -> timestamp
	cg1.Links[2] = txPrimaryAddr // acq name
	binary.LittleEndian.PutUint64(cg1.Data[0:8], RecordIDPrimary)
	binary.LittleEndian.PutUint16(cg1.Data[16:18], CGFlagBus)
	binary.LittleEndian.PutUint32(cg1.Data[24:28], primaryRecSize)

	binary.LittleEndian.PutUint64(cg2.Data[0:8], RecordIDVLSD)
	binary.LittleEndian.PutUint16(cg2.Data[16:18], CGFlagVLSD)

	cnTs.Links[0] = cnDataAddr      // next CN -> payload
	cnTs.Links[2] = txTimestampAddr // TxName
	cnTs.Data[0] = ChannelTypeMaster
	cnTs.Data[1] = SyncTypeTime
	cnTs.Data[2] = DataTypeFloatLE
	binary.LittleEndian.PutUint32(cnTs.Data[4:8], 0)   // byte offset
	binary.LittleEndian.PutUint32(cnTs.Data[8:12], 64) // bit count

	cnPayload.Links[2] = txPayloadAddr
	cnPayload.Data[0] = ChannelTypeFixedLength
	cnPayload.Data[2] = DataTypeByteArr
	binary.LittleEndian.PutUint32(cnPayload.Data[4:8], 8) // byte offset past timestamp
	payloadBits := uint32((primaryRecSize - 8) * 8)
	binary.LittleEndian.PutUint32(cnPayload.Data[8:12], payloadBits)

	dg.Links[1] = cg1Addr      // CG first
	dg.Links[2] = dtHeaderAddr // DT data
	dg.Data[0] = 1             // RecIDSize = 1 byte

	// DT header: length == HeaderSize marks unfinalized; record bytes follow.
	dtHeader := make([]byte, dtHeaderLen)
	copy(dtHeader[0:4], BlockIDDT)
	binary.LittleEndian.PutUint64(dtHeader[8:16], uint64(dtHeaderLen))

	writes := [][]byte{
		WriteIDBlock(true),
		hd.Bytes(),
		txPrimary,
		txTimestamp,
		txPayload,
		cg1.Bytes(),
		cg2.Bytes(),
		cnTs.Bytes(),
		cnPayload.Bytes(),
		dg.Bytes(),
		dtHeader,
	}
	for _, buf := range writes {
		if _, err := w.f.Write(buf); err != nil {
			return err
		}
	}

	w.dtHeaderAt = dtHeaderAddr
	w.dataStart = cur
	w.vlsdOffset = 0
	return nil
}

// AppendCAN appends a single CAN_DataFrame record plus its VLSD payload.
// The writer must have been created with NewCANWriter. Payloads longer than
// MaxCANDataLen (64 bytes, CAN FD max) are rejected with ErrCANDataTooLong.
func (w *Writer) AppendCAN(tsNs int64, canID uint32, tx bool, data []byte) error {
	if w.scheme != LayoutCAN {
		return fmt.Errorf("AppendCAN requires LayoutCAN")
	}
	if len(data) > MaxCANDataLen {
		return fmt.Errorf("%w: got %d bytes", ErrCANDataTooLong, len(data))
	}
	dataLen := uint8(len(data))
	rec := make([]byte, 1+CANRecordSize)
	rec[0] = byte(RecordIDPrimary)
	EncodeCANFrameRecord(rec[1:], tsNs, canID, tx, dataLen, w.vlsdOffset)
	if _, err := w.f.Write(rec); err != nil {
		return err
	}

	return w.appendVLSD(data)
}

// AppendSignal appends a decoded signal record plus its label string as a
// VLSD payload. Label format is "message\x00signal\x00unit" to allow the
// three strings to be recovered by a reader. Strings must not contain NUL
// bytes, and the concatenated label must fit in a uint32 length prefix.
func (w *Writer) AppendSignal(tsNs int64, canID uint32, iface uint32, value float64, message, signal, unit string) error {
	if w.scheme != LayoutSignal {
		return fmt.Errorf("AppendSignal requires LayoutSignal")
	}
	for _, s := range []string{message, signal, unit} {
		if strings.ContainsRune(s, 0) {
			return ErrSignalLabelHasNUL
		}
	}
	labelLen := len(message) + len(signal) + len(unit) + 2
	if uint64(labelLen) > uint64(^uint32(0)) {
		return ErrSignalLabelTooLong
	}

	label := make([]byte, 0, labelLen)
	label = append(label, message...)
	label = append(label, 0)
	label = append(label, signal...)
	label = append(label, 0)
	label = append(label, unit...)

	rec := make([]byte, 1+SignalRecordSize)
	rec[0] = byte(RecordIDPrimary)
	body := rec[1:]
	binary.LittleEndian.PutUint64(body[0:8], math.Float64bits(float64(tsNs)))
	binary.LittleEndian.PutUint64(body[8:16], math.Float64bits(value))
	binary.LittleEndian.PutUint32(body[16:20], iface)
	binary.LittleEndian.PutUint32(body[20:24], canID)
	binary.LittleEndian.PutUint64(body[24:32], w.vlsdOffset)
	if _, err := w.f.Write(rec); err != nil {
		return err
	}

	return w.appendVLSD(label)
}

// appendVLSD writes a single [recID][4-byte len][payload] VLSD record and
// advances the VLSD stream offset.
func (w *Writer) appendVLSD(payload []byte) error {
	vrec := make([]byte, 1+4+len(payload))
	vrec[0] = byte(RecordIDVLSD)
	binary.LittleEndian.PutUint32(vrec[1:5], uint32(len(payload)))
	copy(vrec[5:], payload)
	if _, err := w.f.Write(vrec); err != nil {
		return err
	}
	w.vlsdOffset += uint64(4 + len(payload))
	return nil
}

// Close finalizes the file. The DT block length and the ID block
// unfinalized flag are rewritten so that MDF4 tools recognise the file as
// a finalized measurement. If finalization fails the file is still closed
// and remains readable as an unfinalized stream.
func (w *Writer) Close() error {
	if w.f == nil {
		return nil
	}
	defer func() { w.f = nil }()

	if err := w.finalize(); err != nil {
		_ = w.f.Close()
		return err
	}
	return w.f.Close()
}

// CloseUnfinalized closes the file without rewriting the DT block length or
// the unfinalized flag. Use this when the producer may be killed
// unpredictably — the file will still be valid as a streaming MDF4.
func (w *Writer) CloseUnfinalized() error {
	if w.f == nil {
		return nil
	}
	defer func() { w.f = nil }()
	return w.f.Close()
}

func (w *Writer) finalize() error {
	size, err := w.f.Seek(0, io.SeekEnd)
	if err != nil {
		return err
	}
	blockLen := uint64(size - w.dtHeaderAt)

	var lenBuf [8]byte
	binary.LittleEndian.PutUint64(lenBuf[:], blockLen)
	if _, err := w.f.WriteAt(lenBuf[:], w.dtHeaderAt+8); err != nil {
		return err
	}

	// Clear unfinalized flag at ID block offset 60 and flip magic to
	// "MDF     ".
	if _, err := w.f.WriteAt([]byte("MDF     "), 0); err != nil {
		return err
	}
	var zero [2]byte
	if _, err := w.f.WriteAt(zero[:], 60); err != nil {
		return err
	}
	return nil
}

