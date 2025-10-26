package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	pocsag "github.com/sqpp/pocsag-golang"
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

	encrypt := flag.Bool("encrypt", false, "Enable AES-256 encryption")
	flag.BoolVar(encrypt, "e", false, "Enable AES-256 encryption")

	key := flag.String("key", "", "Encryption key (required if --encrypt is used)")
	flag.StringVar(key, "k", "", "Encryption key (required if --encrypt is used)")

	jsonOutput := flag.Bool("json", false, "Output result as JSON")
	flag.BoolVar(jsonOutput, "j", false, "Output result as JSON")

	version := flag.Bool("version", false, "Show version information")
	flag.BoolVar(version, "v", false, "Show version information")

	flag.Parse()

	// Handle version flag
	if *version {
		fmt.Println(pocsag.GetFullVersionInfo())
		os.Exit(0)
	}

	// Check required parameters
	if *address == 0 || *message == "" {
		fmt.Fprintln(os.Stderr, "Error: Address and message are required")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Note: POCSAG addresses must be multiples of 8")
		fmt.Fprintln(os.Stderr, "      (e.g., 8, 16, 24, 123456, 1234560)")
		fmt.Fprintln(os.Stderr, "\nUsage examples:")
		fmt.Fprintln(os.Stderr, "  # Alphanumeric message (1200 baud):")
		fmt.Fprintln(os.Stderr, "  pocsag --address 123456 --message \"HELLO WORLD\" --output test.wav")
		fmt.Fprintln(os.Stderr, "  pocsag -a 123456 -m \"HELLO WORLD\" -o test.wav")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "  # Numeric message (512 baud):")
		fmt.Fprintln(os.Stderr, "  pocsag --address 999888 --message \"0123456789\" --function 0 --baud 512 --output num.wav")
		fmt.Fprintln(os.Stderr, "  pocsag -a 999888 -m \"0123456789\" -f 0 -b 512 -o num.wav")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "  # High speed message (2400 baud):")
		fmt.Fprintln(os.Stderr, "  pocsag --address 123456 --message \"FAST MSG\" --baud 2400 --output fast.wav")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "  # Encrypted message:")
		fmt.Fprintln(os.Stderr, "  pocsag --address 123456 --message \"SECRET MSG\" --encrypt --key \"mysecretkey\" --output encrypted.wav")
		fmt.Fprintln(os.Stderr, "  pocsag -a 123456 -m \"SECRET MSG\" -e -k \"mysecretkey\" -o encrypted.wav")
		fmt.Fprintln(os.Stderr, "")
		flag.Usage()
		os.Exit(1)
	}

	// Validate encryption parameters
	if *encrypt && *key == "" {
		fmt.Fprintln(os.Stderr, "Error: Encryption key is required when --encrypt is used")
		fmt.Fprintln(os.Stderr, "Use --key or -k to specify the encryption key")
		os.Exit(1)
	}

	// Validate baud rate
	if *baudRate != pocsag.BaudRate512 && *baudRate != pocsag.BaudRate1200 && *baudRate != pocsag.BaudRate2400 {
		fmt.Fprintf(os.Stderr, "Error: Invalid baud rate %d. Supported rates: 512, 1200, 2400\n", *baudRate)
		os.Exit(1)
	}

	// Create POCSAG packet
	var packet []byte
	var err error

	if *encrypt {
		// Create encryption config
		encryptionConfig := pocsag.EncryptionConfig{
			Method: pocsag.EncryptionAES256,
			Key:    pocsag.KeyFromPassword(*key, 32), // 32 bytes for AES-256
		}

		packet, err = pocsag.CreatePOCSAGPacketWithEncryption(uint32(*address), *message, uint8(*funcCode), *baudRate, encryptionConfig)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error creating encrypted packet: %v\n", err)
			os.Exit(1)
		}
	} else {
		packet = pocsag.CreatePOCSAGPacketWithBaudRate(uint32(*address), *message, uint8(*funcCode), *baudRate)
	}

	// Convert to audio
	wavData := pocsag.ConvertToAudioWithBaudRate(packet, *baudRate)

	// Write to file
	err = os.WriteFile(*output, wavData, 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error writing file: %v\n", err)
		os.Exit(1)
	}

	// Output result
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
				} else {
					return "alphanumeric"
				}
			}(),
			"size": len(wavData),
		}
		jsonBytes, _ := json.MarshalIndent(result, "", "  ")
		fmt.Println(string(jsonBytes))
	} else {
		encryptionStatus := ""
		if *encrypt {
			encryptionStatus = " (encrypted)"
		}
		fmt.Printf("✅ Generated %s%s\n", *output, encryptionStatus)
		fmt.Printf("   Address: %d, Function: %d, Baud: %d, Message: %s\n", *address, *funcCode, *baudRate, *message)

		// Show appropriate multimon-ng command based on baud rate
		var multimonCmd string
		switch *baudRate {
		case pocsag.BaudRate512:
			multimonCmd = fmt.Sprintf("multimon-ng -t wav -a POCSAG512 %s", *output)
		case pocsag.BaudRate1200:
			multimonCmd = fmt.Sprintf("multimon-ng -t wav -a POCSAG1200 %s", *output)
		case pocsag.BaudRate2400:
			multimonCmd = fmt.Sprintf("multimon-ng -t wav -a POCSAG2400 %s", *output)
		}
		fmt.Printf("\nTest with: %s\n", multimonCmd)
		if *encrypt {
			fmt.Printf("Note: This message is encrypted. Use pocsag-decode with --key to decrypt.\n")
		}
	}
}
