package jpeg

import (
	"io"
)

// Data source management ported from IJG libjpeg 9f (jdatasrc.c).
//
// The C library used FILE* or memory buffers. In Go, we use io.Reader
// as the backing store, which naturally handles files, byte slices,
// network connections, etc.

// DataSource wraps an io.Reader to provide JPEG compressed data.
// This replaces the C my_source_mgr / jpeg_stdio_src / jpeg_mem_src.
type DataSource struct {
	reader   io.Reader
	buffer   []JOCTET
	startPos int  // Read position in buffer
	avail    int  // Available bytes in buffer (from startPos)
	eofSeen  bool // Whether we've seen EOF
}

// NewDataSource creates a DataSource from an io.Reader.
func NewDataSource(r io.Reader) *DataSource {
	return &DataSource{
		reader: r,
		buffer: make([]JOCTET, InputBufSize),
	}
}

// NewDataSourceFromBytes creates a DataSource from a byte slice.
func NewDataSourceFromBytes(data []byte) *DataSource {
	return &DataSource{
		reader:   nil,
		buffer:   data,
		startPos: 0,
		avail:    len(data),
		eofSeen:  false,
	}
}

// FillBuffer reads more data from the underlying reader.
// Returns true if data was obtained, false on suspension (not used in Go).
// On true EOF, inserts a fake EOI marker.
func (ds *DataSource) FillBuffer(cinfo *JPEGDecompress) bool {
	if ds.reader == nil {
		// Memory source: any request beyond buffer is an error.
		// Insert fake EOI.
		WarnMSDecompress(cinfo, WrnJPEGEOF)
		ds.buffer = []JOCTET{0xFF, byte(JPEGEoi)}
		ds.startPos = 0
		ds.avail = 2
		return true
	}

	n, err := ds.reader.Read(ds.buffer)
	if n <= 0 {
		if err != nil && err != io.EOF && !ds.eofSeen {
			// First read failure on empty input is fatal.
			if ds.avail == 0 && !ds.eofSeen {
				ErrExitDecompress(cinfo, ErrInputEmpty)
			}
		}
		// Insert a fake EOI marker
		WarnMSDecompress(cinfo, WrnJPEGEOF)
		ds.buffer[0] = 0xFF
		ds.buffer[1] = byte(JPEGEoi)
		ds.startPos = 0
		ds.avail = 2
		ds.eofSeen = true
		return true
	}

	ds.startPos = 0
	ds.avail = n
	ds.eofSeen = (err == io.EOF)
	return true
}

// GetByte reads and returns the next byte from the data source.
// Returns an error if no data is available.
func (ds *DataSource) GetByte(cinfo *JPEGDecompress) (byte, error) {
	if ds.avail <= 0 {
		if !ds.FillBuffer(cinfo) {
			return 0, io.EOF
		}
	}
	if ds.avail <= 0 {
		return 0, io.EOF
	}
	b := ds.buffer[ds.startPos]
	ds.startPos++
	ds.avail--
	return b, nil
}

// ReadBytes reads exactly n bytes into the provided slice.
func (ds *DataSource) ReadBytes(cinfo *JPEGDecompress, n int) ([]byte, error) {
	result := make([]byte, n)
	read := 0
	for read < n {
		if ds.avail <= 0 {
			if !ds.FillBuffer(cinfo) {
				return result[:read], io.ErrUnexpectedEOF
			}
		}
		if ds.avail <= 0 {
			return result[:read], io.ErrUnexpectedEOF
		}
		toCopy := ds.avail
		if toCopy > n-read {
			toCopy = n - read
		}
		copy(result[read:], ds.buffer[ds.startPos:ds.startPos+toCopy])
		ds.startPos += toCopy
		ds.avail -= toCopy
		read += toCopy
	}
	return result, nil
}

// SkipData skips numBytes bytes of data from the input.
func (ds *DataSource) SkipData(cinfo *JPEGDecompress, numBytes int64) {
	if numBytes <= 0 {
		return
	}
	remaining := numBytes
	for remaining > 0 {
		if ds.avail <= 0 {
			if !ds.FillBuffer(cinfo) {
				return
			}
		}
		if ds.avail <= 0 {
			return
		}
		toSkip := int64(ds.avail)
		if toSkip > remaining {
			toSkip = remaining
		}
		ds.startPos += int(toSkip)
		ds.avail -= int(toSkip)
		remaining -= toSkip
	}
}

// BytesAvailable returns the number of bytes currently available in the buffer.
func (ds *DataSource) BytesAvailable() int {
	return ds.avail
}

// PeekBytes returns the next n bytes without consuming them.
// Returns nil if not enough bytes are available.
func (ds *DataSource) PeekBytes(n int) []byte {
	if ds.avail < n {
		return nil
	}
	return ds.buffer[ds.startPos : ds.startPos+n]
}

// SetupSource initializes the SourceManager on a JPEGDecompress struct
// from an io.Reader. This replaces the C jpeg_stdio_src function.
func SetupSource(cinfo *JPEGDecompress, r io.Reader) {
	ds := NewDataSource(r)
	sm := &SourceManager{}
	sm.InitSource = func(cinfo *JPEGDecompress) {
		ds.eofSeen = false
		sm.ResetBuffer(nil)
	}
	sm.FillInputBuffer = func(cinfo *JPEGDecompress) bool {
		if !ds.FillBuffer(cinfo) {
			return false
		}
		return resetSourceManagerFromDataSource(sm, ds)
	}
	sm.SkipInputData = func(cinfo *JPEGDecompress, numBytes int64) {
		skipSourceManagerInput(cinfo, sm, numBytes)
	}
	sm.ResyncToRestart = JPEGResyncToRestart
	sm.TermSource = func(cinfo *JPEGDecompress) {
		// No work needed
	}
	cinfo.Src = sm
}

// SetupMemSource initializes the SourceManager from a byte slice.
// This replaces the C jpeg_mem_src function.
func SetupMemSource(cinfo *JPEGDecompress, data []byte) {
	if len(data) == 0 {
		ErrExitDecompress(cinfo, ErrInputEmpty)
	}
	ds := NewDataSourceFromBytes(data)
	sm := &SourceManager{}
	resetSourceManagerFromDataSource(sm, ds)
	sm.InitSource = func(cinfo *JPEGDecompress) {
		// No work needed for memory source
	}
	sm.FillInputBuffer = func(cinfo *JPEGDecompress) bool {
		if !ds.FillBuffer(cinfo) {
			return false
		}
		return resetSourceManagerFromDataSource(sm, ds)
	}
	sm.SkipInputData = func(cinfo *JPEGDecompress, numBytes int64) {
		skipSourceManagerInput(cinfo, sm, numBytes)
	}
	sm.ResyncToRestart = JPEGResyncToRestart
	sm.TermSource = func(cinfo *JPEGDecompress) {
		// No work needed
	}
	cinfo.Src = sm
}

func resetSourceManagerFromDataSource(sm *SourceManager, ds *DataSource) bool {
	if ds.avail <= 0 {
		sm.ResetBuffer(nil)
		return false
	}
	start := ds.startPos
	end := start + ds.avail
	sm.ResetBuffer(ds.buffer[start:end])
	ds.startPos = end
	ds.avail = 0
	return sm.BytesInBuffer() > 0
}

func skipSourceManagerInput(cinfo *JPEGDecompress, sm *SourceManager, numBytes int64) {
	if numBytes <= 0 {
		return
	}
	for numBytes > 0 {
		avail := sm.BytesInBuffer()
		if avail <= 0 {
			if sm.FillInputBuffer == nil || !sm.FillInputBuffer(cinfo) {
				return
			}
			avail = sm.BytesInBuffer()
			if avail <= 0 {
				return
			}
		}
		toSkip := int64(avail)
		if toSkip > numBytes {
			toSkip = numBytes
		}
		sm.Consume(int(toSkip))
		numBytes -= toSkip
	}
}

// ---------------------------------------------------------------------------
// ResyncToRestart - default restart marker resync procedure
// ---------------------------------------------------------------------------

// JPEGResyncToRestart attempts to resynchronize the input stream after
// encountering an unexpected marker. This is the default implementation
// that replaces the C jpeg_resync_to_restart function.
func JPEGResyncToRestart(cinfo *JPEGDecompress, desired int) bool {
	marker := cinfo.UnreadMarker
	action := 1

	WarnMSDecompress(cinfo, WrnMustResync, marker, desired)
	for {
		switch {
		case marker < markerSOF0:
			action = 2
		case marker < JPEGRst0 || marker > markerRST7:
			action = 3
		default:
			switch marker {
			case JPEGRst0 + ((desired + 1) & 7), JPEGRst0 + ((desired + 2) & 7):
				action = 3
			case JPEGRst0 + ((desired - 1) & 7), JPEGRst0 + ((desired - 2) & 7):
				action = 2
			default:
				action = 1
			}
		}
		TraceMS(&cinfo.JPEGCommon, 4, TrcRecoveryAction, marker, action)

		switch action {
		case 1:
			cinfo.UnreadMarker = 0
			return true
		case 2:
			if !nextSourceMarker(cinfo) {
				return false
			}
			marker = cinfo.UnreadMarker
		case 3:
			return true
		}
	}
}

const (
	markerSOF0 = 0xC0
	markerRST7 = JPEGRst0 + 7
)

func nextSourceMarker(cinfo *JPEGDecompress) bool {
	var discarded uint
	if cinfo.Marker != nil {
		discarded = cinfo.Marker.DiscardedBytes()
	}

	for {
		c, ok := readSourceByte(cinfo)
		if !ok {
			return false
		}
		for c != 0xFF {
			discarded++
			setMarkerDiscardedBytes(cinfo, discarded)
			c, ok = readSourceByte(cinfo)
			if !ok {
				return false
			}
		}
		for {
			c, ok = readSourceByte(cinfo)
			if !ok {
				return false
			}
			if c != 0xFF {
				break
			}
		}
		if c != 0 {
			if discarded != 0 {
				WarnMSDecompress(cinfo, WrnExtraneousData, int(discarded), int(c))
				discarded = 0
				setMarkerDiscardedBytes(cinfo, 0)
			}
			cinfo.UnreadMarker = int(c)
			return true
		}
		discarded += 2
		setMarkerDiscardedBytes(cinfo, discarded)
	}
}

func readSourceByte(cinfo *JPEGDecompress) (byte, bool) {
	if cinfo == nil || cinfo.Src == nil {
		return 0, false
	}
	src := cinfo.Src
	for {
		b := src.GetByte()
		if b >= 0 {
			return byte(b), true
		}
		if src.FillInputBuffer == nil || !src.FillInputBuffer(cinfo) {
			return 0, false
		}
	}
}

func setMarkerDiscardedBytes(cinfo *JPEGDecompress, discarded uint) {
	if cinfo.Marker != nil {
		cinfo.Marker.SetDiscardedBytes(discarded)
	}
}
