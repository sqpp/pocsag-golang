package pocsag

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"math"
)

// DecodedMessage represents a decoded POCSAG message
type DecodedMessage struct {
	Address   uint32
	Function  uint8
	Message   string
	IsNumeric bool
}

// DecodeFromAudio decodes POCSAG from WAV audio data
// Uses default 1200 baud for backward compatibility
func DecodeFromAudio(wavData []byte) ([]DecodedMessage, error) {
	return DecodeFromAudioWithBaudRate(wavData, BaudRate1200)
}

// DecodeFromAudioWithDecryption decodes POCSAG from WAV audio data with decryption
func DecodeFromAudioWithDecryption(wavData []byte, baudRate int, encryption EncryptionConfig) ([]DecodedMessage, error) {
	// First decode the audio normally
	messages, err := DecodeFromAudioWithBaudRate(wavData, baudRate)
	if err != nil {
		return nil, err
	}

	// Decrypt messages if encryption is configured
	if encryption.Method != EncryptionNone {
		for i := range messages {
			decryptedMessage, err := DecryptMessage(messages[i].Message, encryption)
			if err != nil {
				// If decryption fails, keep the original message (might not be encrypted)
				continue
			}
			messages[i].Message = decryptedMessage
		}
	}

	return messages, nil
}

// DecodeFromAudioWithBaudRate decodes POCSAG from WAV audio data with specified baud rate
func DecodeFromAudioWithBaudRate(wavData []byte, baudRate int) ([]DecodedMessage, error) {

	// Find data chunk
	// Standard WAV has "data" chunk followed by 4-byte size, then actual samples
	dataOffset := bytes.Index(wavData, []byte("data"))
	startIdx := 44
	if dataOffset != -1 {
		startIdx = dataOffset + 8 // "data" (4) + size (4)
	}

	// Read sample rate from WAV header (bytes 24-27)
	var sampleRate uint32 = 48000 // default
	if len(wavData) > 28 {
		sampleRate = binary.LittleEndian.Uint32(wavData[24:28])
	}

	// Convert audio samples to slice
	samples := make([]float32, 0)
	for i := startIdx; i < len(wavData)-1; i += 2 {
		sample := float32(int16(binary.LittleEndian.Uint16(wavData[i:])))
		samples = append(samples, sample)
	}

	// Demodulate: calculate samples per bit based on baud rate
	samplesPerBit := float64(sampleRate) / float64(baudRate)

	// Strategy 1: Dynamic DC tracking (for recording with significant DC drift)
	// Window size should be baud-dependent to avoid smearing high-baud signals
	// 8 bits is a good compromise for DC tracking.
	lpfWindow := int(samplesPerBit * 8)
	if lpfWindow == 0 {
		lpfWindow = 1
	}

	basebandDynamic := make([]float32, len(samples))
	var sum float32

	for i := 0; i < len(samples); i++ {
		sum += samples[i]
		if i >= lpfWindow {
			sum -= samples[i-lpfWindow]
			dc := sum / float32(lpfWindow)
			basebandDynamic[i-lpfWindow/2] = samples[i-lpfWindow/2] - dc
		}
	}

	// Strategy 2: Global Average DC tracking
	var globalSum float64
	for i := 0; i < len(samples); i++ {
		globalSum += float64(samples[i])
	}
	avgDc := float32(globalSum / float64(len(samples)))
	basebandGlobal := make([]float32, len(samples))
	for i := 0; i < len(samples); i++ {
		basebandGlobal[i] = samples[i] - avgDc
	}

	var bestMessages []DecodedMessage

	// We test different basebands based on recording quality
	// 0: Raw samples (perfect for synthetic)
	// 1: Global Average DC (best for most cases)
	// 2: Dynamic LPF Baseband (for heavy DC drift)
	for strat := 0; strat < 3; strat++ {
		var activeBaseband []float32
		if strat == 0 {
			activeBaseband = samples
		} else if strat == 1 {
			activeBaseband = basebandGlobal
		} else {
			activeBaseband = basebandDynamic
		}

		// Test both polarities
		for polarity := 0; polarity < 2; polarity++ {
			// Higher number of phases for better initial alignment
			phases := 40

			for phase := 0; phase < phases; phase++ {
				bits := make([]byte, 0)
				offset := (float64(phase) * samplesPerBit) / float64(phases)

				currentIndex := offset

				// DPLL Tracking parameters
				// nudgeFactor := 0.01 // Very low nudge for stability

				for currentIndex+samplesPerBit <= float64(len(activeBaseband)) {
					// Integration window
					var bitSum float32 = 0
					window := 0.7
					winOffset := samplesPerBit * (1.0 - window) / 2.0
					startS := currentIndex + winOffset
					endS := startS + samplesPerBit*window

					iStart := int(math.Round(startS))
					iEnd := int(math.Round(endS))

					for j := iStart; j < iEnd && j < len(activeBaseband); j++ {
						bitSum += activeBaseband[j]
					}

					bitVal := byte(0)
					if (polarity == 0 && bitSum > 0) || (polarity == 1 && bitSum < 0) {
						bitVal = 1
					}
					bits = append(bits, bitVal)

					// DPLL: Only use for strategy 1 and 2 (DC tracked signals)
					if strat > 0 {
						searchLen := samplesPerBit * 0.4
						searchStart := currentIndex + samplesPerBit - searchLen/2

						iSearchStart := int(math.Round(searchStart))
						iSearchEnd := int(math.Round(searchStart + searchLen))

						for j := iSearchStart; j < iSearchEnd && j < len(activeBaseband)-1; j++ {
							s1 := activeBaseband[j]
							s2 := activeBaseband[j+1]
							if (s1 > 0 && s2 <= 0) || (s1 <= 0 && s2 > 0) {
								t := -s1 / (s2 - s1)
								actualBoundary := float64(j) + float64(t)
								expectedBoundary := currentIndex + samplesPerBit
								errorOffset := actualBoundary - expectedBoundary

								// Highly conservative nudge
								currentIndex += errorOffset * 0.005
								break
							}
						}
					}

					currentIndex += samplesPerBit
				}

				messages, err := DecodeFromBitstream(bits)
				if err == nil && len(messages) > len(bestMessages) {
					bestMessages = messages

					// Strategy 0 is raw/perfect. If it finds anything, it's almost certainly the correct one.
					if strat == 0 && len(bestMessages) > 0 {
						return bestMessages, nil
					}
				}
			}
		}
	}

	return bestMessages, nil
}

// DecodeFromBitstream decodes POCSAG from a stream of 0/1 bits
func DecodeFromBitstream(bits []byte) ([]DecodedMessage, error) {
	messages := make([]DecodedMessage, 0)

	// Scan for sync word bit-by-bit
	var shiftReg uint32
	syncIdx := -1
	for i := 0; i < len(bits); i++ {
		shiftReg = (shiftReg << 1) | uint32(bits[i])
		if shiftReg == FrameSyncWord {
			syncIdx = i + 1
			break
		}
	}

	if syncIdx == -1 {
		return nil, fmt.Errorf("sync word not found")
	}

	// Helper to read 32 bits from current position
	readWord := func(pos int) (uint32, bool) {
		if pos+32 > len(bits) {
			return 0, false
		}
		var w uint32
		for i := 0; i < 32; i++ {
			w = (w << 1) | uint32(bits[pos+i])
		}
		return w, true
	}

	idx := syncIdx
	var currentAddress uint32
	var currentFunction uint8
	messageCodewords := make([]uint32, 0)
	batchPos := 0

	for {
		cw, ok := readWord(idx)
		if !ok {
			break
		}

		// Every codeword must pass BCH/Parity check, EXCEPT for Sync/Idle constants
		if cw != FrameSyncWord && cw != IdleCodeword {
			if !DoesWordPassBCH(cw) {
				// Log the failure for debugging
				// fmt.Printf("[BitDecode] Parity error at batch bit offset %d: 0x%08X\n", idx, cw)
				break
			}
		}

		idx += 32

		if cw == FrameSyncWord {
			batchPos = 0
			continue
		}

		if cw == IdleCodeword {
			batchPos++
			continue
		}

		isAddress := (cw & (1 << 31)) == 0
		if isAddress {
			if len(messageCodewords) > 0 && currentAddress != 0 {
				msg := decodeMessage(messageCodewords, currentFunction)
				messages = append(messages, DecodedMessage{Address: currentAddress, Function: currentFunction, Message: msg, IsNumeric: currentFunction == FuncNumeric})
			}
			messageCodewords = make([]uint32, 0)

			data := (cw >> 11) & 0x1FFFFF
			currentFunction = uint8(data & 0x3)
			baseAddress := ((data >> 2) & 0x7FFFF)
			frameIndex := uint32(batchPos / 2)
			currentAddress = (baseAddress << 3) | frameIndex
			currentAddress &= 0x1FFFFF
		} else {
			if currentAddress != 0 {
				messageCodewords = append(messageCodewords, cw)
			}
		}
		batchPos++
	}

	if len(messageCodewords) > 0 && currentAddress != 0 {
		msg := decodeMessage(messageCodewords, currentFunction)
		messages = append(messages, DecodedMessage{Address: currentAddress, Function: currentFunction, Message: msg, IsNumeric: currentFunction == FuncNumeric})
	}

	return messages, nil
}

// DecodeFromBinary decodes POCSAG from raw binary data
func DecodeFromBinary(data []byte) ([]DecodedMessage, error) {
	messages := make([]DecodedMessage, 0)

	// Find first frame sync word
	syncIdx := -1
	for i := 0; i < len(data)-3; i++ {
		word := binary.BigEndian.Uint32(data[i:])
		if word == FrameSyncWord {
			syncIdx = i
			break
		}
	}

	if syncIdx == -1 {
		return nil, fmt.Errorf("frame sync word not found")
	}

	// Start reading codewords after sync
	idx := syncIdx + 4

	var currentAddress uint32
	var currentFunction uint8
	messageCodewords := make([]uint32, 0)

	// Keep track of our position within the 16-codeword batch
	// Each batch has 8 frames, each frame has 2 codewords
	batchPos := 0

	for idx+3 < len(data) {
		cw := binary.BigEndian.Uint32(data[idx:])
		idx += 4

		// Check if it's a sync word (start of new batch)
		if cw == FrameSyncWord {
			batchPos = 0 // Reset batch position
			// Continue to next batch without breaking message collection
			continue
		}

		if cw == IdleCodeword {
			// Skip idle codewords - they're just padding between or within messages
			// Don't finalize the message here, as it may continue in the next batch
			batchPos++
			continue
		}

		// Check if it's an address codeword (bit 31 = 0)
		isAddress := (cw & (1 << 31)) == 0

		if isAddress {
			// If we have a pending message, process it first
			if len(messageCodewords) > 0 && currentAddress != 0 {
				msg := decodeMessage(messageCodewords, currentFunction)
				messages = append(messages, DecodedMessage{Address: currentAddress, Function: currentFunction, Message: msg, IsNumeric: currentFunction == FuncNumeric})
			}
			messageCodewords = make([]uint32, 0) // Reset for new address

			// Decode the new address
			// Bits 30-13 contain the 18 most significant bits of the 21-bit address
			// The 3 least significant bits are determined by the frame in the batch (0-7)
			data := (cw >> 11) & 0x1FFFFF
			currentFunction = uint8(data & 0x3)

			// Extract the 18-bit base address from the codeword
			baseAddress := ((data >> 2) & 0x7FFFF)

			// Calculate the frame index (0-7) from the batch position (0-15)
			frameIndex := uint32(batchPos / 2)

			// Combine them: 18 bits shifted left by 3, OR'ed with the 3-bit frame index
			currentAddress = (baseAddress << 3) | frameIndex

			// Note: Standard POCSAG (e.g. PDW) often ignores the 22nd bit (MSB)
			// we mask keep only 21 bits
			currentAddress &= 0x1FFFFF

		} else { // Is Message
			if currentAddress != 0 { // Only collect message parts if we have an address
				messageCodewords = append(messageCodewords, cw)
			}
		}

		batchPos++
	}

	// Process any leftover message at the end
	if len(messageCodewords) > 0 && currentAddress != 0 {
		msg := decodeMessage(messageCodewords, currentFunction)
		messages = append(messages, DecodedMessage{Address: currentAddress, Function: currentFunction, Message: msg, IsNumeric: currentFunction == FuncNumeric})
	}

	return messages, nil
}

// decodeMessage decodes message from codewords
func decodeMessage(codewords []uint32, function uint8) string {
	var bits []byte
	for _, cw := range codewords {
		// Extract the 20-bit data portion (bits 11-30)
		data := (cw >> 11) & 0xFFFFF

		// Convert to bits (MSB first)
		for i := 19; i >= 0; i-- {
			bits = append(bits, byte((data>>i)&1))
		}
	}

	if function == FuncNumeric {
		return decodeNumericFromBits(bits)
	}
	return decodeAlphaFromBits(bits)
}

// decodeNumericFromBits decodes BCD numeric message from bitstream
func decodeNumericFromBits(bits []byte) string {
	result := make([]rune, 0)
	for i := 0; i+3 < len(bits); i += 4 {
		nibble := byte(0)
		for j := 0; j < 4; j++ {
			nibble = (nibble << 1) | bits[i+j]
		}
		nibble = BitReverse4(nibble)

		// Stop at terminator (0xA = unused nibble)
		if nibble == 0xA {
			break
		}

		char := bcdToChar(nibble)
		if char != 0 {
			result = append(result, char)
		}
	}
	msg := string(result)
	// Trim trailing spaces
	for len(msg) > 0 && msg[len(msg)-1] == ' ' {
		msg = msg[:len(msg)-1]
	}
	return msg
}

// bcdToChar converts BCD nibble to character
func bcdToChar(nibble byte) rune {
	switch nibble {
	case 0, 1, 2, 3, 4, 5, 6, 7, 8, 9:
		return rune('0' + nibble)
	case 0xB:
		return 'U'
	case 0xC:
		return ' '
	case 0xD:
		return '-'
	case 0xE:
		return ']'
	case 0xF:
		return '['
	default:
		return 0
	}
}

// decodeAlphaFromBits decodes a 7-bit ASCII bitstream
func decodeAlphaFromBits(bits []byte) string {
	result := make([]byte, 0)
	// Process all available 7-bit groups
	for i := 0; i <= len(bits)-7; i += 7 {
		charBits := byte(0)
		for j := 0; j < 7; j++ {
			charBits = (charBits << 1) | bits[i+j]
		}
		char := BitReverse8(charBits << 1)

		// Stop at ETX terminator (if present)
		if char == 0x03 {
			break
		}
		// Stop at null bytes (padding) - this is the key fix
		if char == 0x00 {
			break
		}
		// Only include printable ASCII characters
		if char >= 0x20 && char <= 0x7E {
			result = append(result, char)
		} else {
			// Stop at any non-printable character (except ETX which we handle above)
			break
		}
	}
	return string(result)
}

// FormatMessage formats a decoded message for display
func (m *DecodedMessage) String() string {
	msgType := "ALPHA"
	if m.IsNumeric {
		msgType = "NUMERIC"
	}
	return fmt.Sprintf("Address: %7d  Function: %d  %-7s  Message: %s",
		m.Address, m.Function, msgType, m.Message)
}

// DecodeReader reads and decodes POCSAG from an io.Reader (WAV file)
func DecodeReader(r io.Reader) ([]DecodedMessage, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	return DecodeFromAudio(data)
}

// DecodeFromLiveStreamWithDecryption decodes POCSAG from continuous audio stream
// This function scans the ENTIRE audio buffer for ALL POCSAG transmissions
// Perfect for real-time radio decoding where signals can appear anywhere in the stream
func DecodeFromLiveStreamWithDecryption(wavData []byte, baudRate int, encryption EncryptionConfig) ([]DecodedMessage, error) {
	fmt.Printf("[LiveDecode] Starting decode: WAV size=%d bytes, baudRate=%d\n", len(wavData), baudRate)

	// Find data chunk
	dataOffset := bytes.Index(wavData, []byte("data"))
	startIdx := 44
	if dataOffset != -1 {
		startIdx = dataOffset + 8 // "data" + 4 bytes length
	}

	// Convert audio samples to bits
	samples := make([]int16, 0)
	for i := startIdx; i < len(wavData)-1; i += 2 {
		sample := int16(binary.LittleEndian.Uint16(wavData[i:]))
		samples = append(samples, sample)
	}

	fmt.Printf("[LiveDecode] Extracted %d audio samples\n", len(samples))

	// Read sample rate from WAV header (bytes 24-27)
	var sampleRate uint32 = 48000 // default
	if len(wavData) > 28 {
		sampleRate = binary.LittleEndian.Uint32(wavData[24:28])
	}

	// Demodulate: calculate samples per bit based on baud rate
	samplesPerBit := float64(sampleRate) / float64(baudRate)
	bits := make([]byte, 0)

	currentIndex := 0.0
	for currentIndex+samplesPerBit <= float64(len(samples)) {
		// Use center sampling or average
		window := 0.5
		start := currentIndex + samplesPerBit*(1.0-window)/2.0
		end := start + samplesPerBit*window

		var sum float64
		count := 0
		for j := int(math.Round(start)); j < int(math.Round(end)) && j < len(samples); j++ {
			sum += float64(samples[j])
			count++
		}

		bitVal := byte(0)
		if count > 0 {
			avg := sum / float64(count)
			if avg < 0 {
				bitVal = 1
			}
		}
		bits = append(bits, bitVal)
		currentIndex += samplesPerBit
	}

	fmt.Printf("[LiveDecode] Demodulated to %d bits (samplesPerBit=%.2f)\n", len(bits), samplesPerBit)

	// Convert bits to bytes
	pocsagData := make([]byte, 0)
	for i := 0; i < len(bits)-7; i += 8 {
		b := byte(0)
		for j := 0; j < 8; j++ {
			b = (b << 1) | bits[i+j]
		}
		pocsagData = append(pocsagData, b)
	}

	fmt.Printf("[LiveDecode] Converted to %d bytes of POCSAG data\n", len(pocsagData))

	// Show first 64 bytes in hex for debugging
	if len(pocsagData) > 64 {
		fmt.Printf("[LiveDecode] First 64 bytes (hex): %x\n", pocsagData[:64])
	}

	// Scan for ALL frame sync words in the entire buffer
	return DecodeFromBinaryLiveStream(pocsagData, encryption)
}

// DecodeFromBinaryLiveStream scans the ENTIRE binary buffer for ALL POCSAG transmissions
// Unlike DecodeFromBinary which stops at the first sync word, this finds ALL signals
func DecodeFromBinaryLiveStream(data []byte, encryption EncryptionConfig) ([]DecodedMessage, error) {
	allMessages := make([]DecodedMessage, 0)
	syncWordsFound := 0

	fmt.Printf("[LiveDecode] Scanning %d bytes for POCSAG sync words...\n", len(data))

	// Scan through the entire buffer looking for ALL frame sync words
	for searchStart := 0; searchStart < len(data)-3; searchStart++ {
		// Find next frame sync word
		syncIdx := -1
		for i := searchStart; i < len(data)-3; i++ {
			word := binary.BigEndian.Uint32(data[i:])
			if word == FrameSyncWord {
				syncIdx = i
				break
			}
		}

		// No more sync words found
		if syncIdx == -1 {
			break
		}

		syncWordsFound++
		fmt.Printf("[LiveDecode] Found sync word #%d at byte offset %d\n", syncWordsFound, syncIdx)

		// Decode this transmission starting from the sync word
		messages := decodeSingleTransmission(data, syncIdx)
		fmt.Printf("[LiveDecode] Decoded %d messages from this transmission\n", len(messages))

		// Decrypt messages if encryption is configured
		if encryption.Method != EncryptionNone && len(encryption.Key) > 0 {
			for i := range messages {
				decryptedMessage, err := DecryptMessage(messages[i].Message, encryption)
				if err != nil {
					// If decryption fails, keep the original message
					fmt.Printf("[LiveDecode] Decrypt error: %v\n", err)
					continue
				}
				messages[i].Message = decryptedMessage
			}
		}

		allMessages = append(allMessages, messages...)

		// Move search position forward past this sync word to find the next one
		searchStart = syncIdx + 4
	}

	fmt.Printf("[LiveDecode] Total: found %d sync words, decoded %d messages\n", syncWordsFound, len(allMessages))

	// If no messages found at all, return error
	if len(allMessages) == 0 {
		return nil, fmt.Errorf("frame sync word not found")
	}

	return allMessages, nil
}

// decodeSingleTransmission decodes one POCSAG transmission starting from a sync word
func decodeSingleTransmission(data []byte, syncIdx int) []DecodedMessage {
	messages := make([]DecodedMessage, 0)

	// Start reading codewords after sync
	idx := syncIdx + 4

	var currentAddress uint32
	var currentFunction uint8
	messageCodewords := make([]uint32, 0)

	// Limit to reasonable transmission length (e.g., 10000 codewords)
	maxCodewords := 10000
	codewordCount := 0
	batchPos := 0

	for idx+3 < len(data) && codewordCount < maxCodewords {
		cw := binary.BigEndian.Uint32(data[idx:])
		idx += 4
		codewordCount++

		// A POCSAG batch consists of 16 codewords.
		// After 16 codewords, there MUST be a Frame Sync Word to maintain sync.
		if batchPos == 16 {
			if cw == FrameSyncWord {
				batchPos = 0
				continue
			} else {
				// Synchronization lost. End of this transmission.
				break
			}
		}

		// Check if it's an unexpected sync word mid-batch
		if cw == FrameSyncWord {
			// This happens if we erroneously thought we were in sync, or if there's corruption.
			// Let's reset the batch position and assume it's the start of a new batch.
			batchPos = 0
			continue
		}

		batchPos++

		if cw == IdleCodeword {
			// Skip idle codewords - they're just padding
			continue
		}

		// Check if it's an address codeword (bit 31 = 0)
		isAddress := (cw & (1 << 31)) == 0

		if isAddress {
			// If we have a pending message, process it first
			if len(messageCodewords) > 0 && currentAddress != 0 {
				msg := decodeMessage(messageCodewords, currentFunction)
				messages = append(messages, DecodedMessage{
					Address:   currentAddress,
					Function:  currentFunction,
					Message:   msg,
					IsNumeric: currentFunction == FuncNumeric,
				})
			}
			messageCodewords = make([]uint32, 0) // Reset for new address

			// Decode the new address
			addrData := (cw >> 11) & 0x1FFFFF
			currentFunction = uint8(addrData & 0x3)

			// Get the base address and calculate frame from batchPos
			// Note: batchPos here went from 1 to 16, so frame index is (batchPos-1)/2
			frameIndex := uint32((batchPos - 1) / 2)
			baseAddress := ((addrData >> 2) & 0x7FFFF)
			currentAddress = (baseAddress << 3) | frameIndex

			// Keep only 21 bits
			currentAddress &= 0x1FFFFF
		} else { // Is Message
			if currentAddress != 0 { // Only collect message parts if we have an address
				messageCodewords = append(messageCodewords, cw)
			}
		}
	}

	// Process any leftover message at the end
	if len(messageCodewords) > 0 && currentAddress != 0 {
		msg := decodeMessage(messageCodewords, currentFunction)
		messages = append(messages, DecodedMessage{
			Address:   currentAddress,
			Function:  currentFunction,
			Message:   msg,
			IsNumeric: currentFunction == FuncNumeric,
		})
	}

	return messages
}
