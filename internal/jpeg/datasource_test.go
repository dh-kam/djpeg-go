package jpeg

import (
	"bytes"
	"io"
	"testing"
)

func newTestDecompress(r io.Reader) *JPEGDecompress {
	cinfo := CreateDecompress()
	if r != nil {
		SetupSource(cinfo, r)
	}
	return cinfo
}

func TestNewDataSource(t *testing.T) {
	data := []byte{0xFF, 0xD8, 0xFF, 0xE0}
	ds := NewDataSource(bytes.NewReader(data))
	if ds == nil {
		t.Fatal("NewDataSource returned nil")
	}
	if ds.buffer == nil {
		t.Error("buffer should not be nil")
	}
	if len(ds.buffer) != InputBufSize {
		t.Errorf("buffer length = %d, want %d", len(ds.buffer), InputBufSize)
	}
}

func TestNewDataSourceFromBytes(t *testing.T) {
	data := []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10}
	ds := NewDataSourceFromBytes(data)
	if ds == nil {
		t.Fatal("NewDataSourceFromBytes returned nil")
	}
	if ds.avail != len(data) {
		t.Errorf("avail = %d, want %d", ds.avail, len(data))
	}
	if ds.startPos != 0 {
		t.Errorf("startPos = %d, want 0", ds.startPos)
	}
	if ds.reader != nil {
		t.Error("reader should be nil for byte source")
	}
}

func TestDataSourceFillBuffer(t *testing.T) {
	data := []byte{0x01, 0x02, 0x03, 0x04, 0x05}
	cinfo := newTestDecompress(bytes.NewReader(data))

	// FillBuffer should read from the reader
	ds := NewDataSource(bytes.NewReader(data))
	ok := ds.FillBuffer(cinfo)
	if !ok {
		t.Error("FillBuffer returned false")
	}
	if ds.avail != len(data) {
		t.Errorf("avail = %d, want %d", ds.avail, len(data))
	}
}

func TestDataSourceFillBufferEOF(t *testing.T) {
	// Empty reader should insert fake EOI and return true
	cinfo := newTestDecompress(bytes.NewReader([]byte{}))
	ds := NewDataSource(bytes.NewReader([]byte{}))

	ok := ds.FillBuffer(cinfo)
	if !ok {
		t.Error("FillBuffer should return true even on EOF")
	}
	// Should have inserted a fake EOI
	if ds.avail != 2 {
		t.Errorf("avail = %d, want 2 (fake EOI)", ds.avail)
	}
	if ds.buffer[0] != 0xFF || ds.buffer[1] != byte(JPEGEoi) {
		t.Errorf("fake EOI = [%#x, %#x], want [0xFF, 0xD9]", ds.buffer[0], ds.buffer[1])
	}
}

func TestDataSourceGetByte(t *testing.T) {
	data := []byte{0xAA, 0xBB, 0xCC}
	cinfo := newTestDecompress(bytes.NewReader(data))
	ds := NewDataSourceFromBytes(data)

	b, err := ds.GetByte(cinfo)
	if err != nil {
		t.Fatalf("GetByte error: %v", err)
	}
	if b != 0xAA {
		t.Errorf("GetByte = 0x%02X, want 0xAA", b)
	}

	b, err = ds.GetByte(cinfo)
	if err != nil {
		t.Fatalf("GetByte error: %v", err)
	}
	if b != 0xBB {
		t.Errorf("GetByte = 0x%02X, want 0xBB", b)
	}
}

func TestDataSourceReadBytes(t *testing.T) {
	data := []byte{0x01, 0x02, 0x03, 0x04, 0x05}
	cinfo := newTestDecompress(bytes.NewReader(data))
	ds := NewDataSourceFromBytes(data)

	result, err := ds.ReadBytes(cinfo, 3)
	if err != nil {
		t.Fatalf("ReadBytes error: %v", err)
	}
	if len(result) != 3 {
		t.Fatalf("ReadBytes returned %d bytes, want 3", len(result))
	}
	if !bytes.Equal(result, []byte{0x01, 0x02, 0x03}) {
		t.Errorf("ReadBytes = %v, want [1 2 3]", result)
	}

	// Read remaining
	result2, err := ds.ReadBytes(cinfo, 2)
	if err != nil {
		t.Fatalf("ReadBytes error: %v", err)
	}
	if !bytes.Equal(result2, []byte{0x04, 0x05}) {
		t.Errorf("ReadBytes = %v, want [4 5]", result2)
	}
}

func TestDataSourceSkipData(t *testing.T) {
	data := []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08}
	cinfo := newTestDecompress(bytes.NewReader(data))
	ds := NewDataSourceFromBytes(data)

	ds.SkipData(cinfo, 3)
	if ds.avail != 5 {
		t.Errorf("after skip, avail = %d, want 5", ds.avail)
	}

	b, err := ds.GetByte(cinfo)
	if err != nil {
		t.Fatalf("GetByte error: %v", err)
	}
	if b != 0x04 {
		t.Errorf("after skip, GetByte = 0x%02X, want 0x04", b)
	}
}

func TestDataSourceSkipDataZero(t *testing.T) {
	data := []byte{0x01, 0x02, 0x03}
	cinfo := newTestDecompress(bytes.NewReader(data))
	ds := NewDataSourceFromBytes(data)

	// Skip zero should be a no-op
	ds.SkipData(cinfo, 0)
	if ds.avail != 3 {
		t.Errorf("after skip(0), avail = %d, want 3", ds.avail)
	}
}

func TestDataSourceBytesAvailable(t *testing.T) {
	data := []byte{0x01, 0x02, 0x03}
	ds := NewDataSourceFromBytes(data)

	if ds.BytesAvailable() != 3 {
		t.Errorf("BytesAvailable = %d, want 3", ds.BytesAvailable())
	}

	ds.startPos++
	ds.avail--
	if ds.BytesAvailable() != 2 {
		t.Errorf("BytesAvailable after consume = %d, want 2", ds.BytesAvailable())
	}
}

func TestDataSourcePeekBytes(t *testing.T) {
	data := []byte{0x01, 0x02, 0x03, 0x04}
	ds := NewDataSourceFromBytes(data)

	peeked := ds.PeekBytes(2)
	if peeked == nil {
		t.Fatal("PeekBytes returned nil")
	}
	if !bytes.Equal(peeked, []byte{0x01, 0x02}) {
		t.Errorf("PeekBytes = %v, want [1 2]", peeked)
	}

	// Should not consume
	if ds.avail != 4 {
		t.Errorf("PeekBytes consumed data: avail = %d, want 4", ds.avail)
	}

	// Request more than available
	peeked = ds.PeekBytes(10)
	if peeked != nil {
		t.Errorf("PeekBytes beyond available should return nil, got %v", peeked)
	}
}

func TestMemSource(t *testing.T) {
	data := []byte{0xFF, 0xD8, 0xFF, 0xD9} // SOI + EOI
	cinfo := CreateDecompress()
	SetupMemSource(cinfo, data)

	if cinfo.Src == nil {
		t.Fatal("Src should not be nil after SetupMemSource")
	}
}

func TestMemSourceEmpty(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic for empty data source")
		}
	}()

	cinfo := CreateDecompress()
	SetupMemSource(cinfo, []byte{})
}

func TestSetupSource(t *testing.T) {
	r := bytes.NewReader([]byte{0xFF, 0xD8})
	cinfo := CreateDecompress()
	SetupSource(cinfo, r)

	if cinfo.Src == nil {
		t.Fatal("Src should not be nil after SetupSource")
	}
	if cinfo.Src.InitSource == nil {
		t.Error("InitSource should not be nil")
	}
	if cinfo.Src.FillInputBuffer == nil {
		t.Error("FillInputBuffer should not be nil")
	}
	if cinfo.Src.SkipInputData == nil {
		t.Error("SkipInputData should not be nil")
	}
	if cinfo.Src.ResyncToRestart == nil {
		t.Error("ResyncToRestart should not be nil")
	}
	if cinfo.Src.TermSource == nil {
		t.Error("TermSource should not be nil")
	}

	// Test InitSource callback
	cinfo.Src.InitSource(cinfo)

	// Test FillInputBuffer callback
	ok := cinfo.Src.FillInputBuffer(cinfo)
	if !ok {
		t.Error("FillInputBuffer should return true")
	}
	if cinfo.Src.BytesInBuffer() != 2 {
		t.Errorf("source buffer after FillInputBuffer = %d, want 2", cinfo.Src.BytesInBuffer())
	}

	// Test SkipInputData callback
	cinfo.Src.SkipInputData(cinfo, 1)
	if cinfo.Src.BytesInBuffer() != 1 {
		t.Errorf("source buffer after SkipInputData = %d, want 1", cinfo.Src.BytesInBuffer())
	}

	// Test TermSource callback
	cinfo.Src.TermSource(cinfo)
}

func TestSetupMemSourceCallbacks(t *testing.T) {
	data := []byte{0xFF, 0xD8, 0xFF, 0xD9}
	cinfo := CreateDecompress()
	SetupMemSource(cinfo, data)
	if cinfo.Src.BytesInBuffer() != len(data) {
		t.Fatalf("memory source buffer = %d, want %d", cinfo.Src.BytesInBuffer(), len(data))
	}

	// Test InitSource callback
	cinfo.Src.InitSource(cinfo)

	// Test FillInputBuffer callback after the initial memory buffer is consumed.
	cinfo.Src.Consume(len(data))
	ok := cinfo.Src.FillInputBuffer(cinfo)
	if !ok {
		t.Error("FillInputBuffer should return true for memory source")
	}
	if got := cinfo.Src.GetByte(); got != 0xFF {
		t.Errorf("first fake EOI byte = 0x%02X, want 0xFF", got)
	}
	if got := cinfo.Src.GetByte(); got != int(JPEGEoi) {
		t.Errorf("second fake EOI byte = 0x%02X, want EOI", got)
	}

	// Test SkipInputData callback
	cinfo.Src.SkipInputData(cinfo, 1)

	// Test TermSource callback
	cinfo.Src.TermSource(cinfo)
}

func TestJPEGResyncToRestart(t *testing.T) {
	cinfo := newTestDecompress(nil)
	cinfo.UnreadMarker = JPEGRst0
	result := JPEGResyncToRestart(cinfo, 0)
	if !result {
		t.Fatal("JPEGResyncToRestart should recover by discarding the desired restart marker")
	}
	if cinfo.UnreadMarker != 0 {
		t.Errorf("UnreadMarker = 0x%02X, want 0", cinfo.UnreadMarker)
	}
}

func TestJPEGResyncToRestartLeavesFutureRestartUnread(t *testing.T) {
	cinfo := newTestDecompress(nil)
	cinfo.UnreadMarker = JPEGRst0 + 1

	if !JPEGResyncToRestart(cinfo, 0) {
		t.Fatal("JPEGResyncToRestart should recover at a future restart marker")
	}
	if cinfo.UnreadMarker != JPEGRst0+1 {
		t.Errorf("UnreadMarker = 0x%02X, want RST1", cinfo.UnreadMarker)
	}
}

func TestJPEGResyncToRestartScansForwardAfterPriorRestart(t *testing.T) {
	cinfo := newTestDecompress(nil)
	cinfo.Src = &SourceManager{NextInputByte: []byte{0x11, 0xFF, byte(JPEGRst0 + 2)}}
	cinfo.UnreadMarker = JPEGRst0 + 7

	if !JPEGResyncToRestart(cinfo, 0) {
		t.Fatal("JPEGResyncToRestart should scan forward from a prior restart marker")
	}
	if cinfo.UnreadMarker != JPEGRst0+2 {
		t.Errorf("UnreadMarker = 0x%02X, want RST2", cinfo.UnreadMarker)
	}
}

func TestJPEGResyncToRestartScansForwardFromInvalidMarker(t *testing.T) {
	cinfo := newTestDecompress(nil)
	cinfo.Src = &SourceManager{NextInputByte: []byte{0x22, 0xFF, byte(JPEGRst0)}}
	cinfo.UnreadMarker = 0x02

	if !JPEGResyncToRestart(cinfo, 0) {
		t.Fatal("JPEGResyncToRestart should scan forward from invalid marker")
	}
	if cinfo.UnreadMarker != 0 {
		t.Errorf("UnreadMarker = 0x%02X, want 0 after desired RST0", cinfo.UnreadMarker)
	}
}

func TestJPEGResyncToRestartSuspendsWithoutSource(t *testing.T) {
	cinfo := newTestDecompress(nil)
	cinfo.UnreadMarker = 0x02

	if JPEGResyncToRestart(cinfo, 0) {
		t.Fatal("JPEGResyncToRestart should suspend when scanning needs input")
	}
}

func TestSourceManagerMethods(t *testing.T) {
	sm := &SourceManager{
		NextInputByte: []byte{0x01, 0x02, 0x03},
	}

	// BytesInBuffer
	if sm.BytesInBuffer() != 3 {
		t.Errorf("BytesInBuffer = %d, want 3", sm.BytesInBuffer())
	}

	// GetByte
	b := sm.GetByte()
	if b != 0x01 {
		t.Errorf("GetByte = 0x%02X, want 0x01", b)
	}
	if sm.BytesInBuffer() != 2 {
		t.Errorf("BytesInBuffer after GetByte = %d, want 2", sm.BytesInBuffer())
	}

	// Consume
	sm.Consume(1)
	if sm.BytesInBuffer() != 1 {
		t.Errorf("BytesInBuffer after Consume(1) = %d, want 1", sm.BytesInBuffer())
	}

	// GetByte when empty
	sm.Consume(10)
	b = sm.GetByte()
	if b != -1 {
		t.Errorf("GetByte on empty should return -1, got %d", b)
	}

	// ResetBuffer
	sm.ResetBuffer([]byte{0xAA, 0xBB})
	if sm.BytesInBuffer() != 2 {
		t.Errorf("BytesInBuffer after ResetBuffer = %d, want 2", sm.BytesInBuffer())
	}
	b = sm.GetByte()
	if b != 0xAA {
		t.Errorf("GetByte after ResetBuffer = 0x%02X, want 0xAA", b)
	}
}

func TestDecompressReader(t *testing.T) {
	// Test the Reader() method returns non-nil
	cinfo := CreateDecompress()
	r := cinfo.Reader()
	if r == nil {
		t.Fatal("Reader() returned nil")
	}

	// Test reading from nil source (should return EOF)
	buf := make([]byte, 2)
	_, err := r.Read(buf)
	if err == nil {
		t.Error("expected error reading from nil source")
	}
}

func TestDecompressReaderReadsSetupSource(t *testing.T) {
	data := []byte{0x01, 0x02, 0x03, 0x04}
	cinfo := CreateDecompress()
	SetupSource(cinfo, bytes.NewReader(data))

	buf := make([]byte, len(data))
	n, err := cinfo.Reader().Read(buf)
	if err != nil {
		t.Fatalf("Reader().Read error: %v", err)
	}
	if n != len(data) {
		t.Fatalf("Reader().Read n = %d, want %d", n, len(data))
	}
	if !bytes.Equal(buf, data) {
		t.Errorf("Reader().Read = %v, want %v", buf, data)
	}
}

func TestDecompressReaderReadsSetupMemSource(t *testing.T) {
	data := []byte{0xAA, 0xBB, 0xCC}
	cinfo := CreateDecompress()
	SetupMemSource(cinfo, data)

	buf := make([]byte, len(data))
	n, err := cinfo.Reader().Read(buf)
	if err != nil {
		t.Fatalf("Reader().Read error: %v", err)
	}
	if n != len(data) {
		t.Fatalf("Reader().Read n = %d, want %d", n, len(data))
	}
	if !bytes.Equal(buf, data) {
		t.Errorf("Reader().Read = %v, want %v", buf, data)
	}
}
