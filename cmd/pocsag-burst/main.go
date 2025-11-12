package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	pocsag "github.com/sqpp/pocsag-golang/v2"
)

func main() {
	jsonInput := flag.String("json", "", "JSON input file with message array (required)")
	flag.StringVar(jsonInput, "j", "", "JSON input file - short form")

	output := flag.String("output", "burst.wav", "Output WAV file path")
	flag.StringVar(output, "o", "burst.wav", "Output WAV file path")

	baudRate := flag.Int("baud", pocsag.BaudRate1200, "Baud rate: 512, 1200, or 2400 (default: 1200)")
	flag.IntVar(baudRate, "b", pocsag.BaudRate1200, "Baud rate: 512, 1200, or 2400")

	jsonOutput := flag.Bool("json-output", false, "Output result as JSON")
	flag.BoolVar(jsonOutput, "jo", false, "Output result as JSON - short form")

	version := flag.Bool("version", false, "Show version information")
	flag.BoolVar(version, "v", false, "Show version information")

	flag.Parse()

	// Handle version flag
	if *version {
		fmt.Println(pocsag.GetFullVersionInfo())
		os.Exit(0)
	}

	if *jsonInput == "" {
		fmt.Fprintln(os.Stderr, "Error: JSON input file required")
		fmt.Fprintln(os.Stderr, "\nUsage examples:")
		fmt.Fprintln(os.Stderr, "  pocsag-burst --json messages.json --output burst.wav")
		fmt.Fprintln(os.Stderr, "  pocsag-burst -j messages.json -o burst.wav")
		fmt.Fprintln(os.Stderr, "  pocsag-burst -j messages.json --baud 512 -o burst.wav")
		fmt.Fprintln(os.Stderr, "  pocsag-burst -j messages.json -b 2400 -o burst.wav")
		fmt.Fprintln(os.Stderr, "  pocsag-burst -j messages.json --json-output")
		fmt.Fprintln(os.Stderr, "  pocsag-burst -j messages.json -jo")
		fmt.Fprintln(os.Stderr, "\nJSON format:")
		fmt.Fprintln(os.Stderr, `  [
    {"address": 123456, "message": "FIRST MESSAGE", "function": 3},
    {"address": 789012, "message": "SECOND MESSAGE", "function": 3},
    {"address": 345678, "message": "0123456789", "function": 0}
  ]`)
		os.Exit(1)
	}

	// Validate baud rate
	if *baudRate != pocsag.BaudRate512 && *baudRate != pocsag.BaudRate1200 && *baudRate != pocsag.BaudRate2400 {
		fmt.Fprintf(os.Stderr, "Error: Invalid baud rate %d. Supported rates: 512, 1200, 2400\n", *baudRate)
		os.Exit(1)
	}

	// Read JSON file
	jsonData, err := os.ReadFile(*jsonInput)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading JSON file: %v\n", err)
		os.Exit(1)
	}

	// Parse JSON
	type JSONMessage struct {
		Address  uint32 `json:"address"`
		Message  string `json:"message"`
		Function uint8  `json:"function"`
	}
	var jsonMessages []JSONMessage
	err = json.Unmarshal(jsonData, &jsonMessages)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing JSON: %v\n", err)
		os.Exit(1)
	}

	// Convert to MessageInfo
	messages := make([]pocsag.MessageInfo, len(jsonMessages))
	for i, jm := range jsonMessages {
		messages[i] = pocsag.MessageInfo{
			Address:  jm.Address,
			Message:  jm.Message,
			Function: jm.Function,
		}
	}

	// Generate burst
	packet := pocsag.CreatePOCSAGBurstWithBaudRate(messages, *baudRate)
	wavData := pocsag.ConvertToAudioWithBaudRate(packet, *baudRate)

	// Write to file
	err = os.WriteFile(*output, wavData, 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error writing file: %v\n", err)
		os.Exit(1)
	}

	// Output result
	if *jsonOutput {
		jsonMessages := make([]map[string]interface{}, len(messages))
		for i, msg := range messages {
			jsonMessages[i] = map[string]interface{}{
				"address":  msg.Address,
				"message":  msg.Message,
				"function": msg.Function,
				"type": func() string {
					if msg.Function == 0 {
						return "numeric"
					} else {
						return "alphanumeric"
					}
				}(),
			}
		}
		result := map[string]interface{}{
			"success":  true,
			"output":   *output,
			"messages": jsonMessages,
			"baud":     *baudRate,
			"count":    len(messages),
			"size":     len(wavData),
		}
		jsonBytes, _ := json.MarshalIndent(result, "", "  ")
		fmt.Println(string(jsonBytes))
	} else {
		fmt.Printf("✅ Generated burst with %d messages: %s (baud: %d)\n", len(messages), *output, *baudRate)
		for i, msg := range messages {
			msgType := "ALPHA"
			if msg.Function == 0 {
				msgType = "NUMERIC"
			}
			fmt.Printf("   %d. Address: %d, Type: %s, Message: %s\n", i+1, msg.Address, msgType, msg.Message)
		}
	}
}
