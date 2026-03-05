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

// CalculateEvenParity calculates even parity bit - exact port from pocsag.c
func CalculateEvenParity(x uint32) uint32 {
	count := 0
	for i := 1; i < 32; i++ { // Count all bits from 1 to 31
		if x&(1<<i) != 0 {
			count++
		}
	}
	parity := uint32(count % 2)
	return (x &^ uint32(1)) | parity // Clear bit 0 and set new parity
}

// DoesWordPassBCH checks if a codeword matches its BCH(31,21) parity and even parity
func DoesWordPassBCH(cw uint32) bool {
	// 1. Check BCH bits (1-31)
	// CalculateBCH returns (data | parity_bits) where bit 0 is 0.
	dataBits := cw & AddressMask
	expected := CalculateBCH(dataBits)

	// Compare bits 1-31 (mask out bit 0)
	if (expected & ^uint32(1)) != (cw & ^uint32(1)) {
		return false
	}

	// 2. Check overall even parity (bit 0)
	// CalculateEvenParity returns the word with the correct bit 0 for even parity
	expectedParity := CalculateEvenParity(cw)
	return expectedParity == cw
}
