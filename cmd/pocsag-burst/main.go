package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

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
    {"address": 345678, "message": "0123456789", "function": 1, "payload_type": "numeric"}
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
		Address     uint32 `json:"address"`
		Message     string `json:"message"`
		Function    uint8  `json:"function"`
		PayloadType string `json:"payload_type"`
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
		payloadType := normalizePayloadType(jm.PayloadType)
		if jm.PayloadType != "" && payloadType == "" {
			fmt.Fprintf(os.Stderr, "Error: Invalid payload_type for message %d. Supported types: numeric, alpha\n", i+1)
			os.Exit(1)
		}
		messages[i] = pocsag.MessageInfo{
			Address:     jm.Address,
			Message:     jm.Message,
			Function:    jm.Function,
			PayloadType: payloadType,
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
				"type":     displayPayloadType(msg.Function, msg.PayloadType),
			}
		}
		numSamples := (len(wavData) - 44) / 2
		durationSec := float64(numSamples) / float64(pocsag.SampleRate)
		result := map[string]interface{}{
			"success":    true,
			"output":     *output,
			"messages":   jsonMessages,
			"baud":       *baudRate,
			"count":      len(messages),
			"size":       len(wavData),
			"duration_s": durationSec,
		}
		jsonBytes, _ := json.MarshalIndent(result, "", "  ")
		fmt.Println(string(jsonBytes))
	} else {
		numSamples := (len(wavData) - 44) / 2
		durationSec := float64(numSamples) / float64(pocsag.SampleRate)
		fmt.Printf("✅ Generated burst with %d messages: %s (baud: %d)\n", len(messages), *output, *baudRate)
		fmt.Printf("   Size: %d bytes, Duration: %.2f s\n", len(wavData), durationSec)
		for i, msg := range messages {
			msgType := "ALPHA"
			if displayPayloadType(msg.Function, msg.PayloadType) == "numeric" {
				msgType = "NUMERIC"
			}
			fmt.Printf("   %d. Address: %d, Type: %s, Message: %s\n", i+1, msg.Address, msgType, msg.Message)
		}
	}
}

func normalizePayloadType(payloadType string) string {
	switch strings.ToLower(strings.TrimSpace(payloadType)) {
	case "":
		return ""
	case "numeric":
		return pocsag.PayloadTypeNumeric
	case "alpha", "alphanumeric":
		return pocsag.PayloadTypeAlpha
	default:
		return ""
	}
}

func displayPayloadType(function uint8, payloadType string) string {
	if payloadType == pocsag.PayloadTypeNumeric {
		return "numeric"
	}
	if payloadType == pocsag.PayloadTypeAlpha {
		return "alphanumeric"
	}
	if function == pocsag.FuncNumeric {
		return "numeric"
	}
	return "alphanumeric"
}
