package pocsag

import (
	"bytes"
)

const (
	// POCSAG constants
	PreambleLength = 576 // bits
	FrameSyncWord  = 0x7CD215D8
	IdleCodeword   = 0x7A89C197

	// Function codes
	FuncNumeric      = 0
	FuncTone1        = 1
	FuncTone2        = 2
	FuncAlphanumeric = 3
)

// BitReverse8 reverses bits in a byte - exact port from pocsag.c
func BitReverse8(b byte) byte {
	b = (b&0xF0)>>4 | (b&0x0F)<<4
	b = (b&0xCC)>>2 | (b&0x33)<<2
	b = (b&0xAA)>>1 | (b&0x55)<<1
	return b
}

// EncodeAddress creates an address codeword - exact port from pocsag.c lines 104-120
func EncodeAddress(address uint32, function uint8) uint32 {
	addr := address
	addr >>= 3                   // divide by 8
	addr &= 0x0007FFFF           // mask to 19 bits
	addr <<= 2                   // shift left by 2
	addr |= uint32(function & 3) // add function bits
	addr <<= 11                  // shift to bits 11-31

	cw := CalculateBCH(addr)
	cw = CalculateEvenParity(cw)
	return cw
}

// BitReverse4 reverses bits in a 4-bit nibble
func BitReverse4(n byte) byte {
	n = (n&0xC)>>2 | (n&0x3)<<2
	n = (n&0xA)>>1 | (n&0x5)<<1
	return n & 0xF
}

// NumericBCDEncoder encodes numeric string to BCD format for POCSAG
// Packs 5 digits per 20-bit message payload (4 bits per digit)
func NumericBCDEncoder(message string) []byte {
	// Convert characters to BCD nibbles
	nibbles := make([]byte, 0, len(message))
	for i := 0; i < len(message); i++ {
		ch := message[i]
		var nibble byte
		switch {
		case ch >= '0' && ch <= '9':
			nibble = ch - '0'
		case ch == 'U' || ch == 'u':
			nibble = 0xB // Urgency
		case ch == ' ':
			nibble = 0xC // Space
		case ch == '-':
			nibble = 0xD // Hyphen
		case ch == ']':
			nibble = 0xE // Right bracket
		case ch == '[':
			nibble = 0xF // Left bracket
		default:
			nibble = 0xC // Default to space for invalid chars
		}
		// Reverse bits in nibble (POCSAG numeric uses bit-reversed BCD)
		nibbles = append(nibbles, BitReverse4(nibble))
	}

	// Add terminator (0xA = unused nibble) to mark end of message
	nibbles = append(nibbles, BitReverse4(0xA))

	// Pack nibbles into bytes (2 nibbles per byte, MSB first)
	numBytes := (len(nibbles) + 1) / 2
	encoded := make([]byte, numBytes)

	for i := 0; i < len(nibbles); i++ {
		byteIdx := i / 2
		if i%2 == 0 {
			// First nibble goes in high 4 bits
			encoded[byteIdx] = nibbles[i] << 4
		} else {
			// Second nibble goes in low 4 bits
			encoded[byteIdx] |= nibbles[i]
		}
	}

	// If odd number of nibbles, pad last byte with 0x3 (bit-reversed 0xC space)
	if len(nibbles)%2 == 1 {
		encoded[numBytes-1] |= 0x03
	}

	return encoded
}

// Ascii7BitEncoder encodes ASCII string to 7-bit - exact port from pocsag.c lines 122-162
func Ascii7BitEncoder(message string) []byte {
	length := len(message)
	encoded := make([]byte, 0, length)

	shift := 1
	currIdx := 0
	curr := byte(0)

	for i := 0; i < length; i++ {
		tmp := uint16(BitReverse8(message[i]))
		tmp &= 0x00FE
		tmp >>= 1
		tmp <<= shift

		curr |= byte(tmp & 0x00FF)
		if currIdx > 0 {
			encoded[currIdx-1] |= byte((tmp & 0xFF00) >> 8)
		}

		shift++
		if shift == 8 {
			shift = 0
		} else {
			if i < length-1 {
				encoded = append(encoded, curr)
				currIdx++
				curr = 0
			}
		}
	}

	if curr > 0 {
		encoded = append(encoded, curr)
	}

	return encoded
}

// SplitMessageIntoFrames splits encoded message into codewords - exact port from pocsag.c lines 165-216
func SplitMessageIntoFrames(encoded7bit []byte) []uint32 {
	chunks := (len(encoded7bit) / 3) + 1
	batches := make([]uint32, chunks)

	curr := 0
	end := len(encoded7bit)

	for i := 0; i < chunks; i++ {
		batch := uint32(0)
		remaining := end - curr

		// Copy 3 bytes or remaining bytes (big-endian)
		if remaining >= 3 {
			batch = (uint32(encoded7bit[curr]) << 24) |
				(uint32(encoded7bit[curr+1]) << 16) |
				(uint32(encoded7bit[curr+2]) << 8)
		} else if remaining > 0 {
			for j := 0; j < remaining; j++ {
				batch |= uint32(encoded7bit[curr+j]) << (24 - j*8)
			}
		}

		// Advance pointer and apply mask/shift
		if i%2 == 0 {
			// Even chunk
			if remaining >= 3 {
				curr += 2
			}
			batch &= 0xFFFFF000
			batch >>= 1
		} else {
			// Odd chunk
			if remaining >= 3 {
				curr += 3
			}
			batch &= 0x0FFFFF00
			batch <<= 3
		}

		batch |= (1 << 31) // set message bit
		batch = CalculateBCH(batch)
		batch = CalculateEvenParity(batch)
		batches[i] = batch
	}

	return batches
}

// CreatePOCSAGPacket creates a complete POCSAG packet
func CreatePOCSAGPacket(address uint32, message string, function uint8) []byte {
	// Generate preamble (alternating 1010...)
	preamble := make([]byte, PreambleLength/8)
	for i := range preamble {
		preamble[i] = 0xAA
	}

	// Create codewords
	codewords := make([]uint32, 0, 16)

	// Add address codeword
	addressCW := EncodeAddress(address, function)
	codewords = append(codewords, addressCW)

	// Add message codewords - use appropriate encoder based on function
	var encodedMessage []byte
	if function == FuncNumeric {
		// Numeric messages use BCD encoding
		encodedMessage = NumericBCDEncoder(message)
	} else {
		// Alphanumeric and other functions use 7-bit ASCII
		encodedMessage = Ascii7BitEncoder(message + "\x03") // ETX terminator
	}

	messageCWs := SplitMessageIntoFrames(encodedMessage)
	codewords = append(codewords, messageCWs...)

	// Pad to 16 codewords (1 batch)
	for len(codewords)%16 != 0 {
		codewords = append(codewords, IdleCodeword)
	}

	// Convert to bytes
	var buf bytes.Buffer
	buf.Write(preamble)

	// Frame sync
	writeUint32BE(&buf, FrameSyncWord)

	// Codewords
	for _, cw := range codewords {
		writeUint32BE(&buf, cw)
	}

	return buf.Bytes()
}

func writeUint32BE(buf *bytes.Buffer, val uint32) {
	buf.WriteByte(byte(val >> 24))
	buf.WriteByte(byte(val >> 16))
	buf.WriteByte(byte(val >> 8))
	buf.WriteByte(byte(val))
}
