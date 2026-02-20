package pocsag

import (
	"bytes"
	"encoding/binary"
	"math"
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

// ConvertToAudioWithBaudRate converts POCSAG bytes to WAV audio with specified baud rate.
// Uses baseband (DC levels): bit 1 = negative, bit 0 = positive. Compatible with pocsag-decode.
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

// FSK tone frequencies for multimon-ng compatibility (mark=1, space=0)
const (
	FSKFreqSpace = 1200.0 // Hz, bit 0
	FSKFreqMark  = 2200.0 // Hz, bit 1 (multimon-ng POCSAG 2200/1200)
)

// ConvertToAudioFSK converts POCSAG bytes to FSK WAV audio (sine waves).
// Compatible with multimon-ng: bit 1 = 2200 Hz, bit 0 = 1200 Hz.
// Use this when you need output decodable by multimon-ng.
func ConvertToAudioFSK(pocsagData []byte, baudRate int) []byte {
	samplesPerSymbol := SampleRate / baudRate
	numBits := len(pocsagData) * 8
	numSamples := numBits * samplesPerSymbol
	audioData := make([]int16, numSamples)

	const amplitude = 16000.0 // leave headroom for 16-bit
	phase := 0.0
	sampleIdx := 0

	for _, b := range pocsagData {
		for i := 7; i >= 0; i-- {
			bit := (b >> i) & 1
			freq := FSKFreqSpace
			if bit == 1 {
				freq = FSKFreqMark
			}
			phaseIncrement := 2.0 * math.Pi * freq / float64(SampleRate)

			for j := 0; j < samplesPerSymbol; j++ {
				phase += phaseIncrement
				for phase > 2.0*math.Pi {
					phase -= 2.0 * math.Pi
				}
				audioData[sampleIdx] = int16(amplitude * math.Sin(phase))
				sampleIdx++
			}
		}
	}

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

// GenerateFSKSamples generates IQ samples from POCSAG bytes for SDR-style waterfall
// Returns interleaved I/Q samples: [I0, Q0, I1, Q1, ...]
func GenerateFSKSamples(pocsagData []byte, baudRate int) []int16 {
	samplesPerBit := SampleRate / baudRate
	numBits := len(pocsagData) * 8
	numSamples := numBits * samplesPerBit

	// Interleaved IQ: 2 values per sample
	samples := make([]int16, numSamples*2)

	// RF simulation parameters
	const carrierFreq = 12000.0   // Center carrier at 12 kHz
	const deviation = 4500.0      // FSK deviation (±4.5 kHz for POCSAG)
	const signalAmplitude = 15000 // Strong signal
	const noiseAmplitude = 2000   // RF noise floor (MUST be visible as blue background)

	carrierPhase := 0.0
	noisePhase1 := 0.0
	noisePhase2 := 0.0

	// FIRST PASS: Generate RF noise floor for ALL samples (creates blue background everywhere)
	for i := 0; i < numSamples; i++ {
		// Wideband RF noise on both I and Q channels
		noisePhase1 += 0.123456
		noisePhase2 += 0.789012

		noiseI := noiseAmplitude * math.Sin(noisePhase1*123.456)
		noiseQ := noiseAmplitude * math.Cos(noisePhase2*456.789)

		samples[i*2] = int16(noiseI)   // I noise
		samples[i*2+1] = int16(noiseQ) // Q noise
	}

	// SECOND PASS: Add FSK signal ON TOP of noise floor (creates bright signal)
	for byteIdx, b := range pocsagData {
		// Process each bit (MSB first)
		for bitPos := 7; bitPos >= 0; bitPos-- {
			bit := (b >> bitPos) & 1

			// Frequency deviation based on bit value
			var freqOffset float64
			if bit == 1 {
				freqOffset = deviation // +4.5 kHz
			} else {
				freqOffset = -deviation // -4.5 kHz
			}

			// Generate IQ samples for this bit and ADD to existing noise
			for j := 0; j < samplesPerBit; j++ {
				sampleIdx := ((byteIdx*8+(7-bitPos))*samplesPerBit + j) * 2

				// Instantaneous frequency
				instantFreq := carrierFreq + freqOffset
				carrierPhase += 2.0 * math.Pi * instantFreq / float64(SampleRate)

				// Keep phase wrapped
				for carrierPhase > 2.0*math.Pi {
					carrierPhase -= 2.0 * math.Pi
				}

				// Generate FSK signal I and Q components
				signalI := signalAmplitude * math.Cos(carrierPhase)
				signalQ := signalAmplitude * math.Sin(carrierPhase)

				// ADD signal to existing noise (bright signal on blue background)
				samples[sampleIdx] += int16(signalI)   // Add to I
				samples[sampleIdx+1] += int16(signalQ) // Add to Q
			}
		}
	}

	return samples
}
