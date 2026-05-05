package marker

import (
	"encoding/binary"
	"io"
)

// markerReader holds the private state for JPEG marker reading.
// Ported from jdmarker.c my_marker_reader.
type markerReader struct {
	// Public state
	SawSOI         bool
	SawSOF         bool
	DiscardedBytes int
	NextRestartNum int

	// Application-overridable marker processing
	processCOM  markerProcessor
	processAPPn [16]markerProcessor

	// Limit on marker data length to save
	lengthLimitCOM  uint
	lengthLimitAPPn [16]uint

	// Status of COM/APPn marker saving
	curMarker *SavedMarker
	bytesRead uint

	// Internal read buffer
	buf    []byte
	bufPos int
	bufLen int
}

// markerProcessor is the function signature for processing a marker.
// Returns error on failure, nil on success.
type markerProcessor func(d *Decompressor) error

// readBytes reads exactly n bytes from the source.
// Returns the bytes or an error (io.ErrUnexpectedEOF if partial).
func (mr *markerReader) readBytes(d *Decompressor, n int) ([]byte, error) {
	if n <= 0 {
		return nil, nil
	}
	result := make([]byte, n)
	totalRead := 0
	for totalRead < n {
		nn, err := d.Src.Read(result[totalRead:])
		totalRead += nn
		if err != nil {
			if err == io.EOF && totalRead < n {
				return nil, io.ErrUnexpectedEOF
			}
			return result[:totalRead], err
		}
	}
	return result, nil
}

// readByte reads one byte from the source.
func (mr *markerReader) readByte(d *Decompressor) (byte, error) {
	b, err := mr.readBytes(d, 1)
	if err != nil {
		return 0, err
	}
	return b[0], nil
}

// readUint16 reads a big-endian uint16 from the source.
func (mr *markerReader) readUint16(d *Decompressor) (uint16, error) {
	b, err := mr.readBytes(d, 2)
	if err != nil {
		return 0, err
	}
	return binary.BigEndian.Uint16(b), nil
}

// readUint32 reads a big-endian uint32 from the source.
func (mr *markerReader) readUint32(d *Decompressor) (uint32, error) {
	b, err := mr.readBytes(d, 4)
	if err != nil {
		return 0, err
	}
	return binary.BigEndian.Uint32(b), nil
}

// skipBytes skips n bytes from the source.
func (mr *markerReader) skipBytes(d *Decompressor, n int64) error {
	if n <= 0 {
		return nil
	}
	_, err := io.CopyN(io.Discard, d.Src, n)
	return err
}

// getSOI processes an SOI marker.
func getSOI(d *Decompressor) error {
	mr := d.marker
	if mr.SawSOI {
		return ErrSOIDuplicate
	}

	// Reset all parameters defined to be reset by SOI
	for i := 0; i < NumArithTbls; i++ {
		d.ArithDCL[i] = 0
		d.ArithDCU[i] = 1
		d.ArithACK[i] = 5
	}
	d.RestartInterval = 0

	// Set initial assumptions
	d.JPEGColorSpace = CSUnknown
	d.ColorTransform = CTNone
	d.CCIR601Sampling = false

	d.SawJFIFMarker = false
	d.JFIFMajorVersion = 1
	d.JFIFMinorVersion = 1
	d.DensityUnit = 0
	d.XDensity = 1
	d.YDensity = 1
	d.SawAdobeMarker = false
	d.AdobeTransform = 0

	mr.SawSOI = true
	return nil
}

// getSOF processes a SOFn marker.
func getSOF(d *Decompressor, isBaseline, isProg, isArith bool) error {
	mr := d.marker

	d.IsBaselineFlag = isBaseline
	d.ProgressiveMode = isProg
	d.ArithCodeFlag = isArith

	length, err := mr.readUint16(d)
	if err != nil {
		return err
	}

	prec, err := mr.readByte(d)
	if err != nil {
		return err
	}
	d.DataPrecision = int(prec)

	height, err := mr.readUint16(d)
	if err != nil {
		return err
	}
	d.ImageHeight = int(height)

	width, err := mr.readUint16(d)
	if err != nil {
		return err
	}
	d.ImageWidth = int(width)

	numComp, err := mr.readByte(d)
	if err != nil {
		return err
	}
	d.NumComponents = int(numComp)

	length -= 8

	if mr.SawSOF {
		return ErrSOFDuplicate
	}

	if d.ImageHeight <= 0 || d.ImageWidth <= 0 || d.NumComponents <= 0 {
		return ErrEmptyImage
	}

	if int(length) != d.NumComponents*3 {
		return ErrBadLength
	}

	if d.CompInfo == nil {
		d.CompInfo = make([]ComponentInfo, d.NumComponents)
	}

	for ci := 0; ci < d.NumComponents; ci++ {
		c, err := mr.readByte(d)
		if err != nil {
			return err
		}

		// Check for duplicate component IDs; create fake ID if needed
		id := int(c)
		for i := 0; i < ci; i++ {
			if id == d.CompInfo[i].ComponentID {
				// Find max ID so far and add 1
				maxID := d.CompInfo[0].ComponentID
				for j := 1; j < ci; j++ {
					if d.CompInfo[j].ComponentID > maxID {
						maxID = d.CompInfo[j].ComponentID
					}
				}
				id = maxID + 1
				break
			}
		}
		d.CompInfo[ci].ComponentID = id
		d.CompInfo[ci].ComponentIndex = ci

		sampling, err := mr.readByte(d)
		if err != nil {
			return err
		}
		d.CompInfo[ci].HSampFactor = int(sampling >> 4)
		d.CompInfo[ci].VSampFactor = int(sampling & 0x0F)

		qtbl, err := mr.readByte(d)
		if err != nil {
			return err
		}
		d.CompInfo[ci].QuantTblNo = int(qtbl)
	}

	mr.SawSOF = true
	return nil
}

// getSOS processes a SOS marker.
func getSOS(d *Decompressor) error {
	mr := d.marker

	if !mr.SawSOF {
		return ErrSOFBeforeSOS
	}

	length, err := mr.readUint16(d)
	if err != nil {
		return err
	}

	n, err := mr.readByte(d)
	if err != nil {
		return err
	}
	numComps := int(n)

	if int(length) != numComps*2+6 || numComps > MaxCompsInScan ||
		(numComps == 0 && !d.ProgressiveMode) {
		return ErrBadLength
	}

	d.CompsInScan = numComps

	for i := 0; i < numComps; i++ {
		c, err := mr.readByte(d)
		if err != nil {
			return err
		}
		cid := int(c)

		// Detect duplicate component IDs in this scan
		for ci := 0; ci < i; ci++ {
			if cid == d.CurCompInfo[ci].ComponentID {
				maxID := d.CurCompInfo[0].ComponentID
				for j := 1; j < i; j++ {
					if d.CurCompInfo[j].ComponentID > maxID {
						maxID = d.CurCompInfo[j].ComponentID
					}
				}
				cid = maxID + 1
				break
			}
		}

		// Find the matching component from SOF
		var found *ComponentInfo
		for ci := 0; ci < d.NumComponents; ci++ {
			if cid == d.CompInfo[ci].ComponentID {
				found = &d.CompInfo[ci]
				break
			}
		}
		if found == nil {
			return ErrBadComponentID
		}

		d.CurCompInfo[i] = found

		tdta, err := mr.readByte(d)
		if err != nil {
			return err
		}
		found.DCTblNo = int(tdta >> 4)
		found.ACTblNo = int(tdta & 0x0F)
	}

	// Collect the additional scan parameters Ss, Se, Ah/Al
	ss, err := mr.readByte(d)
	if err != nil {
		return err
	}
	d.Ss = int(ss)

	se, err := mr.readByte(d)
	if err != nil {
		return err
	}
	d.Se = int(se)

	ahal, err := mr.readByte(d)
	if err != nil {
		return err
	}
	d.Ah = int(ahal >> 4)
	d.Al = int(ahal & 0x0F)

	// Prepare to scan data & restart markers
	mr.NextRestartNum = 0

	// Count another (non-pseudo) SOS marker
	if numComps > 0 {
		d.InputScanNumber++
	}

	return nil
}

// getDHT processes a DHT marker.
func getDHT(d *Decompressor) error {
	mr := d.marker

	length, err := mr.readUint16(d)
	if err != nil {
		return err
	}
	length -= 2

	for length > 16 {
		index, err := mr.readByte(d)
		if err != nil {
			return err
		}

		var bits [17]uint8
		bits[0] = 0
		count := 0
		for i := 1; i <= 16; i++ {
			b, err := mr.readByte(d)
			if err != nil {
				return err
			}
			bits[i] = b
			count += int(b)
		}

		length -= 1 + 16

		if count > 256 || int32(count) > int32(length) {
			return ErrBadHuffTable
		}

		var huffval [256]uint8
		for i := 0; i < count; i++ {
			v, err := mr.readByte(d)
			if err != nil {
				return err
			}
			huffval[i] = v
		}

		length -= uint16(count)

		var htblptr **HuffTable
		if index&0x10 != 0 { // AC table
			idx := int(index - 0x10)
			if idx < 0 || idx >= NumHuffTbls {
				return ErrBadDHTIndex
			}
			htblptr = &d.ACHuffTbls[idx]
		} else { // DC table
			idx := int(index)
			if idx < 0 || idx >= NumHuffTbls {
				return ErrBadDHTIndex
			}
			htblptr = &d.DCHuffTbls[idx]
		}

		if *htblptr == nil {
			t := new(HuffTable)
			*htblptr = t
		}
		(*htblptr).Bits = bits
		for i := 0; i < count; i++ {
			(*htblptr).HuffVal[i] = huffval[i]
		}
	}

	if length != 0 {
		return ErrBadLength
	}

	return nil
}

// getDQT processes a DQT marker.
func getDQT(d *Decompressor) error {
	mr := d.marker

	length, err := mr.readUint16(d)
	if err != nil {
		return err
	}
	length -= 2

	for length > 0 {
		length--
		nprec, err := mr.readByte(d)
		if err != nil {
			return err
		}
		prec := int(nprec >> 4)
		n := int(nprec & 0x0F)

		if n >= NumQuantTbls {
			return ErrBadDQTIndex
		}

		if d.QuantTbls[n] == nil {
			d.QuantTbls[n] = new(QuantTable)
		}
		qtptr := d.QuantTbls[n]

		var count int
		if prec != 0 {
			if int(length) < DCTSize2*2 {
				// Initialize full table for safety
				for i := 0; i < DCTSize2; i++ {
					qtptr.QuantVal[i] = 1
				}
				count = int(length) / 2
			} else {
				count = DCTSize2
			}
		} else {
			if int(length) < DCTSize2 {
				for i := 0; i < DCTSize2; i++ {
					qtptr.QuantVal[i] = 1
				}
				count = int(length)
			} else {
				count = DCTSize2
			}
		}

		// Choose the natural order table based on count
		naturalOrder := NaturalOrder
		switch count {
		case 2 * 2:
			naturalOrder = NaturalOrder2
		case 3 * 3:
			naturalOrder = NaturalOrder3
		case 4 * 4:
			naturalOrder = NaturalOrder4
		case 5 * 5:
			naturalOrder = NaturalOrder5
		case 6 * 6:
			naturalOrder = NaturalOrder6
		case 7 * 7:
			naturalOrder = NaturalOrder7
		}

		for i := 0; i < count; i++ {
			if prec != 0 {
				v, err := mr.readUint16(d)
				if err != nil {
					return err
				}
				qtptr.QuantVal[naturalOrder[i]] = v
			} else {
				v, err := mr.readByte(d)
				if err != nil {
					return err
				}
				qtptr.QuantVal[naturalOrder[i]] = uint16(v)
			}
		}
		if prec != 0 {
			length -= uint16(count * 2)
		} else {
			length -= uint16(count)
		}
	}

	if length != 0 {
		return ErrBadLength
	}

	return nil
}

// getDRI processes a DRI marker.
func getDRI(d *Decompressor) error {
	mr := d.marker

	length, err := mr.readUint16(d)
	if err != nil {
		return err
	}
	if length != 4 {
		return ErrBadLength
	}

	tmp, err := mr.readUint16(d)
	if err != nil {
		return err
	}

	d.RestartInterval = uint(tmp)
	return nil
}

// getDAC processes a DAC marker.
func getDAC(d *Decompressor) error {
	mr := d.marker

	length, err := mr.readUint16(d)
	if err != nil {
		return err
	}
	length -= 2

	for length > 0 {
		index, err := mr.readByte(d)
		if err != nil {
			return err
		}
		val, err := mr.readByte(d)
		if err != nil {
			return err
		}
		length -= 2

		if int(index) >= 2*NumArithTbls {
			return ErrDACIndex
		}

		if int(index) >= NumArithTbls { // AC table
			d.ArithACK[int(index)-NumArithTbls] = val
		} else { // DC table
			d.ArithDCL[int(index)] = val & 0x0F
			d.ArithDCU[int(index)] = val >> 4
			if d.ArithDCL[int(index)] > d.ArithDCU[int(index)] {
				return ErrDACValue
			}
		}
	}

	if length != 0 {
		return ErrBadLength
	}

	return nil
}

// getLSE processes an LSE marker (JPEG extension marker JPG8 = 0xF8).
func getLSE(d *Decompressor) error {
	mr := d.marker

	if !d.marker.SawSOF {
		return ErrSOFBeforeLSE
	}

	if d.NumComponents < 3 {
		return ErrConversionNotImpl
	}

	length, err := mr.readUint16(d)
	if err != nil {
		return err
	}
	if length != 24 {
		return ErrBadLength
	}

	// Read the LSE parameters
	data, err := mr.readBytes(d, 22) // 24 - 2 (length field already read)
	if err != nil {
		return err
	}

	if data[0] != 0x0D { // ID inverse transform specification
		return ErrUnknownMarker
	}

	// MAXTRANS check
	tmp16 := uint16(data[1])<<8 | uint16(data[2])
	if tmp16 != 255 { // MAXJSAMPLE = 255 for 8-bit
		return ErrConversionNotImpl
	}

	if data[3] != 3 { // Nt=3
		return ErrConversionNotImpl
	}

	// Check component IDs match expected ordering
	if int(data[4]) != d.CompInfo[1].ComponentID ||
		int(data[5]) != d.CompInfo[0].ComponentID ||
		int(data[6]) != d.CompInfo[2].ComponentID {
		return ErrConversionNotImpl
	}

	// F1: CENTER1=1, NORM1=0
	if data[7] != 0x80 {
		return ErrConversionNotImpl
	}

	// A(1,1)=0
	tmp16 = uint16(data[8])<<8 | uint16(data[9])
	if tmp16 != 0 {
		return ErrConversionNotImpl
	}

	// A(1,2)=0
	tmp16 = uint16(data[10])<<8 | uint16(data[11])
	if tmp16 != 0 {
		return ErrConversionNotImpl
	}

	// F2: CENTER2=0, NORM2=0
	if data[12] != 0 {
		return ErrConversionNotImpl
	}

	// A(2,1)=1
	tmp16 = uint16(data[13])<<8 | uint16(data[14])
	if tmp16 != 1 {
		return ErrConversionNotImpl
	}

	// A(2,2)=0
	tmp16 = uint16(data[15])<<8 | uint16(data[16])
	if tmp16 != 0 {
		return ErrConversionNotImpl
	}

	// F3: CENTER3=0, NORM3=0
	if data[17] != 0 {
		return ErrConversionNotImpl
	}

	// A(3,1)=1
	tmp16 = uint16(data[18])<<8 | uint16(data[19])
	if tmp16 != 1 {
		return ErrConversionNotImpl
	}

	// A(3,2)=0
	tmp16 = uint16(data[20])<<8 | uint16(data[21])
	if tmp16 != 0 {
		return ErrConversionNotImpl
	}

	d.ColorTransform = CTSubtractGreen
	return nil
}

// examineApp0 examines first few bytes from an APP0 marker.
func examineApp0(d *Decompressor, data []byte, datalen uint, remaining int32) {
	totallen := int32(datalen) + remaining

	if datalen >= 14 && // APP0_DATA_LEN
		data[0] == 0x4A && data[1] == 0x46 && data[2] == 0x49 && data[3] == 0x46 && data[4] == 0 {
		// Found JFIF APP0 marker
		d.SawJFIFMarker = true
		d.JFIFMajorVersion = data[5]
		d.JFIFMinorVersion = data[6]
		d.DensityUnit = data[7]
		d.XDensity = uint16(data[8])<<8 | uint16(data[9])
		d.YDensity = uint16(data[10])<<8 | uint16(data[11])

		if d.JFIFMajorVersion != 1 && d.JFIFMajorVersion != 2 {
			// warning: nonstandard JFIF major version (non-fatal)
		}
	} else if datalen >= 6 &&
		data[0] == 0x4A && data[1] == 0x46 && data[2] == 0x58 && data[3] == 0x58 && data[4] == 0 {
		// Found JFIF "JFXX" extension APP0 marker (just trace)
	} else {
		// Start of APP0 does not match "JFIF" or "JFXX" (just trace)
	}
	_ = totallen
}

// examineApp14 examines first few bytes from an APP14 marker.
func examineApp14(d *Decompressor, data []byte, datalen uint, remaining int32) {
	if datalen >= 12 && // APP14_DATA_LEN
		data[0] == 0x41 && data[1] == 0x64 && data[2] == 0x6F && data[3] == 0x62 && data[4] == 0x65 {
		// Found Adobe APP14 marker
		_ = uint16(data[5])<<8 | uint16(data[6])  // version
		_ = uint16(data[7])<<8 | uint16(data[8])  // flags0
		_ = uint16(data[9])<<8 | uint16(data[10]) // flags1
		transform := data[11]

		d.SawAdobeMarker = true
		d.AdobeTransform = transform
	}
}

// getInterestingAppn processes an APP0 or APP14 marker without saving it.
func getInterestingAppn(d *Decompressor) error {
	mr := d.marker
	const appNDataLen = 14

	length, err := mr.readUint16(d)
	if err != nil {
		return err
	}
	length -= 2

	var numToRead uint
	if int(length) >= appNDataLen {
		numToRead = appNDataLen
	} else if length > 0 {
		numToRead = uint(length)
	} else {
		numToRead = 0
	}

	var b [appNDataLen]byte
	if numToRead > 0 {
		data, err := mr.readBytes(d, int(numToRead))
		if err != nil {
			return err
		}
		copy(b[:numToRead], data)
	}
	remaining := int32(int(length) - int(numToRead))

	switch d.UnreadMarker {
	case M_APP0:
		examineApp0(d, b[:numToRead], numToRead, remaining)
	case M_APP14:
		examineApp14(d, b[:numToRead], numToRead, remaining)
	default:
		return ErrUnknownMarker
	}

	// Skip any remaining data
	if remaining > 0 {
		if err := mr.skipBytes(d, int64(remaining)); err != nil {
			return err
		}
	}

	return nil
}

// saveMarker saves an APPn or COM marker into the marker list.
func saveMarker(d *Decompressor) error {
	mr := d.marker
	curMarker := mr.curMarker

	var dataLength uint
	var data []byte
	var length int32

	if curMarker == nil {
		// Begin reading a marker
		var l uint16
		l, err := mr.readUint16(d)
		if err != nil {
			return err
		}
		length = int32(l) - 2
		if length >= 0 {
			var limit uint
			if d.UnreadMarker == M_COM {
				limit = mr.lengthLimitCOM
			} else {
				limit = mr.lengthLimitAPPn[d.UnreadMarker-M_APP0]
			}
			if uint(length) < limit {
				limit = uint(length)
			}

			curMarker = &SavedMarker{
				Marker:         uint8(d.UnreadMarker),
				OriginalLength: uint(length),
				DataLength:     limit,
			}
			curMarker.Data = make([]byte, limit)

			mr.curMarker = curMarker
			mr.bytesRead = 0
			dataLength = limit
			data = curMarker.Data
		} else {
			dataLength = 0
			data = nil
		}
	} else {
		// Resume reading a marker
		dataLength = curMarker.DataLength
		data = curMarker.Data[mr.bytesRead:]
	}

	if dataLength > 0 && data != nil {
		bytesRead := mr.bytesRead
		remaining := dataLength - bytesRead
		if remaining > 0 {
			n, err := d.Src.Read(data[bytesRead:])
			if err != nil && n == 0 {
				return err
			}
			mr.bytesRead = uint(n) + bytesRead
			if mr.bytesRead < dataLength {
				return ErrSuspension
			}
		}
	}

	// Done reading what we want to read
	if curMarker != nil {
		// Add new marker to end of list
		if d.MarkerList == nil {
			d.MarkerList = curMarker
		} else {
			prev := d.MarkerList
			for prev.Next != nil {
				prev = prev.Next
			}
			prev.Next = curMarker
		}
		data = curMarker.Data
		length = int32(curMarker.OriginalLength - curMarker.DataLength)
	}

	// Reset to initial state for next marker
	mr.curMarker = nil

	// Process the marker if interesting
	switch d.UnreadMarker {
	case M_APP0:
		examineApp0(d, data, curMarker.DataLength, length)
	case M_APP14:
		examineApp14(d, data, curMarker.DataLength, length)
	}

	// Skip any remaining data
	if length > 0 {
		if err := mr.skipBytes(d, int64(length)); err != nil {
			return err
		}
	}

	return nil
}

// skipVariable skips over an unknown or uninteresting variable-length marker.
func skipVariable(d *Decompressor) error {
	mr := d.marker

	length, err := mr.readUint16(d)
	if err != nil {
		return err
	}
	length -= 2

	if length > 0 {
		if err := mr.skipBytes(d, int64(length)); err != nil {
			return err
		}
	}

	return nil
}

// nextMarker finds the next JPEG marker, saves it in d.UnreadMarker.
func nextMarker(d *Decompressor) error {
	mr := d.marker
	var c byte

	for {
		var err error
		c, err = mr.readByte(d)
		if err != nil {
			return err
		}
		// Skip any non-FF bytes
		for c != 0xFF {
			mr.DiscardedBytes++
			c, err = mr.readByte(d)
			if err != nil {
				return err
			}
		}
		// Swallow any duplicate FF bytes
		for {
			c, err = mr.readByte(d)
			if err != nil {
				return err
			}
			if c != 0xFF {
				break
			}
		}
		if c != 0 {
			break // found a valid marker
		}
		// FF/00 stuffed-zero data sequence, discard and loop
		mr.DiscardedBytes += 2
	}

	if mr.DiscardedBytes != 0 {
		mr.DiscardedBytes = 0
	}

	d.UnreadMarker = int(c)
	return nil
}

// firstMarker reads the initial SOI marker.
func firstMarker(d *Decompressor) error {
	mr := d.marker

	c, err := mr.readByte(d)
	if err != nil {
		return err
	}
	c2, err := mr.readByte(d)
	if err != nil {
		return err
	}
	if c != 0xFF || c2 != M_SOI {
		return ErrNoSOI
	}

	d.UnreadMarker = int(c2)
	return nil
}

// readMarkers reads markers until SOS or EOI.
// Returns JPEGReachedSOS, JPEGReachedEOI, or an error.
func readMarkers(d *Decompressor) (int, error) {
	mr := d.marker

	for {
		// Collect the marker proper, unless we already did.
		if d.UnreadMarker == 0 {
			if !mr.SawSOI {
				if err := firstMarker(d); err != nil {
					return JPEGSuspended, err
				}
			} else {
				if err := nextMarker(d); err != nil {
					return JPEGSuspended, err
				}
			}
		}

		switch d.UnreadMarker {
		case M_SOI:
			if err := getSOI(d); err != nil {
				return JPEGSuspended, err
			}

		case M_SOF0: // Baseline
			if err := getSOF(d, true, false, false); err != nil {
				return JPEGSuspended, err
			}

		case M_SOF1: // Extended sequential, Huffman
			if err := getSOF(d, false, false, false); err != nil {
				return JPEGSuspended, err
			}

		case M_SOF2: // Progressive, Huffman
			if err := getSOF(d, false, true, false); err != nil {
				return JPEGSuspended, err
			}

		case M_SOF9: // Extended sequential, arithmetic
			if err := getSOF(d, false, false, true); err != nil {
				return JPEGSuspended, err
			}

		case M_SOF10: // Progressive, arithmetic
			if err := getSOF(d, false, true, true); err != nil {
				return JPEGSuspended, err
			}

		// Currently unsupported SOFn types
		case M_SOF3, M_SOF5, M_SOF6, M_SOF7, M_JPG,
			M_SOF11, M_SOF13, M_SOF14, M_SOF15:
			return JPEGSuspended, ErrSOFUnsupported

		case M_SOS:
			if err := getSOS(d); err != nil {
				return JPEGSuspended, err
			}
			d.UnreadMarker = 0
			return JPEGReachedSOS, nil

		case M_EOI:
			d.UnreadMarker = 0
			return JPEGReachedEOI, nil

		case M_DAC:
			if err := getDAC(d); err != nil {
				return JPEGSuspended, err
			}

		case M_DHT:
			if err := getDHT(d); err != nil {
				return JPEGSuspended, err
			}

		case M_DQT:
			if err := getDQT(d); err != nil {
				return JPEGSuspended, err
			}

		case M_DRI:
			if err := getDRI(d); err != nil {
				return JPEGSuspended, err
			}

		case M_JPG8:
			if err := getLSE(d); err != nil {
				return JPEGSuspended, err
			}

		case M_APP0, M_APP1, M_APP2, M_APP3, M_APP4,
			M_APP5, M_APP6, M_APP7, M_APP8, M_APP9,
			M_APP10, M_APP11, M_APP12, M_APP13, M_APP14, M_APP15:
			if err := mr.processAPPn[d.UnreadMarker-M_APP0](d); err != nil {
				return JPEGSuspended, err
			}

		case M_COM:
			if err := mr.processCOM(d); err != nil {
				return JPEGSuspended, err
			}

		case M_RST0, M_RST1, M_RST2, M_RST3,
			M_RST4, M_RST5, M_RST6, M_RST7, M_TEM:
			// Parameterless markers, just note them

		case M_DNL: // Ignore DNL
			if err := skipVariable(d); err != nil {
				return JPEGSuspended, err
			}

		default:
			// Unknown marker, try to skip it
			if err := skipVariable(d); err != nil {
				return JPEGSuspended, err
			}
		}

		// Successfully processed marker, reset state
		d.UnreadMarker = 0
	}
}

// readRestartMarker reads a restart marker.
// Returns true if a marker was consumed, false if suspension.
func readRestartMarker(d *Decompressor) error {
	mr := d.marker

	// Obtain a marker unless we already did
	if d.UnreadMarker == 0 {
		if err := nextMarker(d); err != nil {
			return err
		}
	}

	if d.UnreadMarker == (M_RST0 + mr.NextRestartNum) {
		// Normal case --- swallow the marker
		d.UnreadMarker = 0
	} else {
		// Restart markers are messed up, try to resync
		if err := resyncToRestart(d, mr.NextRestartNum); err != nil {
			return err
		}
	}

	// Update next-restart state
	mr.NextRestartNum = (mr.NextRestartNum + 1) & 7
	return nil
}

// resyncToRestart attempts to resynchronize the restart marker stream.
func resyncToRestart(d *Decompressor, desired int) error {
	marker := d.UnreadMarker

	for {
		action := 1

		if marker < M_SOF0 {
			action = 2 // invalid marker
		} else if marker < M_RST0 || marker > M_RST7 {
			action = 3 // valid non-restart marker
		} else {
			if marker == M_RST0+((desired+1)&7) ||
				marker == M_RST0+((desired+2)&7) {
				action = 3 // one of the next two expected restarts
			} else if marker == M_RST0+((desired-1)&7) ||
				marker == M_RST0+((desired-2)&7) {
				action = 2 // a prior restart, so advance
			} else {
				action = 1 // desired restart or too far away
			}
		}

		switch action {
		case 1:
			// Discard marker and let entropy decoder resume
			d.UnreadMarker = 0
			return nil
		case 2:
			// Scan to the next marker, repeat the decision loop
			if err := nextMarker(d); err != nil {
				return err
			}
			marker = d.UnreadMarker
		case 3:
			// Return without advancing past this marker
			return nil
		}
	}
}

// resetMarkerReader resets marker processing state for a fresh datastream.
func resetMarkerReader(d *Decompressor) {
	mr := d.marker
	d.CompInfo = nil
	d.InputScanNumber = 0
	d.UnreadMarker = 0
	mr.SawSOI = false
	mr.SawSOF = false
	mr.DiscardedBytes = 0
	mr.curMarker = nil
}

// initMarkerReader initializes the marker reader module.
func initMarkerReader(d *Decompressor) {
	mr := &markerReader{}

	d.marker = mr

	// Initialize COM/APPn processing
	mr.processCOM = skipVariable
	mr.lengthLimitCOM = 0
	for i := 0; i < 16; i++ {
		mr.processAPPn[i] = skipVariable
		mr.lengthLimitAPPn[i] = 0
	}
	mr.processAPPn[0] = getInterestingAppn
	mr.processAPPn[14] = getInterestingAppn

	resetMarkerReader(d)
}

// SaveMarkers controls saving of COM and APPn markers into marker_list.
// markerCode is a marker code such as M_COM or M_APP0..M_APP15.
// lengthLimit is the maximum number of bytes of marker data to save (0 means
// skip the marker data entirely).
func (d *Decompressor) SaveMarkers(markerCode int, lengthLimit uint) {
	mr := d.marker

	var processor markerProcessor

	if lengthLimit > 0 {
		processor = saveMarker
		// If saving APP0/APP14, save at least enough for internal use
		if markerCode == M_APP0 && lengthLimit < 14 {
			lengthLimit = 14
		} else if markerCode == M_APP14 && lengthLimit < 12 {
			lengthLimit = 12
		}
	} else {
		processor = skipVariable
		if markerCode == M_APP0 || markerCode == M_APP14 {
			processor = getInterestingAppn
		}
	}

	if markerCode == M_COM {
		mr.processCOM = processor
		mr.lengthLimitCOM = lengthLimit
	} else if markerCode >= M_APP0 && markerCode <= M_APP15 {
		mr.processAPPn[markerCode-M_APP0] = processor
		mr.lengthLimitAPPn[markerCode-M_APP0] = lengthLimit
	}
}

// SetMarkerProcessor installs a special processing method for COM or APPn markers.
func (d *Decompressor) SetMarkerProcessor(markerCode int, processor func(d *Decompressor) error) {
	mr := d.marker
	if markerCode == M_COM {
		mr.processCOM = processor
	} else if markerCode >= M_APP0 && markerCode <= M_APP15 {
		mr.processAPPn[markerCode-M_APP0] = processor
	}
}
