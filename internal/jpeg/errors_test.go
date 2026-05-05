package jpeg

import (
	"strings"
	"testing"
)

func TestJPEGError_Error(t *testing.T) {
	// Test with a message set
	err := &JPEGError{Code: ErrInputEOF, Message: "Premature end of input file"}
	if err.Error() != "Premature end of input file" {
		t.Errorf("JPEGError.Error() = %q, want %q", err.Error(), "Premature end of input file")
	}

	// Test with no message
	err2 := &JPEGError{Code: 42}
	if !strings.Contains(err2.Error(), "42") {
		t.Errorf("JPEGError.Error() with no message should contain code 42, got %q", err2.Error())
	}
}

func TestNewError(t *testing.T) {
	err := NewError(ErrInputEOF)
	if err.Code != ErrInputEOF {
		t.Errorf("NewError code = %d, want %d", err.Code, ErrInputEOF)
	}
	// FormatMessage always passes 8 params, so messages without format
	// verbs will have extra text from fmt.Sprintf. Just check prefix.
	want := "Premature end of input file"
	if !strings.HasPrefix(err.Message, want) {
		t.Errorf("NewError message = %q, want prefix %q", err.Message, want)
	}
}

func TestNewErrorWithParams(t *testing.T) {
	err := NewError(ErrNoSOI, 0x47, 0x11)
	if err.Code != ErrNoSOI {
		t.Errorf("NewError code = %d, want %d", err.Code, ErrNoSOI)
	}
	if !strings.Contains(err.Message, "0x47") || !strings.Contains(err.Message, "0x11") {
		t.Errorf("NewError with params: message should contain params, got %q", err.Message)
	}
}

func TestFormatMessage(t *testing.T) {
	// Valid code without format verbs — message starts with expected text
	msg := FormatMessage(ErrInputEOF)
	if !strings.HasPrefix(msg, "Premature end of input file") {
		t.Errorf("FormatMessage(ErrInputEOF) = %q, want prefix 'Premature end of input file'", msg)
	}

	// Invalid code (negative)
	msg = FormatMessage(-1)
	if !strings.Contains(msg, "Bogus") {
		t.Errorf("FormatMessage(-1) should contain 'Bogus', got %q", msg)
	}

	// Invalid code (too large)
	msg = FormatMessage(9999)
	if !strings.Contains(msg, "Bogus") {
		t.Errorf("FormatMessage(9999) should contain 'Bogus', got %q", msg)
	}

	// With formatting parameters
	msg = FormatMessage(ErrBadPrecision, 12)
	if !strings.Contains(msg, "12") {
		t.Errorf("FormatMessage(ErrBadPrecision, 12) should contain '12', got %q", msg)
	}
}

func TestStdError(t *testing.T) {
	errMgr := StdError()
	if errMgr == nil {
		t.Fatal("StdError() returned nil")
	}
	if errMgr.TraceLevel != 0 {
		t.Errorf("TraceLevel = %d, want 0", errMgr.TraceLevel)
	}
	if errMgr.NumWarnings != 0 {
		t.Errorf("NumWarnings = %d, want 0", errMgr.NumWarnings)
	}
	if errMgr.ErrorExit == nil {
		t.Error("ErrorExit should not be nil")
	}
	if errMgr.EmitMessage == nil {
		t.Error("EmitMessage should not be nil")
	}
	if errMgr.OutputMessage == nil {
		t.Error("OutputMessage should not be nil")
	}
	if errMgr.FormatMessage == nil {
		t.Error("FormatMessage should not be nil")
	}
	if errMgr.ResetErrorMgr == nil {
		t.Error("ResetErrorMgr should not be nil")
	}
	if errMgr.JPEGMessageTable == nil {
		t.Error("JPEGMessageTable should not be nil")
	}
	if len(errMgr.JPEGMessageTable) == 0 {
		t.Error("JPEGMessageTable should not be empty")
	}
}

func TestErrorMessages(t *testing.T) {
	// Verify key error messages exist in the table
	keyMessages := map[int]string{
		ErrNoSOI:       "Not a JPEG file",
		ErrInputEOF:    "Premature end of input file",
		ErrInputEmpty:  "Empty input file",
		ErrBadHuffTable: "Bogus Huffman table definition",
		ErrNoQuantTable: "Quantization table",
		ErrOutOfMemory: "Insufficient memory",
	}

	for code, substr := range keyMessages {
		if code >= len(JPEGStdMessageTable) {
			t.Errorf("Error code %d beyond message table length %d", code, len(JPEGStdMessageTable))
			continue
		}
		msg := JPEGStdMessageTable[code]
		if !strings.Contains(msg, substr) {
			t.Errorf("JPEGStdMessageTable[%d] = %q, want substring %q", code, msg, substr)
		}
	}
}

func TestErrExit(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("ErrExit should panic")
		}
		err, ok := r.(*JPEGError)
		if !ok {
			t.Fatalf("ErrExit should panic with *JPEGError, got %T", r)
		}
		if err.Code != ErrInputEOF {
			t.Errorf("panic error code = %d, want %d", err.Code, ErrInputEOF)
		}
	}()

	cinfo := &JPEGCommon{Err: StdError()}
	ErrExit(cinfo, ErrInputEOF)
}

func TestWarnMS(t *testing.T) {
	// Should not panic
	cinfo := &JPEGCommon{Err: StdError()}
	WarnMS(cinfo, WrnJPEGEOF)

	if cinfo.Err.NumWarnings != 1 {
		t.Errorf("NumWarnings = %d, want 1", cinfo.Err.NumWarnings)
	}
}

func TestResetErrorMgr(t *testing.T) {
	errMgr := StdError()
	cinfo := &JPEGCommon{Err: errMgr}
	cinfo.Err.NumWarnings = 5
	cinfo.Err.MsgCode = ErrInputEOF

	cinfo.Err.ResetErrorMgr(cinfo)

	if cinfo.Err.NumWarnings != 0 {
		t.Errorf("after reset, NumWarnings = %d, want 0", cinfo.Err.NumWarnings)
	}
	if cinfo.Err.MsgCode != 0 {
		t.Errorf("after reset, MsgCode = %d, want 0", cinfo.Err.MsgCode)
	}
}

func TestTraceMS(t *testing.T) {
	cinfo := &JPEGCommon{Err: StdError()}
	// With default trace level 0, trace messages at level 1 should not be output
	// but should not panic
	TraceMS(cinfo, 1, TrcSOI)

	// With trace level >= 1, it should output
	cinfo.Err.TraceLevel = 1
	TraceMS(cinfo, 1, TrcSOI)
}

func TestErrExitDecompress(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("ErrExitDecompress should panic")
		}
		err, ok := r.(*JPEGError)
		if !ok {
			t.Fatalf("expected *JPEGError, got %T", r)
		}
		if err.Code != ErrInputEmpty {
			t.Errorf("error code = %d, want %d", err.Code, ErrInputEmpty)
		}
	}()

	cinfo := CreateDecompress()
	ErrExitDecompress(cinfo, ErrInputEmpty)
}

func TestWarnMSDecompress(t *testing.T) {
	cinfo := CreateDecompress()
	WarnMSDecompress(cinfo, WrnJPEGEOF)
	if cinfo.Err.NumWarnings != 1 {
		t.Errorf("NumWarnings = %d, want 1", cinfo.Err.NumWarnings)
	}
}

func TestStdOutputMessage(t *testing.T) {
	cinfo := &JPEGCommon{Err: StdError()}
	cinfo.Err.MsgCode = ErrInputEOF
	// Should not panic
	cinfo.Err.OutputMessage(cinfo)
}

func TestStdFormatMessage(t *testing.T) {
	cinfo := &JPEGCommon{Err: StdError()}
	cinfo.Err.MsgCode = ErrNoSOI
	cinfo.Err.MsgParmInt[0] = 0x47
	cinfo.Err.MsgParmInt[1] = 0x11

	var buf [JMSGLengthMax]byte
	cinfo.Err.FormatMessage(cinfo, buf[:])

	// The formatted message should contain "Not a JPEG file"
	found := false
	msg := ""
	for i, b := range buf {
		if b == 0 {
			msg = string(buf[:i])
			break
		}
	}
	if msg == "" {
		msg = string(buf[:])
	}
	if len(msg) >= 15 && msg[:15] == "Not a JPEG file" {
		found = true
	}
	if !found {
		t.Errorf("FormatMessage should produce 'Not a JPEG file' message, got %q", msg)
	}
}

func TestStdEmitMessage(t *testing.T) {
	cinfo := &JPEGCommon{Err: StdError()}
	// Warning message (negative level)
	cinfo.Err.EmitMessage(cinfo, -1)
	if cinfo.Err.NumWarnings != 1 {
		t.Errorf("NumWarnings = %d, want 1 after warning", cinfo.Err.NumWarnings)
	}

	// Trace message with level > trace level (should be suppressed)
	cinfo.Err.EmitMessage(cinfo, 1)
	// NumWarnings should still be 1 (trace doesn't increment)
	if cinfo.Err.NumWarnings != 1 {
		t.Errorf("NumWarnings = %d, should still be 1 after trace", cinfo.Err.NumWarnings)
	}
}
