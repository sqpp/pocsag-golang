package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	pocsag "github.com/sqpp/pocsag-golang"
)

func main() {
	jsonInput := flag.String("json", "", "JSON input file with message array (required)")
	flag.StringVar(jsonInput, "j", "", "JSON input file - short form")

	output := flag.String("output", "burst.wav", "Output WAV file path")
	flag.StringVar(output, "o", "burst.wav", "Output WAV file path")

	flag.Parse()

	if *jsonInput == "" {
		fmt.Fprintln(os.Stderr, "Error: JSON input file required")
		fmt.Fprintln(os.Stderr, "\nUsage examples:")
		fmt.Fprintln(os.Stderr, "  pocsag-burst --json messages.json --output burst.wav")
		fmt.Fprintln(os.Stderr, "  pocsag-burst -j messages.json -o burst.wav")
		fmt.Fprintln(os.Stderr, "\nJSON format:")
		fmt.Fprintln(os.Stderr, `  [
    {"address": 123456, "message": "FIRST MESSAGE", "function": 3},
    {"address": 789012, "message": "SECOND MESSAGE", "function": 3},
    {"address": 345678, "message": "0123456789", "function": 0}
  ]`)
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
	packet := pocsag.CreatePOCSAGBurst(messages)
	wavData := pocsag.ConvertToAudio(packet)

	// Write to file
	err = os.WriteFile(*output, wavData, 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error writing file: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✅ Generated burst with %d messages: %s\n", len(messages), *output)
	for i, msg := range messages {
		msgType := "ALPHA"
		if msg.Function == 0 {
			msgType = "NUMERIC"
		}
		fmt.Printf("   %d. Address: %d, Type: %s, Message: %s\n", i+1, msg.Address, msgType, msg.Message)
	}
}
