package pocsag

// BCH(31,21) error correction code - EXACT port from pocsag-tool

const (
	AddressMask   = 0xFFFFF800 // bits 11-31 are data
	GeneratorPoly = 0x769      // BCH generator polynomial
	NumDataBits   = 21
	NumTotalBits  = 31
)

// CalculateBCH calculates BCH(31,21) parity - exact port from pocsag.c lines 53-80
func CalculateBCH(x uint32) uint32 {
	x &= AddressMask // keep only data bits (11-31)
	dividend := x
	generator := uint32(GeneratorPoly << NumDataBits)
	mask := uint32(1 << NumTotalBits)

	for i := 0; i < NumDataBits; i++ {
		if dividend&mask != 0 {
			dividend ^= generator
		}
		generator >>= 1
		mask >>= 1
	}

	return x | dividend // combine data with BCH parity
}

// CalculateEvenParity calculates even parity bit - exact port from pocsag.c lines 83-101
func CalculateEvenParity(x uint32) uint32 {
	count := 0
	for i := 1; i < NumTotalBits; i++ { // NUM_BITS_INT = 31 in C (not 32!)
		if x&(1<<i) != 0 {
			count++
		}
	}
	return x | uint32(count%2)
}
