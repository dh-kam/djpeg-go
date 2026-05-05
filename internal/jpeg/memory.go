package jpeg

// Memory management ported from IJG libjpeg 9f (jmemmgr.c, jmemsys.h, jmemansi.c).
//
// The C library used a complex pool-based memory allocator with backing store
// support for virtual arrays. In Go, we use Go slices and the garbage collector.
// Pool-based allocation is replaced with simple slice allocation.
// Virtual arrays keep their control structure but use Go slices directly
// (no backing store needed since Go handles memory management).

import "fmt"

// ---------------------------------------------------------------------------
// Pool allocation (simplified for Go)
// ---------------------------------------------------------------------------

// Pool represents a memory pool. In Go, we track allocations but let the GC
// handle deallocation.
type Pool struct {
	objects []interface{}
}

// MemoryPools manages the two pools: permanent and image.
type MemoryPools struct {
	permanent Pool
	image     Pool
}

// NewMemoryPools creates a new set of memory pools.
func NewMemoryPools() *MemoryPools {
	return &MemoryPools{}
}

// ---------------------------------------------------------------------------
// Allocation functions (replacing the jmemmgr.c pool allocator)
// ---------------------------------------------------------------------------

// AllocSmall allocates a "small" object. In Go, this just creates a new
// slice of the appropriate type. The poolID is ignored since Go's GC
// handles memory.
func AllocSmall(poolID int, size int) []byte {
	return make([]byte, size)
}

// AllocSArray allocates a 2-D sample array (JSAMPARRAY).
// Returns a slice of numrows slices, each of length samplesPerRow.
func AllocSArray(samplesPerRow, numrows int) JSAMPARRAY {
	result := make(JSAMPARRAY, numrows)
	for i := range result {
		result[i] = make(JSAMPROW, samplesPerRow)
	}
	return result
}

// AllocBArray allocates a 2-D coefficient-block array (JBLOCKARRAY).
// Returns a slice of numrows slices, each of length blocksPerRow.
func AllocBArray(blocksPerRow, numrows int) JBLOCKARRAY {
	result := make(JBLOCKARRAY, numrows)
	for i := range result {
		result[i] = make(JBLOCKROW, blocksPerRow)
	}
	return result
}

// ---------------------------------------------------------------------------
// Virtual array management
// ---------------------------------------------------------------------------

// RequestVirtSArray requests a virtual 2-D sample array.
// In Go, we just create it immediately since we have GC-managed memory.
func RequestVirtSArray(samplesPerRow, numrows, maxAccess int) *VirtSArray {
	return &VirtSArray{
		MemBuffer:     nil, // Not yet realized
		RowsInArray:   numrows,
		SamplesPerRow: samplesPerRow,
		MaxAccess:     maxAccess,
		PreZero:       true,
	}
}

// RequestVirtBArray requests a virtual 2-D coefficient-block array.
func RequestVirtBArray(blocksPerRow, numrows, maxAccess int) *VirtBArray {
	return &VirtBArray{
		MemBuffer:    nil, // Not yet realized
		RowsInArray:  numrows,
		BlocksPerRow: blocksPerRow,
		MaxAccess:    maxAccess,
		PreZero:      true,
	}
}

// RealizeVirtSArray allocates the in-memory buffer for a virtual sample array.
// In Go, we always allocate the full array (no backing store needed).
func RealizeVirtSArray(vsa *VirtSArray) {
	if vsa.MemBuffer != nil {
		return // Already realized
	}
	rowsInMem := vsa.RowsInArray // Full height in memory (no backing store)
	vsa.RowsInMem = rowsInMem
	vsa.RowsPerChunk = rowsInMem
	vsa.CurStartRow = 0
	vsa.FirstUndefRow = 0
	vsa.Dirty = false
	vsa.MemBuffer = AllocSArray(vsa.SamplesPerRow, rowsInMem)
}

// RealizeVirtBArray allocates the in-memory buffer for a virtual block array.
func RealizeVirtBArray(vba *VirtBArray) {
	if vba.MemBuffer != nil {
		return
	}
	rowsInMem := vba.RowsInArray
	vba.RowsInMem = rowsInMem
	vba.RowsPerChunk = rowsInMem
	vba.CurStartRow = 0
	vba.FirstUndefRow = 0
	vba.Dirty = false
	vba.MemBuffer = AllocBArray(vba.BlocksPerRow, rowsInMem)
}

// AccessVirtSArray accesses a portion of a virtual sample array starting at
// startRow and extending for numRows rows. writable indicates if the caller
// intends to modify the data.
func AccessVirtSArray(ptr *VirtSArray, startRow, numRows int, writable bool) JSAMPARRAY {
	if ptr.MemBuffer == nil {
		RealizeVirtSArray(ptr)
	}
	endRow := startRow + numRows
	if endRow > ptr.RowsInArray || numRows > ptr.MaxAccess {
		panic(fmt.Sprintf("AccessVirtSArray: bad access start=%d num=%d rows=%d maxaccess=%d",
			startRow, numRows, ptr.RowsInArray, ptr.MaxAccess))
	}

	// Pre-zero undefined rows if requested
	if ptr.PreZero && ptr.FirstUndefRow < endRow {
		undefRow := ptr.FirstUndefRow
		if undefRow < startRow {
			undefRow = startRow
		}
		for r := undefRow; r < endRow; r++ {
			if r-ptr.CurStartRow < len(ptr.MemBuffer) {
				for j := range ptr.MemBuffer[r-ptr.CurStartRow] {
					ptr.MemBuffer[r-ptr.CurStartRow][j] = 0
				}
			}
		}
		if writable {
			ptr.FirstUndefRow = endRow
		}
	}

	if writable {
		ptr.Dirty = true
	}

	// Return the appropriate portion of the buffer.
	// Since we always hold the full array in memory, curStartRow is always 0.
	offset := startRow - ptr.CurStartRow
	if offset < 0 || offset+numRows > len(ptr.MemBuffer) {
		panic("AccessVirtSArray: buffer index out of range")
	}
	return ptr.MemBuffer[offset : offset+numRows]
}

// AccessVirtBArray accesses a portion of a virtual block array.
func AccessVirtBArray(ptr *VirtBArray, startRow, numRows int, writable bool) JBLOCKARRAY {
	if ptr.MemBuffer == nil {
		RealizeVirtBArray(ptr)
	}
	endRow := startRow + numRows
	if endRow > ptr.RowsInArray || numRows > ptr.MaxAccess {
		panic(fmt.Sprintf("AccessVirtBArray: bad access start=%d num=%d rows=%d maxaccess=%d",
			startRow, numRows, ptr.RowsInArray, ptr.MaxAccess))
	}

	// Pre-zero undefined rows if requested
	if ptr.PreZero && ptr.FirstUndefRow < endRow {
		undefRow := ptr.FirstUndefRow
		if undefRow < startRow {
			undefRow = startRow
		}
		for r := undefRow; r < endRow; r++ {
			idx := r - ptr.CurStartRow
			if idx >= 0 && idx < len(ptr.MemBuffer) {
				for j := range ptr.MemBuffer[idx] {
					for k := range ptr.MemBuffer[idx][j] {
						ptr.MemBuffer[idx][j][k] = 0
					}
				}
			}
		}
		if writable {
			ptr.FirstUndefRow = endRow
		}
	}

	if writable {
		ptr.Dirty = true
	}

	offset := startRow - ptr.CurStartRow
	if offset < 0 || offset+numRows > len(ptr.MemBuffer) {
		panic("AccessVirtBArray: buffer index out of range")
	}
	return ptr.MemBuffer[offset : offset+numRows]
}

// ---------------------------------------------------------------------------
// Memory manager initialization
// ---------------------------------------------------------------------------

// InitMemoryMgr initializes the memory manager for a JPEGCommon struct.
// In Go, this is greatly simplified compared to the C version.
func InitMemoryMgr(cinfo *JPEGCommon) {
	cinfo.Mem = &MemoryManager{
		MaxMemoryToUse: 1000000, // 1MB default (same as C DEFAULT_MAX_MEM)
		MaxAllocChunk:  1000000000,
	}
}

// FreePool releases all objects belonging to a specified pool.
// In Go, this is a no-op since the GC handles deallocation.
func FreePool(poolID int) {
	// No-op: Go's garbage collector handles memory.
}

// SelfDestruct releases all memory pools.
// In Go, this is a no-op; memory will be collected by the GC when
// the JPEGCommon struct is no longer referenced.
func SelfDestruct(cinfo *JPEGCommon) {
	cinfo.Mem = nil
}
