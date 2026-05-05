package huff

// Probability estimation table for arithmetic coding.
// Ported from jaricom.c (jpeg_aritab).
//
// The packing formula from jaricom.c:
//
//	#define V(i,a,b,c,d) (((INT32)a << 16) | ((INT32)c << 8) | ((INT32)d << 7) | b)
//
// Where:
//
//	a = Qe_Value (probability estimate)
//	b = Next_Index_LPS
//	c = Next_Index_MPS
//	d = Switch_MPS (0 or 1)
//
// Extraction in the decoder (from jdarith.c):
//
//	qe = jpeg_aritab[sv & 0x7F]    // full 32-bit value
//	nl = qe & 0xFF; qe >>= 8       // nl = (d << 7) | b, then shift
//	nm = qe & 0xFF; qe >>= 8       // nm = c, then shift
//	remaining qe = Qe_Value         // a
//
// So the 32-bit layout is: [Qe_Value(16) | Next_Index_MPS(8) | (Switch_MPS(1) | Next_Index_LPS(7))]
//
// Index 113 is the fixed probability estimate of 0.5.

// vPack computes the V() macro from jaricom.c.
func vPack(a, b, c, d int32) int32 {
	return (a << 16) | (c << 8) | (d << 7) | b
}

var arithTab = [114]int32{
	vPack(0x5a1d, 1, 1, 1),    // 0
	vPack(0x2586, 14, 2, 0),   // 1
	vPack(0x1114, 16, 3, 0),   // 2
	vPack(0x080b, 18, 4, 0),   // 3
	vPack(0x03d8, 20, 5, 0),   // 4
	vPack(0x01da, 23, 6, 0),   // 5
	vPack(0x00e5, 25, 7, 0),   // 6
	vPack(0x006f, 28, 8, 0),   // 7
	vPack(0x0036, 30, 9, 0),   // 8
	vPack(0x001a, 33, 10, 0),  // 9
	vPack(0x000d, 35, 11, 0),  // 10
	vPack(0x0006, 9, 12, 0),   // 11
	vPack(0x0003, 10, 13, 0),  // 12
	vPack(0x0001, 12, 13, 0),  // 13
	vPack(0x5a7f, 15, 15, 1),  // 14
	vPack(0x3f25, 36, 16, 0),  // 15
	vPack(0x2cf2, 38, 17, 0),  // 16
	vPack(0x207c, 39, 18, 0),  // 17
	vPack(0x17b9, 40, 19, 0),  // 18
	vPack(0x1182, 42, 20, 0),  // 19
	vPack(0x0cef, 43, 21, 0),  // 20
	vPack(0x09a1, 45, 22, 0),  // 21
	vPack(0x072f, 46, 23, 0),  // 22
	vPack(0x055c, 48, 24, 0),  // 23
	vPack(0x0406, 49, 25, 0),  // 24
	vPack(0x0303, 51, 26, 0),  // 25
	vPack(0x0240, 52, 27, 0),  // 26
	vPack(0x01b1, 54, 28, 0),  // 27
	vPack(0x0144, 56, 29, 0),  // 28
	vPack(0x00f5, 57, 30, 0),  // 29
	vPack(0x00b7, 59, 31, 0),  // 30
	vPack(0x008a, 60, 32, 0),  // 31
	vPack(0x0068, 62, 33, 0),  // 32
	vPack(0x004e, 63, 34, 0),  // 33
	vPack(0x003b, 32, 35, 0),  // 34
	vPack(0x002c, 33, 9, 0),   // 35
	vPack(0x5ae1, 37, 37, 1),  // 36
	vPack(0x484c, 64, 38, 0),  // 37
	vPack(0x3a0d, 65, 39, 0),  // 38
	vPack(0x2ef1, 67, 40, 0),  // 39
	vPack(0x261f, 68, 41, 0),  // 40
	vPack(0x1f33, 69, 42, 0),  // 41
	vPack(0x19a8, 70, 43, 0),  // 42
	vPack(0x1518, 72, 44, 0),  // 43
	vPack(0x1177, 73, 45, 0),  // 44
	vPack(0x0e74, 74, 46, 0),  // 45
	vPack(0x0bfb, 75, 47, 0),  // 46
	vPack(0x09f8, 77, 48, 0),  // 47
	vPack(0x0861, 78, 49, 0),  // 48
	vPack(0x0706, 79, 50, 0),  // 49
	vPack(0x05cd, 48, 51, 0),  // 50
	vPack(0x04de, 50, 52, 0),  // 51
	vPack(0x040f, 50, 53, 0),  // 52
	vPack(0x0363, 51, 54, 0),  // 53
	vPack(0x02d4, 52, 55, 0),  // 54
	vPack(0x025c, 53, 56, 0),  // 55
	vPack(0x01f8, 54, 57, 0),  // 56
	vPack(0x01a4, 55, 58, 0),  // 57
	vPack(0x0160, 56, 59, 0),  // 58
	vPack(0x0125, 57, 60, 0),  // 59
	vPack(0x00f6, 58, 61, 0),  // 60
	vPack(0x00cb, 59, 62, 0),  // 61
	vPack(0x00ab, 61, 63, 0),  // 62
	vPack(0x008f, 61, 32, 0),  // 63
	vPack(0x5b12, 65, 65, 1),  // 64
	vPack(0x4d04, 80, 66, 0),  // 65
	vPack(0x412c, 81, 67, 0),  // 66
	vPack(0x37d8, 82, 68, 0),  // 67
	vPack(0x2fe8, 83, 69, 0),  // 68
	vPack(0x293c, 84, 70, 0),  // 69
	vPack(0x2379, 86, 71, 0),  // 70
	vPack(0x1edf, 87, 72, 0),  // 71
	vPack(0x1aa9, 87, 73, 0),  // 72
	vPack(0x174e, 72, 74, 0),  // 73
	vPack(0x1424, 72, 75, 0),  // 74
	vPack(0x119c, 74, 76, 0),  // 75
	vPack(0x0f6b, 74, 77, 0),  // 76
	vPack(0x0d51, 75, 78, 0),  // 77
	vPack(0x0bb6, 77, 79, 0),  // 78
	vPack(0x0a40, 77, 48, 0),  // 79
	vPack(0x5832, 80, 81, 1),  // 80
	vPack(0x4d1c, 88, 82, 0),  // 81
	vPack(0x438e, 89, 83, 0),  // 82
	vPack(0x3bdd, 90, 84, 0),  // 83
	vPack(0x34ee, 91, 85, 0),  // 84
	vPack(0x2eae, 92, 86, 0),  // 85
	vPack(0x299a, 93, 87, 0),  // 86
	vPack(0x2516, 86, 71, 0),  // 87
	vPack(0x5570, 88, 89, 1),  // 88
	vPack(0x4ca9, 95, 90, 0),  // 89
	vPack(0x44d9, 96, 91, 0),  // 90
	vPack(0x3e22, 97, 92, 0),  // 91
	vPack(0x3824, 99, 93, 0),  // 92
	vPack(0x32b4, 99, 94, 0),  // 93
	vPack(0x2e17, 93, 86, 0),  // 94
	vPack(0x56a8, 95, 96, 1),  // 95
	vPack(0x4f46, 101, 97, 0), // 96
	vPack(0x47e5, 102, 98, 0), // 97
	vPack(0x41cf, 103, 99, 0), // 98
	vPack(0x3c3d, 104, 100, 0), // 99
	vPack(0x375e, 99, 93, 0),  // 100
	vPack(0x5231, 105, 102, 0), // 101
	vPack(0x4c0f, 106, 103, 0), // 102
	vPack(0x4639, 107, 104, 0), // 103
	vPack(0x415e, 103, 99, 0), // 104
	vPack(0x5627, 105, 106, 1), // 105
	vPack(0x50e7, 108, 107, 0), // 106
	vPack(0x4b85, 109, 103, 0), // 107
	vPack(0x5597, 110, 109, 0), // 108
	vPack(0x504f, 111, 107, 0), // 109
	vPack(0x5a10, 110, 111, 1), // 110
	vPack(0x5522, 112, 109, 0), // 111
	vPack(0x59eb, 112, 111, 1), // 112
	vPack(0x5a1d, 113, 113, 0), // 113 - fixed probability estimate of 0.5
}
