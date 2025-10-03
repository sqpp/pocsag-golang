# POCSAG-GO

Complete Go implementation of POCSAG pager protocol with encoder and decoder, directly ported from [pocsag-tool](https://github.com/hazardousfirmware/pocsag-tool).

## Features

- ✅ **Full POCSAG encoder** - Generate pager messages
- ✅ **Full POCSAG decoder** - Decode pager messages from WAV files
- ✅ BCH(31,21) error correction
- ✅ Address encoding/decoding
- ✅ BCD numeric message encoding (function 0)
- ✅ 7-bit ASCII alphanumeric encoding (function 3)
- ✅ WAV audio generation and decoding (48kHz, 1200 baud)
- ✅ Compatible with PDW and multimon-ng
- ✅ JSON output support for API integration

## Installation

```bash
# Install encoder
go install github.com/sqpp/pocsag-golang/cmd/pocsag@latest

# Install decoder
go install github.com/sqpp/pocsag-golang/cmd/pocsag-decode@latest

# Install burst encoder (multiple messages)
go install github.com/sqpp/pocsag-golang/cmd/pocsag-burst@latest
```

Or build from source:

```bash
git clone https://github.com/sqpp/pocsag-golang.git
cd pocsag-golang
go build ./cmd/pocsag
go build ./cmd/pocsag-decode
go build ./cmd/pocsag-burst
```

## Usage

### Encoder

Generate POCSAG messages as WAV audio files:

```bash
# Alphanumeric message (default)
pocsag --address 123456 --message "HELLO WORLD" --output message.wav
pocsag -a 123456 -m "HELLO WORLD" -o message.wav

# Numeric message
pocsag --address 999888 --message "0123456789" --function 0 --output numeric.wav
pocsag -a 999888 -m "0123456789" -f 0 -o numeric.wav

# JSON output (for API integration)
pocsag -a 123456 -m "TEST API" -o test.wav --json
```

**Parameters:**
- `--address` / `-a`: Pager address (RIC) - **REQUIRED**
- `--message` / `-m`: Message text - **REQUIRED**
- `--output` / `-o`: Output WAV file (default: `output.wav`)
- `--function` / `-f`: Message type - `0` for numeric, `3` for alphanumeric (default: `3`)
- `--json` / `-j`: Output result as JSON

### Decoder

Decode POCSAG messages from WAV files:

```bash
pocsag-decode --input message.wav
pocsag-decode -i message.wav

# JSON output (for API integration)
pocsag-decode -i message.wav --json
```

**Output example:**
```
POCSAG1200: Decoded messages:
Address:  123456  Function: 3  ALPHA    Message: HELLO WORLD
```

**JSON output example:**
```json
{
  "success": true,
  "messages": [
    {
      "address": 123456,
      "function": 3,
      "message": "HELLO WORLD",
      "type": "alphanumeric"
    }
  ]
}
```

### Burst Encoder (Multiple Messages)

Send multiple messages in one transmission:

```bash
# Create JSON file with messages
cat > messages.json << 'EOF'
[
  {"address": 123456, "message": "FIRST MESSAGE", "function": 3},
  {"address": 789012, "message": "SECOND MESSAGE", "function": 3},
  {"address": 345678, "message": "0123456789", "function": 0}
]
EOF

# Generate burst
pocsag-burst --json messages.json --output burst.wav
pocsag-burst -j messages.json -o burst.wav
```

**Parameters:**
- `--json` / `-j`: JSON input file with message array - **REQUIRED**
- `--output` / `-o`: Output WAV file (default: `burst.wav`)

**JSON Format:**
```json
[
  {
    "address": 123456,
    "message": "Your message here",
    "function": 3
  }
]
```
- `address`: Pager address (RIC)
- `message`: Message text
- `function`: `0` for numeric, `3` for alphanumeric

## Library Usage

### Encoding

```go
package main

import (
    "os"
    pocsag "github.com/sqpp/pocsag-golang"
)

func main() {
    // Create alphanumeric message
    packet := pocsag.CreatePOCSAGPacket(123456, "HELLO WORLD", pocsag.FuncAlphanumeric)
    
    // Convert to WAV audio
    wavData := pocsag.ConvertToAudio(packet)
    
    // Save to file
    os.WriteFile("output.wav", wavData, 0644)
}
```

### Decoding

```go
package main

import (
    "fmt"
    "os"
    pocsag "github.com/sqpp/pocsag-golang"
)

func main() {
    // Read WAV file
    wavData, _ := os.ReadFile("message.wav")
    
    // Decode messages
    messages, err := pocsag.DecodeFromAudio(wavData)
    if err != nil {
        panic(err)
    }
    
    // Print decoded messages
    for _, msg := range messages {
        fmt.Println(msg.String())
    }
}
```

## API Reference

### Constants

- `FuncNumeric` (0) - Numeric messages
- `FuncAlphanumeric` (3) - Alphanumeric messages
- `FrameSyncWord` (0x7CD215D8) - POCSAG frame sync
- `IdleCodeword` (0x7A89C197) - Idle codeword

### Encoding Functions

- `CreatePOCSAGPacket(address uint32, message string, function uint8) []byte`
  - Creates a complete POCSAG packet with preamble, sync, address, and message
  
- `ConvertToAudio(pocsagData []byte) []byte`
  - Converts POCSAG bytes to WAV audio (48kHz, 16-bit PCM, mono)

- `EncodeAddress(address uint32, function uint8) uint32`
  - Encodes an address codeword with BCH and parity

- `Ascii7BitEncoder(message string) []byte`
  - Encodes ASCII text to 7-bit format for alphanumeric messages

- `NumericBCDEncoder(message string) []byte`
  - Encodes numeric strings to BCD format (supports 0-9, space, -, [, ], U)

### Decoding Functions

- `DecodeFromAudio(wavData []byte) ([]DecodedMessage, error)`
  - Decodes POCSAG messages from WAV audio data

- `DecodeFromBinary(data []byte) ([]DecodedMessage, error)`
  - Decodes POCSAG messages from raw binary data

- `DecodeReader(r io.Reader) ([]DecodedMessage, error)`
  - Decodes POCSAG from an io.Reader

### Types

```go
type DecodedMessage struct {
    Address   uint32
    Function  uint8
    Message   string
    IsNumeric bool
}
```

## Testing with PDW / multimon-ng

The generated WAV files are compatible with:
- **PDW** (Paging Decode for Windows)
- **multimon-ng** - `multimon-ng -t wav -a POCSAG1200 message.wav`

## Credits

Encoder ported from [pocsag-tool](https://github.com/hazardousfirmware/pocsag-tool) by hazardousfirmware.

## License

BSD-2-Clause (same as original pocsag-tool)

