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

	jsonOutput := flag.Bool("json", false, "Output result as JSON")
	flag.BoolVar(jsonOutput, "j", false, "Output result as JSON")

	flag.Parse()

	if *inputFile == "" {
		fmt.Fprintln(os.Stderr, "Error: Input file required")
		fmt.Fprintln(os.Stderr, "\nUsage examples:")
		fmt.Fprintln(os.Stderr, "  pocsag-decode --input message.wav")
		fmt.Fprintln(os.Stderr, "  pocsag-decode -i message.wav")
		flag.Usage()
		os.Exit(1)
	}

	// Read WAV file
	data, err := os.ReadFile(*inputFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading file: %v\n", err)
		os.Exit(1)
	}

	// Decode POCSAG
	messages, err := pocsag.DecodeFromAudio(data)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error decoding: %v\n", err)
		os.Exit(1)
	}

	if len(messages) == 0 {
		if *jsonOutput {
			result := map[string]interface{}{
				"success":  true,
				"messages": []interface{}{},
			}
			jsonBytes, _ := json.MarshalIndent(result, "", "  ")
			fmt.Println(string(jsonBytes))
		} else {
			fmt.Println("No messages found")
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
		}
		jsonBytes, _ := json.MarshalIndent(result, "", "  ")
		fmt.Println(string(jsonBytes))
	} else {
		fmt.Println("POCSAG1200: Decoded messages:")
		for _, msg := range messages {
			fmt.Println(msg.String())
		}
	}
}
