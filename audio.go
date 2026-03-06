package pocsag

import (
	"bytes"
	"encoding/binary"
	"math"
	"math/rand"
	"time"
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
	samplesPerSymbol := float64(SampleRate) / float64(baudRate)
	numBits := len(pocsagData) * 8
	numSamples := int(float64(numBits) * samplesPerSymbol)

	audioData := make([]int16, numSamples)

	for byteIdx, b := range pocsagData {
		for bitPos := 7; bitPos >= 0; bitPos-- {
			bit := (b >> bitPos) & 1
			var sample int16

			if bit == 1 {
				sample = int16(SymbolHigh)
			} else {
				sample = int16(SymbolLow)
			}

			bitIndex := byteIdx*8 + (7 - bitPos)
			startIdx := int(math.Round(float64(bitIndex) * samplesPerSymbol))
			endIdx := int(math.Round(float64(bitIndex+1) * samplesPerSymbol))

			for j := startIdx; j < endIdx; j++ {
				audioData[j] = sample
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
	samplesPerSymbol := float64(SampleRate) / float64(baudRate)
	numBits := len(pocsagData) * 8
	numSamples := int(float64(numBits) * samplesPerSymbol)
	audioData := make([]int16, numSamples)

	const amplitude = 16000.0 // leave headroom for 16-bit
	phase := 0.0

	for byteIdx, b := range pocsagData {
		for bitPos := 7; bitPos >= 0; bitPos-- {
			bit := (b >> bitPos) & 1
			freq := FSKFreqSpace
			if bit == 1 {
				freq = FSKFreqMark
			}
			phaseIncrement := 2.0 * math.Pi * freq / float64(SampleRate)

			bitIndex := byteIdx*8 + (7 - bitPos)
			startIdx := int(float64(bitIndex) * samplesPerSymbol)
			endIdx := int(float64(bitIndex+1) * samplesPerSymbol)

			for j := startIdx; j < endIdx; j++ {
				phase += phaseIncrement
				for phase > 2.0*math.Pi {
					phase -= 2.0 * math.Pi
				}
				audioData[j] = int16(amplitude * math.Sin(phase))
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
	samplesPerBit := float64(SampleRate) / float64(baudRate)
	numBits := len(pocsagData) * 8
	messageSamples := int(float64(numBits) * samplesPerBit)

	// Add noise padding so the waterfall is filled, not empty (black)
	prePadSamples := int(0.5 * SampleRate)
	postPadSamples := int(1.5 * SampleRate)
	numSamples := prePadSamples + messageSamples + postPadSamples

	// Interleaved IQ: 2 values per sample
	samples := make([]int16, numSamples*2)

	// RF simulation parameters
	const carrierFreq = 0.0       // Center carrier at DC (0 Hz) like real SDR IQ data
	const deviation = 4500.0      // FSK deviation (±4.5 kHz for POCSAG)
	const signalAmplitude = 18000 // Strong signal
	const noiseAmplitude = 250    // RF noise floor

	carrierPhase := 0.0
	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	// FIRST PASS: Generate RF noise floor for ALL samples (creates blue background everywhere)
	for i := 0; i < numSamples; i++ {
		// Wideband RF noise on both I and Q channels
		// We want a floor around -70 to -80 dB
		noiseI := (r.Float64()*2 - 1) * noiseAmplitude
		noiseQ := (r.Float64()*2 - 1) * noiseAmplitude

		samples[i*2] = int16(noiseI)   // I noise
		samples[i*2+1] = int16(noiseQ) // Q noise
	}

	// SECOND PASS: Add FSK signal ON TOP of noise floor (creates bright signal)
	for byteIdx, b := range pocsagData {
		// Process each bit (MSB first)
		for bitPos := 7; bitPos >= 0; bitPos-- {
			bit := (b >> bitPos) & 1

			// Frequency deviation based on bit value
			var targetDev float64
			if bit == 1 {
				targetDev = deviation // +4.5 kHz
			} else {
				targetDev = -deviation // -4.5 kHz
			}

			// Generate IQ samples for this bit and ADD to existing noise
			bitIndex := byteIdx*8 + (7 - bitPos)
			startIdx := prePadSamples + int(float64(bitIndex)*samplesPerBit)
			endIdx := prePadSamples + int(float64(bitIndex+1)*samplesPerBit)

			for sampleIdx := startIdx; sampleIdx < endIdx; sampleIdx++ {
				iqIdx := sampleIdx * 2

				// FSK in POCSAG is instantaneous, with natural smoothing coming purely from the hardware/FFT windowing
				instantFreq := carrierFreq + targetDev
				carrierPhase += 2.0 * math.Pi * instantFreq / float64(SampleRate)

				// Keep phase wrapped
				for carrierPhase > 2.0*math.Pi {
					carrierPhase -= 2.0 * math.Pi
				}

				// Generate FSK signal I and Q components
				signalI := signalAmplitude * math.Cos(carrierPhase)
				signalQ := signalAmplitude * math.Sin(carrierPhase)

				// ADD signal to existing noise (bright signal on blue background)
				samples[iqIdx] += int16(signalI)   // Add to I
				samples[iqIdx+1] += int16(signalQ) // Add to Q
			}
		}
	}

	return samples
}
