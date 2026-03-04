# POCSAG-GO v2.2.1

Complete Go implementation of POCSAG pager protocol with encoder and decoder, directly ported from [pocsag-tool](https://github.com/hazardousfirmware/pocsag-tool).

## Features

- ✅ **Full POCSAG encoder** - Generate pager messages
- ✅ **Full POCSAG decoder** - Decode pager messages from WAV files
- ✅ BCH(31,21) error correction
- ✅ Address encoding/decoding (21-bit RIC/capcode; correct frame placement per ITU-R M.584-2)
- ✅ BCD numeric message encoding (function 0)
- ✅ 7-bit ASCII alphanumeric encoding (function 3)
- ✅ WAV audio: 48 kHz, 512/1200/2400 baud; works with **pocsag-decode** and **multimon-ng**
- ✅ **AES-256/AES-128 encryption** - Secure communications
- ✅ JSON output support for API integration

## Installation

```bash
# Install encoder
go install github.com/sqpp/pocsag-golang/v2/cmd/pocsag@latest

# Install decoder
go install github.com/sqpp/pocsag-golang/v2/cmd/pocsag-decode@latest

# Install burst encoder (multiple messages)
go install github.com/sqpp/pocsag-golang/v2/cmd/pocsag-burst@latest
```

Or build from source:

```bash
git clone https://github.com/sqpp/pocsag-golang.git
cd pocsag-golang
make build
# Binaries: bin/pocsag, bin/pocsag-decode, bin/pocsag-burst
```

Or without Make: `go build -o bin/pocsag ./cmd/pocsag` (and similarly for `pocsag-decode`, `pocsag-burst`).

## Usage

### Encoder

Generate POCSAG messages as WAV audio files.

**Inputs (required):**
- `--address` / `-a`: Pager address (RIC/capcode) – full 21-bit identity, e.g. 1234567. The encoder places the message in the correct frame (address % 8). Decoders typically display (address/8)×8.
- `--message` / `-m`: Message text.

**Inputs (optional):**
- `--output` / `-o`: Output WAV path (default: `output.wav`)
- `--function` / `-f`: `0` = numeric, `3` = alphanumeric (default: `3`)
- `--baud` / `-b`: `512`, `1200`, or `2400` (default: `1200`)
- `--encrypt` / `-e`: Enable AES-256 encryption
- `--key` / `-k`: Encryption key (required if `--encrypt`)
- `--json` / `-j`: Print result as JSON to stdout (no decode hint)

**Examples:**

```bash
# Alphanumeric (decode with pocsag-decode or multimon-ng)
pocsag --address 123456 --message "HELLO WORLD" --output message.wav
pocsag -a 123456 -m "HELLO WORLD" -o message.wav

# Numeric (512 baud)
pocsag --address 999888 --message "0123456789" --function 0 --baud 512 --output numeric.wav

# 2400 baud
pocsag --address 123456 --message "FAST MSG" --baud 2400 --output fast.wav

# Encrypted (AES-256)
pocsag --address 123456 --message "SECRET MESSAGE" --encrypt --key "mysecretkey" --output encrypted.wav

# JSON output (for APIs)
pocsag -a 123456 -m "TEST API" -o test.wav --json
```

**Human-readable output (default):**
```
✅ Generated message.wav
   Address: 123456, Function: 3, Baud: 1200, Message: HELLO WORLD

Decode: pocsag-decode -i message.wav  or  multimon-ng -t wav -a POCSAG1200 message.wav
```

**JSON output (`--json`):**
```json
{
  "success": true,
  "output": "message.wav",
  "address": 123456,
  "function": 3,
  "message": "HELLO WORLD",
  "baud": 1200,
  "encrypted": false,
  "type": "alphanumeric",
  "size": 38444
}
```

### Decoder

Decode POCSAG messages from WAV files (from `pocsag` or `pocsag-burst`).

**Inputs:**
- `--input` / `-i`: Input WAV file - **REQUIRED**
- `--baud` / `-b`: `512`, `1200`, or `2400` (default: `1200`)
- `--json` / `-j`: Output JSON to stdout instead of human-readable lines
- `--version` / `-v`: Show version and exit

**Examples:**
```bash
pocsag-decode --input message.wav
pocsag-decode -i message.wav

pocsag-decode -i message.wav --baud 512
pocsag-decode -i message.wav -b 2400

pocsag-decode -i message.wav --json
```

**Human-readable output:**
```
POCSAG1200: Decoded messages:
Address:  123456  Function: 3  ALPHA    Message: HELLO WORLD
```

**JSON output (`--json`):**
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
  ],
  "baud": 1200
}
```

If no messages are found (human-readable): `No messages found (tried 1200 baud)`. With `--json`, `messages` is an empty array.

### Burst Encoder (Multiple Messages)

Encode multiple messages into one WAV (decode with **pocsag-decode** or **multimon-ng**).

**Inputs:**
- `--json` / `-j`: Path to JSON file with message array - **REQUIRED**
- `--output` / `-o`: Output WAV path (default: `burst.wav`)
- `--baud` / `-b`: `512`, `1200`, or `2400` (default: `1200`)
- `--json-output` / `-jo`: Print result as JSON to stdout
- `--version` / `-v`: Show version and exit

**Input JSON format:**
```json
[
  {"address": 123456, "message": "FIRST MESSAGE", "function": 3},
  {"address": 789012, "message": "SECOND MESSAGE", "function": 3},
  {"address": 345678, "message": "0123456789", "function": 0}
]
```
- `address`: Pager address (RIC/capcode), full 21-bit value
- `message`: Message text
- `function`: `0` = numeric, `3` = alphanumeric

**Examples:**
```bash
pocsag-burst --json messages.json --output burst.wav
pocsag-burst -j messages.json -o burst.wav
pocsag-burst -j messages.json -b 512 -o burst.wav
pocsag-burst -j messages.json --json-output
```

**Human-readable output:**
```
✅ Generated burst with 3 messages: burst.wav (baud: 1200)
   1. Address: 123456, Type: ALPHA, Message: FIRST MESSAGE
   2. Address: 789012, Type: ALPHA, Message: SECOND MESSAGE
   3. Address: 345678, Type: NUMERIC, Message: 0123456789
```

**JSON output (`--json-output`):**
```json
{
  "success": true,
  "output": "burst.wav",
  "messages": [
    {
      "address": 123456,
      "function": 3,
      "message": "FIRST MESSAGE",
      "type": "alphanumeric"
    },
    {
      "address": 789012,
      "function": 3,
      "message": "SECOND MESSAGE",
      "type": "alphanumeric"
    }
  ],
  "baud": 1200,
  "count": 2,
  "size": 44844
}
```

## Library Usage

Use the `v2` module: `github.com/sqpp/pocsag-golang/v2`.

### Encoding

```go
package main

import (
    "os"
    pocsag "github.com/sqpp/pocsag-golang/v2"
)

func main() {
    // Create alphanumeric message (address = full 21-bit RIC/capcode)
    packet := pocsag.CreatePOCSAGPacket(123456, "HELLO WORLD", pocsag.FuncAlphanumeric)

    wavData := pocsag.ConvertToAudio(packet)
    os.WriteFile("output.wav", wavData, 0644)
}
```

### Decoding

Decode baseband WAVs (from `ConvertToAudio` or default `pocsag`/`pocsag-burst` output):

```go
package main

import (
    "fmt"
    "os"
    pocsag "github.com/sqpp/pocsag-golang/v2"
)

func main() {
    wavData, _ := os.ReadFile("message.wav")
    messages, err := pocsag.DecodeFromAudio(wavData)
    if err != nil {
        panic(err)
    }
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
- `CreatePOCSAGPacketWithBaudRate(address uint32, message string, function uint8, baudRate int) []byte`
  - Create a POCSAG packet (preamble, sync, address, message). Address is the full 21-bit RIC; codeword is placed in frame (address % 8) per ITU-R M.584-2.

- `ConvertToAudio(pocsagData []byte) []byte`
  - Baseband WAV (48 kHz, 16-bit PCM, mono) for **pocsag-decode**
- `ConvertToAudioWithBaudRate(pocsagData []byte, baudRate int) []byte`
  - Same, with explicit baud rate (512, 1200, 2400)

- `EncodeAddress(address uint32, function uint8) uint32`
  - Encodes an address codeword (BCH + parity). Uses 18 MSBs of the 21-bit address; caller must place it in frame (address % 8).

- `Ascii7BitEncoder(message string) []byte`
  - Encodes ASCII text to 7-bit format for alphanumeric messages

- `NumericBCDEncoder(message string) []byte`
  - Encodes numeric strings to BCD format (supports 0-9, space, -, [, ], U)

### Decoding Functions

- `DecodeFromAudio(wavData []byte) ([]DecodedMessage, error)` — default 1200 baud
- `DecodeFromAudioWithBaudRate(wavData []byte, baudRate int) ([]DecodedMessage, error)` — 512, 1200, or 2400
- `DecodeFromBinary(data []byte) ([]DecodedMessage, error)` — from raw POCSAG bytes
- `DecodeReader(r io.Reader) ([]DecodedMessage, error)` — from an io.Reader (WAV)

### Types

```go
type DecodedMessage struct {
    Address   uint32
    Function  uint8
    Message   string
    IsNumeric bool
}
```

## Encryption

POCSAG-GO supports modern encryption standards for secure communications:

### Supported Encryption Methods

- **AES-256**: Military-grade encryption (default)
- **AES-128**: High-security encryption (alternative)
- **CRC32 Integrity Verification**: Prevents message tampering
- **Base64 Encoding**: Ensures POCSAG protocol compatibility

### Security Features

- 🔐 **Password-based key derivation** using SHA256 hashing
- 🛡️ **Random IV generation** prevents replay attacks
- ✅ **Message integrity verification** with CRC32 checksums
- 🔄 **Backward compatibility** with non-encrypted messages

### Encryption Process

```
Original Message → Add CRC32 → AES-256 Encrypt → Base64 Encode → POCSAG Packet
```

### Decryption Process

```
POCSAG Packet → Base64 Decode → AES-256 Decrypt → Verify CRC32 → Original Message
```

### Usage Examples

```bash
# Create encrypted message
pocsag -a 123456 -m "SECRET MESSAGE" -e -k "mysecretkey" -o encrypted.wav

# JSON output shows encryption status
pocsag -a 123456 -m "SECRET MESSAGE" -e -k "mysecretkey" --json
```

**JSON output (`--json`) for encrypted message:** includes `"encrypted": true` and `"address"` (the RIC you passed).

### Security Notes

- **Key Management**: Use strong, unique keys for each communication group
- **Key Distribution**: Securely share keys with intended recipients
- **Compatibility**: Encrypted messages appear as Base64-encoded data in multimon-ng
- **Performance**: Encryption adds minimal overhead to message processing

## Testing with pocsag-decode and multimon-ng

The default WAV output works with both decoders:

```bash
pocsag -a 123456 -m "HELLO" -o msg.wav
pocsag-decode -i msg.wav
multimon-ng -t wav -a POCSAG1200 msg.wav
```

- 512 baud: `multimon-ng -t wav -a POCSAG512 msg.wav`
- 2400 baud: `multimon-ng -t wav -a POCSAG2400 msg.wav`

**Addresses:** The address is the full 21-bit RIC/capcode (pager number). The encoder places the codeword in the correct frame (address % 8). Decoders (e.g. multimon-ng) typically display (address/8)×8, so e.g. 1234567 may show as 1234560.

## Credits

Encoder ported from [pocsag-tool](https://github.com/hazardousfirmware/pocsag-tool) by hazardousfirmware.

## License

BSD-2-Clause (same as original pocsag-tool)

