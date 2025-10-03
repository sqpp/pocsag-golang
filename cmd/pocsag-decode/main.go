package main

import (
	"flag"
	"fmt"
	"os"

	pocsag "github.com/sqpp/pocsag-golang"
)

func main() {
	inputFile := flag.String("input", "", "Input WAV file to decode (required)")
	flag.StringVar(inputFile, "i", "", "Input WAV file to decode (required) - short form")

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
		fmt.Println("No messages found")
		return
	}

	// Display messages
	fmt.Println("POCSAG1200: Decoded messages:")
	for _, msg := range messages {
		fmt.Println(msg.String())
	}
}
