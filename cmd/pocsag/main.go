package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"math/cmplx"
	"os"

	pocsag "github.com/sqpp/pocsag-golang/v2"
)

func main() {
	address := flag.Uint("address", 0, "Pager address (RIC) - REQUIRED")
	flag.UintVar(address, "a", 0, "Pager address (RIC) - REQUIRED")

	message := flag.String("message", "", "Message text to send - REQUIRED")
	flag.StringVar(message, "m", "", "Message text to send - REQUIRED")

	output := flag.String("output", "output.wav", "Output WAV file path")
	flag.StringVar(output, "o", "output.wav", "Output WAV file path")

	funcCode := flag.Uint("function", pocsag.FuncAlphanumeric, "Message type: 0=numeric, 3=alphanumeric (default: 3)")
	flag.UintVar(funcCode, "f", pocsag.FuncAlphanumeric, "Message type: 0=numeric, 3=alphanumeric")

	baudRate := flag.Int("baud", pocsag.BaudRate1200, "Baud rate: 512, 1200, or 2400 (default: 1200)")
	flag.IntVar(baudRate, "b", pocsag.BaudRate1200, "Baud rate: 512, 1200, or 2400")

	waterfallFile := flag.String("waterfall", "", "Output waterfall PNG file path (optional)")
	flag.StringVar(waterfallFile, "w", "", "Output waterfall PNG file path (optional)")

	encrypt := flag.Bool("encrypt", false, "Enable AES-256 encryption")
	flag.BoolVar(encrypt, "e", false, "Enable AES-256 encryption")

	key := flag.String("key", "", "Encryption key (required if --encrypt is used)")
	flag.StringVar(key, "k", "", "Encryption key (required if --encrypt is used)")

	jsonOutput := flag.Bool("json", false, "Output result as JSON")
	flag.BoolVar(jsonOutput, "j", false, "Output result as JSON")

	version := flag.Bool("version", false, "Show version information")
	flag.BoolVar(version, "v", false, "Show version information")

	flag.Parse()

	if *version {
		fmt.Println(pocsag.GetFullVersionInfo())
		os.Exit(0)
	}

	if *address == 0 || *message == "" {
		fmt.Fprintln(os.Stderr, "Error: Address and message are required")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Note: POCSAG addresses must be multiples of 8")
		fmt.Fprintln(os.Stderr, "      (e.g., 8, 16, 24, 123456, 1234560)")
		fmt.Fprintln(os.Stderr, "\nUsage examples:")
		fmt.Fprintln(os.Stderr, "  pocsag --address 123456 --message \"HELLO WORLD\" --output test.wav")
		fmt.Fprintln(os.Stderr, "  pocsag -a 123456 -m \"HELLO WORLD\" -o test.wav -w waterfall.png")
		fmt.Fprintln(os.Stderr, "")
		flag.Usage()
		os.Exit(1)
	}

	if *encrypt && *key == "" {
		fmt.Fprintln(os.Stderr, "Error: Encryption key is required when --encrypt is used")
		os.Exit(1)
	}

	if *baudRate != pocsag.BaudRate512 && *baudRate != pocsag.BaudRate1200 && *baudRate != pocsag.BaudRate2400 {
		fmt.Fprintf(os.Stderr, "Error: Invalid baud rate %d. Supported rates: 512, 1200, 2400\n", *baudRate)
		os.Exit(1)
	}

	addressVal := uint32(*address)

	var packet []byte
	var err error

	if *encrypt {
		encryptionConfig := pocsag.EncryptionConfig{
			Method: pocsag.EncryptionAES256,
			Key:    pocsag.KeyFromPassword(*key, 32),
		}
		packet, err = pocsag.CreatePOCSAGPacketWithEncryption(addressVal, *message, uint8(*funcCode), *baudRate, encryptionConfig)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error creating encrypted packet: %v\n", err)
			os.Exit(1)
		}
	} else {
		packet = pocsag.CreatePOCSAGPacketWithBaudRate(addressVal, *message, uint8(*funcCode), *baudRate)
	}

	// Generate waterfall PNG via OpenGL (headless offscreen rendering)
	if *waterfallFile != "" {
		iqSamples := pocsag.GenerateFSKSamples(packet, *baudRate)
		cfg := pocsag.DefaultWaterfallConfig()

		// Calculate the frequency bins we want to display
		freqBinSize := float64(cfg.SampleRate) / float64(cfg.FFTSize)
		halfFs := float64(cfg.SampleRate) / 2.0
		minBin := int((cfg.MinFreq + halfFs) / freqBinSize)
		maxBin := int((cfg.MaxFreq + halfFs) / freqBinSize)
		if minBin < 0 {
			minBin = 0
		}
		if maxBin > cfg.FFTSize {
			maxBin = cfg.FFTSize
		}
		numBins := maxBin - minBin

		// Create OpenGL renderer in headless mode (no window shown)
		wgl, err := pocsag.NewWaterfallGL(numBins, cfg.Height, true)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error initializing OpenGL: %v\n", err)
			os.Exit(1)
		}
		defer wgl.Close()

		// Convert IQ samples to complex
		numComplexSamples := len(iqSamples) / 2
		complexSamples := make([]complex128, numComplexSamples)
		for i := 0; i < numComplexSamples; i++ {
			complexSamples[i] = complex(float64(iqSamples[i*2])/32768.0, float64(iqSamples[i*2+1])/32768.0)
		}

		// Process FFT windows and upload each row to the OpenGL texture
		stepSize := int(float64(cfg.FFTSize) * (1.0 - cfg.Overlap))
		if stepSize < 1 {
			stepSize = 1
		}
		numWindows := (numComplexSamples - cfg.FFTSize) / stepSize

		for windowIdx := 0; windowIdx < numWindows; windowIdx++ {
			startIdx := windowIdx * stepSize

			// Apply Hann window
			window := make([]complex128, cfg.FFTSize)
			for i := 0; i < cfg.FFTSize; i++ {
				hannWeight := 0.5 * (1.0 - math.Cos(2.0*math.Pi*float64(i)/float64(cfg.FFTSize-1)))
				window[i] = complexSamples[startIdx+i] * complex(hannWeight, 0)
			}

			// FFT + normalize
			coeffs := pocsag.ComplexFFT(window)
			for i := range coeffs {
				coeffs[i] /= complex(float64(cfg.FFTSize), 0)
			}

			// FFT shift so DC is centered
			shifted := make([]complex128, cfg.FFTSize)
			half := cfg.FFTSize / 2
			for i := 0; i < cfg.FFTSize; i++ {
				shifted[i] = coeffs[(i+half)%cfg.FFTSize]
			}

			// Extract only the frequency bins we want to display
			floatData := make([]float32, numBins)
			for i := 0; i < numBins; i++ {
				binIdx := minBin + i
				if binIdx >= len(shifted) {
					break
				}
				mag := cmplx.Abs(shifted[binIdx])
				floatData[i] = float32(mag * mag)
			}

			wgl.AddLine(floatData)
		}

		// Render once to flush everything to the framebuffer, then save
		wgl.Render()
		if err := wgl.SaveToPNG(*waterfallFile); err != nil {
			fmt.Fprintf(os.Stderr, "Error saving waterfall: %v\n", err)
			os.Exit(1)
		}
	}

	// Convert to WAV
	wavData := pocsag.ConvertToAudioWithBaudRate(packet, *baudRate)

	err = os.WriteFile(*output, wavData, 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error writing WAV file: %v\n", err)
		os.Exit(1)
	}

	if *jsonOutput {
		result := map[string]interface{}{
			"success":   true,
			"output":    *output,
			"address":   *address,
			"function":  *funcCode,
			"message":   *message,
			"baud":      *baudRate,
			"encrypted": *encrypt,
			"type": func() string {
				if *funcCode == 0 {
					return "numeric"
				}
				return "alphanumeric"
			}(),
			"size":       len(wavData),
			"duration_s": float64((len(wavData)-44)/2) / float64(pocsag.SampleRate),
		}
		jsonBytes, _ := json.MarshalIndent(result, "", "  ")
		fmt.Println(string(jsonBytes))
	} else {
		encryptionStatus := ""
		if *encrypt {
			encryptionStatus = " (encrypted)"
		}
		fmt.Printf("✅ Generated %s%s\n", *output, encryptionStatus)
		if *waterfallFile != "" {
			fmt.Printf("✅ Generated waterfall: %s\n", *waterfallFile)
		}
		fmt.Printf("   Address: %d, Function: %d, Baud: %d, Message: %s\n", *address, *funcCode, *baudRate, *message)
		numSamples := (len(wavData) - 44) / 2
		durationSec := float64(numSamples) / float64(pocsag.SampleRate)
		fmt.Printf("   Size: %d bytes, Duration: %.2f s\n", len(wavData), durationSec)
		fmt.Printf("\nDecode: pocsag-decode -i %s  or  multimon-ng -t wav -a POCSAG%d %s\n", *output, *baudRate, *output)
		if *encrypt {
			fmt.Printf("Note: This message is encrypted. Use pocsag-decode with --key to decrypt.\n")
		}
	}
}
