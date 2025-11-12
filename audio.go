package pocsag

import (
	"bytes"
	"encoding/binary"
)

const (
	// Audio constants - from bin2audio.c
	BaudRate1200  = 1200
	BaudRate512   = 512
	BaudRate2400  = 2400
	SampleRate    = 48000
	BitsPerSample = 16
	NumChannels   = 1
)

// BaudRate is the default baud rate for backward compatibility
const BaudRate = BaudRate1200

var (
	SymbolHigh = int16(-12287) // bit 1 (0xD001 as signed)
	SymbolLow  = int16(12287)  // bit 0 (0x2FFF as signed)
)

// ConvertToAudio converts POCSAG bytes to WAV audio - exact port from bin2audio.c
// Uses default 1200 baud for backward compatibility
func ConvertToAudio(pocsagData []byte) []byte {
	return ConvertToAudioWithBaudRate(pocsagData, BaudRate1200)
}

// ConvertToAudioWithBaudRate converts POCSAG bytes to WAV audio with specified baud rate
func ConvertToAudioWithBaudRate(pocsagData []byte, baudRate int) []byte {
	samplesPerSymbol := SampleRate / baudRate

	// Calculate total samples
	numBits := len(pocsagData) * 8
	numSamples := numBits * samplesPerSymbol

	// Audio data
	audioData := make([]int16, numSamples)
	sampleIdx := 0

	// Process each byte
	for _, b := range pocsagData {
		// Process each bit (MSB first)
		for i := 7; i >= 0; i-- {
			bit := (b >> i) & 1
			var sample int16

			if bit == 1 {
				sample = int16(SymbolHigh) // negative value
			} else {
				sample = int16(SymbolLow) // positive value
			}

			// Repeat sample for baud rate
			for j := 0; j < samplesPerSymbol; j++ {
				audioData[sampleIdx] = sample
				sampleIdx++
			}
		}
	}

	// Create WAV file
	return createWAVFile(audioData)
}

func createWAVFile(samples []int16) []byte {
	var buf bytes.Buffer

	dataSize := uint32(len(samples) * 2)
	fileSize := 36 + dataSize
	byteRate := uint32(SampleRate * NumChannels * BitsPerSample / 8)
	blockAlign := uint16(NumChannels * BitsPerSample / 8) // Correct block align for Firefox compatibility

	// RIFF header
	buf.WriteString("RIFF")
	binary.Write(&buf, binary.LittleEndian, fileSize)
	buf.WriteString("WAVE")

	// fmt chunk
	buf.WriteString("fmt ")
	binary.Write(&buf, binary.LittleEndian, uint32(16))            // chunk size
	binary.Write(&buf, binary.LittleEndian, uint16(1))             // PCM format
	binary.Write(&buf, binary.LittleEndian, uint16(NumChannels))   // channels
	binary.Write(&buf, binary.LittleEndian, uint32(SampleRate))    // sample rate
	binary.Write(&buf, binary.LittleEndian, byteRate)              // byte rate
	binary.Write(&buf, binary.LittleEndian, blockAlign)            // block align
	binary.Write(&buf, binary.LittleEndian, uint16(BitsPerSample)) // bits per sample

	// data chunk
	buf.WriteString("data")
	binary.Write(&buf, binary.LittleEndian, dataSize) // Write actual data size for Firefox compatibility

	// Write samples
	for _, sample := range samples {
		binary.Write(&buf, binary.LittleEndian, sample)
	}

	return buf.Bytes()
}
