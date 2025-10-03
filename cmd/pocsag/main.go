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

	jsonOutput := flag.Bool("json", false, "Output result as JSON")
	flag.BoolVar(jsonOutput, "j", false, "Output result as JSON")

	flag.Parse()

	// Check required parameters
	if *address == 0 || *message == "" {
		fmt.Fprintln(os.Stderr, "Error: Address and message are required")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Note: POCSAG addresses must be multiples of 8")
		fmt.Fprintln(os.Stderr, "      (e.g., 8, 16, 24, 123456, 1234560)")
		fmt.Fprintln(os.Stderr, "\nUsage examples:")
		fmt.Fprintln(os.Stderr, "  # Alphanumeric message:")
		fmt.Fprintln(os.Stderr, "  pocsag --address 123456 --message \"HELLO WORLD\" --output test.wav")
		fmt.Fprintln(os.Stderr, "  pocsag -a 123456 -m \"HELLO WORLD\" -o test.wav")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "  # Numeric message:")
		fmt.Fprintln(os.Stderr, "  pocsag --address 999888 --message \"0123456789\" --function 0 --output num.wav")
		fmt.Fprintln(os.Stderr, "  pocsag -a 999888 -m \"0123456789\" -f 0 -o num.wav")
		fmt.Fprintln(os.Stderr, "")
		flag.Usage()
		os.Exit(1)
	}

	// Create POCSAG packet
	packet := pocsag.CreatePOCSAGPacket(uint32(*address), *message, uint8(*funcCode))

	// Convert to audio
	wavData := pocsag.ConvertToAudio(packet)

	// Write to file
	err := os.WriteFile(*output, wavData, 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error writing file: %v\n", err)
		os.Exit(1)
	}

	// Output result
	if *jsonOutput {
		result := map[string]interface{}{
			"success":  true,
			"output":   *output,
			"address":  *address,
			"function": *funcCode,
			"message":  *message,
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
		fmt.Printf("✅ Generated %s\n", *output)
		fmt.Printf("   Address: %d, Function: %d, Message: %s\n", *address, *funcCode, *message)
		fmt.Printf("\nTest with: multimon-ng -t wav -a POCSAG1200 %s\n", *output)
	}
}
