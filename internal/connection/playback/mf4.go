package playback

import (
	"encoding/binary"
	"fmt"
	"io"
	"log/slog"
	"math"
	"os"

	mf4fmt "github.com/robbiebyrd/bb/internal/parser/mf4"
)

// MF4Parser reads ASAM MDF4 (.mf4) log files containing CAN bus data.
//
// It supports both finalized and unfinalized (streaming) files as produced
// by CANedge and similar loggers. Only CAN_DataFrame channel groups are
// extracted; LIN and other bus types are skipped.
//
// The parser handles unsorted files where records from multiple channel
// groups are interleaved with record-ID prefixes, and where payload data
// is stored in a separate VLSD (Variable Length Signal Data) channel group.
type MF4Parser struct {
	l *slog.Logger
}

// cgInfo describes one channel group in an MF4 data group.
type cgInfo struct {
	recordID  uint64
	dataBytes uint32
	acqName   string
	isVLSD    bool
}

func (p *MF4Parser) Parse(path string) ([]LogEntry, error) {
	logger := p.l
	if logger == nil {
		logger = slog.Default()
	}

	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening MF4 file: %w", err)
	}
	defer f.Close()

	// Read the 64-byte ID block.
	idBuf := make([]byte, mf4fmt.IDBlockSize)
	if _, err := io.ReadFull(f, idBuf); err != nil {
		return nil, fmt.Errorf("reading MF4 ID block: %w", err)
	}

	// Read HD block at fixed offset 64.
	hdLinks, _, err := mf4fmt.ReadBlock(f, int64(mf4fmt.IDBlockSize))
	if err != nil {
		return nil, fmt.Errorf("reading HD block: %w", err)
	}
	dgAddr := hdLinks[0] // first data group

	if dgAddr == 0 {
		return nil, nil
	}

	// Walk the DG chain — typically one DG for CAN bus logging.
	var allEntries []LogEntry
	for dgAddr != 0 {
		entries, nextDG, err := p.parseDG(f, dgAddr, logger)
		if err != nil {
			return nil, fmt.Errorf("parsing data group at %d: %w", dgAddr, err)
		}
		allEntries = append(allEntries, entries...)
		dgAddr = nextDG
	}

	// Convert absolute timestamps to offsets from the first frame.
	if len(allEntries) > 0 {
		baseNs := allEntries[0].OffsetNs
		for i := range allEntries {
			allEntries[i].OffsetNs -= baseNs
		}
	}

	return allEntries, nil
}

// parseDG parses a single Data Group and returns CAN log entries plus the
// address of the next DG (0 if none).
func (p *MF4Parser) parseDG(f *os.File, addr int64, logger *slog.Logger) ([]LogEntry, int64, error) {
	dgLinks, dgData, err := mf4fmt.ReadBlock(f, addr)
	if err != nil {
		return nil, 0, fmt.Errorf("reading DG block: %w", err)
	}
	nextDG := dgLinks[0]
	cgFirstAddr := dgLinks[1]
	dtAddr := dgLinks[2]
	recIDSize := int(dgData[0])

	if dtAddr == 0 || cgFirstAddr == 0 {
		return nil, nextDG, nil
	}

	// Build a map of channel groups by record ID.
	cgMap := make(map[uint64]*cgInfo)
	var canCG *cgInfo

	cgAddr := cgFirstAddr
	for cgAddr != 0 {
		cgLinks, cgData, err := mf4fmt.ReadBlock(f, cgAddr)
		if err != nil {
			return nil, nextDG, fmt.Errorf("reading CG block: %w", err)
		}
		recordID := binary.LittleEndian.Uint64(cgData[0:8])
		flags := binary.LittleEndian.Uint16(cgData[16:18])
		dataBytes := binary.LittleEndian.Uint32(cgData[24:28])
		acqName := mf4fmt.ReadText(f, cgLinks[2])
		isVLSD := flags&mf4fmt.CGFlagVLSD != 0

		info := &cgInfo{
			recordID:  recordID,
			dataBytes: dataBytes,
			acqName:   acqName,
			isVLSD:    isVLSD,
		}
		cgMap[recordID] = info

		if acqName == mf4fmt.AcqNameCANDataFrame {
			canCG = info
		}

		cgAddr = cgLinks[0] // next CG
	}

	if canCG == nil {
		logger.Debug("playback: MF4 data group has no CAN_DataFrame channel group")
		return nil, nextDG, nil
	}

	// Read the DT block data.
	dtData, err := mf4fmt.ReadDTData(f, dtAddr)
	if err != nil {
		return nil, nextDG, fmt.Errorf("reading DT data: %w", err)
	}

	// Two-pass parsing: first collect all VLSD payload data, then decode
	// CAN records. This is necessary because in unsorted files, the VLSD
	// offset in a CAN record references accumulated VLSD data that may
	// appear anywhere in the interleaved stream.
	vlsd := collectVLSD(dtData, recIDSize, cgMap)

	// Second pass: extract CAN_DataFrame records.
	var entries []LogEntry
	pos := 0
	for pos < len(dtData) {
		if recIDSize > 0 {
			if pos+recIDSize > len(dtData) {
				break
			}
			recID := mf4fmt.ReadRecordID(dtData[pos:], recIDSize)
			pos += recIDSize

			cg, ok := cgMap[recID]
			if !ok {
				logger.Debug("playback: MF4 unknown record ID", "recID", recID)
				break
			}

			if cg.isVLSD {
				if pos+4 > len(dtData) {
					break
				}
				vlsdLen := int(binary.LittleEndian.Uint32(dtData[pos : pos+4]))
				pos += 4 + vlsdLen
				continue
			}

			recSize := int(cg.dataBytes)
			if pos+recSize > len(dtData) {
				break
			}

			if cg.acqName != mf4fmt.AcqNameCANDataFrame {
				pos += recSize
				continue
			}

			rec := dtData[pos : pos+recSize]
			pos += recSize

			entry, err := parseMF4CANRecord(rec, vlsd)
			if err != nil {
				logger.Debug("playback: MF4 skipping unparseable CAN record", "error", err)
				continue
			}
			entries = append(entries, *entry)
		} else {
			// Sorted file: only one CG, no record ID prefix.
			recSize := int(canCG.dataBytes)
			if pos+recSize > len(dtData) {
				break
			}
			rec := dtData[pos : pos+recSize]
			pos += recSize

			entry, err := parseMF4CANRecord(rec, vlsd)
			if err != nil {
				logger.Debug("playback: MF4 skipping unparseable CAN record", "error", err)
				continue
			}
			entries = append(entries, *entry)
		}
	}

	return entries, nextDG, nil
}

// parseMF4CANRecord decodes a single CAN_DataFrame record. The record
// layout is described in mf4fmt.EncodeCANFrameRecord.
func parseMF4CANRecord(rec []byte, vlsd *vlsdIndex) (*LogEntry, error) {
	if len(rec) < mf4fmt.CANRecordSize {
		return nil, fmt.Errorf("CAN record too short: %d bytes", len(rec))
	}

	tsNs := math.Float64frombits(binary.LittleEndian.Uint64(rec[0:8]))
	can := rec[8:22]

	idRaw := binary.LittleEndian.Uint32(can[0:4])
	canID := (idRaw >> 2) & 0x1FFFFFFF

	dir := can[4] & 1
	dataLength := int((can[4] >> 1) & 0x7F)

	vlsdOffset := binary.LittleEndian.Uint64(can[6:14])

	data := vlsd.lookup(vlsdOffset, dataLength)

	return &LogEntry{
		OffsetNs: int64(tsNs),
		ID:       canID,
		Transmit: dir == 1,
		Length:   uint8(dataLength),
		Data:     data,
	}, nil
}

// vlsdIndex maps raw byte offsets (as stored in CAN_DataFrame records) to
// positions within the concatenated payload buffer.
type vlsdIndex struct {
	buf     []byte         // concatenated VLSD payloads (no length prefixes)
	offsets map[uint64]int // raw stream offset -> index into buf
}

// collectVLSD scans the data stream and builds a VLSD index. In MDF4, each
// VLSD record in the unsorted stream is [4-byte length][payload]. The offset
// stored in a CAN_DataFrame record is the cumulative byte position in the raw
// VLSD stream (including length prefixes). We track both the raw offset and
// the corresponding position in our payload-only buffer.
func collectVLSD(dtData []byte, recIDSize int, cgMap map[uint64]*cgInfo) *vlsdIndex {
	idx := &vlsdIndex{offsets: make(map[uint64]int)}
	var rawOffset uint64
	pos := 0
	for pos < len(dtData) {
		if recIDSize <= 0 {
			break
		}
		if pos+recIDSize > len(dtData) {
			break
		}
		recID := mf4fmt.ReadRecordID(dtData[pos:], recIDSize)
		pos += recIDSize

		cg, ok := cgMap[recID]
		if !ok {
			break
		}

		if cg.isVLSD {
			if pos+4 > len(dtData) {
				break
			}
			vlsdLen := int(binary.LittleEndian.Uint32(dtData[pos : pos+4]))
			// Map the raw offset (including length prefix) to payload position.
			idx.offsets[rawOffset] = len(idx.buf)
			rawOffset += uint64(4 + vlsdLen)
			pos += 4
			if pos+vlsdLen > len(dtData) {
				break
			}
			idx.buf = append(idx.buf, dtData[pos:pos+vlsdLen]...)
			pos += vlsdLen
		} else {
			pos += int(cg.dataBytes)
		}
	}
	return idx
}

// lookup retrieves dataLength bytes starting at the given raw VLSD offset.
func (v *vlsdIndex) lookup(rawOffset uint64, dataLength int) []byte {
	payloadPos, ok := v.offsets[rawOffset]
	if !ok || payloadPos+dataLength > len(v.buf) {
		return nil
	}
	out := make([]byte, dataLength)
	copy(out, v.buf[payloadPos:payloadPos+dataLength])
	return out
}

// isMF4File is a thin wrapper around mf4fmt.IsMF4File retained so the
// playback parser detection code stays local.
func isMF4File(path string) bool {
	return mf4fmt.IsMF4File(path)
}
