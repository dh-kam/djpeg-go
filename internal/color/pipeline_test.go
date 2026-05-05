package color

import (
	"testing"
)

// helperDecompInfo creates a minimal DecompressInfo for testing.
func helperDecompInfo() *DecompressInfo {
	return &DecompressInfo{
		OutputWidth:     8,
		OutputHeight:    8,
		MaxHSampFactor:  1,
		MaxVSampFactor:  1,
		MinDCTHScalSize: 8,
		MinDCTVScalSize: 8,
		NumComponents:   1,
		OutColorComponents: 1,
		OutputComponents:   1,
		JpegColorSpace:   JCS_GRAYSCALE,
		OutColorSpace:    JCS_GRAYSCALE,
		CompInfo: []ComponentInfo{
			{
				ComponentNeeded: true,
				ComponentIndex:  0,
				HSampFactor:     1,
				VSampFactor:     1,
				DCTHScalSize:    8,
				DCTVScalSize:    8,
				WidthInBlocks:   1,
				HeightInBlocks:  1,
				DownsampledWidth:  8,
				DownsampledHeight: 8,
				MCUWidth:       1,
				MCUHeight:      1,
				MCUBlocks:      1,
				MCUSampleWidth: 8,
				LastColWidth:   1,
				LastRowHeight:  1,
			},
		},
	}
}

// ---- CoefController tests ----

func TestNewCoefControllerBaseline(t *testing.T) {
	info := helperDecompInfo()
	cc := NewCoefController(info, false)
	if cc == nil {
		t.Fatal("NewCoefController returned nil for baseline")
	}
	if cc.UseFullBuffer {
		t.Error("baseline should not use full buffer")
	}
	if cc.ConsumeData == nil {
		t.Error("ConsumeData should be set")
	}
	if cc.DecompressData == nil {
		t.Error("DecompressData should be set")
	}
	if len(cc.BlkBuffer) != DMaxBlocksInMCU*DCTSize2 {
		t.Errorf("BlkBuffer len = %d, want %d", len(cc.BlkBuffer), DMaxBlocksInMCU*DCTSize2)
	}
}

func TestNewCoefControllerProgressive(t *testing.T) {
	info := helperDecompInfo()
	cc := NewCoefController(info, true)
	if cc == nil {
		t.Fatal("NewCoefController returned nil for progressive")
	}
	if !cc.UseFullBuffer {
		t.Error("progressive should use full buffer")
	}
	if cc.WholeImage == nil {
		t.Error("WholeImage should be allocated")
	}
}

func TestCoefControllerGetMCUBuffer(t *testing.T) {
	info := helperDecompInfo()
	cc := NewCoefController(info, false)
	buf := cc.GetMCUBuffer()
	if buf == nil {
		t.Fatal("GetMCUBuffer returned nil")
	}
	if len(buf) != DMaxBlocksInMCU {
		t.Errorf("MCUBuffer len = %d, want %d", len(buf), DMaxBlocksInMCU)
	}
}

func TestCoefControllerDummyConsumeData(t *testing.T) {
	info := helperDecompInfo()
	cc := NewCoefController(info, false)
	result := cc.dummyConsumeData(info, func() bool { return true })
	if result != JPEGSuspended {
		t.Errorf("dummyConsumeData = %d, want JPEGSuspended", result)
	}
}

func TestCoefControllerStartInputPass(t *testing.T) {
	info := helperDecompInfo()
	cc := NewCoefController(info, false)
	info.InputIMCURow = 0
	info.CompsInScan = 1
	info.CurCompInfo = []*ComponentInfo{&info.CompInfo[0]}
	cc.StartInputPass(info)
	if info.InputIMCURow != 0 {
		t.Errorf("InputIMCURow = %d, want 0", info.InputIMCURow)
	}
}

func TestCoefControllerStartInputPassMultiComponent(t *testing.T) {
	info := &DecompressInfo{
		OutputWidth:     16,
		OutputHeight:    16,
		MaxHSampFactor:  2,
		MaxVSampFactor:  2,
		MinDCTHScalSize: 8,
		MinDCTVScalSize: 8,
		NumComponents:   3,
		CompsInScan:     3,
		TotalIMCURows:   2,
		CompInfo: []ComponentInfo{
			{ComponentNeeded: true, HSampFactor: 2, VSampFactor: 2, DCTHScalSize: 8, DCTVScalSize: 8, LastRowHeight: 2},
			{ComponentNeeded: true, HSampFactor: 1, VSampFactor: 1, DCTHScalSize: 8, DCTVScalSize: 8, LastRowHeight: 1},
			{ComponentNeeded: true, HSampFactor: 1, VSampFactor: 1, DCTHScalSize: 8, DCTVScalSize: 8, LastRowHeight: 1},
		},
		CurCompInfo: []*ComponentInfo{
			{VSampFactor: 2, LastRowHeight: 2},
		},
	}
	cc := NewCoefController(info, false)
	cc.StartInputPass(info)
	if cc.MCUCtr != 0 {
		t.Errorf("MCUCtr = %d, want 0", cc.MCUCtr)
	}
}

func TestCoefControllerStartOutputPass(t *testing.T) {
	info := helperDecompInfo()
	cc := NewCoefController(info, true)
	cc.StartOutputPass(info)
	if info.OutputIMCURow != 0 {
		t.Errorf("OutputIMCURow = %d, want 0", info.OutputIMCURow)
	}
}

func TestCoefControllerStartOutputPassWithSmoothing(t *testing.T) {
	info := helperDecompInfo()
	info.ProgressiveMode = true
	info.DoBlockSmoothing = true
	info.CoefBits = [][]int{{0, 0, 0, 0, 0, 0}}
	info.CompInfo[0].QuantTable = &QuantTable{}
	for i := 0; i < 64; i++ {
		info.CompInfo[0].QuantTable.QuantVal[i] = 1
	}

	cc := NewCoefController(info, true)
	cc.CoefBitsLatch = make([]int, 6)

	// smoothingOK returns false because coefBits[0][1..5] all 0 => smoothingUseful = false
	cc.StartOutputPass(info)
	if cc.DecompressData == nil {
		t.Error("DecompressData should be set")
	}
}

func TestSmoothPredict(t *testing.T) {
	// Test positive num
	got := smoothPredict(100, 10, 2)
	if got < 0 {
		t.Errorf("smoothPredict(100, 10, 2) = %d, want non-negative", got)
	}

	// Test negative num
	got = smoothPredict(-100, 10, 2)
	if got > 0 {
		t.Errorf("smoothPredict(-100, 10, 2) = %d, want non-positive", got)
	}

	// Test zero
	got = smoothPredict(0, 10, 0)
	if got != 0 {
		t.Errorf("smoothPredict(0, 10, 0) = %d, want 0", got)
	}

	// Test clamping
	got = smoothPredict(100000, 1, 1)
	if got > 1 {
		t.Errorf("smoothPredict(100000, 1, 1) = %d, want <= 1", got)
	}
}

func TestSmoothingOKNotProgressive(t *testing.T) {
	info := helperDecompInfo()
	info.ProgressiveMode = false
	cc := NewCoefController(info, true)
	if cc.smoothingOK(info) {
		t.Error("smoothingOK should return false for non-progressive")
	}
}

func TestSmoothingOKNoCoefBits(t *testing.T) {
	info := helperDecompInfo()
	info.ProgressiveMode = true
	cc := NewCoefController(info, true)
	if cc.smoothingOK(info) {
		t.Error("smoothingOK should return false with no coef bits")
	}
}

func TestSmoothingOKDCUnknown(t *testing.T) {
	info := helperDecompInfo()
	info.ProgressiveMode = true
	info.CoefBits = [][]int{{-1, 0, 0, 0, 0, 0}}
	cc := NewCoefController(info, true)
	if cc.smoothingOK(info) {
		t.Error("smoothingOK should return false when DC is unknown")
	}
}

func TestSmoothingOKNoQuantTable(t *testing.T) {
	info := helperDecompInfo()
	info.ProgressiveMode = true
	info.CoefBits = [][]int{{0, 0, 0, 0, 0, 0}}
	// No quant table set
	cc := NewCoefController(info, true)
	if cc.smoothingOK(info) {
		t.Error("smoothingOK should return false with no quant table")
	}
}

func TestSmoothingOKZeroQuantizer(t *testing.T) {
	info := helperDecompInfo()
	info.ProgressiveMode = true
	info.CoefBits = [][]int{{0, 1, 1, 1, 1, 1}}
	info.CompInfo[0].QuantTable = &QuantTable{}
	// QuantVal[0] is 0 => should return false
	cc := NewCoefController(info, true)
	if cc.smoothingOK(info) {
		t.Error("smoothingOK should return false with zero quantizer")
	}
}

func TestSmoothingOKSuccess(t *testing.T) {
	info := helperDecompInfo()
	info.ProgressiveMode = true
	info.CoefBits = [][]int{{0, 1, 1, 1, 1, 1}}
	info.CompInfo[0].QuantTable = &QuantTable{}
	for i := 0; i < 64; i++ {
		info.CompInfo[0].QuantTable.QuantVal[i] = int16(i + 1)
	}
	cc := NewCoefController(info, true)
	if !cc.smoothingOK(info) {
		t.Error("smoothingOK should return true with valid config")
	}
}

func TestDecompressOnePassScanCompleted(t *testing.T) {
	info := helperDecompInfo()
	info.MCUsPerRow = 1
	info.TotalIMCURows = 1
	info.CompsInScan = 1
	info.CurCompInfo = []*ComponentInfo{&info.CompInfo[0]}

	cc := NewCoefController(info, false)
	cc.MCURowsPerIMCU = 1

	outputBuf := make([][][]byte, 1)
	outputBuf[0] = make([][]byte, 8)
	for r := range outputBuf[0] {
		outputBuf[0][r] = make([]byte, 8)
	}

	result := cc.decompressOnePass(info, outputBuf)
	if result != JPEGScanCompleted {
		t.Errorf("decompressOnePass = %d, want JPEGScanCompleted", result)
	}
}

func TestDecompressOnePassRowCompleted(t *testing.T) {
	info := helperDecompInfo()
	info.MCUsPerRow = 1
	info.TotalIMCURows = 2
	info.CompsInScan = 1
	info.CurCompInfo = []*ComponentInfo{&info.CompInfo[0]}

	cc := NewCoefController(info, false)
	cc.MCURowsPerIMCU = 1

	outputBuf := make([][][]byte, 1)
	outputBuf[0] = make([][]byte, 8)
	for r := range outputBuf[0] {
		outputBuf[0][r] = make([]byte, 8)
	}

	result := cc.decompressOnePass(info, outputBuf)
	if result != JPEGRowCompleted {
		t.Errorf("decompressOnePass = %d, want JPEGRowCompleted", result)
	}
}

func TestConsumeDataProgressive(t *testing.T) {
	info := helperDecompInfo()
	info.MCUsPerRow = 1
	info.TotalIMCURows = 1
	info.CompsInScan = 1
	info.CurCompInfo = []*ComponentInfo{&info.CompInfo[0]}

	cc := NewCoefController(info, true)
	// Progressive mode needs MCUBuffer allocated
	cc.MCUBuffer = make([][]int16, DMaxBlocksInMCU)
	cc.BlkBuffer = make([]int16, DMaxBlocksInMCU*DCTSize2)
	for i := range cc.MCUBuffer {
		cc.MCUBuffer[i] = cc.BlkBuffer[i*DCTSize2 : (i+1)*DCTSize2]
	}
	cc.MCURowsPerIMCU = 1

	decodeCount := 0
	result := cc.consumeData(info, func() bool {
		decodeCount++
		return true
	})
	if result != JPEGScanCompleted {
		t.Errorf("consumeData = %d, want JPEGScanCompleted", result)
	}
	if decodeCount != 1 {
		t.Errorf("decodeMCU called %d times, want 1", decodeCount)
	}
}

func TestConsumeDataSuspended(t *testing.T) {
	info := helperDecompInfo()
	info.MCUsPerRow = 1
	info.TotalIMCURows = 1
	info.CompsInScan = 1
	info.CurCompInfo = []*ComponentInfo{&info.CompInfo[0]}

	cc := NewCoefController(info, true)
	cc.MCUBuffer = make([][]int16, DMaxBlocksInMCU)
	cc.BlkBuffer = make([]int16, DMaxBlocksInMCU*DCTSize2)
	for i := range cc.MCUBuffer {
		cc.MCUBuffer[i] = cc.BlkBuffer[i*DCTSize2 : (i+1)*DCTSize2]
	}
	cc.MCURowsPerIMCU = 1

	result := cc.consumeData(info, func() bool { return false })
	if result != JPEGSuspended {
		t.Errorf("consumeData = %d, want JPEGSuspended", result)
	}
}

func TestBuildMCUPtrList(t *testing.T) {
	info := helperDecompInfo()
	info.MCUsPerRow = 1
	info.CompsInScan = 1
	info.CurCompInfo = []*ComponentInfo{&info.CompInfo[0]}

	cc := NewCoefController(info, true)
	cc.MCUBuffer = make([][]int16, DMaxBlocksInMCU)
	cc.BlkBuffer = make([]int16, DMaxBlocksInMCU*DCTSize2)
	for i := range cc.MCUBuffer {
		cc.MCUBuffer[i] = cc.BlkBuffer[i*DCTSize2 : (i+1)*DCTSize2]
	}
	cc.MCURowsPerIMCU = 1
	cc.buildMCUPtrList(info, 0, 0)
	// Should not panic
}

func TestDecompressDataProgressive(t *testing.T) {
	info := helperDecompInfo()
	info.TotalIMCURows = 1
	info.OutputIMCURow = 0

	cc := NewCoefController(info, true)
	cc.MCURowsPerIMCU = 1

	outputBuf := make([][][]byte, 1)
	outputBuf[0] = make([][]byte, 8)
	for r := range outputBuf[0] {
		outputBuf[0][r] = make([]byte, 8)
	}

	result := cc.decompressData(info, outputBuf)
	if result != JPEGScanCompleted {
		t.Errorf("decompressData = %d, want JPEGScanCompleted", result)
	}
}

func TestDecompressSmoothDataBasic(t *testing.T) {
	info := helperDecompInfo()
	info.TotalIMCURows = 1
	info.OutputIMCURow = 0
	info.CompInfo[0].QuantTable = &QuantTable{}
	for i := 0; i < 64; i++ {
		info.CompInfo[0].QuantTable.QuantVal[i] = 1
	}

	cc := NewCoefController(info, true)
	cc.CoefBitsLatch = make([]int, 6)
	cc.MCURowsPerIMCU = 1

	outputBuf := make([][][]byte, 1)
	outputBuf[0] = make([][]byte, 8)
	for r := range outputBuf[0] {
		outputBuf[0][r] = make([]byte, 8)
	}

	result := cc.decompressSmoothData(info, outputBuf)
	if result != JPEGScanCompleted {
		t.Errorf("decompressSmoothData = %d, want JPEGScanCompleted", result)
	}
}

// ---- MainController tests ----

func TestNewMainController(t *testing.T) {
	info := helperDecompInfo()
	mc := NewMainController(info, false)
	if mc == nil {
		t.Fatal("NewMainController returned nil")
	}
	if mc.Buffer == nil {
		t.Error("Buffer should be allocated")
	}
}

func TestNewMainControllerFullBuffer(t *testing.T) {
	info := helperDecompInfo()
	mc := NewMainController(info, true)
	if mc != nil {
		t.Error("NewMainController with full buffer should return nil")
	}
}

func TestNewMainControllerWithContext(t *testing.T) {
	info := helperDecompInfo()
	mc := NewMainControllerWithContext(info)
	if mc == nil {
		t.Fatal("NewMainControllerWithContext returned nil")
	}
	if mc.XBuffer[0] == nil || mc.XBuffer[1] == nil {
		t.Error("XBuffer should be allocated")
	}
}

func TestNewMainControllerWithContextSmallM(t *testing.T) {
	info := helperDecompInfo()
	info.MinDCTVScalSize = 1
	mc := NewMainControllerWithContext(info)
	if mc != nil {
		t.Error("NewMainControllerWithContext should return nil for M < 2")
	}
}

func TestMainControllerStartPassSimple(t *testing.T) {
	info := helperDecompInfo()
	mc := NewMainController(info, false)
	mc.StartPass(JBufPassThru, false)
	// RowGroupCtr should be set to RowGroupsAvail (mark empty)
	if mc.RowGroupCtr != mc.RowGroupsAvail {
		t.Errorf("RowGroupCtr = %d, want %d", mc.RowGroupCtr, mc.RowGroupsAvail)
	}
}

func TestMainControllerStartPassContext(t *testing.T) {
	info := helperDecompInfo()
	mc := NewMainControllerWithContext(info)
	mc.StartPass(JBufPassThru, true)
	if mc.ContextState != CTXPrepareForIMCU {
		t.Errorf("ContextState = %d, want CTXPrepareForIMCU", mc.ContextState)
	}
}

func TestMainControllerStartPassCrankDest(t *testing.T) {
	info := helperDecompInfo()
	mc := NewMainController(info, false)
	mc.StartPass(JBufCrankDest, false)
	// Should not panic
}

func TestMainControllerProcessDataSimple(t *testing.T) {
	info := helperDecompInfo()
	info.MCUsPerRow = 1
	info.TotalIMCURows = 1
	info.CompsInScan = 1
	info.CurCompInfo = []*ComponentInfo{&info.CompInfo[0]}

	mc := NewMainController(info, false)
	mc.RowGroupCtr = 0
	mc.RowGroupsAvail = 1

	outputBuf := [][]byte{make([]byte, 8)}
	outRowCtr := 0

	mc.ProcessDataSimple(info, nil, nil, outputBuf, &outRowCtr, 1,
		func(info *DecompressInfo, inputBuf [][][]byte, inRowGroupCtr *int, inRowGroupsAvail int,
			outputBuf [][]byte, outRowCtr *int, outRowsAvail int) {
			*outRowCtr++
			*inRowGroupCtr++
		})

	if outRowCtr != 1 {
		t.Errorf("outRowCtr = %d, want 1", outRowCtr)
	}
}

func TestMainControllerProcessDataSimpleWithPostProcessor(t *testing.T) {
	info := helperDecompInfo()
	mc := NewMainController(info, false)
	mc.RowGroupCtr = 0
	mc.RowGroupsAvail = 1

	outputBuf := [][]byte{make([]byte, 8)}
	outRowCtr := 0

	called := false
	pp := &PostProcessor{}
	pp.postProcessData = func(info *DecompressInfo,
		inputBuf [][][]byte, inRowGroupCtr *int, inRowGroupsAvail int,
		outputBuf [][]byte, outRowCtr *int, outRowsAvail int) {
		called = true
	}

	mc.ProcessDataSimple(info, nil, pp, outputBuf, &outRowCtr, 1, nil)
	if !called {
		t.Error("postprocessor should have been called")
	}
}

func TestMainControllerProcessDataContext(t *testing.T) {
	info := helperDecompInfo()
	info.TotalIMCURows = 1

	mc := NewMainControllerWithContext(info)
	mc.ContextState = CTXPrepareForIMCU
	mc.BufferFull = false
	mc.IMCURowCtr = 0

	outputBuf := [][]byte{make([]byte, 8)}
	outRowCtr := 0

	cc := NewCoefController(info, true)
	cc.MCURowsPerIMCU = 1
	info.MCUsPerRow = 1
	info.CompsInScan = 1
	info.CurCompInfo = []*ComponentInfo{&info.CompInfo[0]}

	mc.ProcessDataContext(info, cc, nil, outputBuf, &outRowCtr, 1,
		func(info *DecompressInfo, inputBuf [][][]byte, inRowGroupCtr *int, inRowGroupsAvail int,
			outputBuf [][]byte, outRowCtr *int, outRowsAvail int) {
		})
	// Should not panic
}

func TestMainControllerProcessDataCrankPost(t *testing.T) {
	info := helperDecompInfo()
	mc := NewMainController(info, false)

	outputBuf := [][]byte{make([]byte, 8)}
	outRowCtr := 0

	called := false
	pp := &PostProcessor{}
	pp.postProcessData = func(info *DecompressInfo,
		inputBuf [][][]byte, inRowGroupCtr *int, inRowGroupsAvail int,
		outputBuf [][]byte, outRowCtr *int, outRowsAvail int) {
		called = true
	}

	mc.ProcessDataCrankPost(info, pp, outputBuf, &outRowCtr, 1)
	if !called {
		t.Error("postprocessor should have been called")
	}
}

func TestMainControllerProcessDataCrankPostNil(t *testing.T) {
	info := helperDecompInfo()
	mc := NewMainController(info, false)

	outputBuf := [][]byte{make([]byte, 8)}
	outRowCtr := 0

	mc.ProcessDataCrankPost(info, nil, outputBuf, &outRowCtr, 1)
	// Should not panic
}

func TestMakeFunnyPointers(t *testing.T) {
	info := helperDecompInfo()
	mc := NewMainControllerWithContext(info)
	mc.makeFunnyPointers()
	// Should not panic
}

func TestSetWraparoundPointers(t *testing.T) {
	info := helperDecompInfo()
	mc := NewMainControllerWithContext(info)
	mc.makeFunnyPointers()
	mc.setWraparoundPointers()
	// Should not panic
}

func TestSetBottomPointers(t *testing.T) {
	info := helperDecompInfo()
	mc := NewMainControllerWithContext(info)
	mc.makeFunnyPointers()
	mc.setBottomPointers()
	// Should not panic
}

// ---- PostProcessor tests ----

func TestNewPostProcessor(t *testing.T) {
	info := helperDecompInfo()
	pp := NewPostProcessor(info, false, nil)
	if pp == nil {
		t.Fatal("NewPostProcessor returned nil")
	}
}

func TestNewPostProcessorWithQuantize(t *testing.T) {
	info := helperDecompInfo()
	info.QuantizeColors = true
	pp := NewPostProcessor(info, false, nil)
	if pp == nil {
		t.Fatal("NewPostProcessor returned nil")
	}
	if pp.Buffer == nil {
		t.Error("Buffer should be allocated for quantize mode")
	}
}

func TestNewPostProcessorFullBuffer(t *testing.T) {
	info := helperDecompInfo()
	info.QuantizeColors = true
	pp := NewPostProcessor(info, true, nil)
	if pp == nil {
		t.Fatal("NewPostProcessor returned nil")
	}
	if !pp.NeedFullBuffer {
		t.Error("NeedFullBuffer should be true")
	}
	if pp.WholeImage == nil {
		t.Error("WholeImage should be allocated")
	}
}

func TestPostProcessorStartPass(t *testing.T) {
	info := helperDecompInfo()
	pp := NewPostProcessor(info, false, nil)
	pp.StartPass(JBufPassThru)
	if pp.postProcessData == nil {
		t.Error("postProcessData should be set after StartPass")
	}
}

func TestPostProcessorPostProcessData(t *testing.T) {
	info := helperDecompInfo()
	called := false
	pp := NewPostProcessor(info, false, nil)
	pp.StartPass(JBufPassThru)

	// Override with test function
	pp.postProcessData = func(info *DecompressInfo,
		inputBuf [][][]byte, inRowGroupCtr *int, inRowGroupsAvail int,
		outputBuf [][]byte, outRowCtr *int, outRowsAvail int) {
		called = true
	}

	inputBuf := [][][]byte{{{1, 2, 3}}}
	inRowGroupCtr := 0
	outputBuf := [][]byte{make([]byte, 8)}
	outRowCtr := 0

	pp.PostProcessData(info, inputBuf, &inRowGroupCtr, 1, outputBuf, &outRowCtr, 1)
	if !called {
		t.Error("postProcessData should have been called")
	}
}

func TestPostProcessorPassThruProcess(t *testing.T) {
	info := helperDecompInfo()
	called := false
	pp := NewPostProcessor(info, false, func(info *DecompressInfo,
		inputBuf [][][]byte, inRowGroupCtr *int, inRowGroupsAvail int,
		outputBuf [][]byte, outRowCtr *int, outRowsAvail int) {
		called = true
	})

	inputBuf := [][][]byte{{{1, 2, 3}}}
	inRowGroupCtr := 0
	outputBuf := [][]byte{make([]byte, 8)}
	outRowCtr := 0

	pp.passThruProcess(info, inputBuf, &inRowGroupCtr, 1, outputBuf, &outRowCtr, 1)
	if !called {
		t.Error("upsampler should have been called")
	}
}

func TestPostProcessorPassThruNil(t *testing.T) {
	info := helperDecompInfo()
	pp := NewPostProcessor(info, false, nil)

	inputBuf := [][][]byte{{{1, 2, 3}}}
	inRowGroupCtr := 0
	outputBuf := [][]byte{make([]byte, 8)}
	outRowCtr := 0

	pp.passThruProcess(info, inputBuf, &inRowGroupCtr, 1, outputBuf, &outRowCtr, 1)
	// Should not panic
}

func TestPostProcessorPostProcess1Pass(t *testing.T) {
	info := helperDecompInfo()
	info.QuantizeColors = true

	upsamplerCalled := false
	pp := NewPostProcessor(info, false, func(info *DecompressInfo,
		inputBuf [][][]byte, inRowGroupCtr *int, inRowGroupsAvail int,
		outputBuf [][]byte, outRowCtr *int, outRowsAvail int) {
		upsamplerCalled = true
		*outRowCtr = 1
		*inRowGroupCtr++
	})

	inputBuf := [][][]byte{{{1, 2, 3}}}
	inRowGroupCtr := 0
	outputBuf := [][]byte{make([]byte, 8)}
	outRowCtr := 0

	pp.postProcess1Pass(info, inputBuf, &inRowGroupCtr, 1, outputBuf, &outRowCtr, 1, nil)
	if !upsamplerCalled {
		t.Error("upsampler should have been called")
	}
}

func TestPostProcessorPostProcess1PassWithQuantize(t *testing.T) {
	info := helperDecompInfo()
	info.QuantizeColors = true

	quantizeCalled := false
	pp := NewPostProcessor(info, false, func(info *DecompressInfo,
		inputBuf [][][]byte, inRowGroupCtr *int, inRowGroupsAvail int,
		outputBuf [][]byte, outRowCtr *int, outRowsAvail int) {
		*outRowCtr = 1
		*inRowGroupCtr++
	})

	inputBuf := [][][]byte{{{1, 2, 3}}}
	inRowGroupCtr := 0
	outputBuf := [][]byte{make([]byte, 24)}
	outRowCtr := 0

	pp.postProcess1Pass(info, inputBuf, &inRowGroupCtr, 1, outputBuf, &outRowCtr, 1,
		func(input [][]byte, output [][]byte, numRows int) {
			quantizeCalled = true
		})

	if !quantizeCalled {
		t.Error("colorQuantize should have been called")
	}
}

// ---- MergedUpsampler tests ----

func TestNewMergedUpsamplerH2V1(t *testing.T) {
	info := &DecompressInfo{
		OutputWidth:     8,
		OutputHeight:    4,
		MaxVSampFactor:  1,
		OutColorComponents: 3,
	}
	mu := NewMergedUpsampler(info, false)
	if mu == nil {
		t.Fatal("NewMergedUpsampler returned nil for h2v1")
	}
	if mu.v2 {
		t.Error("should not be v2 for MaxVSampFactor=1")
	}
}

func TestNewMergedUpsamplerH2V2(t *testing.T) {
	info := &DecompressInfo{
		OutputWidth:     8,
		OutputHeight:    4,
		MaxVSampFactor:  2,
		OutColorComponents: 3,
	}
	mu := NewMergedUpsampler(info, false)
	if mu == nil {
		t.Fatal("NewMergedUpsampler returned nil for h2v2")
	}
	if !mu.v2 {
		t.Error("should be v2 for MaxVSampFactor=2")
	}
}

func TestNewMergedUpsamplerBGYCC(t *testing.T) {
	info := &DecompressInfo{
		OutputWidth:     8,
		OutputHeight:    4,
		MaxVSampFactor:  1,
		OutColorComponents: 3,
	}
	mu := NewMergedUpsampler(info, true)
	if mu == nil {
		t.Fatal("NewMergedUpsampler returned nil for BGYCC")
	}
	if mu.CrRTab == nil {
		t.Error("CrRTab should be allocated")
	}
}

func TestMergedUpsamplerStartPass(t *testing.T) {
	info := &DecompressInfo{
		OutputWidth:     8,
		OutputHeight:    4,
		MaxVSampFactor:  1,
		OutColorComponents: 3,
	}
	mu := NewMergedUpsampler(info, false)
	mu.StartPass()
	if mu.SpareFull {
		t.Error("SpareFull should be false after StartPass")
	}
	if mu.RowsToGo != info.OutputHeight {
		t.Errorf("RowsToGo = %d, want %d", mu.RowsToGo, info.OutputHeight)
	}
}

func TestMergedUpsamplerH2V1Upsample(t *testing.T) {
	info := &DecompressInfo{
		OutputWidth:     8,
		OutputHeight:    2,
		MaxVSampFactor:  1,
		OutColorComponents: 3,
	}
	mu := NewMergedUpsampler(info, false)
	mu.StartPass()

	// h2v1: Y is at full resolution (8 cols), Cb/Cr at half (4 cols)
	inputBuf := [][][]byte{
		{{128, 128, 128, 128, 128, 128, 128, 128}, {128, 128, 128, 128, 128, 128, 128, 128}}, // Y: 2 rows x 8 cols
		{{128, 128, 128, 128}, {128, 128, 128, 128}}, // Cb: 2 rows x 4 cols
		{{128, 128, 128, 128}, {128, 128, 128, 128}}, // Cr: 2 rows x 4 cols
	}
	inRowGroupCtr := 0
	outputBuf := [][]byte{
		make([]byte, 24), // 8 pixels * 3 bytes
		make([]byte, 24),
	}
	outRowCtr := 0

	mu.Upsample(info, inputBuf, &inRowGroupCtr, outputBuf, &outRowCtr, 2)
	if outRowCtr != 1 {
		t.Errorf("outRowCtr = %d, want 1 (one row group per Upsample call for h2v1)", outRowCtr)
	}
}

func TestMergedUpsamplerH2V2Upsample(t *testing.T) {
	info := &DecompressInfo{
		OutputWidth:     8,
		OutputHeight:    4,
		MaxVSampFactor:  2,
		OutColorComponents: 3,
	}
	mu := NewMergedUpsampler(info, false)
	mu.StartPass()

	// h2v2: Y at full resolution (8 cols, 2 rows per row group), Cb/Cr at half (4 cols, 1 row)
	inputBuf := [][][]byte{
		{
			{128, 128, 128, 128, 128, 128, 128, 128}, // Y row 0
			{128, 128, 128, 128, 128, 128, 128, 128}, // Y row 1
			{128, 128, 128, 128, 128, 128, 128, 128}, // Y row 2
			{128, 128, 128, 128, 128, 128, 128, 128}, // Y row 3
		},
		{{128, 128, 128, 128}, {128, 128, 128, 128}}, // Cb: 2 rows x 4 cols
		{{128, 128, 128, 128}, {128, 128, 128, 128}}, // Cr: 2 rows x 4 cols
	}
	inRowGroupCtr := 0
	outputBuf := [][]byte{
		make([]byte, 24),
		make([]byte, 24),
	}
	outRowCtr := 0

	mu.Upsample(info, inputBuf, &inRowGroupCtr, outputBuf, &outRowCtr, 2)
	if outRowCtr != 2 {
		t.Errorf("outRowCtr = %d, want 2", outRowCtr)
	}
}

func TestH2V1MergedUpsample(t *testing.T) {
	info := &DecompressInfo{
		OutputWidth:     8,
		OutputHeight:    1,
		MaxVSampFactor:  1,
		OutColorComponents: 3,
	}
	mu := NewMergedUpsampler(info, false)

	inputBuf := [][][]byte{
		{{128, 128, 128, 128, 128, 128, 128, 128}}, // Y: 8 values
		{{128, 128, 128, 128}},                     // Cb: 4 values (one per pair)
		{{128, 128, 128, 128}},                     // Cr: 4 values (one per pair)
	}
	outputBuf := [][]byte{make([]byte, 24)} // 8 pixels * 3 bytes

	mu.h2v1MergedUpsample(info, inputBuf, 0, outputBuf)
	// Should not panic
}

func TestH2V2MergedUpsample(t *testing.T) {
	info := &DecompressInfo{
		OutputWidth:     8,
		OutputHeight:    2,
		MaxVSampFactor:  2,
		OutColorComponents: 3,
	}
	mu := NewMergedUpsampler(info, false)

	// h2v2: Y at full resolution (8 cols, 2 rows), Cb/Cr at half (4 cols, 1 row)
	inputBuf := [][][]byte{
		{{128, 128, 128, 128, 128, 128, 128, 128}, {128, 128, 128, 128, 128, 128, 128, 128}}, // Y: 2 rows x 8 cols
		{{128, 128, 128, 128}}, // Cb: 1 row x 4 cols
		{{128, 128, 128, 128}}, // Cr: 1 row x 4 cols
	}
	outputBuf := [][]byte{
		make([]byte, 24),
		make([]byte, 24),
	}

	mu.h2v2MergedUpsample(info, inputBuf, 0, outputBuf)
	// Should not panic and produce valid values
	for row := 0; row < 2; row++ {
		for col := 0; col < 24; col++ {
			if outputBuf[row][col] > 255 {
				t.Errorf("output[%d][%d] = %d, out of range", row, col, outputBuf[row][col])
			}
		}
	}
}

func TestH2V1MergedOddWidth(t *testing.T) {
	info := &DecompressInfo{
		OutputWidth:     5, // odd width
		OutputHeight:    1,
		MaxVSampFactor:  1,
		OutColorComponents: 3,
	}
	mu := NewMergedUpsampler(info, false)

	inputBuf := [][][]byte{
		{{128, 128, 128, 128, 128}}, // Y: 5 values
		{{128, 128, 128}},           // Cb: 3 values (2 pairs + 1)
		{{128, 128, 128}},           // Cr: 3 values
	}
	outputBuf := [][]byte{make([]byte, 15)} // 5 * 3

	mu.h2v1MergedUpsample(info, inputBuf, 0, outputBuf)
	// Should not panic
}

func TestH2V2MergedOddWidth(t *testing.T) {
	info := &DecompressInfo{
		OutputWidth:     5, // odd width
		OutputHeight:    2,
		MaxVSampFactor:  2,
		OutColorComponents: 3,
	}
	mu := NewMergedUpsampler(info, false)

	inputBuf := [][][]byte{
		{{128, 128, 128, 128, 128}, {128, 128, 128, 128, 128}}, // Y: 2 rows x 5 cols
		{{128, 128, 128}},                                       // Cb: 3 values
		{{128, 128, 128}},                                       // Cr: 3 values
	}
	outputBuf := [][]byte{
		make([]byte, 15),
		make([]byte, 15),
	}

	mu.h2v2MergedUpsample(info, inputBuf, 0, outputBuf)
	// Should not panic
}
