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
	cinfo.Src = &SourceManager{
		InitSource: func(cinfo *JPEGDecompress) {
			// Reset for new image from same source
			ds.eofSeen = false
		},
		FillInputBuffer: func(cinfo *JPEGDecompress) bool {
			return ds.FillBuffer(cinfo)
		},
		SkipInputData: func(cinfo *JPEGDecompress, numBytes int64) {
			ds.SkipData(cinfo, numBytes)
		},
		ResyncToRestart: JPEGResyncToRestart,
		TermSource: func(cinfo *JPEGDecompress) {
			// No work needed
		},
	}
}

// SetupMemSource initializes the SourceManager from a byte slice.
// This replaces the C jpeg_mem_src function.
func SetupMemSource(cinfo *JPEGDecompress, data []byte) {
	if len(data) == 0 {
		ErrExitDecompress(cinfo, ErrInputEmpty)
	}
	ds := NewDataSourceFromBytes(data)
	cinfo.Src = &SourceManager{
		InitSource: func(cinfo *JPEGDecompress) {
			// No work needed for memory source
		},
		FillInputBuffer: func(cinfo *JPEGDecompress) bool {
			return ds.FillBuffer(cinfo)
		},
		SkipInputData: func(cinfo *JPEGDecompress, numBytes int64) {
			ds.SkipData(cinfo, numBytes)
		},
		ResyncToRestart: JPEGResyncToRestart,
		TermSource: func(cinfo *JPEGDecompress) {
			// No work needed
		},
	}
}

// ---------------------------------------------------------------------------
// ResyncToRestart - default restart marker resync procedure
// ---------------------------------------------------------------------------

// JPEGResyncToRestart attempts to resynchronize the input stream after
// encountering an unexpected marker. This is the default implementation
// that replaces the C jpeg_resync_to_restart function.
func JPEGResyncToRestart(cinfo *JPEGDecompress, desired int) bool {
	// This is a simplified version. The full C version has complex logic
	// for handling restart marker resynchronization. For now, we just
	// warn and return false (indicating we couldn't resync).
	WarnMSDecompress(cinfo, WrnMustResync)
	return false
}
