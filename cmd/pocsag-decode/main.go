package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	pocsag "github.com/sqpp/pocsag-golang"
)

func main() {
	inputFile := flag.String("input", "", "Input WAV file to decode (required)")
	flag.StringVar(inputFile, "i", "", "Input WAV file to decode (required) - short form")

	baudRate := flag.Int("baud", pocsag.BaudRate1200, "Baud rate: 512, 1200, or 2400 (default: 1200)")
	flag.IntVar(baudRate, "b", pocsag.BaudRate1200, "Baud rate: 512, 1200, or 2400")

	jsonOutput := flag.Bool("json", false, "Output result as JSON")
	flag.BoolVar(jsonOutput, "j", false, "Output result as JSON")

	flag.Parse()

	if *inputFile == "" {
		fmt.Fprintln(os.Stderr, "Error: Input file required")
		fmt.Fprintln(os.Stderr, "\nUsage examples:")
		fmt.Fprintln(os.Stderr, "  pocsag-decode --input message.wav")
		fmt.Fprintln(os.Stderr, "  pocsag-decode -i message.wav")
		fmt.Fprintln(os.Stderr, "  pocsag-decode -i message.wav --baud 512")
		fmt.Fprintln(os.Stderr, "  pocsag-decode -i message.wav -b 2400")
		flag.Usage()
		os.Exit(1)
	}

	// Validate baud rate
	if *baudRate != pocsag.BaudRate512 && *baudRate != pocsag.BaudRate1200 && *baudRate != pocsag.BaudRate2400 {
		fmt.Fprintf(os.Stderr, "Error: Invalid baud rate %d. Supported rates: 512, 1200, 2400\n", *baudRate)
		os.Exit(1)
	}

	// Read WAV file
	data, err := os.ReadFile(*inputFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading file: %v\n", err)
		os.Exit(1)
	}

	// Decode POCSAG
	messages, err := pocsag.DecodeFromAudioWithBaudRate(data, *baudRate)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error decoding: %v\n", err)
		os.Exit(1)
	}

	if len(messages) == 0 {
		if *jsonOutput {
			result := map[string]interface{}{
				"success":  true,
				"messages": []interface{}{},
				"baud":     *baudRate,
			}
			jsonBytes, _ := json.MarshalIndent(result, "", "  ")
			fmt.Println(string(jsonBytes))
		} else {
			fmt.Printf("No messages found (tried %d baud)\n", *baudRate)
		}
		return
	}

	// Output messages
	if *jsonOutput {
		jsonMessages := make([]map[string]interface{}, len(messages))
		for i, msg := range messages {
			jsonMessages[i] = map[string]interface{}{
				"address":  msg.Address,
				"function": msg.Function,
				"message":  msg.Message,
				"type": func() string {
					if msg.IsNumeric {
						return "numeric"
					} else {
						return "alphanumeric"
					}
				}(),
			}
		}
		result := map[string]interface{}{
			"success":  true,
			"messages": jsonMessages,
			"baud":     *baudRate,
		}
		jsonBytes, _ := json.MarshalIndent(result, "", "  ")
		fmt.Println(string(jsonBytes))
	} else {
		var baudStr string
		switch *baudRate {
		case pocsag.BaudRate512:
			baudStr = "POCSAG512"
		case pocsag.BaudRate1200:
			baudStr = "POCSAG1200"
		case pocsag.BaudRate2400:
			baudStr = "POCSAG2400"
		}
		fmt.Printf("%s: Decoded messages:\n", baudStr)
		for _, msg := range messages {
			fmt.Println(msg.String())
		}
	}
}
